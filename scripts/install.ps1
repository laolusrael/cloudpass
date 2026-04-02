# CloudPass Windows Service Installation Script
# Usage: .\install.ps1 [-Port <int>] [-Help]

param(
    [int]$Port = 8080,
    [switch]$Help
)

if ($Help) {
    Write-Host "Usage: .\install.ps1 [-Port <port>]" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Options:" -ForegroundColor Cyan
    Write-Host "  -Port <port>    Port to run CloudPass on (default: 8080)" -ForegroundColor Cyan
    Write-Host "  -Help           Show this help message" -ForegroundColor Cyan
    exit 0
}

$ErrorActionPreference = "Stop"

function Write-Info {
    param([string]$Message)
    Write-Host $Message -ForegroundColor Green
}

function Write-Warn {
    param([string]$Message)
    Write-Host $Message -ForegroundColor Yellow
}

function Write-Error {
    param([string]$Message)
    Write-Host $Message -ForegroundColor Red
    exit 1
}

function Test-Admin {
    $currentUser = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($currentUser)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

if (-not (Test-Admin)) {
    Write-Error "This script must be run as Administrator"
}

# Validate port
if ($Port -lt 1 -or $Port -gt 65535) {
    Write-Error "Invalid port: $Port. Port must be between 1 and 65535"
}

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$BaseDir = Split-Path -Parent $ScriptDir
$InstallDir = "C:\Program Files\CloudPass"
$CloudPassUser = "CloudPass"
$SSHKeySource = "C:\Windows\System32\config\systemprofile\AppData\Roaming\multipassd\ssh-keys\id_rsa"
$SSHKeyDir = "$InstallDir\.cloudpass"
$SSHKeyTarget = "$SSHKeyDir\id_rsa"

Write-Info "Installing CloudPass..."

# Check for cloudpass.exe in script directory or base directory
$SourceDir = $ScriptDir
if (-not (Test-Path "$ScriptDir\cloudpass.exe")) {
    if (Test-Path "$BaseDir\cloudpass.exe") {
        $SourceDir = $BaseDir
    } else {
        Write-Error "cloudpass.exe not found. Please extract the release first."
    }
}

if (-not (Test-Path "$SourceDir\config.yaml")) {
    Write-Error "config.yaml not found. Please extract the release first."
}

# Create CloudPass user if not exists
try {
    $user = Get-LocalUser -Name $CloudPassUser -ErrorAction SilentlyContinue
    if (-not $user) {
        Write-Info "Creating user $CloudPassUser..."
        New-LocalUser -Name $CloudPassUser -NoPassword -Description "CloudPass Service Account" | Out-Null
    } else {
        Write-Warn "User $CloudPassUser already exists"
    }
} catch {
    # Fallback: try using net user
    Write-Info "Creating user $CloudPassUser..."
    net user $CloudPassUser /add 2>$null || Write-Warn "User may already exist"
}

# Create installation directory
Write-Info "Creating installation directory..."
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

# Copy files
Write-Info "Copying files..."
Copy-Item "$SourceDir\cloudpass.exe" "$InstallDir\" -Force
Copy-Item "$SourceDir\config.yaml" "$InstallDir\" -Force

# Copy SSH key if it exists
if (Test-Path $SSHKeySource) {
    Write-Info "Copying SSH key..."
    New-Item -ItemType Directory -Force -Path $SSHKeyDir | Out-Null
    Copy-Item $SSHKeySource $SSHKeyTarget -Force
    
    # Set permissions: Administrators full control, CloudPass read
    $acl = Get-Acl $SSHKeyTarget
    $adminRule = New-Object System.Security.AccessControl.FileSystemAccessRule(
        "Administrators", "FullControl", "Allow"
    )
    $userRule = New-Object System.Security.AccessControl.FileSystemAccessRule(
        $CloudPassUser, "Read", "Allow"
    )
    $acl.SetAccessRule($adminRule)
    $acl.SetAccessRule($userRule)
    Set-Acl -Path $SSHKeyTarget -AclObject $acl
} else {
    Write-Warn "SSH key not found at $SSHKeySource"
    Write-Warn "Terminal access may not work without SSH key"
    Write-Warn "You can manually copy it later"
}

# Update port in config if not default
if ($Port -ne 8080) {
    Write-Info "Updating port to $Port..."
    $configPath = "$InstallDir\config.yaml"
    $configContent = Get-Content $configPath -Raw
    $configContent = $configContent -replace 'port: 8080', "port: $Port"
    Set-Content -Path $configPath -Value $configContent
}

# Setup multipass authentication
function Setup-MultipassAuth {
    Write-Info "Setting up multipass authentication..."

    try {
        $null = Get-Command multipass -ErrorAction Stop
    } catch {
        Write-Warn "multipass command not found"
        return $false
    }

    try {
        $null = multipass list 2>$null
        Write-Info "Multipass is already accessible without authentication"
        return $true
    } catch {
        Write-Info "Multipass requires authentication"
    }

    # Generate passphrase
    $passphrase = -join ((65..90) + (97..122) + (48..57) | Get-Random -Count 24 | ForEach-Object {[char]$_})

    try {
        $null = multipass set local.passphrase=$passphrase 2>$null
        Write-Info "Passphrase set successfully"
    } catch {
        Write-Warn "Could not set multipass passphrase automatically"
        Write-MultipassAuthGuide
        return $false
    }

    # Store passphrase directly in config.yaml (Windows doesn't support EnvironmentFile)
    $configPath = "$InstallDir\config.yaml"
    if (Test-Path $configPath) {
        $configContent = Get-Content $configPath -Raw
        
        # Add passphrase_env that maps to env var, and also set env for current session
        if ($configContent -notmatch 'passphrase_env:') {
            $configContent = $configContent -replace '(multipass:)', "$1`n  passphrase_env: `"CLOUDPASS_MULTIPASS_PASS`""
        }
        
        Set-Content -Path $configPath -Value $configContent
    }

    # Set environment variable for current user (will apply to service if it runs as that user)
    [Environment]::SetEnvironmentVariable("CLOUDPASS_MULTIPASS_PASS", $passphrase, [EnvironmentVariableTarget]::Machine)

    Write-Info "Multipass authentication configured"
    return $true
}

function Write-MultipassAuthGuide {
    Write-Host ""
    Write-Host "================================================================================" -ForegroundColor Yellow
    Write-Host "WARNING: Could not automatically configure multipass authentication." -ForegroundColor Yellow
    Write-Host "CloudPass may fail to control instances." -ForegroundColor Yellow
    Write-Host ""
    Write-Host "To fix manually:" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "1. As an admin, run:" -ForegroundColor White
    Write-Host "   multipass set local.passphrase=your_secure_password" -ForegroundColor Gray
    Write-Host ""
    Write-Host "2. Create environment variable for CloudPass service:" -ForegroundColor White
    Write-Host "   Run regedit and add to HKLM\SYSTEM\CurrentControlSet\Services\CloudPass\Parameters" -ForegroundColor Gray
    Write-Host "   Name: CLOUDPASS_MULTIPASS_PASS" -ForegroundColor Gray
    Write-Host "   Value: your_secure_password" -ForegroundColor Gray
    Write-Host ""
    Write-Host "3. Update config.yaml multipass section:" -ForegroundColor White
    Write-Host "   passphrase_env: `"CLOUDPASS_MULTIPASS_PASS`"" -ForegroundColor Gray
    Write-Host ""
    Write-Host "4. Restart CloudPass: Restart-Service CloudPass" -ForegroundColor White
    Write-Host "================================================================================" -ForegroundColor Yellow
    Write-Host ""
}

$authResult = Setup-MultipassAuth

# Set ownership
$acl = Get-Acl $InstallDir
$userRule = New-Object System.Security.AccessControl.FileSystemAccessRule(
    $CloudPassUser, "FullControl", "Allow"
)
$acl.SetAccessRule($userRule)
Set-Acl -Path $InstallDir -AclObject $acl

# Create and start service
Write-Info "Creating Windows service..."

$existingService = Get-Service -Name "CloudPass" -ErrorAction SilentlyContinue
if ($existingService) {
    Write-Warn "Service CloudPass already exists, stopping..."
    Stop-Service -Name "CloudPass" -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 1
    sc.exe delete CloudPass 2>$null
    Start-Sleep -Seconds 1
}

$binPath = "$InstallDir\cloudpass.exe"
sc.exe create CloudPass binPath= "$binPath" start= auto DisplayName= "CloudPass" | Out-Null
sc.exe config CloudPass obj= ".\$CloudPassUser" | Out-Null
Start-Service -Name "CloudPass"

Write-Info ""
Write-Info "=========================================="
Write-Info "  CloudPass installed successfully!"
Write-Info "=========================================="
Write-Info ""
Write-Info "Access the UI at: http://localhost:$Port"
Write-Info ""
Write-Info "Commands:"
Write-Info "  Start-Service CloudPass    # Start service"
Write-Info "  Stop-Service CloudPass     # Stop service"
Write-Info "  Get-Service CloudPass      # Check status"
Write-Info "  Get-WinEvent -LogName Application | Where-Object { "`$_.Message -like '*CloudPass*' }  # View logs"
Write-Info ""

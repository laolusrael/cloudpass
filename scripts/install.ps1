# CloudPass Windows Service Installation Script
# Usage: .\install.ps1 [-Port <int>]

param(
    [int]$Port = 8080
)

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

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$InstallDir = "C:\Program Files\CloudPass"
$CloudPassUser = "CloudPass"
$SSHKeySource = "C:\Windows\System32\config\systemprofile\AppData\Roaming\multipassd\ssh-keys\id_rsa"
$SSHKeyDir = "$InstallDir\.cloudpass"
$SSHKeyTarget = "$SSHKeyDir\id_rsa"

Write-Info "Installing CloudPass..."

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
Copy-Item "$ScriptDir\cloudpass.exe" "$InstallDir\" -Force
Copy-Item "$ScriptDir\config.yaml" "$InstallDir\" -Force

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
sc.exe config CloudPass obj= "$env:COMPUTERNAME\$CloudPassUser" | Out-Null
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
Write-Info "  Get-WinEvent -LogName Application | Where-Object { `$_.Message -like '*CloudPass*' }  # View logs"
Write-Info ""

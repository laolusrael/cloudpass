# CloudPass Windows Service Uninstall Script

$ErrorActionPreference = "Stop"

function Write-Info {
    param([string]$Message)
    Write-Host $Message -ForegroundColor Green
}

function Write-Warn {
    param([string]$Message)
    Write-Host $Message -ForegroundColor Yellow
}

function Test-Admin {
    $currentUser = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($currentUser)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

if (-not (Test-Admin)) {
    Write-Host "This script must be run as Administrator" -ForegroundColor Red
    exit 1
}

$InstallDir = "C:\Program Files\CloudPass"
$CloudPassUser = "CloudPass"

Write-Info "Uninstalling CloudPass..."

# Stop and delete service
$service = Get-Service -Name "CloudPass" -ErrorAction SilentlyContinue
if ($service) {
    Write-Info "Stopping CloudPass service..."
    Stop-Service -Name "CloudPass" -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 1
    
    Write-Info "Deleting CloudPass service..."
    sc.exe delete CloudPass 2>$null
}

# Remove installation directory
if (Test-Path $InstallDir) {
    Write-Info "Removing installation directory..."
    Remove-Item -Path $InstallDir -Recurse -Force
}

# Remove user
try {
    $user = Get-LocalUser -Name $CloudPassUser -ErrorAction SilentlyContinue
    if ($user) {
        Write-Info "Removing user $CloudPassUser..."
        Remove-LocalUser -Name $CloudPassUser -ErrorAction SilentlyContinue
    }
} catch {
    # Fallback: try net user
    net user $CloudPassUser /delete 2>$null
}

Write-Info ""
Write-Info "=========================================="
Write-Info "  CloudPass uninstalled successfully!"
Write-Info "=========================================="
Write-Info ""

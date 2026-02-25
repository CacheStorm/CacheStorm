#
# CacheStorm One-Click Setup Script for Windows (PowerShell)
# Usage: irm https://raw.githubusercontent.com/cachestorm/cachestorm/main/scripts/install.ps1 | iex
#

param(
    [Parameter()]
    [ValidateSet("auto", "docker", "binary", "source", "uninstall", "help")]
    [string]$Method = "auto",

    [switch]$Yes
)

# Error handling
$ErrorActionPreference = "Stop"

# Configuration
$Repo = "cachestorm/cachestorm"
$InstallDir = "$env:LOCALAPPDATA\CacheStorm"
$DataDir = "$env:LOCALAPPDATA\CacheStorm\data"
$ConfigDir = "$env:LOCALAPPDATA\CacheStorm\config"
$ServiceName = "CacheStorm"

# Colors
function Write-ColorOutput($ForegroundColor) {
    $fc = $host.UI.RawUI.ForegroundColor
    $host.UI.RawUI.ForegroundColor = $ForegroundColor
    if ($args) {
        Write-Output $args
    }
    $host.UI.RawUI.ForegroundColor = $fc
}

function Write-Success($Message) {
    Write-ColorOutput Green "[✓] $Message"
}

function Write-Info($Message) {
    Write-ColorOutput Cyan "[ℹ] $Message"
}

function Write-Warning($Message) {
    Write-ColorOutput Yellow "[!] $Message"
}

function Write-Error($Message) {
    Write-ColorOutput Red "[✗] $Message"
}

function Write-Banner() {
    Write-ColorOutput Blue @"
╔══════════════════════════════════════════════════════════════╗
║                                                              ║
║                    CacheStorm Installer                      ║
║                                                              ║
║          High-Performance Redis-Compatible Database          ║
║                                                              ║
╚══════════════════════════════════════════════════════════════╝
"@
}

function Test-Command($Command) {
    return [bool](Get-Command -Name $Command -ErrorAction SilentlyContinue)
}

function Get-Architecture() {
    $arch = [System.Environment]::Is64BitOperatingSystem
    $processor = [System.Environment]::GetEnvironmentVariable("PROCESSOR_ARCHITECTURE")

    if ($processor -eq "ARM64") {
        return "arm64"
    }
    return "amd64"
}

function Test-Prerequisites() {
    Write-Info "Checking prerequisites..."

    $script:HasDocker = $false
    $script:HasDockerCompose = $false
    $script:HasGo = $false

    # Check Docker
    try {
        $dockerVersion = docker version --format '{{.Server.Version}}' 2>$null
        if ($dockerVersion) {
            Write-Success "Docker found (v$dockerVersion)"
            $script:HasDocker = $true

            # Check Docker Compose
            try {
                $composeVersion = docker compose version --short 2>$null
                if ($composeVersion) {
                    Write-Success "Docker Compose found (v$composeVersion)"
                    $script:HasDockerCompose = $true
                }
            }
            catch {
                try {
                    $composeVersion = docker-compose version --short 2>$null
                    if ($composeVersion) {
                        Write-Success "Docker Compose found (v$composeVersion)"
                        $script:HasDockerCompose = $true
                    }
                }
                catch {
                    Write-Warning "Docker Compose not found"
                }
            }
        }
    }
    catch {
        Write-Warning "Docker not found"
    }

    # Check Go
    try {
        $goVersion = go version 2>$null
        if ($goVersion) {
            Write-Success "Go found ($goVersion)"
            $script:HasGo = $true
        }
    }
    catch {
        Write-Warning "Go not found"
    }

    Write-Output ""
}

function Install-Docker() {
    Write-Info "Installing CacheStorm with Docker..."

    # Create directories
    New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
    New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null

    # Create default config
    $configPath = "$ConfigDir\cachestorm.yaml"
    if (-not (Test-Path $configPath)) {
        @"
server:
  port: 6379
  http_port: 8080
  bind: 0.0.0.0

storage:
  max_memory: 1gb
  eviction_policy: allkeys-lru

persistence:
  enabled: true
  mode: aof
  aof_fsync: everysec

logging:
  level: info
  format: json
"@ | Set-Content -Path $configPath -Encoding UTF8
        Write-Success "Created default configuration"
    }

    # Create docker-compose.yml
    $composePath = "$InstallDir\docker-compose.yml"
    @"
version: '3.8'

services:
  cachestorm:
    image: ${Repo}:latest
    container_name: cachestorm
    ports:
      - "6379:6379"
      - "8080:8080"
    volumes:
      - ${DataDir}:/data
      - ${ConfigDir}/cachestorm.yaml:/etc/cachestorm/cachestorm.yaml:ro
    environment:
      - CACHESTORM_CONFIG=/etc/cachestorm/cachestorm.yaml
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 3s
      retries: 3
      start_period: 10s

  redisinsight:
    image: redis/redisinsight:latest
    container_name: cachestorm-insight
    ports:
      - "5540:5540"
    volumes:
      - ${DataDir}/redisinsight:/data
    restart: unless-stopped
    profiles:
      - gui
"@ | Set-Content -Path $composePath -Encoding UTF8

    Write-Success "Created docker-compose.yml"

    # Start CacheStorm
    Set-Location $InstallDir
    docker compose up -d

    Write-Output ""
    Write-Success "CacheStorm is running!"
    Write-Output ""
    Write-Info "Redis Protocol: localhost:6379"
    Write-Info "HTTP API:      http://localhost:8080"
    Write-Info "Admin UI:      http://localhost:8080"
    Write-Info "Data Directory: $DataDir"
    Write-Output ""
    Write-Warning "Commands:"
    Write-Output "  docker compose -f '$composePath' logs -f"
    Write-Output "  docker compose -f '$composePath' stop"
    Write-Output "  docker compose -f '$composePath' start"
    Write-Output ""
    Write-Warning "To add GUI (Redis Insight):"
    Write-Output "  docker compose -f '$composePath' --profile gui up -d"
    Write-Output ""
}

function Install-Binary() {
    Write-Info "Installing CacheStorm binary..."

    $arch = Get-Architecture
    $platform = "windows"

    Write-Info "Detected platform: $platform/$arch"

    # Get latest release
    Write-Info "Fetching latest release..."

    try {
        $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
        $asset = $release.assets | Where-Object { $_.name -like "*${platform}_${arch}*.zip" -or $_.name -like "*${platform}*${arch}*.zip" } | Select-Object -First 1

        if (-not $asset) {
            Write-Error "Could not find release for ${platform}/${arch}"
            exit 1
        }

        $downloadUrl = $asset.browser_download_url
        Write-Info "Downloading from: $downloadUrl"

        # Download
        $tempDir = [System.IO.Path]::GetTempPath() + [System.Guid]::NewGuid().ToString()
        New-Item -ItemType Directory -Force -Path $tempDir | Out-Null

        $zipPath = "$tempDir\cachestorm.zip"
        Invoke-WebRequest -Uri $downloadUrl -OutFile $zipPath -UseBasicParsing

        # Extract
        Write-Info "Extracting..."
        Expand-Archive -Path $zipPath -DestinationPath $tempDir -Force

        # Find binary
        $binary = Get-ChildItem -Path $tempDir -Filter "cachestorm.exe" -Recurse | Select-Object -First 1
        if (-not $binary) {
            $binary = Get-ChildItem -Path $tempDir -Filter "*.exe" | Select-Object -First 1
        }

        if (-not $binary) {
            Write-Error "Could not find cachestorm binary in archive"
            exit 1
        }

        # Create directories
        New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
        New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
        New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null

        # Install binary
        Copy-Item -Path $binary.FullName -Destination "$InstallDir\cachestorm.exe" -Force
        Write-Success "Binary installed to $InstallDir\cachestorm.exe"

        # Cleanup
        Remove-Item -Path $tempDir -Recurse -Force

        # Create default config
        $configPath = "$ConfigDir\cachestorm.yaml"
        if (-not (Test-Path $configPath)) {
            @"
server:
  port: 6379
  http_port: 8080
  bind: 0.0.0.0

storage:
  max_memory: 1gb
  eviction_policy: allkeys-lru

persistence:
  enabled: true
  mode: aof
  aof_fsync: everysec

logging:
  level: info
  format: json
"@ | Set-Content -Path $configPath -Encoding UTF8
        }

        # Add to PATH if not already there
        $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
        if ($currentPath -notlike "*$InstallDir*") {
            [Environment]::SetEnvironmentVariable("Path", "$currentPath;$InstallDir", "User")
            Write-Success "Added $InstallDir to PATH (restart terminal to use)"
        }

        # Create startup script
        $startupScript = "$InstallDir\start.bat"
        @"
@echo off
cachestorm.exe --config "$ConfigDir\cachestorm.yaml"
"@ | Set-Content -Path $startupScript -Encoding ASCII

        # Create Windows Service
        Create-WindowsService

        Write-Output ""
        Write-Success "CacheStorm binary installed!"
        Write-Output ""
        Write-Warning "To start CacheStorm:"
        Write-Output "  $startupScript"
        Write-Output "  or: cachestorm.exe --config $ConfigDir\cachestorm.yaml"
        Write-Output ""
        Write-Warning "Windows Service:"
        Write-Output "  Start-Service $ServiceName"
        Write-Output "  Stop-Service $ServiceName"
        Write-Output ""
    }
    catch {
        Write-Error "Failed to install binary: $_"
        exit 1
    }
}

function Create-WindowsService() {
    Write-Info "Creating Windows Service..."

    # Check if service exists
    $service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($service) {
        Write-Warning "Service already exists, removing..."
        Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
        sc.exe delete $ServiceName | Out-Null
        Start-Sleep -Seconds 2
    }

    # Create service using nssm (Non-Sucking Service Manager) or sc
    $binaryPath = "$InstallDir\cachestorm.exe"
    $configPath = "$ConfigDir\cachestorm.yaml"

    try {
        # Try using sc.exe first
        $command = '"' + $binaryPath + '" --config "' + $configPath + '"'
        sc.exe create $ServiceName binPath= $command start= auto DisplayName= "CacheStorm Database" | Out-Null

        if ($?) {
            Write-Success "Created Windows Service '$ServiceName'"
            Write-Warning "Start service with: Start-Service $ServiceName"
        }
    }
    catch {
        Write-Warning "Could not create Windows Service automatically"
        Write-Info "To run as service, consider using NSSM: https://nssm.cc/"
    }
}

function Install-Source() {
    Write-Info "Building CacheStorm from source..."

    if (-not (Test-Command "go")) {
        Write-Error "Go is required to build from source"
        Write-Info "Install Go from: https://golang.org/dl/"
        exit 1
    }

    # Check Go version
    $goVersion = go version
    Write-Info "Found: $goVersion"

    # Clone and build
    $tempDir = [System.IO.Path]::GetTempPath() + [System.Guid]::NewGuid().ToString()
    New-Item -ItemType Directory -Force -Path $tempDir | Out-Null

    try {
        Write-Info "Cloning repository..."
        Set-Location $tempDir
        git clone --depth 1 "https://github.com/$Repo.git"

        Set-Location "$tempDir\cachestorm"

        Write-Info "Building..."
        go build -ldflags="-s -w" -o cachestorm.exe .\cmd\cachestorm

        # Create directories
        New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
        New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
        New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null

        # Install binary
        Copy-Item -Path ".\cachestorm.exe" -Destination "$InstallDir\cachestorm.exe" -Force

        # Create default config
        $configPath = "$ConfigDir\cachestorm.yaml"
        if (-not (Test-Path $configPath)) {
            Copy-Item -Path ".\config\example.yaml" -Destination $configPath -ErrorAction SilentlyContinue
        }

        # Add to PATH
        $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
        if ($currentPath -notlike "*$InstallDir*") {
            [Environment]::SetEnvironmentVariable("Path", "$currentPath;$InstallDir", "User")
        }

        Write-Output ""
        Write-Success "CacheStorm built and installed from source!"
        Write-Output ""
        Write-Warning "To start: cachestorm.exe --config $ConfigDir\cachestorm.yaml"
        Write-Output ""
    }
    finally {
        # Cleanup
        Set-Location $env:USERPROFILE
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

function Uninstall() {
    Write-Info "Uninstalling CacheStorm..."

    # Stop and remove Docker containers
    $composePath = "$InstallDir\docker-compose.yml"
    if (Test-Path $composePath) {
        Write-Info "Stopping Docker containers..."
        Set-Location $InstallDir
        docker compose down 2>$null
        if ($?) {
            Write-Success "Docker containers stopped"
        }
    }

    # Stop Windows Service
    $service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($service) {
        Write-Info "Stopping Windows Service..."
        Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
        sc.exe delete $ServiceName | Out-Null
        Write-Success "Windows Service removed"
    }

    # Remove binary from PATH
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($currentPath -like "*$InstallDir*") {
        $newPath = $currentPath -replace [regex]::Escape(";$InstallDir"), ""
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    }

    Write-Output ""
    Write-Success "CacheStorm uninstalled"
    Write-Output ""
    Write-Warning "Data directory preserved at: $DataDir"
    Write-Warning "To remove data, run: Remove-Item -Recurse -Force '$DataDir'"
    Write-Output ""
}

function Show-Help() {
    Write-Output @"
CacheStorm Installer for Windows

Usage:
  irm https://raw.githubusercontent.com/cachestorm/cachestorm/main/scripts/install.ps1 | iex
  irm .../install.ps1 | iex -Method docker
  irm .../install.ps1 | iex -Method binary
  irm .../install.ps1 | iex -Method source
  irm .../install.ps1 | iex -Method uninstall

Options:
  -Method docker     Install using Docker (recommended)
  -Method binary     Download and install pre-built binary
  -Method source     Build and install from source
  -Method uninstall  Remove CacheStorm
  -Method help       Show this help message
  -Yes               Skip confirmation prompts

Examples:
  # Interactive install (auto-detect best method)
  irm https://raw.githubusercontent.com/cachestorm/cachestorm/main/scripts/install.ps1 | iex

  # Force Docker install
  irm https://raw.githubusercontent.com/cachestorm/cachestorm/main/scripts/install.ps1 | iex -Method docker

  # Uninstall
  irm https://raw.githubusercontent.com/cachestorm/cachestorm/main/scripts/install.ps1 | iex -Method uninstall
"@
}

function Test-Installation() {
    Write-Info "Testing installation..."

    if ($script:Method -eq "docker") {
        Start-Sleep -Seconds 3

        try {
            $container = docker ps --filter "name=cachestorm" --format "{{.Names}}"
            if ($container) {
                Write-Success "CacheStorm container is running"

                # Test HTTP API
                try {
                    $response = Invoke-RestMethod -Uri "http://localhost:8080/health" -TimeoutSec 5
                    Write-Success "HTTP API responding"
                }
                catch {
                    Write-Warning "HTTP API not yet responding (may need more time)"
                }
            }
        }
        catch {
            Write-Warning "Could not verify Docker container status"
        }
    }
    else {
        if (Test-Path "$InstallDir\cachestorm.exe") {
            Write-Success "CacheStorm binary installed"

            try {
                $version = & "$InstallDir\cachestorm.exe" -version 2>$null
                if ($version) {
                    Write-Info "Version: $version"
                }
            }
            catch {
                # Ignore version check errors
            }
        }
    }

    Write-Output ""
}

# Main
Write-Banner

# Handle help
if ($Method -eq "help") {
    Show-Help
    exit 0
}

# Handle uninstall
if ($Method -eq "uninstall") {
    Uninstall
    exit 0
}

# Check prerequisites
Test-Prerequisites

# Auto-detect method
if ($Method -eq "auto") {
    if ($script:HasDocker -and $script:HasDockerCompose) {
        $Method = "docker"
    }
    elseif ($script:HasGo) {
        $Method = "source"
    }
    else {
        $Method = "binary"
    }
    Write-Info "Auto-selected installation method: $Method"
    Write-Output ""
}

$script:Method = $Method

# Confirm installation
if (-not $Yes) {
    Write-Warning "This will install CacheStorm using: $Method"
    $confirm = Read-Host "Press Enter to continue or Ctrl+C to cancel"
}

# Install based on method
try {
    switch ($Method) {
        "docker" { Install-Docker }
        "binary" { Install-Binary }
        "source" { Install-Source }
        default {
            Write-Error "Unknown installation method: $Method"
            Show-Help
            exit 1
        }
    }

    # Test installation
    Test-Installation

    Write-Output ""
    Write-ColorOutput Green "══════════════════════════════════════════════════════════════"
    Write-ColorOutput Green "  Installation Complete!"
    Write-ColorOutput Green "══════════════════════════════════════════════════════════════"
    Write-Output ""
    Write-Info "Documentation: https://github.com/cachestorm/cachestorm"
    Write-Info "Issues:        https://github.com/cachestorm/cachestorm/issues"
    Write-Output ""
}
catch {
    Write-Error "Installation failed: $_"
    exit 1
}

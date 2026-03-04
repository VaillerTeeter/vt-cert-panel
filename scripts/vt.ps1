# File: scripts/vt.ps1 | Purpose: Unified development command entry point | Key Functions: Invoke-EnvCommand, Invoke-InitCommand, Invoke-DebugCommand, Invoke-DockerCommand
# Permissions: Normal user executable (tool install may require elevation) | Applicable Platforms: Windows PowerShell 5.1+ / PowerShell 7+ | Dependencies: PowerShell, winget (optional), Git, Go, Docker

# Script Overview:
# Unified vt-cert-panel development panel management script supporting 4 subcommands:
# 1. env   -- Set environment variables (Technitium URL, Token, ACME email), then execute full initialization (tool install, directory creation, config generation)
# 2. init  -- Detect/install required tools (Git, Go, Docker), create directory structure, generate .env.local config file, execute go mod tidy
# 3. debug -- Start development service (go run ./cmd/server), automatically open browser to http://localhost:8080
# 4. docker -- Build Docker image (project root Dockerfile), can be deployed with docker run

# Execution Flow:
# 1. Parse command line arguments (-Command, -BaseUrl, -Token, etc.)
# 2. Set strict mode (forbid undeclared variables, etc.)
# 3. Dispatch execution based on -Command to different functions
# 4. Functions auto-detect tools (winget install Git, Go, Docker)
# 5. Create data directories (data/, data/certs/, data/acme/, bin/)
# 6. Generate .env.local config file (if not exists)
# 7. Update .env.local environment variable values
# 8. Execute go mod tidy (if Go available), start service or build image

# Parameter: Command | Meaning: Subcommand name | Type: string | Default: empty string | Required: no
# Parameter: BaseUrl | Meaning: Technitium DNS API base URL | Type: string | Default: empty | Required: only for env command
# Parameter: Token | Meaning: Technitium DNS API authentication token | Type: string | Default: empty | Required: only for env command
# Parameter: AcmeEmail | Meaning: ACME account email address | Type: string | Default: empty | Required: only for env command
# Parameter: Version | Meaning: Docker image version number format x.y.z | Type: string | Default: empty | Required: only for docker command
# Parameter: SkipToolInstall | Meaning: whether to skip tool auto-installation | Type: switch | Default: false | Required: no
# Parameter: SkipDocker | Meaning: whether to skip Docker detection and installation | Type: switch | Default: false | Required: no
# Parameter: SkipGoMod | Meaning: whether to skip go mod tidy command | Type: switch | Default: false | Required: no
# Parameter: Help | Meaning: display command help text and exit | Type: switch | Default: false | Required: no

param(
    [Parameter(Position = 0)]
    [string]$Command = "",

    [string]$BaseUrl,
    [string]$Token,
    [string]$AcmeEmail,

    [string]$Version,

    [switch]$SkipToolInstall,
    [switch]$SkipDocker,
    [switch]$SkipGoMod,

    [switch]$Help
)

# Set strict mode: forbid undeclared variables, undefined properties, unregistered functions
Set-StrictMode -Version Latest

# Global error handling strategy: stop immediately when error encountered
$ErrorActionPreference = 'Stop'

# Function: Write-Step | Purpose: Output cyan-colored step information | Input: message string | Output: colored console text | May fail: no
function Write-Step {
    param([string]$Message)
    Write-Host "[vt-cert-panel] $Message" -ForegroundColor Cyan
}

# Function: Write-WarnMsg | Purpose: Output yellow-colored warning information | Input: message string | Output: colored console text | May fail: no
function Write-WarnMsg {
    param([string]$Message)
    Write-Host "[vt-cert-panel] $Message" -ForegroundColor Yellow
}

# Function: Write-Ok | Purpose: Output green-colored success information | Input: message string | Output: colored console text | May fail: no
function Write-Ok {
    param([string]$Message)
    Write-Host "[vt-cert-panel] $Message" -ForegroundColor Green
}

# Function: Write-Err | Purpose: Output red-colored error information | Input: message string | Output: colored console text | May fail: no
function Write-Err {
    param([string]$Message)
    Write-Host "[vt-cert-panel] $Message" -ForegroundColor Red
}

# Function: Show-Help | Purpose: Display script help information and usage examples | Input: none | Output: help text to console | May fail: no
function Show-Help {
    # Variable: lines | Meaning: each line of help information | Type: string[] | Scope: function local | Initial value: help text array
    $lines = @(
        "vt-cert-panel unified script",
        "",
        "Usage:",
        "    powershell -ExecutionPolicy Bypass -File .\scripts\vt.ps1 -Command <command> [options]",
        "",
        "Commands:",
        "    env    Set env vars and run init",
        "    init   Initialize local environment",
        "    debug  Run dev server",
        "    docker Build docker image",
        "",
        "Options:",
        "    -BaseUrl <url>",
        "    -Token <token>",
        "    -AcmeEmail <email>",
        "    -Version <x.y.z>",
        "    -SkipToolInstall",
        "    -SkipDocker",
        "    -SkipGoMod",
        "    -Help"
    )
    # Execute: join array into single string and output to console
    Write-Host ($lines -join "`n")
}

# Function: Test-Tool | Purpose: Detect whether specified tool is installed on system (via Get-Command) | Input: command name | Output: true/false | May fail: no
function Test-Tool {
    param([string]$CommandName)
    # Execute: try to find specified command in system PATH, return true if found, false otherwise
    return [bool](Get-Command $CommandName -ErrorAction SilentlyContinue)
}

# Function: Ensure-WingetPackage | Purpose: Ensure tool is installed via winget (if not yet installed) | Input: tool ID, display name, command name | Output: installation complete or skip log | May fail: winget call failed, network error, permission denied
function Ensure-WingetPackage {
    param(
        [string]$Id,
        [string]$Name,
        [string]$CommandName
    )

    # Condition: if $CommandName is detected to exist, tool is already installed
    if (Test-Tool $CommandName) {
        Write-Ok "$Name is already installed."
        return
    }

    # Condition: if user specified -SkipToolInstall, skip this tool installation
    if ($SkipToolInstall) {
        Write-WarnMsg "$Name is missing and -SkipToolInstall is set."
        return
    }

    # Condition: if winget is not available in system, cannot auto-install
    if (-not (Test-Tool "winget")) {
        Write-WarnMsg "winget is not available, cannot auto-install $Name."
        return
    }

    # Execute: winget install command to install specified package
    Write-Step "Installing $Name via winget..."
    winget install --id $Id -e --accept-package-agreements --accept-source-agreements
}

# Function: Set-EnvFileValue | Purpose: Set or update environment variable key-value pair in .env.local file | Input: file path, key name, value | Output: no direct output, file is modified | May fail: file write permission denied
function Set-EnvFileValue {
    param(
        [string]$FilePath,
        [string]$Key,
        [string]$Value
    )

    # Variable: lines | Meaning: read file content each line as one element | Type: string[] | Scope: function local | Initial value: empty array or file content
    $lines = @()

    # Condition: if file exists, read all content into $lines array
    if (Test-Path $FilePath) {
        $lines = Get-Content -Path $FilePath -Encoding UTF8
    }

    # Variable: found | Meaning: whether the key was found in file | Type: boolean | Scope: function local | Initial value: false
    $found = $false

    # Loop: traverse all lines, find line starting with Key=
    for ($i = 0; $i -lt $lines.Count; $i++) {
        # Condition: if this line starts with Key=
        if ($lines[$i] -match "^$([regex]::Escape($Key))=") {
            # Execute: replace line content with new Key=Value
            $lines[$i] = "$Key=$Value"
            # Execute: mark key found
            $found = $true
            break
        }
    }

    # Condition: if after loop, key was not found
    if (-not $found) {
        # Execute: append Key=Value to array end
        $lines += "$Key=$Value"
    }

    # Variable: utf8NoBom | Meaning: UTF-8 without BOM encoding object | Type: UTF8Encoding | Scope: function local | Initial value: new UTF8Encoding (false)
    $utf8NoBom = New-Object System.Text.UTF8Encoding $false

    # Execute: join $lines array with LF line ending, write to file
    [System.IO.File]::WriteAllText($FilePath, ($lines -join "`n") + "`n", $utf8NoBom)
}

# Function: Validate-RequiredEnv | Purpose: Verify required environment variables are set, throw exception if missing | Input: none | Output: throw exception if validation fails | May fail: any required env var is empty
function Validate-RequiredEnv {
    # Variable: required | Meaning: list of required environment variable names | Type: string[] | Scope: function local | Initial value: 3 required variable names
    $required = @(
        'VT_CERT_TECHNITIUM_BASE_URL',
        'VT_CERT_TECHNITIUM_TOKEN',
        'VT_CERT_ACME_USER_EMAIL'
    )

    # Variable: missing | Meaning: list of missing environment variable names | Type: string[] | Scope: function local | Initial value: empty array
    $missing = @()

    # Loop: traverse each required env var name, check if it is set
    foreach ($name in $required) {
        # Condition: if this env var is empty, not set, or contains only whitespace
        if ([string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable($name))) {
            # Execute: add it to $missing list
            $missing += $name
        }
    }

    # Condition: if any missing variables found
    if ($missing.Count -gt 0) {
        # Execute: throw exception immediately
        throw "Missing required environment variables: $($missing -join ', ')"
    }
}

# Function: Invoke-EnvCommand | Purpose: Set environment variables then automatically execute Invoke-InitCommand | Input: -BaseUrl, -Token, -AcmeEmail required | Output: env vars set prompt + initialization process output | May fail: missing required parameters
function Invoke-EnvCommand {
    # Condition: if any required parameter is empty
    if ([string]::IsNullOrWhiteSpace($BaseUrl) -or [string]::IsNullOrWhiteSpace($Token) -or [string]::IsNullOrWhiteSpace($AcmeEmail)) {
        # Execute: throw exception
        throw "env command requires -BaseUrl, -Token and -AcmeEmail"
    }

    # Execute: set process-level environment variables
    $env:VT_CERT_TECHNITIUM_BASE_URL = $BaseUrl
    $env:VT_CERT_TECHNITIUM_TOKEN = $Token
    $env:VT_CERT_ACME_USER_EMAIL = $AcmeEmail

    # Execute: prompt that env vars are set
    Write-Ok "Environment variables set."

    # Execute: trigger full initialization process
    Invoke-InitCommand
}

# Function: Invoke-InitCommand | Purpose: Initialize Windows development environment detect tools create directories generate config | Input: none, uses global parameters | Output: logs of each initialization operation | May fail: winget install failed, file creation permission denied
function Invoke-InitCommand {
    Write-Step "Initializing Windows development environment..."

    # Variable: projectRoot | Meaning: full path of project root directory | Type: string | Scope: function local | Initial value: parent directory of script
    $projectRoot = Resolve-Path (Join-Path $PSScriptRoot "..")

    # Execute: switch current working directory to project root
    Set-Location $projectRoot

    # Execute: verify required env vars are set
    Validate-RequiredEnv

    # Execute: ensure Git is installed
    Ensure-WingetPackage -Id "Git.Git" -Name "Git" -CommandName "git"

    # Execute: ensure Go is installed
    Ensure-WingetPackage -Id "GoLang.Go" -Name "Go" -CommandName "go"

    # Condition: if user did not specify -SkipDocker
    if (-not $SkipDocker) {
        # Execute: detect and install Docker Desktop
        Ensure-WingetPackage -Id "Docker.DockerDesktop" -Name "Docker Desktop" -CommandName "docker"
    }

    # Variable: dirs | Meaning: list of directories to create | Type: string[] | Scope: function local | Initial value: 4 directory names
    $dirs = @("data", "data/certs", "data/acme", "bin")

    # Loop: traverse each directory, create if not exists
    foreach ($dir in $dirs) {
        # Variable: full | Meaning: full absolute path of directory | Type: string | Scope: loop local | Initial value: projectRoot + dir
        $full = Join-Path $projectRoot $dir

        # Condition: if this directory does not exist
        if (-not (Test-Path $full)) {
            # Execute: create this directory
            New-Item -Path $full -ItemType Directory | Out-Null
        }
    }

    # Variable: envFile | Meaning: full path of .env.local file | Type: string | Scope: function local | Initial value: project root .env.local
    $envFile = Join-Path $projectRoot ".env.local"

    # Condition: if .env.local does not exist
    if (-not (Test-Path $envFile)) {
        # Variable: templatePath | Meaning: path of config file template | Type: string | Scope: block local | Initial value: project root .env.local.template
        $templatePath = Join-Path $projectRoot ".env.local.template"

        # Condition: if template file exists
        if (Test-Path $templatePath) {
            # Execute: copy template directly
            Copy-Item -Path $templatePath -Destination $envFile
        } else {
            # Execute: use hardcoded default config template
            $defaultLines = @(
                "VT_CERT_TECHNITIUM_BASE_URL=",
                "VT_CERT_TECHNITIUM_TOKEN=",
                "VT_CERT_ACME_USER_EMAIL=",
                "VT_CERT_ACME_DIRECTORY_URL=https://acme-v02.api.letsencrypt.org/directory",
                "VT_CERT_TECHNITIUM_DEFAULT_TTL=120",
                "VT_CERT_TECHNITIUM_PROPAGATION_SEC=120",
                "VT_CERT_AUTORENEW_ENABLED=true",
                "VT_CERT_AUTORENEW_CHECK_INTERVAL_HOUR=24",
                "VT_CERT_AUTORENEW_RENEW_BEFORE_DAYS=30"
            )
            # Execute: create UTF-8 without BOM encoding object
            $utf8NoBom = New-Object System.Text.UTF8Encoding $false
            # Execute: write to file
            [System.IO.File]::WriteAllText($envFile, ($defaultLines -join "`n") + "`n", $utf8NoBom)
        }
    }

    # Execute: write current process env var values to .env.local file
    Set-EnvFileValue -FilePath $envFile -Key "VT_CERT_TECHNITIUM_BASE_URL" -Value ([Environment]::GetEnvironmentVariable("VT_CERT_TECHNITIUM_BASE_URL"))
    Set-EnvFileValue -FilePath $envFile -Key "VT_CERT_TECHNITIUM_TOKEN" -Value ([Environment]::GetEnvironmentVariable("VT_CERT_TECHNITIUM_TOKEN"))
    Set-EnvFileValue -FilePath $envFile -Key "VT_CERT_ACME_USER_EMAIL" -Value ([Environment]::GetEnvironmentVariable("VT_CERT_ACME_USER_EMAIL"))

    # Condition: if user did not specify -SkipGoMod
    if (-not $SkipGoMod) {
        # Condition: if Go command is available
        if (Test-Tool "go") {
            # Condition: if GOPROXY environment variable is empty
            if ([string]::IsNullOrWhiteSpace($env:GOPROXY)) {
                # Execute: set to domestic mirror
                $env:GOPROXY = "https://goproxy.cn,direct"
            }

            # Execute: run go mod tidy
            go mod tidy

            # Condition: if go mod tidy returns non-zero exit code
            if ($LASTEXITCODE -ne 0) {
                # Execute: throw exception
                throw "go mod tidy failed (exit code: $LASTEXITCODE)."
            }
        } else {
            # Execute: warn user Go is not available
            Write-WarnMsg "Go is missing, skip go mod tidy."
        }
    }

    # Execute: prompt that initialization is complete
    Write-Ok "Initialization completed."
}

# Function: Invoke-DebugCommand | Purpose: Start development server set log level open browser | Input: none | Output: dev server logs and browser auto-open prompt | May fail: Go compilation failed, port occupied
function Invoke-DebugCommand {
    # Variable: projectRoot | Meaning: project root directory | Type: string | Scope: function local | Initial value: parent directory of script
    $projectRoot = Resolve-Path (Join-Path $PSScriptRoot "..")

    # Execute: switch working directory to project root
    Set-Location $projectRoot

    # Execute: set log level to DEBUG
    $env:VT_CERT_LOG_LEVEL = "debug"

    # Variable: openJob | Meaning: PowerShell job ID for opening browser in background | Type: PSJob | Scope: function local | Initial value: return value of Start-Job
    # Execute: use Start-Job to run background task: wait 2 seconds then open http://localhost:8080 in default browser
    $openJob = Start-Job -ScriptBlock {
        Start-Sleep -Seconds 2
        Start-Process "http://localhost:8080"
    }

    # Execute: try-finally block to ensure background task is cleaned up even if server exits abnormally
    try {
        # Execute: run go run ./cmd/server to start development server
        go run ./cmd/server
    } finally {
        # Execute: regardless of normal or abnormal server exit, run this block
        # Condition: if background task exists
        if ($null -ne $openJob) {
            # Execute: delete background task to clean up resources
            Remove-Job -Id $openJob.Id -Force -ErrorAction SilentlyContinue
        }
    }
}

# Function: Invoke-DockerCommand | Purpose: Build Docker image execute docker build using project root Dockerfile | Input: -Version required format x.y.z | Output: detailed Docker image build log | May fail: version number format error, Docker not installed
function Invoke-DockerCommand {
    # Condition: if Version parameter is empty
    if ([string]::IsNullOrWhiteSpace($Version)) {
        # Execute: throw exception
        throw "Version is required. Example: -Version '1.0.0'"
    }

    # Condition: if Version does not match semantic versioning format (x.y.z)
    if (-not ($Version -match "^[0-9]+\.[0-9]+\.[0-9]+$")) {
        # Execute: throw exception
        throw "Invalid version format. Expected: x.y.z (e.g., 1.0.0)"
    }

    # Variable: projectRoot | Meaning: project root directory | Type: string | Scope: function local | Initial value: parent directory of script
    $projectRoot = Resolve-Path (Join-Path $PSScriptRoot "..")

    # Execute: switch working directory to project root
    Set-Location $projectRoot

    # Execute: docker build command to build image
    # --no-cache skip cache --progress=plain plain text progress -t vt-cert-panel image tag
    docker build --no-cache --progress=plain -t vt-cert-panel:$Version .

    # Condition: if docker build returns non-zero exit code
    if ($LASTEXITCODE -ne 0) {
        # Execute: throw exception
        throw "Docker image build failed"
    }
}

# Condition: if user specified -Help or did not provide any -Command parameter
if ($Help -or [string]::IsNullOrWhiteSpace($Command)) {
    # Execute: display help information
    Show-Help
    # Execute: exit script with success status code 0
    exit 0
}

# Execute: main logic, try to execute user-specified command, catch exception if fails and output error
try {
    # Execute: based on $Command string value execute corresponding function
    switch ($Command.ToLower()) {
        "env" {
            # Execute: env subcommand
            Invoke-EnvCommand
        }
        "init" {
            # Execute: init subcommand
            Invoke-InitCommand
        }
        "debug" {
            # Execute: debug subcommand
            Invoke-DebugCommand
        }
        "docker" {
            # Execute: docker subcommand
            Invoke-DockerCommand
        }
        default {
            # Execute: output error prompt
            Write-Err "Unknown command: $Command"
            Write-Host "Use -Help to see available commands" -ForegroundColor Yellow
            # Execute: exit script with failure status code 1
            exit 1
        }
    }

    # Execute: prompt command executed successfully
    Write-Ok "Command completed successfully"
} catch {
    # Execute: catch any exception error, output error message
    Write-Err "Error: $_"
    # Execute: exit script with failure status code 1
    exit 1
}

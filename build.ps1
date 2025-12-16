<#
.SYNOPSIS
    Build and test lbctl using Docker (no local Go/make required)

.DESCRIPTION
    Runs build, test, and deploy-script validation inside Docker containers.
    Output goes to ./bin/ which is gitignored.

.PARAMETER Command
    test            - Run all tests in container
    build           - Build binary to ./bin/lbctl
    deploy-test     - Test deploy.sh (dry-run)
    deploy-test-full - Test deploy.sh (full install + binary)
    interact        - Interactive shell in deploy environment
    all             - Run test then build (default)

.EXAMPLE
    .\build.ps1 test
    .\build.ps1 build
    .\build.ps1 interact
    .\build.ps1 all
#>

param(
    [Parameter(Position=0)]
    [ValidateSet("test", "build", "deploy-test", "deploy-test-full", "interact", "all")]
    [string]$Command = "all"
)

$ErrorActionPreference = "Stop"

$IMAGE_TEST = "lbctl-test"
$IMAGE_BUILD = "lbctl-build"
$IMAGE_DEPLOY = "lbctl-deploy-test"

function Write-Step($msg) {
    Write-Host "`n=== $msg ===" -ForegroundColor Cyan
}

function Invoke-DockerTest {
    Write-Step "Running tests in AlmaLinux container"
    
    docker build --target lbctl-test -t $IMAGE_TEST .
    if ($LASTEXITCODE -ne 0) { throw "Docker build failed" }
    
    docker run --rm $IMAGE_TEST go test -v ./...
    if ($LASTEXITCODE -ne 0) { throw "Tests failed" }
    
    Write-Host "`nAll tests passed!" -ForegroundColor Green
}

function Invoke-DockerBuild {
    Write-Step "Building binary in container"
    
    docker build --target lbctl-build -t $IMAGE_BUILD .
    if ($LASTEXITCODE -ne 0) { throw "Docker build failed" }
    
    # Create output directory
    if (-not (Test-Path "bin")) {
        New-Item -ItemType Directory -Path "bin" | Out-Null
    }
    
    # Extract binary
    $id = docker create $IMAGE_BUILD
    try {
        docker cp "${id}:/out/lbctl" "./bin/lbctl"
        if ($LASTEXITCODE -ne 0) { throw "Failed to extract binary" }
    } finally {
        docker rm -f $id | Out-Null
    }
    
    Write-Host "`nBinary written to: ./bin/lbctl" -ForegroundColor Green
    Write-Host "Copy to Linux host and run with: ./lbctl --help"
}

function Invoke-DeployTest {
    param([switch]$Full)
    
    $mode = if ($Full) { "full install" } else { "dry-run" }
    Write-Step "Testing deploy.sh ($mode)"
    
    docker build -f scripts/Dockerfile.deploy-test -t $IMAGE_DEPLOY .
    if ($LASTEXITCODE -ne 0) { throw "Docker build failed" }
    
    if ($Full) {
        docker run --privileged --rm $IMAGE_DEPLOY /scripts/deploy.sh --skip-frr-start
    } else {
        docker run --rm $IMAGE_DEPLOY /scripts/deploy.sh --dry-run
    }
    
    if ($LASTEXITCODE -ne 0) { throw "Deploy test failed" }
    
    Write-Host "`nDeploy script test passed!" -ForegroundColor Green
}

function Invoke-Interact {
    Write-Step "Building deploy environment with lbctl binary"
    
    docker build -f scripts/Dockerfile.deploy-test -t $IMAGE_DEPLOY .
    if ($LASTEXITCODE -ne 0) { throw "Docker build failed" }
    
    Write-Host "`nStarting interactive shell..." -ForegroundColor Yellow
    Write-Host "Binary at: /app/lbctl"
    Write-Host "Deploy script: /scripts/deploy.sh --skip-frr-start"
    Write-Host "Exit with: exit`n" -ForegroundColor DarkGray
    
    docker run --privileged -it --rm $IMAGE_DEPLOY /bin/bash
}

# Main
try {
    switch ($Command) {
        "test" {
            Invoke-DockerTest
        }
        "build" {
            Invoke-DockerBuild
        }
        "deploy-test" {
            Invoke-DeployTest
        }
        "deploy-test-full" {
            Invoke-DeployTest -Full
        }
        "interact" {
            Invoke-Interact
        }
        "all" {
            Invoke-DockerTest
            Invoke-DockerBuild
        }
    }
    
    if ($Command -ne "interact") {
        Write-Host "`nDone!" -ForegroundColor Green
    }
} catch {
    Write-Host "`nError: $_" -ForegroundColor Red
    exit 1
}


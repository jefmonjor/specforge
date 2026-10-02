$ErrorActionPreference = "Stop"

Write-Host "==================================================================" -ForegroundColor Cyan
Write-Host "SpecForge v3.0.0 - Multi-Platform Cross-Compiler (Production)" -ForegroundColor Cyan
Write-Host "==================================================================" -ForegroundColor Cyan

$goCmd = if (Get-Command go -ErrorAction SilentlyContinue) { "go" } elseif (Test-Path "C:\Program Files\Go\bin\go.exe") { "C:\Program Files\Go\bin\go.exe" } else { "go" }

$distDir = "dist"
if (!(Test-Path $distDir)) {
    New-Item -ItemType Directory -Path $distDir -Force | Out-Null
}

$targets = @(
    @{ os = "windows"; arch = "amd64"; output = "$distDir/specforge-windows-amd64.exe"; label = "Windows (x86_64)" },
    @{ os = "darwin";  arch = "arm64"; output = "$distDir/specforge-darwin-arm64";     label = "macOS Apple Silicon (M1/M2/M3/M4)" },
    @{ os = "darwin";  arch = "amd64"; output = "$distDir/specforge-darwin-amd64";     label = "macOS Intel (x86_64)" },
    @{ os = "linux";   arch = "amd64"; output = "$distDir/specforge-linux-amd64";      label = "Linux Server / CI (x86_64)" }
)

$env:CGO_ENABLED = "0"

foreach ($t in $targets) {
    $targetLabel = $t.label
    Write-Host "Compilando para $targetLabel..." -ForegroundColor Yellow
    $env:GOOS = $t.os
    $env:GOARCH = $t.arch

    & $goCmd build -ldflags="-s -w" -trimpath -o $t.output .
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Fallo compilando para $targetLabel"
    }
    
    $outPath = $t.output
    $size = (Get-Item $outPath).Length / 1MB
    $sizeStr = "{0:N2} MB" -f $size
    Write-Host "  [OK] Generado: $outPath ($sizeStr)" -ForegroundColor Green
}

# Restaurar entorno
$env:GOOS = "windows"
$env:GOARCH = "amd64"

Write-Host "==================================================================" -ForegroundColor Green
Write-Host "Todos los binarios fueron generados exitosamente en ./dist/" -ForegroundColor Green
Write-Host "==================================================================" -ForegroundColor Green

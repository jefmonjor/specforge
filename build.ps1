$ErrorActionPreference = "Stop"
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8

Write-Host "Compilando SpecForge v3.0.0 (Produccion)..." -ForegroundColor Cyan

$goCmd = if (Get-Command go -ErrorAction SilentlyContinue) { "go" } elseif (Test-Path "C:\Program Files\Go\bin\go.exe") { "C:\Program Files\Go\bin\go.exe" } else { "go" }

# -s: Omitir tabla de símbolos
# -w: Omitir información de DWARF debugging
$env:GOOS = "windows"
$env:GOARCH = "amd64"
& $goCmd build -ldflags="-s -w" -trimpath -o specforge.exe .

Write-Host "Build completado. Tamano del binario:" -ForegroundColor Green
(Get-Item .\specforge.exe).Length / 1MB | ForEach-Object { "{0:N2} MB" -f $_ }

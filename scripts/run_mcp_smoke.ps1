# No-flash MCP smoke on Windows.
# - Rebuilds smartcontext.exe as WINDOWS_GUI
# - Builds mcp_smoke.exe (console parent — output stays in THIS terminal)
# - Child smartcontext.exe is started with CREATE_NO_WINDOW + HideWindow
#
# Usage (from go/ or any cwd):
#   .\scripts\run_mcp_smoke.ps1

$ErrorActionPreference = 'Stop'
$goRoot = Split-Path -Parent $PSScriptRoot
Set-Location -Path $goRoot

Write-Host "Building smartcontext.exe (WINDOWS_GUI)..."
go build -ldflags="-H windowsgui" -o smartcontext.exe ./cmd/smartcontext
if ($LASTEXITCODE -ne 0) { throw "go build smartcontext.exe failed (exit $LASTEXITCODE)" }

$bytes = [System.IO.File]::ReadAllBytes((Join-Path $goRoot 'smartcontext.exe'))
$pe = [BitConverter]::ToInt32($bytes, 0x3C)
$sub = [BitConverter]::ToUInt16($bytes, $pe + 24 + 68)
if ($sub -ne 2) {
    throw "smartcontext.exe subsystem=$sub (expected 2 WINDOWS_GUI). Rebuild failed?"
}
Write-Host "PE subsystem=WINDOWS_GUI (2)"

Write-Host "Building mcp_smoke.exe..."
go build -o mcp_smoke.exe .\scripts\mcp_smoke.go
if ($LASTEXITCODE -ne 0) { throw "go build mcp_smoke.exe failed (exit $LASTEXITCODE)" }

if (-not $env:SMARTCONTEXT_WORKSPACE_ROOTS) {
    $env:SMARTCONTEXT_WORKSPACE_ROOTS = (Resolve-Path (Join-Path $goRoot '..')).Path
}

Write-Host "Running smoke (child uses CREATE_NO_WINDOW + HideWindow)..."
& .\mcp_smoke.exe
$code = $LASTEXITCODE
Remove-Item -Force .\mcp_smoke.exe -ErrorAction SilentlyContinue
exit $code

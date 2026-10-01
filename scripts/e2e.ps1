# OnDex server oqimlarining TO'LIQ avtomatik sinovi (E2E) + hisobot.
#
#   .\scripts\e2e.ps1               # ssenariylar + invariantlar + qamrov ratcheti
#   .\scripts\e2e.ps1 -Open         # tugagach HTML hisobotni ochadi
#   .\scripts\e2e.ps1 -Race         # poyga detektori bilan (Docker, CI bilan bir xil)
#   .\scripts\e2e.ps1 -Mutation     # suite sezgirligi: production'ga ataylab xato kiritib ushlashini tekshiradi
#   .\scripts\e2e.ps1 -SlowMs 150   # sekin deb belgilash chegarasi (standart 250)
#
# Natija: logs\e2e\report.json va report.html
# Yangi ssenariy yozish: internal\e2e\README.md

param(
    [switch]$Open,
    [switch]$Race,
    [switch]$Mutation,
    [int]$SlowMs = 250
)

$ErrorActionPreference = 'Continue'
Set-Location (Split-Path $PSScriptRoot -Parent)
$env:E2E_SLOW_MS = "$SlowMs"
$report = Join-Path (Get-Location) 'logs\e2e\report.json'

if ($Race) {
    # Windows'da gcc yo'q -> `-race` faqat Docker orqali (CI bilan bir xil).
    docker run --rm -v "${PWD}:/src" -w /src -e E2E_SLOW_MS="$SlowMs" golang:1.26 go test -race -count=1 ./internal/e2e/...
} else {
    go test -count=1 ./internal/e2e/...
}
$code = $LASTEXITCODE

if (Test-Path $report) {
    $r = Get-Content $report -Raw | ConvertFrom-Json
    $passed = @($r.steps | Where-Object { $_.passed }).Count
    $failed = @($r.steps | Where-Object { -not $_.passed -and -not $_.skipped }).Count
    Write-Host ""
    Write-Host "Qadamlar: $passed o'tdi, $failed yiqildi | So'rovlar: $($r.calls.Count) | Invariant buzilishi: $($r.violations.Count)"
    Write-Host "Endpoint qamrovi: $($r.coverage.covered)/$($r.coverage.total) (istisno: $($r.coverage.exempt))"
    if ($r.duplicates.Count -gt 0) { Write-Host "Takroriy so'rovlar: $($r.duplicates.Count) ta joyda" -ForegroundColor Yellow }
    Write-Host "Eng sekin 5 endpoint (p95, ms):"
    $r.routes | Select-Object -First 5 | ForEach-Object { Write-Host ("  {0,8:N1}  {1}" -f $_.p95_ms, $_.route) }
    $html = [IO.Path]::ChangeExtension($report, '.html')
    Write-Host "Hisobot: $html"
    if ($Open -and (Test-Path $html)) { Start-Process $html }
}

if ($Mutation) {
    Write-Host ""
    & (Join-Path $PSScriptRoot 'e2e-mutation.ps1')
    if ($LASTEXITCODE -ne 0) { $code = 1 }
}
exit $code

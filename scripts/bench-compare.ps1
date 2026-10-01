# Ikki benchmark natijasini solishtiradi (har bir takrorlash o'rtacha qilinadi).
#   .\scripts\bench-compare.ps1 before after

param(
    [Parameter(Mandatory = $true)][string]$Before,
    [Parameter(Mandatory = $true)][string]$After
)
Set-Location (Split-Path $PSScriptRoot -Parent)

function Load-Runs($label) {
    $files = Get-ChildItem "logs\load\$label-*.json" -ErrorAction SilentlyContinue
    if (-not $files) { throw "logs\load\$label-*.json topilmadi" }
    $files | ForEach-Object { Get-Content $_.FullName -Raw | ConvertFrom-Json }
}
function Avg($vals) { if (-not $vals) { 0 } else { [math]::Round(($vals | Measure-Object -Average).Average, 1) } }

$b = Load-Runs $Before
$a = Load-Runs $After

Write-Host "`nBOSQICH BO'YICHA (o'rtacha $($b.Count) va $($a.Count) takrorlash)" -ForegroundColor Cyan
Write-Host ("{0,10} | {1,18} | {2,18} | {3,18} | {4,16}" -f 'maqsad rps', 'p50 ms (oldin>keyin)', 'p95 ms (oldin>keyin)', 'p99 ms (oldin>keyin)', 'erishilgan rps')
$steps = $b[0].rows | ForEach-Object { $_.target_rps }
foreach ($s in $steps) {
    $rb = $b | ForEach-Object { $_.rows | Where-Object { $_.target_rps -eq $s } }
    $ra = $a | ForEach-Object { $_.rows | Where-Object { $_.target_rps -eq $s } }
    if (-not $rb -or -not $ra) { continue }
    "{0,10} | {1,8} > {2,-8} | {3,8} > {4,-8} | {5,8} > {6,-8} | {7,6} > {8,-6}" -f $s,
        (Avg $rb.p50), (Avg $ra.p50), (Avg $rb.p95), (Avg $ra.p95), (Avg $rb.p99), (Avg $ra.p99),
        (Avg $rb.achieved_rps), (Avg $ra.achieved_rps)
}

Write-Host "`nENDPOINT BO'YICHA (butun test, p50 / p95 ms)" -ForegroundColor Cyan
Write-Host ("{0,-13} | {1,20} | {2,20}" -f 'endpoint', 'oldin p50 / p95', 'keyin p50 / p95')
$names = $b[0].endpoints | ForEach-Object { $_.endpoint }
foreach ($n in $names) {
    $eb = $b | ForEach-Object { $_.endpoints | Where-Object { $_.endpoint -eq $n } }
    $ea = $a | ForEach-Object { $_.endpoints | Where-Object { $_.endpoint -eq $n } }
    "{0,-13} | {1,9} / {2,-8} | {3,9} / {4,-8}" -f $n, (Avg $eb.p50), (Avg $eb.p95), (Avg $ea.p50), (Avg $ea.p95)
}
Write-Host ("`nTashlab yuborilgan iteratsiyalar (server ulgurmadi): oldin {0}, keyin {1}" -f (Avg ($b | ForEach-Object { $_.dropped_iterations })), (Avg ($a | ForEach-Object { $_.dropped_iterations })))

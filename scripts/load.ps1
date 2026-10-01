# OnDex sig'im (capacity) yuklama testi: k6 + resurs o'lchash + hisobot.
#
#   .\scripts\load.ps1                          # standart bosqichlar: 200,500,1000,2000,3000 so'rov/s
#   .\scripts\load.ps1 -Steps 100,300 -StageS 15   # qisqa tutun sinovi
#   .\scripts\load.ps1 -Steps 1000,2000,4000,6000 -MaxVus 8000
#
# ! FAQAT LOKAL (yoki staging). Begona manzil rad etiladi: production VPS bir
# nechta loyihaga umumiy, yuklama boshqa loyihalarni ham yiqitadi.
#
# Natija: logs\load\k6-summary.json, logs\load\resources.csv, konsolda jadval.

param(
    [string]$Api = 'http://localhost:8080',
    [string]$Steps = '200,500,1000,2000,3000',   # vergul bilan: "100,300,1000"
    [int]$StageS = 30,
    [int]$MaxVus = 6000,
    [switch]$AllowRemote,
    [int]$ApiPid = 0,           # API jarayoni PID (berilmasa nomi `api`)
    [string]$SummaryOut = '',
    [string]$K6Script = 'tests\load\capacity.js'   # k6 skripti
)

$ErrorActionPreference = 'Continue'
Set-Location (Split-Path $PSScriptRoot -Parent)

$uri = [Uri]$Api
$local = @('localhost', '127.0.0.1', '::1') -contains $uri.Host
if (-not $local -and -not $AllowRemote) {
    Write-Host "RAD ETILDI: $($uri.Host) lokal emas. Production'ga yuklama yubormang. (Staging uchun -AllowRemote)" -ForegroundColor Red
    exit 2
}
try { $null = Invoke-WebRequest -Uri "$Api/restaurants" -UseBasicParsing -TimeoutSec 5 }
catch { Write-Host "Server javob bermayapti: $Api - avval API'ni ishga tushiring." -ForegroundColor Red; exit 2 }

New-Item -ItemType Directory -Force -Path logs\load | Out-Null
# Resurs fayli har yugurish uchun alohida (oldingi yugurish yoki boshqa dastur
# faylni ochib turgan bo'lsa ham ishlaydi).
$csvName = if ($SummaryOut) { 'resources-' + [IO.Path]::GetFileNameWithoutExtension($SummaryOut) + '.csv' } else { 'resources.csv' }
$csv = Join-Path (Get-Location) "logs\load\$csvName"
'time,api_cpu_pct,k6_cpu_pct,docker_cpu_pct,system_cpu_pct,free_ram_mb,api_ws_mb,k6_ws_mb,tcp_established' | Set-Content $csv -Encoding ascii

# Resurs namunalari: har 2 soniyada (fon ishi).
$sampler = Start-Job -ArgumentList $csv, $ApiPid -ScriptBlock {
    param($csv, $apiPid)
    $cores = [Environment]::ProcessorCount
    function Cpu($names) {
        $p = Get-Process -Name $names -ErrorAction SilentlyContinue
        if (-not $p) { return @{ t = 0; ws = 0 } }
        @{ t = ($p | Measure-Object -Property CPU -Sum).Sum; ws = [math]::Round((($p | Measure-Object -Property WorkingSet64 -Sum).Sum) / 1MB) }
    }
    $prev = @{}
    $last = Get-Date
    while ($true) {
        Start-Sleep -Seconds 2
        $now = Get-Date
        $dt = ($now - $last).TotalSeconds; $last = $now
        $api = if ($apiPid -gt 0) { $pp = Get-Process -Id $apiPid -ErrorAction SilentlyContinue; if ($pp) { @{ t = $pp.CPU; ws = [math]::Round($pp.WorkingSet64 / 1MB) } } else { @{ t = 0; ws = 0 } } } else { Cpu @('api') }; $k6 = Cpu @('k6'); $dk = Cpu @('com.docker.backend', 'vmmem', 'vmmemWSL', 'Docker Desktop')
        $row = @()
        foreach ($pair in @(@('api', $api), @('k6', $k6), @('dk', $dk))) {
            $k = $pair[0]; $cur = $pair[1].t
            $pct = if ($prev.ContainsKey($k) -and $dt -gt 0) { [math]::Round((($cur - $prev[$k]) / $dt) / $cores * 100, 1) } else { 0 }
            $prev[$k] = $cur
            $row += [math]::Max($pct, 0)
        }
        $sys = [math]::Round((Get-Counter '\Processor(_Total)\% Processor Time' -ErrorAction SilentlyContinue).CounterSamples[0].CookedValue, 1)
        $freeRam = [math]::Round((Get-CimInstance Win32_OperatingSystem).FreePhysicalMemory / 1024)
        $tcp = (Get-NetTCPConnection -State Established -ErrorAction SilentlyContinue | Measure-Object).Count
        "$($now.ToString('HH:mm:ss')),$($row[0]),$($row[1]),$($row[2]),$sys,$freeRam,$($api.ws),$($k6.ws),$tcp" | Add-Content $csv -Encoding ascii
    }
}

$env:ONDEX_API = $Api
$stepList = $Steps.Split(',') | ForEach-Object { [int]$_.Trim() }
$env:STEPS = ($stepList -join ',')
$env:STAGE_S = "$StageS"
$env:MAX_VUS = "$MaxVus"
if ($SummaryOut) { $env:SUMMARY_OUT = $SummaryOut }
Write-Host "Yuklama: $($stepList -join ', ') so'rov/s, har bosqich $StageS s (+10 s ko'tarilish), maksimal VU $MaxVus" -ForegroundColor Cyan
k6 run --quiet $K6Script
$code = $LASTEXITCODE

Stop-Job $sampler -ErrorAction SilentlyContinue; Remove-Job $sampler -Force -ErrorAction SilentlyContinue

if (Test-Path $csv) {
    $r = Import-Csv $csv
    if ($r.Count -gt 0) {
        $max = { param($c) ($r | ForEach-Object { [double]$_.$c } | Measure-Object -Maximum).Maximum }
        $min = { param($c) ($r | ForEach-Object { [double]$_.$c } | Measure-Object -Minimum).Minimum }
        Write-Host "`nResurslar (cho'qqi qiymatlar):" -ForegroundColor Cyan
        Write-Host ("  API CPU (barcha yadro ulushi): {0}%   k6 CPU: {1}%   Docker CPU: {2}%   Tizim CPU: {3}%" -f (& $max 'api_cpu_pct'), (& $max 'k6_cpu_pct'), (& $max 'docker_cpu_pct'), (& $max 'system_cpu_pct'))
        Write-Host ("  API RAM: {0} MB   k6 RAM: {1} MB   Eng kam bo'sh RAM: {2} MB   Eng ko'p TCP ulanish: {3}" -f (& $max 'api_ws_mb'), (& $max 'k6_ws_mb'), (& $min 'free_ram_mb'), (& $max 'tcp_established'))
    }
}
Write-Host "`nHisobot: logs\load\k6-summary.json, logs\load\resources.csv"
exit $code

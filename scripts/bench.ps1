# IZOLYATSIYALANGAN benchmark: oldin/keyin solishtirish uchun.
#
#   .\scripts\bench.ps1 -Label before            # joriy kodni o'lchaydi
#   (kodni o'zgartiring)
#   .\scripts\bench.ps1 -Label after
#   .\scripts\bench-compare.ps1 before after     # farq jadvali
#
# Nega alohida muhit: dev serveringizdagi (8080) bazalar va tashqi xizmatlar
# (Telegram bot, SMTP, Firebase...) tegilmasligi shart - ikkinchi nusxa bir xil
# Telegram tokeni bilan polling qilsa 409 beradi, takroriy xabarlar ketadi.
#   * alohida Postgres/Mongo/Redis konteynerlari (portlar 15432, 27118, 16380);
#   * API `.env` YO'Q bo'sh papkada ishga tushadi - tashqi kalitlar yuklanmaydi;
#   * port 18081 (`-ldflags -X main.listenAddr`).
#
# Natija: logs\load\<label>-N.json (N - takrorlash), resurslar logs\load\resources.csv

param(
    [Parameter(Mandatory = $true)][string]$Label,
    [string]$Steps = '',               # bo'sh bo'lsa skriptga mos standart
    [ValidateSet('capacity', 'orders', 'ws')][string]$Script = 'capacity',
    [int]$Seed = 0,                    # orders uchun: oldindan yaratiladigan mijozlar soni (standart 2000)
    [int]$StageS = 30,
    [int]$MaxVus = 4000,
    [int]$Repeat = 2,
    [switch]$TearDown   # tugagach konteynerlarni o'chiradi
)

$ErrorActionPreference = 'Continue'
Set-Location (Split-Path $PSScriptRoot -Parent)
$root = (Get-Location).Path
$apiAddr = 'http://localhost:18081'

function Ensure-Container($name, $image, $port, $inner, $envs) {
    $running = docker ps --format '{{.Names}}' | Where-Object { $_ -eq $name }
    if ($running) { return }
    docker rm -f $name 2>$null | Out-Null
    $args = @('run', '-d', '--name', $name, '-p', "127.0.0.1:${port}:${inner}") + $envs + @($image)
    docker @args | Out-Null
}

Write-Host "Benchmark muhiti ko'tarilmoqda (alohida konteynerlar)..." -ForegroundColor Cyan
Ensure-Container 'bench-pg' 'postgis/postgis:16-3.4' 15432 5432 @('-e', 'POSTGRES_USER=chust', '-e', 'POSTGRES_PASSWORD=bench', '-e', 'POSTGRES_DB=bench')
Ensure-Container 'bench-mongo' 'mongo:7' 27118 27017 @()
Ensure-Container 'bench-redis' 'redis:7-alpine' 16380 6379 @()

$ready = $false
for ($i = 0; $i -lt 60; $i++) {
    docker exec bench-pg pg_isready -U chust -d bench 2>$null | Out-Null
    if ($LASTEXITCODE -eq 0) { $ready = $true; break }
    Start-Sleep -Seconds 1
}
if (-not $ready) { Write-Host 'Postgres tayyor bo''lmadi.' -ForegroundColor Red; exit 2 }

$exeDir = Join-Path $root "build\bench\$Label"
New-Item -ItemType Directory -Force -Path $exeDir | Out-Null
Write-Host "Joriy kod yig'ilmoqda ($Label)..." -ForegroundColor Cyan
go build -ldflags "-X main.listenAddr=:18081" -o "$exeDir\api.exe" ./cmd/api
if ($LASTEXITCODE -ne 0) { Write-Host 'Yig''ib bo''lmadi.' -ForegroundColor Red; exit 2 }

$workDir = Join-Path $root "logs\bench\run-$Label"
New-Item -ItemType Directory -Force -Path $workDir | Out-Null   # `.env` YO'Q: tashqi kalitlar yuklanmaydi

$env:APP_ENV = 'development'
$env:DATABASE_URL = 'postgres://chust:bench@127.0.0.1:15432/bench?sslmode=disable'
$env:MONGODB_URI = 'mongodb://127.0.0.1:27118'
$env:MONGODB_DB = 'bench'
$env:REDIS_ADDR = '127.0.0.1:16380'
$env:JWT_SECRET = 'bench-secret-bench-secret-bench-secret-123456'
$env:PUBLIC_BASE_URL = $apiAddr

if (($Script -eq 'orders' -or $Script -eq 'ws') -and $Seed -le 0) { $Seed = 2000 }
if ($Script -eq 'ws' -and -not $Steps) { $Steps = '10000' }   # ws: ulanishlar soni
if (-not $Steps) { $Steps = if ($Script -eq 'orders') { '25,50,100,200,400' } else { '500,1000,2000,3000' } }
$k6File = if ($Script -eq 'orders') { 'tests\load\orders.js' } else { 'tests\load\capacity.js' }
$tokensFile = Join-Path $root 'logs\bench\tokens.json'
$code = 0
for ($run = 1; $run -le $Repeat; $run++) {
    # Har takrorlashda API yangidan: issiq kesh va xotira holati bir xil bo'lsin.
    $proc = Start-Process -FilePath "$exeDir\api.exe" -WorkingDirectory $workDir -PassThru -WindowStyle Hidden `
        -RedirectStandardOutput "$workDir\out-$run.log" -RedirectStandardError "$workDir\err-$run.log"
    $up = $false
    for ($i = 0; $i -lt 90; $i++) {
        try { $r = Invoke-WebRequest -Uri "$apiAddr/restaurants" -UseBasicParsing -TimeoutSec 3; if ($r.StatusCode -eq 200) { $up = $true; break } } catch {}
        if ($proc.HasExited) { break }
        Start-Sleep -Seconds 1
    }
    if (-not $up) {
        Write-Host "API ko'tarilmadi (PID $($proc.Id)). Jurnal: $workDir\err-$run.log" -ForegroundColor Red
        Get-Content "$workDir\err-$run.log" -Tail 8
        if (-not $proc.HasExited) { Stop-Process -Id $proc.Id -Force }
        $code = 2; break
    }
    if ($Seed -gt 0) {
        Write-Host "Test mijozlar yaratilmoqda ($Seed)..." -ForegroundColor Cyan
        go run ./cmd/benchseed -db $env:DATABASE_URL -secret $env:JWT_SECRET -n $Seed -out $tokensFile
        if ($LASTEXITCODE -ne 0) {
            Write-Host 'Test mijozlar yaratilmadi - benchmark to''xtatildi.' -ForegroundColor Red
            if (-not $proc.HasExited) { Stop-Process -Id $proc.Id -Force }
            $code = 2; break
        }
        $env:TOKENS_FILE = $tokensFile
    }
    Write-Host "`n[$Label] takrorlash $run/$Repeat (API PID $($proc.Id))" -ForegroundColor Cyan
    if ($Script -eq 'ws') {
        # WebSocket: N ta ulanishni ochib ushlab turamiz, API xotirasi/CPU'sini o'lchaymiz.
        $conns = [int]$Steps
        $wsExe = Join-Path $exeDir 'wsload.exe'
        go build -o $wsExe ./cmd/wsload
        $before = Get-Process -Id $proc.Id
        Write-Host ("Ulanishdan oldin API: {0} MB, oqimlar {1}" -f [math]::Round($before.WorkingSet64 / 1MB), $before.Threads.Count)
        $wsOut = "$workDir\ws-$run.log"
        $wp = Start-Process -FilePath $wsExe -PassThru -WindowStyle Hidden -RedirectStandardOutput $wsOut `
            -ArgumentList @('-url', 'ws://localhost:18081/ws', '-tokens', $tokensFile, '-n', $conns, '-rate', '500', '-hold', '30s')
        $held = $false
        for ($i = 0; $i -lt 600 -and -not $wp.HasExited; $i++) {
            Start-Sleep -Seconds 1
            if ((Test-Path $wsOut) -and (Select-String -Path $wsOut -Pattern 'HOLD_START' -Quiet)) { $held = $true; break }
        }
        if ($held) {
            Start-Sleep -Seconds 10
            $cpu0 = (Get-Process -Id $proc.Id).CPU; Start-Sleep -Seconds 10; $now = Get-Process -Id $proc.Id
            Write-Host ("USHLAB TURISHDA API: {0} MB, CPU {1}% (bitta yadro ulushi), oqimlar {2}" -f [math]::Round($now.WorkingSet64 / 1MB), [math]::Round(($now.CPU - $cpu0) / 10 * 100, 1), $now.Threads.Count)
        }
        $wp.WaitForExit()
        Get-Content $wsOut
        if (-not $proc.HasExited) { $after = Get-Process -Id $proc.Id; Write-Host ("Uzilgandan keyin API: {0} MB" -f [math]::Round($after.WorkingSet64 / 1MB)) }
        if ($wp.ExitCode -ne 0) { $code = $wp.ExitCode }
        if (-not $proc.HasExited) { Stop-Process -Id $proc.Id -Force }
        continue
    }
    & (Join-Path $PSScriptRoot 'load.ps1') -Api $apiAddr -Steps $Steps -StageS $StageS -MaxVus $MaxVus `
        -ApiPid $proc.Id -SummaryOut "logs/load/$Label-$run.json" -K6Script $k6File
    if ($LASTEXITCODE -ne 0) { $code = $LASTEXITCODE }
    if (-not $proc.HasExited) { Stop-Process -Id $proc.Id -Force }
    Start-Sleep -Seconds 3
}

if ($TearDown) { docker rm -f bench-pg bench-mongo bench-redis | Out-Null }
exit $code

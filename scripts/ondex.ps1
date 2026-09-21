<#
    OnDex — butun stekni BITTA buyruq bilan ishga tushiradi.

        ondex run          — hammasi (infra + API + web + tunnel + 2 panel + 3 mobil)
        ondex run -Only api,web,tunnel
        ondex run -Skip customer,courier,waiter
        ondex run -Local   — tunnelsiz, localhost/LAN IP rejimi
        ondex stop         — ishga tushirilgan hamma narsani to'xtatadi
        ondex status       — nima ishlab turganini ko'rsatadi
        ondex ip           — Wi-Fi IP ni topib dev.json larga yozadi
        ondex build -Only OnDex        — mijoz, PRODUCTION, telefon ulangan bo'lsa o'rnatiladi
        ondex build -Only OnDexPro     — affitsiant, PRODUCTION
        ondex build -Only OnDexGO      — kuryer, PRODUCTION
        ondex build -Only OnDexAdmin   — admin paneli, PRODUCTION (Windows .exe)
        ondex build -Only Merchant     — restoran paneli, PRODUCTION (Windows .exe)
        ondex build -Only OnDexDev, OnDexProDev, OnDexGoDev, OnDexAdminDev, MerchantDev — DEV versiyalari
        ondex help

    ┌─ NEGA TUNNEL REJIMI STANDART ───────────────────────────────────────┐
    Loyihada ikki dev rejimi bor: `config/dev.json` (localhost + LAN IP)
    va `config/dev-tunnel.json` (HTTPS domenlar).

    Standart qilib TUNNEL tanlandi, chunki:
      * `apps/courier_app` da `dev.json` UMUMAN YO'Q — faqat
        `dev-tunnel.json`. Ya'ni beshta Flutter ilovaning hammasi
        qo'llaydigan yagona rejim shu.
      * `apps/web/.env.local` dagi NEXT_PUBLIC_* qiymatlari allaqachon
        `dev-*-ondex.shoxpro.uz` ga qaratilgan.
      * `next.config.ts` dagi `allowedDevOrigins` ro'yxatida LAN IP YO'Q,
        `dev-web-ondex.shoxpro.uz` bor — LAN IP orqali ochilgan sahifada
        React hydration bloklanadi (tugmalar o'lik bo'ladi).
      * Wi-Fi IP o'zgarishi hech narsani buzmaydi.

    `-Local` bayrog'i localhost rejimiga o'tkazadi, lekin unda kuryer
    ilovasi konfiguratsiyasiz qoladi va o'tkazib yuboriladi.
    └─────────────────────────────────────────────────────────────────────┘

    ┌─ NEGA HAR BIRI ALOHIDA OYNADA ──────────────────────────────────────┐
    `flutter run` va `next dev` hech qachon tugamaydi va interaktiv
    (`r` — hot reload, `q` — chiqish). Ularni bitta oynada fon jarayoni
    qilib yuborilsa klaviatura kiritish yo'qoladi va loglar aralashib
    ketadi. Shuning uchun har biri o'z PowerShell oynasida, sarlavhasi
    "OnDex :: <nom>" bo'lib ochiladi.
    └─────────────────────────────────────────────────────────────────────┘

    ┌─ NEGA MOBIL ILOVALAR NAVBAT BILAN ──────────────────────────────────┐
    Uchta Gradle build'i bir vaqtda ishga tushsa 16 GB RAM va F: diskdagi
    ~19 GB bo'sh joy yetmaydi (release build'lar allaqachon bir marta
    diskni to'ldirgan). `-Stagger` soniya oralig'ida ketma-ket ochiladi.
    └─────────────────────────────────────────────────────────────────────┘
#>

#requires -Version 5.1
[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [ValidateSet('run', 'stop', 'status', 'ip', 'backup', 'build', 'version', 'help')]
    [string]$Command = 'run',

    # `ondex -LastVersion` — $Command ni butunlay chetlab o'tadi (pastdagi
    # dispatch'ga qarang), shuning uchun `ondex -LastVersion` YOLG'IZ ham
    # ishlaydi (Position 0 bo'sh qolsa $Command standart 'run' bo'lib
    # qoladi, lekin bu bayroq shuni ustidan bosib o'tadi).
    [switch]$LastVersion,

    # `ondex -LastVersionDev` — `-LastVersion` bilan bir xil, lekin
    # PROD/DEV orasidan "qaysi kech" emas, HAR DOIM DEV ko'rsatiladi.
    [switch]$LastVersionDev,

    # `ondex backup` — nechta oxirgi TO'LIQ nusxa saqlansin
    [int]$Keep = 7,

    # `ondex run` infra ko'targanda avtomatik zaxira OLINMASIN
    [switch]$NoBackup,

    # Avtomatik zaxiralar orasidagi eng kam vaqt (soat). 0 — har safar.
    [int]$BackupEvery = 6,

    # Faqat shu komponentlar (kalitlar: infra api web tunnel admin restaurant customer courier waiter)
    [string[]]$Only,

    # Shu komponentlardan tashqari hammasi
    [string[]]$Skip,

    # Tunnel o'rniga localhost/LAN IP rejimi
    [switch]$Local,

    # Mobil ilovalar orasidagi kutish (soniya). 0 — barchasi birdan.
    [int]$Stagger = 45,

    # Flutter ilovalarni ishga tushirishdan oldin `flutter pub get`
    [switch]$PubGet
)

$ErrorActionPreference = 'Stop'

# `ondex.cmd` orqali kelganda `-Only api,web` BITTA satr bo'lib biriktiriladi
# (cmd vergulni ajratmaydi). Ikkala shaklni ham qo'llash uchun qo'lda bo'lamiz.
if ($Only) { $Only = @($Only -split ',' | ForEach-Object { $_.Trim() } | Where-Object { $_ }) }
if ($Skip) { $Skip = @($Skip -split ',' | ForEach-Object { $_.Trim() } | Where-Object { $_ }) }

$Root      = Split-Path -Parent $PSScriptRoot
$StateDir  = Join-Path $Root '.ondex'
$PaneDir   = Join-Path $StateDir 'panes'
$StateFile = Join-Path $StateDir 'state.json'
# ┌─ NEGA SDK ADB BIRINCHI ───────────────────────────────────────────────┐
# GlideX'ning O'Z adb nusxasi bor va u SDK'nikidan MUSTAQIL daemon
# ishga tushiradi — ikkalasi BIR VAQTDA ishlasa bir-birini o'ldirib,
# qurilma "topilmadi" bo'lib qoladi (xotira: adb-path-asus-glidex).
# Shuning uchun SDK nusxasi HAR DOIM ustuvor, GlideX faqat SDK
# topilmagan taqdirdagi zaxira.
# └─────────────────────────────────────────────────────────────────────────┘
$AdbExe = 'E:\Sdk\platform-tools\adb.exe'
if (-not (Test-Path $AdbExe)) { $AdbExe = 'C:\Program Files\ASUS\GlideX\adb.exe' }

# ---------------------------------------------------------------- chiqish --

function Write-Head($t) { Write-Host ''; Write-Host "  $t" -ForegroundColor Cyan; Write-Host ('  ' + ('-' * $t.Length)) -ForegroundColor DarkCyan }
function Write-Step($t) { Write-Host "  ->  $t" -ForegroundColor Gray }
function Write-Ok($t)   { Write-Host "  OK  $t" -ForegroundColor Green }
function Write-Note($t) { Write-Host "  !   $t" -ForegroundColor Yellow }
function Write-Bad($t)  { Write-Host "  X   $t" -ForegroundColor Red }

# ┌─ NEGA NATIVE BUYRUQLAR ALOHIDA O'RALADI ────────────────────────────┐
# PowerShell 5.1 da native dasturning stderr'ini `2>$null` yoki `2>&1`
# bilan yo'naltirsak, HAR BIR satr ErrorRecord'ga o'raladi
# (NativeCommandError) va `$ErrorActionPreference = 'Stop'` bo'lganda
# skript o'sha yerda to'xtaydi — dastur 0 qaytargan bo'lsa ham.
#
# `docker info` daemon o'chiq bo'lganda aynan shunday qilardi va butun
# `ondex run` birinchi qadamda yiqilardi. `docker compose up` esa oddiy
# progress'ni ham stderr'ga yozadi, ya'ni MUVAFFAQIYATLI holatda ham.
# └─────────────────────────────────────────────────────────────────────┘

function Invoke-Quiet {
    # Jim ishga tushirish, faqat exit kodi kerak bo'lganda.
    # Yo'naltirish `cmd` ICHIDA bo'lgani uchun PowerShell'ga tegmaydi.
    param([string]$CommandLine)
    cmd /c "$CommandLine >nul 2>&1"
    return $LASTEXITCODE
}

function Invoke-Loud {
    # Chiqishi ko'rinishi kerak bo'lgan native buyruqlar uchun.
    param([string]$CommandLine)
    $prev = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        cmd /c "$CommandLine 2>&1" | ForEach-Object { Write-Host "      $_" -ForegroundColor DarkGray }
    } finally {
        $ErrorActionPreference = $prev
    }
    return $LASTEXITCODE
}

# ------------------------------------------------------------- komponentlar --

# Tartib MUHIM: infra -> api -> web -> tunnel -> UI. Har biri o'zidan
# oldingisiga tayanadi (API bazasiz ishga tushmaydi, panellar API'siz
# bo'sh ekran ko'rsatadi).
$Components = @(
    [pscustomobject]@{ Key = 'infra';      Name = 'Docker infra (Postgres + Redis + MeiliSearch)';  Kind = 'infra';           Dir = '.' }
    [pscustomobject]@{ Key = 'api';        Name = 'Go API  :8080';                     Kind = 'api';             Dir = '.' }
    [pscustomobject]@{ Key = 'web';        Name = 'Next.js super-app / Telegram Mini App  :3000'; Kind = 'web';  Dir = 'apps\web' }
    [pscustomobject]@{ Key = 'tunnel';     Name = 'Cloudflare tunnel (ondex-local)';   Kind = 'tunnel';          Dir = '.' }
    # `Proc` — Windows'da qurilgan EXE nomi (windows/CMakeLists.txt dagi
    # BINARY_NAME). Papka nomidan FARQ QILADI, shuning uchun alohida
    # yozilgan: `ondex status` panel haqiqatan ochilganini shu orqali
    # tekshiradi (oyna tirikligi build muvaffaqiyatini bildirmaydi).
    [pscustomobject]@{ Key = 'admin';      Name = 'Admin panel (Windows)';             Kind = 'flutter-desktop'; Dir = 'apps\admin_panel';      Proc = 'chust_admin' }
    [pscustomobject]@{ Key = 'restaurant'; Name = 'Restoran paneli (Windows)';         Kind = 'flutter-desktop'; Dir = 'apps\restaurant_panel'; Proc = 'chust_restaurant' }
    [pscustomobject]@{ Key = 'customer';   Name = 'Mijoz ilovasi (Android)';           Kind = 'flutter-mobile';  Dir = 'apps\customer_app' }
    [pscustomobject]@{ Key = 'courier';    Name = 'Kuryer ilovasi (Android)';          Kind = 'flutter-mobile';  Dir = 'apps\courier_app' }
    [pscustomobject]@{ Key = 'waiter';     Name = 'Afitsiant ilovasi (Android)';       Kind = 'flutter-mobile';  Dir = 'apps\waiter_app' }
)

# --------------------------------------------------------------- yordamchi --

function Test-Port {
    param([int]$Port, [int]$TimeoutMs = 400)
    $client = New-Object System.Net.Sockets.TcpClient
    try {
        $async = $client.BeginConnect('127.0.0.1', $Port, $null, $null)
        $ok = $async.AsyncWaitHandle.WaitOne($TimeoutMs, $false)
        if ($ok -and $client.Connected) { return $true }
        return $false
    } catch { return $false } finally { $client.Close() }
}

function Wait-Port {
    param([int]$Port, [string]$What, [int]$TimeoutSec = 120)
    $sw = [Diagnostics.Stopwatch]::StartNew()
    while ($sw.Elapsed.TotalSeconds -lt $TimeoutSec) {
        if (Test-Port -Port $Port) {
            Write-Ok "$What tayyor (:$Port, $([math]::Round($sw.Elapsed.TotalSeconds))s)"
            return $true
        }
        Start-Sleep -Milliseconds 800
    }
    Write-Note "$What $TimeoutSec s ichida :$Port da javob bermadi — oynasidagi xatoni ko'ring"
    return $false
}

function Get-OndexTunnel {
    # ┌─ NEGA NOM BO'YICHA EMAS, CONFIG YO'LI BO'YICHA ──────────────────┐
    # Bu kompyuterda BIR NECHTA cloudflared aylanadi: boshqa loyihaning
    # named tunneli va `tunnel --url ...` bilan ochilgan tashlandiq quick
    # tunnellar. `Get-Process cloudflared` ularning hammasini qaytaradi,
    # ya'ni:
    #   * `ondex status` tunnel ishlamasa ham "ISHLAYAPTI" derdi;
    #   * `ondex stop` BOSHQA loyihaning tunnelini o'ldirardi.
    # Yagona ishonchli belgi — buyruq satridagi bizning config yo'limiz.
    # └─────────────────────────────────────────────────────────────────┘
    $needle = (Join-Path $Root 'scripts\cloudflared\config.yml')
    $procs = Get-CimInstance Win32_Process -Filter "Name='cloudflared.exe'" -ErrorAction SilentlyContinue
    return @($procs | Where-Object { $_.CommandLine -and $_.CommandLine -like "*$needle*" })
}

function Test-TunnelConnected {
    # ┌─ JARAYON TIRIK ≠ TUNNEL ISHLAYAPTI ─────────────────────────────┐
    # cloudflared ishga tushib, Cloudflare'ga ULANA OLMASLIGI mumkin
    # (masalan `lookup region1.v2.argotunnel.com: i/o timeout` — stek
    # qayta ko'tarilayotgan paytdagi DNS uzilishi). Jarayon tirik
    # qolaveradi, `ondex status` esa "ISHLAYAPTI" deb yozardi — lekin
    # domenlar 530 qaytarardi va mijoz ilovasida "serverga ulanib
    # bo'lmadi" chiqardi. Ya'ni status YOLG'ON gapirardi.
    #
    # Yagona ishonchli belgi — tunnelning FAOL ULANISHLARI bor-yo'qligi.
    # └─────────────────────────────────────────────────────────────────┘
    if (-not (Get-Command cloudflared -ErrorAction SilentlyContinue)) { return $true }
    $cfg = Join-Path $Root 'scripts\cloudflared\config.yml'
    $info = cmd /c "cloudflared --config `"$cfg`" tunnel info ondex-local 2>&1"
    if ($info -match 'does not have any active connection') { return $false }
    return $true
}

function Test-OurProcess {
    # ┌─ NEGA YO'L BO'YICHA TEKSHIRISH YETARLI EMAS ────────────────────┐
    # Avval jarayon "bizniki"mi degan savolga uning yo'lida loyiha
    # papkasi bor-yo'qligiga qarab javob berilardi. Bu `go run` uchun
    # NOTO'G'RI ishlaydi: u binarni Go keshiga quradi
    # (`D:\gocache\...\api.exe`) va yo'lda `F:\ChustApp` UMUMAN
    # ko'rinmaydi. Natijada `ondex run` o'zining API'sini "begona
    # jarayon" deb e'lon qilib, foydalanuvchiga uni yopishni
    # maslahat berardi.
    #
    # To'g'ri belgi — AJDODLAR zanjiri: bizning pane oynamizdan
    # tug'ilgan har qanday jarayon bizniki. `npm run dev` (pane -> cmd
    # -> node) uchun ham shu ishlaydi.
    # └─────────────────────────────────────────────────────────────────┘
    param([int]$ProcId)
    $tracked = @(Load-State | Where-Object { $_.pid } | ForEach-Object { [int]$_.pid })
    $cur = $ProcId
    for ($i = 0; $i -lt 6 -and $cur -gt 0; $i++) {
        $p = Get-CimInstance Win32_Process -Filter "ProcessId=$cur" -ErrorAction SilentlyContinue
        if (-not $p) { return $false }
        if ("$($p.CommandLine) $($p.ExecutablePath)" -like "*$Root*") { return $true }
        if ($tracked -contains [int]$p.ProcessId) { return $true }
        $cur = [int]$p.ParentProcessId
    }
    return $false
}

function Get-PortOwner {
    # Portni kim band qilgan va u BIZNIKIMI.
    #
    # Nega muhim: :3000 ni begona jarayon (masalan tashlandiq `npx serve`)
    # band qilib tursa, eski kod shunchaki "port band" deb o'tib ketardi —
    # natijada Next.js UMUMAN ishga tushmasdi, lekin tunnel baribir
    # dev-web-ondex.shoxpro.uz ni o'sha BEGONA serverga yo'naltirardi.
    # Sahifa ochilardi, faqat butunlay boshqa ilova bo'lardi.
    param([int]$Port)
    $conn = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
    if (-not $conn) { return $null }
    $procId = $conn[0].OwningProcess
    $wp = Get-CimInstance Win32_Process -Filter "ProcessId=$procId" -ErrorAction SilentlyContinue
    $nm = 'nomalum'
    if ($wp) { $nm = $wp.Name }
    return [pscustomobject]@{
        Pid  = $procId
        Name = $nm
        Ours = (Test-OurProcess -ProcId $procId)
    }
}

function Get-LanIp {
    $ip = Get-NetIPAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue |
        Where-Object {
            $_.InterfaceAlias -notmatch 'Loopback|WSL|vEthernet|Docker|Bluetooth' -and
            $_.IPAddress -notmatch '^(169\.254|127\.)'
        } | Select-Object -First 1
    if ($ip) { return $ip.IPAddress }
    return $null
}

function Get-AndroidDevice {
    if (-not (Test-Path $AdbExe)) {
        $fallback = (Get-Command adb -ErrorAction SilentlyContinue).Source
        if (-not $fallback) { return $null }
        $script:AdbExe = $fallback
    }
    # adb "* daemon started successfully" ni stderr'ga yozadi — shuning
    # uchun bu ham `cmd` orqali (yuqoridagi izohga qarang).
    $lines = cmd /c "`"$AdbExe`" devices 2>nul"
    foreach ($l in $lines) {
        if ($l -match '^(\S+)\s+device$') { return $Matches[1] }
    }
    return $null
}

function Save-Pane {
    # Buyruqni vaqtinchalik .ps1 ga yozib, `powershell -File` bilan ochamiz.
    # Sabab: buyruq satrida qo'shtirnoq/apostrof qochirish Windows'da
    # ishonchsiz — fayl orqali bu muammo umuman yo'q.
    param([string]$Key, [string]$Title, [string]$WorkDir, [string[]]$Body)

    if (-not (Test-Path $PaneDir)) { New-Item -ItemType Directory -Path $PaneDir -Force | Out-Null }
    $file = Join-Path $PaneDir "$Key.ps1"

    $lines = New-Object System.Collections.Generic.List[string]
    $lines.Add('$host.UI.RawUI.WindowTitle = "OnDex :: ' + $Title + '"')
    # ┌─ QUICKEDIT O'CHIRILADI — SERVERNI QOTIRIB QO'YMASIN ────────────┐
    # Windows konsolida "QuickEdit" yoqilgan: oynaga sichqoncha bilan
    # tegilsa (yoki tasodifan bosilsa) matn BELGILANADI va konsol
    # chiqishi TO'XTAYDI. Shu paytda jarayon `stdout` ga yozmoqchi
    # bo'lsa — u BLOKLANADI va butun server qotib qoladi.
    #
    # 2026-08-19 da aynan shu bo'ldi: Go API tirik edi, GET so'rovlar
    # ishlardi, lekin LOG YOZADIGAN so'rovlar (`POST /orders`, to'lov
    # callback'i) abadiy osilib qoldi. Sabab kodda emas, konsolda
    # ekanini topish uchun anchagina vaqt ketdi.
    #
    # Yechim: pane ochilishi bilan QuickEdit o'chiriladi. Matn nusxa
    # olish kerak bo'lsa — oyna sarlavhasidan o'ng tugma > Mark.
    # └─────────────────────────────────────────────────────────────────┘
    $lines.Add('try {')
    $lines.Add('  Add-Type -Namespace OnDex -Name Con -MemberDefinition @"')
    $lines.Add('[DllImport("kernel32.dll", SetLastError=true)] public static extern IntPtr GetStdHandle(int h);')
    $lines.Add('[DllImport("kernel32.dll", SetLastError=true)] public static extern bool GetConsoleMode(IntPtr h, out uint m);')
    $lines.Add('[DllImport("kernel32.dll", SetLastError=true)] public static extern bool SetConsoleMode(IntPtr h, uint m);')
    $lines.Add('"@ -ErrorAction Stop')
    $lines.Add('  $h = [OnDex.Con]::GetStdHandle(-10)')
    $lines.Add('  $m = 0')
    $lines.Add('  if ([OnDex.Con]::GetConsoleMode($h, [ref]$m)) {')
    # 0x40 = ENABLE_QUICK_EDIT_MODE, 0x80 = ENABLE_EXTENDED_FLAGS
    $lines.Add('    [void][OnDex.Con]::SetConsoleMode($h, ($m -band -bnot 0x40) -bor 0x80)')
    $lines.Add('  }')
    $lines.Add('} catch { }')
    $lines.Add('Set-Location -LiteralPath "' + $WorkDir + '"')
    $lines.Add('Write-Host ""')
    $lines.Add('Write-Host "  OnDex :: ' + $Title + '" -ForegroundColor Cyan')
    $lines.Add('Write-Host ""')
    foreach ($b in $Body) { $lines.Add($b) }
    $lines.Add('Write-Host ""')
    $lines.Add('Write-Host "  [ jarayon tugadi - oyna ochiq qoldi ]" -ForegroundColor Yellow')

    # BOM BILAN yoziladi. PowerShell 5.1 BOM'siz `.ps1` ni tizim ANSI
    # kodlashida o'qiydi va `—` kabi belgilar cp1252 da `â€”` ga aylanadi;
    # uning oxirgi belgisi `”` bo'lib, PowerShell uni HAQIQIY qo'shtirnoq
    # deb qabul qilib satrni erta yopadi va butun skript parse bo'lmaydi.
    [System.IO.File]::WriteAllLines($file, $lines, (New-Object System.Text.UTF8Encoding($true)))
    return $file
}

function Start-Pane {
    param([string]$Key, [string]$Title, [string]$WorkDir, [string[]]$Body)
    $file = Save-Pane -Key $Key -Title $Title -WorkDir $WorkDir -Body $Body
    $p = Start-Process -FilePath 'powershell.exe' `
        -ArgumentList '-NoExit', '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', "`"$file`"" `
        -WorkingDirectory $WorkDir -PassThru
    return $p
}

function Load-State {
    if (-not (Test-Path $StateFile)) { return @() }
    try {
        # PowerShell 5.1 da `ConvertFrom-Json` JSON MASSIVINI bitta
        # `Object[]` obyekti qilib chiqaradi va `@(...)` uni YOYMAYDI —
        # ro'yxat 1 elementli bo'lib qolardi. Keyin `$_.key` a'zolar
        # bo'yicha sanashga tushib "api tunnel admin ..." kabi qo'shma
        # qiymat berardi, filtr hech qachon mos kelmasdi va holat faylida
        # takroriy yozuvlar to'planardi.
        # Quvur orqali o'tkazish massivni haqiqatan elementlarga yoyadi.
        $parsed = Get-Content $StateFile -Raw | ConvertFrom-Json
        return @($parsed | ForEach-Object { $_ })
    } catch { return @() }
}

function Save-State($entries) {
    if (-not (Test-Path $StateDir)) { New-Item -ItemType Directory -Path $StateDir -Force | Out-Null }
    # `-InputObject` bilan: quvur orqali berilsa massiv elementlarga
    # bo'linib ketadi va bitta element qolganda JSON massiv bo'lmay qoladi.
    #
    # Kalit bo'yicha takrorlar OLIB TASHLANADI (oxirgisi yutadi): bir xil
    # komponent qayta ishga tushirilganda eski, o'lik PID fayl ichida
    # qolib ketmasligi uchun.
    $seen = @{}
    $arr = @()
    foreach ($e in @($entries | Where-Object { $_ })) {
        if ($e.key) { $seen[$e.key] = $e } else { $arr += $e }
    }
    foreach ($k in $seen.Keys) { $arr += $seen[$k] }
    $json = ConvertTo-Json -InputObject $arr -Depth 4
    [System.IO.File]::WriteAllText($StateFile, $json, (New-Object System.Text.UTF8Encoding($false)))
}

function Ensure-Docker {
    Write-Step 'Docker holati tekshirilmoqda...'
    if ((Invoke-Quiet 'docker info') -eq 0) { Write-Ok 'Docker ishlayapti'; return $true }

    Write-Note 'Docker Desktop ishlamayapti — ishga tushirilmoqda (1-3 daqiqa)'
    $exe = 'C:\Program Files\Docker\Docker\Docker Desktop.exe'
    if (-not (Test-Path $exe)) { Write-Bad "Docker Desktop topilmadi: $exe"; return $false }
    Start-Process -FilePath $exe | Out-Null

    $sw = [Diagnostics.Stopwatch]::StartNew()
    while ($sw.Elapsed.TotalSeconds -lt 240) {
        Start-Sleep -Seconds 5
        if ((Invoke-Quiet 'docker info') -eq 0) {
            Write-Host ''
            Write-Ok "Docker tayyor ($([math]::Round($sw.Elapsed.TotalSeconds))s)"
            return $true
        }
        Write-Host '.' -NoNewline -ForegroundColor DarkGray
    }
    Write-Host ''
    # Xotirada tanilgan nosozlik: WSL vhdx ulanmay qolsa Docker UI abadiy kutadi.
    Write-Bad 'Docker 4 daqiqada ko''tarilmadi. Odatdagi sabab: WSL vhdx ulanmagan.'
    Write-Note 'Yechim: wsl --shutdown  keyin Docker Desktop ni qayta oching.'
    return $false
}

# ------------------------------------------------------------- ondex ip ----

function Invoke-IpRefresh {
    param([switch]$Quiet)

    $ip = Get-LanIp
    if (-not $ip) { Write-Bad 'LAN IP topilmadi (Wi-Fi ulanmaganmi?)'; return $null }

    # Faqat MOBIL ilovalar LAN IP ga muhtoj. Panellar API bilan bir xil
    # mashinada bo'lgani uchun localhost ishlatadi va tegilmaydi.
    $targets = @('apps\customer_app\config\dev.json', 'apps\waiter_app\config\dev.json')
    $changed = 0
    foreach ($rel in $targets) {
        $path = Join-Path $Root $rel
        if (-not (Test-Path $path)) { continue }
        $raw = [System.IO.File]::ReadAllText($path)
        $new = [regex]::Replace($raw, '"ONDEX_(API|WEB)_URL"\s*:\s*"http://[0-9.]+:(\d+)"', {
            param($m)
            '"ONDEX_' + $m.Groups[1].Value + '_URL": "http://' + $ip + ':' + $m.Groups[2].Value + '"'
        })
        if ($new -ne $raw) {
            [System.IO.File]::WriteAllText($path, $new, (New-Object System.Text.UTF8Encoding($false)))
            $changed++
            if (-not $Quiet) { Write-Ok "$rel -> $ip" }
        }
    }
    if ($changed -eq 0 -and -not $Quiet) { Write-Ok "dev.json lar allaqachon $ip da" }
    return $ip
}

# ---------------------------------------------------------- ondex backup ---

function Invoke-Backup {
    param(
        # `ondex run` ichidan chaqirilgan: chiqish qisqa bo'ladi va
        # muvaffaqiyatsizlik stekni ishga tushirishga TO'SQINLIK QILMAYDI.
        [switch]$Auto
    )
    # ┌─ NEGA KERAK ────────────────────────────────────────────────────┐
    # 2026-08-16 gacha loyihada BIRORTA zaxira nusxa mexanizmi yo'q edi
    # — na skript, na cron, na CI qadami. Ya'ni butun katalog va barcha
    # buyurtmalar bitta Docker volume'da, YAGONA nusxada turardi.
    # `docker compose down -v` bitta buyruq bilan hammasini o'chirardi.
    #
    # Bu skript LOKAL nusxa uchun. Production uchun `deploy/backup.sh`.
    # └─────────────────────────────────────────────────────────────────┘
    #
    # ┌─ NEGA `ondex run` GA ULANDI ────────────────────────────────────┐
    # Mexanizm yozildi, lekin QO'LDA chaqirilardi — va 11 kun davomida
    # bir marta ham chaqirilmadi.
    #
    # 2026-08-27 da Docker'ning WSL ma'lumot diski yo'qoldi: C: to'lib
    # ketgan (5.5 GB qolgan), Windows virtual xotirasi tugagan va Docker
    # backend'i shutdown yozuvisiz o'ldirilgan. Qayta yoqilganda Docker
    # `neither WSL2 data distro nor disk exist` deb yangi bo'sh disk
    # yasab formatladi — barcha volume'lar bilan birga.
    # Omon qolgan yagona nusxa 11 kunlik edi.
    #
    # Xulosa: qo'lda chaqiriladigan zaxira — zaxira emas.
    # └─────────────────────────────────────────────────────────────────┘
    if (-not $Auto) { Write-Head 'Zaxira nusxa' }

    if ((Invoke-Quiet 'docker info') -ne 0) {
        if ($Auto) { Write-Note 'Docker ishlamayapti — zaxira o''tkazib yuborildi.' }
        else       { Write-Bad  'Docker ishlamayapti — zaxira olib bo''lmaydi.' }
        return
    }

    $backupRoot = Join-Path $StateDir 'backup'

    # --- Avtomatik rejimda har safar takrorlanmasin ---
    # `ondex run` kuniga bir necha marta chaqiriladi. Oxirgi TO'LIQ nusxa
    # yosh bo'lsa qayta dump olishning ma'nosi yo'q.
    if ($Auto -and $BackupEvery -gt 0) {
        $last = Get-ChildItem $backupRoot -Directory -ErrorAction SilentlyContinue |
            Where-Object { $_.Name -match '^\d{4}-\d{2}-\d{2}_\d{6}$' } |
            Sort-Object Name -Descending | Select-Object -First 1
        if ($last -and $last.LastWriteTime -gt (Get-Date).AddHours(-$BackupEvery)) {
            $age = [math]::Round(((Get-Date) - $last.LastWriteTime).TotalHours, 1)
            Write-Step "zaxira o'tkazildi (oxirgisi $age soat oldin, chegara $BackupEvery soat)"
            return
        }
    }

    if ($Auto) { Write-Step 'zaxira nusxa olinmoqda...' }

    $stamp = Get-Date -Format 'yyyy-MM-dd_HHmmss'
    $dir = Join-Path $backupRoot $stamp
    New-Item -ItemType Directory -Path $dir -Force | Out-Null

    $ok = $true

    # --- PostgreSQL ---
    # `--clean --if-exists` — tiklashda avval mavjud obyektlar
    # tushiriladi, ya'ni bo'sh bo'lmagan bazaga ham tiklab bo'ladi.
    # Yo'naltirish `cmd` ICHIDA: PowerShell'ning `>` operatori chiqishni
    # UTF-16 yoki BOM bilan yozib dump'ni buzardi.
    Write-Step 'PostgreSQL...'
    $pgFile = Join-Path $dir 'postgres.sql'
    cmd /c "docker exec chustapp-db-1 pg_dump -U chust -d chustapp --clean --if-exists > `"$pgFile`" 2>nul"
    $pg = Get-Item $pgFile -ErrorAction SilentlyContinue
    if ($pg -and $pg.Length -gt 1024) {
        Write-Ok ("postgres.sql  {0} KB" -f [math]::Round($pg.Length / 1KB, 1))
    } else {
        Write-Bad 'PostgreSQL dump bo''sh yoki juda kichik — konteyner ishlayaptimi?'
        $ok = $false
    }

    # --- MongoDB ---
    Write-Step 'MongoDB...'
    $mgFile = Join-Path $dir 'mongo.archive'
    Invoke-Quiet 'docker exec chustapp-mongo-1 mongodump --db chustapp --archive=/tmp/ondex-backup.archive' | Out-Null
    Invoke-Quiet "docker cp chustapp-mongo-1:/tmp/ondex-backup.archive `"$mgFile`"" | Out-Null
    Invoke-Quiet 'docker exec chustapp-mongo-1 rm -f /tmp/ondex-backup.archive' | Out-Null
    $mg = Get-Item $mgFile -ErrorAction SilentlyContinue
    if ($mg -and $mg.Length -gt 512) {
        Write-Ok ("mongo.archive {0} KB" -f [math]::Round($mg.Length / 1KB, 1))
    } else {
        Write-Bad 'MongoDB dump bo''sh — katalog yo''qolishi mumkin!'
        $ok = $false
    }

    # --- Tiklash yo'riqnomasi nusxa BILAN BIRGA saqlanadi ---
    # Sabab: zaxira kerak bo'lgan payt odam vahimada bo'ladi va
    # buyruqlarni eslay olmaydi. Yo'riqnoma nusxaning yonida tursin.
    $readme = @'
OnDex zaxira nusxasi — TIKLASH

  Avval konteynerlar ishlab turishi kerak:  ondex run -Only infra

  PostgreSQL (buyurtma, foydalanuvchi, kuryer):
    Get-Content postgres.sql | docker exec -i chustapp-db-1 psql -U chust -d chustapp

  MongoDB (katalog: restoran, menyu, aksiya):
    docker cp mongo.archive chustapp-mongo-1:/tmp/r.archive
    docker exec chustapp-mongo-1 mongorestore --drop --archive=/tmp/r.archive

  DIQQAT: `--drop` mavjud kolleksiyalarni o'chirib qayta yozadi.
  Aralashtirmaslik kerak bo'lsa avval bo'sh bazaga tiklab tekshiring.
'@
    [System.IO.File]::WriteAllText((Join-Path $dir 'TIKLASH.txt'), $readme,
        (New-Object System.Text.UTF8Encoding($true)))

    # ┌─ NUQSONLI NUSXA YAXSHISINI SIQIB CHIQARMASIN ───────────────────┐
    # Avval navbat tozalash `$ok` ga QARAMASDAN ishlardi: bo'sh dump ham
    # navbatda joy egallardi va `-Keep` chegarasidan oshganda HAQIQIY
    # nusxani o'chirib yuborardi.
    #
    # Bu nazariy xavf emas. Zaxira endi avtomatik olinadi, ya'ni baza
    # bo'sh bo'lgan holatda ham (masalan volume yo'qolgandan keyin)
    # `ondex run` har safar bo'sh nusxa yasaydi — yetti marta ishga
    # tushirilsa omon qolgan YAGONA haqiqiy nusxa ham yo'q bo'lardi.
    #
    # Endi: butunlay bo'sh nusxa darrov o'chiriladi, chala nusxa
    # `_notoliq` qo'shimchasi bilan ajratiladi va navbatda qatnashmaydi.
    # └─────────────────────────────────────────────────────────────────┘
    if (-not $ok) {
        $anyData = ($pg -and $pg.Length -gt 1024) -or ($mg -and $mg.Length -gt 512)
        if (-not $anyData) {
            Remove-Item $dir -Recurse -Force -ErrorAction SilentlyContinue
            Write-Bad 'Ikkala dump ham bo''sh — nusxa saqlanmadi, eskilariga tegilmadi.'
            return
        }
        $dir = (Rename-Item -Path $dir -NewName "${stamp}_notoliq" -Force -PassThru).FullName
    }

    # --- Eskilarini tozalash: FAQAT to'liq nusxalar orasida ---
    $all = @(Get-ChildItem $backupRoot -Directory -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -match '^\d{4}-\d{2}-\d{2}_\d{6}$' } |
        Sort-Object Name -Descending)
    if ($all.Count -gt $Keep) {
        foreach ($old in $all[$Keep..($all.Count - 1)]) {
            Remove-Item $old.FullName -Recurse -Force -ErrorAction SilentlyContinue
            Write-Step "eski nusxa o'chirildi: $($old.Name)"
        }
    }

    # Chala nusxalar ham cheksiz to'planmasin — oxirgi uchtasi yetadi.
    $bad = @(Get-ChildItem $backupRoot -Directory -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -match '_notoliq$' } | Sort-Object Name -Descending)
    if ($bad.Count -gt 3) {
        foreach ($old in $bad[3..($bad.Count - 1)]) {
            Remove-Item $old.FullName -Recurse -Force -ErrorAction SilentlyContinue
        }
    }

    if ($Auto) {
        if ($ok) { Write-Ok "zaxira olindi: $stamp" }
        else     { Write-Note "zaxira CHALA: $stamp (stek baribir ishga tushadi)" }
        return
    }

    Write-Host ''
    if ($ok) { Write-Ok "Zaxira tayyor: $dir" } else { Write-Bad "Zaxira TO'LIQ EMAS: $dir" }
    Write-Host "  Saqlanadi: oxirgi $Keep ta to'liq nusxa" -ForegroundColor DarkGray
    Write-Host ''
}

# ----------------------------------------------------------- ondex build ---
#
# ┌─ MAQSAD ────────────────────────────────────────────────────────────────┐
# `ondex build -Only OnDex` — BITTA buyruq bilan: versiya +1, build,
# (mobil bo'lsa) telefon ulangan bo'lsa avtomatik o'rnatish, natija va
# versiya ANIQ ko'rsatiladi. Mavjud `F:\OndexProd\build_mobile.ps1` /
# `build.ps1` QAYTA ISHLATILADI (versiya bump, xotira/Docker tekshiruvi,
# manzil+imzo tasdig'i — hammasi ALLAQACHON o'sha yerda va TAKRORLANMAYDI).
# Bu funksiya faqat ustiga TELEFONGA O'RNATISH va aniq XABAR qo'shadi.
# └────────────────────────────────────────────────────────────────────────┘

# Nishon nomi -> ilova papkasi/turi/rejimi. Nomlar ATAYLAB foydalanuvchi
# so'ragan holida (katta-kichik harf farqlanmaydi — pastdagi qidiruvda).
$BuildTargets = @(
    [pscustomobject]@{ Name = 'OnDex';         App = 'customer_app';     Mode = 'prod'; Kind = 'mobile';  Label = 'OnDex (mijoz)' }
    [pscustomobject]@{ Name = 'OnDexDev';      App = 'customer_app';     Mode = 'dev';  Kind = 'mobile';  Label = 'OnDex (mijoz) — DEV' }
    [pscustomobject]@{ Name = 'OnDexPro';      App = 'waiter_app';       Mode = 'prod'; Kind = 'mobile';  Label = 'OnDexPro (affitsiant)' }
    [pscustomobject]@{ Name = 'OnDexProDev';   App = 'waiter_app';       Mode = 'dev';  Kind = 'mobile';  Label = 'OnDexPro (affitsiant) — DEV' }
    [pscustomobject]@{ Name = 'OnDexGO';       App = 'courier_app';      Mode = 'prod'; Kind = 'mobile';  Label = 'OnDexGO (kuryer)' }
    [pscustomobject]@{ Name = 'OnDexGoDev';    App = 'courier_app';      Mode = 'dev';  Kind = 'mobile';  Label = 'OnDexGO (kuryer) — DEV' }
    [pscustomobject]@{ Name = 'OnDexAdmin';    App = 'admin_panel';      Mode = 'prod'; Kind = 'desktop'; Label = 'OnDex Admin paneli'; Proc = 'chust_admin';      BuildKey = 'admin' }
    [pscustomobject]@{ Name = 'OnDexAdminDev'; App = 'admin_panel';      Mode = 'dev';  Kind = 'desktop'; Label = 'OnDex Admin paneli — DEV'; Proc = 'chust_admin' }
    [pscustomobject]@{ Name = 'Merchant';      App = 'restaurant_panel'; Mode = 'prod'; Kind = 'desktop'; Label = 'OnDex Merchant (restoran paneli)'; Proc = 'chust_restaurant'; BuildKey = 'restoran' }
    [pscustomobject]@{ Name = 'MerchantDev';   App = 'restaurant_panel'; Mode = 'dev';  Kind = 'desktop'; Label = 'OnDex Merchant (restoran paneli) — DEV'; Proc = 'chust_restaurant' }
)

function Get-BuildTarget {
    param([string]$Name)
    return $BuildTargets | Where-Object { $_.Name -eq $Name } | Select-Object -First 1
}

# Ilova versiyasini pubspec.yaml dan o'qiydi ("0.2.3+15" -> "v0.2.3 (15)").
function Get-AppVersionLabel {
    param([string]$AppDir, [switch]$Dev)
    $path = Join-Path (Join-Path $Root "apps\$AppDir") 'pubspec.yaml'
    $m = Select-String -Path $path -Pattern '^version:\s*(.+)$' | Select-Object -First 1
    if (-not $m) { return '(versiya topilmadi)' }
    $raw = $m.Matches.Groups[1].Value.Trim()
    $parts = $raw -split '\+'
    $label = "v$($parts[0])"
    if ($Dev) { $label += '-dev' }
    if ($parts.Count -gt 1) { $label += " ($($parts[1]))" }
    return $label
}

# Qurilmadagi OLDINGI PLANDA turgan ilova — foydalanuvchining ruxsatisiz
# o'yin/boshqa ilova ustiga o'rnatib yubormaslik uchun (standart qoida).
# O'ZIMIZNING paketimiz yoki uy ekrani (launcher) bo'lsa so'ralmaydi.
function Confirm-ForegroundSafe {
    param([string]$Device)
    try {
        $dump = cmd /c "`"$AdbExe`" -s $Device shell dumpsys activity activities 2>nul"
        $line = $dump | Select-String -Pattern 'mResumedActivity|topResumedActivity' | Select-Object -First 1
        if (-not $line) { return $true }
        $text = "$line"
        if ($text -match 'com\.ondex\.' -or $text -match 'launcher') { return $true }
        Write-Note "Telefon old planida boshqa ilova ochiq ko'rinadi:"
        Write-Host "      $text" -ForegroundColor DarkGray
        $ans = Read-Host '  O''rnatishni davom ettiraymi? (h/y)'
        return ($ans -match '^(h|ha|y|yes)$')
    } catch { return $true }
}

# Mobil APK'ni telefonga o'rnatadi (ulangan bo'lsa) va ANIQ natija chiqaradi.
function Install-ApkResult {
    param([string]$ApkPath, [string]$Label, [string]$VersionLabel)
    $device = Get-AndroidDevice
    if (-not $device) {
        Write-Note "Telefon ULANMAGAN — $Label $VersionLabel qurildi, lekin O'RNATILMADI."
        Write-Note "APK: $ApkPath"
        Write-Note 'USB-debugging yoqilgan telefonni ulab, buyruqni qayta ishga tushiring.'
        return
    }
    if (-not (Confirm-ForegroundSafe -Device $device)) {
        Write-Note "O'rnatish sizning so'rovingiz bilan BEKOR qilindi. APK: $ApkPath"
        return
    }
    Write-Step "telefonga o'rnatilmoqda (qurilma: $device)..."
    $out = cmd /c "`"$AdbExe`" -s $device install -r `"$ApkPath`" 2>&1"
    if ($LASTEXITCODE -eq 0 -and ($out -join "`n") -match 'Success') {
        Write-Ok "$Label $VersionLabel — QURILDI VA TELEFONGA O'RNATILDI."
    } else {
        Write-Bad "$Label — o'rnatish muvaffaqiyatsiz:"
        $out | ForEach-Object { Write-Host "      $_" -ForegroundColor DarkGray }
        Write-Note "APK qo'lda o'rnatish uchun tayyor: $ApkPath"
    }
}

function Invoke-Build {
    if (-not $Only) {
        Write-Bad "Nishon ko'rsatilmadi. Masalan: ondex build -Only OnDex"
        Write-Note ("Mavjud nomlar: " + (($BuildTargets | ForEach-Object { $_.Name }) -join ', '))
        return
    }

    foreach ($name in $Only) {
        $t = Get-BuildTarget -Name $name
        if (-not $t) {
            Write-Bad "Noma'lum nishon: '$name'"
            Write-Note ("Mavjud nomlar: " + (($BuildTargets | ForEach-Object { $_.Name }) -join ', '))
            continue
        }

        Write-Head $t.Label
        $appDir = Join-Path $Root "apps\$($t.App)"

        try {
            if ($t.Kind -eq 'mobile' -and $t.Mode -eq 'prod') {
                # ── Mobil, PRODUCTION: F:\OndexProd\build_mobile.ps1 qayta ishlatiladi ──
                Push-Location 'F:\OndexProd'
                try { & '.\build_mobile.ps1' -Apps $t.App }
                finally { Pop-Location }
                $ver = Get-AppVersionLabel -AppDir $t.App
                $apk = Get-ChildItem 'F:\OndexProd\apk' -Filter "$($t.App)-*.apk" -ErrorAction SilentlyContinue |
                    Sort-Object LastWriteTime -Descending | Select-Object -First 1
                if (-not $apk) { throw 'tayyor APK topilmadi' }
                Install-ApkResult -ApkPath $apk.FullName -Label $t.Label -VersionLabel $ver
            }
            elseif ($t.Kind -eq 'mobile' -and $t.Mode -eq 'dev') {
                # ── Mobil, DEV: versiya OSHIRILMAYDI (dev build joriy raqamni
                # "-dev" bilan ko'rsatadi — ondex_core/config.dart). ──
                Push-Location $appDir
                try {
                    Write-Step 'flutter build apk --debug (config/dev-tunnel.json)...'
                    Invoke-Loud 'flutter build apk --debug --dart-define-from-file=config/dev-tunnel.json' | Out-Null
                    if ($LASTEXITCODE -ne 0) { throw "build muvaffaqiyatsiz (kod $LASTEXITCODE)" }
                }
                finally { Pop-Location }
                $apk = Join-Path $appDir 'build\app\outputs\flutter-apk\app-debug.apk'
                if (-not (Test-Path $apk)) { throw 'app-debug.apk topilmadi' }
                $ver = Get-AppVersionLabel -AppDir $t.App -Dev
                Install-ApkResult -ApkPath $apk -Label $t.Label -VersionLabel $ver
            }
            elseif ($t.Kind -eq 'desktop' -and $t.Mode -eq 'prod') {
                # ── Desktop, PRODUCTION: F:\OndexProd\build.ps1 qayta ishlatiladi ──
                Push-Location 'F:\OndexProd'
                try { & '.\build.ps1' -Only $t.BuildKey }
                finally { Pop-Location }
                $ver = Get-AppVersionLabel -AppDir $t.App
                $dst = Join-Path 'F:\OndexProd' $t.BuildKey
                # `build.ps1` arxiv nomlari: OnDex-Admin-*.zip / OnDex-Restoran-*.zip.
                $zipPattern = if ($t.BuildKey -eq 'admin') { 'OnDex-Admin-*.zip' } else { 'OnDex-Restoran-*.zip' }
                $zip = Get-ChildItem 'F:\OndexProd\arxiv' -Filter $zipPattern -ErrorAction SilentlyContinue |
                    Sort-Object LastWriteTime -Descending | Select-Object -First 1
                Write-Ok "$($t.Label) $ver — QURILDI."
                Write-Host "      Papka (ochib ishlatish uchun) : $dst" -ForegroundColor DarkGray
                if ($zip) {
                    Write-Host "      Boshqa qurilmaga YUBORISH uchun shu faylni ko'chiring:" -ForegroundColor Cyan
                    Write-Host "      $($zip.FullName)" -ForegroundColor White
                }
            }
            elseif ($t.Kind -eq 'desktop' -and $t.Mode -eq 'dev') {
                # ── Desktop, DEV: versiya OSHIRILMAYDI. ──
                #
                # `build.ps1` (PROD yo'l) panel ochiqligini o'zi tekshiradi
                # va Remove-Item'dan OLDIN to'xtaydi. Bu DEV yo'lida
                # Remove-Item yo'q, lekin Flutter O'ZI ochiq .exe faylini
                # qayta yozishga urinib, tushunarsiz "Access is denied"
                # bilan yiqiladi. Aniq sabab bilan OLDINDAN to'xtatamiz.
                if ($t.Proc -and (Get-Process -Name $t.Proc -ErrorAction SilentlyContinue)) {
                    throw "panel hozir OCHIQ (jarayon '$($t.Proc)') — avval uni yoping, keyin qayta ishga tushiring."
                }
                Push-Location $appDir
                try {
                    Write-Step 'flutter build windows --debug (config/dev-tunnel.json)...'
                    New-Item -ItemType Directory -Force (Join-Path $appDir 'build\native_assets\windows') | Out-Null
                    Invoke-Loud 'flutter build windows --debug --dart-define-from-file=config/dev-tunnel.json' | Out-Null
                    if ($LASTEXITCODE -ne 0) { throw "build muvaffaqiyatsiz (kod $LASTEXITCODE)" }
                }
                finally { Pop-Location }
                $dst = Join-Path $appDir 'build\windows\x64\runner\Debug'
                $ver = Get-AppVersionLabel -AppDir $t.App -Dev
                Write-Ok "$($t.Label) $ver — QURILDI."
                Write-Host "      Papka: $dst" -ForegroundColor DarkGray
            }
        }
        catch {
            Write-Bad "$($t.Label): $($_.Exception.Message)"
        }
        Write-Host ''
    }
}

# ---------------------------------------------------------- ondex version --
#
# ┌─ IKKI XABAR, IKKI SAVOLGA JAVOB ─────────────────────────────────────────┐
# "Joriy (manba)" — pubspec.yaml dagi versiya. Bu ODIL: hali qurilmagan
# o'zgarish bo'lsa ham SHU raqam ko'rinadi (keyingi build shuni beradi).
#
# "Oxirgi PROD/DEV build" — HAQIQATAN qurilgan faylning O'ZIDAN (APK/EXE
# vaqt belgisi). Ikkalasi bir-biridan farq qilishi mumkin: masalan
# pubspec qo'lda bumplangan, lekin hali qurilmagan bo'lsa — "joriy" oshgan,
# "oxirgi build" esa eskicha qoladi. Bitta raqam bilan bu ikki holat
# chalkashtirilmasin deb ATAYLAB ikkiga ajratilgan.
# └───────────────────────────────────────────────────────────────────────────┘

# Har bir ilova/panel — versiya va build hisobotlari SHU ro'yxatdan.
# `$BuildTargets`dan farqli: bu yerda prod/dev ALOHIDA qator EMAS, bitta
# ilova bitta qator (ikkalasi ham shu qatorda ko'rsatiladi).
$VersionApps = @(
    [pscustomobject]@{ App = 'customer_app';     Label = 'OnDex (mijoz)';         Kind = 'mobile' }
    [pscustomobject]@{ App = 'courier_app';      Label = 'OnDexGO (kuryer)';      Kind = 'mobile' }
    [pscustomobject]@{ App = 'waiter_app';       Label = 'OnDexPro (affitsiant)'; Kind = 'mobile' }
    [pscustomobject]@{ App = 'admin_panel';      Label = 'OnDex Admin';           Kind = 'desktop'; Exe = 'chust_admin.exe';      ProdDir = 'admin' }
    [pscustomobject]@{ App = 'restaurant_panel'; Label = 'OnDex Merchant';        Kind = 'desktop'; Exe = 'chust_restaurant.exe'; ProdDir = 'restoran' }
)

# Fayl nomidan versiyani o'qiydi: "customer_app-0.2.3_15.apk" -> "v0.2.3 (15)".
#
# ANCHOR (`-...$`) ATAYLAB YO'Q: APK nomida versiyadan keyin darhol
# kengaytma keladi ("...0.2.3_15.apk"), lekin panel arxivida orasida
# "-win-x64" bor ("OnDex-Admin-0.1.1_1-win-x64.zip") — qattiq naqsh
# ikkinchisini o'tkazib yubordi va versiya bo'sh ko'rinardi.
function Get-VersionFromFileName {
    param([string]$FileName)
    if ($FileName -match '(\d+\.\d+\.\d+)_(\d+)') {
        return "v$($Matches[1]) ($($Matches[2]))"
    }
    # Build raqamisiz eski fayllar (masalan "OnDex-Admin-0.1.0-win-x64.zip",
    # versiyalash tizimidan OLDIN qurilgan) — build raqamsiz ko'rsatiladi.
    if ($FileName -match '(\d+\.\d+\.\d+)') {
        return "v$($Matches[1])"
    }
    return $null
}

function Format-BuildAge {
    param($When)
    if (-not $When) { return $null }
    $span = (Get-Date) - $When
    $when = $When.ToString('yyyy-MM-dd HH:mm')
    if ($span.TotalMinutes -lt 1)   { return "$when (hozirgina)" }
    if ($span.TotalMinutes -lt 60)  { return "$when ($([int]$span.TotalMinutes) daqiqa oldin)" }
    if ($span.TotalHours -lt 24)    { return "$when ($([int]$span.TotalHours) soat oldin)" }
    return "$when ($([int]$span.TotalDays) kun oldin)"
}

# Bitta ilova uchun: joriy manba versiyasi + oxirgi PROD/DEV build fayli.
function Get-AppVersionInfo {
    param($A)

    $current = Get-AppVersionLabel -AppDir $A.App

    $prodFile = $null
    $devFile  = $null
    if ($A.Kind -eq 'mobile') {
        $prodFile = Get-ChildItem 'F:\OndexProd\apk' -Filter "$($A.App)-*.apk" -ErrorAction SilentlyContinue |
            Sort-Object LastWriteTime -Descending | Select-Object -First 1
        $devPath = Join-Path $Root "apps\$($A.App)\build\app\outputs\flutter-apk\app-debug.apk"
        if (Test-Path $devPath) { $devFile = Get-Item $devPath }
    }
    else {
        $prodPath = Join-Path (Join-Path 'F:\OndexProd' $A.ProdDir) $A.Exe
        if (Test-Path $prodPath) { $prodFile = Get-Item $prodPath }
        $devPath = Join-Path $Root "apps\$($A.App)\build\windows\x64\runner\Debug\$($A.Exe)"
        if (Test-Path $devPath) { $devFile = Get-Item $devPath }
    }

    $prodVer = $null
    if ($prodFile) {
        # Mobil APK nomida versiya bor. Desktop EXE'da yo'q — yonidagi
        # arxivdan olinadi (build.ps1 shu nom bilan yozadi).
        $prodVer = Get-VersionFromFileName -FileName $prodFile.Name
        if (-not $prodVer -and $A.Kind -eq 'desktop') {
            $zipPattern = if ($A.ProdDir -eq 'admin') { 'OnDex-Admin-*.zip' } else { 'OnDex-Restoran-*.zip' }
            $zip = Get-ChildItem 'F:\OndexProd\arxiv' -Filter $zipPattern -ErrorAction SilentlyContinue |
                Sort-Object LastWriteTime -Descending | Select-Object -First 1
            if ($zip) { $prodVer = Get-VersionFromFileName -FileName $zip.Name }
        }
    }

    [pscustomobject]@{
        Label       = $A.Label
        Current     = $current
        ProdVersion = $prodVer
        ProdWhen    = if ($prodFile) { $prodFile.LastWriteTime } else { $null }
        DevWhen     = if ($devFile) { $devFile.LastWriteTime } else { $null }
    }
}

function Invoke-Version {
    Write-Head 'OnDex versiyalari — hammasi'
    $rows = foreach ($a in $VersionApps) {
        $info = Get-AppVersionInfo -A $a
        [pscustomobject]@{
            Ilova                = $info.Label
            'Joriy (manba)'      = $info.Current
            'Oxirgi PROD build'  = if ($info.ProdWhen) { "$($info.ProdVersion)  —  $(Format-BuildAge $info.ProdWhen)" } else { "hali PROD qurilmagan" }
            'Oxirgi DEV build'   = if ($info.DevWhen) { "$($info.Current)-dev  —  $(Format-BuildAge $info.DevWhen)" } else { "hali DEV qurilmagan" }
        }
    }
    $rows | Format-Table -AutoSize -Wrap | Out-String | Write-Host
}

# `ondex -LastVersion` — har bir ilova uchun ENG SO'NGGI qurilgan narsa
# (PROD va DEV orasidan vaqti bo'yicha kattasi), bitta qatorda.
function Invoke-LastVersion {
    # ┌─ FAQAT PROD — DEV BILAN ARALASHTIRILMAYDI ─────────────────────────┐
    # Avval bu yerda "PROD/DEV qaysi kech bo'lsa — o'sha" mantiq bor edi.
    # Natija: bitta ilova DEV qurilgach, PROD ro'yxatiga suqilib kirib,
    # boshqa qatorlar PROD bo'lgan holda o'zi DEV bo'lib chiqardi —
    # ko'rinishda ikkalasi ARALASH edi. Endi `-LastVersion` = doim PROD,
    # `-LastVersionDev` = doim DEV: ikkalasi ham izchil, aralashmaydi.
    # └──────────────────────────────────────────────────────────────────┘
    Write-Head 'Eng oxirgi qurilgan versiya — PROD, har bir ilova'
    $rows = foreach ($a in $VersionApps) {
        $info = Get-AppVersionInfo -A $a
        [pscustomobject]@{
            Ilova    = $info.Label
            Versiya  = if ($info.ProdVersion) { $info.ProdVersion } else { '—' }
            Turi     = 'PROD'
            Qurilgan = if ($info.ProdWhen) { Format-BuildAge $info.ProdWhen } else { 'hali qurilmagan' }
        }
    }
    $rows | Format-Table -AutoSize | Out-String | Write-Host
}

# `ondex -LastVersionDev` — `Invoke-LastVersion` bilan BIR XIL ko'rinish,
# lekin PROD/DEV orasidan tanlamaydi — HAR DOIM DEV build'ni ko'rsatadi
# ("Production'dagi kabi" — foydalanuvchi so'ragan formatning o'zi).
function Invoke-LastVersionDev {
    Write-Head 'Eng oxirgi qurilgan versiya — DEV, har bir ilova'
    $rows = foreach ($a in $VersionApps) {
        $info = Get-AppVersionInfo -A $a
        [pscustomobject]@{
            Ilova    = $info.Label
            Versiya  = "$($info.Current)-dev"
            Turi     = 'DEV'
            Qurilgan = if ($info.DevWhen) { Format-BuildAge $info.DevWhen } else { 'hali qurilmagan' }
        }
    }
    $rows | Format-Table -AutoSize | Out-String | Write-Host
}

# ------------------------------------------------------------- ondex run ---

function Invoke-Run {
    $mode = 'tunnel'
    if ($Local) { $mode = 'local' }
    $cfgName = 'config/dev-tunnel.json'
    if ($mode -eq 'local') { $cfgName = 'config/dev.json' }

    # --- qaysi komponentlar ---
    $selected = $Components
    if ($Only)  { $selected = $Components | Where-Object { $Only  -contains $_.Key } }
    if ($Skip)  { $selected = $selected   | Where-Object { $Skip -notcontains $_.Key } }
    if ($mode -eq 'local') {
        $selected = $selected | Where-Object { $_.Key -ne 'tunnel' }
    }
    if (-not $selected) { Write-Bad 'Hech narsa tanlanmadi. `ondex help` ga qarang.'; return }

    Write-Head "OnDex — $mode rejimi"
    Write-Host "  Loyiha : $Root" -ForegroundColor DarkGray
    Write-Host "  Config : $cfgName" -ForegroundColor DarkGray
    Write-Host ''

    # --- oldindan tekshiruvlar ---
    $device = $null
    $wantsMobile = @($selected | Where-Object { $_.Kind -eq 'flutter-mobile' }).Count -gt 0
    if ($wantsMobile) {
        $device = Get-AndroidDevice
        if (-not $device) {
            Write-Note 'Android qurilma ulanmagan — mobil ilovalar o''tkazib yuboriladi.'
            Write-Note 'USB-debugging yoqilganini tekshiring, keyin: ondex run -Only customer,courier,waiter'
            $selected = $selected | Where-Object { $_.Kind -ne 'flutter-mobile' }
        } else {
            Write-Ok "Android qurilma: $device"
        }
    }

    if ($mode -eq 'local') {
        $ip = Invoke-IpRefresh
        if ($ip) { Write-Ok "LAN IP: $ip" }
        # courier_app da dev.json YO'Q — local rejimda uni ishga tushirib bo'lmaydi.
        if ($selected | Where-Object { $_.Key -eq 'courier' }) {
            Write-Note 'Kuryer ilovasida config/dev.json yo''q (faqat dev-tunnel.json) — o''tkazib yuborildi.'
            $selected = $selected | Where-Object { $_.Key -ne 'courier' }
        }
    }

    $free = [math]::Round((Get-PSDrive -Name $Root.Substring(0,1)).Free / 1GB, 1)
    if ($free -lt 10) { Write-Note "$($Root.Substring(0,1)): diskda faqat $free GB bo'sh — Flutter build'lari to'lib qolishi mumkin." }

    $started = @()

    foreach ($c in $selected) {
        $workDir = $Root
        if ($c.Dir -ne '.') { $workDir = Join-Path $Root $c.Dir }

        switch ($c.Kind) {

            'infra' {
                Write-Head 'Infratuzilma'
                if (-not (Ensure-Docker)) { Write-Bad 'Infra tashlab ketildi — API bazasiz ishga tushmasligi mumkin.'; break }

                # ┌─ KATALOG MONGO'SI — DOCKER'NIKI, WINDOWS XIZMATI EMAS ──┐
                # Kompyuterda mustaqil Windows MongoDB xizmati bor
                # (E:\MongoDBApp) va u 27017'ni band qiladi. U boshqa
                # loyihalarniki — to'xtatilmaydi.
                #
                # Konteyner shu sabab host tomonda 27018'ga chiqariladi
                # va `.env` dagi MONGODB_URI aynan 27018'ga qaraydi.
                # Busiz API Windows xizmatiga ulanardi va katalogdagi
                # 4 ta restoran "yo'qolgandek" ko'rinardi (aslida ular
                # `chustapp_mongodata` volume'ida turgan).
                # └─────────────────────────────────────────────────────┘
                Write-Step 'docker compose up -d db redis mongo meilisearch'
                Push-Location $Root
                try { Invoke-Loud 'docker compose up -d db redis mongo meilisearch' | Out-Null } finally { Pop-Location }

                Wait-Port -Port 5432 -What 'Postgres' -TimeoutSec 90 | Out-Null
                Wait-Port -Port 6380 -What 'Redis'    -TimeoutSec 30 | Out-Null
                Wait-Port -Port 27018 -What 'MongoDB (katalog)' -TimeoutSec 60 | Out-Null
                # MeiliSearch IXTIYORIY: `.env`da `MEILI_HOST` bo'sh
                # bo'lsa ham API tinch ishga tushadi (eski Mongo
                # qidiruviga qaytadi), shuning uchun bu port kutilmaydi —
                # faqat konteyner ko'tarilishi uchun vaqt berildi.
                Wait-Port -Port 7700 -What 'MeiliSearch' -TimeoutSec 30 | Out-Null

                # Ogohlantirish: `.env` hali eski portga qarab tursa,
                # API jimgina BOSHQA bazaga ulanadi va katalog bo'sh
                # ko'rinadi. Bu xato bir marta soatlab izlangan.
                $envFile = Join-Path $Root '.env'
                if (Test-Path $envFile) {
                    $uri = (Get-Content $envFile | Select-String '^MONGODB_URI=').Line
                    if ($uri -and $uri -notmatch '27018') {
                        Write-Bad "MONGODB_URI 27018 ga qaramayapti: $uri"
                        Write-Note 'API Windows MongoDB xizmatiga ulanadi va katalog BO''SH ko''rinadi.'
                    }
                }

                # ┌─ ZAXIRA: BAZALAR KO'TARILGACH, DARROV ──────────────────┐
                # Aynan shu nuqta tanlandi: konteynerlar tirik, lekin API
                # hali yozishni boshlamagan. Ya'ni nusxa ish boshlanishidan
                # OLDINGI toza holatni oladi.
                #
                # Xatolik stekni to'xtatmaydi: zaxira olinmagani ishlashga
                # to'sqinlik qilmasligi kerak. `$ErrorActionPreference`
                # butun skriptda 'Stop' — shuning uchun try/catch SHART.
                # └─────────────────────────────────────────────────────────┘
                if ($NoBackup) {
                    Write-Note '-NoBackup: avtomatik zaxira o''chirilgan.'
                } else {
                    try { Invoke-Backup -Auto }
                    catch { Write-Note "Zaxira olinmadi: $($_.Exception.Message)" }
                }
            }

            'api' {
                Write-Head 'Go API'
                $own = Get-PortOwner -Port 8080
                if ($own) {
                    if ($own.Ours) {
                        Write-Note ":8080 da loyihaning serveri allaqachon ishlayapti (PID $($own.Pid)) — yangisi ochilmadi."
                    } else {
                        Write-Bad ":8080 ni BEGONA jarayon band qilgan: $($own.Name) (PID $($own.Pid))"
                        Write-Note 'Go API ishga TUSHMADI. Portni bo''shating: ondex stop, keyin o''sha jarayonni yoping.'
                    }
                    break
                }
                # CWD loyiha ildizi BO'LISHI SHART: cmd/api `.env` ni
                # `os.ReadFile(".env")` bilan joriy papkadan o'qiydi.
                # API logi faylga ham yoziladi (`logs\api.log`, git'da e'tiborsiz):
                # konsol oynasi yopilsa ham tahlil va o'lchov uchun qoladi
                # (masalan Google Directions so'rovlarini sanash).
                $p = Start-Pane -Key 'api' -Title 'Go API :8080' -WorkDir $Root -Body @(
                    "`$env:API_LOG_FILE = '$Root\logs\api.log'",
                    'go run ./cmd/api'
                )
                $started += [pscustomobject]@{ key = 'api'; pid = $p.Id; name = 'Go API' }
                Wait-Port -Port 8080 -What 'Go API' -TimeoutSec 180 | Out-Null
            }

            'web' {
                Write-Head 'Next.js super-app (Telegram Mini App shu yerda)'
                $own = Get-PortOwner -Port 3000
                if ($own) {
                    if ($own.Ours) {
                        Write-Note ":3000 da loyihaning serveri allaqachon ishlayapti (PID $($own.Pid)) — yangisi ochilmadi."
                    } else {
                        Write-Bad ":3000 ni BEGONA jarayon band qilgan: $($own.Name) (PID $($own.Pid))"
                        Write-Note 'Next.js ishga TUSHMADI, LEKIN tunnel dev-web-ondex ni AYNAN o''sha begona serverga ulaydi.'
                        Write-Note "Yopish: taskkill /PID $($own.Pid) /T /F   keyin: ondex run -Only web"
                    }
                    break
                }
                if (-not (Test-Path (Join-Path $workDir 'node_modules'))) {
                    Write-Note 'node_modules yo''q — npm install ishga tushadi (bir necha daqiqa).'
                    $body = @('npm install', 'npm run dev')
                } else {
                    $body = @('npm run dev')
                }
                $p = Start-Pane -Key 'web' -Title 'Next.js :3000' -WorkDir $workDir -Body $body
                $started += [pscustomobject]@{ key = 'web'; pid = $p.Id; name = 'Next.js' }
                Wait-Port -Port 3000 -What 'Next.js' -TimeoutSec 240 | Out-Null
            }

            'tunnel' {
                Write-Head 'Cloudflare tunnel'
                # Idempotentlik: `ondex run` ni ikkinchi marta yozish
                # IKKINCHI tunnel ochmasligi kerak. Cloudflare bitta
                # nomli tunnelning barcha ulanishlari orasida so'rovni
                # taqsimlaydi — ikkalasi bir joyga qaraganda ishlayveradi,
                # lekin ortiqcha jarayon va chalkash log qoladi.
                $running = @(Get-OndexTunnel)
                if ($running.Count -gt 0) {
                    Write-Note "tunnel allaqachon ishlayapti (PID $($running[0].ProcessId)) — yangisi ochilmadi."
                    break
                }
                $cfg = Join-Path $Root 'scripts\cloudflared\config.yml'
                if (-not (Test-Path $cfg)) { Write-Bad "Tunnel config topilmadi: $cfg"; break }
                if (-not (Get-Command cloudflared -ErrorAction SilentlyContinue)) {
                    Write-Bad 'cloudflared topilmadi. O''rnatish: winget install Cloudflare.cloudflared'
                    break
                }
                # `--config` SHART: busiz BOSHQA loyihaning
                # ~/.cloudflared/config.yml o'qiladi (scripts/cloudflared/config.yml izohiga qarang).
                $p = Start-Pane -Key 'tunnel' -Title 'Cloudflare tunnel' -WorkDir $Root -Body @(
                    'cloudflared --config "' + $cfg + '" tunnel run'
                )
                $started += [pscustomobject]@{ key = 'tunnel'; pid = $p.Id; name = 'Cloudflare tunnel' }
                # Ulanish o'rnatilishini KUTAMIZ va tekshiramiz: jarayon
                # tirik bo'lsa-yu ulanmagan bo'lsa, domenlar 530 qaytaradi
                # va ilova "serverga ulanib bo'lmadi" deydi
                # (Test-TunnelConnected izohiga qarang).
                $connected = $false
                for ($i = 0; $i -lt 6 -and -not $connected; $i++) {
                    Start-Sleep -Seconds 4
                    $connected = Test-TunnelConnected
                }
                if ($connected) {
                    Write-Ok 'https://dev-api-ondex.shoxpro.uz  -> :8080'
                    Write-Ok 'https://dev-web-ondex.shoxpro.uz  -> :3000'
                } else {
                    Write-Bad 'Tunnel ULANMADI (jarayon tirik, lekin Cloudflare''ga ulanish yo''q).'
                    Write-Note 'Domenlar 530 qaytaradi, ilovalar "serverga ulanib bo''lmadi" deydi.'
                    Write-Note 'Yechim: ondex stop, keyin ondex run -Only tunnel (odatda tarmoq/DNS uzilishi).'
                }
            }

            'flutter-desktop' {
                Write-Head $c.Name
                # Idempotentlik, ikki bosqichli tekshiruv:
                #   1) ilova OCHIQmi (EXE jarayoni bor)
                #   2) hali QURILAYAPTImi (oyna tirik, EXE hali yo'q)
                # Ikkinchisi ham muhim: build 1-3 daqiqa oladi va o'sha
                # oraliqda `ondex run` qayta yozilsa, ikkita Flutter
                # bir vaqtda SDK qulfiga to'qnashib IKKALASI ham osilib
                # qolardi (pastdagi `-Stagger` izohiga qarang).
                if ($c.Proc -and (Get-Process -Name $c.Proc -ErrorAction SilentlyContinue)) {
                    Write-Note "allaqachon ochiq — yangisi ishga tushirilmadi."
                    break
                }
                $prev = @(Load-State) | Where-Object { $_.key -eq $c.Key -and $_.pid }
                if ($prev -and (Get-Process -Id $prev[0].pid -ErrorAction SilentlyContinue)) {
                    Write-Note "hali qurilmoqda (oyna PID $($prev[0].pid)) — yangisi ishga tushirilmadi."
                    break
                }
                $body = @()
                if ($PubGet) { $body += 'flutter pub get' }
                $body += "flutter run -d windows --dart-define-from-file=$cfgName"
                $p = Start-Pane -Key $c.Key -Title $c.Name -WorkDir $workDir -Body $body
                $started += [pscustomobject]@{ key = $c.Key; pid = $p.Id; name = $c.Name; proc = $c.Proc }
                Write-Ok 'oyna ochildi (build 1-3 daqiqa; hot reload uchun o''sha oynada `r`)'
                # ┌─ NEGA DESKTOP PANELLAR HAM NAVBAT BILAN ────────────────┐
                # Ikkita `flutter` bir vaqtda ishga tushirilganda Flutter
                # SDK'ning global qulfiga (bin\cache\lockfile) to'qnashadi
                # va IKKALASI ham jimgina osilib qoladi: dart jarayonlari
                # tirik ko'rinadi, lekin C++ kompilyatsiya (cl.exe) umuman
                # boshlanmaydi. Yakka holda o'sha build 24 soniya oladi.
                # └─────────────────────────────────────────────────────────┘
                if ($Stagger -gt 0) {
                    $wait = [math]::Min($Stagger, 30)
                    Write-Step "Flutter SDK qulfini bo'shatish uchun $wait s..."
                    Start-Sleep -Seconds $wait
                }
            }

            'flutter-mobile' {
                Write-Head $c.Name
                $body = @()
                if ($PubGet) { $body += 'flutter pub get' }
                $body += "flutter run -d $device --dart-define-from-file=$cfgName"
                $p = Start-Pane -Key $c.Key -Title $c.Name -WorkDir $workDir -Body $body
                $started += [pscustomobject]@{ key = $c.Key; pid = $p.Id; name = $c.Name }
                Write-Ok "oyna ochildi (qurilma: $device)"
                if ($Stagger -gt 0) {
                    Write-Step "Gradle'ni bo''g''masdan $Stagger s kutilmoqda..."
                    Start-Sleep -Seconds $Stagger
                }
            }
        }
    }

    # Eski yozuvlarni saqlab, yangilarini qo'shamiz (qisman run uchun).
    $startedKeys = @($started | ForEach-Object { $_.key })
    $old = @(Load-State) | Where-Object { $_ -and $_.pid -and ($startedKeys -notcontains $_.key) }
    Save-State (@($old) + @($started))

    Write-Head 'Tayyor'
    Write-Host ''
    Invoke-Status
    Write-Host ''
    Write-Host '  To''xtatish : ' -NoNewline -ForegroundColor DarkGray; Write-Host 'ondex stop' -ForegroundColor White
    Write-Host '  Holat      : ' -NoNewline -ForegroundColor DarkGray; Write-Host 'ondex status' -ForegroundColor White
    Write-Host ''
}

# ---------------------------------------------------------- ondex status ---

function Invoke-Status {
    $rows = @()

    $checks = @(
        @{ N = 'Postgres :5432';       P = 5432 }
        @{ N = 'Redis    :6380';       P = 6380 }
        @{ N = 'MongoDB  :27018 (katalog)'; P = 27018 }
        @{ N = 'Go API   :8080';       P = 8080 }
        @{ N = 'Next.js  :3000';       P = 3000 }
    )
    foreach ($ch in $checks) {
        $up = Test-Port -Port $ch.P
        $s = 'ishlamayapti'
        if ($up) { $s = 'ISHLAYAPTI' }
        $rows += [pscustomobject]@{ Komponent = $ch.N; Holat = $s }
    }

    # `@(...)` SHART: funksiya bitta element qaytarganda PowerShell uni
    # massivdan yechib yuboradi va `.Count` CimInstance'ning o'z a'zosiga
    # tushib qolib $null beradi — tunnel ishlab tursa ham "ishlamayapti"
    # deb ko'rsatardi.
    $cf = @(Get-OndexTunnel)
    $s = 'ishlamayapti'
    if ($cf.Count -gt 0) {
        $s = "ISHLAYAPTI (PID $($cf[0].ProcessId))"
        if (-not (Test-TunnelConnected)) {
            $s = "ULANISH YO'Q (PID $($cf[0].ProcessId)) - qayta ishga tushiring"
        }
    }
    $rows += [pscustomobject]@{ Komponent = 'Cloudflare tunnel (ondex-local)'; Holat = $s }

    # Begona cloudflared'lar haqida OGOHLANTIRISH, lekin ularga tegmaymiz.
    $others = @(Get-CimInstance Win32_Process -Filter "Name='cloudflared.exe'" -ErrorAction SilentlyContinue).Count - $cf.Count
    if ($others -gt 0) {
        $rows += [pscustomobject]@{ Komponent = "  (yana $others ta begona cloudflared - tegilmaydi)"; Holat = '' }
    }

    foreach ($e in (Load-State)) {
        if (-not $e -or -not $e.pid) { continue }
        if ($e.key -in @('api', 'web', 'tunnel')) { continue }
        $alive = Get-Process -Id $e.pid -ErrorAction SilentlyContinue
        $s = 'yopilgan'
        if ($alive) { $s = 'quriladi/ishlayapti (oyna ochiq)' }
        # EXE nomi ma'lum bo'lsa (desktop panellar) — ilovaning O'ZI
        # ochilganini tekshiramiz. Oyna tirikligi build tugaganini
        # bildirmaydi: build osilib qolsa ham oyna ochiq turaveradi.
        if ($e.proc) {
            $app = Get-Process -Name $e.proc -ErrorAction SilentlyContinue
            if ($app) { $s = "OCHIQ (PID $(@($app)[0].Id))" }
            elseif ($alive) { $s = 'quriladi... (ilova hali ochilmadi)' }
        }
        $rows += [pscustomobject]@{ Komponent = $e.name; Holat = $s }
    }

    $rows | Format-Table -AutoSize | Out-String | Write-Host
}

# ------------------------------------------------------------ ondex stop ---

function Invoke-Stop {
    Write-Head 'To''xtatilmoqda'

    foreach ($e in (Load-State)) {
        if (-not $e -or -not $e.pid) { continue }
        $proc = Get-Process -Id $e.pid -ErrorAction SilentlyContinue
        if (-not $proc) { continue }
        # /T — bola jarayonlar bilan birga. `go run` va `npm run dev`
        # haqiqiy ishni BOLA jarayonda bajaradi; faqat oynani o'ldirsak
        # server port'da osilib qolardi.
        Write-Step "$($e.name) (PID $($e.pid))"
        Invoke-Quiet "taskkill /PID $($e.pid) /T /F" | Out-Null
    }

    # Oynasiz qolgan qoldiqlar (oldingi seansdan, holat faylisiz).
    # Portni band qilgan jarayon BIZNIKI ekanini tasdiqlaymiz: buyruq
    # satrida loyiha yo'li bo'lishi shart. Aks holda boshqa loyihaning
    # :3000 da ishlayotgan dev-serverini o'ldirib qo'yish mumkin edi.
    foreach ($port in 8080, 3000) {
        $conn = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue
        if (-not $conn) { continue }
        $procId = $conn[0].OwningProcess
        if (Test-OurProcess -ProcId $procId) {
            Write-Step "port $port bo'shatilmoqda (PID $procId)"
            Invoke-Quiet "taskkill /PID $procId /T /F" | Out-Null
        } else {
            Write-Note "port $port da BEGONA jarayon (PID $procId) — tegilmadi"
        }
    }
    # FAQAT bizning config bilan ishlayotgan tunnel. Boshqa loyihaning
    # tunneli va quick tunnellar tegilmaydi (Get-OndexTunnel izohiga qarang).
    foreach ($t in (Get-OndexTunnel)) {
        Write-Step "cloudflared / ondex-local (PID $($t.ProcessId))"
        Invoke-Quiet "taskkill /PID $($t.ProcessId) /T /F" | Out-Null
    }

    Save-State @()

    # Konteynerlar `stop` qilinadi, `down` EMAS — `down` volume'larni
    # saqlaydi-yu, lekin qayta ko'tarish sekinroq va tarmoqlar qayta
    # yaratiladi. Bazani butunlay tozalash kerak bo'lsa qo'lda:
    #   docker compose down -v
    Write-Step 'docker compose stop'
    Push-Location $Root
    try { Invoke-Quiet 'docker compose stop' | Out-Null } catch { } finally { Pop-Location }

    Write-Ok 'Hammasi to''xtatildi'
    Write-Host ''
}

# ------------------------------------------------------------ ondex help ---

function Invoke-Help {
    Write-Head 'ondex — OnDex dev steki'
    # Bir tirnoqli here-string ('@ ... @'): ichida `$` ham, teskari
    # apostrof ham QOCHIRILMAYDI. Ikki tirnoqlida `flutter` dagi "`f"
    # form-feed belgisiga aylanib matnni buzardi.
    $t = @'
  BUYRUQLAR
    ondex run                 hammasini ishga tushiradi (tunnel rejimi)
    ondex run -Local          tunnelsiz: localhost + LAN IP
    ondex run -Only api,web   faqat sanab o'tilganlar
    ondex run -Skip customer,courier,waiter
    ondex run -PubGet         avval `flutter pub get`
    ondex run -Stagger 0      mobil build'larni birdan (tez, lekin og'ir)
    ondex run -NoBackup       avtomatik zaxirasiz
    ondex run -BackupEvery 0  har safar zaxira (standart: 6 soatda bir)
    ondex stop                hammasini to'xtatadi
    ondex status              nima ishlab turibdi
    ondex backup              Postgres + Mongo nusxasini oladi
    ondex backup -Keep 14     nechta oxirgi nusxa saqlansin (standart 7)

  QURISH — ondex build -Only <nom>
    Bittadan ham, vergul bilan bir nechtadan ham: ondex build -Only OnDex,OnDexGO

    PRODUCTION (versiya +1, telefon ulangan bo'lsa avtomatik o'rnatiladi):
      OnDex          mijoz ilovasi (Android)
      OnDexPro       affitsiant ilovasi (Android)
      OnDexGO        kuryer ilovasi (Android)
      OnDexAdmin     admin paneli (Windows .exe)
      Merchant       restoran paneli (Windows .exe)

    DEV (versiya oshirilmaydi, telefon ulangan bo'lsa avtomatik o'rnatiladi):
      OnDexDev, OnDexProDev, OnDexGoDev, OnDexAdminDev, MerchantDev

    Telefon ulanmagan bo'lsa: build baribir bajariladi, faqat O'RNATILMAYDI
    va aniq ogohlantirish chiqadi (APK yo'li ko'rsatiladi).

    Windows panellarida (OnDexAdmin, Merchant) natija ikki yo'l bilan
    ko'rsatiladi: ochib ishlatish uchun PAPKA va boshqa kompyuterga
    YUBORISH uchun BITTA .zip fayl (F:\OndexProd\arxiv\...).

  VERSIYALAR
    ondex version         5 ta ilova/panel: joriy (manba) versiya +
                           oxirgi PROD va DEV build qachon qurilgani
    ondex -LastVersion     har biri uchun eng oxirgi PROD versiya, 5 ta
                           ilova/panel birga (DEV bilan ARALASHMAYDI)
    ondex -LastVersionDev  xuddi shu ko'rinish, lekin HAR DOIM DEV versiya

  ZAXIRA
    Infra har ko'tarilganda (`ondex run`) o'zi olinadi — 6 soatda bir
    martadan ko'p emas. Nusxalar: .ondex\backup\<sana>\ , yonida
    TIKLASH.txt bilan. Chala nusxa `_notoliq` deb belgilanadi va
    to'liqlarini navbatdan siqib chiqarmaydi.
    ondex ip                  Wi-Fi IP ni dev.json larga yozadi
    ondex help                shu matn

  KOMPONENT KALITLARI
    infra       Docker: Postgres + Redis + MeiliSearch (Mongo — Windows xizmati)
    api         Go API                          :8080
    web         Next.js super-app + Telegram Mini App   :3000
    tunnel      Cloudflare `ondex-local`
    admin       Admin panel        (Flutter Windows)
    restaurant  Restoran paneli    (Flutter Windows)
    customer    Mijoz ilovasi      (Android)
    courier     Kuryer ilovasi     (Android)
    waiter      Afitsiant ilovasi  (Android)

  MANZILLAR (tunnel rejimi)
    https://dev-api-ondex.shoxpro.uz    Go API
    https://dev-web-ondex.shoxpro.uz    web / Telegram Mini App

  DIQQAT
    Tunnel ishlab turganda dev API OMMAVIY internetda bo'ladi va .env
    dagi HAQIQIY Telegram/Resend/R2 kalitlari ishlatiladi. Ish tugagach
    `ondex stop` bilan yoping.
'@
    Write-Host $t
    Write-Host ''
}

# ---------------------------------------------------------------- dispatch --

# `-LastVersion(Dev)` $Command'dan QAT'IY NAZAR ustuvor — buyruqsiz
# (standart $Command='run' bilan) ham to'g'ri ishlashi uchun.
if ($LastVersionDev) {
    Invoke-LastVersionDev
}
elseif ($LastVersion) {
    Invoke-LastVersion
}
else {
    switch ($Command) {
        'run'     { Invoke-Run }
        'stop'    { Invoke-Stop }
        'status'  { Write-Head 'OnDex holati'; Invoke-Status }
        'ip'      { Write-Head 'LAN IP yangilanmoqda'; Invoke-IpRefresh | Out-Null }
        'backup'  { Invoke-Backup }
        'build'   { Invoke-Build }
        'version' { Invoke-Version }
        'help'    { Invoke-Help }
    }
}

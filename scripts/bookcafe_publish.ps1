# bookcafe_publish.ps1 — Book Cafe 3D maketini yangilash.
#
# ┌─ NIMA UCHUN SKRIPT ────────────────────────────────────────────────┐
# Yangilash ketma-ketligi uzun va uning HAR BIR qadami jimgina xato
# berishi mumkin:
#
#   - Godot eksporti skript xatosida ham `EXIT 0` qaytaradi. Bir marta
#     shu sababli ikkita qurilish behuda ketgan: `TouchController`
#     umuman yuklanmagan, ilova esa "ishladi" deb ko'ringan.
#   - PCK ni eski nom bilan yuklash Cloudflare keshiga tushadi va
#     telefon ESKI paketni oladi.
#   - Xesh yangilanmasa ilova faylni "buzilgan" deb rad etadi.
#
# Skript shu uchtasini ham hal qiladi: xatoni ushlaydi, har safar
# yangi nom beradi, xesh va manzilni bitta amalda yozadi.
# └────────────────────────────────────────────────────────────────────┘
#
# ISHLATISH
#
#   Maket o'zgardi (Godot'da tahrirlab Ctrl+S bosilgan):
#       .\scripts\bookcafe_publish.ps1 -Scene
#
#   Modul kodi o'zgardi (touch_controller.gd, collision_builder.gd ...):
#       .\scripts\bookcafe_publish.ps1 -Modules
#
#   Ikkalasi:
#       .\scripts\bookcafe_publish.ps1 -Scene -Modules
#
#   Telefonga o'rnatmasdan (faqat qurish):
#       .\scripts\bookcafe_publish.ps1 -Modules -NoInstall

param(
    # Maketning o'zi o'zgarganda: PCK qayta yig'iladi va R2 ga chiqadi.
    [switch]$Scene,

    # GDScript modullari o'zgarganda: ular ilova ichida turadi,
    # shuning uchun APK qayta quriladi.
    [switch]$Modules,

    [switch]$NoInstall,

    # ┌─ QAYSI SERVERGA QARAB QURILADI ────────────────────────────────┐
    # Standart — PRODUCTION. Ilgari bu yerda tanlov yo'q edi va APK
    # DOIM `dev-tunnel.json` bilan qurilardi, ya'ni telefondagi ilova
    # `dev-api-ondex.shoxpro.uz` bilan gaplashardi.
    #
    # Tunnel o'chirilgach o'sha manzil 530 qaytara boshladi va ikki
    # nosozlik kelib chiqdi: kitob menyusi ochilmadi, hamda ilova
    # maketning yangi `sha256` ini ololmay ESKI paketni ishlatishda
    # davom etdi. Ekranda esa bu "tuzatishlar yetib kelmadi" bo'lib
    # ko'rinardi.
    #
    # Dev tunnelda sinash uchun: `-Dev`.
    # └────────────────────────────────────────────────────────────────┘
    [switch]$Dev,

    # Maket bog'langan restoran.
    #
    # Avvalgi qiymat (`14785d52cd5d9e37`) mavjud emas — u bilan maket
    # HECH KIMGA bog'lanmasdi. Book Cafe ning haqiqiy ID si:
    [string]$RestaurantId = "bdb543b851659709"
)

$ErrorActionPreference = "Stop"

$Godot   = "D:\UnityHub\Godot_v4.7.2-stable_win64.exe\Godot_v4.7.2-stable_win64_console.exe"
$Project = "D:\UnityHub\book-cafe-mobile"
$Boot    = "D:\UnityHub\bookcafe-boot"
$App     = "F:\ChustApp\apps\customer_app"
$Assets  = "$App\android\app\src\main\assets"
# Android SDK dagi to'liq adb. GlideX nusxasi ISHLATILMAYDI: ikkita
# har xil versiyadagi adb bir-birining serverini o'ldiradi va qurilma
# ro'yxatdan tasodifiy yo'qolib turadi.
$Adb     = "E:\Sdk\platform-tools\adb.exe"
$Repo    = "F:\ChustApp"

# Ikkala loyihada bir xil turishi kerak bo'lgan modullar.
#
# ┌─ RO'YXATDA YO'Q FAYL JIMGINA ESKI QOLADI ──────────────────────────┐
# Kutubxona fayllari (`book_shelf.gd`, `book_api.gd`, `ui_kit.gd`) bu
# yerga qo'shilmagan edi. Ular loader loyihasiga bir marta QO'LDA
# ko'chirilgan va shundan keyin hech qachon yangilanmagan: manba
# o'zgarardi, telefonga esa eski nusxa borardi va sabab ko'rinmasdi.
#
# Yangi modul qo'shganda uni SHU YERGA ham qo'shish shart.
# └────────────────────────────────────────────────────────────────────┘
$SharedModules = @("touch_controller.gd", "collision_builder.gd",
                   "table_menu.gd", "ondex_config.gd", "ondex_api.gd",
                   "image_cache.gd", "menu_icons.gd", "ondex_boot.gd",
                   "mobile_quality.gd", "fps_probe.gd",
                   "ui_kit.gd", "book_api.gd", "book_shelf.gd")

function Say([string]$text, [string]$color = "Cyan") {
    Write-Host ""
    Write-Host "  $text" -ForegroundColor $color
}

# ┌─ TASHQI BUYRUQNI CHAQIRISH ────────────────────────────────────────┐
# `$ErrorActionPreference = "Stop"` bilan birga PowerShell tashqi
# dasturning STDERR ga yozgan HAR BIR qatorini xato deb hisoblaydi va
# skriptni to'xtatadi - dastur muvaffaqiyatli tugagan bo'lsa ham.
#
# `flutter` esa har qurishda ogohlantirish yozadi ("Your app uses the
# following plugins that apply Kotlin Gradle Plugin..."). Natijada
# APK butunlay to'g'ri qurilardi-yu, skript o'sha qatorda yiqilardi va
# undan keyingi TEKSHIRUVLAR umuman ishlamasdi.
#
# Yagona ishonchli o'lchov - chiqish kodi. Shuning uchun tashqi
# buyruqlar shu yordamchi orqali chaqiriladi.
# └────────────────────────────────────────────────────────────────────┘
function Invoke-Native {
    param([Parameter(Mandatory = $true)][scriptblock]$Command)
    $old = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    # `Out-Host` SHART: usiz buyruqning butun chiqishi ham qaytariladi
    # va chaqiruvchi chiqish kodi o'rniga MASSIV oladi. O'shanda
    # `if ($code -ne 0)` doim rost bo'lib, muvaffaqiyatli qurilish ham
    # "yiqildi" deb belgilanardi.
    try { & $Command | Out-Host } finally { $ErrorActionPreference = $old }
    return $LASTEXITCODE
}

if (-not $Scene -and -not $Modules) {
    Say "-Scene yoki -Modules bering (yoki ikkalasini)." "Yellow"
    Write-Host "  Maket o'zgardi   -> -Scene"
    Write-Host "  Kod o'zgardi     -> -Modules"
    exit 1
}

# ── 1. MAKET ────────────────────────────────────────────────────────
if ($Scene) {
    Say "1/4  Maket paketi yig'ilmoqda (PCK)..."

    # Sahna saqlanganmi. Godot muharriri ochiq bo'lsa-yu, Ctrl+S
    # bosilmagan bo'lsa, eski nusxa paketlanardi va "yana ishlamadi"
    # degan holat kelib chiqardi.
    $scenePath = "$Project\book_cafe_scene.tscn"
    $age = (Get-Date) - (Get-Item $scenePath).LastWriteTime
    Write-Host ("     sahna oxirgi saqlangan: {0:N0} daqiqa oldin" -f $age.TotalMinutes)

    # ┌─ HAR EKSPORT — YANGI FAYL ─────────────────────────────────────┐
    # Avval nom qat'iy edi (`bookcafe.pck`) va eskisi o'chirilardi.
    # Lekin yuklash 10-15 daqiqa davom etadi va o'sha paytda fayl BAND
    # bo'ladi. Skript qayta ishga tushirilsa, o'chirish
    # "used by another process" xatosi bilan yiqilardi - sababi esa
    # xabardan umuman tushunarli emas edi.
    #
    # Sana-vaqtli nom bu to'qnashuvni butunlay yo'q qiladi: ishlayotgan
    # yuklash o'z faylini ushlab turaveradi, yangi eksport boshqasiga
    # yoziladi.
    # └────────────────────────────────────────────────────────────────┘
    $stamp = Get-Date -Format "yyyyMMdd-HHmm"
    $pck = "$Project\build\bookcafe-$stamp.pck"
    New-Item -ItemType Directory -Force "$Project\build" | Out-Null

    # Ayni paytda yuklash ketayotgan bo'lsa - ogohlantiramiz. Ikkita
    # yuklash bir vaqtda ketsa, qaysi biri oxirida yozilishi noaniq
    # bo'lardi va restoranga ESKI paket bog'lanib qolishi mumkin edi.
    $busy = Get-Process -Name "r2upload" -ErrorAction SilentlyContinue
    if ($busy) {
        $mins = ((Get-Date) - $busy[0].StartTime).TotalMinutes
        Say "TO'XTATILDI: oldingi yuklash hali tugamagan" "Yellow"
        Write-Host ("     r2upload PID {0}, {1:N0} daqiqadan beri ishlayapti" -f $busy[0].Id, $mins)
        Write-Host "     110 MB odatda 10-15 daqiqa oladi - kuting."
        Write-Host "     Holatni ko'rish:"
        Write-Host "       cd F:\ChustApp; go run ./cmd/r2upload -check $RestaurantId"
        exit 1
    }
    $out = & $Godot --headless --path $Project --export-pack "Android arm64" $pck 2>&1
    $bad = $out | Select-String -Pattern "SCRIPT ERROR|Parse Error|Failed to|ERROR:"
    if ($bad) {
        Say "XATO: eksportda muammo bor, to'xtatildi" "Red"
        $bad | Select-Object -First 8 | ForEach-Object { Write-Host "     $($_.Line)" }
        exit 1
    }
    if (-not (Test-Path $pck)) { Say "XATO: PCK yasalmadi" "Red"; exit 1 }
    Write-Host ("     PCK: {0:N1} MB" -f ((Get-Item $pck).Length / 1MB))

    Say "2/4  R2 ga yuklanmoqda va restoranga bog'lanmoqda..."

    # ┌─ HAR SAFAR YANGI NOM ──────────────────────────────────────────┐
    # Eski nomga qayta yozilsa Cloudflare keshi eski nusxani berishda
    # davom etadi va telefonga o'zgarish yetib bormaydi. Sana-vaqtli
    # nom buni butunlay chetlab o'tadi.
    # └────────────────────────────────────────────────────────────────┘
    $key = "bookcafe/bookcafe-$stamp.pck"
    Write-Host "     nom: $key"
    Write-Host "     110 MB yuklash odatda 10-15 daqiqa - oyna yopilmasin."

    # ┌─ MAKET ENDI YOPIQ BUCKET'DA ───────────────────────────────────┐
    # `cmd/r2upload` `R2_BUCKET` ga - OMMAVIY media bucket'iga yozadi.
    # Maket esa `ondexscenes` ga ko'chirilgan va u yerdan faqat
    # imzolangan, muddatli havola bilan beriladi: kirgan foydalanuvchi
    # ko'radi, lekin faylni o'ziga yuklab ololmaydi.
    #
    # Ya'ni bu vositani hozir ishlatish maketni yana OMMAVIY qilib
    # qo'yardi - qilingan ish bekor bo'lardi. Shuning uchun qadam
    # to'xtatiladi va to'g'ri yo'l ko'rsatiladi.
    #
    # r2upload yopiq bucket'ni qo'llagach shu blok olib tashlanadi.
    # └────────────────────────────────────────────────────────────────┘
    Say "PCK tayyor. Yuklash QO'LDA bajariladi:" "Yellow"
    Write-Host "     1) $pck"
    Write-Host "        faylini R2 -> ondexscenes -> scenes/ ichiga yuklang"
    Write-Host "     2) serverda:  ssh shajara  ->  bash ~/ondex/sync-scene.sh"
    Write-Host ""
    Write-Host "     (sync-scene.sh eng yangi .pck ni topib, bazadagi manzil,"
    Write-Host "      xesh va hajmni yangilaydi hamda Redis keshini tozalaydi)"

    # Eski paketlar yig'ilib qolmasin: har biri ~110 MB. Oxirgi
    # uchtasi qoldiriladi - kerak bo'lsa qaytish uchun.
    Get-ChildItem "$Project\build" -Filter "bookcafe-*.pck" |
        Sort-Object LastWriteTime -Descending |
        Select-Object -Skip 3 |
        ForEach-Object {
            try { Remove-Item $_.FullName -Force -ErrorAction Stop }
            catch { }   # band bo'lsa keyingi safar o'chadi
        }

    Say "Maket yangilandi. Telefonda ilovani YOPIB-OCHING - yangi paket o'zi yuklanadi." "Green"
}

# ── 2. MODULLAR ─────────────────────────────────────────────────────
if ($Modules) {
    Say "3/4  Modullar yuklovchi loyihaga ko'chirilmoqda..."

    # Modullar IKKI joyda turadi: maket loyihasida (ish stolida sinash
    # uchun) va yuklovchida (telefonda AYNAN shulari ishlaydi, chunki
    # paket `replace_files = false` bilan ochiladi). Nusxalash
    # unutilsa, telefon eski kodni ishlatib turaveradi.
    foreach ($m in $SharedModules) {
        $src = "$Project\$m"
        if (Test-Path $src) {
            Copy-Item $src "$Boot\$m" -Force
            Write-Host "     $m"
        }
    }

    $out = & $Godot --headless --path $Boot --export-debug "Android arm64" "$Boot\build\boot.apk" 2>&1

    # ┌─ ENG MUHIM TEKSHIRUV ──────────────────────────────────────────┐
    # Godot skript xatosida ham 0 qaytaradi. Tekshirilmasa, autoload
    # jimgina yuklanmay qoladi va o'yinda boshqaruv butunlay o'lik
    # bo'ladi - lekin qurilish "muvaffaqiyatli" ko'rinadi.
    # └────────────────────────────────────────────────────────────────┘
    $bad = $out | Select-String -Pattern "SCRIPT ERROR|Parse Error|Failed to|ERROR:"
    if ($bad) {
        Say "XATO: GDScript xatosi - qurilish TO'XTATILDI" "Red"
        $bad | Select-Object -First 8 | ForEach-Object { Write-Host "     $($_.Line)" }
        exit 1
    }
    Write-Host "     eksport toza - skript xatosi yo'q"

    # Yuklovchining fayllari ilova ichiga ko'chiriladi.
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $zip = [IO.Compression.ZipFile]::OpenRead("$Boot\build\boot.apk")
    try {
        foreach ($e in $zip.Entries) {
            if (-not $e.FullName.StartsWith("assets/") -or $e.Name -eq "") { continue }
            $target = Join-Path $Assets $e.FullName.Substring(7).Replace("/", "\")
            $dir = Split-Path $target -Parent
            if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Force $dir | Out-Null }
            [IO.Compression.ZipFileExtensions]::ExtractToFile($e, $target, $true)
        }
    } finally { $zip.Dispose() }

    Say "4/4  Ilova qurilmoqda (~1-2 daqiqa)..."
    Push-Location $App
    try {
        $cfg = if ($Dev) { "config/dev-tunnel.json" } else { "config/prod.json" }
        Write-Host "     sozlama: $cfg"
        $code = Invoke-Native { flutter build apk --release --dart-define-from-file=$cfg }
        if ($code -ne 0) { Say "XATO: flutter build yiqildi (kod $code)" "Red"; exit 1 }
    } finally { Pop-Location }

    $apk = "$App\build\app\outputs\flutter-apk\app-release.apk"
    Write-Host ("     APK: {0:N1} MB" -f ((Get-Item $apk).Length / 1MB))

    # ┌─ QURILGAN APK HAQIQATDA QAYERGA QARAYAPTI ─────────────────────┐
    # `--dart-define-from-file` jimgina e'tiborsiz qolishi mumkin
    # (fayl nomi xato, ish papkasi boshqa). Shunda build "muvaffaqiyatli"
    # tugaydi-yu, ilova ichida standart `http://localhost:8080` qoladi
    # va telefonda hech narsa ishlamaydi.
    #
    # Shuning uchun manzil APK NING O'ZIDAN tekshiriladi: Dart kodi
    # `lib/arm64-v8a/libapp.so` ichida yotadi va satr o'sha yerda
    # o'zgarishsiz saqlanadi.
    # └────────────────────────────────────────────────────────────────┘
    $want = if ($Dev) { "dev-api-ondex.shoxpro.uz" } else { "api.ondex.uz" }
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $z = [IO.Compression.ZipFile]::OpenRead($apk)
    try {
        $e = $z.Entries | Where-Object { $_.FullName -match 'libapp\.so$' } | Select-Object -First 1
        if ($null -eq $e) { Say "XATO: APK ichida libapp.so yo'q" "Red"; exit 1 }
        $ms = New-Object System.IO.MemoryStream
        $s = $e.Open(); $s.CopyTo($ms); $s.Close()
        $bytes = $ms.ToArray(); $ms.Dispose()
    } finally { $z.Dispose() }

    $pin = [System.Text.Encoding]::UTF8.GetBytes($want)
    $found = $false
    $last = $bytes.Length - $pin.Length
    for ($i = 0; $i -le $last -and -not $found; $i++) {
        if ($bytes[$i] -ne $pin[0]) { continue }
        $ok = $true
        for ($j = 1; $j -lt $pin.Length; $j++) {
            if ($bytes[$i + $j] -ne $pin[$j]) { $ok = $false; break }
        }
        if ($ok) { $found = $true }
    }
    if (-not $found) {
        Say "XATO: APK ichida '$want' topilmadi - sozlama qo'llanmagan" "Red"
        Write-Host "     Ish papkasi va config fayl nomini tekshiring."
        exit 1
    }
    Write-Host "     tekshirildi: APK '$want' ga qarayapti"

    if ($NoInstall) {
        Say "Qurildi. O'rnatish o'tkazib yuborildi (-NoInstall)." "Green"
    } else {
        # ┌─ IP QOTIRIB YOZILMAYDI ────────────────────────────────────┐
        # Wi-Fi orqali ulanish tez-tez uziladi va telefon yangi IP
        # oladi (router DHCP). Manzil kodga yozilgan edi va u
        # o'zgargach o'rnatish jimgina ishlamay qoldi.
        #
        # Endi oxirgi ishlagan manzil yozib boriladi va qayta
        # ulanishda o'sha ishlatiladi.
        # └────────────────────────────────────────────────────────────┘
        $ipFile = Join-Path $PSScriptRoot ".bookcafe_device"
        if ((& $Adb devices | Select-String "device$").Count -eq 0) {
            if (Test-Path $ipFile) {
                & $Adb connect (Get-Content $ipFile -Raw).Trim() | Out-Null
                Start-Sleep -Milliseconds 1500
            }
        }
        # Ulangan manzilni eslab qolamiz.
        $live = & $Adb devices | Select-String "^(\S+)\s+device$"
        if ($live) {
            $live.Matches.Groups[1].Value | Set-Content $ipFile -Encoding ascii
        }
        if ((& $Adb devices | Select-String "device$").Count -eq 0) {
            Say "Qurilma ulanmagan - APK tayyor, keyin o'rnating." "Yellow"
        } else {
            $code = Invoke-Native { & $Adb install -r $apk }
            if ($code -ne 0) { Say "XATO: o'rnatilmadi (kod $code)" "Red"; exit 1 }
            Say "O'rnatildi." "Green"
        }
    }
}

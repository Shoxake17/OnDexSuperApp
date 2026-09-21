<#
.SYNOPSIS
    Android imzolash keystore'ining PAROLINI almashtiradi (bug.md 59-band).

.DESCRIPTION
    Joriy parol — `Parol.2003` — lug'at hujumiga ochiq. `.jks` faylini
    qo'lga kiritgan odam (zaxira nusxa, o'g'irlangan noutbuk, zararli
    dastur) uni bir necha soniyada topadi.

    ┌─ MUHIM: KALITNING O'ZI O'ZGARMAYDI ────────────────────────────┐
    Bu skript FAQAT parolni (himoya qatlamini) almashtiradi.
    Sertifikat, ochiq kalit va SHA-1/SHA-256 barmoq izlari
    O'ZGARISHSIZ qoladi. Ya'ni:

      * Play Store upload key AMAL QILISHDA DAVOM ETADI;
      * Google Maps kalitidagi SHA-1 cheklovi buzilmaydi;
      * mavjud ilovalarga yangilanish chiqarish mumkin bo'lib qoladi.

    Agar KALITNING O'ZI almashtirilsa — bu butunlay boshqa,
    ancha og'ir jarayon (Play Console orqali "upload key reset").
    Bu skript unga TEGMAYDI.
    └────────────────────────────────────────────────────────────────┘

.PARAMETER KeystorePath
    `.jks` fayl yo'li. Berilmasa `key.properties` dan o'qiladi.

.PARAMETER NewPassword
    Yangi parol. Berilmasa — 32 belgili tasodifiy parol yaratiladi
    va ekranda BIR MARTA ko'rsatiladi.

.EXAMPLE
    .\scripts\rotate-keystore-password.ps1

.NOTES
    Skript hech narsani jimgina qilmaydi: har qadamda nima
    bo'layotganini yozadi va oxirida yangi parol bilan keystore'ni
    HAQIQATAN ochib ko'radi (tekshiruvsiz "bajarildi" demaydi).
#>
[CmdletBinding()]
param(
    [string]$KeystorePath,
    [string]$NewPassword
)

$ErrorActionPreference = 'Stop'

# ── Yo'llar ──────────────────────────────────────────────────────────
$repoRoot = Split-Path -Parent $PSScriptRoot
$propsFiles = @(
    "$repoRoot\apps\customer_app\android\key.properties"
    "$repoRoot\apps\courier_app\android\key.properties"
    "$repoRoot\apps\waiter_app\android\key.properties"
) | Where-Object { Test-Path $_ }

if ($propsFiles.Count -eq 0) {
    throw "key.properties topilmadi. Ilovalar android/ papkasida bo'lishi kerak."
}

function Read-Props($path) {
    $out = @{}
    foreach ($line in Get-Content $path) {
        $line = $line.Trim()
        if ($line -eq '' -or $line.StartsWith('#')) { continue }
        $i = $line.IndexOf('=')
        if ($i -lt 1) { continue }
        $out[$line.Substring(0, $i).Trim()] = $line.Substring($i + 1).Trim()
    }
    return $out
}

$first = Read-Props $propsFiles[0]
if (-not $KeystorePath) { $KeystorePath = $first['storeFile'] }
$alias       = $first['keyAlias']
$oldStorePwd = $first['storePassword']
$oldKeyPwd   = $first['keyPassword']

if (-not (Test-Path $KeystorePath)) {
    throw "Keystore topilmadi: $KeystorePath"
}

# `keytool` — JDK bilan keladi. Flutter o'z JDK'sini olib yuradi.
$keytool = (Get-Command keytool -ErrorAction SilentlyContinue).Source
if (-not $keytool) {
    $flutter = (Get-Command flutter -ErrorAction SilentlyContinue).Source
    if ($flutter) {
        $candidate = Join-Path (Split-Path -Parent (Split-Path -Parent $flutter)) 'bin\keytool.exe'
        if (Test-Path $candidate) { $keytool = $candidate }
    }
}
if (-not $keytool) {
    throw "keytool topilmadi. JDK o'rnating yoki PATH ga qo'shing (Android Studio bilan keladi)."
}

Write-Host "Keystore : $KeystorePath"
Write-Host "Alias    : $alias"
Write-Host "keytool  : $keytool"
Write-Host ""

# ── 1. Zaxira nusxa ──────────────────────────────────────────────────
# ENG MUHIM QADAM: parol almashtirish yiqilsa, asl fayl qoladi.
$stamp  = Get-Date -Format 'yyyyMMdd-HHmmss'
$backup = "$KeystorePath.$stamp.bak"
Copy-Item $KeystorePath $backup
Write-Host "[1/5] Zaxira nusxa: $backup" -ForegroundColor Green

# ── 2. Joriy parol to'g'riligini TEKSHIRISH ──────────────────────────
# Almashtirishga urinishdan OLDIN: noto'g'ri parol bilan `keytool`
# faylni buzmaydi, lekin sababi tushunarsiz xato beradi.
& $keytool -list -keystore $KeystorePath -storepass $oldStorePwd -alias $alias > $null 2>&1
if ($LASTEXITCODE -ne 0) {
    throw "Joriy parol noto'g'ri yoki alias topilmadi. key.properties tekshiring."
}
Write-Host "[2/5] Joriy parol tasdiqlandi" -ForegroundColor Green

# ── 3. Yangi parol ───────────────────────────────────────────────────
if (-not $NewPassword) {
    # 32 belgi, kriptografik tasodifiy. Ikki xil belgi turkumi —
    # ba'zi vositalar maxsus belgilarni qo'shtirnoqsiz o'qiy olmaydi,
    # shuning uchun faqat harf/raqam.
    $bytes = New-Object byte[] 24
    [System.Security.Cryptography.RandomNumberGenerator]::Fill($bytes)
    $NewPassword = [Convert]::ToBase64String($bytes) -replace '[^A-Za-z0-9]', ''
    if ($NewPassword.Length -lt 24) { throw "Parol yaratishda xato — qayta urining." }
}
if ($NewPassword.Length -lt 16) {
    throw "Parol juda qisqa (kamida 16 belgi). Zaif parol bu bandning MAQSADIGA zid."
}

# ── 4. Almashtirish ──────────────────────────────────────────────────
# TARTIB MUHIM: avval KALIT paroli, keyin OMBOR paroli. Teskarisida
# `-keypasswd` eski ombor parolini so'rab, ishlamay qolardi.
& $keytool -keypasswd -keystore $KeystorePath -storepass $oldStorePwd `
    -alias $alias -keypass $oldKeyPwd -new $NewPassword
if ($LASTEXITCODE -ne 0) {
    Copy-Item $backup $KeystorePath -Force
    throw "Kalit paroli almashmadi — zaxiradan tiklandi. Hech narsa o'zgarmadi."
}

& $keytool -storepasswd -keystore $KeystorePath -storepass $oldStorePwd -new $NewPassword
if ($LASTEXITCODE -ne 0) {
    Copy-Item $backup $KeystorePath -Force
    throw "Ombor paroli almashmadi — zaxiradan tiklandi. Hech narsa o'zgarmadi."
}
Write-Host "[3/5] Parollar almashtirildi" -ForegroundColor Green

# ── 5. TEKSHIRUV ─────────────────────────────────────────────────────
# "Bajarildi" deyishdan oldin keystore YANGI parol bilan HAQIQATAN
# ochilishini ko'ramiz.
& $keytool -list -keystore $KeystorePath -storepass $NewPassword -alias $alias > $null 2>&1
if ($LASTEXITCODE -ne 0) {
    Copy-Item $backup $KeystorePath -Force
    throw "TEKSHIRUV YIQILDI — zaxiradan tiklandi. Hech narsa o'zgarmadi."
}
Write-Host "[4/5] Yangi parol bilan keystore ochildi (tekshirildi)" -ForegroundColor Green

# ── 6. key.properties fayllarini yangilash ───────────────────────────
foreach ($f in $propsFiles) {
    $text = Get-Content $f -Raw
    $text = $text -replace '(?m)^storePassword=.*$', "storePassword=$NewPassword"
    $text = $text -replace '(?m)^keyPassword=.*$',   "keyPassword=$NewPassword"
    # BOM'siz UTF-8: Gradle BOM bilan faylni noto'g'ri o'qiydi.
    [System.IO.File]::WriteAllText($f, $text, (New-Object System.Text.UTF8Encoding($false)))
    Write-Host "      yangilandi: $f"
}
Write-Host "[5/5] key.properties fayllari yangilandi" -ForegroundColor Green

Write-Host ""
Write-Host "════════════════════════════════════════════════════════════" -ForegroundColor Yellow
Write-Host " YANGI PAROL (BIR MARTA ko'rsatiladi — parol menejeriga saqlang):" -ForegroundColor Yellow
Write-Host ""
Write-Host "   $NewPassword" -ForegroundColor Cyan
Write-Host ""
Write-Host " QOLGAN QADAMLAR — QO'LDA:" -ForegroundColor Yellow
Write-Host "   1. GitHub -> Settings -> Secrets and variables -> Actions:"
Write-Host "        ANDROID_KEYSTORE_PASSWORD  = yangi parol"
Write-Host "        ANDROID_KEY_PASSWORD       = yangi parol"
Write-Host "        ANDROID_KEYSTORE_BASE64    = QAYTA yarating (fayl o'zgardi):"
Write-Host "          [Convert]::ToBase64String([IO.File]::ReadAllBytes('$KeystorePath')) | Set-Clipboard"
Write-Host "   2. Release build bilan sinang:"
Write-Host "        cd apps\customer_app; flutter build appbundle --release"
Write-Host "   3. Build muvaffaqiyatli bo'lgach zaxirani XAVFSIZ joyga ko'chiring:"
Write-Host "        $backup"
Write-Host "      (unda ESKI parol ishlaydi — uni diskda qoldirmang)"
Write-Host "════════════════════════════════════════════════════════════" -ForegroundColor Yellow

# Ilova versiyasini oshiradi — pubspec va config fayllarini BIRGA.
#
# ┌─ NEGA SKRIPT KERAK ────────────────────────────────────────────────┐
# Versiya IKKI joyda yashaydi va ular bir-biridan xabarsiz:
#
#   pubspec.yaml `version:`      -> Android `versionCode`/`versionName`
#   config/*.json ONDEX_APP_VERSION -> ilovada KO'RINADIGAN matn
#                                      (`ondex_core/config.dart`)
#
# Ular qo'lda yangilanganda birinchisi esdan chiqadi va ikkita build
# bir xil `versionCode` bilan chiqadi (Android eskisini yangisidan
# ajrata olmaydi), yoki ekranda eski raqam turadi. Ikkalasi bitta
# buyruq bilan o'zgarsa, chalkashlik imkoni yo'qoladi.
# └────────────────────────────────────────────────────────────────────┘
#
# Ishlatish:
#   .\scripts\bump-version.ps1                     # customer, build +1
#   .\scripts\bump-version.ps1 -App courier
#   .\scripts\bump-version.ps1 -Part minor         # 0.2.5+7 -> 0.3.0+8
#   .\scripts\bump-version.ps1 -DryRun             # faqat ko'rsatadi

[CmdletBinding()]
param(
    [ValidateSet('customer', 'courier', 'waiter', 'admin', 'restaurant')]
    [string]$App = 'customer',

    # build — faqat `+N` oshadi (kundalik build uchun).
    # patch/minor/major — semver qismi oshadi, `+N` HAR DOIM oshadi.
    [ValidateSet('build', 'patch', 'minor', 'major')]
    [string]$Part = 'build',

    [switch]$DryRun
)

$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent $PSScriptRoot

$Dirs = @{
    customer   = 'apps\customer_app'
    courier    = 'apps\courier_app'
    waiter     = 'apps\waiter_app'
    admin      = 'apps\admin_panel'
    restaurant = 'apps\restaurant_panel'
}

$appDir = Join-Path $Root $Dirs[$App]
$pubspec = Join-Path $appDir 'pubspec.yaml'
if (-not (Test-Path $pubspec)) { throw "pubspec topilmadi: $pubspec" }

# ── UTF-8 (BOM'siz) yozish ──
#
# `Set-Content` tizim kodlashiga o'tadi va o'zbekcha izohlarni buzadi;
# `-Encoding utf8` esa BOM qo'shadi va Flutter YAML'ni o'qiy olmay
# qoladi. Shu sabab .NET orqali.
$utf8 = New-Object System.Text.UTF8Encoding($false)
function Save-Text([string]$Path, [string]$Text) {
    if ($DryRun) { return }
    [System.IO.File]::WriteAllText($Path, $Text, $utf8)
}

# ── pubspec ──

$raw = [System.IO.File]::ReadAllText($pubspec)
$m = [regex]::Match($raw, '(?m)^version:\s*(\d+)\.(\d+)\.(\d+)(?:\+(\d+))?\s*$')
if (-not $m.Success) { throw "pubspec.yaml da `version:` qatori topilmadi yoki shakli notanish" }

$major = [int]$m.Groups[1].Value
$minor = [int]$m.Groups[2].Value
$patch = [int]$m.Groups[3].Value
$build = if ($m.Groups[4].Success) { [int]$m.Groups[4].Value } else { 0 }

$old = "$major.$minor.$patch" + $(if ($m.Groups[4].Success) { "+$build" } else { '' })

switch ($Part) {
    'major' { $major++; $minor = 0; $patch = 0 }
    'minor' { $minor++; $patch = 0 }
    'patch' { $patch++ }
}
# `versionCode` HAR DOIM oshadi — Android uchun u yagona haqiqat
# manbai. Semver qismi o'zgarmasa ham build raqami oshishi SHART,
# aks holda telefon yangi APK'ni eskisidan ajrata olmaydi.
$build++

$new = "$major.$minor.$patch+$build"
Save-Text $pubspec ([regex]::Replace($raw, '(?m)^version:.*$', "version: $new", 1))

# ── config/*.json ──
#
# ┌─ EKRANDA BUILD RAQAMI YO'Q ────────────────────────────────────────┐
# `+N` faqat Android uchun (`versionCode`) — u foydalanuvchiga hech
# narsa aytmaydi va "0.2.0+2" chalkash ko'rinadi. Shuning uchun
# ekranga faqat semver chiqadi: `OnDex versiya: 0.2.0`.
#
# dev fayllariga `-dev` qo'shiladi: raqam qaysi serverga
# qaralayotganini ham aytishi kerak, aks holda dev build prod'dan
# farqlanmasdi.
# └────────────────────────────────────────────────────────────────────┘
$semver = "$major.$minor.$patch"
$configDir = Join-Path $appDir 'config'
$touched = @()
if (Test-Path $configDir) {
    foreach ($f in Get-ChildItem $configDir -Filter '*.json') {
        $text = [System.IO.File]::ReadAllText($f.FullName)
        if ($text -notmatch '"ONDEX_APP_VERSION"') { continue }
        $value = if ($f.Name -like 'dev*') { "$semver-dev" } else { $semver }
        $updated = [regex]::Replace($text,
            '("ONDEX_APP_VERSION"\s*:\s*")[^"]*(")', "`${1}$value`${2}", 1)
        Save-Text $f.FullName $updated
        $touched += "$($f.Name) -> $value"
    }
}

# ── Natija ──

$tag = if ($DryRun) { '[DryRun] ' } else { '' }
Write-Host "$tag$App`: $old -> $new" -ForegroundColor Green
foreach ($t in $touched) { Write-Host "  $t" -ForegroundColor DarkGray }
Write-Host ''
Write-Host 'Keyingi qadam:' -ForegroundColor Cyan
Write-Host "  cd $($Dirs[$App])"
Write-Host '  flutter build apk --release --dart-define-from-file=config/prod.json'

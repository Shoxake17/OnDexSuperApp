# OnDex ilovalari va panellari versiyasi — BITTA joydan.
#
#   .\scripts\version.ps1                              # hammasini ko'rsatadi
#   .\scripts\version.ps1 bump -Apps customer_app      # 0.2.2+14 -> 0.2.3+15
#   .\scripts\version.ps1 bump                         # hammasi +1
#   .\scripts\version.ps1 sync                         # konfiglarni pubspec bilan tenglaydi
#   .\scripts\version.ps1 files -Apps restaurant_panel # tegishli fayllar ro'yxati
#
# +- QOIDA (2026-09-16) ---------------------------------------------------+
# Har yangilanish (reliz build) versiyani +1 ga oshiradi: patch raqami va
# build raqami (Android versionCode) birga. Prod build skriptlari
# (F:\OndexProd\build.ps1, build_mobile.ps1) buni O'ZI chaqiradi.
#
# Versiya BIR manbada — `pubspec.yaml` (web uchun `package.json`). Ilova
# ichida ko'rinadigan qiymat (`ONDEX_APP_VERSION`, `config/*.json`) shu
# yerdan yoziladi: prod -> "X.Y.Z+N", dev -> "X.Y.Z-dev+N". Avval ular qo'lda
# alohida tahrirlanardi va bir-biridan ajralib ketardi.
#
# Fayllar BOM'siz UTF-8 da yoziladi: PowerShell'ning `Set-Content` i
# kodlashni buzadi, `-Encoding utf8` esa BOM qo'shib JSON o'quvchilarni
# yiqitadi.
# +-----------------------------------------------------------------------+

param(
    [Parameter(Position = 0)]
    [ValidateSet('show', 'bump', 'sync', 'files')]
    [string]$Action = 'show',

    [ValidateSet('customer_app', 'courier_app', 'waiter_app', 'restaurant_panel', 'admin_panel', 'web')]
    [string[]]$Apps = @('customer_app', 'courier_app', 'waiter_app', 'restaurant_panel', 'admin_panel', 'web'),

    [string]$Root = (Split-Path -Parent $PSScriptRoot)
)

$ErrorActionPreference = 'Stop'
$utf8 = New-Object System.Text.UTF8Encoding($false)

function Read-Text([string]$Path) { [IO.File]::ReadAllText($Path) }
function Write-Text([string]$Path, [string]$Text) { [IO.File]::WriteAllText($Path, $Text, $utf8) }

function Get-AppFiles([string]$App) {
    if ($App -eq 'web') {
        $dir = Join-Path $Root 'apps\web'
        return @('package.json', 'package-lock.json') |
            ForEach-Object { Join-Path $dir $_ } | Where-Object { Test-Path $_ }
    }
    $dir = Join-Path $Root "apps\$App"
    $files = @(Join-Path $dir 'pubspec.yaml')
    $cfg = Join-Path $dir 'config'
    if (Test-Path $cfg) {
        $files += Get-ChildItem $cfg -Filter '*.json' | Sort-Object Name | ForEach-Object FullName
    }
    return $files
}

function Get-AppVersion([string]$App) {
    if ($App -eq 'web') {
        $json = Read-Text (Join-Path $Root 'apps\web\package.json')
        if ($json -notmatch '"version"\s*:\s*"(\d+)\.(\d+)\.(\d+)"') { throw "web: package.json da X.Y.Z versiya yo'q" }
        return [pscustomobject]@{ Major = [int]$Matches[1]; Minor = [int]$Matches[2]; Patch = [int]$Matches[3]; Build = $null }
    }
    $yaml = Read-Text (Join-Path $Root "apps\$App\pubspec.yaml")
    if ($yaml -notmatch '(?m)^version:\s*(\d+)\.(\d+)\.(\d+)\+(\d+)\s*$') { throw "${App}: pubspec.yaml da 'version: X.Y.Z+N' yo'q" }
    return [pscustomobject]@{ Major = [int]$Matches[1]; Minor = [int]$Matches[2]; Patch = [int]$Matches[3]; Build = [int]$Matches[4] }
}

function Format-Version($V, [switch]$Dev) {
    $core = '{0}.{1}.{2}' -f $V.Major, $V.Minor, $V.Patch
    if ($null -eq $V.Build) { return $core }
    if ($Dev) { return '{0}-dev+{1}' -f $core, $V.Build }
    return '{0}+{1}' -f $core, $V.Build
}

function Set-AppVersion([string]$App, $V) {
    if ($App -eq 'web') {
        $pkgPath = Join-Path $Root 'apps\web\package.json'
        $pkg = Read-Text $pkgPath
        $name = if ($pkg -match '"name"\s*:\s*"([^"]+)"') { $Matches[1] } else { throw 'web: package.json da name yo''q' }
        $value = Format-Version $V
        Write-Text $pkgPath ([regex]::new('("version"\s*:\s*")[^"]*(")').Replace($pkg, ('${1}' + $value + '${2}'), 1))
        $lockPath = Join-Path $Root 'apps\web\package-lock.json'
        if (Test-Path $lockPath) {
            # Lock faylda loyihaning O'Z versiyasi ikki joyda (ildiz va `packages[""]`).
            $lock = Read-Text $lockPath
            $re = '("name"\s*:\s*"' + [regex]::Escape($name) + '"\s*,\s*"version"\s*:\s*")[^"]*(")'
            Write-Text $lockPath ([regex]::new($re).Replace($lock, ('${1}' + $value + '${2}'), 2))
        }
        return
    }

    $pubspec = Join-Path $Root "apps\$App\pubspec.yaml"
    $yaml = Read-Text $pubspec
    Write-Text $pubspec ([regex]::new('(?m)^version:\s*\S+').Replace($yaml, ('version: ' + (Format-Version $V)), 1))

    foreach ($file in (Get-AppFiles $App | Where-Object { $_ -like '*.json' })) {
        $text = Read-Text $file
        if ($text -notmatch '"ONDEX_APP_VERSION"') { continue }
        $isProd = [IO.Path]::GetFileName($file) -eq 'prod.json'
        $value = if ($isProd) { Format-Version $V } else { Format-Version $V -Dev }
        Write-Text $file ([regex]::new('("ONDEX_APP_VERSION"\s*:\s*")[^"]*(")').Replace($text, ('${1}' + $value + '${2}'), 1))
    }
}

switch ($Action) {
    'files' {
        foreach ($app in $Apps) { Get-AppFiles $app }
    }
    'show' {
        $Apps | ForEach-Object {
            [pscustomobject]@{ Ilova = $_; Versiya = (Format-Version (Get-AppVersion $_)) }
        } | Format-Table -AutoSize
    }
    'sync' {
        foreach ($app in $Apps) {
            $v = Get-AppVersion $app
            Set-AppVersion $app $v
            Write-Host ('{0,-18} {1}  (konfiglar tenglandi)' -f $app, (Format-Version $v))
        }
    }
    'bump' {
        foreach ($app in $Apps) {
            $old = Get-AppVersion $app
            $new = [pscustomobject]@{
                Major = $old.Major; Minor = $old.Minor; Patch = $old.Patch + 1
                Build = if ($null -eq $old.Build) { $null } else { $old.Build + 1 }
            }
            Set-AppVersion $app $new
            Write-Host ('{0,-18} {1} -> {2}' -f $app, (Format-Version $old), (Format-Version $new)) -ForegroundColor Green
        }
    }
}

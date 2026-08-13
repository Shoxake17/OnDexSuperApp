# Lokal ishlab chiqish tunnelini ishga tushiradi (Cloudflare `ondex-local`).
#
#   .\scripts\dev_tunnel.ps1
#
# Ochiladigan manzillar:
#   https://dev-api-ondex.shoxpro.uz  ->  localhost:8080  (Go API)
#   https://dev-web-ondex.shoxpro.uz  ->  localhost:3000  (Next.js)
#
# Sabab va sozlamalar: scripts/cloudflared/config.yml ichidagi izohlar.
#
# Tunnel API va web serverlarni O'ZI ishga tushirmaydi — ular alohida
# oynalarda ishlab turishi kerak. Server ishlamayotgan bo'lsa tunnel
# baribir ko'tariladi va Cloudflare 502 qaytaradi (bu KUTILGAN holat,
# tunnel nosozligi emas).

$ErrorActionPreference = 'Stop'

$cfg = Join-Path $PSScriptRoot 'cloudflared\config.yml'
if (-not (Test-Path $cfg)) {
    throw "Config topilmadi: $cfg"
}

$cloudflared = (Get-Command cloudflared -ErrorAction SilentlyContinue).Source
if (-not $cloudflared) {
    throw "cloudflared topilmadi. O'rnatish: winget install Cloudflare.cloudflared"
}

# Ogohlantirish, to'xtatish emas: tunnel serverlardan oldin ham
# ko'tarilishi mumkin, tartib muhim emas.
foreach ($p in 8080, 3000) {
    $up = Test-NetConnection -ComputerName 127.0.0.1 -Port $p -InformationLevel Quiet -WarningAction SilentlyContinue
    if (-not $up) {
        Write-Warning "localhost:$p tinglanmayapti — o'sha xizmat hozircha 502 beradi."
    }
}

Write-Host ""
Write-Host "  API :  https://dev-api-ondex.shoxpro.uz  -> localhost:8080"
Write-Host "  WEB :  https://dev-web-ondex.shoxpro.uz  -> localhost:3000"
Write-Host ""
Write-Host "  To'xtatish: Ctrl+C"
Write-Host ""

# `--config` SHART: busiz boshqa loyihaning ~/.cloudflared/config.yml i
# o'qiladi. Batafsil sabab config.yml ichida.
& $cloudflared --config $cfg tunnel run

# ChustApp demo: auth (SMS kod + JWT) bilan buyurtmaning to'liq hayot sikli.
# Avval boshqa terminalda server ishlab turishi kerak:  go run ./cmd/api
$ErrorActionPreference = 'Stop'
$base = 'http://localhost:8080'

function Get-Token([string]$phone) {
    $rc = Invoke-RestMethod -Method Post -Uri "$base/auth/request-code" -Body ('{"phone":"' + $phone + '"}')
    $v  = Invoke-RestMethod -Method Post -Uri "$base/auth/verify" -Body ('{"phone":"' + $phone + '","code":"' + $rc.dev_code + '"}')
    return $v
}
function TokenHeader([string]$token) { return @{Authorization = "Bearer $token"} }

Write-Host "0) Kirish: mijoz, restoran va kuryerlar SMS kod bilan login qilmoqda..." -ForegroundColor Cyan
$customer = Get-Token '+998901234567'
$rest     = Get-Token '+998900000010'
$c1       = Get-Token '+998900000001'
$c2       = Get-Token '+998900000002'
$c3       = Get-Token '+998900000003'
Write-Host "   Mijoz: $($customer.user.id) (yangi ro'yxatdan o'tdi), Restoran: $($rest.user.name), Kuryerlar: 3 ta`n"

Write-Host "1) Tokensiz buyurtma berish urinishi (401 bo'lishi kerak)..." -ForegroundColor Cyan
try {
    Invoke-RestMethod -Method Post -Uri "$base/orders" -Body '{}'
} catch {
    Write-Host "   Rad etildi: $($_.ErrorDetails.Message)`n" -ForegroundColor Yellow
}

Write-Host "2) Kuryerlar o'z tokenlari bilan online bo'lmoqda..." -ForegroundColor Cyan
Invoke-RestMethod -Method Post -Uri "$base/couriers/c1/available" -Headers (TokenHeader $c1.token) -Body '{"available":true}' | Out-Null
Invoke-RestMethod -Method Post -Uri "$base/couriers/c2/available" -Headers (TokenHeader $c2.token) -Body '{"available":true}' | Out-Null
Invoke-RestMethod -Method Post -Uri "$base/couriers/c3/available" -Headers (TokenHeader $c3.token) -Body '{"available":true}' | Out-Null
Write-Host "   c1, c2, c3 online`n"

Write-Host "3) Mijoz restoranlar va menyuni ko'rmoqda..." -ForegroundColor Cyan
$restaurants = Invoke-RestMethod -Uri "$base/restaurants"
$menu = Invoke-RestMethod -Uri "$base/restaurants/r1/menu"
Write-Host "   Restoran: $($restaurants[0].name); menyuda $($menu.Count) ta taom"
$menu | ForEach-Object { Write-Host "     - $($_.name): $($_.price_tiyin / 100) so'm" }

Write-Host "`n4) Mijoz buyurtma bermoqda (narx YUBORMAYAPTI - server hisoblaydi)..." -ForegroundColor Cyan
$body = @{
    delivery_lat = 41.0056; delivery_lng = 71.2378
    items = @(
        @{product_id='p1'; qty=2},
        @{product_id='p3'; qty=1; price_tiyin=1}  # narxni aldashga urinish - server e'tiborsiz qoldiradi
    )
} | ConvertTo-Json -Depth 5
$order = Invoke-RestMethod -Method Post -Uri "$base/orders" -Headers (TokenHeader $customer.token) -Body $body
Write-Host "   Yaratildi: id=$($order.id), jami=$($order.total_tiyin / 100) so'm (2x Osh + 1x Choy, katalogdan)"
Write-Host "   Mijoz yuborgan soxta narx (1 tiyin) e'tiborsiz qoldirildi`n"

Write-Host "5) Mijoz restoran nomidan qabul qilishga urinmoqda (403/409 bo'lishi kerak)..." -ForegroundColor Cyan
try {
    Invoke-RestMethod -Method Post -Uri "$base/orders/$($order.id)/transition" -Headers (TokenHeader $customer.token) -Body '{"to":"accepted"}'
} catch {
    Write-Host "   Rad etildi: $($_.ErrorDetails.Message)`n" -ForegroundColor Yellow
}

Write-Host "6) Restoran qabul qilmoqda va kuryer chaqirmoqda..." -ForegroundColor Cyan
$r = Invoke-RestMethod -Method Post -Uri "$base/orders/$($order.id)/transition" -Headers (TokenHeader $rest.token) -Body '{"to":"accepted"}'
Write-Host "   Holat: $($r.status)"
Invoke-RestMethod -Method Post -Uri "$base/orders/$($order.id)/dispatch" -Headers (TokenHeader $rest.token) | Out-Null
Start-Sleep 1
Write-Host "   Dispatch boshlandi (server logida taklif kimga ketganini ko'rasiz)`n"

Write-Host "7) Kuryer c1 taklifni qabul qilmoqda..." -ForegroundColor Cyan
Invoke-RestMethod -Method Post -Uri "$base/couriers/c1/respond" -Headers (TokenHeader $c1.token) -Body ('{"order_id":"' + $order.id + '","accepted":true}') | Out-Null
Start-Sleep 1
Write-Host "   Qabul qilindi`n"

Write-Host "8) Restoran tayyorlaydi, kuryer yetkazadi..." -ForegroundColor Cyan
foreach ($step in @(
    @{to='preparing'; tok=$rest.token},
    @{to='ready';     tok=$rest.token},
    @{to='picked_up'; tok=$c1.token},
    @{to='delivered'; tok=$c1.token})) {
    $r = Invoke-RestMethod -Method Post -Uri "$base/orders/$($order.id)/transition" -Headers (TokenHeader $step.tok) -Body ('{"to":"' + $step.to + '"}')
    Write-Host "   -> $($r.status)"
}

Write-Host "`n9) Mijoz o'z buyurtmasini ko'rmoqda:" -ForegroundColor Cyan
Invoke-RestMethod -Uri "$base/orders/$($order.id)" -Headers (TokenHeader $customer.token) | ConvertTo-Json -Depth 5



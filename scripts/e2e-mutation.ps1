# E2E suite SEZGIRLIGI testi (mutatsiya): production kodga ataylab xato kiritadi
# va E2E suite uni USHLASHINI tekshiradi. Ushlamasa — suite shu sohada ko'r.
#
#   .\scripts\e2e-mutation.ps1
#
# Har mutatsiya: fayl, eski matn (aynan BIR marta uchrashi shart), yangi matn.
# Fayl DOIM asl holatiga qaytariladi (xato bo'lsa ham).

$ErrorActionPreference = 'Continue'
Set-Location (Split-Path $PSScriptRoot -Parent)
$enc = New-Object Text.UTF8Encoding($false)

$mutations = @(
    @{ name = 'pul: yetkazish haqi jamga QO''SHILMAYDI'; file = 'internal\orders\service.go';
       old = 'o.TotalTiyin = subtotal - discount + fee'; new = 'o.TotalTiyin = subtotal - discount' },
    @{ name = 'xavfsizlik: begona mijoz buyurtmani ko''ra oladi'; file = 'internal\httpapi\authz.go';
       old = 'return o.CustomerID == c.Subject'; new = 'return true' },
    @{ name = 'xavfsizlik: begona restoran buyurtmani boshqara oladi'; file = 'internal\httpapi\authz.go';
       old = 'case users.RoleRestaurant:{NL}		return o.RestaurantID == c.EntityID'; new = 'case users.RoleRestaurant:{NL}		return true' },
    @{ name = 'to''lov: o''chirilgan kanal bilan to''lov ochiladi'; file = 'internal\httpapi\routes_payments.go';
       old = 'if !rest.AcceptsChannel(channel) {'; new = 'if false && !rest.AcceptsChannel(channel) {' },
    @{ name = 'ko''rinish: o''chirilgan Payme mijozga ko''rinadi'; file = 'internal\catalog\settings.go';
       old = 'Payme: p.Payme && online,'; new = 'Payme: online,' },
    @{ name = 'qoida: ta''tildagi kuryer haqi mijozga qo''shiladi'; file = 'internal\staff\staff.go';
       old = '|| !m.AccessEffective() ||'; new = '||' },
    @{ name = 'qoida: stolda yetkazish haqi olinadi (quote dine_in ni e''tiborsiz qoldiradi)'; file = 'internal\httpapi\routes_orders.go';
       old = 'claimsFrom(r).Subject, req.DineIn)'; new = 'claimsFrom(r).Subject, false)' },
    @{ name = 'xavfsizlik: chiqishdan keyin token ishlaydi'; file = 'internal\revoke\revoke.go';
       old = 'return issuedAt.Before(t)'; new = 'return issuedAt.Before(t) && false' },
    @{ name = 'xavfsizlik: moliya reestrida draft summa qo''shiladi'; file = 'internal\httpapi\routes_finance.go';
       old = 'case multicard.StatusSuccess:'; new = 'case multicard.StatusSuccess, multicard.StatusDraft:' },
    @{ name = 'pul: rad etilgan to''langan buyurtma puli QAYTMAYDI'; file = 'internal\orders\service.go';
       old = 'if err := s.payments.ReleaseForOrder(ctx, o.ID); err != nil {'; new = 'if err := error(nil); err != nil {' },
    @{ name = 'hamyon: keshbek 1% emas 2%'; file = 'internal\wallet\wallet.go';
       old = 'CashbackPercent = 1'; new = 'CashbackPercent = 2' },
    @{ name = 'validatsiya: miqdor chegarasi (100) olib tashlanadi'; file = 'internal\catalog\service.go';
       old = 'r.Qty < 1 || r.Qty > 100'; new = 'r.Qty < 1' },
    @{ name = 'aksiya: 1+1 har dona bepul'; file = 'internal\promotions\apply.go';
       old = 'int64(lines[i].Qty/2) * lines[i].UnitPriceTiyin'; new = 'int64(lines[i].Qty) * lines[i].UnitPriceTiyin' },
    @{ name = 'aksiya: summa chegirmasi qator summasidan oshadi'; file = 'internal\promotions\apply.go';
       old = 'if s := lines[i].subtotal(); d > s {'; new = 'if s := lines[i].subtotal(); false && d > s {' },
    @{ name = 'aksiya: minimal buyurtma summasi e''tiborsiz'; file = 'internal\promotions\apply.go';
       old = 'if p.MinOrderAmountTiyin > 0 && subtotal < p.MinOrderAmountTiyin {'; new = 'if false {' },
    @{ name = 'aksiya: maksimal chegirma cheklovi ishlamaydi'; file = 'internal\promotions\apply.go';
       old = 'if granted <= limit {'; new = 'if true || granted <= limit {' },
    @{ name = 'nazoratchi: "kuryer topilmadi" darhol avto-bekor'; file = 'internal\orders\dispatch_state.go';
       old = 'deadline := now.Add(CourierNotFoundCancelAfter)'; new = 'deadline := now.Add(0)' },
    @{ name = 'kesh: restoran saqlangach kesh bekor qilinmaydi (yopilgan restoran ochiq ko''rinadi)'; file = 'internal\storage\cached_catalog.go';
       old = 'c.rests.invalidate(r.ID)'; new = '_ = r.ID' },
    @{ name = 'kesh: aksiya saqlangach kesh bekor qilinmaydi (yangi aksiya ko''rinmaydi)'; file = 'internal\storage\cached_promotions.go';
       old = 'c.lists.invalidate(p.RestaurantID)'; new = '_ = p.RestaurantID' },
    @{ name = 'kesh: xodim yangilangach kesh bekor qilinmaydi (eski yetkazish haqi)'; file = 'internal\storage\cached_staff.go';
       old = 'c.Repository.Update(ctx, m, events){NL}' + [char]9 + 'c.lists.invalidate(m.RestaurantID)'; new = 'c.Repository.Update(ctx, m, events){NL}' + [char]9 + '_ = m.RestaurantID' }
)

$survived = @()
foreach ($m in $mutations) {
    $path = Join-Path (Get-Location) $m.file
    $orig = [IO.File]::ReadAllText($path, $enc)
    $nl = if ($orig.Contains("`r`n")) { "`r`n" } else { "`n" }
    $m.old = $m.old.Replace('{NL}', $nl); $m.new = $m.new.Replace('{NL}', $nl)
    $count = ([regex]::Matches($orig, [regex]::Escape($m.old))).Count
    if ($count -ne 1) {
        Write-Host "SKIP  $($m.name): '$($m.old)' $count marta uchradi (aynan 1 kerak)" -ForegroundColor Yellow
        $survived += "$($m.name) [mutatsiya qo'llanmadi]"
        continue
    }
    try {
        [IO.File]::WriteAllText($path, $orig.Replace($m.old, $m.new), $enc)
        $out = & go test ./internal/e2e/ -count=1 2>&1 | Out-String
        $failed = $LASTEXITCODE -ne 0
    } finally {
        [IO.File]::WriteAllText($path, $orig, $enc)
    }
    if ($failed) {
        Write-Host "USHLANDI  $($m.name)" -ForegroundColor Green
    } else {
        Write-Host "O'TIB KETDI  $($m.name)" -ForegroundColor Red
        $survived += $m.name
    }
}

& go build ./... | Out-Null
Write-Host ""
if ($survived.Count -eq 0) {
    Write-Host "Hamma $($mutations.Count) mutatsiya ushlandi." -ForegroundColor Green
    exit 0
}
Write-Host "Ushlanmagan mutatsiyalar:" -ForegroundColor Red
$survived | ForEach-Object { Write-Host "  - $_" }
exit 1

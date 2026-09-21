# Restoran panelining Windows `.exe` belgisi — KO'P O'LCHAMLI `.ico`.
#
#   .\scripts\windows_icon.ps1
#
# +- NEGA QO'LDA, `flutter_launcher_icons` BILAN EMAS (2026-09-16) ---------+
# Generator bitta rasmdan hamma o'lchamni yasaydi. OnDex Merchant logosi
# esa KENG va ichida "MERCHANT" yozuvi bor: 256px da chiroyli, 32px da
# (Explorer, vazifalar paneli, oyna sarlavhasi) o'qib bo'lmaydigan dog'ga
# aylanadi - jonli build'da aynan shunday chiqdi.
#
# Shuning uchun `.ico` ichida IKKI xil chizma bo'ladi:
#   16/24/32/48 -> sof OnDex belgisi  ($MarkSource);
#   64/128/256  -> to'liq Merchant logosi ($FullSource).
# Windows har joyda mos o'lchamni o'zi tanlaydi.
#
# Kichik o'lchamlar uchun belgi Merchant logosidan KESIB OLINMAYDI: u
# yerda "MERCHANT" lentasi belgining pastki o'ng burchagiga chiqib turadi
# va qaysi chegara bilan kesilmasin, kesimda oq chiziq bo'lib qoladi.
#
# Manba sifatida `image/OnDexMerchant.png` ham ISHLATILMAYDI - uning foni
# shaffof emas, qora; kichik o'lchamlarda belgi atrofida qora dog' chiqadi.
# Tozalangan, shaffof fondagi nusxasi - `assets/app_icon_windows.png`.
#
# Shu sabab `pubspec.yaml` da `windows: generate: false` - aks holda
# generator bu faylni bitta chizmali `.ico` bilan bosib ketardi.
# +-----------------------------------------------------------------------+

param(
    # Kichik o'lchamlar: sof OnDex belgisi (shaffof fon).
    [string]$MarkSource = 'F:\ChustApp\image\ondex.png',
    # Katta o'lchamlar: to'liq Merchant logosi (shaffof fon, tozalangan).
    [string]$FullSource = 'F:\ChustApp\apps\restaurant_panel\assets\app_icon_windows.png',
    [string]$Output = 'F:\ChustApp\apps\restaurant_panel\windows\runner\resources\app_icon.ico'
)

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing

function New-Square {
    param([System.Drawing.Bitmap]$Src, [System.Drawing.Rectangle]$Box, [int]$Size, [double]$Ratio)
    $scale = [Math]::Min($Size * $Ratio / $Box.Width, $Size * $Ratio / $Box.Height)
    $dw = [int][Math]::Round($Box.Width * $scale)
    $dh = [int][Math]::Round($Box.Height * $scale)
    $canvas = New-Object System.Drawing.Bitmap $Size, $Size, ([System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $g = [System.Drawing.Graphics]::FromImage($canvas)
    $g.Clear([System.Drawing.Color]::Transparent)
    $g.InterpolationMode = 'HighQualityBicubic'
    $g.SmoothingMode = 'HighQuality'
    $g.PixelOffsetMode = 'HighQuality'
    $dst = New-Object System.Drawing.Rectangle ([int](($Size - $dw) / 2)), ([int](($Size - $dh) / 2)), $dw, $dh
    $g.DrawImage($Src, $dst, $Box, [System.Drawing.GraphicsUnit]::Pixel)
    $g.Dispose()
    return $canvas
}

# Shaffof bo'lmagan qismning chegarasi - rasm atrofidagi bo'sh joy
# kesiladi, aks holda belgi kvadratning ichida keragidan kichik chiqadi.
function Get-ContentBox {
    param([System.Drawing.Bitmap]$Src)
    $minX = $Src.Width; $minY = $Src.Height; $maxX = -1; $maxY = -1
    for ($y = 0; $y -lt $Src.Height; $y += 2) {
        for ($x = 0; $x -lt $Src.Width; $x += 2) {
            if ($Src.GetPixel($x, $y).A -gt 40) {
                if ($x -lt $minX) { $minX = $x }; if ($x -gt $maxX) { $maxX = $x }
                if ($y -lt $minY) { $minY = $y }; if ($y -gt $maxY) { $maxY = $y }
            }
        }
    }
    if ($maxX -lt 0) { throw "Rasmda shaffof bo'lmagan piksel yo'q" }
    return New-Object System.Drawing.Rectangle $minX, $minY, ($maxX - $minX + 1), ($maxY - $minY + 1)
}

$mark = [System.Drawing.Bitmap]::FromFile($MarkSource)
$full = [System.Drawing.Bitmap]::FromFile($FullSource)
try {
    $markBox = Get-ContentBox -Src $mark
    $fullBox = Get-ContentBox -Src $full

    $plan = @(
        @{ Size = 16;  Img = $mark; Box = $markBox; Ratio = 0.94 },
        @{ Size = 24;  Img = $mark; Box = $markBox; Ratio = 0.94 },
        @{ Size = 32;  Img = $mark; Box = $markBox; Ratio = 0.94 },
        @{ Size = 48;  Img = $mark; Box = $markBox; Ratio = 0.94 },
        @{ Size = 64;  Img = $full; Box = $fullBox; Ratio = 0.96 },
        @{ Size = 128; Img = $full; Box = $fullBox; Ratio = 0.96 },
        @{ Size = 256; Img = $full; Box = $fullBox; Ratio = 0.96 }
    )

    $images = @()
    foreach ($p in $plan) {
        $bmp = New-Square -Src $p.Img -Box $p.Box -Size $p.Size -Ratio $p.Ratio
        $ms = New-Object System.IO.MemoryStream
        $bmp.Save($ms, [System.Drawing.Imaging.ImageFormat]::Png)
        $bmp.Dispose()
        $images += [pscustomobject]@{ Size = $p.Size; Bytes = $ms.ToArray() }
        $ms.Dispose()
    }

    # ICO: sarlavha + har bir chizma uchun yozuv, chizmalar PNG sifatida
    # (Windows Vista+ buni qo'llaydi va 256px uchun bu YAGONA yo'l).
    # O'lcham maydoni bir bayt: 256 u yerga sig'maydi va 0 bilan yoziladi.
    $out = New-Object System.IO.MemoryStream
    $bw = New-Object System.IO.BinaryWriter $out
    $bw.Write([uint16]0); $bw.Write([uint16]1); $bw.Write([uint16]$images.Count)
    $offset = 6 + 16 * $images.Count
    foreach ($img in $images) {
        $dim = if ($img.Size -ge 256) { 0 } else { $img.Size }
        $bw.Write([byte]$dim); $bw.Write([byte]$dim)
        $bw.Write([byte]0); $bw.Write([byte]0)
        $bw.Write([uint16]1); $bw.Write([uint16]32)
        $bw.Write([uint32]$img.Bytes.Length)
        $bw.Write([uint32]$offset)
        $offset += $img.Bytes.Length
    }
    foreach ($img in $images) { $bw.Write($img.Bytes) }
    $bw.Flush()
    [IO.File]::WriteAllBytes($Output, $out.ToArray())
    $bw.Dispose(); $out.Dispose()

    "Yozildi: {0} ({1:N0} KB)" -f $Output, ((Get-Item $Output).Length / 1KB)
    "  16-48px  <- {0}" -f (Split-Path $MarkSource -Leaf)
    "  64-256px <- {0}" -f (Split-Path $FullSource -Leaf)
}
finally { $mark.Dispose(); $full.Dispose() }

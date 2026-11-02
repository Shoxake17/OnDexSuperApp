/// Taom kartochkasi va kartochkalar to'ri — BITTA nusxa.
///
/// ┌─ NEGA BU FAYL ────────────────────────────────────────────────────┐
/// Kartochka mijoz ilovasida uch joyda (menyu, qidiruv, "Istaklarim")
/// va affitsiant ilovasining buyurtma ekranida kerak. Vebdagi mos fayl —
/// `apps/web/app/(food)/restaurants/[id]/product-card.tsx`. Hammasi
/// ATAYLAB bir xil ko'rinadi; ranglar esa ilovaning mavzusidan
/// ([MenuPalette]) — mijoz ilovasi yorug', affitsiant ilovasi qorong'i.
/// └───────────────────────────────────────────────────────────────────┘
library;

import 'package:flutter/material.dart';
import 'package:ondex_core/ondex_core.dart' show formatSum, formatWeightUnit;

import 'menu_widgets.dart';
import 'product_pricing.dart';

/// Kartochka ko'rinishi.
///  * [tile] — taomlar: kvadrat rasm, boshqaruv rasm USTIDA, rasm ostida narx va nom;
///  * [wide] — cho'zinchoq rasmli mahsulotlar (Qurt): ramkali karta, rasm, nom, tavsif,
///    pastda narx (+ og'irlik) va o'ng tomonda \+\ yoki \− n +\ (maket: image/qurtplus.png).
enum ProductCardLayout { tile, wide }

/// Kvadrat rasm, chap yuqorida ilovaga xos uya ([topLeft] — mijoz
/// ilovasida "Istaklarim" yuragi), o'ng yuqorida aksiya lentasi, pastda
/// "+" yoki "− n +" boshqaruvi; rasm ostida narx, nom, og'irlik.
class ProductCard extends StatelessWidget {
  final Map<String, dynamic> product;

  /// Tanlangan miqdor. `null` — miqdor boshqaruvi UMUMAN chizilmaydi
  /// ("Istaklarim" sahifasi shunday: u yerdan savatga qo'shilmaydi,
  /// chunki bitta buyurtma bitta restorandan bo'lishi shart).
  final int? qty;

  final ProductDiscount? discount;
  final bool promoted;

  /// Rasmning chap yuqori burchagidagi ilovaga xos element.
  ///
  /// Sevimlilar faqat MIJOZ ilovasida bor (u yerda `FavoritesStore`
  /// yashaydi) — umumiy kartochka ular haqida hech narsa bilmaydi.
  final Widget? topLeft;

  final VoidCallback? onAdd;

  /// "+" tugmasining kaliti (mijoz ilovasidagi yordamchi "barmog'i"
  /// aynan shu tugmani topishi uchun).
  final Key? addKey;

  final VoidCallback? onRemove;
  final VoidCallback? onTap;

  /// Nom/narx ostiga qo'shiladigan qator (masalan "Istaklarim" dagi
  /// restoran nomi). Menyuda ishlatilmaydi.
  final Widget? footer;

  /// Rasm kengligi : balandligi. Standart 1 (kvadrat — taomlar); Qurt kabi cho'zinchoq
  /// rasmli mahsulotlar uchun kattaroq (2.4). To'r balandligi ham shunga moslanadi:
  /// [productGridDelegate] imageAspectRatio si BIR XIL berilishi shart.
  final double imageAspectRatio;

  /// Rasm qutiga qanday sig'adi: cover (kesadi, taomlar) yoki contain (kesmaydi).
  final BoxFit imageFit;

  /// Kartochka ko'rinishi ([ProductCardLayout]). Standart — taom kartochkasi.
  final ProductCardLayout layout;

  /// Faqat [ProductCardLayout.wide]: nom ostidagi qisqa matn (ikki qatorgacha).
  final String? subtitle;

  const ProductCard({
    super.key,
    required this.product,
    this.qty,
    this.discount,
    this.promoted = false,
    this.topLeft,
    this.onAdd,
    this.addKey,
    this.onRemove,
    this.onTap,
    this.footer,
    this.imageAspectRatio = 1,
    this.imageFit = BoxFit.cover,
    this.layout = ProductCardLayout.tile,
    this.subtitle,
  });

  @override
  Widget build(BuildContext context) {
    if (layout == ProductCardLayout.wide) return _buildWide(context);
    final style = MenuPalette.of(context);
    final name = (product['name'] as String?) ?? '';
    final image = (product['image_url'] as String?) ?? '';
    final price = (product['price_tiyin'] as num?)?.toInt() ?? 0;
    final available = product['available'] != false;
    final weight = (product['weight'] as num?)?.toDouble() ?? 0;
    final unit = formatWeightUnit((product['weight_unit'] as String?) ?? '');

    return Opacity(
      opacity: available ? 1 : 0.4,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          AspectRatio(
            aspectRatio: imageAspectRatio,
            child: Stack(
              children: [
                Positioned.fill(
                  child: ClipRRect(
                    borderRadius: BorderRadius.circular(16),
                    child: Material(
                      color: style.imageBackground,
                      child: InkWell(
                        onTap: onTap,
                        child: RemoteImage(
                          url: image,
                          fit: imageFit,
                          placeholder: Center(
                            child: Icon(Icons.restaurant_menu,
                                size: 30, color: style.placeholderIcon),
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
                if (topLeft != null)
                  Positioned(left: 8, top: 8, child: topLeft!),
                if (promoted && available)
                  Positioned(
                    right: 8,
                    top: 8,
                    child: _Badge(
                      text: discount?.label ?? 'Aksiya',
                      background: style.brand,
                      foreground: Colors.white,
                    ),
                  ),
                if (!available)
                  const Positioned(
                    left: 8,
                    bottom: 8,
                    child: _Badge(
                      text: 'Tugadi',
                      background: Color(0xD9000000),
                      foreground: Colors.white,
                    ),
                  ),
                if (available && qty != null && qty == 0)
                  Positioned(
                    right: 8,
                    bottom: 8,
                    child: RoundIconButton(
                        key: addKey,
                        icon: Icons.add,
                        onTap: onAdd,
                        iconSize: 19),
                  ),
                if (available && qty != null && qty! > 0)
                  Positioned(
                    left: 8,
                    right: 8,
                    bottom: 8,
                    child: Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        RoundIconButton(icon: Icons.remove, onTap: onRemove, iconSize: 19),
                        Container(
                          padding: const EdgeInsets.symmetric(
                              horizontal: 10, vertical: 5),
                          decoration: BoxDecoration(
                            color: style.controlBackground,
                            borderRadius: BorderRadius.circular(20),
                            boxShadow: const [
                              BoxShadow(
                                color: Color(0x33000000),
                                blurRadius: 4,
                                offset: Offset(0, 2),
                              ),
                            ],
                          ),
                          child: Text(
                            '$qty',
                            style: TextStyle(
                                fontWeight: FontWeight.bold,
                                fontSize: 13,
                                color: style.controlForeground),
                          ),
                        ),
                        RoundIconButton(
                            key: addKey,
                            icon: Icons.add,
                            onTap: onAdd,
                            iconSize: 19),
                      ],
                    ),
                  ),
              ],
            ),
          ),
          const SizedBox(height: 8),
          InkWell(
            onTap: onTap,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (discount != null)
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.end,
                    children: [
                      Text(
                        formatSum(discount!.discountedPriceTiyin),
                        style: TextStyle(
                            fontWeight: FontWeight.bold,
                            fontSize: 15,
                            color: style.discountPrice),
                      ),
                      const SizedBox(width: 6),
                      Flexible(
                        child: Text(
                          formatSum(price),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(
                            fontSize: 12,
                            color: style.mutedText,
                            decoration: TextDecoration.lineThrough,
                          ),
                        ),
                      ),
                    ],
                  )
                else
                  Text(
                    formatSum(price),
                    style: TextStyle(
                        fontWeight: FontWeight.bold,
                        fontSize: 15,
                        color: style.text),
                  ),
                const SizedBox(height: 2),
                Text(
                  name,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(fontSize: 13, height: 1.25, color: style.text),
                ),
                if (weight > 0)
                  Padding(
                    padding: const EdgeInsets.only(top: 2),
                    child: Text(
                      '${weight % 1 == 0 ? weight.toInt() : weight.toStringAsFixed(1)} $unit'
                          .trim(),
                      style: TextStyle(fontSize: 12, color: style.mutedText),
                    ),
                  ),
                if (footer != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 4),
                    child: footer,
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  /// Cho'zinchoq kartochka (maket: image/qurtplus.png): ramka ichida rasm, nom, tavsif,
  /// pastda narx va o'ngda "+" (savatda bo'lsa "− n +").
  Widget _buildWide(BuildContext context) {
    final style = MenuPalette.of(context);
    final name = (product['name'] as String?) ?? '';
    final image = (product['image_url'] as String?) ?? '';
    final price = (product['price_tiyin'] as num?)?.toInt() ?? 0;
    final available = product['available'] != false;
    final weight = (product['weight'] as num?)?.toDouble() ?? 0;
    final unit = formatWeightUnit((product['weight_unit'] as String?) ?? '');
    final sub = (subtitle ?? '').trim();

    Widget circle(IconData icon, VoidCallback? onTap, {Key? key}) => InkResponse(
      key: key,
      onTap: onTap,
      radius: 22,
      child: Container(
        width: 34,
        height: 34,
        decoration: BoxDecoration(
          shape: BoxShape.circle,
          color: onTap == null ? style.controlDisabled : style.brand,
        ),
        child: Icon(icon, size: 20, color: Colors.white),
      ),
    );

    Widget control() {
      if (!available || qty == null) return const SizedBox.shrink();
      if (qty == 0) {
        return Align(
          alignment: Alignment.centerRight,
          child: circle(Icons.add, onAdd, key: addKey),
        );
      }
      // Restoran menyusidagidek: "− n +" butun qatorni egallaydi (chapda −, o'rtada son, o'ngda +).
      return Container(
        height: 36,
        width: double.infinity,
        decoration: BoxDecoration(
          color: Colors.white,
          borderRadius: BorderRadius.circular(20),
          border: Border.all(color: style.chipBorder),
        ),
        child: Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            InkResponse(
              onTap: onRemove,
              radius: 20,
              child: SizedBox(
                width: 34,
                height: 36,
                child: Icon(Icons.remove, size: 20, color: style.brand),
              ),
            ),
            Text(
              '$qty',
              textAlign: TextAlign.center,
              style: const TextStyle(fontWeight: FontWeight.w800, fontSize: 14),
            ),
            InkResponse(
              key: addKey,
              onTap: onAdd,
              radius: 20,
              child: SizedBox(
                width: 34,
                height: 36,
                child: Icon(Icons.add, size: 20, color: style.brand),
              ),
            ),
          ],
        ),
      );
    }

    return Opacity(
      opacity: available ? 1 : 0.45,
      child: Container(
        padding: const EdgeInsets.all(8),
        decoration: BoxDecoration(
          color: Colors.white,
          borderRadius: BorderRadius.circular(18),
          border: Border.all(color: const Color(0xFFEDEDED)),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            AspectRatio(
              aspectRatio: imageAspectRatio,
              child: Stack(
                children: [
                  Positioned.fill(
                    child: ClipRRect(
                      borderRadius: BorderRadius.circular(12),
                      child: Material(
                        color: style.imageBackground,
                        child: InkWell(
                          onTap: onTap,
                          child: RemoteImage(
                            url: image,
                            fit: imageFit,
                            placeholder: Center(
                              child: Icon(Icons.restaurant_menu, size: 28, color: style.placeholderIcon),
                            ),
                          ),
                        ),
                      ),
                    ),
                  ),
                  if (topLeft != null) Positioned(left: 6, top: 6, child: topLeft!),
                  if (promoted && available)
                    Positioned(
                      right: 6,
                      top: 6,
                      child: _Badge(text: discount?.label ?? 'Aksiya', background: style.brand, foreground: Colors.white),
                    ),
                  if (!available)
                    const Positioned(
                      left: 6,
                      bottom: 6,
                      child: _Badge(text: 'Tugadi', background: Color(0xD9000000), foreground: Colors.white),
                    ),
                ],
              ),
            ),
            const SizedBox(height: 8),
            InkWell(
              onTap: onTap,
              child: Text(
                name,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(fontSize: 14.5, fontWeight: FontWeight.w800, color: style.text),
              ),
            ),
            const SizedBox(height: 2),
            SizedBox(
              height: _kWideSubtitleHeight,
              child: Text(
                sub,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(fontSize: 12, height: 1.3, color: style.mutedText),
              ),
            ),
            const SizedBox(height: 6),
            // 1-qator: narx va og'irlik — butun kenglikda, boshqaruv bilan siqishmaydi.
            SizedBox(
              height: _kWidePriceHeight,
              child: FittedBox(
                fit: BoxFit.scaleDown,
                alignment: Alignment.centerLeft,
                child: Text.rich(
                  TextSpan(
                    children: [
                      TextSpan(
                        text: formatSum(price),
                        style: TextStyle(fontSize: 15, fontWeight: FontWeight.w800, color: style.brand),
                      ),
                      if (weight > 0)
                        TextSpan(
                          text: '  / ${weight % 1 == 0 ? weight.toInt() : weight.toStringAsFixed(1)} $unit'.trimRight(),
                          style: TextStyle(fontSize: 12, fontWeight: FontWeight.w500, color: style.mutedText),
                        ),
                    ],
                  ),
                  maxLines: 1,
                ),
              ),
            ),
            const SizedBox(height: 6),
            // 2-qator: "+" (o'ngda) yoki butun qatorli "− n +" (restoran menyusidagidek).
            SizedBox(height: 36, child: control()),
          ],
        ),
      ),
    );
  }
}

class _Badge extends StatelessWidget {
  final String text;
  final Color background;
  final Color foreground;

  const _Badge({
    required this.text,
    required this.background,
    required this.foreground,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 3),
      decoration: BoxDecoration(
        color: background,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Text(
        text,
        style: TextStyle(
            fontSize: 11, fontWeight: FontWeight.bold, color: foreground),
      ),
    );
  }
}

/// Rasm ostidagi matn blokining balandligi (piksel).
///
/// Narx (~20) + oraliqlar (8+2) + ikki qatorli nom (13px × 1.25 × 2 ≈ 34)
/// + og'irlik (17) + kichik zaxira. Ikki qator DOIM zaxiraga olinadi —
/// aks holda bir qatorli va ikki qatorli nomli kartochkalar turli
/// balandlikda bo'lib, to'r notekis ko'rinardi.
///
/// Qiymat QURILMADA o'lchandi: avval 82 edi va "Istaklarim" ekranida
/// "BOTTOM OVERFLOWED BY 4.0 PIXELS" bergan.
const _kCardTextHeight = 88.0;

/// [ProductCard.footer] bor bo'lganda qo'shiladigan balandlik
/// (restoran nomi qatori: 16px logo + 4px chekinish).
const kCardFooterHeight = 24.0;

/// [ProductCardLayout.wide]: nom ostidagi ikki qatorli tavsif balandligi.
const _kWideSubtitleHeight = 32.0;

/// [ProductCardLayout.wide]: rasmdan tashqari qolgan balandlik: ramka bo'shlig'i 16 +
/// rasm ostidagi 8 + nom ~20 + 2 + tavsif 32 + 6 + narx qatori 20 + 6 + boshqaruv qatori 36 + zaxira 4.
const _kWidePriceHeight = 20.0;
const _kWideCardChromeHeight =
    16 + 8 + 20 + 2 + _kWideSubtitleHeight + 6 + _kWidePriceHeight + 6 + 36 + 4.0;

/// Kartochkalar to'ri — vebdagi `grid-cols-2` bilan bir xil.
///
/// `mainAxisExtent` bilan balandlik ANIQ hisoblanadi: kartochka
/// balandligi = kvadrat rasm (kenglikka teng) + matn bloki (kenglikdan
/// MUSTAQIL) — qat'iy `childAspectRatio` keng ekranda bo'sh joy, tor
/// ekranda kesilish berardi.
///
/// [availableWidth] — to'r egallaydigan kenglik (yon chekinishlar
/// AYIRILGAN holda).
SliverGridDelegate productGridDelegate(
  double availableWidth, {
  int columns = 2,
  double spacing = 14,
  double extraHeight = 0,
  double imageAspectRatio = 1,
  ProductCardLayout layout = ProductCardLayout.tile,
}) {
  final cell = (availableWidth - spacing * (columns - 1)) / columns;
  if (layout == ProductCardLayout.wide) {
    // Ramkali kartochka: rasm kartaning ichki kengligiga (cell - 16) moslanadi.
    return SliverGridDelegateWithFixedCrossAxisCount(
      crossAxisCount: columns,
      crossAxisSpacing: spacing,
      mainAxisSpacing: spacing,
      mainAxisExtent: (cell - 16) / imageAspectRatio + _kWideCardChromeHeight,
    );
  }
  return SliverGridDelegateWithFixedCrossAxisCount(
    crossAxisCount: columns,
    crossAxisSpacing: spacing,
    mainAxisSpacing: 20,
    mainAxisExtent: cell / imageAspectRatio + _kCardTextHeight + extraHeight,
  );
}

/// Ekran kengligidan to'r kengligini hisoblaydi — barcha chaqiruv
/// joylarida yon chekinish 16px.
///
/// [extraHeight] — kartochkada qo'shimcha qator bo'lsa
/// ("Istaklarim" dagi restoran nomi): [kCardFooterHeight].
SliverGridDelegate productGridOf(BuildContext context,
        {double extraHeight = 0,
        double imageAspectRatio = 1,
        ProductCardLayout layout = ProductCardLayout.tile}) =>
    productGridDelegate(MediaQuery.sizeOf(context).width - 32,
        spacing: layout == ProductCardLayout.wide ? 12 : 14,
        extraHeight: extraHeight,
        imageAspectRatio: imageAspectRatio,
        layout: layout);

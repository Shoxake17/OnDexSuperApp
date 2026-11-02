/// Savat va buyurtma rasmiylashtirish ekranlarining UMUMIY bloklari.
///
/// ┌─ NEGA BU FAYL ──────────────────────────────────────────────────────┐
/// Restoran savati (`cart_screen.dart`) + rasmiylashtirish
/// (`checkout_screen.dart`) va Qurt savati + rasmiylashtirish
/// (`qurt_cart_screen.dart`, `qurt_checkout_screen.dart`) BIR XIL ko'rinadi.
/// Ilgari bu bloklar ikkala ekranda ham (restoranda — yopiq `_Card`,
/// `_CardRow`, `_TotalLine`, `_Bottom` ...) alohida yozilgan edi; qurt uchun
/// nusxa ko'chirish o'rniga bitta joyga chiqarildi. Bloklar faqat KO'RINISH:
/// ma'lumot, narx hisobi va tarmoq mantig'i chaqiruvchida qoladi
/// (restoran — `QuoteService`, qurt — `qurt_cart.dart`).
/// └─────────────────────────────────────────────────────────────────────┘
library;

import 'package:flutter/material.dart';

import '../api.dart';
import '../screens/catalog_screen.dart' show kBrand;
import 'common.dart';
import 'empty_state.dart';
import 'page_sheet.dart';
import 'qty_stepper.dart';

/// Cho'zinchoq (taxminan 2.4 : 1) mahsulot rasmi uchun kichik ko'rinish (Qurt).
const kWideThumbSize = Size(100, 42);

const _kPlaceholderBg = Color(0xFFF5F5F5);
const _kPlaceholderIcon = Color(0xFFBDBDBD);

/// Mahsulot rasmi: bo'sh bo'lsa joy egallovchi belgi.
Widget _orderImage(
  String url,
  Size size,
  BoxFit fit,
  IconData placeholderIcon,
  double radius,
) {
  final placeholder = Container(
    color: _kPlaceholderBg,
    child: Icon(placeholderIcon, size: 24, color: _kPlaceholderIcon),
  );
  return ClipRRect(
    borderRadius: BorderRadius.circular(radius),
    child: SizedBox(
      width: size.width,
      height: size.height,
      child:
          url.isEmpty
              ? placeholder
              : RemoteImage(url: url, fit: fit, placeholder: placeholder),
    ),
  );
}

// ═══════════════════════════════════════════════════════════════════
// SAVAT SAHIFASI
// ═══════════════════════════════════════════════════════════════════

/// Savatdagi bitta qator: rasm, nom (+ og'irlik), qator summasi, − N +.
class CartLineTile extends StatelessWidget {
  const CartLineTile({
    super.key,
    required this.name,
    required this.lineTotalTiyin,
    required this.qty,
    required this.onAdd,
    required this.onRemove,
    this.imageUrl = '',
    this.weightLabel = '',
    this.strikeTotalTiyin,
    this.placeholderIcon = Icons.restaurant_menu,
    this.imageFit = BoxFit.cover,
    this.imageSize = const Size(64, 64),
  });

  final String name;
  final int lineTotalTiyin;
  final int qty;
  final VoidCallback onAdd;
  final VoidCallback onRemove;
  final String imageUrl;

  /// Nomdan keyingi xira yozuv (masalan "250 g"). Bo'sh — chizilmaydi.
  final String weightLabel;

  /// Chegirmagacha bo'lgan summa; [lineTotalTiyin] dan katta bo'lsa chizilgan holda ko'rsatiladi.
  final int? strikeTotalTiyin;
  final IconData placeholderIcon;
  final BoxFit imageFit;
  final Size imageSize;

  @override
  Widget build(BuildContext context) {
    final discounted =
        strikeTotalTiyin != null && lineTotalTiyin < strikeTotalTiyin!;
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 12),
      child: Row(
        children: [
          _orderImage(imageUrl, imageSize, imageFit, placeholderIcon, 12),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text.rich(
                  TextSpan(
                    text: name,
                    style: const TextStyle(fontWeight: FontWeight.w600),
                    children: [
                      if (weightLabel.isNotEmpty)
                        TextSpan(
                          text: '  $weightLabel',
                          style: const TextStyle(
                            fontWeight: FontWeight.normal,
                            color: Color(0xFF9E9E9E),
                          ),
                        ),
                    ],
                  ),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 4),
                // QATOR SUMMASI (miqdor × narx) — birlik narxi emas.
                if (discounted)
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.end,
                    children: [
                      Text(
                        formatSum(lineTotalTiyin),
                        style: const TextStyle(
                          fontWeight: FontWeight.bold,
                          color: Color(0xFFE53935),
                        ),
                      ),
                      const SizedBox(width: 6),
                      Flexible(
                        child: Text(
                          formatSum(strikeTotalTiyin!),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                            fontSize: 12,
                            color: Color(0xFF757575),
                            decoration: TextDecoration.lineThrough,
                          ),
                        ),
                      ),
                    ],
                  )
                else
                  Text(
                    formatSum(lineTotalTiyin),
                    style: const TextStyle(fontWeight: FontWeight.bold),
                  ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          QtyStepper(qty: qty, onAdd: onAdd, onRemove: onRemove),
        ],
      ),
    );
  }
}

/// Savat sahifasining pastki paneli: "Buyurtma rasmiylashtirish  ·  summa".
///
/// [wrapButton] — tugmani o'rab oladigan qatlam (restoranda Shaddiy yordamchisi
/// aynan shu tugmani topishi uchun `AgentCheckoutAnchor`).
class CartBottomBar extends StatelessWidget {
  const CartBottomBar({
    super.key,
    required this.trailing,
    required this.onPressed,
    this.label = 'Buyurtma rasmiylashtirish',
    this.loading = false,
    this.error,
    this.onRetry,
    this.totalKey,
    this.buttonKey,
    this.wrapButton,
  });

  final Key? buttonKey;
  final String label;
  final String trailing;

  /// `null` — tugma o'chiq (masalan summa hali hisoblanmagan).
  final VoidCallback? onPressed;
  final bool loading;
  final String? error;
  final VoidCallback? onRetry;
  final Key? totalKey;
  final Widget Function(Widget button)? wrapButton;

  @override
  Widget build(BuildContext context) {
    final button = SizedBox(
      height: 52,
      width: double.infinity,
      child: FilledButton(
        key: buttonKey,
        style: FilledButton.styleFrom(
          backgroundColor: kBrand,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(14),
          ),
        ),
        onPressed: onPressed,
        child:
            loading
                ? const SizedBox(
                  width: 20,
                  height: 20,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: Colors.white,
                  ),
                )
                : Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Text(
                      label,
                      style: const TextStyle(
                        fontSize: 15.5,
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                    Text(
                      trailing,
                      key: totalKey,
                      style: const TextStyle(
                        fontSize: 15.5,
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                  ],
                ),
      ),
    );

    return SafeArea(
      child: Container(
        decoration: const BoxDecoration(
          color: Colors.white,
          border: Border(top: BorderSide(color: Color(0xFFE5E5E5))),
        ),
        padding: const EdgeInsets.fromLTRB(16, 10, 16, 10),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (error != null)
              Padding(
                padding: const EdgeInsets.only(bottom: 8),
                child: Row(
                  children: [
                    const Icon(
                      Icons.error_outline,
                      size: 16,
                      color: Color(0xFFB3261E),
                    ),
                    const SizedBox(width: 6),
                    Expanded(
                      child: Text(
                        error!,
                        style: const TextStyle(
                          fontSize: 12.5,
                          color: Color(0xFFB3261E),
                        ),
                      ),
                    ),
                    if (onRetry != null)
                      TextButton(
                        onPressed: onRetry,
                        child: const Text('Qayta'),
                      ),
                  ],
                ),
              ),
            wrapButton == null ? button : wrapButton!(button),
          ],
        ),
      ),
    );
  }
}

// ═══════════════════════════════════════════════════════════════════
// BUYURTMA RASMIYLASHTIRISH SAHIFASI
// ═══════════════════════════════════════════════════════════════════

/// Oq karta — maketdagi barcha bloklar shu ko'rinishda.
class OrderCard extends StatelessWidget {
  const OrderCard({super.key, required this.child});
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: const Color(0xFFEEEEEE)),
      ),
      clipBehavior: Clip.antiAlias,
      child: child,
    );
  }
}

/// Ikon + sarlavha + izoh qatori.
class OrderCardRow extends StatelessWidget {
  const OrderCardRow({
    super.key,
    required this.icon,
    required this.title,
    this.subtitle,
    this.note,
    this.trailing,
    this.onTap,
  });

  final IconData icon;
  final String title;
  final String? subtitle;
  final String? note;
  final Widget? trailing;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final row = Padding(
      padding: const EdgeInsets.all(16),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 22, color: kBrand),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: const TextStyle(
                    fontSize: 15.5,
                    fontWeight: FontWeight.bold,
                  ),
                ),
                if (subtitle != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 3),
                    child: Text(
                      subtitle!,
                      style: const TextStyle(
                        fontSize: 14.5,
                        color: Color(0xFF424242),
                      ),
                    ),
                  ),
                if (note != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 2),
                    child: Text(
                      note!,
                      style: const TextStyle(
                        fontSize: 13,
                        color: Color(0xFF9E9E9E),
                      ),
                    ),
                  ),
              ],
            ),
          ),
          if (trailing != null) ...[const SizedBox(width: 8), trailing!],
          if (onTap != null)
            const Padding(
              padding: EdgeInsets.only(left: 6),
              child: Icon(
                Icons.chevron_right,
                size: 20,
                color: Color(0xFF9E9E9E),
              ),
            ),
        ],
      ),
    );

    if (onTap == null) return row;
    return InkWell(onTap: onTap, child: row);
  }
}

/// "Buyurtma tafsilotlari" kartasining sarlavhasi.
class OrderDetailsHeader extends StatelessWidget {
  const OrderDetailsHeader({super.key, this.title = 'Buyurtma tafsilotlari'});
  final String title;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 4),
      child: Row(
        children: [
          const Icon(Icons.shopping_bag_outlined, size: 22, color: kBrand),
          const SizedBox(width: 12),
          Text(
            title,
            style: const TextStyle(fontSize: 16, fontWeight: FontWeight.bold),
          ),
        ],
      ),
    );
  }
}

/// Buyurtmadagi bitta pozitsiya — rasm, nom, tavsif, − N +, qator summasi.
class OrderItemTile extends StatelessWidget {
  const OrderItemTile({
    super.key,
    required this.name,
    required this.lineTotalTiyin,
    required this.qty,
    required this.onAdd,
    required this.onRemove,
    this.imageUrl = '',
    this.description = '',
    this.strikeTotalTiyin,
    this.placeholderIcon = Icons.restaurant_menu,
    this.imageFit = BoxFit.cover,
    this.imageSize = const Size(76, 76),
  });

  final String name;
  final String description;
  final int lineTotalTiyin;
  final int? strikeTotalTiyin;
  final int qty;
  final VoidCallback onAdd;
  final VoidCallback onRemove;
  final String imageUrl;
  final IconData placeholderIcon;
  final BoxFit imageFit;
  final Size imageSize;

  @override
  Widget build(BuildContext context) {
    final desc = description.trim();
    return Padding(
      padding: const EdgeInsets.all(16),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _orderImage(imageUrl, imageSize, imageFit, placeholderIcon, 12),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  name,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    fontSize: 15.5,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                if (desc.isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.only(top: 4),
                    child: Text(
                      desc,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        fontSize: 13,
                        height: 1.35,
                        color: Color(0xFF9E9E9E),
                      ),
                    ),
                  ),
              ],
            ),
          ),
          const SizedBox(width: 10),
          Column(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              QtyStepper(qty: qty, onAdd: onAdd, onRemove: onRemove),
              const SizedBox(height: 10),
              Text(
                formatSum(lineTotalTiyin),
                style: const TextStyle(
                  fontSize: 15.5,
                  fontWeight: FontWeight.bold,
                  color: kBrand,
                ),
              ),
              if (strikeTotalTiyin != null &&
                  lineTotalTiyin < strikeTotalTiyin!)
                Text(
                  formatSum(strikeTotalTiyin!),
                  style: const TextStyle(
                    fontSize: 12,
                    color: Color(0xFF9E9E9E),
                    decoration: TextDecoration.lineThrough,
                  ),
                ),
            ],
          ),
        ],
      ),
    );
  }
}

class OrderTotalLine extends StatelessWidget {
  const OrderTotalLine({
    super.key,
    required this.label,
    required this.value,
    this.color,
  });

  final String label;
  final String value;
  final Color? color;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Expanded(
            child: Text(
              label,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(fontSize: 14.5, color: Color(0xFF757575)),
            ),
          ),
          const SizedBox(width: 12),
          Text(
            value,
            style: TextStyle(
              fontSize: 14.5,
              color: color ?? const Color(0xFF424242),
            ),
          ),
        ],
      ),
    );
  }
}

/// "Jami to'lov:" qatori (punktir chiziqdan keyin).
class OrderGrandTotal extends StatelessWidget {
  const OrderGrandTotal({
    super.key,
    required this.valueTiyin,
    this.valueKey,
    this.label = 'Jami to\'lov:',
  });

  final int valueTiyin;
  final Key? valueKey;
  final String label;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(
          label,
          style: const TextStyle(fontSize: 16, fontWeight: FontWeight.bold),
        ),
        Text(
          formatSum(valueTiyin),
          key: valueKey,
          style: const TextStyle(
            fontSize: 18,
            fontWeight: FontWeight.bold,
            color: kBrand,
          ),
        ),
      ],
    );
  }
}

/// Maketdagi punktir chiziq.
class OrderDashedLine extends StatelessWidget {
  const OrderDashedLine({super.key});

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: 1,
      child: CustomPaint(painter: _DashedPainter(), child: Container()),
    );
  }
}

class _DashedPainter extends CustomPainter {
  @override
  void paint(Canvas canvas, Size size) {
    final paint =
        Paint()
          ..color = const Color(0xFFE0E0E0)
          ..strokeWidth = 1;
    const dash = 4.0;
    const gap = 4.0;
    var x = 0.0;
    while (x < size.width) {
      canvas.drawLine(Offset(x, 0), Offset(x + dash, 0), paint);
      x += dash + gap;
    }
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;
}

class OrderErrorBox extends StatelessWidget {
  const OrderErrorBox({super.key, required this.text});
  final String text;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: const Color(0xFFFBEAE9),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Row(
        children: [
          const Icon(Icons.error_outline, size: 18, color: Color(0xFFB3261E)),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              text,
              style: const TextStyle(color: Color(0xFFB3261E), fontSize: 13.5),
            ),
          ),
        ],
      ),
    );
  }
}

/// Rasmiylashtirish sahifasining pastki katta tugmasi (qulf belgisi + summa).
class OrderSubmitBar extends StatelessWidget {
  const OrderSubmitBar({
    super.key,
    required this.label,
    required this.ready,
    required this.submitting,
    required this.onPressed,
    this.buttonKey,
    this.wrapButton,
  });

  final Key? buttonKey;
  final String label;
  final bool ready;
  final bool submitting;
  final VoidCallback? onPressed;
  final Widget Function(Widget button)? wrapButton;

  @override
  Widget build(BuildContext context) {
    final button = SizedBox(
      height: 56,
      child: FilledButton(
        key: buttonKey,
        style: FilledButton.styleFrom(
          backgroundColor: kBrand,
          disabledBackgroundColor: kBrand.withValues(alpha: 0.5),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(16),
          ),
        ),
        onPressed: ready ? onPressed : null,
        child:
            submitting
                ? const SizedBox(
                  width: 22,
                  height: 22,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: Colors.white,
                  ),
                )
                : Row(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    const Icon(Icons.lock_outline, size: 18),
                    const SizedBox(width: 8),
                    // Summa katta bo'lsa matn qatorga sig'masdi: `scaleDown`.
                    Flexible(
                      child: FittedBox(
                        fit: BoxFit.scaleDown,
                        child: Text(
                          label,
                          maxLines: 1,
                          style: const TextStyle(
                            fontSize: 16.5,
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                      ),
                    ),
                    const SizedBox(width: 8),
                    const Icon(Icons.chevron_right, size: 22),
                  ],
                ),
      ),
    );
    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 8, 16, 12),
        child: wrapButton == null ? button : wrapButton!(button),
      ),
    );
  }
}

// ═══════════════════════════════════════════════════════════════════
// SUZUVCHI SAVAT TUGMASI
// ═══════════════════════════════════════════════════════════════════

/// Menyu va Qurt sahifasidagi suzuvchi "summa + savat" tugmasi.
class CartPill extends StatelessWidget {
  const CartPill({
    super.key,
    required this.totalTiyin,
    required this.count,
    required this.onTap,
  });

  final int totalTiyin;
  final int count;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return FloatingActionButton.extended(
      onPressed: onTap,
      backgroundColor: kBrand,
      foregroundColor: Colors.white,
      elevation: 4,
      extendedPadding: const EdgeInsets.symmetric(horizontal: 22),
      label: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            formatSum(totalTiyin),
            style: const TextStyle(fontSize: 15, fontWeight: FontWeight.bold),
          ),
          const SizedBox(width: 12),
          Stack(
            clipBehavior: Clip.none,
            children: [
              const Icon(Icons.shopping_basket_outlined, size: 22),
              Positioned(
                right: -7,
                top: -7,
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 4),
                  constraints: const BoxConstraints(minWidth: 16),
                  decoration: BoxDecoration(
                    color: Colors.black,
                    borderRadius: BorderRadius.circular(20),
                  ),
                  child: Text(
                    '$count',
                    textAlign: TextAlign.center,
                    style: const TextStyle(
                      fontSize: 10,
                      fontWeight: FontWeight.bold,
                      color: Colors.white,
                    ),
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

/// Sahifa pastidagi "Savatcha — N ta mahsulot  [Buyurtmani davom ettirish →]" paneli
/// (maket: image/qurtplus.png). Savat bo'sh bo'lsa chaqiruvchi uni chizmaydi.
class CartSummaryBar extends StatelessWidget {
  const CartSummaryBar({
    super.key,
    required this.count,
    required this.totalTiyin,
    required this.onTap,
    this.title = 'Savatcha',
    this.action = 'Buyurtmani davom ettirish',
  });

  final int count;
  final int totalTiyin;
  final VoidCallback onTap;
  final String title;
  final String action;

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 6, 16, 10),
        child: Material(
          color: kBrand,
          borderRadius: BorderRadius.circular(20),
          elevation: 3,
          child: InkWell(
            borderRadius: BorderRadius.circular(20),
            onTap: onTap,
            child: Padding(
              padding: const EdgeInsets.fromLTRB(14, 10, 10, 10),
              child: Row(
                children: [
                  Stack(
                    clipBehavior: Clip.none,
                    children: [
                      const Icon(Icons.shopping_basket_outlined, color: Colors.white, size: 26),
                      Positioned(
                        right: -8,
                        top: -8,
                        child: Container(
                          padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 1),
                          constraints: const BoxConstraints(minWidth: 18),
                          decoration: BoxDecoration(
                            color: Colors.white,
                            borderRadius: BorderRadius.circular(20),
                          ),
                          child: Text(
                            '$count',
                            textAlign: TextAlign.center,
                            style: const TextStyle(color: kBrand, fontSize: 10.5, fontWeight: FontWeight.w800),
                          ),
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(width: 14),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(
                          title,
                          style: const TextStyle(color: Colors.white, fontWeight: FontWeight.w800, fontSize: 15),
                        ),
                        Text(
                          '$count ta mahsulot · ${formatSum(totalTiyin)}',
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(color: Colors.white70, fontSize: 12),
                        ),
                      ],
                    ),
                  ),
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
                    decoration: BoxDecoration(
                      color: Colors.white,
                      borderRadius: BorderRadius.circular(16),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(
                          action,
                          style: const TextStyle(color: kBrand, fontWeight: FontWeight.w700, fontSize: 12.5),
                        ),
                        const SizedBox(width: 4),
                        const Icon(Icons.arrow_forward, size: 16, color: kBrand),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
// ═══════════════════════════════════════════════════════════════════
// SAHIFA QOBIQLARI
// ═══════════════════════════════════════════════════════════════════

/// Bo'sh savat (restoran va Qurt savati uchun bir xil).
class EmptyCartView extends StatelessWidget {
  const EmptyCartView({super.key});

  @override
  Widget build(BuildContext context) {
    return const Center(
      child: SingleChildScrollView(
        child: EmptyState(
          image: 'assets/empty/cart.png',
          title: 'Savatchangiz ovqat kutib zerikib qoldi 😔',
          subtitle:
              'Unga yordam bering — menyudan o\'zingizga eng '
              'yoqqanini tanlang!',
        ),
      ),
    );
  }
}

/// Rasmiylashtirish sahifasining yuqori paneli: yumaloq orqaga tugmasi + markazda sarlavha.
class OrderPageAppBar extends StatelessWidget implements PreferredSizeWidget {
  const OrderPageAppBar({super.key, this.title = 'Rasmiylashtirish'});
  final String title;

  @override
  Size get preferredSize => const Size.fromHeight(kToolbarHeight);

  @override
  Widget build(BuildContext context) {
    return PageAppBar(
      backgroundColor: const Color(0xFFFAFAFA),
      centerTitle: true,
      titleSpacing: 0,
      leadingWidth: 64,
      leading: Padding(
        padding: const EdgeInsets.only(left: 16),
        child: Material(
          color: Colors.white,
          borderRadius: BorderRadius.circular(14),
          child: InkWell(
            borderRadius: BorderRadius.circular(14),
            onTap: () => Navigator.of(context).pop(),
            child: const SizedBox(
              width: 44,
              height: 44,
              child: Icon(Icons.arrow_back, size: 22),
            ),
          ),
        ),
      ),
      titleWidget: Text(
        title,
        style: const TextStyle(fontSize: 21, fontWeight: FontWeight.bold),
      ),
    );
  }
}

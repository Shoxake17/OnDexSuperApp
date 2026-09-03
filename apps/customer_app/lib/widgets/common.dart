import 'package:flutter/material.dart';

import '../api.dart';
import '../screens/catalog_screen.dart' show kBrand;
import 'auth_flow.dart' show authSnack;

// ═══════════════════════════════════════════════════════════════════
// XABAR CHIQARISH
// ═══════════════════════════════════════════════════════════════════

/// Ekranlarga `snack('...')` metodini beradi.
///
/// To'rtta ekran o'zining `_snack` metodini saqlardi: uchtasi bir xil
/// edi, to'rtinchisi (`address_screen`) esa BOSHQACHA — u
/// `ScaffoldMessenger` ni to'g'ridan-to'g'ri chaqirar va xato rangini
/// umuman ishlatmasdi.
///
/// `mounted` tekshiruvi shu yerda: `await` dan keyin ekran yopilgan
/// bo'lsa xabar ko'rsatish xato beradi.
extension SnackMessenger on State {
  void snack(String msg, {bool error = false}) {
    if (!mounted) return;
    authSnack(context, msg, error: error);
  }
}

/// Ilova bo'ylab TAKRORLANADIGAN mayda bo'laklar — bir joyda.
///
/// Bu yerdagi har bir vidjet ilgari 2–4 ekranda alohida yozilgan edi
/// va nusxalar bir-biridan asta-sekin uzoqlashib ketgandi (rang,
/// o'lcham, matn farqi). Endi o'zgartirish faqat shu faylda qilinadi.

// ═══════════════════════════════════════════════════════════════════
// DUMALOQ IKON TUGMASI
// ═══════════════════════════════════════════════════════════════════

/// Oq doiradagi ikon tugmasi — xarita boshqaruvi, kartochkadagi
/// "+/−" va miqdor boshqaruvi shuni ishlatadi.
///
/// Ilgari uchta fayl (`address_screen`, `product_grid`, `qty_stepper`)
/// o'zining `_RoundButton` klassini saqlardi. Ular bir xil edi, faqat
/// soya va o'lcham farq qilardi — endi ular parametr.
class RoundIconButton extends StatelessWidget {
  const RoundIconButton({
    super.key,
    required this.icon,
    required this.onTap,
    this.size = 34,
    this.iconSize,
    this.elevation = 3,
    this.tooltip,
    this.iconColor = Colors.black,
  });

  final IconData icon;

  /// `null` — tugma o'chirilgan (bosilmaydi).
  final VoidCallback? onTap;

  final double size;

  /// Berilmasa o'lchamdan hisoblanadi.
  final double? iconSize;

  final double elevation;
  final String? tooltip;
  final Color iconColor;

  @override
  Widget build(BuildContext context) {
    const disabled = Color(0xFFBDBDBD);

    final btn = Material(
      color: Colors.white,
      shape: const CircleBorder(),
      elevation: elevation,
      child: InkWell(
        customBorder: const CircleBorder(),
        onTap: onTap,
        child: SizedBox(
          width: size,
          height: size,
          child: Icon(
            icon,
            size: iconSize ?? size * 0.56,
            color: onTap == null ? disabled : iconColor,
          ),
        ),
      ),
    );
    return tooltip == null ? btn : Tooltip(message: tooltip!, child: btn);
  }
}

// ═══════════════════════════════════════════════════════════════════
// OFLAYN BELGISI
// ═══════════════════════════════════════════════════════════════════

/// "Yangilab bo'lmadi — saqlangan nusxa" tasmasi.
///
/// Katalog va menyu ekranlarida ikkita alohida klass bor edi
/// (`_OfflineNotice`, `_OfflineStrip`) — bezagi bayt-bayt bir xil,
/// faqat matni va tashqi chekinishi farq qilardi.
class OfflineNotice extends StatelessWidget {
  const OfflineNotice({
    super.key,
    required this.message,
    this.margin = const EdgeInsets.fromLTRB(16, 4, 16, 8),
  });

  final String message;
  final EdgeInsets margin;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: margin,
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: const Color(0xFFFFF4E5),
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: const Color(0xFFFFD9A8)),
      ),
      child: Row(
        children: [
          const Icon(Icons.cloud_off, size: 16, color: Color(0xFF9A5B00)),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              message,
              style: const TextStyle(fontSize: 12.5, color: Color(0xFF9A5B00)),
            ),
          ),
        ],
      ),
    );
  }
}

// ═══════════════════════════════════════════════════════════════════
// XATO HOLATI
// ═══════════════════════════════════════════════════════════════════

/// Yuklab bo'lmagan ekran uchun yagona ko'rinish: ikon, matn va
/// qayta urinish tugmasi.
///
/// Ilgari to'rtta ekran to'rt xil chizardi (ikkita `_ErrorView` klassi
/// va ikkita ichma-ich yozilgan `ListView` bloki), matn ham
/// "Qayta urinish" / "Qaytadan urinish" deb ikki xil edi.
class ErrorView extends StatelessWidget {
  const ErrorView({
    super.key,
    required this.message,
    required this.onRetry,
    this.icon = Icons.error_outline,
    this.retryLabel = 'Qayta urinish',
  });

  final String message;

  /// `null` — qayta urinish tugmasi ko'rsatilmaydi.
  final VoidCallback? onRetry;

  final IconData icon;
  final String retryLabel;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 40, color: const Color(0xFF9E9E9E)),
            const SizedBox(height: 12),
            Text(
              message,
              textAlign: TextAlign.center,
              style: const TextStyle(color: Color(0xFF757575)),
            ),
            if (onRetry != null) ...[
              const SizedBox(height: 16),
              TextButton(
                onPressed: onRetry,
                child: Text(
                  retryLabel,
                  style: const TextStyle(
                      color: kBrand, fontWeight: FontWeight.w600),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

/// Aylantiriladigan ro'yxatlar uchun ([RefreshIndicator] ichida).
///
/// `Center` ishlatilsa ro'yxat aylanmaydi va pastga tortib yangilash
/// ishlamay qoladi.
class ErrorViewList extends StatelessWidget {
  const ErrorViewList({
    super.key,
    required this.message,
    required this.onRetry,
    this.icon = Icons.error_outline,
  });

  final String message;
  final VoidCallback? onRetry;
  final IconData icon;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) => SingleChildScrollView(
        physics: const AlwaysScrollableScrollPhysics(),
        child: ConstrainedBox(
          constraints: BoxConstraints(minHeight: constraints.maxHeight),
          child: ErrorView(message: message, onRetry: onRetry, icon: icon),
        ),
      ),
    );
  }
}

// ═══════════════════════════════════════════════════════════════════
// OQ KARTA
// ═══════════════════════════════════════════════════════════════════

/// Ramkali oq karta — hisob bloklari, bildirishnoma va profil
/// qatorlari shu ko'rinishda.
class AppCard extends StatelessWidget {
  const AppCard({
    super.key,
    required this.child,
    this.padding,
    this.margin,
    this.radius = 16,
  });

  final Widget child;
  final EdgeInsets? padding;
  final EdgeInsets? margin;
  final double radius;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: margin,
      padding: padding,
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(radius),
        border: Border.all(color: const Color(0xFFE5E5E5)),
      ),
      clipBehavior: padding == null ? Clip.antiAlias : Clip.none,
      child: child,
    );
  }
}

// ═══════════════════════════════════════════════════════════════════
// TARMOQDAN KELADIGAN RASM
// ═══════════════════════════════════════════════════════════════════

/// Serverdagi rasm. Yo'l bo'sh yoki rasm yuklanmasa — EKRAN BUZILMAYDI,
/// o'rniga [placeholder] chiziladi.
///
/// `Image.network(..., errorBuilder: ...)` naqshi o'nta faylda qo'lda
/// takrorlanardi va ba'zilarida `errorBuilder` umuman yo'q edi — ya'ni
/// yiqilgan rasm qizil xato quticha bo'lib chiqardi.
class RemoteImage extends StatelessWidget {
  const RemoteImage({
    super.key,
    required this.url,
    this.width,
    this.height,
    this.fit = BoxFit.cover,
    this.placeholder,
  });

  /// Server bergan yo'l — nisbiy ham, to'liq URL ham bo'lishi mumkin
  /// ([fullImageUrl] ikkalasini ham to'g'ri ishlaydi).
  final String url;

  final double? width;
  final double? height;
  final BoxFit fit;
  final Widget? placeholder;

  @override
  Widget build(BuildContext context) {
    final empty = placeholder ?? const SizedBox.shrink();
    if (url.trim().isEmpty) {
      return SizedBox(width: width, height: height, child: empty);
    }
    return Image.network(
      fullImageUrl(url),
      width: width,
      height: height,
      fit: fit,
      // Rasm kelguncha JOY EGALLANADI — aks holda ro'yxatlar yuklanish
      // paytida sakrab qolardi. Ilgari bu faqat kartochkada bor edi.
      loadingBuilder: (context, child, progress) => progress == null
          ? child
          : SizedBox(width: width, height: height),
      errorBuilder: (_, __, ___) =>
          SizedBox(width: width, height: height, child: empty),
    );
  }
}

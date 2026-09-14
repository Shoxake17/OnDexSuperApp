import 'package:flutter/material.dart';

import '../screens/catalog_screen.dart' show kBrand;
import 'auth_flow.dart' show authSnack;

// Dumaloq ikon tugmasi va tarmoq rasmi endi `packages/ondex_menu` da —
// affitsiant ilovasining menyusi ham AYNI vidjetlarni ishlatadi. Mavjud
// importlar (`widgets/common.dart`) buzilmasin deb shu yerdan qayta
// eksport qilinadi.
export 'package:ondex_menu/ondex_menu.dart' show RoundIconButton, RemoteImage;

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


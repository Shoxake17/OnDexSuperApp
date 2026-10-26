import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';

import 'config.dart';

/// Huquqiy hujjatlar — BARCHA OnDex ilovalari uchun yagona manba
/// (bug.md 70-band).
///
/// ┌─ NEGA UMUMIY PAKETDA ──────────────────────────────────────────────┐
/// Maxfiylik siyosati va ommaviy oferta beshta ilovaning HAMMASIDA
/// ko'rinishi kerak:
///
///   * mijoz ilovasi   — Play Store/App Store TALABI;
///   * kuryer ilovasi  — u ham do'konga chiqadi;
///   * affitsiant      — u ham do'konga chiqadi;
///   * panellar        — xodim shartlarni bilishi kerak.
///
/// Manzil beshta joyda qo'lda yozilsa, ular ajralib ketardi va
/// bittasida eski havola qolib ketardi — bu esa do'kon ko'rigida
/// rad javobiga olib keladi.
///
/// Manzil `webAppUrl` dan quriladi, ya'ni dev/prod avtomatik to'g'ri
/// bo'ladi (`--dart-define=ONDEX_WEB_URL=...`).
/// └────────────────────────────────────────────────────────────────────┘
class LegalLinks {
  const LegalLinks._();

  /// Maxfiylik siyosati.
  static String get privacyUrl => '$webAppUrl/maxfiylik';

  /// Ommaviy oferta (foydalanish shartlari).
  static String get offerUrl => '$webAppUrl/oferta';
}

/// Huquqiy hujjatlarni brauzerda ochadi.
///
/// `mode: externalApplication` — ATAYLAB: hujjat ilova ichidagi
/// WebView'da emas, TIZIM brauzerida ochilishi kerak. Sabab ikkita:
///
///   1. foydalanuvchi manzil satrini ko'radi va hujjat haqiqatan
///      `ondex.uz` dan kelayotganini tekshira oladi;
///   2. do'kon ko'ruvchisi havolani tashqarida ocha oladi.
///
/// Ochib bo'lmasa `false` qaytaradi — chaqiruvchi foydalanuvchiga
/// aytadi. Jimgina o'tib ketish yaramaydi: bosildi-yu hech narsa
/// bo'lmadi degan holat "buzuq ilova" taassurotini qoldiradi.
Future<bool> openLegalUrl(String url) async {
  try {
    return await launchUrl(
      Uri.parse(url),
      mode: LaunchMode.externalApplication,
    );
  } catch (_) {
    return false;
  }
}

/// Hujjatni ochish usuli — `LegalSection` tashqarisidan beriladi.
///
/// ┌─ NEGA TASHQARIDAN ──────────────────────────────────────────────────┐
/// `ondex_core` faqat mobil ilovalarga emas, `admin_panel`ga (Windows
/// desktop) ham bog'langan. Agar shu yerning o'zida `webview_flutter`
/// ishlatilsa, desktop build ham o'sha paketga qaram bo'lib qolardi —
/// u yerda esa butunlay boshqa WebView yechimi bor (`webview_windows`).
///
/// Shuning uchun WebView ekrani HAR BIR mobil ilovaning O'ZIDA yoziladi
/// (`legal_webview_screen.dart`), bu yerga esa faqat CHAQIRISH usuli
/// ko'rsatiladi. `onOpen` berilmasa — eski xatti-harakat (tizim
/// brauzeri) saqlanadi, ya'ni hech narsa buzilmaydi.
/// └────────────────────────────────────────────────────────────────────┘
typedef LegalOpener = Future<void> Function(
  BuildContext context,
  String title,
  String url,
);

/// Profil/sozlamalar ekranlariga qo'yiladigan tayyor bo'lim.
///
/// Beshta ilovada bir xil ko'rinadi va bir xil manzilga boradi.
/// Ilovaning o'z dizayn tizimi bo'lgani uchun ranglar TASHQARIDAN
/// beriladi (`iconColor`), tuzilma esa umumiy.
class LegalSection extends StatelessWidget {
  const LegalSection({
    super.key,
    this.iconColor,
    this.showDivider = true,
    this.onOpen,
  });

  final Color? iconColor;
  final bool showDivider;

  /// Berilsa, hujjat shu orqali (odatda ILOVA ICHIDAGI WebView) ochiladi.
  /// Berilmasa — tizim brauzerida (`openLegalUrl`).
  final LegalOpener? onOpen;

  @override
  Widget build(BuildContext context) {
    final color = iconColor ?? Theme.of(context).colorScheme.primary;
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        _LegalTile(
          icon: Icons.description_outlined,
          title: 'Ommaviy oferta',
          subtitle: 'Foydalanish shartlari',
          color: color,
          url: LegalLinks.offerUrl,
          onOpen: onOpen,
        ),
        if (showDivider) const Divider(height: 1, indent: 56),
        _LegalTile(
          icon: Icons.privacy_tip_outlined,
          title: 'Maxfiylik siyosati',
          subtitle: 'Ma\'lumotlaringiz qanday himoyalanadi',
          color: color,
          url: LegalLinks.privacyUrl,
          onOpen: onOpen,
        ),
      ],
    );
  }
}

class _LegalTile extends StatelessWidget {
  const _LegalTile({
    required this.icon,
    required this.title,
    required this.subtitle,
    required this.color,
    required this.url,
    required this.onOpen,
  });

  final IconData icon;
  final String title;
  final String subtitle;
  final Color color;
  final String url;
  final LegalOpener? onOpen;

  @override
  Widget build(BuildContext context) {
    return ListTile(
      leading: Icon(icon, color: color),
      title: Text(title),
      subtitle: Text(subtitle, style: const TextStyle(fontSize: 12)),
      // Ilova ichida ochilsa — oddiy navigatsiya belgisi; tashqi
      // brauzerga tushsa (`onOpen` berilmagan) — buni ham ko'rsatamiz.
      trailing: Icon(
        onOpen != null ? Icons.chevron_right : Icons.open_in_new,
        size: onOpen != null ? 20 : 18,
      ),
      onTap: () async {
        final open = onOpen;
        if (open != null) {
          await open(context, title, url);
          return;
        }
        final ok = await openLegalUrl(url);
        if (ok || !context.mounted) return;
        // Ochilmadi — SABABINI aytamiz va manzilni ko'rsatamiz,
        // shunda foydalanuvchi uni qo'lda ocha oladi.
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Brauzer ochilmadi. Manzil: $url'),
            duration: const Duration(seconds: 6),
          ),
        );
      },
    );
  }
}

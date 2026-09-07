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
  });

  final Color? iconColor;
  final bool showDivider;

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
        ),
        if (showDivider) const Divider(height: 1, indent: 56),
        _LegalTile(
          icon: Icons.privacy_tip_outlined,
          title: 'Maxfiylik siyosati',
          subtitle: 'Ma\'lumotlaringiz qanday himoyalanadi',
          color: color,
          url: LegalLinks.privacyUrl,
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
  });

  final IconData icon;
  final String title;
  final String subtitle;
  final Color color;
  final String url;

  @override
  Widget build(BuildContext context) {
    return ListTile(
      leading: Icon(icon, color: color),
      title: Text(title),
      subtitle: Text(subtitle, style: const TextStyle(fontSize: 12)),
      trailing: const Icon(Icons.open_in_new, size: 18),
      onTap: () async {
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

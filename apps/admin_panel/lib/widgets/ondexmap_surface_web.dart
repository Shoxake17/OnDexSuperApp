import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';

/// WEB — OnDexMap muharriri iframe ichida KO'RSATILMAYDI.
///
/// ┌─ NEGA IFRAME EMAS ─────────────────────────────────────────────────┐
/// Admin paneli web'da HTTPS orqali ochiladi, OnDexMap muharriri esa
/// `http://127.0.0.1:8091` da. Brauzer HTTPS sahifa ichiga HTTP
/// resursni qo'yishga ruxsat bermaydi (mixed content) va iframe
/// JIMGINA bo'sh qoladi — konsoldan boshqa hech qayerda sabab
/// ko'rinmaydi.
///
/// Shu sabab bu yerda ataylab iframe emas, TUSHUNTIRISH va tashqi
/// oynada ochish tugmasi turadi. Muharrir baribir faqat shu
/// kompyuterda ishlaydi (OnDexMap admin serveri 127.0.0.1 ga
/// bog'langan), ya'ni uni yangi oynada ochish to'liq ish beradi.
/// └────────────────────────────────────────────────────────────────────┘
///
/// ┌─ NEGA `url_launcher`, `dart:html` EMAS ────────────────────────────┐
/// Avval bu yerda `dart:html` ishlatilgan edi va `flutter analyze`
/// uni eskirgan deb belgiladi (`deprecated_member_use`). `url_launcher`
/// loyihada allaqachon bor, barcha platformalarda ishlaydi va
/// `avoid_web_libraries_in_flutter` lint ogohlantirishini ham
/// keltirib chiqarmaydi.
/// └────────────────────────────────────────────────────────────────────┘
class OndexMapSurface extends StatelessWidget {
  final String url;

  const OndexMapSurface({super.key, required this.url});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.open_in_new, size: 48),
            const SizedBox(height: 16),
            const Text(
              'OnDexMap muharriri alohida oynada ochiladi',
              style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 12),
            const Text(
              'Brauzer versiyasida muharrir panel ichiga joylashtirilmaydi:\n'
              'HTTPS sahifa ichiga HTTP manzilni qo\'yib bo\'lmaydi.\n'
              'Desktop ilovada u to\'g\'ridan-to\'g\'ri shu yerda ochiladi.',
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 20),
            SelectableText(url, style: const TextStyle(fontSize: 12)),
            const SizedBox(height: 20),
            FilledButton.icon(
              onPressed: () => launchUrl(
                Uri.parse(url),
                webOnlyWindowName: 'ondexmap',
              ),
              icon: const Icon(Icons.open_in_new),
              label: const Text('Muharrirni ochish'),
            ),
          ],
        ),
      ),
    );
  }
}

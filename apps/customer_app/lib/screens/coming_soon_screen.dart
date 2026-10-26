import 'package:flutter/material.dart';

import '../widgets/page_sheet.dart';
import '../widgets/sheet_page.dart';
import 'catalog_screen.dart' show kBrand;

/// Hali qurilmagan xizmat bo'limlari uchun UMUMIY "tez orada" ekrani.
///
/// ┌─ NEGA EKRAN BOR, LEKIN FUNKSIYA YO'Q ─────────────────────────────┐
/// Bosh sahifadagi bir nechta karta (Do'kon, Computer Club, ...) ilgari
/// faqat "tez orada" degan snackbar ko'rsatardi (`WalletScreen`
/// izohidagi sabab bilan bir xil). Lekin karta "ishlaydigandek"
/// ko'rinardi — bosilganda ekran ochilmasligi foydalanuvchini
/// chalg'itardi.
///
/// Har bir xizmat uchun shu ekran bitta marta yoziladi — faqat
/// sarlavha, matn va maskot rasmi farq qiladi (`_Service` ro'yxatidagi
/// har bir "tez orada" karta shu vidjetni chaqiradi).
/// └───────────────────────────────────────────────────────────────────┘
class ComingSoonScreen extends StatelessWidget {
  const ComingSoonScreen({
    super.key,
    required this.title,
    required this.heading,
    required this.subtitle,
    required this.mascotAsset,
    this.mascotWidth = 210,
  });

  /// Sahifa sarlavhasi (yuqori panel).
  final String title;

  /// Katta sarlavha — "... qismi hali mavjud emas".
  final String heading;

  /// Ikki qatorli tavsif. `maxLines: 2` bilan birga ishlaydi, shuning
  /// uchun QISQA bo'lishi kerak — tor ekranda ham ikki qatordan
  /// oshmasligi uchun.
  final String subtitle;

  final String mascotAsset;
  final double mascotWidth;

  @override
  Widget build(BuildContext context) {
    return SheetPage(
      child: Scaffold(
        backgroundColor: Colors.white,
        appBar: PageAppBar(
          titleWidget: Text(
            title,
            style: const TextStyle(fontSize: 22, fontWeight: FontWeight.bold),
          ),
        ),
        // ┌─ TEPADA TEXLANIB QOLMASIN ───────────────────────────────┐
        // Oddiy `SingleChildScrollView` tarkibni YUQORIGA yopishtiradi:
        // ekran bo'sh joyi hammasi pastda qolib, maskot sarlavha
        // ostida "osilib" qolgandek ko'rinardi. `ConstrainedBox` +
        // `Center` tarkibni ekran balandligida O'RTAGA tortadi —
        // kichik ekranda esa (sig'masa) baribir skroll qiladi.
        // └─────────────────────────────────────────────────────────┘
        body: LayoutBuilder(
          builder: (context, constraints) {
            return SingleChildScrollView(
              padding: const EdgeInsets.fromLTRB(24, 0, 24, 24),
              child: ConstrainedBox(
                constraints: BoxConstraints(minHeight: constraints.maxHeight),
                child: Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Image.asset(
                        mascotAsset,
                        width: mascotWidth,
                        fit: BoxFit.contain,
                      ),
                      const SizedBox(height: 4),
                      Text(
                        heading,
                        textAlign: TextAlign.center,
                        style: const TextStyle(
                          fontSize: 20,
                          fontWeight: FontWeight.bold,
                          color: Color(0xFF111827),
                        ),
                      ),
                      const SizedBox(height: 10),
                      Text(
                        subtitle,
                        textAlign: TextAlign.center,
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                          fontSize: 14,
                          height: 1.45,
                          color: Color(0xFF6B7280),
                        ),
                      ),
                      const SizedBox(height: 24),
                      const _ComingSoonCard(),
                    ],
                  ),
                ),
              ),
            );
          },
        ),
      ),
    );
  }
}

/// "Yangi imkoniyatlar tez orada!" bloki (`image/Do'kon.png`).
class _ComingSoonCard extends StatelessWidget {
  const _ComingSoonCard();

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: const Color(0xFFFDF1E7),
        borderRadius: BorderRadius.circular(18),
      ),
      // `center` — ikonalar matn blokining O'RTASIGA tortiladi. `start`
      // bilan ular tepaga yopishib qolardi, chunki matn bloki (sarlavha +
      // tavsif) ikonadan balandroq.
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          Container(
            width: 44,
            height: 44,
            decoration: const BoxDecoration(
              color: Colors.white,
              shape: BoxShape.circle,
            ),
            child: const Icon(
              Icons.rocket_launch_outlined,
              color: kBrand,
              size: 22,
            ),
          ),
          const SizedBox(width: 14),
          const Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Yangi imkoniyatlar\ntez orada!',
                  style: TextStyle(
                    fontWeight: FontWeight.bold,
                    fontSize: 14.5,
                    color: Color(0xFF111827),
                  ),
                ),
                SizedBox(height: 4),
                // Qisqartirilgan matn + `maxLines` — tor ekranda ham
                // ANIQ ikki qatorda qoladi (avval uchinchi qatorga
                // "toshib ketardi").
                Text(
                  'Sizlar uchun qulay va foydali\n'
                  'funksiyalar ustida ishlayapmiz.',
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontSize: 12.5,
                    color: Color(0xFF6B7280),
                    height: 1.4,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          Container(
            width: 36,
            height: 36,
            decoration: const BoxDecoration(
              color: Colors.white,
              shape: BoxShape.circle,
            ),
            child: const Icon(Icons.arrow_forward, color: kBrand, size: 18),
          ),
        ],
      ),
    );
  }
}

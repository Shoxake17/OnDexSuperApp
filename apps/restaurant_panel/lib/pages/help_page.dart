import 'package:flutter/material.dart';

import '../api.dart'; // `ondex_core` ni qayta eksport qiladi (LegalSection)
// `kPagePadding` shu yerda (`page_header.dart`), `theme.dart` da emas.
import '../widgets/page_header.dart';

/// Yordam markazi — huquqiy hujjatlar va aloqa.
///
/// ┌─ NEGA `ComingSoonPage` O'RNIGA (bug.md 70-band) ───────────────────┐
/// Bu bo'lim avval butunlay bo'sh edi ("tez orada"). Ayni paytda
/// restoran xodimi ishlayotgan shartlarni — ommaviy ofertani va
/// maxfiylik siyosatini — KO'RA OLISHI kerak: u platforma orqali
/// mijozning shaxsiy ma'lumotlariga (ism, telefon, manzil) kirish
/// huquqiga ega.
///
/// Ya'ni bu sahifa "tez orada" emas, ENG KAMIDA huquqiy hujjatlarni
/// ko'rsatishi kerak edi. Qolgan qismlari (savol-javob, chat) keyin
/// qo'shiladi va ular uchun sahifa allaqachon tayyor bo'ladi.
/// └────────────────────────────────────────────────────────────────────┘
class HelpPage extends StatelessWidget {
  const HelpPage({super.key});

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      padding: kPagePadding,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const PageHeader(
            title: 'Yordam markazi',
            subtitle: 'Hujjatlar va bog\'lanish',
          ),
          const SizedBox(height: 20),

          _card(
            title: 'Huquqiy hujjatlar',
            child: const LegalSection(),
          ),
          const SizedBox(height: 16),

          _card(
            title: 'Bog\'lanish',
            child: const Column(
              children: [
                ListTile(
                  leading: Icon(Icons.phone_outlined),
                  title: Text('+998 90 278 42 07'),
                  subtitle: Text('Dushanba–Yakshanba, 09:00–21:00',
                      style: TextStyle(fontSize: 12)),
                ),
                Divider(height: 1, indent: 56),
                ListTile(
                  leading: Icon(Icons.mail_outline),
                  title: Text('info@ondex.uz'),
                  subtitle: Text('Umumiy savollar',
                      style: TextStyle(fontSize: 12)),
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),

          // Hali qurilmagan qismlar ATAYLAB shu yerda aytiladi:
          // xodim nimani kutishini bilsin.
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: const Color(0xFFF5F5F5),
              borderRadius: BorderRadius.circular(12),
            ),
            child: const Text(
              'Savol-javob bo\'limi va ilova ichidagi qo\'llab-quvvatlash '
              'chati tez orada qo\'shiladi. Hozircha yuqoridagi telefon '
              'yoki pochta orqali murojaat qiling.',
              style: TextStyle(fontSize: 13, color: Color(0xFF616161)),
            ),
          ),
        ],
      ),
    );
  }

  Widget _card({required String title, required Widget child}) {
    return Container(
      width: double.infinity,
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: const Color(0xFFEEEEEE)),
      ),
      clipBehavior: Clip.antiAlias,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 16, 16, 4),
            child: Text(title,
                style: const TextStyle(
                    fontSize: 15, fontWeight: FontWeight.bold)),
          ),
          child,
        ],
      ),
    );
  }
}

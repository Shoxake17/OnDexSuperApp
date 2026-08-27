import 'package:flutter/material.dart';

import '../widgets/page_sheet.dart';
import '../widgets/sheet_page.dart';

/// Hamyon — HALI QURILMAGAN.
///
/// Veb bilan parity: `apps/web/app/(food)/wallet/page.tsx`.
///
/// ┌─ NEGA EKRAN BOR, LEKIN FUNKSIYA YO'Q ─────────────────────────────┐
/// Maketda (image/restarant.png) sarlavhada hamyon ikoni turibdi va u
/// interfeysning bir qismi. Lekin backendda balans, tranzaksiya
/// daftari yoki to'lov tizimi UMUMAN yo'q — na jadval, na endpoint.
///
/// Ikonni bosganda hech narsa bo'lmasligi yoki bo'sh ekran ochilishi
/// eng yomon variant: mijoz buni ilovaning nosozligi deb biladi.
/// Shuning uchun ekran holatni OCHIQ aytadi.
///
/// Haqiqiy hamyon qurilganda shu fayl almashtiriladi — ikon, marshrut
/// va sarlavhadagi joy allaqachon tayyor bo'ladi.
/// └───────────────────────────────────────────────────────────────────┘
class WalletScreen extends StatelessWidget {
  const WalletScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return SheetPage(
        child: Scaffold(
      backgroundColor: Colors.white,
      appBar: const PageAppBar(
        titleWidget: Text('Hamyon',
            style: TextStyle(fontSize: 22, fontWeight: FontWeight.bold)),
      ),
      body: Center(
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 40),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 80,
                height: 80,
                decoration: const BoxDecoration(
                  shape: BoxShape.circle,
                  color: Color(0xFFF5F5F5),
                ),
                child: const Icon(Icons.account_balance_wallet_outlined,
                    size: 36, color: Color(0xFF9E9E9E)),
              ),
              const SizedBox(height: 20),
              const Text('Tayyorlanmoqda',
                  style:
                      TextStyle(fontSize: 18, fontWeight: FontWeight.w600)),
              const SizedBox(height: 8),
              const Text(
                'Balans, to\'ldirish va bonuslar — to\'lov tizimi '
                '(Payme/Click) ulanganidan keyin shu yerda bo\'ladi. '
                'Hozircha buyurtma uchun to\'lov kuryerga naqd yoki '
                'karta bilan amalga oshiriladi.',
                textAlign: TextAlign.center,
                style: TextStyle(
                    fontSize: 13.5, height: 1.55, color: Color(0xFF757575)),
              ),
            ],
          ),
        ),
      ),
    ));
  }
}

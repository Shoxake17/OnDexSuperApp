import 'package:flutter/material.dart';

/// Native tab ekranlari (Istaklarim / Buyurtmalarim / Profil) uchun
/// UMUMIY qobiq — `apps/web/app/(food)/mobile-sheet.tsx` bilan BIR XIL
/// ko'rinish beradi: tepada juda kichik bo'shliq, keyin yuqori
/// burchaklari 20px dumaloqlangan "karta".
///
/// ┌─ RANGLAR NEGA O'ZGARDI ───────────────────────────────────────────┐
/// Avval bu yerda QORONG'I ranglar qat'iy yozilgan edi (#121212 fon,
/// #1A1A1A karta). Ular ilova qorong'i mavzuda va sahifalar WebView
/// ichida bo'lgan davrdan qolgan.
///
/// Ilova YORUG' mavzuga o'tgach (`main.dart`: `Brightness.light`,
/// `scaffoldBackgroundColor: Colors.white`) bu uchta ekran oq
/// ilovaning ichida QORA karta bo'lib chizila boshladi — Istaklarim,
/// Buyurtmalarim va Profil. Kod o'zgarmagani uchun bu jimgina sodir
/// bo'ldi.
///
/// Endi ranglar vebdagi YORUG' rejim bilan bir xil:
///   orqa fon  #E5E5E5  (`bg-neutral-200`)
///   karta     oq       (`bg-white`)
/// └───────────────────────────────────────────────────────────────────┘
///
/// Material `AppBar` ATAYLAB ishlatilmaydi — u o'z foni/elevatsiyasi
/// bilan kartaning dumaloq burchagini buzadi. Sarlavha oddiy matn
/// sifatida kartaning ichida chiziladi (veb sahifalardagi kabi).
class SheetScaffold extends StatelessWidget {
  final String title;
  final Widget child;

  /// Sarlavha o'ng tomonidagi ixtiyoriy tugmalar (masalan "tozalash").
  final List<Widget> actions;

  const SheetScaffold({
    super.key,
    required this.title,
    required this.child,
    this.actions = const [],
  });

  static const _card = Colors.white;

  @override
  Widget build(BuildContext context) {
    // ┌─ TAB EKRANLARIDA DUMALOQ BURCHAK YO'Q ────────────────────────┐
    // Istaklarim / Buyurtmalarim / Profil — bosh sahifa bilan bir xil
    // qatlamda: ular ILOVANING O'ZI, ustiga ochilgan panel emas.
    // Dumaloq burchak faqat PUSH qilingan sahifalarda (menyu, savat,
    // checkout) — u yerda karta ostidagi sahifa ko'rinib turadi.
    // └───────────────────────────────────────────────────────────────┘
    return DecoratedBox(
      decoration: const BoxDecoration(color: _card),
      child: Container(
          color: _card,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Padding(
                padding: EdgeInsets.fromLTRB(20, 14, actions.isEmpty ? 20 : 8, 10),
                child: Row(
                  children: [
                    Expanded(
                      child: Text(
                        title,
                        style: const TextStyle(
                          fontSize: 22,
                          fontWeight: FontWeight.bold,
                          // Rang ATAYLAB aniq: qorong'i mavzudan
                          // qolgan oq matn yorug' kartada ko'rinmay
                          // qolgan edi.
                          color: Color(0xFF171717),
                        ),
                      ),
                    ),
                    ...actions,
                  ],
                ),
              ),
              Expanded(child: child),
            ],
          )),
    );
  }
}

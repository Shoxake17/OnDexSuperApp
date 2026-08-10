import 'package:flutter/material.dart';

/// Native tab ekranlari (Istaklarim / Buyurtmalarim / Profil) uchun
/// UMUMIY qobiq — WebView ichidagi Next.js sahifalari bilan BIR XIL
/// ko'rinish beradi: tepada juda kichik bo'shliq, keyin yuqori
/// burchaklari 20px dumaloqlangan "karta".
///
/// Ranglar `apps/web/app/(food)/mobile-sheet.tsx` bilan atayin bir xil:
///   orqa fon  #121212  (= main.dart'dagi scaffoldBackgroundColor)
///   karta     #1A1A1A
/// Ikkalasi mos bo'lmasa, native tab'dan WebView tab'iga o'tganda
/// ko'zga tashlanadigan sakrash/chok sezilardi.
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

  static const _backdrop = Color(0xFF121212);
  static const _card = Color(0xFF1A1A1A);

  @override
  Widget build(BuildContext context) {
    return Container(
      color: _backdrop,
      padding: const EdgeInsets.only(top: 4),
      child: ClipRRect(
        borderRadius: const BorderRadius.vertical(top: Radius.circular(20)),
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
                        ),
                      ),
                    ),
                    ...actions,
                  ],
                ),
              ),
              Expanded(child: child),
            ],
          ),
        ),
      ),
    );
  }
}

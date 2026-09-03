import 'package:flutter/material.dart';

/// Sarlavhadagi dumaloq ikon tugmasi — bildirishnoma, hamyon va shu
/// kabilar.
///
/// ┌─ NEGA ALOHIDA FAYLDA ──────────────────────────────────────────────┐
/// Ilgari bu tugma `catalog_screen.dart` ichida yopiq (`_IconButton`)
/// edi va super app bosh sahifasi O'ZINIKINI qayta chizgan edi. Ikkita
/// nusxa ikkita haqiqat degani: bittasi doira, ikkinchisi kvadrat
/// bo'lib qolgandi, belgilari ham har xil edi — bitta ilovada bir xil
/// tugma ikki xil ko'rinardi.
///
/// Endi ikkala ekran ham SHU faylni chaqiradi. Ko'rinish bir joyda
/// o'zgaradi va ikkiga bo'linib ketmaydi.
/// └────────────────────────────────────────────────────────────────────┘
class HeaderActionButton extends StatelessWidget {
  final IconData icon;

  /// Ekran o'quvchisi uchun nom (`Semantics`).
  final String label;

  final VoidCallback onTap;

  /// O'qilmaganlar soni. 0 — belgi chizilmaydi.
  final int badge;

  const HeaderActionButton({
    super.key,
    required this.icon,
    required this.label,
    required this.onTap,
    this.badge = 0,
  });

  /// Belgida ko'rsatiladigan eng katta son — undan yuqorisi "9+".
  static const _maxBadge = 9;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: label,
      child: InkResponse(
        radius: 24,
        onTap: onTap,
        child: Stack(
          clipBehavior: Clip.none,
          children: [
            Container(
              width: 40,
              height: 40,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                border: Border.all(color: const Color(0xFFE5E5E5)),
                color: Colors.white,
              ),
              child: Icon(icon, size: 20, color: const Color(0xFF666666)),
            ),
            if (badge > 0)
              Positioned(
                right: -2,
                top: -2,
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 4),
                  constraints: const BoxConstraints(minWidth: 18),
                  height: 18,
                  alignment: Alignment.center,
                  decoration: BoxDecoration(
                    color: const Color(0xFFEF4444),
                    borderRadius: BorderRadius.circular(20),
                  ),
                  // Maketdagi qizil nuqta — lekin raqam bilan:
                  // "nechta?" degan savol nuqtadan javob olmaydi.
                  child: Text(
                    badge > _maxBadge ? '$_maxBadge+' : '$badge',
                    style: const TextStyle(
                      fontSize: 10,
                      fontWeight: FontWeight.bold,
                      color: Colors.white,
                      height: 1,
                    ),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

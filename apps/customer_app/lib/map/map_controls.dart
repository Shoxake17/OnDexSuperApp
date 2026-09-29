import 'package:flutter/material.dart';
import 'package:lucide_icons_flutter/lucide_icons.dart';

/// XARITA BOSHQARUVLARI — yagona ko'rinish.
///
/// ┌─ NEGA ALOHIDA FAYL ────────────────────────────────────────────────┐
/// Xarita ustidagi tugmalar uch ekranda uchraydi (xarita bo'limi,
/// manzil tanlash, kuryer kuzatuvi) va ular BIR XIL ko'rinishi kerak.
/// Har ekranda qayta yozilsa, ular asta-sekin bir-biridan uzoqlashardi.
///
/// Shakl — `image/geologo/map.png` dagi kabi dumaloq-kvadrat
/// (squircle), dumaloq emas: yon tomondagi tugmalar ustuni shu
/// shaklda tartibli ko'rinadi. Ilova yorug' mavzuda, shuning uchun
/// fon oq, belgilar to'q.
/// └────────────────────────────────────────────────────────────────────┘

/// Tugma o'lchami va burchak radiusi.
const double kMapButtonSize = 46;
const double kMapButtonRadius = 15;

/// Belgi rangi — deyarli qora (brend ranglari bilan raqobatlashmasin).
const Color kMapIconColor = Color(0xFF27272A);

/// Bitta kvadrat-dumaloq tugma.
class MapSquareButton extends StatelessWidget {
  const MapSquareButton({
    super.key,
    required this.icon,
    required this.onTap,
    this.tooltip,
    this.active = false,
  });

  final IconData icon;
  final VoidCallback? onTap;
  final String? tooltip;

  /// Yoqilgan holat (masalan sun'iy yo'ldosh) — brend rangida.
  final bool active;

  @override
  Widget build(BuildContext context) {
    final tugma = Material(
      color: active ? const Color(0xFF27272A) : Colors.white,
      borderRadius: BorderRadius.circular(kMapButtonRadius),
      elevation: 3,
      shadowColor: Colors.black26,
      child: InkWell(
        borderRadius: BorderRadius.circular(kMapButtonRadius),
        onTap: onTap,
        child: SizedBox(
          width: kMapButtonSize,
          height: kMapButtonSize,
          child: Icon(
            icon,
            size: 21,
            color: active ? Colors.white : kMapIconColor,
          ),
        ),
      ),
    );
    return tooltip == null ? tugma : Tooltip(message: tooltip!, child: tugma);
  }
}

/// Masshtab juftligi — bitta blokda «+» va «−».
///
/// `map.png` dagi kabi ikkita alohida tugma emas, bitta ustun:
/// ular doim birga ishlatiladi va yonma-yon turgani aniqroq.
class MapZoomButtons extends StatelessWidget {
  const MapZoomButtons({super.key, required this.onIn, required this.onOut});

  final VoidCallback onIn;
  final VoidCallback onOut;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.white,
      borderRadius: BorderRadius.circular(kMapButtonRadius),
      elevation: 3,
      shadowColor: Colors.black26,
      child: SizedBox(
        width: kMapButtonSize,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            _Half(
              icon: LucideIcons.plus,
              onTap: onIn,
              radius: const BorderRadius.vertical(
                  top: Radius.circular(kMapButtonRadius)),
            ),
            const Divider(height: 1, indent: 11, endIndent: 11),
            _Half(
              icon: LucideIcons.minus,
              onTap: onOut,
              radius: const BorderRadius.vertical(
                  bottom: Radius.circular(kMapButtonRadius)),
            ),
          ],
        ),
      ),
    );
  }
}

class _Half extends StatelessWidget {
  const _Half({required this.icon, required this.onTap, required this.radius});

  final IconData icon;
  final VoidCallback onTap;
  final BorderRadius radius;

  @override
  Widget build(BuildContext context) {
    return InkWell(
      borderRadius: radius,
      onTap: onTap,
      child: SizedBox(
        height: kMapButtonSize - 5,
        child: Icon(icon, size: 20, color: kMapIconColor),
      ),
    );
  }
}

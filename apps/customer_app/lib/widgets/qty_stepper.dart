import 'package:flutter/material.dart';

import 'common.dart';

/// Miqdor boshqaruvi "− n +" — BITTA nusxa.
///
/// ┌─ NEGA ALOHIDA FAYL ───────────────────────────────────────────────┐
/// Bu boshqaruv savatda ham, rasmiylashtirish ekranida ham kerak.
/// Avval ikkalasi ALOHIDA yozilgan edi (`_QtyBox`/`_RoundButton` va
/// `_QtyStepper`/`_StepIcon`) — bir xil ish qiladigan ikki kod, ikki
/// xil ko'rinishda. Ya'ni Flutter STEKI ICHIDA dublikat, loyiha
/// qoidasi buni taqiqlaydi.
///
/// Endi ikkala ekran SHU vidjetni ishlatadi: ko'rinish ham, xatti-
/// harakat ham bir xil va o'zgartirish BIR joyda qilinadi.
/// └───────────────────────────────────────────────────────────────────┘
class QtyStepper extends StatelessWidget {
  final int qty;
  final VoidCallback onAdd;
  final VoidCallback onRemove;

  /// Tugma o'lchami. Savatda ixcham (32), rasmiylashtirishda sal
  /// kattaroq bo'lishi mumkin — lekin SHAKL bir xil qoladi.
  final double size;

  const QtyStepper({
    super.key,
    required this.qty,
    required this.onAdd,
    required this.onRemove,
    this.size = 32,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        RoundIconButton(
            icon: Icons.remove, onTap: onRemove, size: size, elevation: 2),
        SizedBox(
          width: size - 4,
          child: Text(
            '$qty',
            textAlign: TextAlign.center,
            style: const TextStyle(fontWeight: FontWeight.bold),
          ),
        ),
        RoundIconButton(
            icon: Icons.add, onTap: onAdd, size: size, elevation: 2),
      ],
    );
  }
}

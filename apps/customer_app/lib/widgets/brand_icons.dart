import 'dart:math' as math;

import 'package:flutter/material.dart';

/// Ijtimoiy tarmoq brendlarining ORIGINAL belgilari.
///
/// Nega `CustomPainter`: bu belgilar Material ikonkalar to'plamida YO'Q
/// (`Icons.g_mobiledata` — Google logotipi emas, `Icons.send` — Telegram
/// emas). Avval ular o'rniga oddiy "G" harfi va qog'oz samolyot ikonkasi
/// ishlatilgan edi — bu brend talablariga ham, dizaynga ham mos emas.
/// PNG asset o'rniga vektor: har qanday o'lchamda aniq chiqadi va ilova
/// hajmini oshirmaydi.

/// Google'ning RASMIY logotipi — `assets/google_logo.png`.
///
/// Bu belgi ATAYLAB chizilmaydi: Google logotipi himoyalangan savdo
/// belgisi va uning brend qoidalari aniq fayl ishlatishni talab qiladi.
/// Avval u `CustomPainter` bilan taqlid qilingan edi — yaqin, lekin
/// rasman noto'g'ri. Endi rasmiy fayl ishlatiladi.
///
/// Fayl topilmasa ilova YIQILMAYDI — chizilgan zaxira variant
/// (`_GooglePainter`) ko'rsatiladi.
class GoogleIcon extends StatelessWidget {
  final double size;
  const GoogleIcon({super.key, this.size = 18});

  @override
  Widget build(BuildContext context) => SizedBox(
        width: size,
        height: size,
        child: Image.asset(
          'assets/google_logo.png',
          fit: BoxFit.contain,
          errorBuilder: (_, __, ___) => CustomPaint(painter: _GooglePainter()),
        ),
      );
}

class _GooglePainter extends CustomPainter {
  static const _blue = Color(0xFF4285F4);
  static const _green = Color(0xFF34A853);
  static const _yellow = Color(0xFFFBBC05);
  static const _red = Color(0xFFEA4335);

  @override
  void paint(Canvas canvas, Size size) {
    final w = size.width;
    final stroke = w * 0.21;
    final rect = Rect.fromLTWH(stroke / 2, stroke / 2, w - stroke, w - stroke);
    final p = Paint()
      ..style = PaintingStyle.stroke
      ..strokeWidth = stroke
      ..strokeCap = StrokeCap.butt;

    // Burchaklar: 0° = soat 3 (o'ng), musbat yo'nalish — soat strelkasi
    // bo'yicha (pastga).
    //
    // Yoylar bir-biriga TEGIB turadi (oralarida tirqish yo'q) — avvalgi
    // variantda ular orasida oq bo'shliqlar qolib, belgi "singan"
    // ko'rinardi. Yagona ochiq joy — o'ng tomonda, ko'k "til" chiqadigan
    // joy; harfni "G" qiladigan narsa aynan shu.
    double rad(double deg) => deg * math.pi / 180;
    canvas.drawArc(rect, rad(-80), rad(64), false, p..color = _blue); // o'ng-yuqori
    canvas.drawArc(rect, rad(-170), rad(90), false, p..color = _red); // yuqori
    canvas.drawArc(rect, rad(110), rad(80), false, p..color = _yellow); // chap
    canvas.drawArc(rect, rad(20), rad(90), false, p..color = _green); // past

    // Ko'k gorizontal "til": qalinligi halqa bilan BIR XIL, markazdan
    // o'ng chetgacha boradi va tirqishni to'ldiradi. Avval u juda uzun
    // edi va halqa ichiga kirib ketardi.
    canvas.drawRect(
      Rect.fromLTWH(w * 0.50, (w - stroke) / 2, w * 0.47, stroke),
      Paint()..color = _blue,
    );
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;
}

/// Telegram'ning ko'k doira ichidagi qog'oz samolyoti.
class TelegramIcon extends StatelessWidget {
  final double size;
  const TelegramIcon({super.key, this.size = 18});

  @override
  Widget build(BuildContext context) =>
      SizedBox(width: size, height: size, child: CustomPaint(painter: _TelegramPainter()));
}

class _TelegramPainter extends CustomPainter {
  @override
  void paint(Canvas canvas, Size size) {
    final w = size.width;
    // Brendning rasmiy ko'k gradienti.
    canvas.drawCircle(
      Offset(w / 2, w / 2),
      w / 2,
      Paint()
        ..shader = const LinearGradient(
          begin: Alignment.topCenter,
          end: Alignment.bottomCenter,
          colors: [Color(0xFF37BBFE), Color(0xFF007DBB)],
        ).createShader(Rect.fromLTWH(0, 0, w, w)),
    );

    final white = Paint()..color = Colors.white;
    // Samolyot tanasi.
    final plane = Path()
      ..moveTo(w * 0.22, w * 0.50)
      ..lineTo(w * 0.78, w * 0.27)
      ..lineTo(w * 0.66, w * 0.75)
      ..lineTo(w * 0.50, w * 0.60)
      ..lineTo(w * 0.38, w * 0.71)
      ..lineTo(w * 0.40, w * 0.56)
      ..close();
    canvas.drawPath(plane, white);
    // Qanot burmasi — logotipni tanib olinadigan qiladi.
    final fold = Path()
      ..moveTo(w * 0.40, w * 0.56)
      ..lineTo(w * 0.70, w * 0.35)
      ..lineTo(w * 0.50, w * 0.60)
      ..close();
    canvas.drawPath(fold, Paint()..color = const Color(0xFFD6EEF9));
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;
}

/// O'zbekiston bayrog'i — RASMIY rasm (`assets/uz_flag.png`).
///
/// Avval u uchta rangli chiziq bilan chizilgan edi — bayroqdagi oy va
/// yulduzlar yo'q edi, ya'ni bu bayroq emas, taxmin edi. Endi haqiqiy
/// rasm ishlatiladi; fayl topilmasa ilova yiqilmasligi uchun oddiy
/// uch chiziqli zaxira qoladi.
class UzFlag extends StatelessWidget {
  final double width;
  const UzFlag({super.key, this.width = 22});

  @override
  Widget build(BuildContext context) {
    return ClipRRect(
      borderRadius: BorderRadius.circular(2),
      child: SizedBox(
        width: width,
        height: width * 0.55, // 270x148 rasmning tabiiy nisbati
        child: Image.asset(
          'assets/uz_flag.png',
          fit: BoxFit.cover,
          errorBuilder: (_, __, ___) => const Column(
            children: [
              Expanded(child: ColoredBox(color: Color(0xFF0099B5), child: SizedBox.expand())),
              Expanded(child: ColoredBox(color: Colors.white, child: SizedBox.expand())),
              Expanded(child: ColoredBox(color: Color(0xFF1EB53A), child: SizedBox.expand())),
            ],
          ),
        ),
      ),
    );
  }
}

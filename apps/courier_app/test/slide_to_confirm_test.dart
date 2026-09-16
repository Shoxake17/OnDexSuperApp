import 'package:chust_courier/widgets/slide_to_confirm.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

/// Kuryer ekranidagi pastki tugma kabi: bosqich o'tgach AYNAN o'sha joyga
/// yana `SlideToConfirm` qo'yiladi, faqat yozuvi boshqa. Kalit ATAYLAB
/// berilmagan — Flutter holat obyektini qayta ishlatadi va widget buni
/// o'zi to'g'ri hal qilishi kerak.
class _Steps extends StatefulWidget {
  const _Steps({required this.results});

  /// Har bosqichda `onConfirm` qaytaradigan natija (server javobi).
  final List<bool> results;

  @override
  State<_Steps> createState() => _StepsState();
}

class _StepsState extends State<_Steps> {
  static const labels = ['Yetib keldim', 'Buyurtma olindi', 'Mijozga yetkazdim', 'Tugadi'];
  int step = 0;
  int calls = 0;

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      home: Scaffold(
        body: Center(
          child: SizedBox(
            width: 360,
            child: SlideToConfirm(
              label: labels[step],
              busy: false,
              onConfirm: () async {
                final ok = widget.results[calls++];
                if (ok) setState(() => step++);
                return ok;
              },
            ),
          ),
        ),
      ),
    );
  }
}

Future<void> _slide(WidgetTester tester) async {
  await tester.drag(find.byType(GestureDetector).last, const Offset(400, 0));
  // `pumpAndSettle` EMAS: aylanuvchi indikator cheksiz animatsiya, u
  // bo'lsa `pumpAndSettle` muddat tugab yiqilardi va sabab xiralashardi.
  await tester.pump(const Duration(milliseconds: 500));
}

void main() {
  // BUG (2026-09-15, lokal sinovda): "Yetib keldim" -> "Buyurtma olindi" ->
  // "Mijozga yetkazdim" da tugma abadiy aylanib qolardi, ilovani qayta
  // ochgandagina keyingi bosqich ishlardi.
  testWidgets('muvaffaqiyatli bosqichdan keyin keyingi tugma aylanib QOLMAYDI', (tester) async {
    await tester.pumpWidget(const _Steps(results: [true, true, true]));

    for (final next in ['Buyurtma olindi', 'Mijozga yetkazdim', 'Tugadi']) {
      await _slide(tester);
      expect(find.text('$next   »'), findsOneWidget, reason: 'keyingi bosqich: $next');
      expect(find.byType(CircularProgressIndicator), findsNothing,
          reason: '"$next" tugmasi aylanib qoldi (eski holat qayta ishlatildi)');
      expect(find.byIcon(Icons.arrow_forward), findsOneWidget, reason: '"$next" surib bo\'lmaydi');
    }
  });

  testWidgets('server rad etsa tutqich boshiga qaytadi va qayta urinish mumkin', (tester) async {
    await tester.pumpWidget(const _Steps(results: [false, true]));

    await _slide(tester);
    expect(find.text('Yetib keldim   »'), findsOneWidget);
    expect(find.byType(CircularProgressIndicator), findsNothing);

    await _slide(tester);
    expect(find.text('Buyurtma olindi   »'), findsOneWidget);
    expect(find.byType(CircularProgressIndicator), findsNothing);
  });
}

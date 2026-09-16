import 'package:chust_courier/location_reporter.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  final t0 = DateTime(2026, 9, 15, 21, 30);
  DateTime at(int s) => t0.add(Duration(seconds: s));

  test('aniq nuqta har doim ishlatiladi', () {
    expect(LocationReporter.acceptFix(accuracy: 12, now: at(0), lastGoodAt: at(0)), isTrue);
    // 0 — platforma aniqlikni bermadi, bloklanmaydi.
    expect(LocationReporter.acceptFix(accuracy: 0, now: at(0), lastGoodAt: at(0)), isTrue);
  });

  test('aniqligi past nuqta — faqat aniq nuqta uzoq kelmasa', () {
    expect(LocationReporter.acceptFix(accuracy: 1500, now: at(30), lastGoodAt: at(0)), isFalse);
    expect(LocationReporter.acceptFix(accuracy: 1500, now: at(119), lastGoodAt: at(0)), isFalse);
    expect(LocationReporter.acceptFix(accuracy: 1500, now: at(120), lastGoodAt: at(0)), isTrue);
    // Hali birorta aniq nuqta bo'lmagan — kuryer joylashuvsiz qolmasin.
    expect(LocationReporter.acceptFix(accuracy: 1500, now: at(0)), isTrue);
  });
}

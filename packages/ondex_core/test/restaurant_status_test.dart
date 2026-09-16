import 'package:flutter_test/flutter_test.dart';
import 'package:ondex_core/ondex_core.dart';

void main() {
  // Toshkent vaqti = UTC + 5. 2026-09-14 — dushanba.
  DateTime tk(int day, int hour, [int minute = 0]) =>
      DateTime.utc(2026, 9, day, hour - 5, minute);

  test('server holati o\'qiladi; open_now bo\'lmasa — qo\'lda tugma', () {
    final closed = RestaurantOpenStatus.fromJson({
      'open': true,
      'open_now': false,
      'closed_reason': 'hours',
      'open_changes_at': '2026-09-15T04:00:00Z',
    });
    expect(closed.open, isFalse);
    expect(closed.reason, RestaurantClosedReason.hours);
    expect(closed.changesAt, tk(15, 9));

    final legacy = RestaurantOpenStatus.fromJson({'open': true});
    expect(legacy.open, isTrue);
    expect(RestaurantOpenStatus.fromJson({'open': false}).reason, RestaurantClosedReason.manual);
  });

  test('izoh: bugun, ertaga, hafta kuni, yopilishga oz qoldi', () {
    final opensTomorrow = RestaurantOpenStatus(
        open: false, reason: RestaurantClosedReason.hours, changesAt: tk(15, 9));
    expect(opensTomorrow.detail(tk(14, 23)), 'Ertaga 09:00 da ochiladi');
    expect(opensTomorrow.detail(tk(15, 1)), '09:00 da ochiladi');
    expect(opensTomorrow.closedMessage(tk(14, 23)), 'Restoran hozir yopiq · Ertaga 09:00 da ochiladi');

    final opensMonday = RestaurantOpenStatus(
        open: false, reason: RestaurantClosedReason.hours, changesAt: tk(21, 18));
    expect(opensMonday.detail(tk(19, 12)), 'Dushanba 18:00 da ochiladi');

    final open = RestaurantOpenStatus(open: true, changesAt: tk(14, 22));
    expect(open.detail(tk(14, 20)), isNull);
    expect(open.detail(tk(14, 21, 30)), '22:00 da yopiladi');
  });

  test('vaqt o\'tsa holat almashadi; qo\'lda yopilgan ochilmaydi', () {
    final open = RestaurantOpenStatus(open: true, changesAt: tk(14, 22));
    expect(open.at(tk(14, 21, 59)).open, isTrue);
    final later = open.at(tk(14, 22));
    expect(later.open, isFalse);
    expect(later.reason, RestaurantClosedReason.hours);

    const manual = RestaurantOpenStatus(open: false, reason: RestaurantClosedReason.manual);
    expect(manual.at(tk(20, 12)).open, isFalse);
    expect(manual.closedMessage(tk(14, 12)), 'Restoran hozir buyurtma qabul qilmayapti');
  });
}

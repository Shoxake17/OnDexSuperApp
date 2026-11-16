import 'package:chust_customer/reservations/my_reservations_screen.dart';
import 'package:chust_customer/reservations/reservation_screen.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('zaklad matni mijozga tushunarli', () {
    Map<String, dynamic> r(String s) => {'deposit_tiyin': 20000000, 'deposit_state': s};
    expect(customerDepositText({'deposit_tiyin': 0}), '');
    expect(customerDepositText(r('paid')), 'Zaklad 200 000 so\'m to\'langan');
    expect(customerDepositText(r('refunded')), contains('qaytarildi'));
    expect(customerDepositText(r('kept')), contains('qaytarilmaydi'));
    expect(customerDepositText(r('applied')), contains('hisobingizdan ayriladi'));
  });

  test('holatlar o\'zbekcha', () {
    for (final s in ['held', 'confirmed', 'seated', 'completed', 'cancelled', 'no_show', 'expired']) {
      expect(customerReservationStatus(s).$1, isNot(s));
    }
  });

  test('Toshkent vaqti va HH:mm', () {
    expect(hhmm(toTashkent(DateTime.utc(2026, 11, 16, 14, 30))), '19:30');
  });
}

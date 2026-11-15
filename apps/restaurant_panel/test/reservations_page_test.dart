import 'package:chust_restaurant/pages/reservations_page.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('zaklad matni holatga mos', () {
    Map<String, dynamic> r(String state, {String method = 'card'}) =>
        {'deposit_tiyin': 10000000, 'deposit_state': state, 'deposit_method': method};
    expect(depositText({'deposit_tiyin': 0}), 'Zakladsiz');
    expect(depositText(r('pending')), contains('to\'lanmagan'));
    expect(depositText(r('paid', method: 'wallet')), 'Zaklad 100 000 so\'m to\'langan (Wallet)');
    expect(depositText(r('applied')), contains('hisobdan ayiring'));
    expect(depositText(r('refunded')), contains('qaytarildi'));
    expect(depositText(r('kept')), contains('restoranda qoldi'));
  });

  test('har holatning o\'zbekcha nomi bor', () {
    for (final s in ['held', 'confirmed', 'seated', 'completed', 'cancelled', 'no_show', 'expired']) {
      final (label, _, _) = reservationStatusStyle(s);
      expect(label, isNot(s), reason: s);
    }
  });

  test('Toshkent vaqti UTC+5', () {
    expect(tashkent(DateTime.utc(2026, 11, 16, 14)).hour, 19);
  });
}

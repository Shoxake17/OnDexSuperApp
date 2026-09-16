import 'package:chust_courier/order_view.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('paymentInstructionFor', () {
    test('naqd — summani olish kerak', () {
      final p = paymentInstructionFor({'payment_method': 'cash', 'total_tiyin': 4500000});
      expect(p.kind, PaymentKind.collectCash);
      expect(p.title, contains('Naqd'));
    });

    test('karta ushlab qolingan yoki yechilgan — pul olinmaydi', () {
      for (final state in ['held', 'paid']) {
        final p = paymentInstructionFor({'payment_method': 'card', 'payment_state': state});
        expect(p.kind, PaymentKind.paidOnline, reason: state);
        expect(p.subtitle, contains('OLMANG'));
      }
    });

    // Xato "pul oling" ko'rsatmasi mijozdan ikki marta olish demak.
    test('karta tasdiqlanmagan yoki tur noma\'lum — "pul oling" DEYILMAYDI', () {
      for (final order in [
        {'payment_method': 'card', 'payment_state': 'awaiting'},
        {'payment_method': 'card', 'payment_state': 'failed'},
        {'payment_method': 'card'},
        <String, dynamic>{},
        {'payment_method': 'crypto'},
      ]) {
        final p = paymentInstructionFor(order);
        expect(p.kind, PaymentKind.unknown, reason: '$order');
        expect(p.title.toLowerCase(), isNot(contains('oling')), reason: '$order');
      }
    });
  });

  group('cashSummaryFor', () {
    test('buyurtma yo\'q', () {
      final c = cashSummaryFor(null);
      expect(c.kind, isNull);
      expect(c.value, 'Buyurtma yo\'q');
    });

    test('naqd — buyurtma narxi ko\'rinadi', () {
      final c = cashSummaryFor({'payment_method': 'cash', 'total_tiyin': 4500000});
      expect(c.kind, PaymentKind.collectCash);
      expect(c.label, 'Naqd olinadi');
      expect(c.value, contains('45'));
    });

    test('karta yoki noma\'lum — summa YOZILMAYDI', () {
      for (final order in [
        {'payment_method': 'card', 'payment_state': 'paid', 'total_tiyin': 4500000},
        {'payment_method': 'card', 'payment_state': 'awaiting', 'total_tiyin': 4500000},
        {'total_tiyin': 4500000},
      ]) {
        final c = cashSummaryFor(order);
        expect(c.kind, isNot(PaymentKind.collectCash), reason: '$order');
        expect(c.value, isNot(contains('45')), reason: '$order');
      }
    });
  });
}

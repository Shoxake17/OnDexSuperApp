import 'package:chust_customer/tablebill/bill_screen.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('o\'z taomlari summasi server qoidasi bilan bir xil', () {
    final bill = {
      'orders': [
        {
          'id': 'o1',
          'lines': [
            {'line': 0, 'qty': 3, 'paid_qty': 1, 'unit_tiyin': 333333, 'amount_tiyin': 1000000},
            {'line': 1, 'qty': 1, 'paid_qty': 0, 'unit_tiyin': 500000, 'amount_tiyin': 500000},
          ],
        },
      ],
    };
    // 1 ta (to'liq emas) — unit * qty.
    expect(itemsAmount(bill, {lineKey('o1', 0): 1}), 333333);
    // Qolgan 2 tasi (oxirgi qism) — amount - unit*paid (yaxlitlash qoldig'i bilan).
    expect(itemsAmount(bill, {lineKey('o1', 0): 2}), 1000000 - 333333);
    expect(itemsAmount(bill, {lineKey('o1', 0): 2, lineKey('o1', 1): 1}), 1000000 - 333333 + 500000);
    expect(itemsAmount(bill, {}), 0);
  });
}

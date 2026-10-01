import 'dart:convert';

import 'package:chust_customer/api.dart';
import 'package:chust_customer/data/payment_options.dart';
import 'package:chust_customer/data/quote_service.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

/// YETKAZISH HAQI FAQAT RASMIYLASHTIRISHDA.
///
/// Server `total_tiyin` ga yetkazish haqini ALLAQACHON qo'shadi. Menyu va
/// savatda mijoz faqat TAOMLAR summasini ko'rishi kerak; yetkazish haqi va
/// to'lanadigan jami — rasmiylashtirishda.
void main() {
  group('parseQuote: taomlar / yetkazish / to\'lanadigan jami', () {
    test('yetkazish haqi taomlar summasiga QO\'SHILMAYDI', () {
      // Taomlar 35 000, yetkazish 8 000 -> server jami 43 000.
      final q = parseQuote({
        'subtotal_tiyin': 3500000,
        'discount_tiyin': 0,
        'delivery_fee_tiyin': 800000,
        'total_tiyin': 4300000,
      });
      expect(q.goodsTiyin, 3500000, reason: 'menyu/savat summasi');
      expect(q.deliveryFeeTiyin, 800000);
      expect(q.payableTiyin, 4300000, reason: 'to\'lanadigan jami');
      expect(q.isUsable, isTrue);
    });

    test('chegirma bilan: taomlar = jami - yetkazish', () {
      final q = parseQuote({
        'subtotal_tiyin': 4000000,
        'discount_tiyin': 500000,
        'delivery_fee_tiyin': 800000,
        'total_tiyin': 4300000, // 40 000 - 5 000 + 8 000
      });
      expect(q.goodsTiyin, 3500000);
    });

    test('yetkazish bepul (oylik kuryer) — jami = taomlar', () {
      final q = parseQuote({
        'subtotal_tiyin': 3500000,
        'delivery_fee_tiyin': 0,
        'total_tiyin': 3500000,
      });
      expect(q.goodsTiyin, 3500000);
      expect(q.deliveryFeeTiyin, 0);
      expect(q.payableTiyin, 3500000);
    });

    test('eski server (delivery_fee_tiyin yo\'q) — haq yo\'q deb olinadi', () {
      final q = parseQuote({'subtotal_tiyin': 1800000, 'total_tiyin': 1800000});
      expect(q.goodsTiyin, 1800000);
      expect(q.deliveryFeeTiyin, 0);
      expect(q.payableTiyin, 1800000);
    });

    test('manfiy haq e\'tiborga olinmaydi', () {
      final q = parseQuote({'delivery_fee_tiyin': -5, 'total_tiyin': 1000000});
      expect(q.deliveryFeeTiyin, 0);
      expect(q.goodsTiyin, 1000000);
    });

    test('ishonchsiz jami (0 yoki yo\'q) — foydalanib bo\'lmaydi', () {
      for (final res in [
        {'total_tiyin': 0},
        {'total_tiyin': -100},
        <String, dynamic>{},
      ]) {
        final q = parseQuote(res);
        expect(q.isUsable, isFalse, reason: '$res');
        expect(q.goodsTiyin, isNull);
        expect(q.payableTiyin, isNull);
      }
    });

    test('faqat yetkazish haqidan iborat "jami" taom summasi bo\'lib qolmaydi', () {
      // total == fee -> taomlar 0: bunday javob ishonchsiz.
      final q = parseQuote({'delivery_fee_tiyin': 800000, 'total_tiyin': 800000});
      expect(q.goodsTiyin, isNull);
      expect(q.isUsable, isFalse);
    });
  });

  group('api.quote: stol buyurtmasi yetkazish haqisiz', () {
    Future<Map<String, dynamic>> sent(bool? dineIn) async {
      late Map<String, dynamic> body;
      await http.runWithClient(() async {
        if (dineIn == null) {
          await api.quote('rest-a', [
            {'product_id': 'p1', 'qty': 1}
          ]);
        } else {
          await api.quote('rest-a', [
            {'product_id': 'p1', 'qty': 1}
          ], dineIn: dineIn);
        }
      }, () => MockClient((req) async {
            body = jsonDecode(req.body) as Map<String, dynamic>;
            return http.Response('{"total_tiyin":1000}', 200,
                headers: {'content-type': 'application/json'});
          }));
      return body;
    }

    test('stol buyurtmasida dine_in:true yuboriladi', () async {
      expect((await sent(true))['dine_in'], isTrue);
    });

    test('yetkazishda dine_in yuborilmaydi (server yetkazish deb oladi)', () async {
      expect((await sent(false)).containsKey('dine_in'), isFalse);
      expect((await sent(null)).containsKey('dine_in'), isFalse);
    });
  });

  group('paymentOptionVisible: faqat restoran yoqqan usullar', () {
    const options = {
      'cash': true,
      'card': false,
      'payme': true,
      'click': false,
      'uzum': true,
    };

    test('o\'chirilgan usul ko\'rinmaydi, yoqilgani ko\'rinadi', () {
      expect(paymentOptionVisible(options, 'uzum'), isTrue);
      expect(paymentOptionVisible(options, 'payme'), isTrue);
      expect(paymentOptionVisible(options, 'cash'), isTrue);
      expect(paymentOptionVisible(options, 'click'), isFalse);
      expect(paymentOptionVisible(options, 'card'), isFalse);
    });

    test('Uzum o\'chirilsa faqat Uzum yo\'qoladi', () {
      final off = {...options, 'uzum': false};
      expect(paymentOptionVisible(off, 'uzum'), isFalse);
      expect(paymentOptionVisible(off, 'payme'), isTrue);
    });

    test('payment_options yo\'q (yuklanmagan/eski server) — usul ochiq', () {
      expect(paymentOptionVisible(null, 'uzum'), isTrue);
      expect(paymentOptionVisible('xato', 'uzum'), isTrue);
      expect(paymentOptionVisible(<String, dynamic>{}, 'uzum'), isTrue);
    });

    test('noto\'g\'ri turdagi qiymat usulni yashirmaydi', () {
      expect(paymentOptionVisible({'uzum': 'false'}, 'uzum'), isTrue);
    });
  });
}

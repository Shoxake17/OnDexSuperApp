import 'package:chust_courier/offer_alert.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('offerNotificationId', () {
    test('barqaror va musbat (fon va asosiy izolyatda bir xil)', () {
      expect(offerNotificationId('e83d6a41d81f5931'), offerNotificationId('e83d6a41d81f5931'));
      expect(offerNotificationId('e83d6a41d81f5931'), isNonNegative);
      expect(offerNotificationId('e83d6a41d81f5931') <= 0x7fffffff, isTrue);
      expect(offerNotificationId('o1'), isNot(offerNotificationId('o2')));
    });
  });

  group('offerAlertFromData', () {
    final now = DateTime(2026, 9, 15, 12);
    Map<String, dynamic> data({String kind = 'courier_offer', Object? expiresIn = 20}) => {
          'kind': kind,
          'order_id': 'o1',
          'title': 'Yangi buyurtma',
          'body': 'Book Cafe — qabul qilish uchun ilovani oching',
          'expires_at': expiresIn == null
              ? null
              : '${now.add(Duration(seconds: expiresIn as int)).millisecondsSinceEpoch}',
        };

    test('to\'g\'ri taklif — qolgan vaqtgacha chalinadi', () {
      final a = offerAlertFromData(data(), now)!;
      expect(a.orderId, 'o1');
      expect(a.timeout, const Duration(seconds: 20));
      expect(a.body, contains('Book Cafe'));
    });

    test('muddati o\'tgan yoki deyarli tugagan — signal yo\'q', () {
      expect(offerAlertFromData(data(expiresIn: -5), now), isNull);
      expect(offerAlertFromData(data(expiresIn: 1), now), isNull);
    });

    test('noto\'g\'ri xabar — signal yo\'q', () {
      expect(offerAlertFromData(data(kind: 'order_status'), now), isNull);
      expect(offerAlertFromData(data(expiresIn: null), now), isNull);
      expect(offerAlertFromData({...data(), 'order_id': ''}, now), isNull);
    });

    test('server juda uzun muddat yuborsa ham cheklanadi', () {
      expect(offerAlertFromData(data(expiresIn: 3600), now)!.timeout, kMaxOfferAlert);
    });

    test('matn bo\'lmasa standart matn', () {
      final a = offerAlertFromData({...data(), 'title': '', 'body': null}, now)!;
      expect(a.title, 'Yangi buyurtma');
      expect(a.body, isNotEmpty);
    });
  });
}

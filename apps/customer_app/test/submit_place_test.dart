import 'dart:convert';
import 'dart:typed_data';

import 'package:chust_customer/map/ondexmap_api.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:flutter_test/flutter_test.dart';

/// OB'EKT YUBORISH TESTLARI.
///
/// Diqqat markazida — XAVFSIZLIK va server kontrakti: honeypot
/// maydoni, chegaralar va foydalanuvchi ma'lumoti sizib chiqmasligi.
void main() {
  /// Yuborilgan so'rovning XOM tanasini ushlaydi.
  ///
  /// `MockClient.streaming` ishlatiladi, oddiy `MockClient` emas:
  /// u so'rovni `BaseRequest` sifatida beradi va `MultipartRequest`
  /// maydonlariga (`fields`, `files`) kirib bo'lmaydi. Xom tana esa
  /// hamma narsani ko'rsatadi — aynan tarmoqqa nima chiqayotganini.
  ({http.Client client, List<String> bodies}) xomTutuvchi(
      {int status = 201, String body = '{"status":"pending"}'}) {
    final bodies = <String>[];
    final c = MockClient.streaming((req, bodyStream) async {
      bodies.add(await utf8.decodeStream(bodyStream));
      return http.StreamedResponse(
          Stream.value(utf8.encode(body)), status);
    });
    return (client: c, bodies: bodies);
  }

  group('honeypot', () {
    test('`website` maydoni HAR DOIM bo\'sh yuboriladi', () async {
      // ⚠️ Bu maydon — bot tuzog'i. To'ldirilgan bo'lsa server
      // yuborishni jimgina tashlab yuboradi, ya'ni haqiqiy
      // foydalanuvchining ob'ekti yo'qolardi.
      final t = xomTutuvchi();
      await OndexMapApi(client: t.client).submitPlace(
        kind: 'organization',
        lat: 41.0,
        lng: 71.2,
        name: 'Sinov',
        site: 'example.uz',
      );
      expect(t.bodies, hasLength(1));
      expect(t.bodies.single, contains('"website":""'));
      // Sayt manzili `site` ga tushadi, `website` ga EMAS.
      expect(t.bodies.single, contains('"site":"example.uz"'));
    });
  });

  group('maxfiylik', () {
    test('foydalanuvchi haqida hech narsa yuborilmaydi', () async {
      final t = xomTutuvchi();
      await OndexMapApi(client: t.client).submitPlace(
        kind: 'organization',
        lat: 41.0,
        lng: 71.2,
        name: 'Sinov',
        phone: '+998901234567',
      );
      final b = t.bodies.single;
      // Taklif ANONIM ketadi: akkaunt, token, qurilma belgisi yo'q.
      for (final taqiq in ['token', 'user_id', 'device', 'session', 'auth']) {
        expect(b.toLowerCase(), isNot(contains('"$taqiq"')),
            reason: '«$taqiq» so\'rovga tushib qolgan');
      }
      // Telefon — OB'EKTNIKI, u yuboriladi.
      expect(b, contains('+998901234567'));
    });
  });

  group('chegaralar mijozda ham tekshiriladi', () {
    Uint8List katta(int n) => Uint8List(n);

    test('4 tadan ortiq rasm — tarmoqqa CHIQMAYDI', () async {
      var chaqirildi = false;
      final c = MockClient.streaming((_, __) async {
        chaqirildi = true;
        return http.StreamedResponse(const Stream.empty(), 201);
      });
      await expectLater(
        OndexMapApi(client: c).submitPlace(
          kind: 'organization',
          lat: 41.0,
          lng: 71.2,
          name: 'Sinov',
          photos: [
            for (var i = 0; i < kOndexMaxPhotos + 1; i++)
              (name: 'r$i.jpg', bytes: katta(10)),
          ],
        ),
        throwsA(isA<OndexMapException>()),
      );
      expect(chaqirildi, isFalse, reason: 'ortiqcha rasm yuborilmasligi kerak');
    });

    test('6 MB dan katta rasm — tarmoqqa CHIQMAYDI', () async {
      var chaqirildi = false;
      final c = MockClient.streaming((_, __) async {
        chaqirildi = true;
        return http.StreamedResponse(const Stream.empty(), 201);
      });
      await expectLater(
        OndexMapApi(client: c).submitPlace(
          kind: 'organization',
          lat: 41.0,
          lng: 71.2,
          name: 'Sinov',
          photos: [(name: 'katta.jpg', bytes: katta(kOndexMaxPhotoBytes + 1))],
        ),
        throwsA(isA<OndexMapException>()),
      );
      expect(chaqirildi, isFalse);
    });
  });

  group('server xatolari', () {
    test('429 — tushunarli xabar', () async {
      final c = MockClient.streaming((_, __) async => http.StreamedResponse(
          Stream.value(utf8.encode('{"error":"soatiga 5 tadan ko\'p"}')), 429));
      await expectLater(
        OndexMapApi(client: c).submitPlace(
            kind: 'organization', lat: 41.0, lng: 71.2, name: 'Sinov'),
        throwsA(isA<OndexMapException>().having(
            (e) => e.message, 'xabar', contains('ko\'p ob\'ekt'))),
      );
    });

    test('400 — server sababi ko\'rsatiladi', () async {
      // Foydalanuvchi tuzata oladigan xato: sababni yashirmaymiz.
      final c = MockClient.streaming((_, __) async => http.StreamedResponse(
          Stream.value(utf8.encode('{"error":"nom juda qisqa"}')), 400));
      await expectLater(
        OndexMapApi(client: c).submitPlace(
            kind: 'organization', lat: 41.0, lng: 71.2, name: 'A'),
        throwsA(isA<OndexMapException>()
            .having((e) => e.message, 'xabar', 'nom juda qisqa')),
      );
    });
  });

  group('turlar ro\'yxati', () {
    test('faqat NUQTA turlari — chiziq turlari yo\'q', () {
      // `road`, `crossing`, `fence` xaritada chizishni talab qiladi;
      // ular ilovada hali qo'llanmaydi va ro'yxatga tushmasligi kerak,
      // aks holda server `400` qaytarardi.
      final kalitlar = kOndexPlaceKinds.map((k) => k.key).toSet();
      for (final chiziq in ['road', 'crossing', 'fence']) {
        expect(kalitlar, isNot(contains(chiziq)));
      }
      expect(kalitlar, contains('organization'));
      expect(kalitlar, contains('other'));
    });
  });
}

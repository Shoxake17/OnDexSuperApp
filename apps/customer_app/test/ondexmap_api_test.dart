import 'dart:convert';

import 'package:chust_customer/map/map_backend.dart';
import 'package:chust_customer/map/ondexmap_api.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:flutter_test/flutter_test.dart';

/// ONDEXMAP API KLIENTI TESTLARI.
///
/// Javoblar soxta `http.Client` bilan beriladi: tarmoqqa chiqmaymiz,
/// lekin JAVOB SHAKLI haqiqiy serverdan olingan (2026-09-28 da
/// `maps.ondex.uz` dan o'lchangan) — ya'ni tahlil qiladigan kod
/// haqiqiy ma'lumotga qarshi sinaladi.
void main() {
  http.Client soxta(String body, {int status = 200, void Function(Uri)? kuzat}) {
    return MockClient((req) async {
      kuzat?.call(req.url);
      return http.Response(body, status,
          headers: {'content-type': 'application/json; charset=utf-8'});
    });
  }

  group('search', () {
    const javob = '''
{"results":[
 {"id":"g23272","type":"town","name":"Chust","label":"Shaharcha","lat":41.004,"lng":71.234,"score":122},
 {"id":"g23298","type":"district","name":"Chust Tumani","label":"Tuman","lat":40.999,"lng":71.241,"score":108}
]}''';

    test('natijalarni o\'qiydi', () async {
      final api = OndexMapApi(client: soxta(javob));
      final r = await api.search('Chust');
      expect(r, hasLength(2));
      expect(r.first.name, 'Chust');
      expect(r.first.label, 'Shaharcha');
      expect(r.first.lat, closeTo(41.004, 1e-6));
    });

    test('qisqa so\'rovda tarmoqqa CHIQMAYDI', () async {
      var chaqirildi = false;
      final api = OndexMapApi(
        client: soxta(javob, kuzat: (_) => chaqirildi = true),
      );
      expect(await api.search('C'), isEmpty);
      expect(chaqirildi, isFalse, reason: 'bir harf uchun so\'rov yuborilmaydi');
    });

    test('yaqinlik nuqtasi so\'rovga qo\'shiladi', () async {
      Uri? url;
      final api = OndexMapApi(client: soxta(javob, kuzat: (u) => url = u));
      await api.search('Chust', near: (lat: 41.0, lng: 71.2));
      expect(url!.queryParameters['lat'], startsWith('41.0'));
      expect(url!.queryParameters['lng'], startsWith('71.2'));
    });

    test('buzuq yozuv butun ro\'yxatni yiqitmaydi', () async {
      // Koordinatasiz yozuv tashlab yuboriladi, qolgani qaytadi.
      final api = OndexMapApi(client: soxta(jsonEncode({
        'results': [
          {'id': 'x', 'name': 'Koordinatasiz'},
          {'id': 'g1', 'name': 'Yaxshi', 'label': 'Ko\'cha', 'lat': 41.0, 'lng': 71.2},
        ]
      })));
      final r = await api.search('test');
      expect(r, hasLength(1));
      expect(r.single.name, 'Yaxshi');
    });

    test('server xatosi tushunarli xabar beradi', () async {
      final api = OndexMapApi(client: soxta('{}', status: 500));
      expect(() => api.search('Chust'), throwsA(isA<OndexMapException>()));
    });
  });

  group('route', () {
    const javob = '''
{"distance_m":1673.2,"duration_s":249.7,
 "geometry":{"coordinates":[[71.236039,41.00286],[71.236426,41.002922],[71.237203,41.003189]]}}''';

    test('geometriyani [lat,lng] ga o\'giradi', () async {
      // ⚠️ Server GeoJSON tartibida beradi: [lng, lat]. Almashtirib
      // yuborilsa yo'l butunlay boshqa joyda chizilardi.
      final api = OndexMapApi(client: soxta(javob));
      final r = await api.route(
          fromLat: 41.003, fromLng: 71.236, toLat: 41.004, toLng: 71.237);
      expect(r.points, hasLength(3));
      expect(r.points.first.lat, closeTo(41.00286, 1e-6));
      expect(r.points.first.lng, closeTo(71.236039, 1e-6));
      expect(r.distanceMeters, closeTo(1673.2, 0.1));
    });

    test('so\'rov parametrlari server kutgan nomlarda', () async {
      Uri? url;
      final api = OndexMapApi(client: soxta(javob, kuzat: (u) => url = u));
      await api.route(
          fromLat: 41.003, fromLng: 71.236, toLat: 41.01, toLng: 71.25);
      expect(url!.queryParameters.keys,
          containsAll(['from_lat', 'from_lng', 'to_lat', 'to_lng']));
    });

    test('yo\'l topilmasa aniq xato', () async {
      final api = OndexMapApi(client: soxta(
          '{"distance_m":0,"duration_s":0,"geometry":{"coordinates":[]}}'));
      expect(
        () => api.route(fromLat: 41.0, fromLng: 71.2, toLat: 41.0, toLng: 71.2),
        throwsA(isA<OndexMapException>()),
      );
    });
  });

  group('marshrut qamrovi — BUTUN O\'ZBEKISTON', () {
    // ┌─ TESKARI GEOKODLASHDAN FARQ QILADI ──────────────────────────┐
    // Marshrut o'z OSRM grafigimizdan keladi (`uzbekistan.osm.pbf`),
    // teskari geokodlash esa mahalla/ko'cha ma'lumotidan. Shuning
    // uchun qamrovlari boshqa-boshqa va ular alohida tekshiriladi.
    // └──────────────────────────────────────────────────────────────┘
    test('mamlakatning to\'rt chekkasi ham ichkarida', () {
      for (final (nom, lat, lng) in const [
        ('Chust', 41.0030, 71.2360),
        ('Toshkent', 41.2995, 69.2401),
        ('Buxoro', 39.7700, 64.4200),
        ('Nukus', 42.4600, 59.6100),
        ('Termiz', 37.2200, 67.2800),
      ]) {
        expect(ondexRouteCovers(lat, lng), isTrue, reason: nom);
      }
    });

    test('mamlakat tashqarisi — qamrovdan chetda', () {
      for (final (nom, lat, lng) in const [
        ('Qozogʻiston, Aqtoʻbe', 47.2, 60.0),
        ('Afgʻoniston ichkarisi', 34.5, 67.1),
        ('Kaspiy dengizi', 41.0, 51.0),
      ]) {
        expect(ondexRouteCovers(lat, lng), isFalse, reason: nom);
      }
    });

    test('teskari geokodlash hamon TORROQ', () {
      // Ikkalasi bir xil bo'lib qolsa, ulardan biri jimgina
      // noto'g'ri ishlardi.
      expect(ondexRouteCovers(41.2995, 69.2401), isTrue, reason: 'marshrut');
      expect(ondexReverseCovers(41.2995, 69.2401), isFalse, reason: 'reverse');
    });
  });

  group('matn formatlari', () {
    test('masofa', () {
      expect(ondexDistanceText(340), '340 m');
      expect(ondexDistanceText(1673.2), '1,7 km');
      expect(ondexDistanceText(24500), '25 km');
    });

    test('vaqt', () {
      expect(ondexDurationText(30), '1 daq');
      expect(ondexDurationText(249.7), '4 daq');
      expect(ondexDurationText(3600), '1 soat');
      expect(ondexDurationText(4800), '1 soat 20 daq');
    });
  });
}

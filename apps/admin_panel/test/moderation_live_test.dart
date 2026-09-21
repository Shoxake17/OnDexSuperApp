import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:chust_admin/ondexmap/editor_geometry.dart';
import 'package:chust_admin/ondexmap/moderation_api.dart';
import 'package:chust_admin/ondexmap/ondexmap_session_io.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:latlong2/latlong.dart';

// JONLI shartnoma testi: Dart klienti ↔ HAQIQIY OnDexMap serveri.
//
// Faqat `ONDEXMAP_LIVE=1` bo'lganda ishlaydi (OnDexMap API :8090 va admin :8091
// ishlab turishi, lokal sessiya fayli mavjud bo'lishi kerak). Sinov ob'ektini
// o'zi yaratadi va oxirida o'zi olib tashlaydi.
//
//     $env:ONDEXMAP_LIVE='1'; flutter test test/moderation_live_test.dart
//
// Nega kerak: Go va Dart alohida loyihalar — JSON kalitlari o'zgarsa mock
// testlar buni sezmaydi. Bu test ikki tomonni haqiqiy javoblar bilan tekshiradi.

const _api = 'http://localhost:8090';

void main() {
  final live = Platform.environment['ONDEXMAP_LIVE'] == '1';

  test('taklif yuborish → ro\'yxat → tahrirlab tasdiqlash → hammaga ko\'rinadi → olib tashlash',
      () async {
    final api = ModerationApi(defaultUrl: 'http://127.0.0.1:8091');
    expect(readOndexMapSession(), isNotNull, reason: 'lokal sessiya fayli yo\'q');

    // 1. Forma qoidalari serverdan.
    final meta = await api.meta();
    expect(meta.kinds.length, 11);
    expect(meta.kind('organization')!.allowed, contains('phone'));
    expect(meta.categories, contains('Kafe'));

    // 2. Ommaviy API orqali taklif yuboriladi (haqiqiy multipart).
    final tag = 'DARTLIVE${DateTime.now().millisecondsSinceEpoch.toRadixString(36)}';
    final req = http.MultipartRequest('POST', Uri.parse('$_api/v1/places'))
      ..headers['Origin'] = 'http://localhost:3100'
      ..fields['data'] = jsonEncode({
        'kind': 'organization', 'lat': 41.0031, 'lng': 71.2412,
        'name': '$tag Non', 'category': 'Kafe', 'phone': '+998 90 123-45-67',
        'hours': '09:00–18:00', 'description': '', 'street': '', 'house': '', 'website': '',
      })
      ..files.add(http.MultipartFile.fromBytes('photos', _png, filename: 'p.png'));
    final sent = await http.Response.fromStream(await req.send());
    expect(sent.statusCode, 201, reason: sent.body);

    // 3. Moderator ro'yxatda ko'radi.
    final list = await api.submissions('pending');
    final mine = list.items.firstWhere((s) => s.name == '$tag Non');
    expect(mine.kind, 'organization');
    expect(mine.kindLabel, 'Tashkilot');
    expect(mine.photos, 1);
    expect(mine.lat, closeTo(41.0031, 1e-6));
    expect(list.pending, greaterThanOrEqualTo(1));

    // Rasm haqiqiy JPEG (server tozalagan).
    final photo = await api.submissionPhoto(mine.id, 0);
    expect(photo.sublist(0, 2), [0xFF, 0xD8]);

    // Karantindagi ob'ekt ommaviy xaritada YO'Q.
    final before = await http.get(Uri.parse('$_api/v1/places?bbox=71.20,40.99,71.28,41.02'));
    expect(before.body, isNot(contains(tag)));

    // 4. Tuzatib tasdiqlaydi.
    final placeId = await api.approve(
      id: mine.id,
      edit: {
        'kind': 'organization', 'lat': mine.lat, 'lng': mine.lng,
        'name': '$tag Non (tuzatildi)', 'category': 'Kafe',
        'phone': mine.phone, 'hours': mine.hours,
        'street': '', 'house': '', 'description': '',
      },
      keepPhotos: [0],
    );

    try {
      // 5. Endi HAMMAGA ko'rinadi.
      final after = await http.get(Uri.parse('$_api/v1/places?bbox=71.20,40.99,71.28,41.02'));
      expect(after.body, contains(placeId));
      final detail = jsonDecode((await http.get(Uri.parse('$_api/v1/places/$placeId'))).body) as Map;
      expect(detail['name'], '$tag Non (tuzatildi)');
      expect(detail['photos'], 1);

      // Xaritadagi ob'ektlar ro'yxatida (moderator ko'radi).
      final placed = await api.places();
      expect(placed.any((p) => p.id == placeId), isTrue);
      expect((await api.submissions('pending')).items.any((s) => s.id == mine.id), isFalse);
    } finally {
      // 6. Olib tashlash.
      await api.deletePlace(placeId);
    }
    final gone = await http.get(Uri.parse('$_api/v1/places/$placeId'));
    expect(gone.statusCode, 404);
  }, skip: live ? false : 'ONDEXMAP_LIVE=1 emas — jonli test o\'tkazib yuborildi');

  test('muharrir: config → mahalla yaratish/yangilash → ko\'cha + muqobil nom → o\'chirish', () async {
    final api = ModerationApi(defaultUrl: 'http://127.0.0.1:8091');

    // Config: sir yo'q, faqat lokal proksi manzili.
    final cfg = await api.editorConfig();
    expect(cfg.satelliteUrl, isNotNull, reason: 'SATELLITE_URL sozlangan bo\'lishi kerak');
    expect(cfg.satelliteUrl, startsWith('http://127.0.0.1:'));
    expect(cfg.satelliteUrl, isNot(contains('token')));

    final tag = 'DARTLIVE${DateTime.now().millisecondsSinceEpoch.toRadixString(36)}';
    final createdIds = <String, String>{}; // id → qatlam
    try {
      // 1. Mahalla yaratish (Chust yaqinida, kichik kvadrat).
      const ring = [
        LatLng(41.0100, 71.2000),
        LatLng(41.0100, 71.2010),
        LatLng(41.0110, 71.2010),
        LatLng(41.0110, 71.2000),
      ];
      expect(drawingProblem(GeomKind.polygon, ring), isNull);
      final mid = await api.saveFeature(
        layer: 'mahalla',
        name: '$tag mahalla',
        source: 'survey',
        geometry: toGeoJson(GeomKind.polygon, ring),
      );
      createdIds[mid] = 'mahalla';

      // 2. Ro'yxatda: server ST_Multi qaytaradi — mijoz uni o'qiy oladi.
      final list = await api.features('mahalla');
      final mine = list.firstWhere((f) => f.id == mid);
      expect(mine.name, '$tag mahalla');
      expect(mine.source, 'survey');
      final shape = parseGeometry(mine.geometry);
      expect(shape, isNotNull, reason: 'serverdan kelgan geometriya o\'qildi');
      expect(shape!.kind, GeomKind.polygon);
      expect(shape.parts.single, hasLength(4));
      expect(shape.bounds[0], closeTo(41.0100, 1e-6));

      // 3. Yangilash: nom o'zgaradi, geometriya serverdan kelgani bilan AYNAN qayta yuboriladi.
      await api.saveFeature(
        layer: 'mahalla',
        id: mid,
        name: '$tag mahalla (yangi)',
        source: 'official',
        geometry: mine.geometry,
      );
      final after = (await api.features('mahalla')).firstWhere((f) => f.id == mid);
      expect(after.name, '$tag mahalla (yangi)');
      expect(after.source, 'official');
      expect(parseGeometry(after.geometry)!.parts.single, hasLength(4));

      // 4. Ko'cha + muqobil nom.
      final sid = await api.saveFeature(
        layer: 'street',
        name: '$tag ko\'cha',
        source: 'survey',
        streetKind: 'tor_kocha',
        geometry: toGeoJson(GeomKind.line, const [LatLng(41.0200, 71.2000), LatLng(41.0205, 71.2020)]),
      );
      createdIds[sid] = 'street';
      final street = (await api.features('street')).firstWhere((f) => f.id == sid);
      expect(street.streetKind, 'tor_kocha');
      expect(parseGeometry(street.geometry)!.kind, GeomKind.line);
      await api.addAlias(streetId: sid, alias: '$tag katta', kind: 'xalq', source: 'survey');

      // 5. Server tekshiruvi: hududdan tashqaridagi nuqta rad etiladi va xabar yetib keladi.
      await expectLater(
        api.saveFeature(
          layer: 'mahalla',
          name: '$tag tashqarida',
          source: 'survey',
          geometry: toGeoJson(GeomKind.polygon, const [LatLng(0, 0), LatLng(0, 1), LatLng(1, 1)]),
        ),
        throwsA(isA<ModerationException>().having((e) => e.message, 'm', contains('hudud'))),
      );
    } finally {
      // 6. Tozalash.
      for (final e in createdIds.entries) {
        await api.deleteFeature(e.value, e.key);
      }
    }
    for (final e in createdIds.entries) {
      final left = await api.features(e.value);
      expect(left.any((f) => f.id == e.key), isFalse, reason: 'o\'chirilmagan: ${e.key}');
    }
  }, skip: live ? false : 'ONDEXMAP_LIVE=1 emas — jonli test o\'tkazib yuborildi');

  test('server xatosi xabari mijozga yetib keladi (tekshiruv)', () async {
    final api = ModerationApi(defaultUrl: 'http://127.0.0.1:8091');
    await expectLater(
      api.approve(id: '00000000-0000-4000-8000-000000000000', edit: {'kind': 'zavod', 'lat': 41.0, 'lng': 71.2}, keepPhotos: const []),
      throwsA(isA<ModerationException>()
          .having((e) => e.message, 'message', contains('tur'))),
    );
    await expectLater(
      api.reject('00000000-0000-4000-8000-000000000000', 'x'),
      throwsA(isA<ModerationException>().having((e) => e.message, 'message', contains('topilmadi'))),
    );
  }, skip: live ? false : 'ONDEXMAP_LIVE=1 emas — jonli test o\'tkazib yuborildi');
}

/// 200×150 shaffof bo'lmagan PNG (server JPEG'ga aylantiradi).
final _png = _buildPng(200, 150);

List<int> _buildPng(int w, int h) {
  final raw = BytesBuilder();
  for (var y = 0; y < h; y++) {
    raw.addByte(0);
    for (var x = 0; x < w; x++) {
      raw.add([x % 255, y % 255, 120]);
    }
  }
  List<int> chunk(String type, List<int> data) {
    final td = [...type.codeUnits, ...data];
    final len = ByteData(4)..setUint32(0, data.length);
    final crc = ByteData(4)..setUint32(0, _crc32(td));
    return [...len.buffer.asUint8List(), ...td, ...crc.buffer.asUint8List()];
  }

  final ihdr = ByteData(13)
    ..setUint32(0, w)
    ..setUint32(4, h)
    ..setUint8(8, 8)
    ..setUint8(9, 2);
  return [
    137, 80, 78, 71, 13, 10, 26, 10,
    ...chunk('IHDR', ihdr.buffer.asUint8List()),
    ...chunk('IDAT', zlib.encode(raw.toBytes())),
    ...chunk('IEND', const []),
  ];
}

int _crc32(List<int> data) {
  var crc = 0xffffffff;
  for (final b in data) {
    crc ^= b;
    for (var k = 0; k < 8; k++) {
      crc = (crc & 1) != 0 ? (crc >> 1) ^ 0xedb88320 : crc >> 1;
    }
  }
  return (crc ^ 0xffffffff) & 0xffffffff;
}

import 'dart:async';
import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;

import 'map_backend.dart';

/// ONDEXMAP API KLIENTI — qidiruv va marshrut.
///
/// ┌─ NEGA `/v1`, `/v2` EMAS ───────────────────────────────────────────┐
/// `/v2` — TASHQI dasturchilar uchun: u API kalit talab qiladi va
/// kalit tariflar bo'yicha hisoblanadi. ChustApp esa OnDex'ning O'Z
/// mahsuloti, ya'ni birinchi tomon — unga kalit ham, hisob ham
/// kerak emas.
///
/// `/v1` aynan shu maqsad uchun: OnDexMap saytining o'zi ham shundan
/// foydalanadi. Kalit ilovaga joylashtirilmaydi, demak uni ajratib
/// olish ham mumkin emas.
/// └────────────────────────────────────────────────────────────────────┘

/// Qidiruv natijasi.
@immutable
class OndexPlace {
  const OndexPlace({
    required this.id,
    required this.name,
    required this.label,
    required this.lat,
    required this.lng,
  });

  final String id;
  final String name;

  /// O'zbekcha tur nomi: «Shaharcha», «Ko'cha», «Maktab».
  final String label;

  final double lat;
  final double lng;

  static OndexPlace? fromJson(Map<String, dynamic> j) {
    final lat = (j['lat'] as num?)?.toDouble();
    final lng = (j['lng'] as num?)?.toDouble();
    final id = j['id'] as String?;
    if (lat == null || lng == null || id == null) return null;
    return OndexPlace(
      id: id,
      name: (j['name'] as String?) ?? '',
      label: (j['label'] as String?) ?? (j['type'] as String?) ?? '',
      lat: lat,
      lng: lng,
    );
  }
}

/// Marshrut: yo'l chizig'i, masofa va vaqt.
@immutable
class OndexRoute {
  const OndexRoute({
    required this.points,
    required this.distanceMeters,
    required this.durationSeconds,
  });

  /// `[lat, lng]` juftliklari — xaritaga chizish uchun.
  final List<({double lat, double lng})> points;
  final double distanceMeters;
  final double durationSeconds;
}

/// Server javob bermaganda yoki rad etganda.
class OndexMapException implements Exception {
  OndexMapException(this.message);
  final String message;
  @override
  String toString() => message;
}

class OndexMapApi {
  OndexMapApi({http.Client? client}) : _client = client ?? http.Client();

  final http.Client _client;

  static const _timeout = Duration(seconds: 12);

  Uri _u(String path, Map<String, String> q) =>
      Uri.parse('$kOndexApiBase$path').replace(queryParameters: q);

  Future<Map<String, dynamic>> _get(Uri url) async {
    final http.Response res;
    try {
      res = await _client.get(url).timeout(_timeout);
    } on TimeoutException {
      throw OndexMapException('Xarita serveri javob bermadi');
    } catch (_) {
      throw OndexMapException('Internetni tekshiring');
    }
    if (res.statusCode != 200) {
      throw OndexMapException('Xarita serveri xatosi (${res.statusCode})');
    }
    try {
      return jsonDecode(utf8.decode(res.bodyBytes)) as Map<String, dynamic>;
    } catch (_) {
      throw OndexMapException('Javobni o\'qib bo\'lmadi');
    }
  }

  /// Nom bo'yicha qidirish. Qamrov — butun O'zbekiston.
  ///
  /// [near] berilsa natijalar shu nuqtaga yaqinlashtiriladi — xaritada
  /// qidirayotgan odam odatda ko'rinib turgan joyni nazarda tutadi.
  Future<List<OndexPlace>> search(
    String query, {
    ({double lat, double lng})? near,
    int limit = 12,
  }) async {
    final q = query.trim();
    if (q.length < 2) return const [];
    final params = {'q': q, 'limit': '$limit'};
    if (near != null) {
      params['lat'] = near.lat.toStringAsFixed(6);
      params['lng'] = near.lng.toStringAsFixed(6);
    }
    final j = await _get(_u('/v1/search', params));
    final raw = (j['results'] as List?) ?? const [];
    return [
      for (final r in raw)
        if (r is Map<String, dynamic>)
          if (OndexPlace.fromJson(r) case final p?) p,
    ];
  }

  /// A nuqtadan B nuqtagacha haydash marshruti.
  ///
  /// ⚠️ Bu endpoint butun mamlakatni QAMRAMAYDI — u
  /// [ondexRouteCovers] belgilagan hudud bilan cheklangan. Tashqarida
  /// server `400` qaytaradi, shuning uchun chaqiruvchi avval
  /// tekshirishi kerak.
  Future<OndexRoute> route({
    required double fromLat,
    required double fromLng,
    required double toLat,
    required double toLng,
  }) async {
    final j = await _get(_u('/v1/route', {
      'from_lat': fromLat.toStringAsFixed(6),
      'from_lng': fromLng.toStringAsFixed(6),
      'to_lat': toLat.toStringAsFixed(6),
      'to_lng': toLng.toStringAsFixed(6),
    }));

    final geom = j['geometry'] as Map<String, dynamic>?;
    final coords = (geom?['coordinates'] as List?) ?? const [];
    final pts = <({double lat, double lng})>[];
    for (final c in coords) {
      if (c is List && c.length >= 2) {
        final lng = (c[0] as num?)?.toDouble();
        final lat = (c[1] as num?)?.toDouble();
        if (lat != null && lng != null) pts.add((lat: lat, lng: lng));
      }
    }
    if (pts.length < 2) {
      throw OndexMapException('Bu ikki nuqta orasida yo\'l topilmadi');
    }
    return OndexRoute(
      points: pts,
      distanceMeters: (j['distance_m'] as num?)?.toDouble() ?? 0,
      durationSeconds: (j['duration_s'] as num?)?.toDouble() ?? 0,
    );
  }

  /// Yangi ob'ekt yuboradi (`POST /v1/places`).
  ///
  /// ┌─ NIMA BO'LADI SO'NG ───────────────────────────────────────────┐
  /// Ob'ekt darhol xaritaga CHIQMAYDI — u karantinga tushadi va
  /// moderator tasdiqlagandan keyin hammaga ko'rinadi. Server
  /// `status: "pending"` qaytaradi.
  /// └────────────────────────────────────────────────────────────────┘
  ///
  /// ⚠️ `website` maydoni ATAYLAB bo'sh yuboriladi: u asalari
  /// (honeypot) — odam ko'rmaydi, bot to'ldiradi. To'ldirilgan
  /// bo'lsa server yuborishni jimgina tashlab yuboradi.
  Future<void> submitPlace({
    required String kind,
    required double lat,
    required double lng,
    required String name,
    String category = '',
    String description = '',
    String phone = '',
    String hours = '',
    String street = '',
    String house = '',
    String site = '',
    String social = '',
    List<({String name, List<int> bytes})> photos = const [],
  }) async {
    if (photos.length > kOndexMaxPhotos) {
      throw OndexMapException(
          'Ko\'pi bilan $kOndexMaxPhotos ta rasm yuborish mumkin');
    }
    for (final p in photos) {
      if (p.bytes.length > kOndexMaxPhotoBytes) {
        throw OndexMapException('«${p.name}» juda katta — 6 MB dan oshmasin');
      }
    }

    final req = http.MultipartRequest('POST', Uri.parse('$kOndexApiBase/v1/places'));
    req.fields['data'] = jsonEncode({
      'kind': kind,
      'lat': lat,
      'lng': lng,
      'name': name.trim(),
      'category': category.trim(),
      'description': description.trim(),
      'phone': phone.trim(),
      'hours': hours.trim(),
      'street': street.trim(),
      'house': house.trim(),
      'site': site.trim(),
      'social': social.trim(),
      // Asalari — hech qachon to'ldirilmaydi.
      'website': '',
    });
    for (final p in photos) {
      req.files.add(http.MultipartFile.fromBytes('photos', p.bytes,
          filename: p.name));
    }

    final http.StreamedResponse res;
    try {
      // Rasm yuklash sekin tarmoqda uzoq davom etadi — umumiy 12 s
      // muddat bu yerda kam.
      res = await _client.send(req).timeout(const Duration(seconds: 90));
    } on TimeoutException {
      throw OndexMapException('Yuborilmadi — internet sekin');
    } catch (_) {
      throw OndexMapException('Internetni tekshiring');
    }

    if (res.statusCode == 201 || res.statusCode == 200) return;

    final body = await res.stream.bytesToString();
    // Server xato sababini `{"error": "..."}` da beradi — uni
    // ko'rsatamiz, chunki u foydalanuvchi tuzata oladigan narsa
    // (nom qisqa, rasm katta, soatlik chegara).
    String xabar;
    try {
      xabar = (jsonDecode(body) as Map<String, dynamic>)['error'] as String? ??
          'Yuborilmadi';
    } catch (_) {
      xabar = 'Yuborilmadi (${res.statusCode})';
    }
    if (res.statusCode == 429) {
      xabar = 'Juda ko\'p ob\'ekt yubordingiz — birozdan keyin urinib ko\'ring';
    }
    throw OndexMapException(xabar);
  }

  void dispose() => _client.close();
}

/// Bitta ob'ektga qo'shsa bo'ladigan rasmlar soni (server chegarasi).
const int kOndexMaxPhotos = 4;

/// Bitta rasmning eng katta hajmi (server chegarasi).
const int kOndexMaxPhotoBytes = 6 * 1024 * 1024;

/// Qo'shsa bo'ladigan ob'ekt turlari.
///
/// ┌─ NEGA HAMMASI EMAS ────────────────────────────────────────────────┐
/// Serverda o'n bitta tur bor, lekin uchtasi CHIZIQ shaklida
/// (`road`, `crossing`, `fence`) — ularni qo'shish uchun xaritada
/// yo'l chizish kerak. Bu alohida muharrir talab qiladi, shuning
/// uchun hozircha nuqta turlari qabul qilinadi.
///
/// Ro'yxat va nomlar serverdagi `places.Kinds` bilan bir xil.
/// └────────────────────────────────────────────────────────────────────┘
const List<({String key, String label})> kOndexPlaceKinds = [
  (key: 'organization', label: 'Tashkilot'),
  (key: 'address', label: 'Manzil'),
  (key: 'entrance', label: 'Bino kirishi'),
  (key: 'barrier', label: 'Shlagbaum'),
  (key: 'stop', label: 'Transport bekati'),
  (key: 'parking', label: 'Avtoturargoh'),
  (key: 'gate', label: 'Darvoza'),
  (key: 'other', label: 'Boshqa ob\'ekt'),
];

/// Masofa: «1,7 km» yoki «340 m».
String ondexDistanceText(double meters) {
  if (meters < 1000) return '${meters.round()} m';
  final km = meters / 1000;
  return '${km.toStringAsFixed(km < 10 ? 1 : 0).replaceAll('.', ',')} km';
}

/// Vaqt: «5 daq» yoki «1 soat 20 daq».
String ondexDurationText(double seconds) {
  final m = (seconds / 60).round();
  if (m < 60) return '${m < 1 ? 1 : m} daq';
  final h = m ~/ 60;
  final qoldiq = m % 60;
  return qoldiq == 0 ? '$h soat' : '$h soat $qoldiq daq';
}

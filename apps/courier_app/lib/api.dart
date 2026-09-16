import 'dart:typed_data';

import 'package:google_maps_flutter/google_maps_flutter.dart' show LatLng;
import 'package:http/http.dart' as http;
import 'package:ondex_core/ondex_core.dart';

import 'voice_navigation.dart' show NavStep;

// Umumiy yadro qayta eksport qilinadi — mavjud ekranlar `api.dart` ni
// import qilgani uchun ular tegilmasdan ishlashda davom etadi.
export 'package:ondex_core/ondex_core.dart';

/// Backend manzili — endi `--dart-define=ONDEX_API_URL=...` orqali
/// beriladi (qarang: ondex_core/config.dart). Avval bu yerda LAN IP
/// QATTIQ yozilgan edi.
const baseUrl = apiBaseUrl;

/// Kuryer ilovasiga XOS endpointlar. Umumiy qismi (`send`, xato ishlash,
/// timeout, 401, `requestCode`/`verify`/`me`/`logout`/`wsTicket`/
/// `mapsApiKey`) `ondex_core.ApiClient` da.
class CourierApi extends ApiClient {
  CourierApi() : super(baseUrl: apiBaseUrl);

  /// Mijoz akkaunti kuryerlikka ariza beradi. Javobda YANGI token keladi
  /// (rol "courier"ga o'zgargani uchun) — chaqiruvchi shu tokenni saqlashi
  /// va `token`ga o'rnatishi kerak.
  /// [vehicleType] — dispatch matching engine uchun MUHIM: Google Distance
  /// Matrix'ga qaysi rejim (piyoda/velosiped/mashina) bilan murojaat
  /// qilinishini belgilaydi. Qiymatlar: "foot", "bike", "moped", "car".
  Future<Map<String, dynamic>> registerCourier(
    String name,
    String vehicleType,
  ) async {
    final d = await send('POST', '/couriers/register', {
      'name': name,
      'vehicle_type': vehicleType,
    });
    return Map<String, dynamic>.from(d);
  }

  /// Kuryerning o'z holati: approved, available, joylashuv.
  Future<Map<String, dynamic>> getCourier(String courierId) async =>
      Map<String, dynamic>.from(await send('GET', '/couriers/$courierId'));

  /// Kuryerning yakunlanmagan buyurtmasi ID'si yoki `null`.
  ///
  /// Ilova qayta ochilganda joriy buyurtmani TIKLASH uchun: avval u faqat
  /// xotirada turardi va Android fondagi ilovani o'ldirsa "olindi"/
  /// "yetkazdim" tugmalari yo'qolardi (kuryer ilovasi auditi, 2-band).
  Future<String?> activeOrderId(String courierId) async {
    final d = await send('GET', '/couriers/$courierId/active-order');
    return (d is Map) ? d['order_id'] as String? : null;
  }

  /// Kuryerga HOZIR ochiq turgan taklif (`WebSocket` "offer" xabari bilan bir
  /// xil shakl) yoki `null`. Ilova qayta ochilganda, oldinga chiqqanda va push
  /// bosilganda taklifni tiklash uchun.
  Future<Map<String, dynamic>?> pendingOffer(String courierId) async {
    final d = await send('GET', '/couriers/$courierId/offer');
    final offer = d is Map ? d['offer'] : null;
    return offer is Map ? Map<String, dynamic>.from(offer) : null;
  }

  /// "Siz `restoran` restoraniga yetib keldingiz" (WAV, serverda bir marta
  /// yasalgan) yoki `null` — u holda ilovadagi umumiy ibora aytiladi.
  /// Matnni ilova bermaydi: server nomni buyurtmadan oladi.
  Future<Uint8List?> arrivalVoice(String courierId, String orderId) async {
    try {
      final uri = Uri.parse('$baseUrl/couriers/$courierId/voice/arrival'
          '?order_id=${Uri.encodeQueryComponent(orderId)}');
      final r = await http.get(uri, headers: {
        if (token != null) 'Authorization': 'Bearer $token',
      }).timeout(const Duration(seconds: 60));
      final type = r.headers['content-type'] ?? '';
      if (r.statusCode != 200 || !type.startsWith('audio/') || r.bodyBytes.length > 4 * 1024 * 1024) {
        return null;
      }
      return r.bodyBytes;
    } catch (_) {
      return null;
    }
  }

  /// Push tokeni — kuryer akkauntiga bog'lanadi (`lib/push.dart`).
  Future<void> savePushToken(String token, String platform) =>
      send('POST', '/me/push-token', {'token': token, 'platform': platform});

  /// Chiqishda: token QUERY orqali (server shu yerdan o'qiydi).
  Future<void> deletePushToken(String token) =>
      send('DELETE', '/me/push-token?token=${Uri.encodeQueryComponent(token)}');

  Future<void> setAvailable(String courierId, bool available) =>
      send('POST', '/couriers/$courierId/available', {'available': available});

  /// Joriy koordinatani serverga yuboradi (dispatch shu qiymatga qarab
  /// eng yaqin kuryerni tanlaydi).
  Future<void> updateLocation(String courierId, double lat, double lng) =>
      send('POST', '/couriers/$courierId/location', {'lat': lat, 'lng': lng});

  /// Taklifga javob: qabul qilish yoki rad etish.
  Future<void> respond(String courierId, String orderId, bool accepted) =>
      send('POST', '/couriers/$courierId/respond', {
        'order_id': orderId,
        'accepted': accepted,
      });

  Future<Map<String, dynamic>> getOrder(String id) async =>
      Map<String, dynamic>.from(await send('GET', '/orders/$id'));

  /// Restoran ma'lumoti (nomi, manzili) — buyurtmani olish uchun qayerga
  /// borish kerakligini ko'rsatish uchun (ochiq endpoint).
  Future<Map<String, dynamic>> restaurant(String id) async =>
      Map<String, dynamic>.from(await send('GET', '/restaurants/$id'));

  /// Koordinatani manzil matniga aylantiradi (yetkazib berish nuqtasini
  /// ko'rsatish uchun) — backend orqali (CORS sabab to'g'ridan-to'g'ri
  /// brauzerdan chaqirib bo'lmaydi).
  // Mantiq `ondex_core.ApiClient.reverseGeocode` da.

  /// Ikki nuqta orasidagi HAQIQIY yo'l marshrutini (Google Directions API,
  /// backend orqali) oladi — xaritada kuryerdan restoran/mijozgacha chiziq
  /// chizish uchun VA taklif kartochkasida "necha daqiqada yetib borishi"
  /// vaqtini ko'rsatish uchun ([RouteResult.durationSeconds]). [mode]:
  /// "driving" (moped/mashina), "bicycling" (velosiped) yoki "walking"
  /// (piyoda) — kuryerning transport turiga mos. Marshrut topilmasa
  /// (masalan piyoda uchun imkonsiz masofa) bo'sh natija qaytadi.
  Future<RouteResult> route(
    double originLat,
    double originLng,
    double destLat,
    double destLng, {
    String mode = 'driving',
  }) async {
    try {
      final d = await send(
        'GET',
        '/geocode/route?origin_lat=$originLat&origin_lng=$originLng'
            '&dest_lat=$destLat&dest_lng=$destLng&mode=$mode',
      );
      final points = (d['points'] as List?) ?? [];
      // Ovozli yo'l ko'rsatish uchun manevrlar (noto'g'ri yozuvlar tashlanadi).
      final steps = <NavStep>[];
      final rawSteps = d['steps'];
      if (rawSteps is List) {
        for (final s in rawSteps.take(200)) {
          if (s is! Map) continue;
          final m = s['maneuver'];
          final lat = s['lat'];
          final lng = s['lng'];
          if (m is String && lat is num && lng is num && lat.isFinite && lng.isFinite) {
            steps.add(NavStep(m, LatLng(lat.toDouble(), lng.toDouble())));
          }
        }
      }
      return RouteResult(
        steps: steps,
        points: points
            .map(
              (p) => LatLngPoint(
                (p['lat'] as num).toDouble(),
                (p['lng'] as num).toDouble(),
              ),
            )
            .toList(),
        durationSeconds: (d['duration_seconds'] as num?)?.toInt(),
        distanceMeters: (d['distance_meters'] as num?)?.toInt(),
      );
    } on ApiException {
      return const RouteResult(points: [], durationSeconds: null);
    }
  }

  /// Buyurtma holatini o'zgartiradi — server YANGILANGAN buyurtmani
  /// qaytaradi (masalan "picked_up"ga o'tganda, taomlar/mijoz manzili
  /// endi ko'rinadi — bulargacha yashiringan edi).
  Future<Map<String, dynamic>> transition(String orderId, String to) async =>
      Map<String, dynamic>.from(
        await send('POST', '/orders/$orderId/transition', {'to': to}),
      );

  // `mapsApiKey()`, `me()`, `logout()`, `wsTicket()`, `requestCode()`,
  // `verify()` — hammasi `ondex_core.ApiClient` da (meros orqali
  // mavjud). Bu yerda TAKRORLANMAYDI.
}

/// Xaritada chiziladigan marshrut nuqtasi.
class LatLngPoint {
  final double lat;
  final double lng;
  const LatLngPoint(this.lat, this.lng);
}

/// Google Directions natijasi — polyline nuqtalari + vaqt/masofa.
///
/// Marshrut topilmasa (piyoda uchun imkonsiz masofa, tarmoq xatosi)
/// bo'sh `points` bilan qaytadi va xarita shunchaki chiziqsiz
/// ko'rsatiladi — istisno tashlanmaydi.
class RouteResult {
  final List<LatLngPoint> points;
  final int? durationSeconds;
  final int? distanceMeters;

  /// Aytiladigan manevrlar (ovozli yo'l ko'rsatish).
  final List<NavStep> steps;
  const RouteResult({
    required this.points,
    required this.durationSeconds,
    this.distanceMeters,
    this.steps = const [],
  });
}

/// Butun ilova uchun bitta umumiy client.
final api = CourierApi();

/// Rasm manzili — `ondex_core.coreFullImageUrl` ustidagi yupqa o'ram.
String fullImageUrl(String? path) => coreFullImageUrl(path, apiBaseUrl);
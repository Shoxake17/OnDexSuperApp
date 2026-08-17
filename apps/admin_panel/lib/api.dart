import 'dart:convert';
import 'dart:typed_data';

import 'package:http/http.dart' as http;
import 'package:ondex_core/ondex_core.dart';

// Umumiy yadro qayta eksport qilinadi - sahifalar api.dart ni import
// qilgani uchun formatSum, ApiException va boshqalarga o'zgarishsiz
// kirishda davom etadi. Ular endi 4 ta ilovada takrorlanmaydi.
export 'package:ondex_core/ondex_core.dart';



/// Backend manzili — BUILD vaqtida `--dart-define-from-file` orqali keladi,
/// bu yerga qattiq yozilmaydi (`config/dev.json`, `config/dev-tunnel.json`,
/// `config/prod.json`). Sozlama berilmasa `http://localhost:8080`.
const baseUrl = apiBaseUrl;

class AdminApi {
  String? token;

  Map<String, String> get _headers => {
        'Content-Type': 'application/json',
        if (token != null) 'Authorization': 'Bearer $token',
        // `ondex_core` dagi bir xil qiymat (`<platforma>/<versiya>`).
        'X-Ondex-Client': clientHeaderValue,
      };

  Future<dynamic> _send(String method, String path,
      [Map<String, dynamic>? body]) async {
    final uri = Uri.parse('$baseUrl$path');
    final http.Response r;
    if (method == 'GET') {
      r = await http.get(uri, headers: _headers);
    } else if (method == 'DELETE') {
      r = await http.delete(uri, headers: _headers);
    } else {
      r = await http.post(uri, headers: _headers, body: jsonEncode(body ?? {}));
    }
    return _parse(r);
  }

  dynamic _parse(http.Response r) {
    final data = r.body.isEmpty ? null : jsonDecode(utf8.decode(r.bodyBytes));
    if (r.statusCode >= 400) {
      throw ApiException(
          (data is Map && data['error'] != null) ? data['error'] : 'Server xatosi');
    }
    return data;
  }

  /// Telegram bot orqali tasdiqlash kodini so'raydi.
  ///
  /// ┌─ KOD BU JAVOBDA YO'Q — ATAYLAB ──────────────────────────────────┐
  /// Server faqat bir martalik deep link qaytaradi. Kod foydalanuvchi
  /// botda RAQAMINI ULASHGANDAN va u shu yerda kiritilgan raqam bilan
  /// MOS KELGANDAN keyingina yaratiladi hamda FAQAT Telegram orqali
  /// yetkaziladi (`internal/telegram/verifier.go` — mos kelmasa kod
  /// umuman yuborilmaydi).
  ///
  /// Shu sabab havolani begonaga yuborish foyda bermaydi: kod raqam
  /// egasining Telegramiga tushadi, havolani ochgan odamnikiga emas.
  /// └──────────────────────────────────────────────────────────────────┘
  ///
  /// Avval bu yerda `/auth/request-code` chaqirilardi va kod dev rejimda
  /// javobda (`dev_code`) qaytardi. Production'da SMS provayderi
  /// ulanmagani uchun u kodni HECH QAYERGA yetkazmasdi — panelga kirish
  /// amalda imkonsiz edi.
  Future<String> telegramStart(String phone) async {
    final d = await _send('POST', '/auth/telegram/start', {'phone': phone});
    return d['deep_link'] as String;
  }

  Future<Map<String, dynamic>> verify(String phone, String code) async {
    final d = await _send('POST', '/auth/verify', {'phone': phone, 'code': code});
    token = d['token'] as String;
    return Map<String, dynamic>.from(d);
  }

  /// Google Maps kaliti — server .env dan beradi (faqat login qilganlarga).
  Future<String> mapsApiKey() async {
    final d = await _send('GET', '/config/maps');
    return d['maps_api_key'] as String;
  }

  /// Koordinatani manzil matniga aylantiradi — server orqali (Google
  /// Geocoding API'ni brauzerdan to'g'ridan-to'g'ri chaqirish CORS
  /// tomonidan bloklanadi, shuning uchun backend proksi qiladi).
  Future<String?> reverseGeocode(double lat, double lng) async {
    final d = await _send('GET', '/geocode/reverse?lat=$lat&lng=$lng');
    final addr = d['address'] as String?;
    return (addr == null || addr.isEmpty) ? null : addr;
  }

  /// WebSocket bileti — jonli kanal uchun (`lib/live.dart`).
  ///
  /// Uzoq muddatli token URL'ga HECH QACHON qo'yilmaydi: avval shu
  /// yerda 30 soniyalik, bir martalik bilet olinadi (sabab
  /// `internal/ws/tickets.go` da). Superadmin roli ham AYNAN shu
  /// biletga yoziladi — ma'muriyat kanaliga obuna shundan keladi.
  Future<String> wsTicket() async {
    final d = await _send('POST', '/ws/ticket');
    return d['ticket'] as String;
  }

  Future<Map<String, dynamic>> stats() async =>
      Map<String, dynamic>.from(await _send('GET', '/admin/stats'));

  Future<List<dynamic>> orders() async =>
      (await _send('GET', '/admin/orders')) as List<dynamic>? ?? [];

  Future<List<dynamic>> couriers() async =>
      (await _send('GET', '/admin/couriers')) as List<dynamic>? ?? [];

  Future<void> approveCourier(String id, bool approved) =>
      _send('POST', '/admin/couriers/$id/approve', {'approved': approved});

  /// Mijozlar ro'yxati: `{'count': int, 'items': [...]}`.
  ///
  /// Har bir yozuvda `devices` — foydalanuvchi qaysi ilovadan
  /// kirgani (`tma`, `android`, `ios`, `web`, ...) va ilova versiyasi.
  /// Server buni har so'rovdagi `X-Ondex-Client` sarlavhasidan yig'adi.
  Future<Map<String, dynamic>> customers() async =>
      Map<String, dynamic>.from(await _send('GET', '/admin/customers'));

  /// Affitsiantlar ro'yxati — mijozlar bilan bir xil shaklda, ustiga
  /// `restaurant_id`/`restaurant_name` qo'shiladi.
  Future<Map<String, dynamic>> waiters() async =>
      Map<String, dynamic>.from(await _send('GET', '/admin/waiters'));

  /// Akkauntni o'chiradi.
  ///
  /// Server tomonda bu bitta amalda uch ishni bajaradi: sessiyani bekor
  /// qiladi (qo'lidagi token darhol yaroqsiz bo'ladi), ochiq ilovaga
  /// WebSocket orqali `account_deleted` yuboradi va shaxsiy
  /// ma'lumotlarni o'chiradi. Yakunlanmagan buyurtmasi bor akkaunt
  /// uchun 409 qaytadi — bu XATO EMAS, ataylab qo'yilgan to'siq.
  Future<void> deleteUser(String id) => _send('DELETE', '/admin/users/$id');

  Future<List<dynamic>> restaurants() async =>
      (await _send('GET', '/restaurants')) as List<dynamic>? ?? [];

  Future<Map<String, dynamic>> createRestaurant({
    required String name,
    required String address,
    required String phone,
    required String staffName,
    double lat = 41.0030,
    double lng = 71.2360,
  }) async =>
      Map<String, dynamic>.from(await _send('POST', '/admin/restaurants', {
        'name': name,
        'address': address,
        'phone': phone,
        'staff_name': staffName,
        'lat': lat,
        'lng': lng,
      }));

  Future<void> setRestaurantOpen(String id, bool open) =>
      _send('POST', '/admin/restaurants/$id/open', {'open': open});

  Future<void> deleteRestaurant(String id) =>
      _send('DELETE', '/admin/restaurants/$id');

  /// Restoran ma'lumotlarini tahrirlaydi. Har doim TO'LIQ holatni
  /// yuboradi (edit oynasi mavjud qiymatlar bilan oldindan to'ldirilgan).
  ///
  /// ┌─ REYTING/ETA PARAMETRLARI NEGA `required` ────────────────────────┐
  /// Server bu maydonlarni SHARTSIZ o'zlashtiradi. Ya'ni ular JSON'da
  /// bo'lmasa Go nol qiymat oladi va mavjud reyting/vaqt JIMGINA
  /// NOLLANADI — tahrir oynasida faqat nomni o'zgartirgan admin
  /// buni sezmasdi. `required` shu xatoni kompilyatsiya vaqtida
  /// to'xtatadi (standart qiymat qo'yilsa, kelajakdagi chaqiruvchi
  /// uni jimgina o'tkazib yuborardi).
  /// └───────────────────────────────────────────────────────────────────┘
  Future<Map<String, dynamic>> editRestaurant({
    required String id,
    required String name,
    required String address,
    required double lat,
    required double lng,
    required double rating,
    required int ratingCount,
    required int etaMinMinutes,
    required int etaMaxMinutes,
    String logoUrl = '',
    String coverUrl = '',
    String tags = '',
  }) async =>
      Map<String, dynamic>.from(await _send('POST', '/admin/restaurants/$id', {
        'name': name,
        'address': address,
        'lat': lat,
        'lng': lng,
        'logo_url': logoUrl,
        'cover_url': coverUrl,
        'tags': tags,
        'rating': rating,
        'rating_count': ratingCount,
        'eta_min_minutes': etaMinMinutes,
        'eta_max_minutes': etaMaxMinutes,
      }));

  /// Rasm yuklaydi. `type`: 'cover' — restoran banneri (keng, to'ldirib
  /// kesiladi), 'logo' — restoran logosi (kvadrat, TO'LDIRIB kesiladi,
  /// oq joysiz), bo'sh — mahsulot rasmi (kvadrat, oq joy bilan).
  Future<String> uploadImage(Uint8List bytes, String filename,
      {String type = ''}) async {
    final uri = Uri.parse(
        '$baseUrl/uploads${type.isEmpty ? '' : '?type=$type'}');
    final req = http.MultipartRequest('POST', uri)
      ..headers['Authorization'] = 'Bearer $token'
      ..files.add(http.MultipartFile.fromBytes('file', bytes, filename: filename));
    final streamed = await req.send();
    final resp = await http.Response.fromStream(streamed);
    final data = _parse(resp);
    return data['url'] as String;
  }
}

final api = AdminApi();

/// Server qaytargan rasm manzilini ko'rsatish uchun tayyorlaydi: lokal disk
/// rejimida nisbiy yo'l ("/uploads/...") keladi — baseUrl qo'shiladi; R2
/// rejimida to'liq URL keladi — o'zgartirmasdan ishlatiladi.
String imageUrl(String? path) {
  if (path == null || path.isEmpty) return '';
  if (path.startsWith('http://') || path.startsWith('https://')) return path;
  return '$baseUrl$path';
}

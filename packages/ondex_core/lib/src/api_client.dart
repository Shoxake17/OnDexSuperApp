import 'dart:convert';

import 'package:http/http.dart' as http;

import 'config.dart';

/// Serverdan kelgan xato — barcha ilovalar shu bitta turdan foydalanadi.
class ApiException implements Exception {
  final String message;

  /// HTTP status kodi. Avval TASHLAB YUBORILARDI: to'rtala ilovada ham
  /// `statusCode` hech qayerda saqlanmasdi va hammasi bir xil
  /// `ApiException` ga aylanardi. Natijada `401` (token eskirgan) va
  /// `500` (server yotdi) farqlanmasdi — restoran paneli 401 ni yutib,
  /// Kanban'da "Hozircha faol buyurtma yo'q" ko'rsatardi va oshxona
  /// ishlashni to'xtatardi.
  final int statusCode;

  ApiException(this.message, {this.statusCode = 0});

  /// Sessiya yaroqsiz — chaqiruvchi login ekraniga qaytarishi kerak.
  bool get isUnauthorized => statusCode == 401;

  /// Tarmoq/timeout xatosi (server javob bermadi).
  bool get isNetwork => statusCode == 0;

  @override
  String toString() => message;
}

/// OnDex API'siga so'rov yuboradigan YAGONA klient.
///
/// Avval bu sinf to'rtta ilovada (mijoz, kuryer, admin panel, restoran
/// paneli) deyarli baytma-bayt takrorlangan edi — `_send`, `_parse`,
/// `ApiException`, `requestCode`, `verify`, `wsTicket` hammasi 4 nusxada.
/// Oqibati shunchaki ortiqcha kod emas edi: HAR BIR tuzatishni 4 marta
/// qilish kerak bo'lardi va amalda 4 marta qilinmasdi. Masalan `401`
/// ishlash va HTTP timeout to'rtala nusxada ham yo'q edi.
class ApiClient {
  ApiClient({required this.baseUrl, this.timeout = const Duration(seconds: 20)});

  /// Backend manzili. `--dart-define=ONDEX_API_URL=...` orqali beriladi
  /// (qarang: `config.dart`) — kodga yozilmaydi.
  final String baseUrl;

  /// HAR BIR so'rov uchun muddat.
  ///
  /// `package:http` da standart timeout YO'Q. Yarim ochiq TCP ulanish
  /// (Wi-Fi'dan mobil internetga o'tish) Future'ni ABADIY osib qo'yardi —
  /// foydalanuvchi "Yuklanmoqda..." ni cheksiz ko'rardi va qayta urinish
  /// yo'li yo'q edi.
  final Duration timeout;

  String? token;

  /// Sessiya yaroqsiz bo'lganda (401) chaqiriladi — ilova login ekraniga
  /// qaytarishi uchun. Har bir ilova o'zi ulaydi.
  void Function()? onUnauthorized;

  Map<String, String> get _headers => {
        'Content-Type': 'application/json',
        if (token != null) 'Authorization': 'Bearer $token',
        // Qaysi ilova va qaysi versiya — superadmin panelidagi
        // "Qurilma" ustuni uchun (`config.dart`). Faqat ma'lumot:
        // server hech qanday huquqni bu qiymatga bog'lamaydi.
        'X-Ondex-Client': clientHeaderValue,
      };

  Future<dynamic> send(String method, String path,
      [Map<String, dynamic>? body]) async {
    final uri = Uri.parse('$baseUrl$path');
    http.Response r;
    try {
      switch (method) {
        case 'GET':
          r = await http.get(uri, headers: _headers).timeout(timeout);
        case 'DELETE':
          r = await http.delete(uri, headers: _headers).timeout(timeout);
        default:
          r = await http
              .post(uri, headers: _headers, body: jsonEncode(body ?? {}))
              .timeout(timeout);
      }
    } catch (_) {
      // Tarmoq xatosi/timeout — statusCode 0 bilan belgilanadi, shunda
      // chaqiruvchi uni "server xatosi" dan ajrata oladi.
      throw ApiException('Serverga ulanib bo\'lmadi — internetni tekshiring');
    }
    return _parse(r);
  }

  dynamic _parse(http.Response r) {
    dynamic data;
    if (r.body.isNotEmpty) {
      try {
        data = jsonDecode(utf8.decode(r.bodyBytes));
      } catch (_) {
        // JSON emas (masalan nginx 502 HTML sahifasi). Avval bu
        // `FormatException` bo'lib, status tekshiruvidan OLDIN otilardi
        // va `on ApiException catch` bloklari uni umuman ushlamasdi —
        // tugma bosilardi, hech narsa bo'lmasdi, xabar chiqmasdi.
        data = null;
      }
    }
    if (r.statusCode >= 400) {
      final msg = (data is Map && data['error'] != null)
          ? data['error'].toString()
          : 'Server xatosi (${r.statusCode})';
      if (r.statusCode == 401) onUnauthorized?.call();
      throw ApiException(msg, statusCode: r.statusCode);
    }
    return data;
  }

  // ---------- Barcha ilovalarga umumiy endpointlar ----------

  /// SMS kod so'raydi. Dev rejimda server kodni javobda qaytaradi.
  ///
  /// DIQQAT: production'da bu kodni yetkazishi SMS provayderiga bog'liq.
  /// Eskiz sozlanmagan bo'lsa server `LogSms` ga tushadi — kod faqat
  /// server logiga yoziladi, javob esa baribir muvaffaqiyatli bo'ladi.
  /// Ya'ni ilova "kod yuborildi" deb ko'rsatadi-yu, hech kim kod
  /// olmaydi. Shu sabab `telegramStart` birinchi tanlov bo'lishi kerak.
  Future<String?> requestCode(String phone) async {
    final d = await send('POST', '/auth/request-code', {'phone': phone});
    return (d is Map) ? d['dev_code'] as String? : null;
  }

  /// Telegram bot orqali kod so'raydi — javob bir martalik deep link.
  ///
  /// ┌─ KOD BU JAVOBDA YO'Q — ATAYLAB ─────────────────────────────────┐
  /// Kod foydalanuvchi botda RAQAMINI ULASHGANDAN va u shu yerda
  /// berilgan raqam bilan MOS KELGANDAN keyingina yaratiladi
  /// (`internal/telegram/verifier.go`). Shuning uchun havolani
  /// begonaga yuborish foyda bermaydi: kod raqam EGASINING
  /// Telegramiga tushadi, havolani ochgan odamnikiga emas.
  /// └─────────────────────────────────────────────────────────────────┘
  ///
  /// Kod oxirida odatdagi `CodeStore` ga tushadi, ya'ni tekshirish
  /// baribir `verify()` bilan — alohida mantiq kerak emas.
  Future<String> telegramStart(String phone) async {
    final d = await send('POST', '/auth/telegram/start', {'phone': phone});
    return d['deep_link'] as String;
  }

  /// Kodni tekshiradi va tokenni o'rnatadi. Javob: {token, user}.
  Future<Map<String, dynamic>> verify(String phone, String code) async {
    final d = await send('POST', '/auth/verify', {'phone': phone, 'code': code});
    token = d['token'] as String;
    return Map<String, dynamic>.from(d);
  }

  /// Login qilgan foydalanuvchining ma'lumotlari (rol shu yerdan aniqlanadi).
  Future<Map<String, dynamic>> me() async =>
      Map<String, dynamic>.from(await send('GET', '/me'));

  /// Serverda sessiyani bekor qiladi — shu foydalanuvchining BARCHA
  /// tokenlari darhol yaroqsiz bo'ladi.
  ///
  /// Xato yutiladi: tarmoq yo'q bo'lsa ham foydalanuvchi ilovadan chiqa
  /// olishi kerak. Bu — qo'shimcha himoya, chiqishning sharti emas.
  Future<void> logout() async {
    try {
      await send('POST', '/auth/logout');
    } catch (_) {}
  }

  /// `GET /ws` uchun qisqa muddatli (30s), bir martalik bilet.
  ///
  /// Brauzer WebSocket API'si `Authorization` header qo'ya olmaydi,
  /// shuning uchun uzoq muddatli JWT hech qachon URL'da yurmaydi.
  Future<String> wsTicket() async {
    final d = await send('POST', '/ws/ticket');
    return d['ticket'] as String;
  }

  /// Google Maps kaliti — server `.env` dan beradi (faqat login qilganlarga).
  Future<String> mapsApiKey() async {
    final d = await send('GET', '/config/maps');
    return d['maps_api_key'] as String;
  }
}

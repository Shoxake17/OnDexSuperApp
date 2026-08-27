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
        case 'PATCH':
          // Qisman yangilash (masalan `PATCH /tables/{id}`). Busiz
          // restoran paneli o'z klientini saqlashga majbur edi.
          r = await http
              .patch(uri, headers: _headers, body: jsonEncode(body ?? {}))
              .timeout(timeout);
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

  /// Koordinatadan manzil matni (`GET /geocode/reverse`).
  ///
  /// Google Geocoding API brauzerdan CORS tufayli chaqirilmaydi —
  /// shuning uchun so'rov SERVER orqali o'tadi. Uchta ilovada
  /// (mijoz, kuryer, admin) bir xil yozilgan edi.
  Future<String> reverseGeocode(double lat, double lng) async {
    final d = await send('GET', '/geocode/reverse?lat=$lat&lng=$lng');
    return (d is Map ? d['address'] as String? : null) ?? '';
  }

  /// Rasm yuklaydi va server bergan yo'lni qaytaradi (`POST /uploads`).
  ///
  /// ┌─ NEGA ALOHIDA ─────────────────────────────────────────────────┐
  /// Bu YAGONA `multipart` so'rov — qolganlari JSON. Shuning uchun u
  /// `send` orqali o'tmaydi, lekin javobni AYNAN o'sha `_parse` bilan
  /// o'qiydi: 401, JSON bo'lmagan javob va xato matni bir xil
  /// ishlansin.
  ///
  /// Admin va restoran panellarida ikki nusxa edi va ikkalasi ham
  /// timeout'siz — yarim ochiq ulanishda yuklash abadiy osilib qolardi.
  /// └────────────────────────────────────────────────────────────────┘
  /// [type] — server rasmni qanday kesishini belgilaydi: `cover`
  /// (keng banner), `logo` (kvadrat, oq joysiz), bo'sh (mahsulot rasmi).
  Future<String> uploadImage(
    List<int> bytes,
    String filename, {
    String type = '',
    String field = 'file',
    Map<String, String> fields = const {},
  }) async {
    final data = await sendMultipart(
      'POST',
      '/uploads${type.isEmpty ? '' : '?type=$type'}',
      bytes,
      filename,
      field: field,
      fields: fields,
      errorText: 'Rasm yuklanmadi — internetni tekshiring',
      // Rasm chegarasi 5MB — odatiy timeout yetadi. Uzunroq PDF
      // timeout'i bu yerga tarqalmasin: xatti-harakat o'zgarmasligi kerak.
      uploadTimeout: timeout,
    );
    return (data is Map ? data['url'] as String? : null) ?? '';
  }

  /// Ixtiyoriy `multipart` yuklash — javob JSON sifatida qaytadi.
  ///
  /// `uploadImage` faqat `url` ni qaytaradi va bu uzoq vaqt yetarli edi.
  /// Kitob PDF i esa manzil BILAN BIRGA ajratilgan matnni ham qaytaradi,
  /// ya'ni bitta satrga sig'maydi — shuning uchun transport shu yerga
  /// ajratildi va `uploadImage` uning ustiga qurildi.
  ///
  /// Yuklash uzoq davom etadi (25 MB PDF), shuning uchun timeout odatiy
  /// so'rovnikidan uzunroq: aks holda katta fayl har safar uzilardi.
  Future<dynamic> sendMultipart(
    String method,
    String path,
    List<int> bytes,
    String filename, {
    String field = 'file',
    Map<String, String> fields = const {},
    String errorText = 'Fayl yuklanmadi — internetni tekshiring',
    Duration? uploadTimeout,
  }) async {
    final req = http.MultipartRequest(method, Uri.parse('$baseUrl$path'))
      ..headers['X-Ondex-Client'] = clientHeaderValue
      ..fields.addAll(fields)
      ..files.add(http.MultipartFile.fromBytes(field, bytes,
          filename: filename));
    if (token != null) req.headers['Authorization'] = 'Bearer $token';

    http.Response resp;
    try {
      final streamed =
          await req.send().timeout(uploadTimeout ?? timeout * 6);
      resp = await http.Response.fromStream(streamed);
    } catch (_) {
      throw ApiException(errorText);
    }
    return _parse(resp);
  }
}

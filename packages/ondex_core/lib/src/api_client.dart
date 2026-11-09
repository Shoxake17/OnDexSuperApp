import 'dart:convert';

import 'package:http/http.dart' as http;

import 'analytics.dart';
import 'config.dart';
import 'token_store.dart';

enum _RefreshResult { ok, rejected, network }

/// 401 bo'lsa refresh QILINMAYDIGAN yo'llar (o'zlari token beradi yoki
/// kirish bosqichi: 401 = noto'g'ri parol/kod, sessiya tugashi emas).
const _noRefresh = {
  '/v2/auth/token/refresh',
  '/v2/auth/merchant/otp/verify',
  '/v2/auth/merchant/password',
  '/v2/auth/admin/otp/verify',
  '/v2/auth/mfa/password',
  '/v2/auth/mfa/factor',
  '/v2/auth/mfa/verify',
  '/v2/auth/mfa/totp/enroll',
  '/v2/auth/mfa/totp/confirm',
};

/// Biznes egasi kirishining bir bosqichi natijasi.
class MerchantStep {
  MerchantStep._done(this.data) : done = true, mfaToken = null, method = null;
  MerchantStep._challenge(this.mfaToken, this.method, this.data) : done = false;

  /// Sessiya o'rnatildi (token omborda).
  final bool done;

  /// Keyingi bosqich tokeni (5–10 daqiqa).
  final String? mfaToken;

  /// `otp` | `password` | `totp_enroll` | `totp`.
  final String? method;

  /// Server javobi.
  final Map<String, dynamic> data;
}

/// TOTP ulash ma'lumoti: `uri` — QR uchun, `secret` — qo'lda kiritish uchun.
class TotpEnrollment {
  const TotpEnrollment({required this.secret, required this.uri});
  final String secret;
  final String uri;
}

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

  /// Xato javobining TO'LIQ tanasi (JSON obyekt bo'lsa).
  ///
  /// `message` faqat `error` maydonini oladi, ya'ni server qo'shgan
  /// qolgan hamma narsa yo'qolardi. Ba'zi xatolar esa davom etish
  /// uchun ZARUR ma'lumot bilan keladi — masalan `409 total_changed`
  /// javobidagi YANGI summa (`total_tiyin`): usiz ilova "narx
  /// o'zgardi" deb ayta oladi-yu, yangi raqamni ko'rsata olmaydi.
  ///
  /// Raqamni foydalanuvchiga ko'rsatiladigan matndan ajratib olish
  /// (regex bilan) mo'rt yo'l: matn tarjima qilinsa yoki qayta
  /// yozilsa jimgina ishlamay qoladi.
  ///
  /// JSON obyekt bo'lmasa `null`.
  final Map<String, dynamic>? data;

  ApiException(this.message, {this.statusCode = 0, this.data});

  /// Xato kodi (`data['code']`) — matnga emas, shu maydonga qarang.
  String get code => (data?['code'] ?? '').toString();

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
  ApiClient({
    required this.baseUrl,
    this.timeout = const Duration(seconds: 20),
  });

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

  // ---------- Markaziy sessiya (A4a, AUTH-PHASE-A.md) ----------
  //
  // ┌─ QANDAY ISHLAYDI ───────────────────────────────────────────────────┐
  // Yangi kirish (`merchantVerify`) access token (10 daqiqa) VA refresh
  // token beradi. Ikkalasi `TokenStore` da (Keystore/Keychain) saqlanadi.
  // 401 kelsa so'rov BIR MARTA refresh qilinib qayta yuboriladi — faqat
  // refresh ham rad etilsa (sessiya markazda yopilgan, xodim bo'shatilgan)
  // `onUnauthorized` chaqiriladi. Tarmoq xatosida sessiya O'CHIRILMAYDI.
  //
  // Bir vaqtda kelgan bir nechta 401 BITTA refresh'ni kutadi (refresh token
  // bir martalik: ikki parallel refresh ikkinchisini "o'g'irlik" deb
  // sessiyani yopib yuborardi).
  // └─────────────────────────────────────────────────────────────────────┘

  /// Refresh token; `null` — eski (HS256) sessiya yoki kirilmagan.
  String? refreshToken;

  TokenStore? _store;
  Future<_RefreshResult>? _refreshing;

  /// Sessiya saqlanadigan joy (ilova ishga tushganda bir marta).
  void attachStore(TokenStore store) => _store = store;

  /// Markaziy (yangi) sessiyami.
  bool get isCentralSession => refreshToken != null;

  /// Mijoz ilovasi (137): eski kirish endpointlari (`/auth/verify`,
  /// `/auth/login`, Google, Firebase, email, Telegram) `X-Ondex-Session: v2`
  /// sarlavhasi bilan chaqiriladi — server eski 30 kunlik token o'rniga
  /// markaziy sessiya (access + refresh) beradi. Kirish ekranlari o'zgarmaydi.
  bool wantCentralSession = false;

  /// Kirish javobini qabul qiladi: markaziy sessiya (`refresh_token` bor)
  /// yoki eski token. Sessiya omborga (`attachStore`) yoziladi.
  Future<void> adoptLogin(Map d) async {
    final rt = d['refresh_token'];
    if (rt is String && rt.isNotEmpty) {
      token = (d['access_token'] ?? d['token']) as String;
      refreshToken = rt;
    } else {
      token = d['token'] as String;
      refreshToken = null;
    }
    await _persist();
  }

  /// Foydalanuvchining faol sessiyalari (qurilmalar) — markaziy sessiyada.
  /// Har biri: id, client_id, device_name, created_at, last_used_at, current.
  Future<List<Map<String, dynamic>>> mySessions() async {
    final d = await send('GET', '/v2/auth/sessions');
    return [
      for (final s in ((d as Map)['sessions'] as List? ?? const []))
        Map<String, dynamic>.from(s as Map),
    ];
  }

  /// Boshqa qurilmani chiqarish (o'sha qurilmaning tokeni darhol o'ladi).
  Future<void> revokeSession(String id) =>
      send('DELETE', '/v2/auth/sessions/${Uri.encodeComponent(id)}');

  /// Joriy sessiyani omborga qayta yozadi (ilova kodi tokenni o'zi
  /// yozmasin: refresh token yo'qolib qolardi).
  Future<void> saveSession() => _persist();

  /// Saqlangan sessiyani tiklaydi. Eski formatdagi (bitta token satri)
  /// sessiya ham o'qiladi — foydalanuvchi yangilanishdan keyin chiqib ketmaydi.
  Future<bool> restoreSession() async {
    final raw = await _store?.read();
    if (raw == null || raw.isEmpty) return false;
    if (raw.startsWith('{')) {
      try {
        final m = jsonDecode(raw) as Map<String, dynamic>;
        token = m['a'] as String?;
        refreshToken = m['r'] as String?;
      } catch (_) {
        await clearSession();
        return false;
      }
    } else {
      token = raw;
      refreshToken = null;
    }
    return token != null;
  }

  Future<void> _persist() async {
    final s = _store;
    if (s == null) return;
    if (token == null) {
      await s.clear();
    } else {
      await s.write(
        refreshToken == null
            ? token!
            : jsonEncode({'a': token, 'r': refreshToken}),
      );
    }
  }

  /// Sessiyani shu qurilmadan o'chiradi (server chaqirilmaydi).
  Future<void> clearSession() async {
    token = null;
    refreshToken = null;
    await _store?.clear();
  }

  /// Merchant xodimi kirishi (OnDexPro / OnDexGO): telefon kodi -> markaziy
  /// sessiya. [clientId] — `ondex-pro` yoki `ondex-go`.
  ///
  /// Biznes EGASI uchun server ikki bosqichli tasdiq (TOTP) talab qiladi —
  /// mobil xodim ilovalarida bu ekran yo'q, shuning uchun aniq xabar beriladi.
  Future<Map<String, dynamic>> merchantVerify(
    String phone,
    String code, {
    required String clientId,
    String? deviceName,
  }) async {
    final step = await merchantLoginCode(
      phone,
      code,
      clientId: clientId,
      deviceName: deviceName,
    );
    if (!step.done) {
      throw ApiException(
        'Bu raqam biznes egasiniki — kirish ikki bosqichli tasdiq bilan.\n'
        'Merchant panel orqali kiring.',
        statusCode: 403,
      );
    }
    return step.data;
  }

  // ---------- Biznes egasi kirishi (A4b): bosqichma-bosqich ----------
  //
  // Server oqimi (`internal/httpapi/routes_merchant_auth.go`):
  //   parol | kod  ->  [otp | password]  ->  [totp_enroll -> confirm]  ->  totp  ->  sessiya
  // Har chaqiruv `MerchantStep` qaytaradi: `done` bo'lsa sessiya o'rnatilgan
  // (token + refresh omborda), aks holda `method` keyingi bosqichni bildiradi.

  /// Telefon + parol (faqat egasi). Xodim/mijoz — 401 (bir xil xato).
  Future<MerchantStep> merchantLoginPassword(
    String phone,
    String password, {
    required String clientId,
    String? deviceName,
  }) => _merchantStep('/v2/auth/merchant/password', {
    'phone': phone,
    'password': password,
    'client_id': clientId,
    if (deviceName != null) 'device_name': deviceName,
  });

  /// Telefon + kod (`telegramStart` / `requestCode` dan keyin).
  Future<MerchantStep> merchantLoginCode(
    String phone,
    String code, {
    required String clientId,
    String? deviceName,
  }) => _merchantStep('/v2/auth/merchant/otp/verify', {
    'phone': phone,
    'code': code,
    'client_id': clientId,
    if (deviceName != null) 'device_name': deviceName,
  });

  /// OnDex egasi (admin, platform realmi — A6): telefon + kod. Keyin HAR
  /// safar parol va TOTP (`mfaFactor` / `mfaSetPassword` / `mfaVerify`).
  Future<MerchantStep> adminLoginCode(
    String phone,
    String code, {
    String? deviceName,
  }) => _merchantStep('/v2/auth/admin/otp/verify', {
    'phone': phone,
    'code': code,
    if (deviceName != null) 'device_name': deviceName,
  });

  /// `method == otp` (telefon kodi) yoki `password` (parol) bosqichi.
  Future<MerchantStep> mfaFactor(
    String mfaToken, {
    String? code,
    String? password,
  }) => _merchantStep('/v2/auth/mfa/factor', {
    'mfa_token': mfaToken,
    if (code != null) 'code': code,
    if (password != null) 'password': password,
  });

  /// `method == password_set`: parolsiz egasi kod bilan kirgach yangi parol
  /// o'rnatadi (server zaif parolni 400 bilan rad etadi).
  Future<MerchantStep> mfaSetPassword(
    String mfaToken,
    String password,
    String confirm,
  ) => _merchantStep('/v2/auth/mfa/password', {
    'mfa_token': mfaToken,
    'password': password,
    'password_confirm': confirm,
  });

  /// `method == totp`: autentifikator kodi YOKI tiklash kodi.
  Future<MerchantStep> mfaVerify(String mfaToken, String code) => _merchantStep(
    '/v2/auth/mfa/verify',
    {'mfa_token': mfaToken, 'code': code},
  );

  /// `method == totp_enroll`: yangi kalit (QR uchun `otpauth://` havola).
  Future<TotpEnrollment> mfaEnroll(String mfaToken) async {
    final d = await send('POST', '/v2/auth/mfa/totp/enroll', {
      'mfa_token': mfaToken,
    });
    return TotpEnrollment(
      secret: d['secret'] as String,
      uri: d['otpauth_uri'] as String,
    );
  }

  /// Ulashni yakunlaydi: sessiya o'rnatiladi, tiklash kodlari BIR MARTA qaytadi.
  Future<List<String>> mfaConfirm(String mfaToken, String code) async {
    final d = await send('POST', '/v2/auth/mfa/totp/confirm', {
      'mfa_token': mfaToken,
      'code': code,
    });
    await _setCentral(Map<String, dynamic>.from(d['tokens'] as Map));
    return List<String>.from(d['recovery_codes'] as List);
  }

  Future<MerchantStep> _merchantStep(
    String path,
    Map<String, dynamic> body,
  ) async {
    final d = Map<String, dynamic>.from(await send('POST', path, body) as Map);
    if (d['mfa_required'] == true) {
      return MerchantStep._challenge(
        d['mfa_token'] as String,
        d['method'] as String,
        d,
      );
    }
    await _setCentral(d);
    return MerchantStep._done(d);
  }

  Future<void> _setCentral(Map<String, dynamic> d) async {
    token = d['access_token'] as String;
    refreshToken = d['refresh_token'] as String?;
    await _persist();
  }

  Future<_RefreshResult> _refresh() {
    return _refreshing ??= _doRefresh().whenComplete(() => _refreshing = null);
  }

  /// Boshqa OnDex servisiga (masalan OnDexMap admin serveri) yuboriladigan
  /// access token — muddati 60 soniyadan kam qolgan bo'lsa AVVAL yangilanadi
  /// (o'sha servis OnDex'ni chaqirib tekshiradi, eskirgan token 401 bo'lardi).
  /// Sessiya yo'q yoki markazda yopilgan — `null`.
  Future<String?> freshAccessToken() async {
    final t = token;
    if (t == null) return null;
    if (refreshToken == null || !_expiresSoon(t)) return t;
    switch (await _refresh()) {
      case _RefreshResult.ok:
        return token;
      case _RefreshResult.network:
        return t; // tarmoq — eski token bilan urinib ko'radi
      case _RefreshResult.rejected:
        return null;
    }
  }

  static bool _expiresSoon(String jwt) {
    try {
      final parts = jwt.split('.');
      if (parts.length != 3) return false;
      final payload =
          jsonDecode(
                utf8.decode(base64Url.decode(base64Url.normalize(parts[1]))),
              )
              as Map<String, dynamic>;
      final exp = payload['exp'];
      if (exp is! num) return false;
      final at = DateTime.fromMillisecondsSinceEpoch(exp.toInt() * 1000);
      return at.difference(DateTime.now()) < const Duration(seconds: 60);
    } catch (_) {
      return false;
    }
  }

  Future<_RefreshResult> _doRefresh() async {
    final rt = refreshToken;
    if (rt == null) return _RefreshResult.rejected;
    http.Response r;
    try {
      r = await http
          .post(
            Uri.parse('$baseUrl/v2/auth/token/refresh'),
            headers: {
              'Content-Type': 'application/json',
              'X-Ondex-Client': clientHeaderValue,
            },
            body: jsonEncode({'refresh_token': rt}),
          )
          .timeout(timeout);
    } catch (_) {
      return _RefreshResult.network;
    }
    if (r.statusCode == 200) {
      try {
        final d = jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
        token = d['access_token'] as String;
        refreshToken = d['refresh_token'] as String;
        await _persist();
        return _RefreshResult.ok;
      } catch (_) {
        return _RefreshResult.network;
      }
    }
    if (r.statusCode == 401) {
      // Markazda yopilgan — qayta urinish behuda. `token` ATAYLAB qoldiriladi:
      // ilovalarning `onUnauthorized` handleri "token null = kirish
      // jarayonidagi xato" deb hech narsa qilmaydi; tokenni o'sha handler
      // (`clearSession`) tozalaydi va login ekraniga o'tkazadi.
      refreshToken = null;
      await _store?.clear();
      return _RefreshResult.rejected;
    }
    return _RefreshResult.network; // 5xx / 429 — vaqtincha, sessiya qoladi
  }

  Map<String, String> get _headers => {
    'Content-Type': 'application/json',
    if (token != null) 'Authorization': 'Bearer $token',
    // Qaysi ilova va qaysi versiya — superadmin panelidagi
    // "Qurilma" ustuni uchun (`config.dart`). Faqat ma'lumot:
    // server hech qanday huquqni bu qiymatga bog'lamaydi.
    'X-Ondex-Client': clientHeaderValue,
    if (wantCentralSession) 'X-Ondex-Session': 'v2',
  };

  Future<dynamic> send(
    String method,
    String path, [
    Map<String, dynamic>? body,
  ]) async {
    final used = token;
    var r = await _raw(method, path, body);
    if (r.statusCode == 401 &&
        refreshToken != null &&
        !_noRefresh.contains(path)) {
      if (token != null && token != used) {
        // Shu orada boshqa so'rov allaqachon yangilagan — refresh'siz qayta.
        r = await _raw(method, path, body);
      } else {
        switch (await _refresh()) {
          case _RefreshResult.ok:
            r = await _raw(method, path, body);
          case _RefreshResult.network:
            throw ApiException(
              'Serverga ulanib bo\'lmadi — internetni tekshiring',
            );
          case _RefreshResult.rejected:
            break; // pastda _parse 401 ni ko'rib onUnauthorized'ni chaqiradi
        }
      }
    }
    // Kirish bosqichidagi 401 — noto'g'ri parol/kod, sessiya tugashi EMAS.
    return _parse(r, notifyUnauthorized: !_noRefresh.contains(path));
  }

  Future<http.Response> _raw(
    String method,
    String path,
    Map<String, dynamic>? body,
  ) async {
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
        case 'PUT':
          // To'liq almashtirish (masalan `PUT /admin/support/contacts`).
          // Avval bu shox yo'q edi va PUT jimgina POST bo'lib ketardi
          // (server 405 qaytarardi).
          r = await http
              .put(uri, headers: _headers, body: jsonEncode(body ?? {}))
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
    return r;
  }

  dynamic _parse(http.Response r, {bool notifyUnauthorized = true}) {
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
      if (r.statusCode == 401 && notifyUnauthorized) onUnauthorized?.call();
      throw ApiException(
        msg,
        statusCode: r.statusCode,
        data: data is Map ? Map<String, dynamic>.from(data) : null,
      );
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
    final d = await send('POST', '/auth/verify', {
      'phone': phone,
      'code': code,
    });
    await adoptLogin(d as Map);
    return Map<String, dynamic>.from(d);
  }

  /// Telefon + parol bilan kirish. Javob: {token, user} (tokenni o'rnatadi).
  ///
  /// Parolni superadmin hamkor akkaunti (restoran, do'kon) yaratganda beradi,
  /// egasi `/me/password` bilan keyin almashtirishi mumkin. Noto'g'ri raqam
  /// ham, noto'g'ri parol ham BIR XIL xato beradi (raqam mavjudligi sezilmaydi).
  Future<Map<String, dynamic>> loginWithPassword(
    String login,
    String password,
  ) async {
    final d = await send('POST', '/auth/login', {
      'login': login,
      'password': password,
    });
    await adoptLogin(d as Map);
    return Map<String, dynamic>.from(d);
  }

  /// Login qilgan foydalanuvchining ma'lumotlari (rol shu yerdan aniqlanadi).
  ///
  /// ┌─ TAHLIL SHU YERGA ULANGAN ─────────────────────────────────────┐
  /// `identify()` ni har bir ilovaning har bir kirish yo'liga qo'shish
  /// mumkin edi: parol bilan kirish, SMS kod, Telegram, PIN tiklash,
  /// parolni unutish — mijoz ilovasining o'zida beshta joy. Bittasi
  /// unutilsa, o'sha yo'ldan kirgan foydalanuvchi ANONIM bo'lib
  /// qolardi va buni sezish deyarli imkonsiz.
  ///
  /// `me()` esa yagona nuqta: rolni aniqlash uchun uni HAMMA ilova
  /// kirgandan keyin chaqiradi. Shuning uchun bog'lanish shu yerda.
  ///
  /// Tahlil o'chirilgan bo'lsa (`ONDEX_POSTHOG_KEY` bo'sh) hech narsa
  /// bajarilmaydi.
  /// └────────────────────────────────────────────────────────────────┘
  Future<Map<String, dynamic>> me() async {
    final user = Map<String, dynamic>.from(await send('GET', '/me'));
    final id = (user['id'] as String?) ?? '';
    if (id.isNotEmpty) {
      await Analytics.instance.identify(
        userId: id,
        phone: user['phone'] as String?,
        name: user['name'] as String?,
        role: user['role'] as String?,
      );
    }
    return user;
  }

  /// Serverda sessiyani bekor qiladi — shu foydalanuvchining BARCHA
  /// tokenlari darhol yaroqsiz bo'ladi.
  ///
  /// Xato yutiladi: tarmoq yo'q bo'lsa ham foydalanuvchi ilovadan chiqa
  /// olishi kerak. Bu — qo'shimcha himoya, chiqishning sharti emas.
  Future<void> logout() async {
    try {
      // Markaziy sessiya — markazda yopiladi (boshqa qurilmalar tegilmaydi);
      // eski sessiya — eski yo'l.
      await send('POST', isCentralSession ? '/v2/auth/logout' : '/auth/logout');
    } catch (_) {}
    refreshToken = null;
    // ┌─ TAHLILNI HAM UZAMIZ ──────────────────────────────────────┐
    // Busiz keyingi foydalanuvchining harakatlari CHIQIB KETGAN
    // odamning yo'liga yozilardi — bitta qurilmani ikki kishi
    // ishlatganda (masalan restoran xodimlari almashganda) tahlil
    // umuman ishonchsiz bo'lib qolardi.
    //
    // YANGI anonim ID beriladi (eskisi allaqachon akkauntga
    // birlashtirilgan — `resetAnonymousId` izohiga qarang).
    try {
      await Analytics.instance.reset(await Analytics.resetAnonymousId());
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
  ///
  /// `GOOGLE_MAP=false` rejimida serverda kalit bo'lmasligi mumkin — bu
  /// holda Google yo'li ishlamaydi va chaqiruvchi xatoni ushlaydi.
  Future<String> mapsApiKey() async {
    final d = await send('GET', '/config/maps');
    final key = d is Map ? d['maps_api_key'] : null;
    if (key is! String || key.isEmpty) {
      throw ApiException('Google xaritasi sozlanmagan');
    }
    return key;
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
      ..files.add(
        http.MultipartFile.fromBytes(field, bytes, filename: filename),
      );
    if (token != null) req.headers['Authorization'] = 'Bearer $token';

    http.Response resp;
    try {
      final streamed = await req.send().timeout(uploadTimeout ?? timeout * 6);
      resp = await http.Response.fromStream(streamed);
    } catch (_) {
      throw ApiException(errorText);
    }
    return _parse(resp);
  }
}

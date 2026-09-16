import 'dart:math';

import 'package:ondex_core/ondex_core.dart';

import 'delivery_tracking.dart';

// Umumiy yadro qayta eksport qilinadi — mavjud ekranlar `api.dart` ni
// import qilgani uchun ular tegilmasdan ishlashda davom etadi.
export 'package:ondex_core/ondex_core.dart';

/// Xatoni foydalanuvchiga ko'rsatiladigan matnga aylantiradi.
///
/// Server yuborgan xabar ma'noli (masalan "restoran yopiq"), shuning
/// uchun u ustun turadi. Boshqa har qanday xato — tarmoq uzilishi,
/// JSON buzilishi, kutilmagan holat — texnik tafsilotsiz, chaqiruvchi
/// bergan umumiy matn bilan ko'rsatiladi.
///
/// Ilgari bu shart har ekranda qo'lda yozilardi
/// (`e is ApiException ? e.message : '...'`) va matnlar bir-biridan
/// farq qila boshlagandi.
String errorText(Object e, String fallback) =>
    e is ApiException ? e.message : fallback;

/// Buyurtma yaratishda ishlatiladigan bir martalik tasodifiy kalit
/// (idempotency key) — tarmoq uzilib, javob kelmay qolgan holatda
/// foydalanuvchi qayta bossa, server SHU KALITNI oldin ko'rgan-ko'rmaganini
/// tekshirib, ikkinchi (dublikat) buyurtma yaratmaydi. Chaqiruvchi buni
/// BIR MARTA generatsiya qilib, muvaffaqiyatli buyurtmagacha bo'lgan
/// BARCHA qayta urinishlarda AYNAN SHU qiymatni qayta ishlatishi kerak.
String newIdempotencyKey() {
  final rnd = Random.secure();
  final bytes = List<int>.generate(16, (_) => rnd.nextInt(256));
  return bytes.map((b) => b.toRadixString(16).padLeft(2, '0')).join();
}

/// Backend manzili — endi `--dart-define=ONDEX_API_URL=...` orqali
/// beriladi (qarang: ondex_core/config.dart). Avval bu yerda LAN IP
/// QATTIQ yozilgan edi va tarmoq o'zgarganda to'rtta faylni qo'lda
/// tahrirlash kerak bo'lardi.
const baseUrl = apiBaseUrl;

/// Mijoz ilovasiga XOS endpointlar. Umumiy qismi (`send`, xato
/// ishlash, timeout, 401, `requestCode`/`verify`/`me`/`logout`/
/// `wsTicket`/`mapsApiKey`) `ondex_core.ApiClient` da — u yerda BIR
/// marta yozilgan va to'rtala ilovaga birdan yetadi.
class CustomerApi extends ApiClient {
  CustomerApi() : super(baseUrl: apiBaseUrl);
  Future<List<dynamic>> restaurants() async =>
      (await send('GET', '/restaurants')) as List<dynamic>? ?? [];

  /// Bitta restoran. Menyu ekrani holatni ("Ochiq/Yopiq", ish vaqti) shu
  /// bilan YANGI oladi — katalog keshi eskirgan bo'lishi mumkin.
  Future<Map<String, dynamic>> restaurant(String id) async => Map<String, dynamic>.from(
      await send('GET', '/restaurants/${Uri.encodeComponent(id)}') as Map);

  /// Barcha restoranlar uchun umumiy, standart taom turkumlari (backend'da
  /// bitta joyda saqlanadi — restoran paneli ham xuddi shu ro'yxatdan
  /// foydalanadi). Restoranlar sahifasidagi turkum qatori HAR DOIM shu
  /// to'liq ro'yxatni ko'rsatadi — biror restoranda hozircha o'sha
  /// turkumdan taom bo'lmasa ham (bosilganda "taom topilmadi" chiqadi).
  Future<List<dynamic>> categories() async =>
      (await send('GET', '/categories')) as List<dynamic>? ?? [];

  Future<List<dynamic>> menu(String restaurantId) async =>
      (await send('GET', '/restaurants/$restaurantId/menu'))
          as List<dynamic>? ??
      [];

  /// Shu restoranning HOZIR faol aksiyalari — menyuda chegirma
  /// belgisi ko'rsatish uchun (taxminiy, faqat vizual). Haqiqiy chegirma
  /// har doim quote()/createOrder() orqali serverda hisoblanadi.
  Future<List<dynamic>> activePromotions(String restaurantId) async =>
      (await send('GET', '/restaurants/$restaurantId/active-promotions'))
          as List<dynamic>? ??
      [];

  /// Savat holatini checkout'dan OLDIN ko'rish uchun HAQIQIY narxlash —
  /// real buyurtma yaratmaydi. createOrder() bilan bir xil backend
  /// mantig'idan foydalanadi, shuning uchun bu yerda ko'rsatilgan
  /// summa buyurtma yaratilganda yozilgan summa bilan har doim mos keladi.
  Future<Map<String, dynamic>> quote(
          String restaurantId, List<Map<String, dynamic>> items) async =>
      Map<String, dynamic>.from(await send(
          'POST', '/restaurants/$restaurantId/quote', {'items': items}));

  /// Turkum (yoki nom) bo'yicha BARCHA restoranlardagi mos taomlarni
  /// qidiradi — restoranga bog'liq emas. Har bir natijada qaysi
  /// restorandan ekanligi ham keladi (restaurant_id/name/logo_url/open).
  Future<List<dynamic>> searchProducts(String query) async =>
      (await send(
              'GET', '/products/search?q=${Uri.encodeQueryComponent(query)}'))
          as List<dynamic>? ??
      [];

  /// "Istaklarim" — mijoz yurak belgisi bilan saqlagan mahsulotlar,
  /// searchProducts() bilan bir xil to'liq shaklda (mahsulot + qaysi
  /// restorandan ekanligi) — eng yangi qo'shilgani birinchi. FAQAT
  /// "Istaklarim" sahifasining o'zi uchun (to'liq kartochka chizish kerak
  /// bo'lganda) — boshqa joyda faqat yurak belgisi holatini bilish uchun
  /// favoriteIds() (yengilroq) ishlatiladi.
  Future<List<dynamic>> favorites() async =>
      (await send('GET', '/favorites')) as List<dynamic>? ?? [];

  /// Faqat ID'lar — mahsulot/restoran ma'lumotisiz (favorites()dan farqli,
  /// hech qanday katalog boyitish qilinmaydi). Menyu/turkum ekranlarida
  /// kartochkalardagi yurak belgisining boshlang'ich holatini bilish
  /// uchun shu YETARLI — to'liq ro'yxatni so'rash har safar keraksiz
  /// server ishiga (mahsulot+restoran qidiruviga) olib kelardi.
  Future<Set<String>> favoriteIds() async {
    final list = await send('GET', '/favorites/ids') as List<dynamic>? ?? [];
    return list.cast<String>().toSet();
  }

  Future<void> addFavorite(String productId) =>
      send('POST', '/favorites/$productId');

  Future<void> removeFavorite(String productId) =>
      send('DELETE', '/favorites/$productId');

  // ── Bildirishnomalar ────────────────────────────────────────────────
  //
  // ┌─ NEGA JONLI XABARDAN TASHQARI TARIX KERAK ──────────────────────┐
  // WebSocket xabari FAQAT ilova ochiq bo'lganda yetadi. Ilova yopiq,
  // tarmoq uzilgan yoki soket o'lik bo'lsa xabar yo'qolardi. Server
  // ularni bazaga yozadi (`internal/httpapi/routes_notifications.go`)
  // va shu endpointlar o'sha tarixni beradi.
  // └──────────────────────────────────────────────────────────────────┘

  /// Eng yangilaridan. Kirmagan foydalanuvchida 401 — bu NORMAL holat.
  Future<List<dynamic>> notifications({int limit = 50}) async =>
      (await send('GET', '/notifications?limit=$limit')) as List<dynamic>? ??
      [];

  /// Qo'ng'iroq belgisidagi raqam.
  Future<int> unreadNotificationCount() async {
    final res = await send('GET', '/notifications/unread-count');
    if (res is Map) return (res['count'] as num?)?.toInt() ?? 0;
    return 0;
  }

  Future<void> markAllNotificationsRead() =>
      send('POST', '/notifications/read-all');

  /// items: [{product_id, qty}] — narx yuborilmaydi, server katalogdan
  /// hisoblaydi. idempotencyKey — newIdempotencyKey() bilan BIR MARTA
  /// generatsiya qilinib, muvaffaqiyatli javob kelguncha bo'lgan BARCHA
  /// qayta urinishlarda o'zgarmasdan qayta ishlatilishi kerak (tarmoq
  /// uzilib qayta bosilganda dublikat buyurtma yaratilmasligi uchun).
  /// ┌─ IKKI TUR — BITTA ENDPOINT ────────────────────────────────────┐
  /// Server buyurtma turini `table_token` bor-yo'qligiga qarab
  /// aniqlaydi (`internal/httpapi/routes_orders.go`):
  ///   * token BOR   -> `dine_in`, manzil TALAB QILINMAYDI;
  ///   * token YO'Q  -> yetkazib berish, manzil MAJBURIY.
  ///
  /// Shuning uchun bu yerda ham bitta metod: ikkita alohida metod
  /// yozilsa, ular vaqt o'tib bir-biridan uzoqlashardi (masalan
  /// idempotentlik faqat bittasiga qo'shilib qolardi).
  /// └─────────────────────────────────────────────────────────────────┘
  ///
  /// NARX YUBORILMAYDI — server katalogdan o'zi hisoblaydi. Klient
  /// yuborgan summaga ishonish to'g'ridan-to'g'ri firibgarlik yo'li
  /// bo'lardi.
  Future<Map<String, dynamic>> createOrder({
    required List<Map<String, dynamic>> items,
    required String idempotencyKey,
    double? lat,
    double? lng,
    String? tableToken,
    int? partySize,
    /// `cash` yoki `card`. Kartada buyurtma TO'LOV KUTIB turadi va
    /// restoranga ko'rinmaydi — mijoz `startPayment()` orqali to'lov
    /// havolasini olishi kerak.
    String paymentMethod = 'cash',

    /// Ekranda mijozga KO'RSATILGAN jami (tiyin).
    ///
    /// Berilsa, server hisobi bilan solishtiriladi va farq bo'lsa
    /// buyurtma yaratilmaydi — `ApiException(statusCode: 409)` keladi.
    /// Ya'ni mijoz ko'rgan raqamdan boshqa summaga hech qachon rozi
    /// bo'lib qolmaydi (aksiya tugashi, narx tahriri).
    int? expectedTotalTiyin,
  }) async {
    final body = <String, dynamic>{
      'items': items,
      'idempotency_key': idempotencyKey,
      'payment_method': paymentMethod,
    };
    if (expectedTotalTiyin != null && expectedTotalTiyin > 0) {
      body['expected_total_tiyin'] = expectedTotalTiyin;
    }
    if (tableToken != null && tableToken.isNotEmpty) {
      body['table_token'] = tableToken;
      if (partySize != null && partySize > 0) body['party_size'] = partySize;
    } else {
      // MANZIL bu yerdan OLINMAYDI — server uni `/me/address` dan
      // o'qiydi (routes_orders.go). Bu maydonlar eski klientlar bilan
      // moslik uchun qoldirilgan va serverga TA'SIR QILMAYDI:
      // koordinatani so'rov tanasidan qabul qilish xizmat hududi
      // tekshiruvini chetlab o'tish yo'li edi.
      body['delivery_lat'] = lat;
      body['delivery_lng'] = lng;
    }
    return Map<String, dynamic>.from(await send('POST', '/orders', body));
  }

  /// Buyurtma kuzatuvi: yetkazish yo'li (A→B), kuryer va qolgan vaqt.
  /// Kuzatuv yo'q bo'lsa (masalan stol buyurtmasi) `null`.
  ///
  /// [withPlanned] `false` — A→B yo'li (yuzlab nuqta) javobga qo'shilmaydi:
  /// u o'zgarmaydi va ekranda allaqachon bor (mobil trafik tejaladi).
  Future<DeliveryTracking?> orderTracking(String orderId, {bool withPlanned = true}) async {
    try {
      final d = await send('GET', '/orders/$orderId/tracking${withPlanned ? '' : '?planned=0'}');
      return d is Map<String, dynamic> ? DeliveryTracking.fromJson(d) : null;
    } on ApiException catch (e) {
      if (e.statusCode == 404) return null;
      rethrow;
    }
  }

  Future<Map<String, dynamic>> getOrder(String id) async =>
      Map<String, dynamic>.from(await send('GET', '/orders/$id'));

  /// Karta orqali to'lovni boshlaydi va to'lov sahifasi havolasini
  /// qaytaradi (`pay_url`).
  ///
  /// SUMMA YUBORILMAYDI — server uni buyurtmadan oladi. Takroriy
  /// chaqiruv xavfsiz: hali amal qilayotgan urinish bo'lsa, o'sha
  /// havola qaytadi (provayder bir xil tranzaksiya ID'sini qayta
  /// ishlatishga ruxsat bermaydi).
  ///
  /// [retry] — mijoz "qayta urinish" bosganda YANGI tranzaksiya
  /// ochtiradi. Bu kerak, chunki urinish bank tomonda o'lgan bo'lishi
  /// mumkin (OTP kodini xato kiritish yoki SMS ni ko'p marta so'rash),
  /// va o'sha havolani qayta ochish yordam bermaydi.
  Future<Map<String, dynamic>> startPayment(String orderId,
          {bool retry = false}) async =>
      Map<String, dynamic>.from(
          await send('POST', '/orders/$orderId/pay${retry ? '?retry=1' : ''}'));

  /// To'lov holati — mijoz to'lov sahifasidan qaytgach shu yerdan
  /// kuzatiladi (`payment_state`: awaiting/held/paid/failed).
  Future<Map<String, dynamic>> paymentStatus(String orderId) async =>
      Map<String, dynamic>.from(await send('GET', '/orders/$orderId/payment'));

  /// Stol QR kodidagi tokendan qaysi restoran va qaysi stol ekanini
  /// aniqlaydi (`qr_scan_screen.dart`).
  ///
  /// Javobda TOKEN QAYTMAYDI — chaqiruvchi uni allaqachon biladi,
  /// qaytarish esa uni keraksiz joylarga (loglar, kesh) yoyardi.
  ///
  /// Endpoint autentifikatsiya talab qiladi: usiz istalgan odam
  /// tokenlarni birma-bir sinab ko'ra olardi.
  Future<Map<String, dynamic>> resolveTable(String token) async =>
      Map<String, dynamic>.from(await send(
          'GET', '/tables/resolve?token=${Uri.encodeQueryComponent(token)}'));

  // `me()`, `logout()`, `mapsApiKey()`, `wsTicket()`, `requestCode()`,
  // `verify()` — hammasi `ondex_core.ApiClient` da (meros orqali
  // mavjud). Bu yerda TAKRORLANMAYDI.

  /// Mijozning o'z buyurtmalari tarixi — har birida restoran nomi/logotipi
  /// ham keladi ("Buyurtmalarim" bo'limi).
  Future<List<dynamic>> myOrders() async =>
      (await send('GET', '/me/orders')) as List<dynamic>? ?? [];

  /// Ro'yxatdan o'tish. TOKEN QAYTARMAYDI — akkaunt tasdiqlanmagan
  /// holatda yaratiladi va SMS kod yuboriladi. Token faqat
  /// `verify()` dan keyin olinadi (register_screen.dart izohiga qarang).
  /// Dev rejimda server SMS kodni javobda qaytaradi — OTP kataklarini
  /// avtomatik to'ldirish uchun. Production'da har doim `null`.
  Future<String?> register({
    required String phone,
    required String firstName,
    required String lastName,
    required String password,
    required String passwordConfirm,
    String email = '',
  }) async {
    final d = await send('POST', '/auth/register', {
      'phone': phone,
      'email': email,
      'first_name': firstName,
      'last_name': lastName,
      'password': password,
      'password_confirm': passwordConfirm,
    });
    return (d is Map) ? d['dev_code'] as String? : null;
  }

  /// Parol bilan kirish — telefon YOKI email.
  Future<Map<String, dynamic>> loginWithPassword(
      String login, String password) async {
    final d = await send('POST', '/auth/login', {
      'login': login,
      'password': password,
    });
    token = d['token'] as String;
    return Map<String, dynamic>.from(d);
  }

  /// Google hisobi bilan kirish / ro'yxatdan o'tish.
  ///
  /// `firebaseIdToken` — Google hisobi tanlangandan keyin Firebase
  /// bergan ID token. Email SO'ROVGA QO'SHILMAYDI: server uni
  /// tokenning imzosini tekshirib, token ICHIDAN oladi va ustiga
  /// `sign_in_provider == google.com` hamda `email_verified == true`
  /// ekanini talab qiladi.
  Future<Map<String, dynamic>> loginWithGoogle(String firebaseIdToken) async {
    final d = await send('POST', '/auth/google', {'id_token': firebaseIdToken});
    token = d['token'] as String;
    return Map<String, dynamic>.from(d);
  }

  /// "Telegram bilan kirish" — deep link va kuzatish tokenini beradi.
  ///
  /// Kod oqimidan FARQI: bu yerda telefon raqami yuborilmaydi —
  /// kimlikni Telegram belgilaydi (`internal/telegram` izohiga qarang).
  Future<({String deepLink, String token})> telegramLoginStart() async {
    final d = await send('POST', '/auth/telegram/login/start');
    return (deepLink: d['deep_link'] as String, token: d['token'] as String);
  }

  /// Kirish tasdiqlanganini tekshiradi va yakunlaydi.
  ///
  /// `confirmSecret` — bot yuborgan "OnDex'ga qaytish" havolasidagi
  /// (`ondex://auth?c=...`) maxfiy kalit. U MAJBURIY: kalitsiz server
  /// har doim "hali tayyor emas" deb javob beradi.
  ///
  /// NEGA: kuzatish tokenining o'zi yetarli bo'lganda, deep linkni
  /// qurbonga yuborgan hujumchi uning akkauntiga kirib olardi. Kalit
  /// esa tasdiqlagan odamning QURILMASIGA tushadi — hujumchining
  /// qurilmasi uni hech qachon ko'rmaydi (backend
  /// `telegram.Pending.ConfirmSecret` izohiga qarang).
  ///
  /// `null` — hali tasdiqlanmagan (foydalanuvchi Telegramda).
  Future<Map<String, dynamic>?> telegramLoginStatus(
      String pollToken, String confirmSecret) async {
    final d = await send(
        'GET',
        '/auth/telegram/login/status'
        '?token=${Uri.encodeQueryComponent(pollToken)}'
        '&c=${Uri.encodeQueryComponent(confirmSecret)}');
    if (d is Map && d['pending'] == true) return null;
    token = d['token'] as String;
    return Map<String, dynamic>.from(d);
  }

  // `telegramStart()` endi `ondex_core.ApiClient` da — affitsiant
  // ilovasi ham shu oqimga o'tgach nusxa ikkitaga chiqdi.

  /// Firebase Phone Auth bilan kirish.
  ///
  /// `idToken` — Firebase SDK bergan ID token. Telefon raqami
  /// SO'ROVGA QO'SHILMAYDI: server uni tokenning IMZOSINI tekshirib,
  /// token ICHIDAN oladi. Aks holda istalgan odam istalgan raqamni
  /// yozib yuborardi.
  Future<Map<String, dynamic>> loginWithFirebase(String idToken) async {
    final d = await send('POST', '/auth/firebase', {'id_token': idToken});
    token = d['token'] as String;
    return Map<String, dynamic>.from(d);
  }

  /// Emailga 6 xonali tasdiqlash kodi so'raydi.
  ///
  /// Dev rejimda server kodni javobda qaytaradi (SMTP ulanmagan
  /// bo'lsa ham sinash mumkin). Production'da har doim `null`.
  Future<String?> requestEmailCode(String email) async {
    final d = await send('POST', '/auth/email/request-code', {'email': email});
    return (d is Map) ? d['dev_code'] as String? : null;
  }

  /// Email kodini tasdiqlaydi va tokenni o'rnatadi.
  ///
  /// Telefon oqimidagi `verify()` bilan bir xil: qaytgan token
  /// "kod bilan tasdiqlangan" deb belgilanadi, ya'ni keyingi 15
  /// daqiqada `setPassword()` joriy parolni so'ramaydi.
  Future<Map<String, dynamic>> verifyEmail(String email, String code) async {
    final d = await send(
        'POST', '/auth/email/verify', {'email': email, 'code': code});
    token = d['token'] as String;
    return Map<String, dynamic>.from(d);
  }

  /// Parol o'rnatish yoki o'zgartirish.
  ///
  /// Akkauntda parol ALLAQACHON bo'lsa `current` majburiy (server
  /// tekshiradi). Ro'yxatdan o'tish oqimida esa parol hali yo'q —
  /// u SMS tasdig'idan KEYIN, aynan shu chaqiruv bilan o'rnatiladi.
  /// Sabab: parolni faqat amaldagi token egasi (ya'ni raqamni
  /// tasdiqlagan odam) qo'ya olishi kerak — aks holda begona odam
  /// ro'yxatdan o'tish orqali sizning akkauntingizga o'z parolini
  /// qo'yib qo'yardi.
  ///
  /// Server barcha eski sessiyalarni bekor qiladi va javobda YANGI
  /// token beradi — uni darhol o'rnatamiz, aks holda foydalanuvchi
  /// parolini o'zgartirgani uchun tizimdan chiqib ketardi.
  Future<void> setPassword({
    String current = '',
    required String password,
    required String passwordConfirm,
  }) async {
    final d = await send('POST', '/me/password', {
      'current_password': current,
      'password': password,
      'password_confirm': passwordConfirm,
    });
    if (d is Map && d['token'] is String) {
      token = d['token'] as String;
    }
  }

  /// "Akkauntni o'chirish" (Google Play/App Store talabi — veb nusxasi
  /// https://ondex.uz/delete-account).
  ///
  /// MA'LUMOT O'CHIRILMAYDI: buyurtmalar, sevimlilar, manzil — hammasi
  /// saqlanadi. Faqat kirish yopiladi va server BARCHA sessiyalarni
  /// bekor qiladi. O'sha telefon/email bilan qaytadan tasdiqlansa
  /// (SMS/Telegram/Google), akkaunt avtomatik tiklanadi
  /// (`internal/users/service.go` dagi `reactivateIfDeleted`).
  ///
  /// `currentPassword`: akkauntda parol bo'lsa va bu sessiya SMS kod
  /// bilan yaqinda (15 daq) tasdiqlanmagan bo'lsa SHART — server
  /// `ErrCurrentPasswordWrong` (matni: "joriy parol noto'g'ri")
  /// qaytaradi, chaqiruvchi shu matndan parol maydonini ko'rsatishi
  /// kerakligini bilib oladi (`SetPassword` bilan bir xil qoida).
  Future<void> deleteAccount({String currentPassword = ''}) async {
    await send('POST', '/me/delete-account',
        {'current_password': currentPassword});
  }

  /// Ism/familiyani saqlash (profil).
  Future<Map<String, dynamic>> updateName(String first, String last) async =>
      Map<String, dynamic>.from(
          await send('POST', '/me', {'first_name': first, 'last_name': last}));

  /// Koordinatani manzil matniga aylantiradi — server orqali (Google
  /// Geocoding API'ni brauzerdan to'g'ridan-to'g'ri chaqirish CORS
  /// tomonidan bloklanadi, shuning uchun backend proksi qiladi).
  // Mantiq `ondex_core.ApiClient.reverseGeocode` da — u topilmasa
  // BO'SH satr qaytaradi.

  /// Foydalanuvchi yozgan matnga mos manzil takliflari ro'yxati.
  Future<List<Map<String, String>>> addressAutocomplete(String input) async {
    final d = await send('GET',
        '/geocode/autocomplete?input=${Uri.encodeQueryComponent(input)}');
    if (d is! List) return [];
    return d
        .map((e) => Map<String, String>.from(e as Map))
        .toList(growable: false);
  }

  /// Tanlangan taklif uchun aniq koordinata + to'liq manzil.
  Future<Map<String, dynamic>> placeDetails(String placeId) async {
    final d = await send(
        'GET', '/geocode/place?place_id=${Uri.encodeQueryComponent(placeId)}');
    return Map<String, dynamic>.from(d);
  }

  /// Mijozning xaritadan tanlab saqlagan yetkazib berish manzili
  /// (hali tanlanmagan bo'lsa lat/lng == 0 va matn maydonlari bo'sh keladi).
  Future<Map<String, dynamic>> getMyAddress() async =>
      Map<String, dynamic>.from(await send('GET', '/me/address'));

  Future<Map<String, dynamic>> saveAddress({
    required double lat,
    required double lng,
    required String text,
    String entrance = '',
    String floor = '',
    String apartment = '',
    String intercom = '',
    String comment = '',
  }) async {
    final d = await send('POST', '/me/address', {
      'lat': lat,
      'lng': lng,
      'text': text,
      'entrance': entrance,
      'floor': floor,
      'apartment': apartment,
      'intercom': intercom,
      'comment': comment,
    });
    return Map<String, dynamic>.from(d);
  }

  // ── Ulangan ilovalar (tashqi AI agentlar) ────────────────────────
  //
  // ┌─ NIMA UCHUN BU EKRAN BOR ────────────────────────────────────┐
  // Tashqi yordamchi (masalan Shaddiy AI) foydalanuvchi nomidan
  // buyurtma bera oladi. Bunday huquq FAQAT shu yerdan beriladi va
  // FAQAT shu yerdan uziladi — ya'ni odam nima bo'layotganini
  // ko'radi va istalgan payt to'xtata oladi.
  //
  // Backend: internal/httpapi/routes_me_agent.go
  // └──────────────────────────────────────────────────────────────┘

  /// Rozilik ekrani uchun: kim so'rayapti va nimaga.
  Future<Map<String, dynamic>> agentLinkInfo(String code) async =>
      Map<String, dynamic>.from(await send('GET', '/me/agent/link/$code'));

  /// Ruxsat berish. perOrderLimitTiyin = 0 — HAR BIR buyurtma
  /// ilovada alohida tasdiqlanadi (eng qattiq rejim, standart).
  Future<Map<String, dynamic>> agentApprove(
    String code, {
    required List<String> scopes,
    int perOrderLimitTiyin = 0,
    int dailyLimitTiyin = 0,
  }) async =>
      Map<String, dynamic>.from(
          await send('POST', '/me/agent/link/$code/approve', {
        'scopes': scopes,
        'per_order_limit_tiyin': perOrderLimitTiyin,
        'daily_limit_tiyin': dailyLimitTiyin,
      }));

  Future<void> agentDeny(String code) async =>
      send('POST', '/me/agent/link/$code/deny');

  /// Ulangan ilovalar ro'yxati.
  Future<List<dynamic>> agentGrants() async =>
      (await send('GET', '/me/agent/grants')) as List<dynamic>? ?? [];

  /// Uzish — ta'siri DARHOL (server tokenni har so'rovda tekshiradi).
  Future<void> agentRevoke(String grantId) async =>
      send('DELETE', '/me/agent/grants/$grantId');

  /// Agent nima qilgani (audit).
  Future<List<dynamic>> agentActivity() async =>
      (await send('GET', '/me/agent/activity')) as List<dynamic>? ?? [];

  /// Tasdiqlashni kutayotgan buyurtmalar (chegaradan oshganlar).
  Future<List<dynamic>> agentDrafts() async =>
      (await send('GET', '/me/agent/drafts')) as List<dynamic>? ?? [];

  Future<Map<String, dynamic>> agentApproveDraft(String draftId) async =>
      Map<String, dynamic>.from(
          await send('POST', '/me/agent/drafts/$draftId/approve'));

  Future<void> agentRejectDraft(String draftId) async =>
      send('POST', '/me/agent/drafts/$draftId/reject');

  // ── Ilova ichidagi AI yordamchi ──────────────────────────────────
  //
  // ┌─ "Ulangan ilovalar" BILAN CHALKASHTIRMANG ───────────────────┐
  // Yuqoridagi `agent*` metodlari TASHQI ilovaga ruxsat berish
  // uchun. Bular esa OnDex ilovasining O'Z yordamchisi: oddiy
  // sessiya bilan ishlaydi va buyurtma YARATMAYDI — u savat
  // taklifini qaytaradi, tugmani foydalanuvchi bosadi.
  //
  // Backend: internal/httpapi/routes_assistant.go
  // └──────────────────────────────────────────────────────────────┘

  /// Yordamchi serverda yoqilganmi (tugmani ko'rsatish uchun).
  ///
  /// `enabled` — matnli chat, `voice` — OVOZLI rejim (Gemini Live).
  /// Ikkalasi ALOHIDA: `GEMINI_API_KEY` qo'yilmagan serverda chat
  /// ishlaydi, ovoz esa yo'q. Busiz ilova mikrofon tugmasini chizib,
  /// bosilganda 404 ko'rsatardi.
  /// Yordamchi amallari va ularning ruxsat holati.
  ///
  /// Ro'yxatni SERVER beradi: ilova nomlarni qo'lda yozsa, backend'da
  /// yangi amal qo'shilganda u ilovada ko'rinmasdi.
  Future<List<Map<String, dynamic>>> aiTools() async {
    final d = await send('GET', '/me/ai-tools');
    final list = (d is Map ? d['tools'] : null) as List? ?? const [];
    return list.cast<Map<String, dynamic>>();
  }

  /// Yoqilgan amallarni saqlaydi (o'chirilganlarni server hisoblaydi).
  Future<void> setAiTools(List<String> enabled) async {
    await send('PUT', '/me/ai-tools', {'enabled': enabled});
  }

  Future<Map<String, dynamic>> aiStatus() async {
    final d = await send('GET', '/ai/status');
    return d is Map<String, dynamic> ? d : const {};
  }

  /// Bitta savol. `history` — oldingi xabarlar (`{role, content}`),
  /// ilovada saqlanadi; server sessiya ushlab turmaydi.
  Future<Map<String, dynamic>> aiChat(
    String message,
    List<Map<String, String>> history,
  ) async =>
      Map<String, dynamic>.from(await send('POST', '/ai/chat', {
        'message': message,
        'history': history,
      }));
}

/// Butun ilova uchun bitta umumiy client.
final api = CustomerApi();

/// Rasm manzili — `ondex_core.fullImageUrl` ustidagi yupqa o'ram
/// (baseUrl har safar yozilmasligi uchun).
String fullImageUrl(String? path) => coreFullImageUrl(path, apiBaseUrl);
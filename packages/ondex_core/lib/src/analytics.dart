/// PostHog'ga hodisa yuborish — BARCHA platformalarda.
///
/// ┌─ NEGA SDK EMAS, TO'G'RIDAN-TO'G'RI HTTP ───────────────────────────┐
/// `posthog_flutter` paketi Windows'ni QO'LLAMAYDI (pub.dev: android,
/// ios, macos, web). Admin va restoran panellari esa aynan Windows
/// desktop ilovalari.
///
/// Ya'ni SDK ga tayansak panellar umuman tahlilsiz qolardi. PostHog'ning
/// hodisa qabul qilish API'si esa oddiy HTTP POST — u har joyda
/// ishlaydi va hech qanday platforma plagini talab qilmaydi.
///
/// Mobil ilovalarda SDK QO'SHIMCHA ravishda ishlatiladi: u SEANS
/// YOZUVINI (session replay) beradi, buni HTTP bilan qilib bo'lmaydi.
/// Ya'ni ikkalasi bir-birini almashtirmaydi, to'ldiradi.
/// └────────────────────────────────────────────────────────────────────┘
library;

import 'dart:async';
import 'dart:convert';
import 'dart:math';

import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';

import 'config.dart';

/// Hodisalarni PostHog'ga yuboradi.
///
/// Butun ilovada BITTA nusxa ishlatiladi (`Analytics.instance`).
class Analytics {
  Analytics._();

  static final Analytics instance = Analytics._();

  /// ┌─ SDK BRIDGE: posthog_flutter bilan sinxronlash ─────────────┐
  /// Mijoz, kuryer va affitsiant mobil ilovalarida Session Replay
  /// (ekran yozuvi) `posthog_flutter: ^5.39.0` SDK orqali ishlaydi.
  /// Bu SDK O'Z ICHKIDA `distinctId` ni saqlaydi. Biz HTTP orqali
  /// `Analytics.identify()` chaqirsak ham, SDK BILISHI UMMUMIY
  /// EMAS — va Recordinglar odamga biriktirilmagan holatda qoladi
  /// (anonym ID bilan yoziladi va person sahifasida ko'rinmaydi).
  ///
  /// SHUNING UCHUN callback orqali bridge: mobil ilova main.dart da
  /// SDK ni setup() qilgandan keyin shu callbacklarni ro'yxatdan
  /// o'tkazadi — va biz identify/reset chaqirilganda SDK bilan ham
  /// bir xil amal bajarilishini kafolatlaymiz.
  ///
  /// Windows/Linux/macOS panellarida bu callbacklar ro'yxatdan
  /// o'tkazilmaydi (posthog_flutter ularni qo'llamaydi) — hammasi
  /// noto'g'ri ishlashdan xavfsiz.
  /// └───────────────────────────────────────────────────────────────┘
  Future<void> Function({
    required String userId,
    String? phone,
    String? name,
    String? role,
  })? onIdentify;

  Future<void> Function(String newAnonymousId)? onReset;

  /// Kim ekanligimiz. Login qilinmaguncha qurilma identifikatori.
  String _distinctId = '';

  /// Har bir hodisaga qo'shiladigan doimiy xossalar.
  final Map<String, Object?> _superProps = {};

  /// ┌─ NAVBAT VA GURUHLAB YUBORISH ────────────────────────────────────┐
  /// Har bosishda alohida so'rov yuborish tarmoqni behuda band qiladi
  /// va sekin ulanishda interfeysni sekinlashtiradi.
  ///
  /// Shuning uchun hodisalar navbatga yig'iladi va 5 soniyada bir
  /// (yoki 20 tadan oshganda) BIR SO'ROVDA yuboriladi.
  /// └──────────────────────────────────────────────────────────────────┘
  final List<Map<String, Object?>> _queue = [];
  Timer? _timer;

  static const _flushEvery = Duration(seconds: 5);
  static const _maxBatch = 20;

  /// ┌─ NAVBAT CHEKSIZ O'SMASLIGI KERAK ────────────────────────────────┐
  /// Internet uzoq vaqt yo'q bo'lsa navbat cheksiz o'sib, xotirani
  /// yeb qo'yardi. Chegaradan oshganda ENG ESKI hodisalar tashlanadi:
  /// yangi harakatlar eskisidan qimmatroq.
  /// └──────────────────────────────────────────────────────────────────┘
  static const _maxQueue = 200;

  bool get _on => posthogEnabled;

  /// Ishga tushirish — ilova boshlanganda bir marta.
  ///
  /// ┌─ `$session_id` — NEGA SHU YERDA QO'YILADI ─────────────────────────┐
  /// Busiz PostHog interfeysida hodisa oldidagi "View recording" tugmasi
  /// har doim "No session ID associated with this event" derdi — hatto
  /// mobil ilovada HAQIQIY video yozuv mavjud bo'lsa ham (SDK session
  /// replay'ni O'ZINING ichki seans ID'si bilan yozadi, biz esa HTTP
  /// orqali yuborgan hodisalarga uni HECH QACHON qo'shmagan edik).
  ///
  /// Shu yerda avval TAXMINIY (mahalliy generatsiya qilingan) qiymat
  /// qo'yiladi — bu Windows panellarida ham hodisalarni kamida "seans"
  /// sifatida guruhlaydi (video bo'lmasa ham). Mobil ilovada esa
  /// `setSdkSessionId()` orqali HAQIQIY SDK seansi bilan USTIGA YOZILADI
  /// (`main.dart`, `Posthog().getSessionId()`) — shundan keyin HTTP orqali
  /// yuborilgan har qanday hodisa ("Screen", custom hodisalar) SDK yozib
  /// turgan video bilan TO'G'RI bog'lanadi.
  /// └────────────────────────────────────────────────────────────────────┘
  void start({required String distinctId}) {
    if (!_on) return;
    _distinctId = distinctId;
    _superProps['platform'] = clientPlatform;
    _superProps['app_version'] = appVersion;
    _superProps['\$session_id'] = _newLocalSessionId();
    _timer ??= Timer.periodic(_flushEvery, (_) => flush());
  }

  static String _newLocalSessionId() {
    final rnd = Random.secure();
    return List.generate(32, (_) => rnd.nextInt(16).toRadixString(16)).join();
  }

  /// SDK'ning HAQIQIY seans ID'sini o'rnatadi (`Posthog().getSessionId()`).
  /// Faqat mobil ilovalarda chaqiriladi — Windows panellarida SDK
  /// umuman yo'q, mahalliy generatsiya qilingan qiymat qoladi.
  void setSdkSessionId(String? id) {
    if (id != null && id.isNotEmpty) _superProps['\$session_id'] = id;
  }

  /// ┌─ ANONIM IDENTIFIKATOR ───────────────────────────────────────────┐
  /// Foydalanuvchi hali kirmagan bo'lsa ham uning yo'lini kuzatish
  /// kerak: katalogni ochdi, mahsulot ko'rdi, ro'yxatdan o'tdi.
  ///
  /// Identifikator QURILMAGA bog'lanadi va saqlanadi — aks holda har
  /// ochilishda yangi "odam" paydo bo'lib, bitta foydalanuvchining
  /// yo'li o'nlab bo'lakka bo'linib ketardi.
  ///
  /// Kirgach `identify()` chaqiriladi va PostHog anonim yo'lni
  /// haqiqiy akkauntga BIRLASHTIRADI (`$anon_distinct_id`).
  /// └──────────────────────────────────────────────────────────────────┘
  static const _anonKey = 'ondex_analytics_anon_id';

  static Future<String> anonymousId() async {
    final prefs = await SharedPreferences.getInstance();
    final saved = prefs.getString(_anonKey);
    if (saved != null && saved.isNotEmpty) return saved;
    return _newAnonymousId(prefs);
  }

  /// ┌─ CHIQISHDA YANGI ANONIM ID SHART ────────────────────────────────┐
  /// Chiqqandan keyin ESKI anonim ID ni qayta ishlatib bo'lmaydi:
  /// `identify()` uni allaqachon o'sha akkauntga BIRLASHTIRGAN
  /// (`$anon_distinct_id`). Ya'ni eski ID bilan davom etsak, keyingi
  /// odamning harakatlari chiqib ketgan odamning yo'liga yozilardi.
  ///
  /// Bu bitta qurilmani bir necha kishi ishlatganda (restoran xodimlari
  /// almashganda) tahlilni butunlay ishonchsiz qilardi.
  /// └──────────────────────────────────────────────────────────────────┘
  static Future<String> resetAnonymousId() async =>
      _newAnonymousId(await SharedPreferences.getInstance());

  static Future<String> _newAnonymousId(SharedPreferences prefs) async {
    final rnd = Random.secure();
    final id =
        'anon_${List.generate(16, (_) => rnd.nextInt(16).toRadixString(16)).join()}';
    await prefs.setString(_anonKey, id);
    return id;
  }

  /// Qulaylik: anonim ID ni o'qib, `start()` ni chaqiradi.
  ///
  /// Xatolik ilovani TO'XTATMAYDI — tahlil hech qachon ishga
  /// tushishga to'sqinlik qilmasligi kerak.
  static Future<void> bootstrap() async {
    if (!posthogEnabled) return;
    try {
      instance.start(distinctId: await anonymousId());
    } catch (e) {
      if (kDebugMode) debugPrint('[Analytics] boshlanmadi: $e');
    }
  }

  /// Foydalanuvchi kirgach chaqiriladi: hodisalar endi ANIQ odamga
  /// bog'lanadi.
  ///
  /// `$set` — PostHog'da odam xossalari. Admin panelida odamni
  /// telefon raqami bo'yicha topish uchun kerak.
  Future<void> identify({
    required String userId,
    String? phone,
    String? name,
    String? role,
  }) async {
    if (!_on) return;
    final prev = _distinctId;
    _distinctId = userId;
    capture('\$identify', {
      '\$anon_distinct_id': prev,
      '\$set': {
        if (phone != null && phone.isNotEmpty) 'phone': phone,
        if (name != null && name.isNotEmpty) 'name': name,
        if (role != null && role.isNotEmpty) 'role': role,
      },
    });
    // SDK BRIDGE: posthog_flutter Session Replay ni shu odamga
    // biriktirish uchun SDK ga ham aytamiz. Callback berilmagan bo'lsa
    // (panel uchun) hech narsa.
    try {
      await onIdentify?.call(
        userId: userId,
        phone: phone,
        name: name,
        role: role,
      );
    } catch (e) {
      if (kDebugMode) debugPrint('[Analytics] SDK identify xatosi: $e');
    }
  }

  /// Chiqishda: keyingi hodisalar boshqa odamga yozilmasin.
  Future<void> reset(String anonymousId) async {
    if (!_on) return;
    await flush();
    _distinctId = anonymousId;
    // SDK BRIDGE: posthog_flutter SDK ham yangi anonim idga o'tkazamiz,
    // aks holda chiqib ketgan odamning Recordingiga keyingi kishi
    // o'tib yozadi.
    try {
      await onReset?.call(anonymousId);
    } catch (e) {
      if (kDebugMode) debugPrint('[Analytics] SDK reset xatosi: $e');
    }
  }

  /// Hodisa yozish.
  void capture(String event, [Map<String, Object?>? props]) {
    if (!_on || _distinctId.isEmpty) return;
    _queue.add({
      'event': event,
      'distinct_id': _distinctId,
      'timestamp': DateTime.now().toUtc().toIso8601String(),
      'properties': {..._superProps, ...?props},
    });
    if (_queue.length > _maxQueue) {
      _queue.removeRange(0, _queue.length - _maxQueue);
    }
    if (_queue.length >= _maxBatch) flush();
  }

  /// Ekran ochilganda.
  void screen(String name, [Map<String, Object?>? props]) =>
      capture('\$screen', {'\$screen_name': name, ...?props});

  /// Navbatni yuboradi.
  ///
  /// Xatolik JIMGINA yutiladi: tahlil ilovaning ishlashiga TA'SIR
  /// QILMASLIGI kerak. PostHog yiqilgani uchun mijoz buyurtma bera
  /// olmay qolsa, bu tahlildan ko'ra qimmatroq zarar bo'lardi.
  Future<void> flush() async {
    if (!_on || _queue.isEmpty) return;
    final batch = List<Map<String, Object?>>.from(_queue);
    _queue.clear();
    try {
      await http
          .post(
            Uri.parse('$posthogHost/batch/'),
            headers: const {'Content-Type': 'application/json'},
            body: jsonEncode({
              'api_key': posthogApiKey,
              'batch': batch,
            }),
          )
          .timeout(const Duration(seconds: 10));
    } catch (e) {
      if (kDebugMode) {
        debugPrint('[Analytics] yuborilmadi: $e');
      }
      // Qayta urinish uchun navbatga QAYTARAMIZ — lekin faqat
      // chegaragacha, aks holda uzilgan tarmoqda navbat cheksiz o'sardi.
      _queue.insertAll(0, batch.take(_maxQueue - _queue.length).toList());
    }
  }

  /// Ilova yopilishidan oldin.
  Future<void> dispose() async {
    _timer?.cancel();
    _timer = null;
    await flush();
  }
}

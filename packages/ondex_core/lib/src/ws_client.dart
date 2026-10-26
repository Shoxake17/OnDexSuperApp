import 'dart:async';
import 'dart:convert';
import 'dart:math';

import 'package:web_socket_channel/web_socket_channel.dart';

/// Jonli yangilanishlar uchun WebSocket klienti — barcha ilovalar uchun
/// bitta nusxa.
///
/// Avval qayta ulanish mantiqi TO'RT joyda takrorlangan edi va to'rttasida
/// ham AYNI xatolar bor edi:
///   * `StreamSubscription` saqlanmasdi/bekor qilinmasdi (sizib chiqish);
///   * eski kanal yopilmasdan yangisi tayinlanardi;
///   * fiksirlangan 2 sekundlik interval — backoff yo'q, chegara yo'q,
///     jitter yo'q. Server yotganda har bir klient daqiqada 30 marta
///     urinardi va ko'tarilayotgan serverni yana yiqitardi;
///   * `jsonDecode` himoyasiz edi — kutilmagan freym butun ishlovchini
///     o'ldirardi va foydalanuvchi buni umuman sezmasdi.
///
/// ┌─ "QAYTA ULANMAY QOLISH" NOSOZLIGI (2026-10-20) ────────────────────┐
/// Restoran paneli soat 07:02:55 da uzilgach, 12 daqiqa davomida
/// umuman qayta ulanmadi (server logida birorta `ws: ulandi` yo'q),
/// holbuki panel oynasi ochiq turardi. Natijada karta bilan to'langan
/// buyurtma restoranga JONLI yetmasdi — faqat zaxira so'rov sikli
/// uni kechikib ko'rsatardi.
///
/// Ikki sabab tuzatildi:
///
///  1. `_closeCurrent()` cheksiz kutishi mumkin edi. `sink.close()`
///     o'lik soketda hech qachon yakunlanmasligi mumkin, u esa
///     `connect()` ning BIRINCHI qatori — ya'ni qayta ulanish
///     boshlanmasdan osilib qolardi va boshqa hech qanday taymer
///     qolmagani uchun klient ABADIY jim bo'lardi. Endi yopish
///     muddat bilan chegaralangan.
///
///  2. "Yarim ochiq" ulanish sezilmasdi. TCP uzilganini OS har doim
///     ham darhol aytmaydi: soket ochiq ko'rinadi, lekin hech narsa
///     kelmaydi. Endi klient har 25 soniyada `{"type":"ping"}`
///     yuboradi va server `{"type":"pong"}` qaytaradi; javob
///     kelmasa ulanish o'lik deb hisoblanib qayta quriladi.
/// └────────────────────────────────────────────────────────────────────┘
class WsClient {
  WsClient({
    required this.ticketProvider,
    required this.urlBuilder,
    required this.onEvent,
    this.onStateChange,
  });

  /// Har ulanishdan OLDIN yangi bir martalik bilet oladi.
  final Future<String> Function() ticketProvider;

  /// Biletdan to'liq `ws://...` manzilini quradi.
  final String Function(String ticket) urlBuilder;

  /// Kelgan hodisa (JSON obyekt).
  final void Function(Map<String, dynamic> event) onEvent;

  /// Ulanish holati o'zgarganda (UI'da "oflayn" ko'rsatish uchun).
  final void Function(bool connected)? onStateChange;

  WebSocketChannel? _channel;
  StreamSubscription<dynamic>? _sub;
  Timer? _retryTimer;
  Timer? _pingTimer;
  Timer? _stableTimer;
  int _attempt = 0;
  bool _disposed = false;

  /// Oxirgi freym kelgan payt (har qanday xabar yoki `pong`).
  DateTime _lastFrame = DateTime.now();

  /// Server `pong` bilan javob beradimi.
  ///
  /// Hali yangilanmagan server javob bermaydi. Bunday holda jimlik
  /// qo'riqchisi O'CHADI — aks holda u mutlaqo sog'lom ulanishni har
  /// 70 soniyada uzib tashlardi.
  bool _pongSeen = false;

  static const _baseDelay = Duration(seconds: 2);
  static const _maxDelay = Duration(minutes: 2);

  /// Eski ulanishni yopishga beriladigan vaqt. O'tib ketsa yopish
  /// fonda qoldiriladi va yangi ulanish baribir boshlanadi.
  static const _closeTimeout = Duration(seconds: 3);

  /// Tiriklik so'rovi oralig'i.
  static const _heartbeat = Duration(seconds: 25);

  /// Shuncha vaqt davomida serverdan hech narsa kelmasa ulanish o'lik
  /// deb hisoblanadi (ikkita o'tkazib yuborilgan `ping` dan ko'proq).
  static const _silenceLimit = Duration(seconds: 70);

  /// Ulanish shuncha vaqt tirik tursa "muvaffaqiyatli" deb hisoblanadi
  /// va backoff nolga qaytadi.
  ///
  /// Avval `_attempt` soket OCHILISHI bilanoq nollanardi — ya'ni
  /// darhol uzilib ketadigan ulanishda ham. Shu sabab backoff amalda
  /// hech qachon o'smasdi va yiqilgan server har 2 soniyada butun
  /// mijozlar to'lqinini olardi.
  static const _stableAfter = Duration(seconds: 10);

  final _rand = Random();

  Future<void> connect() async {
    if (_disposed) return;
    await _closeCurrent();
    if (_disposed) return;
    try {
      final ticket = await ticketProvider();
      if (_disposed) return;
      final ch = WebSocketChannel.connect(Uri.parse(urlBuilder(ticket)));
      _channel = ch;
      _sub = ch.stream.listen(
        _handleData,
        onError: (_) => _scheduleRetry(),
        onDone: _scheduleRetry,
        cancelOnError: true,
      );
      _lastFrame = DateTime.now();
      _stableTimer?.cancel();
      _stableTimer = Timer(_stableAfter, () => _attempt = 0);
      _startHeartbeat();
      onStateChange?.call(true);
    } catch (_) {
      _scheduleRetry();
    }
  }

  void _handleData(dynamic raw) {
    // Har qanday freym — ulanish tirikligining dalili.
    _lastFrame = DateTime.now();
    // Himoyalangan dekod: binar freym yoki kutilmagan shakl butun
    // tinglovchini o'ldirmasligi kerak.
    try {
      if (raw is! String) return;
      final decoded = jsonDecode(raw);
      if (decoded is! Map<String, dynamic>) return;
      if (decoded['type'] == 'pong') {
        // Faqat tiriklik javobi — ekranlarga uzatilmaydi.
        _pongSeen = true;
        return;
      }
      onEvent(decoded);
    } catch (_) {
      // Noto'g'ri xabar shunchaki tashlanadi — ulanish tirik qoladi.
    }
  }

  /// Davriy tiriklik so'rovi va "yarim ochiq" ulanish qo'riqchisi.
  void _startHeartbeat() {
    _pingTimer?.cancel();
    _pingTimer = Timer.periodic(_heartbeat, (_) {
      if (_disposed) return;
      final ch = _channel;
      if (ch == null) return;
      if (_pongSeen && DateTime.now().difference(_lastFrame) > _silenceLimit) {
        // Server bizni eshitmayapti — soket ochiq ko'rinsa ham o'lik.
        _scheduleRetry();
        return;
      }
      try {
        ch.sink.add('{"type":"ping"}');
      } catch (_) {
        _scheduleRetry();
      }
    });
  }

  void _scheduleRetry() {
    if (_disposed) return;
    onStateChange?.call(false);
    _pingTimer?.cancel();
    _pingTimer = null;
    _stableTimer?.cancel();
    _stableTimer = null;
    _retryTimer?.cancel();

    // Eksponensial backoff + jitter. Jitter MUHIM: usiz barcha klientlar
    // bir vaqtda qayta urinadi ("thundering herd") va endigina
    // ko'tarilgan serverni yana yiqitadi.
    final expMs = _baseDelay.inMilliseconds * (1 << _attempt.clamp(0, 6));
    final cappedMs = expMs.clamp(0, _maxDelay.inMilliseconds);
    final jitterMs = _rand.nextInt((cappedMs ~/ 4) + 1);
    _attempt++;

    _retryTimer = Timer(Duration(milliseconds: cappedMs + jitterMs), connect);
  }

  /// Eski ulanishni yopadi — LEKIN cheksiz kutmaydi.
  ///
  /// O'lik soketda `cancel()` yoki `sink.close()` hech qachon
  /// yakunlanmasligi mumkin. Avval bu `connect()` ni butunlay osib
  /// qo'yardi va klient boshqa hech qachon ulanmasdi.
  Future<void> _closeCurrent() async {
    final sub = _sub;
    final ch = _channel;
    _sub = null;
    _channel = null;
    _pingTimer?.cancel();
    _pingTimer = null;
    _stableTimer?.cancel();
    _stableTimer = null;
    try {
      await sub?.cancel().timeout(_closeTimeout);
    } catch (_) {}
    try {
      await ch?.sink.close().timeout(_closeTimeout);
    } catch (_) {}
  }

  /// Ekran yopilganda MAJBURIY chaqiriladi.
  Future<void> dispose() async {
    _disposed = true;
    _retryTimer?.cancel();
    _retryTimer = null;
    await _closeCurrent();
  }
}

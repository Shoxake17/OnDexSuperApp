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
  int _attempt = 0;
  bool _disposed = false;

  static const _baseDelay = Duration(seconds: 2);
  static const _maxDelay = Duration(minutes: 2);
  final _rand = Random();

  Future<void> connect() async {
    if (_disposed) return;
    await _closeCurrent();
    try {
      final ticket = await ticketProvider();
      final ch = WebSocketChannel.connect(Uri.parse(urlBuilder(ticket)));
      _channel = ch;
      _sub = ch.stream.listen(
        _handleData,
        onError: (_) => _scheduleRetry(),
        onDone: _scheduleRetry,
        cancelOnError: true,
      );
      _attempt = 0; // muvaffaqiyatli ulanish — backoff nolga qaytadi
      onStateChange?.call(true);
    } catch (_) {
      _scheduleRetry();
    }
  }

  void _handleData(dynamic raw) {
    // Himoyalangan dekod: binar freym yoki kutilmagan shakl butun
    // tinglovchini o'ldirmasligi kerak.
    try {
      if (raw is! String) return;
      final decoded = jsonDecode(raw);
      if (decoded is Map<String, dynamic>) onEvent(decoded);
    } catch (_) {
      // Noto'g'ri xabar shunchaki tashlanadi — ulanish tirik qoladi.
    }
  }

  void _scheduleRetry() {
    if (_disposed) return;
    onStateChange?.call(false);
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

  Future<void> _closeCurrent() async {
    await _sub?.cancel();
    _sub = null;
    try {
      await _channel?.sink.close();
    } catch (_) {}
    _channel = null;
  }

  /// Ekran yopilganda MAJBURIY chaqiriladi.
  Future<void> dispose() async {
    _disposed = true;
    _retryTimer?.cancel();
    await _closeCurrent();
  }
}

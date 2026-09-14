import 'dart:async';
import 'dart:math' as math;

import 'package:flutter/foundation.dart';

import 'api.dart';
import 'live.dart';

/// Restoran bildirishnomalari — butun panel uchun BITTA jonli manba
/// (qo'ng'iroq belgisi, yon menyu rozetkasi, "Bildirishnomalar" sahifasi).
///
/// ┌─ UZILISHSIZ VA KECHIKISHSIZ ──────────────────────────────────────┐
/// 1. Server bildirishnomani AVVAL bazaga yozadi, keyin rahbariyat
///    kanaliga jonli yuboradi — panel ochiq bo'lsa u darhol keladi.
/// 2. Har bildirishnomaning o'suvchi `seq` raqami bor. Soket uzilsa
///    (tarmoq, server qayta ishga tushdi, noutbuk uyqudan uyg'ondi)
///    qayta ulangan zahoti "oxirgi olingan raqamdan keyingilari"
///    so'raladi — orada kelganlarning HECH BIRI tushib qolmaydi.
/// 3. Bir xabar ikki yo'ldan kelishi mumkin (jonli + qayta so'rov) —
///    `id` bo'yicha bir marta ko'rsatiladi.
/// 4. Soket ulanmagan paytda zaxira so'rov tez-tez (15s), ulangan
///    paytda siyrak (60s) ishlaydi.
/// └───────────────────────────────────────────────────────────────────┘
class NotificationCenter extends ChangeNotifier {
  NotificationCenter({LiveBus? bus}) : _busOverride = bus;

  final LiveBus? _busOverride;
  LiveBus get _bus => _busOverride ?? restaurantLive;

  int _unread = 0;
  int _latestSeq = 0;

  /// O'qilmaganlar soni (server qiymati).
  int get unread => _unread;

  /// Panel olgan eng katta tartib raqami.
  int get latestSeq => _latestSeq;

  final _incoming = StreamController<Map<String, dynamic>>.broadcast();
  final _readEvents = StreamController<Map<String, dynamic>>.broadcast();

  /// Yangi bildirishnomalar (JSON), `seq` tartibida, takrorsiz.
  Stream<Map<String, dynamic>> get incoming => _incoming.stream;

  /// O'qilgan deb belgilashlar (boshqa kompyuterda ham bosilgan bo'lishi mumkin).
  Stream<Map<String, dynamic>> get readEvents => _readEvents.stream;

  static const _seenCap = 500;
  final _seen = <String>{};
  final _seenOrder = <String>[];

  StreamSubscription<Map<String, dynamic>>? _eventSub;
  StreamSubscription<bool>? _connSub;
  Timer? _timer;
  bool _started = false;
  bool _syncing = false;
  bool _syncAgain = false;

  bool get started => _started;

  /// Shell ochilganda (kirishdan keyin) bir marta.
  void start() {
    if (_started) return;
    _started = true;
    _eventSub = _bus.events.listen(handleEvent);
    _connSub = _bus.connection.listen((connected) {
      if (connected) sync();
      _restartTimer();
    });
    _restartTimer();
    refreshSummary(initial: true);
  }

  /// Chiqishda: holat tozalanadi (keyingi akkaunt eski raqamni ko'rmasin).
  Future<void> stop() async {
    await _eventSub?.cancel();
    await _connSub?.cancel();
    _eventSub = null;
    _connSub = null;
    _timer?.cancel();
    _started = false;
    _unread = 0;
    _latestSeq = 0;
    _seen.clear();
    _seenOrder.clear();
    notifyListeners();
  }

  void _restartTimer() {
    _timer?.cancel();
    final interval = _bus.connected ? const Duration(seconds: 60) : const Duration(seconds: 15);
    _timer = Timer.periodic(interval, (_) => sync());
  }

  /// Jonli kanal hodisasi.
  @visibleForTesting
  void handleEvent(Map<String, dynamic> e) {
    switch (e['type']) {
      case 'restaurant_notification':
        final n = e['notification'];
        if (n is! Map) return;
        final accepted = _accept(Map<String, dynamic>.from(n));
        final u = e['unread'];
        if (u is num) _unread = u.toInt();
        if (accepted || u is num) notifyListeners();
      case 'restaurant_notifications_read':
        final u = e['unread'];
        if (u is num) {
          _unread = u.toInt();
          notifyListeners();
        }
        if (!_readEvents.isClosed) _readEvents.add(e);
    }
  }

  bool _accept(Map<String, dynamic> n) {
    final id = n['id'];
    if (id is! String || id.isEmpty || _seen.contains(id)) return false;
    _seen.add(id);
    _seenOrder.add(id);
    if (_seenOrder.length > _seenCap) _seen.remove(_seenOrder.removeAt(0));
    final seq = n['seq'];
    if (seq is num) _latestSeq = math.max(_latestSeq, seq.toInt());
    if (!_incoming.isClosed) _incoming.add(n);
    return true;
  }

  /// O'qilmaganlar soni; `initial` — ishga tushishda boshlang'ich raqam.
  Future<void> refreshSummary({bool initial = false}) async {
    try {
      final s = await api.notificationsSummary();
      final u = s['unread'];
      final latest = s['latest_seq'];
      if (u is num) _unread = u.toInt();
      if (latest is num) {
        if (initial && _latestSeq == 0) {
          _latestSeq = latest.toInt();
        } else if (latest.toInt() > _latestSeq) {
          await sync();
        }
      }
      notifyListeners();
    } catch (_) {
      // Zaxira so'rov keyingi siklda qayta urinadi.
    }
  }

  /// Oxirgi olingan raqamdan keyingilarini oladi (qayta ulanish, zaxira).
  Future<void> sync() async {
    if (_syncing) {
      _syncAgain = true;
      return;
    }
    _syncing = true;
    try {
      do {
        _syncAgain = false;
        if (_latestSeq == 0) {
          await refreshSummary(initial: true);
          continue;
        }
        var after = _latestSeq;
        for (var page = 0; page < 20; page++) {
          final res = await api.notifications(after: after, limit: 100);
          final items = res['items'] is List ? res['items'] as List : const [];
          for (final it in items) {
            if (it is! Map) continue;
            final m = Map<String, dynamic>.from(it);
            _accept(m);
            final seq = m['seq'];
            if (seq is num) after = math.max(after, seq.toInt());
          }
          final u = res['unread'];
          if (u is num) _unread = u.toInt();
          if (items.length < 100) break;
        }
        notifyListeners();
      } while (_syncAgain);
    } catch (_) {
      // Tarmoq yo'q — keyingi siklda.
    } finally {
      _syncing = false;
    }
  }

  /// O'qilgan deb belgilaydi; serverdagi o'qilmaganlar sonini qaytaradi.
  Future<int> markRead(String id) async {
    final res = await api.markNotificationRead(id);
    final u = res['unread'];
    if (u is num) {
      _unread = u.toInt();
      notifyListeners();
    }
    return _unread;
  }

  /// `upToSeq` gacha hammasini o'qilgan qiladi.
  Future<int> markAllRead(int upToSeq) async {
    if (upToSeq <= 0) return _unread;
    final res = await api.markAllNotificationsRead(upToSeq);
    final u = res['unread'];
    if (u is num) {
      _unread = u.toInt();
      notifyListeners();
    }
    return _unread;
  }
}

/// Panel bo'ylab yagona nusxa.
final notificationCenter = NotificationCenter();

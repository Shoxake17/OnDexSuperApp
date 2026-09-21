import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:ondex_support/ondex_support.dart' show SupportThread;

import 'api.dart';
import 'live.dart';

/// Restoranlardan kelgan qo'llab-quvvatlash xabarlari — panel bo'ylab BITTA
/// holat: navigatsiyadagi "Chat" rozetkasi va chat sahifasining jonli
/// hodisalari.
///
/// Hodisalar ma'muriyat kanalidan keladi (`a:platform`, faqat admin roli).
/// Umumiy o'qilmaganlar soni serverdan olinadi: bir nechta admin bir vaqtda
/// ishlasa ham raqam to'g'ri qoladi.
class AdminSupportInbox extends ChangeNotifier {
  AdminSupportInbox({LiveBus? bus}) : _busOverride = bus;

  final LiveBus? _busOverride;
  LiveBus get _bus => _busOverride ?? adminLive;

  int _unread = 0;
  int get unread => _unread;

  final _events = StreamController<Map<String, dynamic>>.broadcast();
  Stream<Map<String, dynamic>> get events => _events.stream;
  Stream<bool> get connection => _bus.connection;

  StreamSubscription<Map<String, dynamic>>? _eventSub;
  StreamSubscription<bool>? _connSub;
  Timer? _timer;
  Timer? _debounce;
  bool _started = false;

  void start() {
    if (_started) return;
    _started = true;
    _eventSub = _bus.events.listen(handleEvent);
    _connSub = _bus.connection.listen((connected) {
      if (connected) refresh();
      _restartTimer();
    });
    _restartTimer();
    refresh();
  }

  Future<void> stop() async {
    await _eventSub?.cancel();
    await _connSub?.cancel();
    _eventSub = null;
    _connSub = null;
    _timer?.cancel();
    _debounce?.cancel();
    _started = false;
    _unread = 0;
    notifyListeners();
  }

  void _restartTimer() {
    _timer?.cancel();
    final interval = _bus.connected ? const Duration(seconds: 60) : const Duration(seconds: 15);
    _timer = Timer.periodic(interval, (_) => refresh());
  }

  @visibleForTesting
  void handleEvent(Map<String, dynamic> e) {
    final type = e['type'];
    if (type != 'support_message' && type != 'support_read') return;
    if (!_events.isClosed) _events.add(e);
    // Bir nechta hodisa ketma-ket kelsa bitta so'rov yetadi.
    _debounce?.cancel();
    _debounce = Timer(const Duration(milliseconds: 300), refresh);
  }

  Future<void> refresh() async {
    try {
      final s = await api.supportSummary();
      final u = s['unread'];
      if (u is num && u.toInt() != _unread) {
        _unread = u.toInt();
        notifyListeners();
      }
    } catch (_) {
      // Zaxira so'rov keyingi siklda qayta urinadi.
    }
  }
}

final supportInbox = AdminSupportInbox();

/// Restoran id -> o'qilmagan chat xabarlari soni (faqat o'qilmagani bor
/// restoranlar). Restoranlar kartalaridagi va restoran modulidagi rozetka
/// uchun. Xato chaqiruvchiga o'tadi: rozetka ixtiyoriy ma'lumot, shuning
/// uchun chaqiruvchi uni yutib yuboradi.
Future<Map<String, int>> fetchUnreadByRestaurant() async {
  final res = await api.supportThreads();
  final raw = res['items'];
  final out = <String, int>{};
  if (raw is! List) return out;
  for (final item in raw) {
    if (item is! Map) continue;
    final t = SupportThread.tryParse(item);
    final r = item['restaurant'];
    final id = r is Map ? r['id'] : null;
    if (t == null || id is! String || id.isEmpty || t.unread <= 0) continue;
    out[id] = t.unread;
  }
  return out;
}

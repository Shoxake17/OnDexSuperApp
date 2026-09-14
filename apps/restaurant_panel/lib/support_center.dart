import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:ondex_support/ondex_support.dart';

import 'api.dart';
import 'live.dart';

/// OnDex qo'llab-quvvatlash chati — panel bo'ylab BITTA holat: yon
/// menyudagi "Chat markazi" rozetkasi, "Yordam markazi" dagi javoblar
/// soni va chat sahifasining jonli hodisalari.
///
/// Hodisalar `NotificationCenter` bilan AYNI jonli kanaldan keladi
/// (restoran rahbariyati kanali — affitsiant unga obuna emas). Soket
/// uzilgan paytda zaxira so'rov tez-tez, ulanganda siyrak ishlaydi.
class SupportCenter extends ChangeNotifier {
  SupportCenter({LiveBus? bus}) : _busOverride = bus;

  final LiveBus? _busOverride;
  LiveBus get _bus => _busOverride ?? restaurantLive;

  SupportThread? _thread;
  bool _supportOnline = false;

  /// Restoran o'qimagan admin javoblari.
  int get unread => _thread?.unread ?? 0;

  /// OnDex administratori hozir panelda (jonli ulanishda)mi.
  bool get supportOnline => _supportOnline;

  SupportThread? get thread => _thread;

  final _events = StreamController<Map<String, dynamic>>.broadcast();

  /// `support_message` / `support_read` hodisalari (chat sahifasi uchun).
  Stream<Map<String, dynamic>> get events => _events.stream;

  Stream<bool> get connection => _bus.connection;

  StreamSubscription<Map<String, dynamic>>? _eventSub;
  StreamSubscription<bool>? _connSub;
  Timer? _timer;
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

  /// Chiqishda: keyingi akkaunt eski raqamni ko'rmasin.
  Future<void> stop() async {
    await _eventSub?.cancel();
    await _connSub?.cancel();
    _eventSub = null;
    _connSub = null;
    _timer?.cancel();
    _started = false;
    _thread = null;
    _supportOnline = false;
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
    applyThread(SupportThread.tryParse(e['thread']));
    if (!_events.isClosed) _events.add(e);
  }

  /// Chat sahifasi o'qish belgisini yangilaganda ham shu yerga beradi.
  void applyThread(SupportThread? t, {bool? supportOnline}) {
    var changed = false;
    if (t != null) {
      final merged = _thread == null ? t : _thread!.merge(t);
      changed = merged.unread != _thread?.unread || merged.lastSeq != _thread?.lastSeq;
      _thread = merged;
    }
    if (supportOnline != null && supportOnline != _supportOnline) {
      _supportOnline = supportOnline;
      changed = true;
    }
    if (changed) notifyListeners();
  }

  Future<void> refresh() async {
    try {
      final s = await api.supportSummary();
      applyThread(SupportThread.tryParse(s['thread']), supportOnline: s['support_online'] == true);
    } catch (_) {
      // Zaxira so'rov keyingi siklda qayta urinadi.
    }
  }
}

/// Panel bo'ylab yagona nusxa.
final supportCenter = SupportCenter();

import 'dart:async';

import 'ws_client.dart';

/// Butun ilova uchun BITTA jonli kanal.
///
/// ┌─ MUAMMO (tuzatilgan) ─────────────────────────────────────────────┐
/// Panellarda jonli yangilanish EKRAN darajasida edi: buyurtmalar
/// sahifasi o'z WebSocket'ini ochardi, qolgan hamma narsa esa
/// (yon paneldagi hisoblagich, boshqaruv ko'rsatkichlari, kuryerlar
/// ro'yxati) 5-20 soniyalik so'rov sikliga tayanardi. Natijada
/// foydalanuvchi bir ekranda darhol, boshqasida esa kechikib
/// yangilanishni ko'rardi — va bu "real-time emas" degan taassurot
/// qoldirardi.
///
/// Har bir ekranga alohida WsClient qo'shish yechim EMAS: bitta
/// foydalanuvchi 3-4 ta soket ochardi, har biri o'z bileti bilan
/// (`POST /ws/ticket`) va o'z qayta ulanish sikli bilan.
///
/// `LiveBus` — bitta ulanish, ko'p tinglovchi. Ekranlar unga obuna
/// bo'ladi va kerakli hodisada o'zini yangilaydi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// POLLING O'CHIRILMAYDI, lekin ZAXIRAGA aylanadi: soket uzilgan
/// bo'lishi mumkin (tarmoq, server restart, quvvat tejash rejimi) va
/// bunday paytda ekran eskirib qolmasligi kerak. Tavsiya: soket
/// ulangan bo'lsa siyrak (60s), uzilgan bo'lsa tez-tez (10-15s)
/// so'rash.
class LiveBus {
  LiveBus({required this.ticketProvider, required this.urlBuilder});

  /// `POST /ws/ticket` — har ulanishdan oldin yangi bir martalik bilet.
  final Future<String> Function() ticketProvider;

  /// Biletdan to'liq `ws(s)://.../ws?ticket=...` manzilini quradi.
  final String Function(String ticket) urlBuilder;

  final _events = StreamController<Map<String, dynamic>>.broadcast();
  final _connection = StreamController<bool>.broadcast();
  WsClient? _ws;
  bool _connected = false;

  /// Serverdan kelgan hodisalar (`{"type": "...", ...}`).
  Stream<Map<String, dynamic>> get events => _events.stream;

  /// Ulanish holati o'zgarishlari — UI'da "jonli/oflayn" belgisi va
  /// zaxira polling tezligini tanlash uchun.
  Stream<bool> get connection => _connection.stream;

  bool get connected => _connected;

  /// Ulanadi. Ikki marta chaqirilsa ikkinchisi e'tiborsiz qoladi —
  /// shell qayta qurilganda tasodifan ikkinchi soket ochilmasin.
  void start() {
    if (_ws != null) return;
    _ws = WsClient(
      ticketProvider: ticketProvider,
      urlBuilder: urlBuilder,
      onEvent: (e) {
        if (!_events.isClosed) _events.add(e);
      },
      onStateChange: (c) {
        _connected = c;
        if (!_connection.isClosed) _connection.add(c);
      },
    )..connect();
  }

  /// Chiqishda (logout) va ilova yopilganda chaqiriladi.
  Future<void> stop() async {
    await _ws?.dispose();
    _ws = null;
    _connected = false;
    if (!_connection.isClosed) _connection.add(false);
  }

  /// Butunlay yopish — oqimlar ham yopiladi (obunachilar tugagach).
  Future<void> dispose() async {
    await stop();
    await _events.close();
    await _connection.close();
  }
}

/// Ekranni jonli hodisada va zaxira jadval bo'yicha yangilab turadi.
///
/// ┌─ NEGA ALOHIDA SINF ───────────────────────────────────────────────┐
/// "Hodisaga obuna bo'l + zaxira taymer qo'y + ulanish holati
/// o'zgarganda taymer tezligini almashtir + ikkalasini ham `dispose`
/// da tozala" — bu naqsh har bir ekranda takrorlanadi. Qo'lda
/// yozilganda esa doim BITTA qadam esdan chiqadi: odatda
/// `StreamSubscription` bekor qilinmaydi va ekran yopilgandan keyin
/// ham `setState` chaqirilib, ilova xato beradi.
/// └───────────────────────────────────────────────────────────────────┘
class LiveRefresher {
  LiveRefresher({
    required this.bus,
    required this.onRefresh,
    required this.types,
    this.connectedInterval = const Duration(seconds: 60),
    this.offlineInterval = const Duration(seconds: 10),
  });

  final LiveBus bus;

  /// Ma'lumotni qayta yuklash (odatda ekranning `_load` metodi).
  final Future<void> Function() onRefresh;

  /// Qaysi hodisa turlari shu ekranga tegishli. Bo'sh to'plam —
  /// HAMMASI (ehtiyot bo'ling: keraksiz qayta yuklashlar).
  final Set<String> types;

  /// Soket ishlayotgandagi zaxira so'rov oralig'i.
  final Duration connectedInterval;

  /// Soket uzilgandagi (yoki hali ulanmagandagi) oraliq.
  final Duration offlineInterval;

  StreamSubscription<Map<String, dynamic>>? _eventSub;
  StreamSubscription<bool>? _connSub;
  Timer? _timer;

  void start() {
    _eventSub = bus.events.listen((e) {
      final type = e['type'];
      if (types.isEmpty || (type is String && types.contains(type))) {
        onRefresh();
      }
    });
    // Ulanish holati o'zgarganda zaxira tezligi qayta tanlanadi.
    _connSub = bus.connection.listen((_) => _restartTimer());
    _restartTimer();
  }

  void _restartTimer() {
    _timer?.cancel();
    final interval = bus.connected ? connectedInterval : offlineInterval;
    _timer = Timer.periodic(interval, (_) => onRefresh());
  }

  /// Ekranning `dispose` metodidan MAJBURIY chaqiriladi.
  void dispose() {
    _eventSub?.cancel();
    _connSub?.cancel();
    _timer?.cancel();
  }
}

import 'dart:async';
import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';

// `tableText` — `api.dart` orqali `ondex_core` dan keladi.
import '../api.dart';
import '../models/activity.dart';
import '../models/waiter_order.dart';
import '../push.dart';

/// Butun ilovaning yagona holat manbai.
///
/// ┌─ NEGA BITTA STORE ────────────────────────────────────────────────┐
/// Ilgari hamma narsa bitta ekran ichida (`waiter_shell.dart`) turardi.
/// Endi to'rtta bo'lim (Buyurtmalar, Stollar, Bildirishnomalar, Profil)
/// AYNI ma'lumotga qaraydi. Har biri o'zicha so'rov yuborsa — to'rtta
/// polling, to'rtta WebSocket va bir-biriga mos kelmaydigan ro'yxatlar
/// bo'lardi. Shuning uchun tarmoq bilan FAQAT shu sinf gaplashadi,
/// ekranlar esa uni tinglaydi.
/// └───────────────────────────────────────────────────────────────────┘
class WaiterStore extends ChangeNotifier {
  WaiterStore();

  // ── Jonli ma'lumot ──
  List<WaiterOrder> _orders = [];
  String _restaurantName = '';
  String _userName = '';
  String _userPhone = '';
  bool _loading = true;
  bool _online = false;
  String? _error;
  final Set<String> _busy = {};

  // ── Qurilmada saqlanadigan ──
  List<FeedItem> _feed = [];
  List<ServedRecord> _history = [];
  bool _soundEnabled = true;

  /// Qaysi buyurtma uchun "tayyor" signali allaqachon berilgan.
  ///
  /// Busiz har 20 soniyalik polling AYNI buyurtma uchun qayta-qayta
  /// ovoz chalar va tasmani bir xil yozuv bilan to'ldirardi.
  final Set<String> _notifiedReady = {};
  bool _seeded = false;

  WsClient? _ws;
  Timer? _poll;
  Timer? _ticker;
  SharedPreferences? _prefs;

  /// Sessiya tugaganda chaqiriladi (401 yoki akkaunt o'chirilgan).
  VoidCallback? onSessionLost;

  // ── O'qish uchun ──
  List<WaiterOrder> get orders => _orders;
  String get restaurantName => _restaurantName;
  String get userName => _userName;
  String get userPhone => _userPhone;
  bool get loading => _loading;
  bool get online => _online;
  String? get error => _error;
  bool get soundEnabled => _soundEnabled;
  List<FeedItem> get feed => _feed;
  List<ServedRecord> get history => _history;

  bool isBusy(String orderId) => _busy.contains(orderId);

  /// Faol ro'yxatdan buyurtmani topadi.
  ///
  /// Tafsilot ekranlari buyurtmaning NUSXASINI saqlamaydi, har chizishda
  /// shu yerdan oladi — WS orqali holat o'zgarganda ekran o'zi
  /// yangilanadi. `null` = buyurtma yakunlangan va ro'yxatdan chiqqan.
  WaiterOrder? orderById(String id) {
    for (final o in _orders) {
      if (o.id == id) return o;
    }
    return null;
  }

  /// Bitta stolning faol buyurtmalari.
  List<WaiterOrder> ordersForTable(String label) =>
      _orders.where((o) => o.tableLabel == label).toList();

  /// Tayyor buyurtmalar — affitsiantning yagona shoshilinch ishi.
  /// Eng uzoq kutgani birinchi (taom sovimoqda).
  List<WaiterOrder> get readyOrders {
    final list = _orders.where((o) => o.isReady).toList();
    list.sort((a, b) {
      final ar = a.readyAt, br = b.readyAt;
      if (ar == null || br == null) return 0;
      return ar.compareTo(br);
    });
    return list;
  }

  /// Oshxonada tayyorlanayotgan va hali qabul qilinmagan buyurtmalar.
  List<WaiterOrder> get pendingOrders {
    final list = _orders.where((o) => !o.isReady).toList();
    list.sort((a, b) {
      final ac = a.createdAt, bc = b.createdAt;
      if (ac == null || bc == null) return 0;
      return ac.compareTo(bc);
    });
    return list;
  }

  List<TableGroup> get tables => TableGroup.from(_orders);

  int get unreadCount => _feed.where((f) => !f.read).length;

  /// Bugun yetkazilgan buyurtmalar (smena hisobi).
  List<ServedRecord> get todayHistory =>
      _history.where((h) => h.isToday).toList();

  int get todayTotalTiyin =>
      todayHistory.fold(0, (s, h) => s + h.totalTiyin);

  // ── Hayot sikli ──

  Future<void> start() async {
    api.onUnauthorized = _handleSessionLost;
    _prefs = await SharedPreferences.getInstance();
    _restoreLocal();

    // Restoran nomi va xodim ismi — faqat KO'RSATISH uchun. Ikkalasi
    // ham alohida `try` ichida: biri ishlamasa ikkinchisi va butun
    // ilova baribir ishlashi kerak, buyurtmalar ro'yxati bularga
    // umuman bog'liq emas.
    try {
      final me = await api.waiterMe();
      _restaurantName = me['restaurant_name'] as String? ?? '';
    } catch (_) {}
    try {
      final user = await api.me();
      _userName = user['name'] as String? ?? '';
      _userPhone = user['phone'] as String? ?? '';
    } catch (_) {}

    await refresh();
    _connectWs();

    // ┌─ IKKI QATLAMLI YANGILANISH ─────────────────────────────────┐
    // 1. WebSocket — darhol (odatiy holat);
    // 2. 20 soniyalik polling — ZAXIRA.
    //
    // Ikkinchisi shart: WebSocket uzilib qolsa (tunnel, Wi-Fi
    // almashuvi, telefon uxlashi) affitsiant buni SEZMASDAN eskirgan
    // ro'yxatga qarab turardi va taom sovib qolardi. `WsClient` qayta
    // ulanadi, lekin backoff 2 daqiqagacha o'sadi — o'sha oraliqda
    // polling qoplaydi.
    // └─────────────────────────────────────────────────────────────┘
    _poll = Timer.periodic(const Duration(seconds: 20), (_) => refresh());

    // "5 daqiqadan beri kutmoqda" yozuvi o'zi yangilanishi uchun.
    // Tarmoqqa chiqmaydi — faqat qayta chizadi.
    _ticker = Timer.periodic(const Duration(seconds: 30), (_) {
      if (_orders.any((o) => o.isReady)) notifyListeners();
    });

    await registerPush();
  }

  @override
  void dispose() {
    _ws?.dispose();
    _poll?.cancel();
    _ticker?.cancel();
    super.dispose();
  }

  Future<void> refresh() async {
    try {
      final raw = await api.orders();
      _applyOrders(raw.map(WaiterOrder.fromJson).toList());
      _error = null;
    } on ApiException catch (e) {
      // 401 ni `onUnauthorized` allaqachon ushlaydi.
      if (e.isUnauthorized) return;
      _error = e.message;
    } finally {
      _loading = false;
      notifyListeners();
    }
  }

  /// Yangi ro'yxatni qo'llaydi va "tayyor bo'ldi" hodisasini aniqlaydi.
  ///
  /// Aniqlash AYNAN shu yerda: WebSocket xabari yo'qolishi mumkin
  /// (uzilish, backoff), lekin polling baribir yangi ro'yxat olib
  /// keladi. Ovoz va tasma yozuvi ikkala yo'l uchun ham bitta joydan
  /// chiqadi — aks holda bir hodisa ikki marta bildirilardi.
  void _applyOrders(List<WaiterOrder> list) {
    final readyNow = list.where((o) => o.isReady).toList();

    if (!_seeded) {
      // Ilova endi ochildi: hozir tayyor turgan buyurtmalar uchun
      // ovoz chalinmaydi (ular ALLAQACHON tayyor edi). Aks holda har
      // ochilishda zal bo'ylab signal yangrardi.
      _notifiedReady.addAll(readyNow.map((o) => o.id));
      _seeded = true;
    } else {
      for (final o in readyNow) {
        if (_notifiedReady.add(o.id)) {
          _pushFeed(
            id: '${o.id}:ready',
            kind: 'ready',
            title: 'Buyurtma tayyor',
            body: '${tableText(o.tableLabel)} · ${o.shortNumber}',
            at: o.readyAt ?? DateTime.now(),
          );
          if (_soundEnabled) playReadySound();
        }
      }
    }

    // Ro'yxatdan tushib ketgan buyurtmalar uchun belgini tozalaymiz —
    // aks holda to'plam smena davomida cheksiz o'sib borardi.
    final aliveIds = list.map((o) => o.id).toSet();
    _notifiedReady.removeWhere((id) => !aliveIds.contains(id));

    _orders = list;
  }

  void _connectWs() {
    _ws = WsClient(
      ticketProvider: api.wsTicket,
      urlBuilder: (t) => wsUrl(t),
      onStateChange: (c) {
        _online = c;
        notifyListeners();
      },
      onEvent: (event) {
        final type = event['type'] as String?;

        // Akkaunt superadmin tomonidan o'chirildi — DARHOL chiqamiz.
        //
        // Server tokenni allaqachon bekor qilgan, ya'ni keyingi so'rov
        // baribir 401 bo'lardi. Lekin ilova so'rovni 20 soniyada bir
        // yuboradi — usiz odam o'chirilgan akkaunt bilan ekranga qarab
        // turaverardi.
        if (type == 'account_deleted') {
          _handleSessionLost();
          return;
        }

        if (type == 'new_order') {
          final table = event['table_label'] as String?;
          _pushFeed(
            id: 'new:${event['order_id']}',
            kind: 'new',
            title: 'Yangi buyurtma',
            body: table == null || table.isEmpty
                ? 'Yangi stol buyurtmasi keldi'
                : '${tableText(table)} buyurtma berdi',
            at: DateTime.now(),
          );
        }

        // Hodisa ICHIDAGI ma'lumotga tayanmaymiz — server javobi yagona
        // haqiqat manbai bo'lib qoladi (aks holda ikki manba ajralib
        // ketardi). "Tayyor" aniqlash `_applyOrders` da.
        if (type == 'new_order' || type == 'order_status') {
          refresh();
        }
      },
    )..connect();
  }

  // ── Harakat: yetkazish ──

  /// "Yetkazdim" — taom stolga olib borildi (terminal holat).
  ///
  /// Server tomonda holat mashinasi buni FAQAT `ready` holatidagi stol
  /// buyurtmasi uchun ruxsat beradi (`statemachine.go`), shuning uchun
  /// bu yerda qayta tekshirish shart emas — lekin tugma ham faqat
  /// `ready` da ko'rsatiladi, ya'ni odam imkonsiz amalni umuman
  /// ko'rmaydi.
  Future<String?> markServed(WaiterOrder order) async {
    if (_busy.contains(order.id)) return null;
    _busy.add(order.id);
    notifyListeners();
    try {
      await api.markServed(order.id);
      _recordServed(order);
      _pushFeed(
        id: '${order.id}:served',
        kind: 'served',
        title: 'Yetkazildi',
        body: '${tableText(order.tableLabel)} · ${order.shortNumber}',
        at: DateTime.now(),
      );
      await refresh();
      return null;
    } on ApiException catch (e) {
      return e.message;
    } finally {
      _busy.remove(order.id);
      notifyListeners();
    }
  }

  // ── Bildirishnomalar tasmasi ──

  void _pushFeed({
    required String id,
    required String kind,
    required String title,
    required String body,
    required DateTime at,
  }) {
    // Bir xil hodisa ikki kanaldan (WS + polling) kelishi mumkin —
    // `id` bo'yicha takror tashlanadi.
    if (_feed.any((f) => f.id == id)) return;
    _feed = [
      FeedItem(id: id, kind: kind, title: title, body: body, at: at, read: false),
      ..._feed,
    ];
    // Tasma cheksiz o'smasin — telefon xotirasi va JSON hajmi uchun.
    if (_feed.length > 60) _feed = _feed.sublist(0, 60);
    _saveFeed();
    notifyListeners();
  }

  void markFeedRead() {
    if (_feed.every((f) => f.read)) return;
    _feed = _feed.map((f) => f.copyWith(read: true)).toList();
    _saveFeed();
    notifyListeners();
  }

  void clearFeed() {
    _feed = [];
    _saveFeed();
    notifyListeners();
  }

  // ── Smena tarixi ──

  void _recordServed(WaiterOrder o) {
    if (_history.any((h) => h.orderId == o.id)) return;
    _history = [
      ServedRecord(
        orderId: o.id,
        shortNumber: o.shortNumber,
        tableLabel: o.tableLabel,
        totalTiyin: o.totalTiyin,
        itemCount: o.itemCount,
        servedAt: DateTime.now(),
      ),
      ..._history,
    ];
    if (_history.length > 200) _history = _history.sublist(0, 200);
    _saveHistory();
  }

  // ── Sozlamalar ──

  Future<void> setSoundEnabled(bool value) async {
    _soundEnabled = value;
    setPushSoundEnabled(value);
    await _prefs?.setBool(_kSound, value);
    notifyListeners();
  }

  // ── Sessiya ──

  Future<void> _handleSessionLost() async {
    _poll?.cancel();
    _ticker?.cancel();
    _ws?.dispose();
    onSessionLost?.call();
  }

  /// Chiqish — push tokeni ham uziladi.
  ///
  /// Token o'chirilmasa, chiqib ketgan affitsiantning telefoni keyingi
  /// xodimning bildirishnomalarini olishda davom etardi.
  Future<void> logout() async {
    await unregisterPush();
    try {
      await api.logout();
    } catch (_) {
      // Server javob bermasa ham lokal sessiya tozalanadi.
    }
  }

  // ── Lokal saqlash ──

  static const _kFeed = 'waiter_feed_v1';
  static const _kHistory = 'waiter_history_v1';
  static const _kSound = 'waiter_sound_enabled';

  void _restoreLocal() {
    final p = _prefs;
    if (p == null) return;

    _soundEnabled = p.getBool(_kSound) ?? true;
    setPushSoundEnabled(_soundEnabled);

    _feed = _decodeList(p.getString(_kFeed), FeedItem.fromJson);
    _history = _decodeList(p.getString(_kHistory), ServedRecord.fromJson);
  }

  /// Buzilgan JSON butun ilovani yiqitmasligi kerak — saqlangan yozuv
  /// shunchaki bezak, uning uchun kirish ekranida qulash mumkin emas.
  List<T> _decodeList<T>(String? raw, T? Function(Map<String, dynamic>) parse) {
    if (raw == null || raw.isEmpty) return [];
    try {
      final data = jsonDecode(raw);
      if (data is! List) return [];
      return data
          .whereType<Map>()
          .map((e) => parse(Map<String, dynamic>.from(e)))
          .whereType<T>()
          .toList();
    } catch (_) {
      return [];
    }
  }

  void _saveFeed() {
    _prefs?.setString(
      _kFeed,
      jsonEncode(_feed.map((f) => f.toJson()).toList()),
    );
  }

  void _saveHistory() {
    _prefs?.setString(
      _kHistory,
      jsonEncode(_history.map((h) => h.toJson()).toList()),
    );
  }
}

import 'dart:async';
import 'dart:math' as math;

import 'package:flutter/material.dart';

import '../api.dart';
import '../notification_center.dart';
import '../theme.dart';

part 'notifications/alert_models.dart';
part 'notifications/alert_widgets.dart';

/// "Bildirishnomalar" — `image/Bildirishnomalar.png` bo'yicha.
///
/// ┌─ HAMMASI HAQIQIY VA JONLI ────────────────────────────────────────┐
/// Xabarlar server hodisalaridan yasaladi: yangi buyurtma, bekor
/// qilingan buyurtma, karta to'lovi, kuryer xatosi, xodimlar
/// o'zgarishi, qabul qilinmagan buyurtma eslatmasi, kunlik hisobot,
/// platforma yangilanishi. Soxta/namuna xabar YO'Q.
///
/// Yangi xabar `NotificationCenter` orqali DARHOL ro'yxat boshiga
/// qo'shiladi (sahifani yangilash shart emas). Boshqa kompyuterda
/// "o'qildi" bosilsa, bu yerda ham belgi o'chadi.
/// └───────────────────────────────────────────────────────────────────┘
class NotificationsPage extends StatefulWidget {
  const NotificationsPage({
    super.key,
    this.center,
    this.onOpenOrders,
    this.onOpenStaff,
    this.onOpenStatistics,
    this.onOpenSettings,
  });

  /// Testlarda almashtiriladi; ilovada — panel bo'ylab yagona nusxa.
  final NotificationCenter? center;
  final VoidCallback? onOpenOrders;
  final VoidCallback? onOpenStaff;
  final VoidCallback? onOpenStatistics;
  final VoidCallback? onOpenSettings;

  @override
  State<NotificationsPage> createState() => _NotificationsPageState();
}

class _NotificationsPageState extends State<NotificationsPage> {
  NotificationCenter get _center => widget.center ?? notificationCenter;

  final _items = <AlertItem>[];
  final _search = TextEditingController();
  final _scroll = ScrollController();
  String _category = '';
  String _period = '';
  int _total = 0;
  Map<String, int> _byCategory = const {};
  int _pageUnread = 0;
  int _nextBefore = 0;
  int _latestSeq = 0;
  bool _loading = true;
  bool _loadingMore = false;
  bool _markingAll = false;
  String? _error;
  Timer? _debounce;
  int _requestId = 0;
  StreamSubscription<Map<String, dynamic>>? _incomingSub;
  StreamSubscription<Map<String, dynamic>>? _readSub;

  /// Ilovada markaz ishlab turadi va u aniq qiymat beradi.
  int get _unread => _center.started ? _center.unread : _pageUnread;

  bool get _hasFilters => _category.isNotEmpty || _period.isNotEmpty || _search.text.trim().isNotEmpty;

  @override
  void initState() {
    super.initState();
    _search.addListener(_onSearchChanged);
    _scroll.addListener(_onScroll);
    _incomingSub = _center.incoming.listen(_onIncoming);
    _readSub = _center.readEvents.listen(_onRead);
    _center.addListener(_onCenter);
    _load();
  }

  @override
  void dispose() {
    _debounce?.cancel();
    _incomingSub?.cancel();
    _readSub?.cancel();
    _center.removeListener(_onCenter);
    _search.dispose();
    _scroll.dispose();
    super.dispose();
  }

  void _onCenter() {
    if (mounted) setState(() {});
  }

  String _lastSearch = '';

  void _onSearchChanged() {
    final q = _search.text.trim();
    if (q == _lastSearch) {
      setState(() {});
      return;
    }
    _lastSearch = q;
    _debounce?.cancel();
    _debounce = Timer(const Duration(milliseconds: 350), _load);
    setState(() {});
  }

  void _onScroll() {
    if (!_scroll.hasClients) return;
    if (_scroll.position.pixels >= _scroll.position.maxScrollExtent - 240) _loadMore();
  }

  Future<void> _load() async {
    final id = ++_requestId;
    setState(() {
      _loading = _items.isEmpty;
      _error = null;
    });
    try {
      final res = await api.notifications(category: _category, period: _period, query: _search.text);
      if (!mounted || id != _requestId) return;
      final page = AlertPage.fromJson(res);
      setState(() {
        _items
          ..clear()
          ..addAll(page.items);
        _nextBefore = page.nextBefore;
        _total = page.total;
        _byCategory = page.byCategory;
        _pageUnread = page.unread;
        _latestSeq = math.max(_latestSeq, page.latestSeq);
        _loading = false;
      });
      _scheduleFill();
    } on ApiException catch (e) {
      if (!mounted || id != _requestId) return;
      setState(() {
        _loading = false;
        _error = e.message;
      });
    } catch (_) {
      if (!mounted || id != _requestId) return;
      setState(() {
        _loading = false;
        _error = 'Bildirishnomalarni yuklab bo\'lmadi. Internet aloqasini tekshirib, qayta urinib ko\'ring.';
      });
    }
  }

  Future<void> _loadMore() async {
    if (_loadingMore || _nextBefore <= 0 || _loading) return;
    final id = _requestId;
    setState(() => _loadingMore = true);
    try {
      final res = await api.notifications(
          category: _category, period: _period, query: _search.text, before: _nextBefore);
      if (!mounted || id != _requestId) return;
      final page = AlertPage.fromJson(res);
      setState(() {
        for (final it in page.items) {
          if (!_items.any((x) => x.id == it.id)) _items.add(it);
        }
        _nextBefore = page.nextBefore;
      });
      _scheduleFill();
    } catch (_) {
      // Keyingi aylantirishda qayta urinadi.
    } finally {
      if (mounted) setState(() => _loadingMore = false);
    }
  }

  /// Ro'yxat ekranni to'ldirmasa uni aylantirib bo'lmaydi va keyingi
  /// sahifa hech qachon yuklanmasdi — shunday holatda o'zi yuklanadi.
  void _scheduleFill() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted || !_scroll.hasClients || _nextBefore <= 0 || _loadingMore) return;
      if (_scroll.position.maxScrollExtent <= 0) _loadMore();
    });
  }

  bool _matches(AlertItem it) {
    if (_category.isNotEmpty && it.category != _category) return false;
    final q = _search.text.trim().toLowerCase();
    return q.isEmpty || '${it.title}\n${it.body}'.toLowerCase().contains(q);
  }

  /// Jonli yangi xabar — filtrga mos bo'lsa ro'yxatga, har holda hisobga.
  void _onIncoming(Map<String, dynamic> json) {
    final it = AlertItem.fromJson(json);
    if (!mounted || it.id.isEmpty || _items.any((x) => x.id == it.id)) return;
    setState(() {
      _latestSeq = math.max(_latestSeq, it.seq);
      if (_matches(it)) {
        _items.add(it);
        _items.sort((a, b) => b.seq.compareTo(a.seq));
      }
      _total += 1;
      _byCategory = {..._byCategory, it.category: (_byCategory[it.category] ?? 0) + 1};
      if (!it.read) _pageUnread += 1;
    });
  }

  void _onRead(Map<String, dynamic> e) {
    if (!mounted) return;
    final id = e['id'];
    final upTo = e['up_to_seq'];
    final unread = e['unread'];
    setState(() {
      for (var i = 0; i < _items.length; i++) {
        final it = _items[i];
        if (!it.read && ((id is String && it.id == id) || (upTo is num && it.seq <= upTo))) {
          _items[i] = it.copyWith(read: true);
        }
      }
      if (unread is num) _pageUnread = unread.toInt();
    });
  }

  void _markLocal(bool Function(AlertItem it) test) {
    for (var i = 0; i < _items.length; i++) {
      if (!_items[i].read && test(_items[i])) _items[i] = _items[i].copyWith(read: true);
    }
  }

  void _toast(String m) {
    if (!mounted) return;
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(m)));
  }

  Future<void> _open(AlertItem it) async {
    if (!it.read) {
      setState(() {
        _markLocal((x) => x.id == it.id);
        if (_pageUnread > 0) _pageUnread -= 1;
      });
      try {
        final unread = await _center.markRead(it.id);
        if (mounted) setState(() => _pageUnread = unread);
      } catch (_) {
        // Belgi serverda keyingi yuklashda tiklanadi; o'tish to'xtamaydi.
      }
    }
    final target = switch (it.kind) {
      'new_order' ||
      'order_cancelled' ||
      'order_waiting' ||
      'payment_received' ||
      'dispatch_failed' ||
      'courier_not_found' =>
        widget.onOpenOrders,
      'staff_added' || 'staff_status' || 'staff_position' => widget.onOpenStaff,
      'daily_report' => widget.onOpenStatistics,
      _ => null,
    };
    target?.call();
  }

  Future<void> _markAll() async {
    final upTo = math.max(_latestSeq, _center.latestSeq);
    if (upTo <= 0 || _markingAll) return;
    setState(() => _markingAll = true);
    try {
      final unread = await _center.markAllRead(upTo);
      if (!mounted) return;
      setState(() {
        _markLocal((x) => x.seq <= upTo);
        _pageUnread = unread;
      });
      _toast('Barcha bildirishnomalar o\'qildi');
    } on ApiException catch (e) {
      _toast(e.message);
    } catch (_) {
      _toast('O\'qilgan deb belgilab bo\'lmadi. Internet aloqasini tekshiring.');
    } finally {
      if (mounted) setState(() => _markingAll = false);
    }
  }

  void _clearFilters() {
    _debounce?.cancel();
    setState(() {
      _category = '';
      _period = '';
      _lastSearch = '';
      _search.clear();
    });
    _load();
  }

  @override
  Widget build(BuildContext context) {
    final unread = _unread;
    final hasUnread = unread > 0 || _items.any((i) => !i.read);
    return Padding(
      padding: const EdgeInsets.fromLTRB(24, 20, 24, 20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _AlertsHeader(
            busy: _markingAll,
            onReadAll: hasUnread ? _markAll : null,
            onRefresh: _load,
            onSettings: widget.onOpenSettings,
          ),
          const SizedBox(height: 16),
          Expanded(
            child: LayoutBuilder(builder: (context, box) {
              final list = _AlertListCard(
                items: _items,
                loading: _loading,
                loadingMore: _loadingMore,
                error: _error,
                hasFilters: _hasFilters,
                controller: _scroll,
                onOpen: _open,
                onRetry: _load,
                onClearFilters: _clearFilters,
              );
              final summary = _SummaryCard(total: _total, unread: unread, byCategory: _byCategory);
              final filters = _FilterCard(
                category: _category,
                period: _period,
                search: _search,
                onCategory: (v) {
                  setState(() => _category = v);
                  _load();
                },
                onPeriod: (v) {
                  setState(() => _period = v);
                  _load();
                },
              );
              if (box.maxWidth >= 1000 && box.maxHeight >= 480) {
                return Row(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Expanded(child: list),
                    const SizedBox(width: 16),
                    SizedBox(
                      width: box.maxWidth >= 1400 ? 360 : 320,
                      child: SingleChildScrollView(
                        primary: false,
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.stretch,
                          children: [summary, const SizedBox(height: 16), filters],
                        ),
                      ),
                    ),
                  ],
                );
              }
              return SingleChildScrollView(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    summary,
                    const SizedBox(height: 16),
                    filters,
                    const SizedBox(height: 16),
                    SizedBox(height: math.max(480.0, box.maxHeight), child: list),
                  ],
                ),
              );
            }),
          ),
        ],
      ),
    );
  }
}

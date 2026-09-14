part of 'statistics_page.dart';

/// "Barcha buyurtmalar" — Statistika sahifasidagi "So'nggi buyurtmalar"
/// blokining to'liq ko'rinishi: restoran ochilgandan beri BARCHA
/// buyurtmalar, eng yangisidan, xuddi o'sha jadval ko'rinishida.
///
/// ┌─ NEGA `api.orders()` EMAS ────────────────────────────────────────┐
/// U eng so'nggi 100 ta buyurtmani beradi — shuni "hammasi" deb
/// ko'rsatilsa, 101-buyurtmadan eskisi jimgina yo'qolardi. Bu yerda
/// server kursor bilan sahifalaydi (`GET /restaurants/{id}/orders/history`),
/// yuqoridagi raqamlar esa butun tarix bo'yicha serverda hisoblanadi.
/// └───────────────────────────────────────────────────────────────────┘
class _OrderHistoryView extends StatefulWidget {
  /// Davr filtri (yuqori panelda) va qaytish tugmasi shu obyektda.
  final StatisticsController controller;

  const _OrderHistoryView({required this.controller});

  @override
  State<_OrderHistoryView> createState() => _OrderHistoryViewState();
}

const _historyFilters = <(String, String)>[
  ('all', 'Barchasi'),
  ('in_progress', 'Jarayonda'),
  ('completed', 'Bajarilgan'),
  ('cancelled', 'Bekor qilingan'),
];

const _historyRowHeight = 46.0;

class _Lifetime {
  final int orders;
  final int completed;
  final int inProgress;
  final int cancelled;
  final int revenueTiyin;
  final int inProgressTiyin;
  final int? avgOrderTiyin;
  final DateTime? firstOrderAt;

  const _Lifetime({
    required this.orders,
    required this.completed,
    required this.inProgress,
    required this.cancelled,
    required this.revenueTiyin,
    required this.inProgressTiyin,
    required this.avgOrderTiyin,
    required this.firstOrderAt,
  });

  factory _Lifetime.fromJson(Map<String, dynamic> j) => _Lifetime(
        orders: _int(j['orders']),
        completed: _int(j['completed']),
        inProgress: _int(j['in_progress']),
        cancelled: _int(j['cancelled']),
        revenueTiyin: _int(j['revenue_tiyin']),
        inProgressTiyin: _int(j['in_progress_tiyin']),
        avgOrderTiyin: _intOrNull(j['avg_order_tiyin']),
        firstOrderAt: parseOrderAt(j['first_order_at']),
      );

  int countFor(String status) => switch (status) {
        'in_progress' => inProgress,
        'completed' => completed,
        'cancelled' => cancelled,
        _ => orders,
      };
}

class _OrderHistoryViewState extends State<_OrderHistoryView> {
  static const _pageSize = 30;

  final _scroll = ScrollController();
  final _orders = <Map<String, dynamic>>[];
  final _ids = <String>{};

  String _status = 'all';

  /// Hozir yuklangan davr (`null` — butun tarix).
  DateTimeRange? _range;
  _Lifetime? _summary;

  /// Keyingi sahifa kursori; `null` — ro'yxat tugagan.
  String? _cursor;
  bool _loading = true;
  bool _loadingMore = false;
  String? _error;
  String? _moreError;

  /// Filtr tez almashtirilganda eski javob yangisini bosib qolmasin.
  int _seq = 0;

  @override
  void initState() {
    super.initState();
    _range = widget.controller.historyRange;
    widget.controller.addListener(_onControllerChanged);
    _scroll.addListener(_onScroll);
    _reload(initial: true);
  }

  @override
  void dispose() {
    widget.controller.removeListener(_onControllerChanged);
    _scroll.dispose();
    super.dispose();
  }

  /// Yuqori paneldagi davr almashsa — ro'yxat va xulosa boshidan yuklanadi.
  void _onControllerChanged() {
    final r = widget.controller.historyRange;
    if (r == _range) return;
    _range = r;
    _reload();
  }

  static String _messageOf(Object e) => e is ApiException
      ? e.message
      : 'Buyurtmalarni yuklab bo\'lmadi. Internet aloqasini tekshirib, qayta urinib ko\'ring.';

  Future<void> _reload({bool initial = false}) async {
    final seq = ++_seq;
    void reset() {
      // Ro'yxat DARHOL tozalanadi: aks holda filtr almashib, so'rov
      // yiqilganda eski filtrning buyurtmalari yangi filtr ostida
      // qolib ketardi.
      _orders.clear();
      _ids.clear();
      _cursor = null;
      _loading = true;
      _loadingMore = false;
      _error = null;
      _moreError = null;
    }

    if (initial) {
      reset();
    } else {
      setState(reset);
    }
    try {
      final json = await api.orderHistory(
          status: _status, limit: _pageSize, from: _range?.start, to: _range?.end);
      if (!mounted || seq != _seq) return;
      setState(() {
        _append(json);
        final s = json['summary'];
        if (s is Map) _summary = _Lifetime.fromJson(Map<String, dynamic>.from(s));
        _loading = false;
      });
      if (_scroll.hasClients) _scroll.jumpTo(0);
      _fillViewport();
    } catch (e) {
      if (!mounted || seq != _seq) return;
      setState(() {
        _error = _messageOf(e);
        _loading = false;
      });
    }
  }

  void _append(Map<String, dynamic> json) {
    for (final o in _maps(json['orders'])) {
      final id = _str(o['id']);
      // Sahifalar chegarasida takror kelgan buyurtma ikki marta chizilmasin.
      if (id.isNotEmpty && !_ids.add(id)) continue;
      _orders.add(o);
    }
    final next = _str(json['next_cursor']);
    _cursor = next.isEmpty ? null : next;
  }

  Future<void> _loadMore() async {
    final cursor = _cursor;
    if (cursor == null || _loading || _loadingMore) return;
    final seq = _seq;
    setState(() {
      _loadingMore = true;
      _moreError = null;
    });
    try {
      final json = await api.orderHistory(
          status: _status,
          cursor: cursor,
          limit: _pageSize,
          from: _range?.start,
          to: _range?.end);
      if (!mounted || seq != _seq) return;
      setState(() {
        _append(json);
        _loadingMore = false;
      });
      _fillViewport();
    } catch (e) {
      if (!mounted || seq != _seq) return;
      setState(() {
        _moreError = _messageOf(e);
        _loadingMore = false;
      });
    }
  }

  void _onScroll() {
    if (!_scroll.hasClients || _moreError != null) return;
    if (_scroll.position.extentAfter < 600) _loadMore();
  }

  /// Katta ekranda birinchi sahifa ro'yxatni to'ldirmasa, scroll hodisasi
  /// umuman bo'lmaydi — keyingi sahifa shu yerda so'raladi.
  void _fillViewport() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted || !_scroll.hasClients || _moreError != null) return;
      if (_scroll.position.maxScrollExtent <= 0 && _cursor != null) _loadMore();
    });
  }

  void _setStatus(String status) {
    if (status == _status) return;
    _status = status;
    _reload();
  }

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(builder: (context, box) {
      final top = <Widget>[
        _buildHeader(),
        const SizedBox(height: _gap),
        _buildKpis(),
        const SizedBox(height: _gap),
      ];
      // Past yoki tor oynada jadvalga joy qolmaydi — butun sahifa
      // aylanadi, jadval esa o'z balandligini saqlaydi.
      if (box.maxWidth < 900 || box.maxHeight < 620) {
        return SingleChildScrollView(
          padding: _pagePadding,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [...top, SizedBox(height: 560, child: _buildTable())],
          ),
        );
      }
      return Padding(
        padding: _pagePadding,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [...top, Expanded(child: _buildTable())],
        ),
      );
    });
  }

  Widget _buildHeader() {
    final first = _summary?.firstOrderAt;
    final buttonStyle = OutlinedButton.styleFrom(
      foregroundColor: OnDexColors.ink,
      side: const BorderSide(color: OnDexColors.cardBorder),
      padding: const EdgeInsets.symmetric(horizontal: 12),
      minimumSize: const Size(0, 38),
      tapTargetSize: MaterialTapTargetSize.shrinkWrap,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
      textStyle: const TextStyle(fontSize: 13, fontWeight: FontWeight.w600),
    );
    return Row(
      children: [
        OutlinedButton.icon(
          onPressed: widget.controller.leaveHistory,
          style: buttonStyle,
          icon: const Icon(Icons.arrow_back_rounded, size: 17),
          label: Text(widget.controller.historyBackLabel),
        ),
        const SizedBox(width: 14),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              const Text('Barcha buyurtmalar',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                      fontSize: 19, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
              const SizedBox(height: 2),
              Text(
                  _range != null
                      ? 'Tanlangan davr: ${_rangeText(_range!.start, _range!.end)}'
                      : first == null
                          ? 'Restoran ochilgandan beri'
                          : 'Restoran ochilgandan beri · birinchi buyurtma: ${_dateLong(first)}',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontSize: 12.5, color: OnDexColors.inkDim)),
            ],
          ),
        ),
        const SizedBox(width: 10),
        OutlinedButton.icon(
          onPressed: _loading ? null : _reload,
          style: buttonStyle,
          icon: const Icon(Icons.refresh_rounded, size: 17),
          label: const Text('Yangilash'),
        ),
      ],
    );
  }

  Widget _buildKpis() {
    final s = _summary;
    // Birinchi yuklash yoki xato: joy saqlanadi, jadval sakramasin.
    if (s == null) return const SizedBox(height: 78);

    final note = s.inProgress > 0
        ? '+${formatSum(s.inProgressTiyin)} jarayonda (${_count(s.inProgress)})'
        : null;
    final reserve = note != null;
    final String ordersCaption;
    if (s.orders > 0) {
      ordersCaption = _range == null ? 'Restoran ochilgandan beri' : 'Tanlangan davrda';
    } else {
      ordersCaption = _range == null ? 'Hali buyurtma yo\'q' : 'Bu davrda buyurtma yo\'q';
    }
    return _ResponsiveWrap(
      minItemWidth: 180,
      spacing: 12,
      children: [
        _KpiCard(
          label: 'Jami buyurtmalar',
          value: _count(s.orders),
          icon: Icons.inventory_2_outlined,
          color: OnDexColors.info,
          background: OnDexColors.infoBg,
          delta: null,
          caption: ordersCaption,
          reserveNote: reserve,
          hint: 'Barcha buyurtmalar. To\'lanmagan karta buyurtmalari kirmaydi.',
        ),
        _KpiCard(
          label: 'Jami tushum',
          value: formatSum(s.revenueTiyin),
          icon: Icons.payments_outlined,
          color: OnDexColors.primary,
          background: OnDexColors.primaryTint,
          delta: null,
          caption: 'Bajarilgan buyurtmalardan',
          note: note,
          reserveNote: reserve,
          hint: 'Faqat bajarilgan (yetkazilgan yoki stolga berilgan) buyurtmalar. Hali '
              'yakunlanmaganlar summasi alohida ko\'rsatiladi va tushumga qo\'shilmaydi.',
        ),
        _KpiCard(
          label: 'O\'rtacha buyurtma',
          value: formatSum(s.avgOrderTiyin ?? 0),
          icon: Icons.trending_up_rounded,
          color: OnDexColors.purple,
          background: OnDexColors.purpleBg,
          delta: null,
          caption: s.avgOrderTiyin == null ? 'Bajarilgan buyurtma yo\'q' : 'Tushum ÷ bajarilganlar',
          reserveNote: reserve,
          hint: 'Jami tushum ÷ bajarilgan buyurtmalar soni. Bajarilgan buyurtma bo\'lmasa 0.',
        ),
        _KpiCard(
          label: 'Bajarilgan buyurtmalar',
          value: _count(s.completed),
          icon: Icons.check_circle_outline_rounded,
          color: OnDexColors.success,
          background: OnDexColors.successBg,
          delta: null,
          caption: '${_percent(s.completed, s.orders)} buyurtmalardan',
          reserveNote: reserve,
        ),
        _KpiCard(
          label: 'Bekor qilinganlar',
          value: _count(s.cancelled),
          icon: Icons.highlight_off_rounded,
          color: OnDexColors.danger,
          background: OnDexColors.dangerBg,
          delta: null,
          caption: '${_percent(s.cancelled, s.orders)} buyurtmalardan',
          reserveNote: reserve,
          hint: 'Restoran rad etgan yoki bekor qilingan buyurtmalar.',
        ),
      ],
    );
  }

  Widget _buildTable() {
    return _Panel(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          LayoutBuilder(builder: (context, c) {
            const title = _CardTitle(
              icon: Icons.receipt_long_rounded,
              title: 'Buyurtmalar ro\'yxati',
              hint: 'Eng yangisidan. Pastga aylantirilganda keyingi buyurtmalar o\'zi yuklanadi.',
            );
            final chips = Wrap(
              spacing: 6,
              runSpacing: 6,
              children: [
                for (final (value, label) in _historyFilters)
                  _HistoryFilterChip(
                    label: label,
                    count: _summary?.countFor(value),
                    selected: value == _status,
                    onTap: () => _setStatus(value),
                  ),
              ],
            );
            if (c.maxWidth >= 820) {
              return Row(
                children: [const Expanded(child: title), const SizedBox(width: 12), chips],
              );
            }
            return Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [title, const SizedBox(height: 8), chips],
            );
          }),
          const SizedBox(height: 10),
          Expanded(child: _buildList()),
        ],
      ),
    );
  }

  Widget _buildList() {
    if (_loading) {
      return const Center(
        child: SizedBox(width: 26, height: 26, child: CircularProgressIndicator(strokeWidth: 2.4)),
      );
    }
    if (_error != null) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.cloud_off_rounded, size: 34, color: OnDexColors.inkFaint),
            const SizedBox(height: 10),
            Text(_error!,
                textAlign: TextAlign.center,
                style: const TextStyle(fontSize: 13.5, color: OnDexColors.ink)),
            const SizedBox(height: 14),
            FilledButton(onPressed: _reload, child: const Text('Qayta urinish')),
          ],
        ),
      );
    }
    if (_orders.isEmpty) {
      final all = _status == 'all';
      return _EmptyText(_range == null
          ? (all ? 'Hali buyurtma yo\'q' : 'Bu holatdagi buyurtma yo\'q')
          : (all ? 'Bu davrda buyurtma yo\'q' : 'Bu davrda shu holatdagi buyurtma yo\'q'));
    }
    return LayoutBuilder(builder: (context, c) {
      final showCustomer = c.maxWidth >= 640;
      final showTime = c.maxWidth >= 440;
      return Column(
        children: [
          SizedBox(
            height: 28,
            child: _OrderTableHeader(
                showCustomer: showCustomer, showTime: showTime, fullDate: true),
          ),
          Expanded(
            child: ListView.builder(
              controller: _scroll,
              itemCount: _orders.length + 1,
              itemBuilder: (context, i) {
                if (i == _orders.length) return _buildFooter();
                return SizedBox(
                  height: _historyRowHeight,
                  child: _RecentOrderRow(
                    order: _orders[i],
                    showCustomer: showCustomer,
                    showTime: showTime,
                    last: false,
                    fullDate: true,
                  ),
                );
              },
            ),
          ),
        ],
      );
    });
  }

  Widget _buildFooter() {
    const height = 56.0;
    if (_loadingMore) {
      return const SizedBox(
        height: height,
        child: Center(
          child: SizedBox(width: 22, height: 22, child: CircularProgressIndicator(strokeWidth: 2)),
        ),
      );
    }
    if (_moreError != null) {
      return SizedBox(
        height: height,
        child: Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Flexible(
              child: Text(_moreError!,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontSize: 12.5, color: OnDexColors.danger)),
            ),
            TextButton(onPressed: _loadMore, child: const Text('Qayta urinish')),
          ],
        ),
      );
    }
    if (_cursor == null) {
      return SizedBox(
        height: height,
        child: Center(
          child: Text('Hammasi ko\'rsatildi · ${_count(_orders.length)}',
              style: const TextStyle(fontSize: 12.5, color: OnDexColors.inkFaint)),
        ),
      );
    }
    return const SizedBox(height: height);
  }
}

class _HistoryFilterChip extends StatelessWidget {
  final String label;

  /// `null` — xulosa hali kelmagan.
  final int? count;
  final bool selected;
  final VoidCallback onTap;

  const _HistoryFilterChip({
    required this.label,
    required this.count,
    required this.selected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final fg = selected ? Colors.white : OnDexColors.ink;
    return Material(
      color: selected ? OnDexColors.primary : OnDexColors.cardBg,
      shape: StadiumBorder(
          side: BorderSide(color: selected ? OnDexColors.primary : OnDexColors.cardBorder)),
      child: InkWell(
        customBorder: const StadiumBorder(),
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(label,
                  style: TextStyle(fontSize: 12.5, fontWeight: FontWeight.w600, color: fg)),
              if (count != null) ...[
                const SizedBox(width: 6),
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 1),
                  decoration: BoxDecoration(
                    color: selected
                        ? Colors.white.withValues(alpha: 0.22)
                        : OnDexColors.pageBg,
                    borderRadius: BorderRadius.circular(999),
                  ),
                  child: Text(_groupDigits(count!),
                      style: TextStyle(
                          fontSize: 11,
                          fontWeight: FontWeight.w700,
                          color: selected ? Colors.white : OnDexColors.inkDim)),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}

/// Taomlar ustuni: bir qatorda hammasi ("2x Lavash, 3x Cola"), sig'masa
/// kesiladi — to'liq ro'yxat sichqoncha ustida.
class _ItemsCell extends StatelessWidget {
  final Map<String, dynamic> order;

  const _ItemsCell({required this.order});

  @override
  Widget build(BuildContext context) {
    final lines = _itemLines(order);
    final text = Text(lines.isEmpty ? '—' : lines.join(', '),
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        style: const TextStyle(fontSize: 12.5, color: OnDexColors.ink));
    if (lines.isEmpty) return text;
    return Tooltip(
      message: lines.join('\n'),
      waitDuration: const Duration(milliseconds: 300),
      child: text,
    );
  }
}

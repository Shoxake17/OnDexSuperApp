import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:ondex_menu/ondex_menu.dart';

import '../api.dart';
import '../models/waiter_table.dart';
import '../state/waiter_store.dart';
import '../theme.dart';

/// Affitsiant ilovasining menyu ranglari — `theme.dart` tokenlaridan.
///
/// Menyu vidjetlari (kartochka, chip, "+/−") umumiy paketda
/// (`packages/ondex_menu`) va ranglarni mavzudan oladi. Bu yerda ular
/// affitsiant ilovasining QORONG'I palitrasiga moslanadi — ekran
/// ilovaning qolgan qismidan ajralib, yorug' bo'lak bo'lib qolmaydi.
const _waiterMenuStyle = MenuPalette(
  brand: kBrandColor,
  text: kInk,
  mutedText: kInkFaint,
  imageBackground: kSurfaceRaised,
  placeholderIcon: kInkGhost,
  controlBackground: kSurfaceRaised,
  controlForeground: kInk,
  controlDisabled: kInkGhost,
  chipBorder: kBorder,
  chipText: kInkDim,
  discountPrice: Color(0xFFFF6B6B),
);

/// Affitsiant stolga buyurtma kiritadi — MIJOZ ILOVASIDAGI MENYU bilan
/// bir xil tuzilishda, affitsiant ilovasining qorong'i mavzusida.
///
/// ┌─ NEGA ALOHIDA CHIZILMAGAN ────────────────────────────────────────┐
/// Taom kartochkasi, to'r, turkum chiplari va narx/aksiya hisobi
/// `packages/ondex_menu` da — mijoz ilovasining menyusi AYNAN shu kodni
/// ishlatadi. Affitsiant mehmon ko'radigan narx va aksiya lentasini
/// ko'radi; kartochka o'zgarsa, ikkala ilova birga o'zgaradi. Farq
/// faqat ranglarda ([MenuPalette]).
///
/// Oxirida SAVAT YO'Q: taom tanlanadi va "Oshxonaga yuborish" bosilishi
/// bilan buyurtma to'g'ridan-to'g'ri restoran paneliga tushadi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// ┌─ XAVFSIZLIK ──────────────────────────────────────────────────────┐
///  * Ilova faqat taom ID'si va miqdorini yuboradi. Restoranni, stolni
///    va narxni SERVER tekshiradi (`POST /waiter/orders`): stol shu
///    affitsiant restoraniga tegishli, taomlar shu restoranniki, joy
///    ochiq. Begona stol yoki begona restoran taomi rad etiladi.
///  * `idempotency_key` tanlov o'zgarganda yangilanadi, qayta bosishda
///    O'ZGARMAYDI — zaif tarmoqda oshxonaga ikkinchi buyurtma tushmaydi.
/// └───────────────────────────────────────────────────────────────────┘
class NewOrderScreen extends StatefulWidget {
  const NewOrderScreen({super.key, required this.table, required this.store});

  final WaiterTable table;
  final WaiterStore store;

  @override
  State<NewOrderScreen> createState() => _NewOrderScreenState();
}

class _NewOrderScreenState extends State<NewOrderScreen> {
  /// Serverdagi `PriceOrder` chegarasi bilan bir xil.
  static const _maxQty = 100;

  final _scroll = ScrollController();
  final _listKey = GlobalKey();
  final _searchCtrl = TextEditingController();
  final _active = ValueNotifier<String>('');
  final _sectionKeys = <String, GlobalKey>{};

  List<Map<String, dynamic>> _menu = const [];
  List<Map<String, dynamic>> _promos = const [];
  List<MenuSection> _sections = const [];
  bool _loading = true;
  String? _error;
  bool _searching = false;
  String _query = '';

  final Map<String, int> _qty = {};
  bool _sending = false;
  String _idempotencyKey = newIdempotencyKey();

  @override
  void initState() {
    super.initState();
    _scroll.addListener(_onScroll);
    _load();
  }

  @override
  void dispose() {
    _scroll.dispose();
    _searchCtrl.dispose();
    _active.dispose();
    super.dispose();
  }

  // ── Ma'lumot ──────────────────────────────────────────────────────

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final restaurantId = await widget.store.ensureRestaurantId();
      if (restaurantId.isEmpty) {
        _error = 'Akkaunt restoranga biriktirilmagan';
        return;
      }
      final menu = await api.menu(restaurantId);
      // Aksiyalar yiqilsa ham menyu ko'rinaveradi — ular faqat lenta va
      // chegirma narxiga ta'sir qiladi (mijoz menyusidagi bilan bir xil).
      var promos = const <Map<String, dynamic>>[];
      try {
        promos = await api.activePromotions(restaurantId);
      } catch (_) {}
      _menu = menu.where((p) => ((p['id'] as String?) ?? '').isNotEmpty).toList();
      _promos = promos;
      _sections = buildMenuSections(_menu);
      for (final s in _sections) {
        _sectionKeys.putIfAbsent(s.title, () => GlobalKey());
      }
      if (_sections.isNotEmpty) _active.value = _sections.first.title;
    } on ApiException catch (e) {
      _error = e.message;
    } catch (_) {
      _error = 'Menyuni yuklab bo\'lmadi';
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  int get _count => _qty.values.fold(0, (s, q) => s + q);

  /// Tanlovning CHEGIRMASIZ summasi — aksiyaning "minimal buyurtma
  /// summasi" shartini tekshirish uchun (`computeProductDiscount`).
  int get _rawSubtotal {
    var sum = 0;
    for (final p in _menu) {
      sum += ((p['price_tiyin'] as num?)?.toInt() ?? 0) * (_qty[p['id']] ?? 0);
    }
    return sum;
  }

  /// Taxminiy jami. Yakuniy summani SERVER hisoblaydi — tugmada "≈".
  int get _estimate {
    final subtotal = _rawSubtotal;
    var total = 0;
    for (final p in _menu) {
      final q = _qty[p['id']] ?? 0;
      if (q == 0) continue;
      final d = computeProductDiscount(p, _promos, cartSubtotalTiyin: subtotal);
      total += (d?.discountedPriceTiyin ?? (p['price_tiyin'] as num?)?.toInt() ?? 0) * q;
    }
    return total;
  }

  void _change(Map<String, dynamic> p, int delta) {
    final id = (p['id'] as String?) ?? '';
    if (id.isEmpty || p['available'] == false || _sending) return;
    var next = (_qty[id] ?? 0) + delta;
    if (next < 0) next = 0;
    if (next > _maxQty) next = _maxQty;
    setState(() {
      if (next == 0) {
        _qty.remove(id);
      } else {
        _qty[id] = next;
      }
      // Tanlov o'zgardi — bu YANGI buyurtma niyati, kalit ham yangi.
      _idempotencyKey = newIdempotencyKey();
    });
  }

  // ── Yuborish ──────────────────────────────────────────────────────

  Future<void> _send() async {
    if (_sending || _count == 0) return;
    final messenger = ScaffoldMessenger.of(context);
    final navigator = Navigator.of(context);
    setState(() => _sending = true);
    try {
      final created = await widget.store.placeOrder(
        table: widget.table,
        partySize: 0,
        quantities: Map.of(_qty),
        idempotencyKey: _idempotencyKey,
      );
      _qty.clear();
      messenger.showSnackBar(SnackBar(
        content: Text(
            'Oshxonaga yuborildi · ${created.shortNumber} · ${formatSum(created.totalTiyin)}'),
      ));
      if (mounted) navigator.pop(true);
    } on ApiException catch (e) {
      // Kalit O'ZGARMAYDI: qayta bosish o'sha buyurtmani qaytaradi,
      // yangisini yaratmaydi.
      messenger.showSnackBar(SnackBar(content: Text('Yuborilmadi: ${e.message}')));
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  Future<bool> _confirmDiscard() async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: kSurface,
        title: const Text('Tanlov bekor qilinsinmi?'),
        content: const Text('Tanlangan taomlar oshxonaga yuborilmagan.'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Qolish'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Bekor qilish', style: TextStyle(color: kDangerColor)),
          ),
        ],
      ),
    );
    return ok == true;
  }

  // ── Skroll: faol turkum ───────────────────────────────────────────

  double get _listTop {
    final box = _listKey.currentContext?.findRenderObject() as RenderBox?;
    return box == null || !box.hasSize ? 0 : box.localToGlobal(Offset.zero).dy;
  }

  /// Mijoz menyusidagi bilan bir xil qoida: ro'yxat tepasidan YUQORIDA
  /// boshlangan eng oxirgi bo'lim — joriy.
  void _onScroll() {
    if (!_scroll.hasClients || _sections.isEmpty) return;
    final line = _listTop + 8;
    var activeIndex = 0;
    for (var i = 0; i < _sections.length; i++) {
      final box = _sectionKeys[_sections[i].title]?.currentContext?.findRenderObject()
          as RenderBox?;
      if (box == null || !box.hasSize) continue;
      if (box.localToGlobal(Offset.zero).dy > line) {
        activeIndex = i - 1;
        break;
      }
      activeIndex = i;
    }
    if (activeIndex < 0) activeIndex = 0;
    if (_scroll.offset >= _scroll.position.maxScrollExtent - 4) {
      activeIndex = _sections.length - 1;
    }
    _active.value = _sections[activeIndex].title;
  }

  /// Chip bosilganda o'sha bo'limga suradi. Bo'lim hali qurilmagan
  /// bo'lsa (ekrandan uzoq) bir ekran surib qayta uriniladi.
  Future<void> _scrollToCategory(String category) async {
    final targetIndex = _sections.indexWhere((s) => s.title == category);
    if (targetIndex < 0) return;
    final currentIndex = _sections.indexWhere((s) => s.title == _active.value);
    final down = targetIndex >= (currentIndex < 0 ? 0 : currentIndex);

    for (var attempt = 0; attempt < 20; attempt++) {
      if (!mounted || !_scroll.hasClients) return;
      final box = _sectionKeys[category]?.currentContext?.findRenderObject() as RenderBox?;
      if (box != null && box.hasSize) {
        final dy = box.localToGlobal(Offset.zero).dy;
        final target = (_scroll.offset + dy - _listTop)
            .clamp(0.0, _scroll.position.maxScrollExtent);
        await _scroll.animateTo(target,
            duration: const Duration(milliseconds: 280), curve: Curves.easeOut);
        return;
      }
      final step = _scroll.position.viewportDimension * 0.9;
      final next = (down ? _scroll.offset + step : _scroll.offset - step)
          .clamp(0.0, _scroll.position.maxScrollExtent);
      if (next == _scroll.offset) return;
      _scroll.jumpTo(next);
      await WidgetsBinding.instance.endOfFrame;
    }
  }

  // ── Chizish ───────────────────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    // Ilovaning O'Z qorong'i mavzusi saqlanadi, faqat menyu vidjetlari
    // uchun rang uslubi qo'shiladi.
    final base = Theme.of(context);
    final theme = base.copyWith(extensions: [
      ...base.extensions.values.where((e) => e is! MenuPalette),
      _waiterMenuStyle,
    ]);

    return PopScope(
      canPop: _qty.isEmpty && !_sending,
      onPopInvokedWithResult: (didPop, _) async {
        if (didPop || _sending) return;
        if (await _confirmDiscard() && context.mounted) {
          setState(_qty.clear);
          Navigator.of(context).pop();
        }
      },
      child: AnnotatedRegion<SystemUiOverlayStyle>(
        value: SystemUiOverlayStyle.light,
        child: Theme(
          data: theme,
          child: Scaffold(
            key: const ValueKey('new-order-screen'),
            backgroundColor: kBackground,
            body: SafeArea(
              bottom: false,
              child: Column(
                children: [
                  _header(),
                  if (!_loading && _error == null && _query.isEmpty && _sections.length > 1)
                    _chips(),
                  Expanded(key: _listKey, child: _body()),
                ],
              ),
            ),
            bottomNavigationBar: _count == 0 ? null : _sendBar(),
          ),
        ),
      ),
    );
  }

  Widget _header() {
    final name = widget.store.restaurantName;
    return SizedBox(
      height: kToolbarHeight,
      child: Row(
        children: [
          IconButton(
            onPressed: () => Navigator.of(context).maybePop(),
            icon: const Icon(Icons.arrow_back, color: kInk),
            tooltip: 'Orqaga',
          ),
          Expanded(
            child: _searching
                ? TextField(
                    controller: _searchCtrl,
                    autofocus: true,
                    cursorColor: kBrandColor,
                    style: const TextStyle(color: kInk, fontSize: 16),
                    textInputAction: TextInputAction.search,
                    onChanged: (v) => setState(() => _query = v.trim().toLowerCase()),
                    decoration: const InputDecoration(
                      hintText: 'Taom qidirish',
                      hintStyle: TextStyle(color: kInkGhost),
                      filled: false,
                      border: InputBorder.none,
                      enabledBorder: InputBorder.none,
                      focusedBorder: InputBorder.none,
                    ),
                  )
                : Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Text(
                        name.isEmpty ? 'Menyu' : name,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                            fontSize: 17, fontWeight: FontWeight.bold, color: kInk),
                      ),
                      Text(
                        tableText(widget.table.displayLabel),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(fontSize: 12.5, color: kInkFaint),
                      ),
                    ],
                  ),
          ),
          IconButton(
            onPressed: _menu.isEmpty
                ? null
                : () => setState(() {
                      _searching = !_searching;
                      if (!_searching) {
                        _searchCtrl.clear();
                        _query = '';
                      }
                    }),
            icon: Icon(_searching ? Icons.close : Icons.search, color: kInk),
            tooltip: _searching ? 'Qidiruvni yopish' : 'Qidirish',
          ),
        ],
      ),
    );
  }

  Widget _chips() {
    return SizedBox(
      height: 46,
      child: ValueListenableBuilder<String>(
        valueListenable: _active,
        builder: (_, active, _) => ListView.separated(
          scrollDirection: Axis.horizontal,
          padding: const EdgeInsets.fromLTRB(16, 0, 16, 10),
          itemCount: _sections.length,
          separatorBuilder: (_, _) => const SizedBox(width: 8),
          itemBuilder: (_, i) {
            final title = _sections[i].title;
            return MenuCategoryChip(
              label: title,
              active: title == active,
              onTap: () => _scrollToCategory(title),
            );
          },
        ),
      ),
    );
  }

  Widget _body() {
    if (_loading) return const Center(child: CircularProgressIndicator());
    if (_error != null) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(32),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const Icon(Icons.cloud_off, size: 44, color: kInkGhost),
              const SizedBox(height: 12),
              Text(_error!,
                  textAlign: TextAlign.center, style: const TextStyle(color: kInkDim)),
              const SizedBox(height: 16),
              OutlinedButton(onPressed: _load, child: const Text('Qaytadan urinish')),
            ],
          ),
        ),
      );
    }
    if (_menu.isEmpty) {
      return const Center(
        child: Padding(
          padding: EdgeInsets.all(32),
          child: Text('Menyu hozircha bo\'sh', style: TextStyle(color: kInkDim)),
        ),
      );
    }

    final grid = productGridOf(context);
    final subtotal = _rawSubtotal;
    final index = PromotionIndex(_promos);

    if (_query.isNotEmpty) {
      final found = _menu
          .where((p) => ((p['name'] as String?) ?? '').toLowerCase().contains(_query))
          .toList();
      return CustomScrollView(
        slivers: [
          if (found.isEmpty)
            const SliverFillRemaining(
              hasScrollBody: false,
              child: Center(
                child: Text('Hech narsa topilmadi', style: TextStyle(color: kInkDim)),
              ),
            )
          else
            SliverPadding(
              padding: const EdgeInsets.fromLTRB(16, 16, 16, 0),
              sliver: SliverGrid.builder(
                gridDelegate: grid,
                itemCount: found.length,
                itemBuilder: (_, i) => _card(found[i], subtotal, index, keyPrefix: 'search_'),
              ),
            ),
          const SliverToBoxAdapter(child: SizedBox(height: 24)),
        ],
      );
    }

    return CustomScrollView(
      controller: _scroll,
      slivers: [
        for (final s in _sections) ...[
          SliverToBoxAdapter(
            child: Padding(
              key: _sectionKeys[s.title],
              padding: const EdgeInsets.fromLTRB(16, 20, 16, 8),
              child: Text(
                s.title,
                style: const TextStyle(
                    fontSize: 18, fontWeight: FontWeight.bold, color: kInk),
              ),
            ),
          ),
          SliverPadding(
            padding: const EdgeInsets.symmetric(horizontal: 16),
            sliver: SliverGrid.builder(
              gridDelegate: grid,
              itemCount: s.items.length,
              itemBuilder: (_, i) => _card(s.items[i], subtotal, index),
            ),
          ),
        ],
        const SliverToBoxAdapter(child: SizedBox(height: 24)),
      ],
    );
  }

  Widget _card(
    Map<String, dynamic> p,
    int subtotal,
    PromotionIndex index, {
    String keyPrefix = '',
  }) {
    final id = (p['id'] as String?) ?? '';
    final qty = _qty[id] ?? 0;
    final discount = computeProductDiscount(p, _promos, cartSubtotalTiyin: subtotal);
    return ProductCard(
      key: ValueKey('$keyPrefix$id'),
      product: p,
      qty: qty,
      discount: discount,
      promoted: index.covers(p, discount),
      onAdd: qty < _maxQty && !_sending ? () => _change(p, 1) : null,
      onRemove: _sending ? null : () => _change(p, -1),
    );
  }

  Widget _sendBar() {
    return Container(
      padding: const EdgeInsets.fromLTRB(16, 10, 16, 12),
      decoration: const BoxDecoration(
        color: kSurface,
        border: Border(top: BorderSide(color: kBorder)),
      ),
      child: SafeArea(
        top: false,
        child: SizedBox(
          height: 54,
          child: FilledButton(
            key: const ValueKey('new-order-submit'),
            onPressed: _sending ? null : _send,
            style: FilledButton.styleFrom(
              backgroundColor: kBrandColor,
              foregroundColor: Colors.white,
              disabledBackgroundColor: kBrandColor.withValues(alpha: 0.6),
              disabledForegroundColor: Colors.white,
              padding: const EdgeInsets.symmetric(horizontal: 14),
              shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
            ),
            child: _sending
                ? const Row(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      SizedBox(
                        width: 18,
                        height: 18,
                        child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                      ),
                      SizedBox(width: 10),
                      Text('Yuborilmoqda...',
                          style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
                    ],
                  )
                : Row(
                    children: [
                      Container(
                        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                        decoration: BoxDecoration(
                          color: Colors.white,
                          borderRadius: BorderRadius.circular(20),
                        ),
                        child: Text(
                          '$_count',
                          style: const TextStyle(
                              color: kBrandColor, fontWeight: FontWeight.bold, fontSize: 13),
                        ),
                      ),
                      const SizedBox(width: 12),
                      const Expanded(
                        child: Text(
                          'Oshxonaga yuborish',
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold),
                        ),
                      ),
                      const SizedBox(width: 8),
                      Text(
                        '≈ ${formatSum(_estimate)}',
                        style: const TextStyle(fontSize: 15, fontWeight: FontWeight.bold),
                      ),
                    ],
                  ),
          ),
        ),
      ),
    );
  }
}

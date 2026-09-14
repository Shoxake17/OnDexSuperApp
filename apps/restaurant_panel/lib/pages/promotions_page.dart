import 'package:flutter/material.dart';

import '../api.dart';
import '../theme.dart';
import '../widgets/date_range_dialog.dart';
import '../widgets/page_header.dart';

enum _PromoStatusFilter { all, active, scheduled, expired, paused }

const _pageSizeOptions = [10, 20, 50];

// Backend'dagi promotions.MaxNameLength/MaxDescriptionLength bilan bir xil.
const promotionMaxNameLength = 100;
const promotionMaxDescriptionLength = 200;

const _promoTypes = [
  'percent',
  'fixed_amount',
  'bogo',
  'bundle',
  'free_delivery',
  'loyalty',
];

/// Chegirma birligi tur bilan qat'iy belgilangan holatdagi ko'rsatkich —
/// tashqi ko'rinishi maydonlar bilan bir xil, lekin bosilmaydi.
class _FixedUnitBox extends StatelessWidget {
  final String label;

  const _FixedUnitBox({required this.label});

  @override
  Widget build(BuildContext context) => Container(
        height: 44,
        alignment: Alignment.center,
        decoration: BoxDecoration(
          color: OnDexColors.pageBg,
          borderRadius: BorderRadius.circular(10),
          border: Border.all(color: OnDexColors.cardBorder),
        ),
        child: Text(label,
            style: const TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w600,
                color: OnDexColors.inkDim)),
      );
}

/// Tur MAJBURLAYDIGAN chegirma birligi (null = birlikni restoran o'zi
/// tanlaydi).
///
/// ┌─ NEGA MAJBURIY ───────────────────────────────────────────────────┐
/// Avval "Aksiya turi" va "Chegirma birligi" mustaqil edi, ya'ni
/// "Summa orqali chegirma" turini tanlab, birlikni `%` da qoldirish
/// mumkin edi. Bunday yozuvni backend SUMMA (tiyin) deb, mijoz ilovasi
/// esa FOIZ deb o'qirdi: menyuda "-20%" ko'rinib, haqiqatda 20 tiyin
/// chegirma berilardi.
///
/// Endi tur birlikni belgilaydi va backend zid juftlikni umuman
/// qabul qilmaydi (routes_promotions.go). Qolgan turlarda (1+1,
/// to'plam, sodiqlik) birlik haqiqatan ham erkin.
/// └───────────────────────────────────────────────────────────────────┘
String? _unitForType(String type) => switch (type) {
      'percent' => 'percent',
      'fixed_amount' => 'amount',
      _ => null,
    };

String _typeLabel(String type) => switch (type) {
      'percent' => 'Foiz orqali chegirma',
      'fixed_amount' => 'Summa orqali chegirma',
      'bogo' => '1+1 aksiyasi',
      'bundle' => 'To\'plam aksiya',
      'free_delivery' => 'Yetkazib berish chegirmasi',
      'loyalty' => 'Sodiqlik bonusi',
      _ => type,
    };

(Color, Color) _typeStyle(String type) => switch (type) {
      'percent' => (OnDexColors.primary, OnDexColors.primaryTint),
      'fixed_amount' => (OnDexColors.purple, OnDexColors.purpleBg),
      'bogo' => (OnDexColors.pink, OnDexColors.pinkBg),
      'bundle' => (OnDexColors.success, OnDexColors.successBg),
      'free_delivery' => (OnDexColors.info, OnDexColors.infoBg),
      'loyalty' => (OnDexColors.teal, OnDexColors.tealBg),
      _ => (OnDexColors.inkDim, OnDexColors.pageBg),
    };

/// Aksiya holati — SAQLANMAYDI, doim StartAt/EndAt/Indefinite va Active
/// bayrog'iga qarab joriy vaqtga nisbatan jonli hisoblanadi (backend'dagi
/// Promotion.ComputeStatus bilan bir xil mantiq).
enum _PromoStatus { paused, scheduled, active, expired }

_PromoStatus _computeStatus(
    DateTime start, DateTime end, bool indefinite, bool active, DateTime now) {
  if (!active) return _PromoStatus.paused;
  if (now.isBefore(start)) return _PromoStatus.scheduled;
  if (!indefinite && now.isAfter(end)) return _PromoStatus.expired;
  return _PromoStatus.active;
}

(String, Color, Color) _statusStyle(_PromoStatus s) => switch (s) {
      _PromoStatus.active => (
          'Faol',
          OnDexColors.success,
          OnDexColors.successBg
        ),
      _PromoStatus.expired => (
          'Tugagan',
          OnDexColors.inkDim,
          OnDexColors.pageBg
        ),
      _PromoStatus.scheduled => (
          'Rejalashtirilgan',
          OnDexColors.info,
          OnDexColors.infoBg
        ),
      _PromoStatus.paused => (
          'To\'xtatilgan',
          OnDexColors.warning,
          OnDexColors.warningBg
        ),
    };

// Backend sanani to'liq RFC3339 vaqt belgisi sifatida qaytaradi (Go
// time.Time'ning standart JSON shakli, masalan "2026-08-02T10:00:00Z"),
// va endi vaqt qismi ham ma'noli (faqat sana emas) — shuning uchun
// to'g'ridan-to'g'ri DateTime.parse ishlatiladi, mahalliy vaqtga o'giriladi.
DateTime _parseAt(String? s) {
  if (s == null || s.isEmpty) return DateTime.now();
  try {
    return DateTime.parse(s).toLocal();
  } catch (_) {
    return DateTime.now();
  }
}

const _monthNamesShort = [
  'yan', 'fev', 'mar', 'apr', 'may', 'iyun',
  'iyul', 'avg', 'sen', 'okt', 'noy', 'dek', // ignore-format
];

String _fmtDate(DateTime d) =>
    '${d.day}-${_monthNamesShort[d.month - 1]}, ${d.year}';

String _fmtDateTime(DateTime d) =>
    '${_fmtDate(d)}, ${d.hour.toString().padLeft(2, '0')}:${d.minute.toString().padLeft(2, '0')}';

/// Aksiyalar sahifasi — image/aksiya.png namunasiga mos: 4 statistika
/// kartochkasi, qidiruv+turi+holat filtri, ustunli jadval va sahifalash.
/// "Foydalanish"/"Savdo" ustunlari ATAYLAB doim 0 ko'rsatiladi — bu HAQIQIY
/// qiymat (checkout hali aksiyalarni qo'llamaydi, shuning uchun hali
/// birorta ham aksiya "ishlatilmagan"), soxta raqam emas. Checkout
/// integratsiyasi alohida, keyingi bosqichdagi ish (ROADMAP'ga qarang).
class PromotionsPage extends StatefulWidget {
  const PromotionsPage({super.key});

  @override
  State<PromotionsPage> createState() => _PromotionsPageState();
}

class _PromotionsPageState extends State<PromotionsPage> {
  List<Map<String, dynamic>> _promotions = [];
  bool _loading = true;

  final _searchCtrl = TextEditingController();
  String _search = '';
  _PromoStatusFilter _statusFilter = _PromoStatusFilter.all;
  String? _typeFilter;
  int _page = 0;
  int _pageSize = _pageSizeOptions[0];

  bool _showForm = false;
  Map<String, dynamic>? _editing;

  @override
  void initState() {
    super.initState();
    _load();
    _searchCtrl.addListener(() {
      setState(() {
        _search = _searchCtrl.text.trim().toLowerCase();
        _page = 0;
      });
    });
  }

  @override
  void dispose() {
    _searchCtrl.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final list = await api.promotions();
      if (!mounted) return;
      setState(() {
        _promotions = list.cast<Map<String, dynamic>>();
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() => _loading = false);
      _snack('Xato: $e');
    }
  }

  void _snack(String msg) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg)));
  }

  void _openForm({Map<String, dynamic>? existing}) {
    setState(() {
      _editing = existing;
      _showForm = true;
    });
  }

  void _closeForm() {
    setState(() {
      _showForm = false;
      _editing = null;
    });
  }

  Future<void> _confirmDelete(Map<String, dynamic> p) async {
    final yes = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        icon: const Icon(Icons.warning_amber_rounded,
            color: Colors.red, size: 40),
        title: Text('"${p['name']}" aksiyasi o\'chirilsinmi?'),
        content: const Text('Bu amalni ortga qaytarib bo\'lmaydi.'),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('Yo\'q, bekor qilish')),
          FilledButton(
              style: FilledButton.styleFrom(backgroundColor: Colors.red),
              onPressed: () => Navigator.pop(ctx, true),
              child: const Text('Ha, o\'chirilsin')),
        ],
      ),
    );
    if (yes != true) return;
    try {
      await api.deletePromotion(p['id']);
      _snack('"${p['name']}" aksiyasi o\'chirildi');
      _load();
    } catch (e) {
      _snack('Xato: $e');
    }
  }

  void _showStats(Map<String, dynamic> p) {
    showDialog(
        context: context, builder: (_) => _PromoStatsDialog(promotion: p));
  }

  List<Map<String, dynamic>> get _filtered {
    final now = DateTime.now();
    var list = _promotions;
    if (_search.isNotEmpty) {
      list = list
          .where((p) =>
              (p['name'] as String? ?? '').toLowerCase().contains(_search))
          .toList();
    }
    if (_typeFilter != null) {
      list = list.where((p) => p['type'] == _typeFilter).toList();
    }
    if (_statusFilter != _PromoStatusFilter.all) {
      list = list.where((p) {
        final status = _computeStatus(
            _parseAt(p['start_at']),
            _parseAt(p['end_at']),
            p['indefinite'] == true,
            p['active'] != false,
            now);
        return switch (_statusFilter) {
          _PromoStatusFilter.active => status == _PromoStatus.active,
          _PromoStatusFilter.scheduled => status == _PromoStatus.scheduled,
          _PromoStatusFilter.expired => status == _PromoStatus.expired,
          _PromoStatusFilter.paused => status == _PromoStatus.paused,
          _PromoStatusFilter.all => true,
        };
      }).toList();
    }
    return list;
  }

  void _clearFilters() {
    setState(() {
      _searchCtrl.clear();
      _search = '';
      _typeFilter = null;
      _statusFilter = _PromoStatusFilter.all;
      _page = 0;
    });
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Center(child: CircularProgressIndicator());
    }

    final now = DateTime.now();
    final total = _promotions.length;
    var activeCount = 0;
    var expiredCount = 0;
    var totalSalesTiyin = 0;
    for (final p in _promotions) {
      final status = _computeStatus(
          _parseAt(p['start_at']),
          _parseAt(p['end_at']),
          p['indefinite'] == true,
          p['active'] != false,
          now);
      if (status == _PromoStatus.active) activeCount++;
      if (status == _PromoStatus.expired) expiredCount++;
      totalSalesTiyin += (p['sales_total_tiyin'] ?? 0) as int;
    }

    if (_showForm) {
      return SingleChildScrollView(
        padding: kPagePadding,
        child: _PromotionFormPage(
          key: ValueKey(_editing?['id'] ?? '__new__'),
          existing: _editing,
          onCancel: _closeForm,
          onSaved: ({
            bool keepOpen = false,
            List<String> stoppedNames = const [],
            List<String> adjustedNames = const [],
          }) {
            final base = _editing == null ? 'Aksiya yaratildi' : 'Saqlandi';
            final notes = <String>[
              if (stoppedNames.isNotEmpty)
                'to\'xtatildi: ${stoppedNames.join(', ')}',
              if (adjustedNames.isNotEmpty)
                'bu mahsulot(lar)ga endi tegishli emas: ${adjustedNames.join(', ')}',
            ];
            _snack(notes.isEmpty
                ? base
                : '$base. Bir xil mahsulot/turkumga to\'qnashgani uchun avtomatik '
                    '${notes.join('; ')}.');
            _load();
            if (!keepOpen) _closeForm();
          },
        ),
      );
    }

    final activePct = total == 0 ? 0.0 : activeCount / total * 100;
    final expiredPct = total == 0 ? 0.0 : expiredCount / total * 100;

    final filtered = _filtered;
    final totalPages =
        filtered.isEmpty ? 1 : ((filtered.length - 1) ~/ _pageSize) + 1;
    final page = _page.clamp(0, totalPages - 1);
    final pageItems = filtered.skip(page * _pageSize).take(_pageSize).toList();

    return RefreshIndicator(
      onRefresh: _load,
      child: SingleChildScrollView(
        padding: kPagePadding,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            _Header(
              onAdd: () => _openForm(),
            ),
            const SizedBox(height: 20),
            Row(
              children: [
                Expanded(
                  child: _StatCard(
                    label: 'Jami aksiyalar',
                    value: '$total',
                    footer: 'Faol: $activeCount ta',
                    footerColor: OnDexColors.success,
                    icon: Icons.sell_rounded,
                    color: OnDexColors.primary,
                    bg: OnDexColors.primaryTint,
                  ),
                ),
                const SizedBox(width: 16),
                Expanded(
                  child: _StatCard(
                    label: 'Faol aksiyalar',
                    value: '$activeCount',
                    footer: '${activePct.toStringAsFixed(1)}%',
                    footerColor: OnDexColors.success,
                    icon: Icons.trending_up_rounded,
                    color: OnDexColors.success,
                    bg: OnDexColors.successBg,
                  ),
                ),
                const SizedBox(width: 16),
                Expanded(
                  child: _StatCard(
                    label: 'Tugagan aksiyalar',
                    value: '$expiredCount',
                    footer: '${expiredPct.toStringAsFixed(1)}%',
                    footerColor: OnDexColors.inkDim,
                    icon: Icons.schedule_rounded,
                    color: OnDexColors.inkDim,
                    bg: OnDexColors.pageBg,
                  ),
                ),
                const SizedBox(width: 16),
                Expanded(
                  child: _StatCard(
                    label: 'Aksiya orqali savdo',
                    value: formatSum(totalSalesTiyin),
                    footer: totalSalesTiyin > 0
                        ? 'Checkout orqali haqiqiy'
                        : 'Hali qo\'llanilmagan',
                    footerColor: totalSalesTiyin > 0
                        ? OnDexColors.success
                        : OnDexColors.inkFaint,
                    icon: Icons.payments_rounded,
                    color: OnDexColors.purple,
                    bg: OnDexColors.purpleBg,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 20),
            _FilterRow(
              searchCtrl: _searchCtrl,
              statusFilter: _statusFilter,
              onStatusChanged: (s) => setState(() {
                _statusFilter = s;
                _page = 0;
              }),
              typeFilter: _typeFilter,
              onTypeChanged: (t) => setState(() {
                _typeFilter = t;
                _page = 0;
              }),
              onClear: _clearFilters,
            ),
            const SizedBox(height: 20),
            if (_promotions.isEmpty)
              const _EmptyState(
                  text:
                      'Hali aksiya yo\'q — yuqoridagi "Yangi aksiya yaratish" tugmasini bosing')
            else if (filtered.isEmpty)
              const _EmptyState(text: 'Filtrga mos aksiya topilmadi')
            else
              _PromoTable(
                promotions: pageItems,
                onEdit: (p) => _openForm(existing: p),
                onStats: _showStats,
                onDelete: _confirmDelete,
              ),
            if (filtered.isNotEmpty) ...[
              const SizedBox(height: 16),
              _PaginationRow(
                totalCount: filtered.length,
                page: page,
                totalPages: totalPages,
                pageSize: _pageSize,
                onPageChanged: (p) => setState(() => _page = p),
                onPageSizeChanged: (s) => setState(() {
                  _pageSize = s;
                  _page = 0;
                }),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Sarlavha
// ---------------------------------------------------------------------------

/// Sarlavha bloki — dizayn `widgets/page_header.dart` da (menyu va
/// boshqa bo'limlar bilan bitta manbadan).
class _Header extends StatelessWidget {
  final VoidCallback onAdd;
  const _Header({required this.onAdd});

  @override
  Widget build(BuildContext context) => PageHeader(
        title: 'Aksiyalar',
        subtitle: 'Aksiyalarni boshqarish va samaradorligini kuzatish',
        action: PageActionButton(
          icon: Icons.add_rounded,
          label: 'Yangi aksiya yaratish',
          onPressed: onAdd,
        ),
      );
}

// ---------------------------------------------------------------------------
// Statistika kartochkalari
// ---------------------------------------------------------------------------

class _StatCard extends StatelessWidget {
  final String label;
  final String value;
  final String footer;
  final Color footerColor;
  final IconData icon;
  final Color color;
  final Color bg;
  const _StatCard({
    required this.label,
    required this.value,
    required this.footer,
    required this.footerColor,
    required this.icon,
    required this.color,
    required this.bg,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(18),
      decoration: BoxDecoration(
        color: OnDexColors.cardBg,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Expanded(
                child: Text(label,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                        fontSize: 12.5,
                        color: OnDexColors.inkDim,
                        fontWeight: FontWeight.w600)),
              ),
              Container(
                width: 36,
                height: 36,
                decoration: BoxDecoration(color: bg, shape: BoxShape.circle),
                child: Icon(icon, size: 17, color: color),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Text(value,
              style: const TextStyle(
                  fontSize: 22,
                  fontWeight: FontWeight.w800,
                  color: OnDexColors.ink)),
          const SizedBox(height: 4),
          Text(footer,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                  fontSize: 11.5,
                  color: footerColor,
                  fontWeight: FontWeight.w600)),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Filtr qatori — bitta qatorda (Menyu sahifasidagi naqshga mos)
// ---------------------------------------------------------------------------

class _FilterRow extends StatelessWidget {
  final TextEditingController searchCtrl;
  final _PromoStatusFilter statusFilter;
  final ValueChanged<_PromoStatusFilter> onStatusChanged;
  final String? typeFilter;
  final ValueChanged<String?> onTypeChanged;
  final VoidCallback onClear;

  const _FilterRow({
    required this.searchCtrl,
    required this.statusFilter,
    required this.onStatusChanged,
    required this.typeFilter,
    required this.onTypeChanged,
    required this.onClear,
  });

  InputDecoration _dec(String hint, {IconData? icon}) => InputDecoration(
        isDense: true,
        hintText: hint,
        hintStyle: const TextStyle(fontSize: 13, color: OnDexColors.inkFaint),
        prefixIcon: icon == null
            ? null
            : Icon(icon, size: 19, color: OnDexColors.inkFaint),
        filled: true,
        fillColor: OnDexColors.cardBg,
        contentPadding:
            const EdgeInsets.symmetric(vertical: 12, horizontal: 14),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(10),
          borderSide: const BorderSide(color: OnDexColors.cardBorder),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(10),
          borderSide: const BorderSide(color: OnDexColors.primary),
        ),
        border: OutlineInputBorder(borderRadius: BorderRadius.circular(10)),
      );

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: TextField(
            controller: searchCtrl,
            decoration: _dec('Aksiya nomi bilan qidirish...',
                icon: Icons.search_rounded),
          ),
        ),
        const SizedBox(width: 12),
        SizedBox(
          width: 180,
          child: DropdownButtonFormField<_PromoStatusFilter>(
            initialValue: statusFilter,
            isExpanded: true,
            dropdownColor: OnDexColors.cardBg,
            decoration: _dec('Barcha holatlar'),
            items: const [
              DropdownMenuItem(
                  value: _PromoStatusFilter.all,
                  child: Text('Barcha holatlar')),
              DropdownMenuItem(
                  value: _PromoStatusFilter.active, child: Text('Faol')),
              DropdownMenuItem(
                  value: _PromoStatusFilter.scheduled,
                  child: Text('Rejalashtirilgan')),
              DropdownMenuItem(
                  value: _PromoStatusFilter.expired, child: Text('Tugagan')),
              DropdownMenuItem(
                  value: _PromoStatusFilter.paused,
                  child: Text('To\'xtatilgan')),
            ],
            onChanged: (v) => onStatusChanged(v ?? _PromoStatusFilter.all),
          ),
        ),
        const SizedBox(width: 12),
        SizedBox(
          width: 190,
          child: DropdownButtonFormField<String?>(
            initialValue: typeFilter,
            isExpanded: true,
            dropdownColor: OnDexColors.cardBg,
            decoration: _dec('Barcha turlar'),
            hint: const Text('Barcha turlar',
                style: TextStyle(fontSize: 13, color: OnDexColors.inkFaint)),
            items: [
              const DropdownMenuItem<String?>(
                  value: null, child: Text('Barcha turlar')),
              for (final t in _promoTypes)
                DropdownMenuItem<String?>(value: t, child: Text(_typeLabel(t))),
            ],
            onChanged: onTypeChanged,
          ),
        ),
        const SizedBox(width: 12),
        OutlinedButton.icon(
          onPressed: onClear,
          style: OutlinedButton.styleFrom(
            foregroundColor: OnDexColors.inkDim,
            side: const BorderSide(color: OnDexColors.cardBorder),
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 13),
            shape:
                RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
          ),
          icon: const Icon(Icons.refresh_rounded, size: 16),
          label: const Text('Filtrlarni tozalash'),
        ),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Jadval
// ---------------------------------------------------------------------------

const _colTypeWidth = 130.0;
const _colDiscountWidth = 90.0;
const _colPeriodWidth = 190.0;
const _colStatusWidth = 130.0;
const _colUsageWidth = 90.0;
const _colSalesWidth = 110.0;
const _colActionsWidth = 110.0;

class _PromoTable extends StatelessWidget {
  final List<Map<String, dynamic>> promotions;
  final void Function(Map<String, dynamic>) onEdit;
  final void Function(Map<String, dynamic>) onStats;
  final void Function(Map<String, dynamic>) onDelete;

  const _PromoTable({
    required this.promotions,
    required this.onEdit,
    required this.onStats,
    required this.onDelete,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: OnDexColors.cardBg,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      child: Column(
        children: [
          const _TableHeaderRow(),
          const Divider(height: 1, color: OnDexColors.cardBorder),
          for (var i = 0; i < promotions.length; i++) ...[
            if (i > 0) const Divider(height: 1, color: OnDexColors.cardBorder),
            _PromoRow(
              promotion: promotions[i],
              onEdit: () => onEdit(promotions[i]),
              onStats: () => onStats(promotions[i]),
              onDelete: () => onDelete(promotions[i]),
            ),
          ],
        ],
      ),
    );
  }
}

class _TableHeaderRow extends StatelessWidget {
  const _TableHeaderRow();

  @override
  Widget build(BuildContext context) {
    const style = TextStyle(
        fontSize: 12, fontWeight: FontWeight.w700, color: OnDexColors.inkDim);
    return const Padding(
      padding: EdgeInsets.symmetric(horizontal: 18, vertical: 14),
      child: Row(
        children: [
          Expanded(flex: 3, child: Text('Aksiya', style: style)),
          SizedBox(width: _colTypeWidth, child: Text('Turi', style: style)),
          SizedBox(
              width: _colDiscountWidth, child: Text('Chegirma', style: style)),
          SizedBox(
              width: _colPeriodWidth,
              child: Text('Amal qilish muddati', style: style)),
          SizedBox(width: _colStatusWidth, child: Text('Holati', style: style)),
          SizedBox(
              width: _colUsageWidth,
              child: Text('Foydalanish',
                  style: style, textAlign: TextAlign.center)),
          SizedBox(
              width: _colSalesWidth,
              child: Text('Savdo', style: style, textAlign: TextAlign.right)),
          SizedBox(
              width: _colActionsWidth,
              child: Text('Amallar', style: style, textAlign: TextAlign.right)),
        ],
      ),
    );
  }
}

class _PromoRow extends StatelessWidget {
  final Map<String, dynamic> promotion;
  final VoidCallback onEdit;
  final VoidCallback onStats;
  final VoidCallback onDelete;

  const _PromoRow({
    required this.promotion,
    required this.onEdit,
    required this.onStats,
    required this.onDelete,
  });

  @override
  Widget build(BuildContext context) {
    final imgUrl = promotion['image_url'] as String? ?? '';
    final name = promotion['name'] as String? ?? '';
    final desc = (promotion['description'] as String? ?? '').trim();
    final type = promotion['type'] as String? ?? '';
    final (typeColor, typeBg) = _typeStyle(type);
    final start = _parseAt(promotion['start_at']);
    final end = _parseAt(promotion['end_at']);
    final indefinite = promotion['indefinite'] == true;
    final active = promotion['active'] != false;
    final status =
        _computeStatus(start, end, indefinite, active, DateTime.now());
    final (statusLabel, statusColor, statusBg) = _statusStyle(status);
    final discountUnit = promotion['discount_unit'] as String? ?? 'percent';
    final discountValue = (promotion['discount_value'] ?? 0) as int;
    final usageCount = (promotion['usage_count'] ?? 0) as int;
    final salesTotalTiyin = (promotion['sales_total_tiyin'] ?? 0) as int;

    final String discountText = discountValue <= 0
        ? '—'
        : discountUnit == 'amount'
            ? formatSum(discountValue)
            : '$discountValue%';

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 10),
      child: Row(
        children: [
          Expanded(
            flex: 3,
            child: Row(
              children: [
                ClipRRect(
                  borderRadius: BorderRadius.circular(10),
                  child: SizedBox(
                    width: 44,
                    height: 44,
                    child: imgUrl.isEmpty
                        ? Container(
                            color: OnDexColors.pageBg,
                            child: const Icon(Icons.local_offer_rounded,
                                size: 18, color: OnDexColors.inkFaint),
                          )
                        : Image.network(
                            fullImageUrl(imgUrl),
                            fit: BoxFit.cover,
                            errorBuilder: (_, __, ___) => Container(
                              color: OnDexColors.pageBg,
                              child: const Icon(Icons.local_offer_rounded,
                                  size: 18, color: OnDexColors.inkFaint),
                            ),
                          ),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(name,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                              fontSize: 13.5,
                              fontWeight: FontWeight.w700,
                              color: OnDexColors.ink)),
                      if (desc.isNotEmpty)
                        Text(desc,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: const TextStyle(
                                fontSize: 12, color: OnDexColors.inkFaint)),
                    ],
                  ),
                ),
              ],
            ),
          ),
          SizedBox(
            width: _colTypeWidth,
            child: Align(
              alignment: Alignment.centerLeft,
              child: Container(
                padding:
                    const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                decoration: BoxDecoration(
                    color: typeBg, borderRadius: BorderRadius.circular(999)),
                child: Text(_typeLabel(type),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                        fontSize: 11,
                        fontWeight: FontWeight.w700,
                        color: typeColor)),
              ),
            ),
          ),
          SizedBox(
            width: _colDiscountWidth,
            child: Text(discountText,
                style: const TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w700,
                    color: OnDexColors.ink)),
          ),
          SizedBox(
            width: _colPeriodWidth,
            child: Text(
                indefinite
                    ? '${_fmtDate(start)} – cheksiz'
                    : '${_fmtDate(start)} – ${_fmtDate(end)}',
                style:
                    const TextStyle(fontSize: 12.5, color: OnDexColors.inkDim)),
          ),
          SizedBox(
            width: _colStatusWidth,
            child: Align(
              alignment: Alignment.centerLeft,
              child: Container(
                padding:
                    const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                decoration: BoxDecoration(
                    color: statusBg, borderRadius: BorderRadius.circular(999)),
                child: Text(statusLabel,
                    style: TextStyle(
                        fontSize: 11,
                        fontWeight: FontWeight.w700,
                        color: statusColor)),
              ),
            ),
          ),
          SizedBox(
            width: _colUsageWidth,
            child: Text('$usageCount marta',
                textAlign: TextAlign.center,
                style:
                    const TextStyle(fontSize: 12.5, color: OnDexColors.inkDim)),
          ),
          SizedBox(
            width: _colSalesWidth,
            child: Text(formatSum(salesTotalTiyin),
                textAlign: TextAlign.right,
                style:
                    const TextStyle(fontSize: 12.5, color: OnDexColors.inkDim)),
          ),
          SizedBox(
            width: _colActionsWidth,
            child: Row(
              mainAxisAlignment: MainAxisAlignment.end,
              children: [
                _RowIconButton(
                    icon: Icons.edit_rounded,
                    tooltip: 'Tahrirlash',
                    onTap: onEdit,
                    accent: true),
                _RowIconButton(
                    icon: Icons.bar_chart_rounded,
                    tooltip: 'Statistika',
                    onTap: onStats),
                PopupMenuButton<String>(
                  tooltip: 'Ko\'proq',
                  icon: const Icon(Icons.more_vert_rounded,
                      size: 18, color: OnDexColors.inkDim),
                  onSelected: (v) {
                    if (v == 'delete') onDelete();
                  },
                  itemBuilder: (context) => [
                    const PopupMenuItem(
                      value: 'delete',
                      child: Text('O\'chirish',
                          style: TextStyle(color: OnDexColors.danger)),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _RowIconButton extends StatelessWidget {
  final IconData icon;
  final String tooltip;
  final VoidCallback onTap;
  final bool accent;
  const _RowIconButton(
      {required this.icon,
      required this.tooltip,
      required this.onTap,
      this.accent = false});

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: tooltip,
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(8),
        child: Padding(
          padding: const EdgeInsets.all(6),
          child: Icon(icon,
              size: 17,
              color: accent ? OnDexColors.primary : OnDexColors.inkDim),
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Bo'sh holat
// ---------------------------------------------------------------------------

class _EmptyState extends StatelessWidget {
  final String text;
  const _EmptyState({required this.text});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(vertical: 48),
      decoration: BoxDecoration(
        color: OnDexColors.cardBg,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      child: Center(
          child: Text(text, style: const TextStyle(color: OnDexColors.inkDim))),
    );
  }
}

// ---------------------------------------------------------------------------
// Sahifalash — markazdan boshlanadi (Menyu sahifasidagi naqshga mos)
// ---------------------------------------------------------------------------

class _PaginationRow extends StatelessWidget {
  final int totalCount;
  final int page;
  final int totalPages;
  final int pageSize;
  final ValueChanged<int> onPageChanged;
  final ValueChanged<int> onPageSizeChanged;

  const _PaginationRow({
    required this.totalCount,
    required this.page,
    required this.totalPages,
    required this.pageSize,
    required this.onPageChanged,
    required this.onPageSizeChanged,
  });

  @override
  Widget build(BuildContext context) {
    final countText = Text('Jami $totalCount ta aksiya',
        style: const TextStyle(fontSize: 13, color: OnDexColors.inkDim));
    final pageControls = Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        _PageArrow(
            icon: Icons.chevron_left_rounded,
            enabled: page > 0,
            onTap: () => onPageChanged(page - 1)),
        for (final p in _visiblePages(page, totalPages))
          if (p == null)
            const Padding(
              padding: EdgeInsets.symmetric(horizontal: 6),
              child: Text('…', style: TextStyle(color: OnDexColors.inkFaint)),
            )
          else
            _PageNumberButton(
                number: p, selected: p == page, onTap: () => onPageChanged(p)),
        _PageArrow(
            icon: Icons.chevron_right_rounded,
            enabled: page < totalPages - 1,
            onTap: () => onPageChanged(page + 1)),
      ],
    );
    final pageSizeControl = Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        const Text('Har sahifada:',
            style: TextStyle(fontSize: 13, color: OnDexColors.inkDim)),
        const SizedBox(width: 8),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 10),
          decoration: BoxDecoration(
            color: OnDexColors.cardBg,
            border: Border.all(color: OnDexColors.cardBorder),
            borderRadius: BorderRadius.circular(8),
          ),
          child: DropdownButtonHideUnderline(
            child: DropdownButton<int>(
              value: pageSize,
              isDense: true,
              items: [
                for (final s in _pageSizeOptions)
                  DropdownMenuItem(value: s, child: Text('$s'))
              ],
              onChanged: (v) => onPageSizeChanged(v ?? _pageSizeOptions[0]),
            ),
          ),
        ),
      ],
    );

    return LayoutBuilder(builder: (context, c) {
      if (c.maxWidth < 720) {
        return Wrap(
          spacing: 16,
          runSpacing: 12,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: [countText, pageControls, pageSizeControl],
        );
      }
      return Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          Expanded(
              child: Align(alignment: Alignment.centerLeft, child: countText)),
          Center(child: pageControls),
          Expanded(
              child: Align(
                  alignment: Alignment.centerRight, child: pageSizeControl)),
        ],
      );
    });
  }
}

List<int?> _visiblePages(int page, int totalPages) {
  if (totalPages <= 7) return [for (var i = 0; i < totalPages; i++) i];
  final result = <int?>{0, totalPages - 1, page, page - 1, page + 1}
      .where((p) => p != null && p >= 0 && p < totalPages)
      .cast<int>()
      .toList()
    ..sort();
  final out = <int?>[];
  for (var i = 0; i < result.length; i++) {
    if (i > 0 && result[i] - result[i - 1] > 1) out.add(null);
    out.add(result[i]);
  }
  return out;
}

class _PageArrow extends StatelessWidget {
  final IconData icon;
  final bool enabled;
  final VoidCallback onTap;
  const _PageArrow(
      {required this.icon, required this.enabled, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: enabled ? onTap : null,
      borderRadius: BorderRadius.circular(8),
      child: Container(
        width: 32,
        height: 32,
        margin: const EdgeInsets.symmetric(horizontal: 2),
        decoration: BoxDecoration(
            border: Border.all(color: OnDexColors.cardBorder),
            borderRadius: BorderRadius.circular(8)),
        child: Icon(icon,
            size: 18, color: enabled ? OnDexColors.ink : OnDexColors.inkFaint),
      ),
    );
  }
}

class _PageNumberButton extends StatelessWidget {
  final int number;
  final bool selected;
  final VoidCallback onTap;
  const _PageNumberButton(
      {required this.number, required this.selected, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(8),
      child: Container(
        width: 32,
        height: 32,
        margin: const EdgeInsets.symmetric(horizontal: 2),
        alignment: Alignment.center,
        decoration: BoxDecoration(
          color: selected ? OnDexColors.primary : Colors.transparent,
          borderRadius: BorderRadius.circular(8),
        ),
        child: Text('${number + 1}',
            style: TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w700,
                color: selected ? Colors.white : OnDexColors.inkDim)),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Aksiya turi yordamchilari — jadval pill'i va formadagi tur tanlash
// kartochkalari uchun umumiy.
// ---------------------------------------------------------------------------

IconData _typeIcon(String type) => switch (type) {
      'percent' => Icons.percent_rounded,
      'fixed_amount' => Icons.timer_outlined,
      'bogo' => Icons.card_giftcard_rounded,
      'bundle' => Icons.shopping_basket_rounded,
      'free_delivery' => Icons.local_shipping_rounded,
      'loyalty' => Icons.workspace_premium_rounded,
      _ => Icons.local_offer_rounded,
    };

/// Aksiya turi tanlash kartochkalarida ko'rsatiladigan qisqa tavsif
/// (image/aksiyaqosh.png namunasidagi 6 ta karta subtitle'lariga mos).
String _typeShortDesc(String type) => switch (type) {
      'percent' => 'Mahsulot yoki buyurtmaga foiz orqali chegirma berish',
      'fixed_amount' => 'Belgilangan summaga chegirma berish',
      'bogo' => 'Bir mahsulot olganda, ikkinchisini bepul berish',
      'bundle' => 'Mahsulotlar to\'plamini maxsus narxda taklif qilish',
      'free_delivery' => 'Yetkazib berish xizmatiga chegirma berish',
      'loyalty' => 'Mijozlarga sodiqlik uchun bonus yoki chegirma berish',
      _ => '',
    };

// ---------------------------------------------------------------------------
// Statistika oynasi (bar-chart ikonkasi) — HAQIQIY, checkout'da shu aksiya
// tanlanganda promotions.Repository.IncrementUsage orqali oshadi.
// ---------------------------------------------------------------------------

class _PromoStatsDialog extends StatelessWidget {
  final Map<String, dynamic> promotion;
  const _PromoStatsDialog({required this.promotion});

  @override
  Widget build(BuildContext context) {
    final usageCount = (promotion['usage_count'] ?? 0) as int;
    final salesTotalTiyin = (promotion['sales_total_tiyin'] ?? 0) as int;
    return AlertDialog(
      title: Text(promotion['name'] as String? ?? ''),
      content: SizedBox(
        width: 340,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _StatRow(label: 'Foydalanilgan marta', value: '$usageCount'),
            _StatRow(label: 'Jami savdo', value: formatSum(salesTotalTiyin)),
            const SizedBox(height: 12),
            Text(
              usageCount > 0
                  ? 'Bu raqamlar mijoz ilovasidagi checkout\'da shu aksiya '
                      'haqiqatan qo\'llanilgan buyurtmalardan hisoblangan.'
                  : 'Bu aksiya hali birorta buyurtmada qo\'llanilmagan. '
                      'Mijoz shartlarga mos savat bilan buyurtma bersa, '
                      'checkout\'da avtomatik qo\'llaniladi.',
              style: const TextStyle(fontSize: 12.5, color: OnDexColors.inkDim),
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Yopish'))
      ],
    );
  }
}

class _StatRow extends StatelessWidget {
  final String label;
  final String value;
  const _StatRow({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        children: [
          Text(label,
              style: const TextStyle(fontSize: 13, color: OnDexColors.inkDim)),
          const Spacer(),
          Text(value,
              style: const TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w700,
                  color: OnDexColors.ink)),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Aksiya yaratish/tahrirlash — to'liq sahifa (Mahsulot qo'shish bilan bir
// xil uslubda, image/aksiya.png namunasida alohida yaratish sahifasi
// ko'rsatilmagan, shuning uchun mavjud "_ProductFormPage" naqshiga mos
// qilib qurilgan).
// ---------------------------------------------------------------------------

class _PromotionFormPage extends StatefulWidget {
  final Map<String, dynamic>? existing;
  final VoidCallback onCancel;
  // keepOpen=true — "Saqlab, davom etish" bosilganda: ro'yxat yangilanadi,
  // lekin forma yopilmaydi (yangi aksiya uchun tozalanadi).
  final void Function({
    bool keepOpen,
    List<String> stoppedNames,
    List<String> adjustedNames,
  }) onSaved;

  const _PromotionFormPage(
      {super.key,
      this.existing,
      required this.onCancel,
      required this.onSaved});

  @override
  State<_PromotionFormPage> createState() => _PromotionFormPageState();
}

class _PromotionFormPageState extends State<_PromotionFormPage> {
  late final TextEditingController _name;
  late final TextEditingController _description;
  late final TextEditingController _discountValue;
  late final TextEditingController _minOrderAmount;
  late final TextEditingController _maxDiscountAmount;
  late final TextEditingController _minPreviousOrders;
  String _type = 'percent';
  String _discountUnit = 'percent';
  DateTime _startAt = DateTime.now();
  DateTime _endAt = DateTime.now().add(const Duration(days: 30));
  bool _indefinite = false;
  bool _active = true;
  bool _appliesToProducts = false;
  bool _appliesToOrders = false;
  bool _appliesToCategories = false;
  List<String> _targetProductIds = [];
  List<String> _targetCategories = [];

  // Maqsadli mahsulot/turkum tanlovi UCHUN shu restoranning HAQIQIY
  // menyusi/turkumlaridan foydalaniladi (soxta/erkin matn emas).
  List<Map<String, dynamic>> _allProducts = [];
  List<String> _allCategories = [];
  bool _loadingTargets = true;

  String _existingImageUrl = '';
  bool _saving = false;
  String? _error;

  bool get _isEdit => widget.existing != null;

  @override
  void initState() {
    super.initState();
    final e = widget.existing;
    _name = TextEditingController(text: e?['name'] ?? '');
    _description = TextEditingController(text: e?['description'] ?? '');
    _type = (e?['type'] as String?) ?? 'percent';
    if (!_promoTypes.contains(_type)) _type = 'percent';
    _discountUnit = (e?['discount_unit'] as String?) ?? 'percent';
    if (_discountUnit != 'percent' && _discountUnit != 'amount') {
      _discountUnit = 'percent';
    }
    // Tur birlikni majburlasa — saqlangan (ehtimol zid) qiymat emas,
    // AYNAN shu birlik ishlatiladi: backend ham qiymatni shunday
    // hisoblaydi (`Promotion.EffectiveUnit`), ya'ni forma haqiqatni
    // ko'rsatadi.
    _discountUnit = _unitForType(_type) ?? _discountUnit;
    // discount_value backend'da: percent birligi uchun 1-100 (raqamning
    // o'zi), amount birligi uchun TIYIN (boshqa summa maydonlari — masalan
    // min_order_amount_tiyin — bilan bir xil konvensiya). Forma esa
    // foydalanuvchiga har doim so'mda ko'rsatadi, shuning uchun amount
    // birligida 100'ga bo'linadi (pastda _submit()da qaytadan ko'paytiriladi).
    final discountValue = (e?['discount_value'] ?? 0) as int;
    final discountValueSum =
        _discountUnit == 'amount' ? discountValue ~/ 100 : discountValue;
    _discountValue = TextEditingController(
        text: discountValueSum > 0 ? '$discountValueSum' : '');
    final minOrder = (e?['min_order_amount_tiyin'] ?? 0) as int;
    _minOrderAmount =
        TextEditingController(text: minOrder > 0 ? '${minOrder ~/ 100}' : '');
    final maxDiscount = (e?['max_discount_amount_tiyin'] ?? 0) as int;
    _maxDiscountAmount = TextEditingController(
        text: maxDiscount > 0 ? '${maxDiscount ~/ 100}' : '');
    final minPreviousOrders = (e?['min_previous_orders'] ?? 0) as int;
    _minPreviousOrders = TextEditingController(
        text: minPreviousOrders > 0 ? '$minPreviousOrders' : '');
    if (e != null) {
      _startAt = _parseAt(e['start_at'] as String?);
      _endAt = _parseAt(e['end_at'] as String?);
      _indefinite = e['indefinite'] == true;
      _active = e['active'] != false;
      _appliesToOrders = e['applies_to_orders'] == true;
      // Eski (o'zaro ziddiyatli) yozuvlarni ochishda tozalanadi — agar
      // applies_to_orders=true bo'lsa, mahsulot/kategoriya tanlovi baribir
      // backend'da e'tiborsiz qoldiriladi (yuqoridagi izohga qarang),
      // shuning uchun formada ham ko'rsatilmaydi.
      _appliesToProducts =
          !_appliesToOrders && e['applies_to_products'] == true;
      _appliesToCategories =
          !_appliesToOrders && e['applies_to_categories'] == true;
      _targetProductIds = _appliesToOrders
          ? []
          : ((e['target_product_ids'] as List?) ?? const [])
              .cast<String>()
              .toList();
      _targetCategories = _appliesToOrders
          ? []
          : ((e['target_categories'] as List?) ?? const [])
              .cast<String>()
              .toList();
    }
    _existingImageUrl = e?['image_url'] as String? ?? '';
    _loadTargets();
  }

  Future<void> _loadTargets() async {
    try {
      final products = await api.menu();
      final categories = await api.categories();
      if (!mounted) return;
      setState(() {
        _allProducts = products.cast<Map<String, dynamic>>();
        _allCategories = categories.cast<String>();
        _loadingTargets = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() => _loadingTargets = false);
    }
  }

  String _productName(String id) {
    for (final p in _allProducts) {
      if (p['id'] == id) return (p['name'] as String?) ?? id;
    }
    return id;
  }

  // Sana va vaqt BITTA oynada (Statistika sahifasidagi kalendar bilan bir
  // xil). Avval ikkita ketma-ket Material oynasi ochilardi: sana, keyin
  // soat siferblati.
  Future<void> _pickStartAt() async {
    final picked = await showOnDexDateTimePicker(
      context: context,
      initial: _startAt,
      firstDate: DateTime.now().subtract(const Duration(days: 365)),
      lastDate: DateTime.now().add(const Duration(days: 365 * 2)),
      title: 'Boshlanish sanasi va vaqti',
    );
    if (picked == null || !mounted) return;
    setState(() {
      _startAt = picked;
      if (!_endAt.isAfter(_startAt)) {
        _endAt = _startAt.add(const Duration(hours: 1));
      }
    });
  }

  Future<void> _pickEndAt() async {
    final picked = await showOnDexDateTimePicker(
      context: context,
      initial: _endAt.isBefore(_startAt) ? _startAt : _endAt,
      firstDate: _startAt,
      lastDate: DateTime.now().add(const Duration(days: 365 * 2)),
      title: 'Tugash sanasi va vaqti',
    );
    if (picked == null || !mounted) return;
    // Bir kunning o'zida boshlanishdan oldingi soat tanlanishi mumkin —
    // bunday aksiya hech qachon faol bo'lmasdi.
    if (!picked.isAfter(_startAt)) {
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(
          content: Text('Tugash vaqti boshlanish vaqtidan keyin bo\'lishi kerak')));
      return;
    }
    setState(() => _endAt = picked);
  }

  Future<void> _openTargetPicker() async {
    final result = await showDialog<(List<String>, List<String>)>(
      context: context,
      builder: (ctx) => _TargetPickerDialog(
        products: _allProducts,
        categories: _allCategories,
        showProducts: _appliesToProducts,
        showCategories: _appliesToCategories,
        selectedProductIds: _targetProductIds,
        selectedCategories: _targetCategories,
      ),
    );
    if (result == null || !mounted) return;
    setState(() {
      _targetProductIds = result.$1;
      _targetCategories = result.$2;
    });
  }

  Future<void> _submit({required bool keepOpen}) async {
    final name = _name.text.trim();
    if (name.isEmpty) {
      setState(() => _error = 'Aksiya nomini kiriting');
      return;
    }
    final discountValue = int.tryParse(_discountValue.text.trim()) ?? 0;
    if (_discountUnit == 'percent') {
      if (discountValue < 1 || discountValue > 100) {
        setState(() => _error = 'Chegirma foizini 1-100 oralig\'ida kiriting');
        return;
      }
    } else if (discountValue <= 0) {
      setState(() => _error = 'Chegirma summasini to\'g\'ri kiriting');
      return;
    }
    final minOrder = int.tryParse(_minOrderAmount.text.trim()) ?? 0;
    final maxDiscount = int.tryParse(_maxDiscountAmount.text.trim()) ?? 0;
    if (minOrder < 0 || maxDiscount < 0) {
      setState(() => _error = 'Summalar manfiy bo\'lishi mumkin emas');
      return;
    }
    final minPreviousOrders = int.tryParse(_minPreviousOrders.text.trim()) ?? 0;
    if (minPreviousOrders < 0) {
      setState(() =>
          _error = 'Oldingi buyurtmalar soni manfiy bo\'lishi mumkin emas');
      return;
    }
    if (!_indefinite && !_endAt.isAfter(_startAt)) {
      setState(() =>
          _error = 'Tugash sanasi boshlanish sanasidan keyin bo\'lishi kerak');
      return;
    }
    if (!_appliesToProducts && !_appliesToOrders && !_appliesToCategories) {
      setState(() => _error = 'Aksiya qo\'llaniladigan joyni tanlang');
      return;
    }
    if (_appliesToProducts && _targetProductIds.isEmpty) {
      setState(() => _error = 'Maqsadli mahsulotlarni tanlang');
      return;
    }
    if (_appliesToCategories && _targetCategories.isEmpty) {
      setState(() => _error = 'Maqsadli turkumlarni tanlang');
      return;
    }

    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      final resp = await api.savePromotion(
        id: widget.existing?['id'],
        name: name,
        description: _description.text.trim(),
        type: _type,
        discountUnit: _discountUnit,
        // amount birligida foydalanuvchi so'mda kiritadi, backend esa
        // (min_order_amount_tiyin/max_discount_amount_tiyin bilan bir xil
        // konvensiyada) tiyinni kutadi — shuning uchun 100'ga ko'paytiriladi
        // (percent'da esa qiymat 1-100 raqamning o'zi, o'zgarishsiz).
        discountValue:
            _discountUnit == 'amount' ? discountValue * 100 : discountValue,
        minOrderAmountTiyin: minOrder * 100,
        maxDiscountAmountTiyin: maxDiscount * 100,
        minPreviousOrders: _type == 'loyalty' ? minPreviousOrders : 0,
        startAt: _startAt.toUtc().toIso8601String(),
        endAt: _indefinite ? '' : _endAt.toUtc().toIso8601String(),
        indefinite: _indefinite,
        active: _active,
        appliesToProducts: _appliesToProducts,
        appliesToOrders: _appliesToOrders,
        appliesToCategories: _appliesToCategories,
        targetProductIds: _appliesToProducts ? _targetProductIds : const [],
        targetCategories: _appliesToCategories ? _targetCategories : const [],
        imageUrl: _existingImageUrl,
      );
      if (!mounted) return;
      final stopped = ((resp['stopped_promotion_names'] as List?) ?? const [])
          .cast<String>();
      final adjusted = ((resp['adjusted_promotion_names'] as List?) ?? const [])
          .cast<String>();
      widget.onSaved(
          keepOpen: keepOpen, stoppedNames: stopped, adjustedNames: adjusted);
      if (keepOpen) _resetForNext();
    } on ApiException catch (e) {
      setState(() => _error = e.message);
    } catch (_) {
      setState(() => _error = 'Xato yuz berdi, qayta urinib ko\'ring');
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  void _resetForNext() {
    setState(() {
      _name.clear();
      _description.clear();
      _type = 'percent';
      _discountUnit = 'percent';
      _discountValue.clear();
      _minOrderAmount.clear();
      _maxDiscountAmount.clear();
      _startAt = DateTime.now();
      _endAt = DateTime.now().add(const Duration(days: 30));
      _indefinite = false;
      _active = true;
      _appliesToProducts = false;
      _appliesToOrders = false;
      _appliesToCategories = false;
      _targetProductIds = [];
      _targetCategories = [];
    });
  }

  InputDecoration _formDec({String? hint, String? suffix, String? helper}) =>
      InputDecoration(
        isDense: true,
        hintText: hint,
        hintStyle: const TextStyle(fontSize: 13, color: OnDexColors.inkFaint),
        suffixText: suffix,
        helperText: helper,
        helperMaxLines: 2,
        filled: true,
        fillColor: OnDexColors.cardBg,
        contentPadding:
            const EdgeInsets.symmetric(vertical: 13, horizontal: 14),
        enabledBorder: OutlineInputBorder(
            borderRadius: BorderRadius.circular(10),
            borderSide: const BorderSide(color: OnDexColors.cardBorder)),
        focusedBorder: OutlineInputBorder(
            borderRadius: BorderRadius.circular(10),
            borderSide: const BorderSide(color: OnDexColors.primary)),
        border: OutlineInputBorder(borderRadius: BorderRadius.circular(10)),
      );

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          crossAxisAlignment: CrossAxisAlignment.center,
          children: [
            _CircleBackButton(onTap: _saving ? null : widget.onCancel),
            const SizedBox(width: 14),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                      _isEdit ? 'Aksiyani tahrirlash' : 'Yangi aksiya yaratish',
                      style: const TextStyle(
                          fontSize: 27,
                          fontWeight: FontWeight.w800,
                          color: OnDexColors.ink)),
                  const SizedBox(height: 4),
                  Text(
                      _isEdit
                          ? 'Aksiya ma\'lumotlarini yangilang'
                          : 'Restoranda yangi aksiya qo\'shing va sozlang',
                      style: const TextStyle(
                          fontSize: 14, color: OnDexColors.inkDim)),
                ],
              ),
            ),
          ],
        ),
        const SizedBox(height: 24),
        LayoutBuilder(builder: (context, c) {
          final stacked = c.maxWidth < 900;

          final left = _FormCard(
            title: 'Asosiy ma\'lumotlar',
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                _LabeledField(
                  label: 'Aksiya nomi',
                  required: true,
                  child: TextField(
                    controller: _name,
                    maxLength: promotionMaxNameLength,
                    onChanged: (_) => setState(() {}),
                    decoration: _formDec(hint: 'Masalan: 20% chegirma'),
                  ),
                ),
                const SizedBox(height: 6),
                _LabeledField(
                  label: 'Tavsif',
                  child: TextField(
                    controller: _description,
                    maxLines: 3,
                    maxLength: promotionMaxDescriptionLength,
                    onChanged: (_) => setState(() {}),
                    decoration: _formDec(hint: 'Aksiyaning qisqacha tavsifi'),
                  ),
                ),
                const SizedBox(height: 6),
                _LabeledField(
                  label: 'Aksiya turi',
                  required: true,
                  child: LayoutBuilder(builder: (context, tc) {
                    final columns =
                        tc.maxWidth >= 620 ? 3 : (tc.maxWidth >= 380 ? 2 : 1);
                    final cardWidth =
                        (tc.maxWidth - (columns - 1) * 12) / columns;
                    return Wrap(
                      spacing: 12,
                      runSpacing: 12,
                      children: [
                        for (final t in _promoTypes)
                          SizedBox(
                            width: cardWidth,
                            child: _TypeSelectCard(
                              type: t,
                              selected: _type == t,
                              onTap: _saving
                                  ? null
                                  : () => setState(() {
                                        _type = t;
                                        // Tur birlikni belgilaydi —
                                        // zid juftlik yaratib
                                        // bo'lmaydi (_unitForType).
                                        _discountUnit =
                                            _unitForType(t) ?? _discountUnit;
                                      }),
                            ),
                          ),
                      ],
                    );
                  }),
                ),
                const SizedBox(height: 16),
                _LabeledField(
                  label: 'Aksiya qo\'llaniladigan joy',
                  required: true,
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Wrap(
                        spacing: 24,
                        runSpacing: 8,
                        children: [
                          _CheckChip(
                            label: 'Mahsulotlar',
                            value: _appliesToProducts,
                            onChanged: _saving
                                ? null
                                : (v) => setState(() {
                                      _appliesToProducts = v;
                                      if (!v) _targetProductIds = [];
                                      // "Buyurtmalar" (butun savat) va aniq
                                      // mahsulot/turkum tanlovi BIRGA
                                      // ma'noga ega emas — backend
                                      // AppliesToOrders=true bo'lsa
                                      // TargetProductIDs'ni butunlay
                                      // e'tiborsiz qoldiradi va BUTUN
                                      // savatga chegirma beradi
                                      // (internal/promotions/apply.go,
                                      // eligibleLines). Shuning uchun
                                      // o'zaro istisno qilingan.
                                      if (v) _appliesToOrders = false;
                                    }),
                          ),
                          _CheckChip(
                            label: 'Buyurtmalar',
                            value: _appliesToOrders,
                            onChanged: _saving
                                ? null
                                : (v) => setState(() {
                                      _appliesToOrders = v;
                                      if (v) {
                                        _appliesToProducts = false;
                                        _appliesToCategories = false;
                                        _targetProductIds = [];
                                        _targetCategories = [];
                                      }
                                    }),
                          ),
                          _CheckChip(
                            label: 'Kategoriyalar',
                            value: _appliesToCategories,
                            onChanged: _saving
                                ? null
                                : (v) => setState(() {
                                      _appliesToCategories = v;
                                      if (!v) _targetCategories = [];
                                      if (v) _appliesToOrders = false;
                                    }),
                          ),
                        ],
                      ),
                      const SizedBox(height: 4),
                      Text(
                        _appliesToOrders
                            ? '"Buyurtmalar" tanlangan — chegirma BUTUN savatga beriladi, aniq mahsulot/kategoriya tanlanmaydi'
                            : 'Aksiya qayerda qo\'llanilishini tanlang (mahsulot va kategoriyani birga tanlash mumkin)',
                        style: const TextStyle(
                            fontSize: 11.5, color: OnDexColors.inkFaint),
                      ),
                    ],
                  ),
                ),
                if (_appliesToProducts || _appliesToCategories) ...[
                  const SizedBox(height: 16),
                  _LabeledField(
                    label: 'Maqsadli mahsulotlar / kategoriyalar',
                    required: true,
                    child: _loadingTargets
                        ? const SizedBox(
                            height: 46,
                            child: Center(
                                child: SizedBox(
                                    width: 18,
                                    height: 18,
                                    child: CircularProgressIndicator(
                                        strokeWidth: 2))))
                        : _TargetPickerField(
                            selectedCount: _targetProductIds.length +
                                _targetCategories.length,
                            onTap: _saving ? null : _openTargetPicker,
                          ),
                  ),
                  if (_targetProductIds.isNotEmpty ||
                      _targetCategories.isNotEmpty) ...[
                    const SizedBox(height: 8),
                    Wrap(
                      spacing: 6,
                      runSpacing: 6,
                      children: [
                        for (final id in _targetProductIds)
                          _TargetChip(
                            label: _productName(id),
                            onRemove: _saving
                                ? null
                                : () => setState(
                                    () => _targetProductIds.remove(id)),
                          ),
                        for (final cat in _targetCategories)
                          _TargetChip(
                            label: cat,
                            onRemove: _saving
                                ? null
                                : () => setState(
                                    () => _targetCategories.remove(cat)),
                          ),
                      ],
                    ),
                  ],
                ],
              ],
            ),
          );

          final discountCard = _FormCard(
            title: 'Chegirma sozlamalari',
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                _LabeledField(
                  label: 'Chegirma qiymati',
                  required: true,
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Expanded(
                        child: TextField(
                          controller: _discountValue,
                          keyboardType: TextInputType.number,
                          decoration: _formDec(
                              hint:
                                  _discountUnit == 'percent' ? '20' : '50000'),
                        ),
                      ),
                      const SizedBox(width: 10),
                      // Birlik TURGA bog'liq: "Foiz orqali chegirma" —
                      // faqat %, "Summa orqali chegirma" — faqat so'm.
                      // Bunday turlarda tanlov ko'rsatilmaydi (o'zgartirib
                      // bo'lmaydigan dropdown ko'rsatish faqat
                      // chalkashtiradi), qolganlarida esa erkin tanlanadi.
                      SizedBox(
                        width: 92,
                        child: _unitForType(_type) != null
                            ? _FixedUnitBox(
                                label:
                                    _discountUnit == 'percent' ? '%' : 'so\'m')
                            : DropdownButtonFormField<String>(
                                key: ValueKey('unit-$_discountUnit'),
                                initialValue: _discountUnit,
                                isExpanded: true,
                                dropdownColor: OnDexColors.cardBg,
                                decoration: _formDec(),
                                items: const [
                                  DropdownMenuItem(
                                      value: 'percent', child: Text('%')),
                                  DropdownMenuItem(
                                      value: 'amount', child: Text('so\'m')),
                                ],
                                onChanged: _saving
                                    ? null
                                    : (v) => setState(
                                        () => _discountUnit = v ?? 'percent'),
                              ),
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                    switch (_type) {
                      'percent' =>
                        'Foiz (1-100). Tanlangan turga ko\'ra birlik — %',
                      'fixed_amount' =>
                        'Aniq summa (so\'m). Tanlangan turga ko\'ra birlik — so\'m',
                      _ => 'Foiz (%) yoki aniq summa kiriting',
                    },
                    style: const TextStyle(
                        fontSize: 11.5, color: OnDexColors.inkFaint)),
                const SizedBox(height: 16),
                _LabeledField(
                  label: 'Minimal buyurtma summasi (ixtiyoriy)',
                  child: TextField(
                    controller: _minOrderAmount,
                    keyboardType: TextInputType.number,
                    decoration: _formDec(hint: '0', suffix: 'so\'m'),
                  ),
                ),
                const SizedBox(height: 4),
                const Text('Aksiya amal qilishi uchun minimal summa',
                    style:
                        TextStyle(fontSize: 11.5, color: OnDexColors.inkFaint)),
                const SizedBox(height: 16),
                _LabeledField(
                  label: 'Maksimal chegirma (ixtiyoriy)',
                  child: TextField(
                    controller: _maxDiscountAmount,
                    keyboardType: TextInputType.number,
                    decoration: _formDec(hint: '0', suffix: 'so\'m'),
                  ),
                ),
                const SizedBox(height: 4),
                const Text('Bir buyurtmaga maksimal chegirma miqdori',
                    style:
                        TextStyle(fontSize: 11.5, color: OnDexColors.inkFaint)),
                if (_type == 'loyalty') ...[
                  const SizedBox(height: 16),
                  _LabeledField(
                    label: 'Sodiqlik sharti',
                    child: TextField(
                      controller: _minPreviousOrders,
                      keyboardType: TextInputType.number,
                      decoration: _formDec(hint: '0', suffix: 'marta'),
                    ),
                  ),
                  const SizedBox(height: 4),
                  const Text(
                      'Mijoz kamida shuncha marta shu restorandan buyurtma '
                      'bergan bo\'lishi kerak (0 = shart yo\'q)',
                      style: TextStyle(
                          fontSize: 11.5, color: OnDexColors.inkFaint)),
                ],
              ],
            ),
          );

          final periodCard = _FormCard(
            title: 'Faollik davri',
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Expanded(
                      child: _LabeledField(
                        label: 'Boshlanish sanasi',
                        required: true,
                        child: _DateTimeBox(
                            dateTime: _startAt,
                            onTap: _saving ? null : _pickStartAt),
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: _LabeledField(
                        label: 'Tugash sanasi',
                        required: !_indefinite,
                        child: _DateTimeBox(
                          dateTime: _endAt,
                          onTap: (_saving || _indefinite) ? null : _pickEndAt,
                          disabled: _indefinite,
                        ),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 12),
                _CheckChip(
                  label: 'Cheksiz muddatga qoldirish',
                  value: _indefinite,
                  onChanged:
                      _saving ? null : (v) => setState(() => _indefinite = v),
                ),
              ],
            ),
          );

          final statusCard = _FormCard(
            title: 'Aksiya holati',
            child: Row(
              children: [
                Switch(
                  value: _active,
                  activeTrackColor: OnDexColors.primary,
                  onChanged:
                      _saving ? null : (v) => setState(() => _active = v),
                ),
                const SizedBox(width: 6),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(_active ? 'Faol' : 'Nofaol',
                          style: const TextStyle(
                              fontSize: 13.5,
                              fontWeight: FontWeight.w700,
                              color: OnDexColors.ink)),
                      const SizedBox(height: 2),
                      const Text('Aksiya yaratilgandan so\'ng faol bo\'ladi',
                          style: TextStyle(
                              fontSize: 11.5, color: OnDexColors.inkFaint)),
                    ],
                  ),
                ),
              ],
            ),
          );

          final right = Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              discountCard,
              const SizedBox(height: 16),
              periodCard,
              const SizedBox(height: 16),
              statusCard
            ],
          );

          if (stacked) {
            return Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [left, const SizedBox(height: 16), right],
            );
          }
          return Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(flex: 3, child: left),
              const SizedBox(width: 20),
              Expanded(flex: 2, child: right),
            ],
          );
        }),
        if (_error != null) ...[
          const SizedBox(height: 16),
          Text(_error!, style: const TextStyle(color: OnDexColors.danger)),
        ],
        const SizedBox(height: 24),
        Wrap(
          alignment: WrapAlignment.spaceBetween,
          spacing: 12,
          runSpacing: 12,
          children: [
            OutlinedButton(
              onPressed: _saving ? null : widget.onCancel,
              style: OutlinedButton.styleFrom(
                foregroundColor: OnDexColors.inkDim,
                side: const BorderSide(color: OnDexColors.cardBorder),
                padding:
                    const EdgeInsets.symmetric(horizontal: 26, vertical: 18),
                textStyle: const TextStyle(
                    fontSize: 14.5, fontWeight: FontWeight.w700),
                shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(10)),
              ),
              child: const Text('Bekor qilish'),
            ),
            Wrap(
              spacing: 12,
              runSpacing: 12,
              children: [
                if (!_isEdit)
                  OutlinedButton(
                    onPressed: _saving ? null : () => _submit(keepOpen: true),
                    style: OutlinedButton.styleFrom(
                      foregroundColor: OnDexColors.ink,
                      side: const BorderSide(color: OnDexColors.cardBorder),
                      padding: const EdgeInsets.symmetric(
                          horizontal: 26, vertical: 18),
                      textStyle: const TextStyle(
                          fontSize: 14.5, fontWeight: FontWeight.w700),
                      shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(10)),
                    ),
                    child: const Text('Saqlab, davom etish'),
                  ),
                FilledButton(
                  onPressed: _saving ? null : () => _submit(keepOpen: false),
                  style: FilledButton.styleFrom(
                    padding: const EdgeInsets.symmetric(
                        horizontal: 32, vertical: 18),
                    textStyle: const TextStyle(
                        fontSize: 14.5, fontWeight: FontWeight.w700),
                  ),
                  child: Text(_saving
                      ? 'Saqlanmoqda...'
                      : (_isEdit ? 'Saqlash' : 'Yaratish')),
                ),
              ],
            ),
          ],
        ),
      ],
    );
  }
}

class _FormCard extends StatelessWidget {
  final String title;
  final Widget child;
  const _FormCard({required this.title, required this.child});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: OnDexColors.cardBg,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(title,
              style: const TextStyle(
                  fontSize: 15,
                  fontWeight: FontWeight.w800,
                  color: OnDexColors.ink)),
          const SizedBox(height: 16),
          child,
        ],
      ),
    );
  }
}

class _LabeledField extends StatelessWidget {
  final String label;
  final bool required;
  final Widget child;
  const _LabeledField(
      {required this.label, required this.child, this.required = false});

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text.rich(
          TextSpan(
            text: label,
            style: const TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w700,
                color: OnDexColors.ink),
            children: [
              if (required)
                const TextSpan(
                    text: ' *', style: TextStyle(color: OnDexColors.danger))
            ],
          ),
        ),
        const SizedBox(height: 6),
        child,
      ],
    );
  }
}

class _CircleBackButton extends StatelessWidget {
  final VoidCallback? onTap;
  const _CircleBackButton({required this.onTap});

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(999),
      child: Container(
        width: 40,
        height: 40,
        decoration: BoxDecoration(
          color: OnDexColors.cardBg,
          shape: BoxShape.circle,
          border: Border.all(color: OnDexColors.cardBorder),
        ),
        child: const Icon(Icons.arrow_back_rounded,
            size: 19, color: OnDexColors.ink),
      ),
    );
  }
}

class _DateTimeBox extends StatelessWidget {
  final DateTime dateTime;
  final VoidCallback? onTap;
  final bool disabled;
  const _DateTimeBox(
      {required this.dateTime, required this.onTap, this.disabled = false});

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(10),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 13),
        decoration: BoxDecoration(
          color: disabled ? OnDexColors.pageBg : OnDexColors.cardBg,
          border: Border.all(color: OnDexColors.cardBorder),
          borderRadius: BorderRadius.circular(10),
        ),
        child: Row(
          children: [
            Expanded(
              child: Text(
                disabled ? '—' : _fmtDateTime(dateTime),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                    fontSize: 12.5,
                    fontWeight: FontWeight.w600,
                    color: disabled ? OnDexColors.inkFaint : OnDexColors.ink),
              ),
            ),
            const SizedBox(width: 6),
            const Icon(Icons.calendar_today_rounded,
                size: 14, color: OnDexColors.inkDim),
          ],
        ),
      ),
    );
  }
}

class _CheckChip extends StatelessWidget {
  final String label;
  final bool value;
  final ValueChanged<bool>? onChanged;
  const _CheckChip(
      {required this.label, required this.value, required this.onChanged});

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onChanged == null ? null : () => onChanged!(!value),
      borderRadius: BorderRadius.circular(8),
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 4),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 20,
              height: 20,
              decoration: BoxDecoration(
                color: value ? OnDexColors.primary : Colors.transparent,
                borderRadius: BorderRadius.circular(5),
                border: Border.all(
                    color: value ? OnDexColors.primary : OnDexColors.cardBorder,
                    width: 1.5),
              ),
              child: value
                  ? const Icon(Icons.check_rounded,
                      size: 14, color: Colors.white)
                  : null,
            ),
            const SizedBox(width: 8),
            Text(label,
                style: const TextStyle(
                    fontSize: 13.5,
                    fontWeight: FontWeight.w600,
                    color: OnDexColors.ink)),
          ],
        ),
      ),
    );
  }
}

class _TypeSelectCard extends StatelessWidget {
  final String type;
  final bool selected;
  final VoidCallback? onTap;
  const _TypeSelectCard(
      {required this.type, required this.selected, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final (color, bg) = _typeStyle(type);
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(12),
      child: Container(
        padding: const EdgeInsets.all(14),
        decoration: BoxDecoration(
          color: selected ? bg : OnDexColors.cardBg,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(
              color: selected ? color : OnDexColors.cardBorder,
              width: selected ? 1.5 : 1),
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Container(
              width: 38,
              height: 38,
              decoration: BoxDecoration(
                  color: bg, borderRadius: BorderRadius.circular(10)),
              child: Icon(_typeIcon(type), size: 18, color: color),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(_typeLabel(type),
                      style: const TextStyle(
                          fontSize: 13,
                          fontWeight: FontWeight.w700,
                          color: OnDexColors.ink)),
                  const SizedBox(height: 3),
                  Text(_typeShortDesc(type),
                      style: const TextStyle(
                          fontSize: 11.5,
                          color: OnDexColors.inkDim,
                          height: 1.3)),
                ],
              ),
            ),
            Icon(
                selected
                    ? Icons.radio_button_checked_rounded
                    : Icons.radio_button_off_rounded,
                size: 18,
                color: selected ? color : OnDexColors.inkFaint),
          ],
        ),
      ),
    );
  }
}

class _TargetPickerField extends StatelessWidget {
  final int selectedCount;
  final VoidCallback? onTap;
  const _TargetPickerField({required this.selectedCount, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(10),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 13),
        decoration: BoxDecoration(
          color: OnDexColors.cardBg,
          border: Border.all(color: OnDexColors.cardBorder),
          borderRadius: BorderRadius.circular(10),
        ),
        child: Row(
          children: [
            Expanded(
              child: Text(
                selectedCount == 0 ? 'Tanlang' : '$selectedCount ta tanlandi',
                style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: selectedCount == 0
                        ? OnDexColors.inkFaint
                        : OnDexColors.ink),
              ),
            ),
            const Icon(Icons.keyboard_arrow_down_rounded,
                size: 18, color: OnDexColors.inkDim),
          ],
        ),
      ),
    );
  }
}

class _TargetChip extends StatelessWidget {
  final String label;
  final VoidCallback? onRemove;
  const _TargetChip({required this.label, required this.onRemove});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.only(left: 10, right: 4, top: 4, bottom: 4),
      decoration: BoxDecoration(
        color: OnDexColors.pageBg,
        borderRadius: BorderRadius.circular(999),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(label,
              style: const TextStyle(
                  fontSize: 11.5,
                  fontWeight: FontWeight.w600,
                  color: OnDexColors.ink)),
          if (onRemove != null)
            InkWell(
              onTap: onRemove,
              borderRadius: BorderRadius.circular(999),
              child: const Padding(
                padding: EdgeInsets.all(3),
                child: Icon(Icons.close_rounded,
                    size: 13, color: OnDexColors.inkDim),
              ),
            ),
        ],
      ),
    );
  }
}

/// Maqsadli mahsulot/turkum tanlash oynasi — shu restoranning HAQIQIY
/// menyusi/turkumlaridan (api.menu()/api.categories()) tanlanadi.
class _TargetPickerDialog extends StatefulWidget {
  final List<Map<String, dynamic>> products;
  final List<String> categories;
  final bool showProducts;
  final bool showCategories;
  final List<String> selectedProductIds;
  final List<String> selectedCategories;

  const _TargetPickerDialog({
    required this.products,
    required this.categories,
    required this.showProducts,
    required this.showCategories,
    required this.selectedProductIds,
    required this.selectedCategories,
  });

  @override
  State<_TargetPickerDialog> createState() => _TargetPickerDialogState();
}

class _TargetPickerDialogState extends State<_TargetPickerDialog> {
  late Set<String> _products;
  late Set<String> _categories;

  @override
  void initState() {
    super.initState();
    _products = widget.selectedProductIds.toSet();
    _categories = widget.selectedCategories.toSet();
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Maqsadli mahsulotlar / kategoriyalar'),
      content: SizedBox(
        width: 420,
        height: 420,
        child: ListView(
          children: [
            if (widget.showCategories) ...[
              const Text('Kategoriyalar',
                  style: TextStyle(
                      fontSize: 12.5,
                      fontWeight: FontWeight.w700,
                      color: OnDexColors.inkDim)),
              if (widget.categories.isEmpty)
                const Padding(
                  padding: EdgeInsets.symmetric(vertical: 8),
                  child: Text('Hali turkum yo\'q',
                      style: TextStyle(
                          fontSize: 12.5, color: OnDexColors.inkFaint)),
                ),
              for (final c in widget.categories)
                CheckboxListTile(
                  dense: true,
                  contentPadding: EdgeInsets.zero,
                  value: _categories.contains(c),
                  title: Text(c, style: const TextStyle(fontSize: 13.5)),
                  onChanged: (v) => setState(() {
                    if (v == true) {
                      _categories.add(c);
                    } else {
                      _categories.remove(c);
                    }
                  }),
                ),
              const SizedBox(height: 10),
            ],
            if (widget.showProducts) ...[
              const Text('Mahsulotlar',
                  style: TextStyle(
                      fontSize: 12.5,
                      fontWeight: FontWeight.w700,
                      color: OnDexColors.inkDim)),
              if (widget.products.isEmpty)
                const Padding(
                  padding: EdgeInsets.symmetric(vertical: 8),
                  child: Text('Hali mahsulot yo\'q',
                      style: TextStyle(
                          fontSize: 12.5, color: OnDexColors.inkFaint)),
                ),
              for (final p in widget.products)
                CheckboxListTile(
                  dense: true,
                  contentPadding: EdgeInsets.zero,
                  value: _products.contains(p['id']),
                  title: Text(p['name'] as String? ?? '',
                      style: const TextStyle(fontSize: 13.5)),
                  subtitle: (p['category'] as String? ?? '').isEmpty
                      ? null
                      : Text(p['category'] as String,
                          style: const TextStyle(fontSize: 11.5)),
                  onChanged: (v) => setState(() {
                    final id = p['id'] as String? ?? '';
                    if (v == true) {
                      _products.add(id);
                    } else {
                      _products.remove(id);
                    }
                  }),
                ),
            ],
          ],
        ),
      ),
      actions: [
        TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Bekor qilish')),
        FilledButton(
          onPressed: () => Navigator.pop(
              context, (_products.toList(), _categories.toList())),
          child: const Text('Tanlash'),
        ),
      ],
    );
  }
}

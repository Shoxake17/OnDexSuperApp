import 'dart:async';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:pdf/pdf.dart';
import 'package:pdf/widgets.dart' as pw;
import 'package:printing/printing.dart';
import 'package:qr_flutter/qr_flutter.dart';

import '../api.dart';
import '../live.dart';
import '../theme.dart';
import '../widgets/file_saver.dart';
import 'tables/table_models.dart';

part 'tables/table_dialogs.dart';
part 'tables/table_panels.dart';
part 'tables/table_qr.dart';

/// "QR Stollar" — restoranning barcha joylari (stol, kabina, VIP xona,
/// topchan, bar stoyka...) va ularning O'ZGARMAS QR kodlari. Joylashuv
/// `image/QR.png` namunasi bo'yicha.
///
/// ┌─ HAMMA HOLAT SERVERDAN ───────────────────────────────────────────┐
/// "Band" — joyga bog'langan yakunlanmagan buyurtma borligi, "So'nggi
/// skanerlangan" — QR haqiqatan skanerlangan vaqt (`GET /tables/resolve`),
/// "Tozalanmoqda" — xodim qo'ygan belgi. Hech biri shu yerda taxmin
/// qilinmaydi (`internal/tables/occupancy.go`).
/// └───────────────────────────────────────────────────────────────────┘
///
/// ┌─ QR KOD BIR MARTA YARATILADI ─────────────────────────────────────┐
/// QR ichida faqat serverdagi abadiy havola (`qr_link`). "Qayta yaratish"
/// tugmasi ham, endpointi ham ATAYLAB yo'q: QR menyu varaqasiga chop
/// etilib joyda turadi — uni almashtirish butun zalni qayta chop etish
/// degani. Ekrandagi va yuklab olingan (PDF) QR bir xil ma'lumotdan.
/// └───────────────────────────────────────────────────────────────────┘
class TablesPage extends StatefulWidget {
  const TablesPage({super.key, this.live = true});

  /// Jonli yangilanish (soket + zaxira so'rov, "N daqiqadan beri" yozuvlari).
  /// Testlarda o'chiriladi.
  final bool live;

  @override
  State<TablesPage> createState() => _TablesPageState();
}

const _gap = 14.0;
const _cardHeight = 160.0;
const _wideBreakpoint = 1180.0;
const _panelTint = Color(0xFFFBF6EF);
const _monthsShort = [
  'yan', 'fev', 'mar', 'apr', 'may', 'iyun', //
  'iyul', 'avg', 'sen', 'okt', 'noy', 'dek',
];
const _noLinkText =
    'QR havolasi yo\'q: serverda Telegram bot sozlanmagan (TELEGRAM_BOT_TOKEN).';

String _two(int n) => n.toString().padLeft(2, '0');

String _clock(DateTime t) => '${_two(t.hour)}:${_two(t.minute)}';

String _dateText(DateTime t) => '${t.day}-${_monthsShort[t.month - 1]}, ${t.year}';

/// Bugun — faqat soat; boshqa kun — sana bilan.
String _when(DateTime t) {
  final now = DateTime.now();
  if (t.year == now.year && t.month == now.month && t.day == now.day) return _clock(t);
  return '${t.day}-${_monthsShort[t.month - 1]} ${_clock(t)}';
}

String _sinceText(DateTime since) {
  final m = DateTime.now().difference(since).inMinutes;
  if (m < 1) return 'hozirgina';
  if (m < 60) return '$m daq';
  final h = m ~/ 60;
  return m % 60 == 0 ? '$h soat' : '$h soat ${m % 60} daq';
}

class _TablesPageState extends State<TablesPage> {
  List<DiningTable> _tables = const [];
  String _restaurantName = '';
  bool _loading = true;
  String? _error;
  String? _selectedId;

  /// Filtrlar: '' — barcha turlar.
  String _kind = '';
  TableStatus? _status;
  String? _zone;
  final _searchCtrl = TextEditingController();
  String _query = '';

  LiveRefresher? _live;
  Timer? _tick;
  int _loadSeq = 0;

  /// Oxirgi joylashuv keng edimi — "QR kod" tugmasi keng ekranda o'ng
  /// paneldagi QR'ni almashtiradi, torda alohida oyna ochadi.
  bool _isWide = true;

  @override
  void initState() {
    super.initState();
    _searchCtrl.addListener(() {
      final q = _searchCtrl.text.trim();
      if (q != _query) setState(() => _query = q);
    });
    _load();
    if (widget.live) {
      _live = LiveRefresher(
        bus: restaurantLive,
        onRefresh: _load,
        types: const {'new_order', 'order_status'},
        offlineInterval: const Duration(seconds: 20),
      )..start();
      _tick = Timer.periodic(const Duration(seconds: 30), (_) {
        if (mounted) setState(() {});
      });
    }
  }

  @override
  void dispose() {
    _live?.dispose();
    _tick?.cancel();
    _searchCtrl.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final seq = ++_loadSeq;
    try {
      final raw = await api.tables();
      if (!mounted || seq != _loadSeq) return;
      final list = [
        for (final e in raw)
          if (e is Map) DiningTable.fromJson(Map<String, dynamic>.from(e)),
      ]..sort(compareTables);
      setState(() {
        _tables = list;
        _loading = false;
        _error = null;
        if (_selectedId == null || !list.any((t) => t.id == _selectedId)) {
          _selectedId = list.isEmpty ? null : list.first.id;
        }
        if (_zone != null && !list.any((t) => t.zone == _zone)) _zone = null;
      });
    } on ApiException catch (e) {
      if (!mounted || seq != _loadSeq) return;
      setState(() {
        _loading = false;
        _error = e.message;
      });
    } catch (_) {
      if (!mounted || seq != _loadSeq) return;
      setState(() {
        _loading = false;
        _error = 'Joylarni yuklab bo\'lmadi. Internet aloqasini tekshirib, qayta urinib ko\'ring.';
      });
    }
    if (_restaurantName.isEmpty) {
      try {
        final r = await api.myRestaurant();
        if (mounted) setState(() => _restaurantName = '${r['name'] ?? ''}'.trim());
      } catch (_) {
        // Faqat PDF sarlavhasi uchun — sahifani to'xtatmaydi.
      }
    }
  }

  // ─── Hosila ma'lumotlar ─────────────────────────────────────────────

  List<String> get _zones {
    final set = <String>{for (final t in _tables) t.zone};
    return set.toList()..sort(compareZones);
  }

  bool get _multiZone => _zones.length > 1;

  List<(TableKindInfo, int)> get _kindCounts {
    final counts = <String, int>{};
    for (final t in _tables) {
      counts[t.kind] = (counts[t.kind] ?? 0) + 1;
    }
    return [
      for (final k in kTableKinds)
        if (counts[k.kind] != null) (k, counts[k.kind]!),
    ];
  }

  List<DiningTable> get _visible => _tables
      .where((t) =>
          (_kind.isEmpty || t.kind == _kind) &&
          (_status == null || t.status == _status) &&
          (_zone == null || t.zone == _zone) &&
          t.matches(_query))
      .toList();

  DiningTable? get _selected {
    for (final t in _tables) {
      if (t.id == _selectedId) return t;
    }
    return null;
  }

  int get _extraFilterCount => (_status == null ? 0 : 1) + (_zone == null ? 0 : 1);

  void _resetFilters() {
    _searchCtrl.clear();
    setState(() {
      _kind = '';
      _status = null;
      _zone = null;
      _query = '';
    });
  }

  /// Yangi xabar oldingisini kutib navbatda turmaydi: aks holda "band joyni
  /// o'chirib bo'lmaydi" kabi javob 4 soniya kechikib, boshqa amalga
  /// tegishlidek ko'rinardi.
  void _toast(String m) {
    if (!mounted) return;
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(m)));
  }

  // ─── Amallar ────────────────────────────────────────────────────────

  Future<void> _create() async {
    final ids = await showDialog<List<String>>(
      context: context,
      barrierDismissible: false,
      builder: (_) => _TableEditorDialog(
        zones: _zones,
        onSubmit: (d) async {
          if (d.batch) {
            final list = await api.createTablesBatch(
              zone: d.zone,
              kind: d.kind,
              prefix: d.prefix,
              from: d.from,
              count: d.count,
              capacity: d.capacity,
            );
            return [
              for (final e in list)
                if (e is Map) '${e['id']}',
            ];
          }
          final one = await api.createTable(
              label: d.label, zone: d.zone, kind: d.kind, capacity: d.capacity);
          return ['${one['id']}'];
        },
      ),
    );
    if (ids == null || !mounted) return;
    // Yangi joylar filtr ortida qolib ketmasin.
    _resetFilters();
    if (ids.isNotEmpty) _selectedId = ids.first;
    await _load();
    _toast(ids.length == 1
        ? 'Joy qo\'shildi — QR kodi tayyor'
        : '${ids.length} ta joy qo\'shildi — QR kodlari tayyor');
  }

  Future<void> _edit(DiningTable t) async {
    final ids = await showDialog<List<String>>(
      context: context,
      barrierDismissible: false,
      builder: (_) => _TableEditorDialog(
        zones: _zones,
        existing: t,
        onSubmit: (d) async {
          final patch = d.patchFor(t);
          if (patch.isNotEmpty) await api.updateTable(t.id, patch);
          return [t.id];
        },
      ),
    );
    if (ids == null || !mounted) return;
    await _load();
    _toast('O\'zgarishlar saqlandi');
  }

  Future<void> _patch(DiningTable t, Map<String, Object?> body, String done) async {
    try {
      await api.updateTable(t.id, body);
      await _load();
      _toast(done);
    } on ApiException catch (e) {
      _toast(e.message);
    } catch (_) {
      _toast('Saqlab bo\'lmadi. Internet aloqasini tekshiring.');
    }
  }

  Future<void> _delete(DiningTable t) async {
    if (t.status == TableStatus.occupied) {
      _toast('Band joyni o\'chirib bo\'lmaydi — avval undagi buyurtmalarni yakunlang');
      return;
    }
    if (!await _confirmDelete(context, t)) return;
    try {
      await api.deleteTable(t.id);
      if (_selectedId == t.id) _selectedId = null;
      await _load();
      _toast('"${t.displayLabel}" o\'chirildi');
    } on ApiException catch (e) {
      _toast(e.message);
    } catch (_) {
      _toast('O\'chirib bo\'lmadi. Internet aloqasini tekshiring.');
    }
  }

  Future<void> _openDetails(DiningTable t) async {
    setState(() => _selectedId = t.id);
    final action = await showDialog<_DetailAction>(
      context: context,
      builder: (_) => _TableDetailsDialog(table: t, multiZone: _multiZone),
    );
    if (action == null || !mounted) return;
    switch (action) {
      case _DetailAction.edit:
        await _edit(t);
      case _DetailAction.cleaning:
        final start = t.cleaningSince == null;
        await _patch(t, {'cleaning': start},
            start ? '"${t.title}" — tozalanmoqda' : '"${t.title}" — tozalash tugadi');
      case _DetailAction.active:
        await _patch(
            t,
            {'active': !t.active},
            t.active
                ? '"${t.title}" vaqtincha yopildi — QR kodi bilan buyurtma qabul qilinmaydi'
                : '"${t.title}" qayta ochildi');
      case _DetailAction.downloadPdf:
        await _downloadQr(t);
      case _DetailAction.delete:
        await _delete(t);
    }
  }

  /// QR varaqasi (PDF, A6) — bosmaga tayyor fayl.
  Future<void> _downloadQr(DiningTable t) async {
    if (t.qrLink == null) {
      _toast(_noLinkText);
      return;
    }
    try {
      final bytes = await _buildQrPdf(table: t, restaurantName: _restaurantName);
      final path = await saveBytesAs(
        fileName: _qrFileName(t, 'pdf'),
        bytes: bytes,
        extension: 'pdf',
      );
      if (path != null) _toast('QR kod saqlandi: $path');
    } catch (e) {
      _toast('QR kodni saqlab bo\'lmadi: $e');
    }
  }

  Future<void> _showQr(DiningTable t) async {
    setState(() => _selectedId = t.id);
    if (_isWide) return;
    await showDialog<void>(
      context: context,
      builder: (ctx) => Dialog(
        backgroundColor: Colors.transparent,
        surfaceTintColor: Colors.transparent,
        insetPadding: const EdgeInsets.all(16),
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 440),
          child: SingleChildScrollView(
            child: _QrCard(
              table: t,
              multiZone: _multiZone,
              onDownloadPdf: () {
                Navigator.of(ctx).pop();
                _downloadQr(t);
              },
            ),
          ),
        ),
      ),
    );
  }

  Future<void> _openFilterMenu(BuildContext anchor) async {
    final box = anchor.findRenderObject()! as RenderBox;
    final overlay = Overlay.of(anchor).context.findRenderObject()! as RenderBox;
    final position = RelativeRect.fromRect(
      Rect.fromPoints(
        box.localToGlobal(Offset(0, box.size.height + 4), ancestor: overlay),
        box.localToGlobal(box.size.bottomRight(const Offset(0, 4)), ancestor: overlay),
      ),
      Offset.zero & overlay.size,
    );
    const section = TextStyle(
        fontSize: 11, letterSpacing: 0.6, fontWeight: FontWeight.w700, color: OnDexColors.inkFaint);
    final zones = _zones;
    final value = await showMenu<String>(
      context: context,
      position: position,
      items: [
        const PopupMenuItem(enabled: false, height: 28, child: Text('HOLAT', style: section)),
        CheckedPopupMenuItem(value: 'status:', checked: _status == null, child: const Text('Barcha holatlar')),
        for (final s in TableStatus.values)
          CheckedPopupMenuItem(value: 'status:${s.name}', checked: _status == s, child: Text(s.title)),
        if (zones.length > 1) ...[
          const PopupMenuDivider(),
          const PopupMenuItem(enabled: false, height: 28, child: Text('ZAL', style: section)),
          CheckedPopupMenuItem(value: 'zone:', checked: _zone == null, child: const Text('Barcha zallar')),
          for (final z in zones)
            CheckedPopupMenuItem(value: 'zone:$z', checked: _zone == z, child: Text(z)),
        ],
        const PopupMenuDivider(),
        const PopupMenuItem(value: 'reset', child: Text('Filtrlarni tozalash')),
      ],
    );
    if (value == null || !mounted) return;
    if (value == 'reset') {
      _resetFilters();
      return;
    }
    final i = value.indexOf(':');
    final key = value.substring(0, i);
    final arg = value.substring(i + 1);
    setState(() {
      if (key == 'status') {
        _status = arg.isEmpty ? null : TableStatus.values.byName(arg);
      } else {
        _zone = arg.isEmpty ? null : arg;
      }
    });
  }

  void _toggleStatusFilter(TableStatus? s) =>
      setState(() => _status = s == null || _status == s ? null : s);

  Future<void> _showBreakdown() => showDialog<void>(
        context: context,
        builder: (_) => _StatusBreakdownDialog(tables: _tables, zones: _zones),
      );

  Future<void> _showAllScans() async {
    final picked = await showDialog<DiningTable>(
      context: context,
      builder: (_) => _RecentScansDialog(tables: _tables, multiZone: _multiZone),
    );
    if (picked != null && mounted) setState(() => _selectedId = picked.id);
  }

  // ─── Ko'rinish ──────────────────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    if (_loading && _tables.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }
    return Padding(
      padding: const EdgeInsets.fromLTRB(24, 20, 24, 20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _TablesHeader(
            searchCtrl: _searchCtrl,
            filterCount: _extraFilterCount,
            onFilter: _openFilterMenu,
            onAdd: _create,
          ),
          const SizedBox(height: 16),
          if (_error != null && _tables.isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(bottom: 12),
              child: _ErrorStrip(message: _error!, onRetry: _load),
            ),
          Expanded(
            child: _error != null && _tables.isEmpty
                ? _ErrorState(message: _error!, onRetry: _load)
                : _buildBody(),
          ),
        ],
      ),
    );
  }

  Widget _buildBody() {
    return LayoutBuilder(builder: (context, box) {
      _isWide = box.maxWidth >= _wideBreakpoint;
      final sidebar = Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _QrCard(
            table: _selected,
            multiZone: _multiZone,
            onDownloadPdf: _selected == null ? null : () => _downloadQr(_selected!),
          ),
          const SizedBox(height: _gap),
          _StatusCard(
            tables: _tables,
            selected: _status,
            onStatus: _toggleStatusFilter,
            onDetails: _showBreakdown,
          ),
          const SizedBox(height: _gap),
          _RecentScansCard(
            tables: _tables,
            multiZone: _multiZone,
            onOpen: _openDetails,
            onShowAll: _showAllScans,
          ),
        ],
      );
      final chips = _KindChipsBar(
        total: _tables.length,
        kinds: _kindCounts,
        selected: _kind,
        onKind: (k) => setState(() => _kind = k),
      );
      if (_isWide) {
        // Turlar faqat chap ustun ustida: o'ng paneldagi QR kartasi sarlavha
        // ostidan, turlar bilan BIR balandlikdan boshlanadi (namunadagidek).
        return Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  chips,
                  const SizedBox(height: _gap),
                  Expanded(child: _buildGrid(scrollable: true)),
                ],
              ),
            ),
            const SizedBox(width: 16),
            SizedBox(
              width: box.maxWidth >= 1500 ? 420 : 380,
              child: SingleChildScrollView(child: sidebar),
            ),
          ],
        );
      }
      return SingleChildScrollView(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            chips,
            const SizedBox(height: _gap),
            _buildGrid(scrollable: false),
            const SizedBox(height: 16),
            sidebar,
          ],
        ),
      );
    });
  }

  Widget _buildGrid({required bool scrollable}) {
    if (_tables.isEmpty) return _EmptyState(onAdd: _create);
    final list = _visible;
    if (list.isEmpty) return _NoMatches(onReset: _resetFilters);
    return LayoutBuilder(builder: (context, box) {
      const minWidth = 268.0;
      final columns = ((box.maxWidth + _gap) / (minWidth + _gap)).floor().clamp(1, 4);
      return GridView.builder(
        shrinkWrap: !scrollable,
        physics: scrollable ? null : const NeverScrollableScrollPhysics(),
        gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
          crossAxisCount: columns,
          crossAxisSpacing: _gap,
          mainAxisSpacing: _gap,
          mainAxisExtent: _cardHeight,
        ),
        itemCount: list.length,
        itemBuilder: (context, i) {
          final t = list[i];
          return _TableCard(
            key: ValueKey('table-${t.id}'),
            table: t,
            selected: t.id == _selectedId,
            showZone: _multiZone,
            onSelect: () => setState(() => _selectedId = t.id),
            onOpen: () => _openDetails(t),
            onQr: () => _showQr(t),
          );
        },
      );
    });
  }
}

// ─── Sarlavha ─────────────────────────────────────────────────────────

class _TablesHeader extends StatelessWidget {
  const _TablesHeader({
    required this.searchCtrl,
    required this.filterCount,
    required this.onFilter,
    required this.onAdd,
  });

  final TextEditingController searchCtrl;
  final int filterCount;
  final void Function(BuildContext anchor) onFilter;
  final VoidCallback onAdd;

  @override
  Widget build(BuildContext context) {
    final title = Row(
      children: [
        Container(
          width: 54,
          height: 54,
          decoration: BoxDecoration(
            color: OnDexColors.primaryTint,
            borderRadius: BorderRadius.circular(14),
          ),
          child: const Icon(Icons.table_restaurant_rounded, color: OnDexColors.primary, size: 28),
        ),
        const SizedBox(width: 14),
        const Flexible(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text('QR Stollar',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(fontSize: 24, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
              SizedBox(height: 2),
              Text('Mijozlar uchun QR kodlar orqali stol buyurtmalarini qabul qiling',
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(fontSize: 13.5, color: OnDexColors.inkDim)),
            ],
          ),
        ),
      ],
    );

    final search = TextField(
      controller: searchCtrl,
      style: const TextStyle(fontSize: 13.5),
      decoration: InputDecoration(
        isDense: true,
        hintText: 'Stol raqami yoki QR kodini qidiring...',
        hintStyle: const TextStyle(fontSize: 13, color: OnDexColors.inkFaint),
        prefixIcon: const Icon(Icons.search_rounded, size: 19, color: OnDexColors.inkFaint),
        filled: true,
        fillColor: OnDexColors.cardBg,
        contentPadding: const EdgeInsets.symmetric(vertical: 13),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: OnDexColors.cardBorder),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: OnDexColors.primary),
        ),
        border: OutlineInputBorder(borderRadius: BorderRadius.circular(12)),
      ),
    );

    final filter = Builder(
      builder: (anchor) => _FilterButton(count: filterCount, onPressed: () => onFilter(anchor)),
    );

    final add = FilledButton.icon(
      onPressed: onAdd,
      style: FilledButton.styleFrom(
        backgroundColor: OnDexColors.primary,
        foregroundColor: Colors.white,
        minimumSize: const Size(0, 46),
        padding: const EdgeInsets.symmetric(horizontal: 18),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
        textStyle: const TextStyle(fontSize: 14, fontWeight: FontWeight.w700),
      ),
      icon: const Icon(Icons.add_rounded, size: 20),
      label: const Text('Yangi stol qo\'shish'),
    );

    // Yuqorida BITTA qator: sarlavha, qidiruv, filtr va "Yangi stol
    // qo'shish". Turlar bu yerda EMAS — joylar ro'yxati ustida
    // (`_KindChipsBar`), aks holda o'ng paneldagi QR kartasi ham ular
    // ostiga tushib ketardi.
    return LayoutBuilder(builder: (context, box) {
      final w = box.maxWidth;
      final Widget top;
      if (w >= 980) {
        top = Row(
          children: [
            Expanded(child: title),
            const SizedBox(width: 16),
            SizedBox(width: w >= 1300 ? 340 : 270, child: search),
            const SizedBox(width: 10),
            filter,
            const SizedBox(width: 12),
            add,
          ],
        );
      } else {
        // Tor oynada bitta qatorga sig'maydi — ustma-ust.
        top = Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            title,
            const SizedBox(height: 12),
            Row(children: [Expanded(child: search), const SizedBox(width: 8), filter]),
            const SizedBox(height: 10),
            add,
          ],
        );
      }
      return top;
    });
  }
}

/// Restoranda BOR turlarning HAMMASI (sonlari bilan), chap chetdan.
/// `Wrap`, gorizontal aylanadigan ro'yxat emas: ish stolida sichqoncha
/// g'ildiragi yon tomonga aylantirmaydi va sig'magan turlar ko'rinmay
/// qolardi.
class _KindChipsBar extends StatelessWidget {
  const _KindChipsBar({
    required this.total,
    required this.kinds,
    required this.selected,
    required this.onKind,
  });

  final int total;
  final List<(TableKindInfo, int)> kinds;
  final String selected;
  final ValueChanged<String> onKind;

  @override
  Widget build(BuildContext context) {
    return Wrap(
      spacing: 8,
      runSpacing: 8,
      children: [
        _KindChip(
          label: 'Barchasi',
          count: total,
          selected: selected.isEmpty,
          onTap: () => onKind(''),
        ),
        for (final (k, n) in kinds)
          _KindChip(
            icon: k.icon,
            label: k.title,
            count: n,
            selected: selected == k.kind,
            onTap: () => onKind(k.kind),
          ),
      ],
    );
  }
}

class _KindChip extends StatelessWidget {
  const _KindChip({
    required this.label,
    required this.count,
    required this.selected,
    required this.onTap,
    this.icon,
  });

  final String label;
  final int count;
  final bool selected;
  final VoidCallback onTap;
  final IconData? icon;

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
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              if (icon != null) ...[
                Icon(icon, size: 16, color: selected ? Colors.white : OnDexColors.inkDim),
                const SizedBox(width: 6),
              ],
              Text(label, style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: fg)),
              const SizedBox(width: 7),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 1),
                decoration: BoxDecoration(
                  color: selected ? Colors.white.withValues(alpha: 0.22) : OnDexColors.pageBg,
                  borderRadius: BorderRadius.circular(999),
                ),
                child: Text('$count',
                    style: TextStyle(
                        fontSize: 11.5,
                        fontWeight: FontWeight.w700,
                        color: selected ? Colors.white : OnDexColors.inkDim)),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _FilterButton extends StatelessWidget {
  const _FilterButton({required this.count, required this.onPressed});

  final int count;
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: 'Holat va zal bo\'yicha filtr',
      child: SizedBox(
        width: 46,
        height: 46,
        child: Stack(
          clipBehavior: Clip.none,
          children: [
            Positioned.fill(
              child: OutlinedButton(
                onPressed: onPressed,
                style: OutlinedButton.styleFrom(
                  padding: EdgeInsets.zero,
                  foregroundColor: count > 0 ? OnDexColors.primary : OnDexColors.ink,
                  backgroundColor: count > 0 ? OnDexColors.primaryTint : OnDexColors.cardBg,
                  side: BorderSide(color: count > 0 ? OnDexColors.primary : OnDexColors.cardBorder),
                  shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
                ),
                child: const Icon(Icons.tune_rounded, size: 20),
              ),
            ),
            if (count > 0)
              Positioned(
                right: -4,
                top: -4,
                child: Container(
                  width: 18,
                  height: 18,
                  alignment: Alignment.center,
                  decoration: const BoxDecoration(color: OnDexColors.primary, shape: BoxShape.circle),
                  child: Text('$count',
                      style: const TextStyle(
                          fontSize: 10.5, fontWeight: FontWeight.w800, color: Colors.white)),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

// ─── Kartochka ────────────────────────────────────────────────────────

class _TableCard extends StatelessWidget {
  const _TableCard({
    super.key,
    required this.table,
    required this.selected,
    required this.showZone,
    required this.onSelect,
    required this.onOpen,
    required this.onQr,
  });

  final DiningTable table;
  final bool selected;
  final bool showZone;
  final VoidCallback onSelect;
  final VoidCallback onOpen;
  final VoidCallback onQr;

  @override
  Widget build(BuildContext context) {
    final t = table;
    final subtitle = [
      t.capacity == null ? 'Sig\'im ko\'rsatilmagan' : '${t.capacity} kishilik',
      if (showZone) t.zone,
    ].join(' · ');
    return Material(
      color: OnDexColors.cardBg,
      borderRadius: BorderRadius.circular(16),
      child: InkWell(
        borderRadius: BorderRadius.circular(16),
        onTap: onSelect,
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 150),
          padding: const EdgeInsets.fromLTRB(16, 14, 6, 14),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(16),
            border: Border.all(
              color: selected ? OnDexColors.primary : OnDexColors.cardBorder,
              width: selected ? 1.6 : 1,
            ),
            boxShadow: const [
              BoxShadow(color: Color(0x0A1F1710), blurRadius: 14, offset: Offset(0, 4)),
            ],
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _KindIconTile(kind: t.kindInfo, status: t.status, size: 46),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(t.title,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: const TextStyle(
                                fontSize: 20,
                                height: 1.15,
                                fontWeight: FontWeight.w800,
                                color: OnDexColors.ink)),
                        const SizedBox(height: 3),
                        Text(subtitle,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: const TextStyle(fontSize: 12.5, color: OnDexColors.inkDim)),
                      ],
                    ),
                  ),
                  const SizedBox(width: 6),
                  _FitPill(status: t.status),
                  SizedBox(
                    width: 30,
                    height: 26,
                    child: IconButton(
                      tooltip: 'Batafsil',
                      padding: EdgeInsets.zero,
                      onPressed: onOpen,
                      icon: const Icon(Icons.chevron_right_rounded, size: 22, color: OnDexColors.inkFaint),
                    ),
                  ),
                ],
              ),
              const Spacer(),
              Row(
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  Expanded(child: _CardFooterInfo(table: t)),
                  const SizedBox(width: 8),
                  Padding(
                    padding: const EdgeInsets.only(right: 8),
                    child: OutlinedButton.icon(
                      onPressed: onQr,
                      style: OutlinedButton.styleFrom(
                        foregroundColor: OnDexColors.ink,
                        side: const BorderSide(color: OnDexColors.cardBorder),
                        minimumSize: const Size(0, 36),
                        padding: const EdgeInsets.symmetric(horizontal: 12),
                        tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
                        textStyle: const TextStyle(fontSize: 12.5, fontWeight: FontWeight.w600),
                      ),
                      icon: const Icon(Icons.qr_code_2_rounded, size: 17),
                      label: const Text('QR kod'),
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _CardFooterInfo extends StatelessWidget {
  const _CardFooterInfo({required this.table});

  final DiningTable table;

  static const _dim = TextStyle(fontSize: 12, color: OnDexColors.inkDim);

  @override
  Widget build(BuildContext context) {
    final t = table;
    switch (t.status) {
      case TableStatus.occupied:
        final o = t.currentOrder!;
        final more = t.activeOrders.length - 1;
        final at = o.createdAt;
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(o.shortNumber.isEmpty ? 'Buyurtma' : 'Buyurtma ${o.shortNumber}',
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(
                    fontSize: 12.5, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
            const SizedBox(height: 2),
            Text(
                [
                  if (at != null) _clock(at),
                  '${o.items} ta mahsulot',
                  if (more > 0) '+$more buyurtma',
                ].join(' · '),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: _dim),
          ],
        );
      case TableStatus.cleaning:
        final since = t.cleaningSince;
        return Text(since == null ? 'Tozalanmoqda' : 'Tozalanmoqda · ${_sinceText(since)}',
            maxLines: 1, overflow: TextOverflow.ellipsis, style: _dim);
      case TableStatus.inactive:
        return const Text('Vaqtincha yopilgan',
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(fontSize: 12, color: OnDexColors.danger, fontWeight: FontWeight.w600));
      case TableStatus.available:
        final scanned = t.lastScannedAt;
        if (scanned == null) return const SizedBox.shrink();
        return Text('Oxirgi skanerlash: ${_when(scanned)}',
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: const TextStyle(fontSize: 12, color: OnDexColors.inkFaint));
    }
  }
}

// ─── Bo'sh va xato holatlar ───────────────────────────────────────────

class _EmptyState extends StatelessWidget {
  const _EmptyState({required this.onAdd});

  final VoidCallback onAdd;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 480),
        child: Container(
          padding: const EdgeInsets.all(28),
          decoration: BoxDecoration(
            color: OnDexColors.cardBg,
            borderRadius: BorderRadius.circular(18),
            border: Border.all(color: OnDexColors.cardBorder),
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 64,
                height: 64,
                decoration: BoxDecoration(
                    color: OnDexColors.primaryTint, borderRadius: BorderRadius.circular(18)),
                child: const Icon(Icons.qr_code_2_rounded, size: 34, color: OnDexColors.primary),
              ),
              const SizedBox(height: 14),
              const Text('Hali joy qo\'shilmagan',
                  style: TextStyle(fontSize: 18, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
              const SizedBox(height: 6),
              const Text(
                'Stol, kabina, VIP xona yoki topchan qo\'shing — har biriga o\'zgarmas QR kod '
                'yaratiladi. Mijoz uni skanerlab, shu joyga buyurtma beradi.',
                textAlign: TextAlign.center,
                style: TextStyle(fontSize: 13.5, color: OnDexColors.inkDim),
              ),
              const SizedBox(height: 18),
              FilledButton.icon(
                onPressed: onAdd,
                style: FilledButton.styleFrom(
                  backgroundColor: OnDexColors.primary,
                  minimumSize: const Size(0, 44),
                  shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
                ),
                icon: const Icon(Icons.add_rounded),
                label: const Text('Birinchi joyni qo\'shish'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _NoMatches extends StatelessWidget {
  const _NoMatches({required this.onReset});

  final VoidCallback onReset;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 40),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.search_off_rounded, size: 40, color: OnDexColors.inkFaint),
            const SizedBox(height: 10),
            const Text('Filtr bo\'yicha joy topilmadi',
                style: TextStyle(fontSize: 15, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
            const SizedBox(height: 10),
            OutlinedButton(onPressed: onReset, child: const Text('Filtrlarni tozalash')),
          ],
        ),
      ),
    );
  }
}

class _ErrorState extends StatelessWidget {
  const _ErrorState({required this.message, required this.onRetry});

  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.cloud_off_rounded, size: 40, color: OnDexColors.inkFaint),
          const SizedBox(height: 10),
          Text(message,
              textAlign: TextAlign.center,
              style: const TextStyle(fontSize: 14, color: OnDexColors.ink)),
          const SizedBox(height: 14),
          FilledButton(onPressed: onRetry, child: const Text('Qayta urinish')),
        ],
      ),
    );
  }
}

class _ErrorStrip extends StatelessWidget {
  const _ErrorStrip({required this.message, required this.onRetry});

  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.fromLTRB(14, 6, 6, 6),
      decoration: BoxDecoration(color: OnDexColors.dangerBg, borderRadius: BorderRadius.circular(10)),
      child: Row(
        children: [
          const Icon(Icons.error_outline_rounded, size: 18, color: OnDexColors.danger),
          const SizedBox(width: 10),
          Expanded(
            child: Text('Yangilab bo\'lmadi: $message',
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(fontSize: 13, color: OnDexColors.ink)),
          ),
          TextButton(onPressed: onRetry, child: const Text('Qayta urinish')),
        ],
      ),
    );
  }
}

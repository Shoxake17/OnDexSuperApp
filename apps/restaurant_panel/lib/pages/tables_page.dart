import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:pdf/pdf.dart';
import 'package:pdf/widgets.dart' as pw;
import 'package:printing/printing.dart';
import 'package:qr_flutter/qr_flutter.dart';

import '../api.dart';
import '../theme.dart';
import '../widgets/page_header.dart';

const kDefaultZone = 'Asosiy zal';

/// Stollar va ularning QR kodlari.
///
/// QR ichida `https://t.me/<bot>/<short_name>?startapp=<token>` havolasi
/// bor. Token BIR MARTA yaratiladi va serverda hech qachon almashtirilmaydi —
/// shuning uchun kartochkadagi QR ertasi kuni boshqa kodga aylanib ketmaydi.
class TablesPage extends StatefulWidget {
  const TablesPage({super.key});

  @override
  State<TablesPage> createState() => _TablesPageState();
}

class _TablesPageState extends State<TablesPage> {
  List<Map<String, dynamic>> _tables = [];
  String _restaurantName = '';
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final results = await Future.wait([
        api.tables(),
        api.myRestaurant(),
      ]);
      if (!mounted) return;
      final rest = Map<String, dynamic>.from(results[1] as Map);
      setState(() {
        _tables = (results[0] as List).cast<Map<String, dynamic>>();
        _restaurantName = '${rest['name'] ?? ''}'.trim();
        _loading = false;
        _error = null;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = '$e';
      });
    }
  }

  List<String> get _knownZones {
    final set = <String>{};
    for (final t in _tables) {
      set.add(_zoneOf(t));
    }
    final extra = set.where((z) => z != kDefaultZone).toList()
      ..sort((a, b) => a.toLowerCase().compareTo(b.toLowerCase()));
    return [kDefaultZone, ...extra];
  }

  Future<void> _addTable() async {
    final result = await showDialog<_NewTable>(
      context: context,
      barrierDismissible: false,
      builder: (ctx) => _AddTableDialog(knownZones: _knownZones),
    );
    if (result == null) return;
    try {
      await api.createTable(result.number, zone: result.zone);
      await _load();
    } catch (e) {
      _toast('$e');
    }
  }

  Future<void> _rename(Map<String, dynamic> t) async {
    final label = await _askLabel(
      context,
      title: 'Stol raqamini o\'zgartirish',
      initial: t['label'] as String? ?? '',
    );
    if (label == null) return;
    try {
      await api.renameTable(t['id'] as String, label);
      await _load();
    } catch (e) {
      _toast('$e');
    }
  }

  Future<void> _toggleActive(Map<String, dynamic> t) async {
    try {
      await api.setTableActive(t['id'] as String, !(t['active'] == true));
      await _load();
    } catch (e) {
      _toast('$e');
    }
  }

  Future<void> _delete(Map<String, dynamic> t) async {
    final ok = await _confirm(
      context,
      title: 'Stolni o\'chirish',
      message: '"${_zoneOf(t)} · ${t['label']}" stoli o\'chiriladi.\n\n'
          'DIQQAT: chop etilgan QR kod ABADIY ishlamay '
          'qoladi va uni QAYTARIB BO\'LMAYDI — varaqani qayta chop etish '
          'kerak bo\'ladi.\n\n'
          'Stolni vaqtincha ishlatmaslik uchun o\'chirish o\'rniga '
          '"Vaqtincha yopish" ni tanlang — QR kod saqlanib qoladi.',
    );
    if (!ok) return;
    try {
      await api.deleteTable(t['id'] as String);
      await _load();
    } catch (e) {
      _toast('$e');
    }
  }

  void _toast(String m) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(m)));
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Center(child: CircularProgressIndicator());
    }
    return PageScaffold(
      title: 'Stollar (QR kod)',
      subtitle: 'Har bir stolga QR kod chop etib, stol ustiga qo\'ying. '
          'Mijoz uni skanerlab, o\'sha stolga buyurtma beradi.',
      action: PageActionButton(
        icon: Icons.add_rounded,
        label: 'Stol qo\'shish',
        onPressed: _addTable,
      ),
      error: _error,
      child: _tables.isEmpty
          ? const Center(
              child: Text(
                'Hali stol qo\'shilmagan',
                style: TextStyle(color: OnDexColors.inkDim),
              ),
            )
          : _TablesByZone(
              tables: _tables,
              restaurantName: _restaurantName,
              onRename: _rename,
              onToggleActive: _toggleActive,
              onDelete: _delete,
              onPrintError: _toast,
            ),
    );
  }
}

class _TablesByZone extends StatelessWidget {
  const _TablesByZone({
    required this.tables,
    required this.restaurantName,
    required this.onRename,
    required this.onToggleActive,
    required this.onDelete,
    required this.onPrintError,
  });

  final List<Map<String, dynamic>> tables;
  final String restaurantName;
  final void Function(Map<String, dynamic>) onRename;
  final void Function(Map<String, dynamic>) onToggleActive;
  final void Function(Map<String, dynamic>) onDelete;
  final void Function(String) onPrintError;

  List<MapEntry<String, List<Map<String, dynamic>>>> _groups() {
    final map = <String, List<Map<String, dynamic>>>{};
    for (final t in tables) {
      map.putIfAbsent(_zoneOf(t), () => []).add(t);
    }
    final keys = map.keys.toList()
      ..sort((a, b) {
        if (a == kDefaultZone && b != kDefaultZone) return -1;
        if (b == kDefaultZone && a != kDefaultZone) return 1;
        return a.toLowerCase().compareTo(b.toLowerCase());
      });
    return [for (final k in keys) MapEntry(k, map[k]!)];
  }

  @override
  Widget build(BuildContext context) {
    final groups = _groups();
    return CustomScrollView(
      slivers: [
        for (final g in groups) ...[
          SliverToBoxAdapter(
            child: Padding(
              padding: const EdgeInsets.only(bottom: 12, top: 4),
              child: Row(
                children: [
                  Container(
                    width: 36,
                    height: 36,
                    decoration: BoxDecoration(
                      color: OnDexColors.primaryTint,
                      borderRadius: BorderRadius.circular(10),
                    ),
                    child: const Icon(Icons.meeting_room_rounded,
                        color: OnDexColors.primary, size: 20),
                  ),
                  const SizedBox(width: 10),
                  Expanded(
                    child: Text(
                      g.key,
                      style: const TextStyle(
                        fontSize: 18,
                        fontWeight: FontWeight.w800,
                        color: OnDexColors.ink,
                      ),
                    ),
                  ),
                  Text(
                    '${g.value.length} ta stol',
                    style: const TextStyle(
                      color: OnDexColors.inkDim,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ],
              ),
            ),
          ),
          SliverGrid(
            gridDelegate: const SliverGridDelegateWithMaxCrossAxisExtent(
              maxCrossAxisExtent: 280,
              mainAxisExtent: 372,
              crossAxisSpacing: 12,
              mainAxisSpacing: 12,
            ),
            delegate: SliverChildBuilderDelegate(
              (_, i) => _TableCard(
                table: g.value[i],
                restaurantName: restaurantName,
                onRename: () => onRename(g.value[i]),
                onToggleActive: () => onToggleActive(g.value[i]),
                onDelete: () => onDelete(g.value[i]),
                onPrintError: onPrintError,
              ),
              childCount: g.value.length,
            ),
          ),
          const SliverToBoxAdapter(child: SizedBox(height: 22)),
        ],
      ],
    );
  }
}

class _TableCard extends StatelessWidget {
  const _TableCard({
    required this.table,
    required this.restaurantName,
    required this.onRename,
    required this.onToggleActive,
    required this.onDelete,
    required this.onPrintError,
  });

  final Map<String, dynamic> table;
  final String restaurantName;
  final VoidCallback onRename;
  final VoidCallback onToggleActive;
  final VoidCallback onDelete;
  final void Function(String) onPrintError;

  @override
  Widget build(BuildContext context) {
    final active = table['active'] == true;
    final label = table['label'] as String? ?? '';
    final zone = _zoneOf(table);
    final link = table['qr_link'] as String?;

    return Card(
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(14),
        side: BorderSide(
          color: active
              ? OnDexColors.cardBorder
              : OnDexColors.danger.withValues(alpha: 0.55),
        ),
      ),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    tableText(label),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                      fontSize: 17,
                      fontWeight: FontWeight.bold,
                      color: OnDexColors.ink,
                    ),
                  ),
                ),
                if (!active)
                  const Padding(
                    padding: EdgeInsets.only(right: 4),
                    child: Text(
                      'Faol emas',
                      style: TextStyle(color: Colors.orange, fontSize: 12),
                    ),
                  ),
                PopupMenuButton<String>(
                  onSelected: (v) => switch (v) {
                    'rename' => onRename(),
                    'active' => onToggleActive(),
                    'delete' => onDelete(),
                    _ => null,
                  },
                  itemBuilder: (_) => [
                    const PopupMenuItem(
                      value: 'rename',
                      child: Text('Raqamini o\'zgartirish'),
                    ),
                    PopupMenuItem(
                      value: 'active',
                      child: Text(active ? 'Vaqtincha yopish' : 'Yoqish'),
                    ),
                    const PopupMenuItem(
                      value: 'delete',
                      child: Text('O\'chirish'),
                    ),
                  ],
                ),
              ],
            ),
            const SizedBox(height: 8),
            Expanded(
              child: Center(
                child: link == null
                    ? const Padding(
                        padding: EdgeInsets.all(8),
                        child: Text(
                          'QR havolasi yo\'q.\n\nTELEGRAM_BOT_TOKEN '
                          'sozlanmagan — bot nomi aniqlanmadi.',
                          textAlign: TextAlign.center,
                          style: TextStyle(color: Colors.grey, fontSize: 12),
                        ),
                      )
                    : Container(
                        color: Colors.white,
                        padding: const EdgeInsets.all(8),
                        child: QrImageView(
                          data: link,
                          version: QrVersions.auto,
                          size: 180,
                          backgroundColor: Colors.white,
                        ),
                      ),
              ),
            ),
            const SizedBox(height: 8),
            SizedBox(
              width: double.infinity,
              child: FilledButton.icon(
                onPressed: link == null
                    ? null
                    : () async {
                        try {
                          await printTableQr(
                            restaurantName: restaurantName,
                            zone: zone,
                            label: label,
                            link: link,
                          );
                        } catch (e) {
                          onPrintError('$e');
                        }
                      },
                icon: const Icon(Icons.print_rounded, size: 18),
                label: const Text('Chop etish'),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _NewTable {
  const _NewTable({required this.zone, required this.number});
  final String zone;
  final String number;
}

class _AddTableDialog extends StatefulWidget {
  const _AddTableDialog({required this.knownZones});
  final List<String> knownZones;

  @override
  State<_AddTableDialog> createState() => _AddTableDialogState();
}

class _AddTableDialogState extends State<_AddTableDialog> {
  late final TextEditingController _numberCtrl;
  late final TextEditingController _zoneCtrl;
  late List<String> _zones;
  late String _selected;
  bool _addingZone = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _numberCtrl = TextEditingController();
    _zoneCtrl = TextEditingController();
    _zones = List.of(widget.knownZones);
    if (!_zones.contains(kDefaultZone)) {
      _zones.insert(0, kDefaultZone);
    }
    _selected = kDefaultZone;
  }

  @override
  void dispose() {
    _numberCtrl.dispose();
    _zoneCtrl.dispose();
    super.dispose();
  }

  void _commitNewZone() {
    final name = _zoneCtrl.text.trim();
    if (name.isEmpty) {
      setState(() => _error = 'Zona nomini yozing');
      return;
    }
    final exists = _zones.any((z) => z.toLowerCase() == name.toLowerCase());
    setState(() {
      _error = null;
      if (!exists) _zones.add(name);
      _selected = exists
          ? _zones.firstWhere((z) => z.toLowerCase() == name.toLowerCase())
          : name;
      _addingZone = false;
      _zoneCtrl.clear();
    });
  }

  void _save() {
    final number = _numberCtrl.text.trim();
    if (number.isEmpty) {
      setState(() => _error = 'Stol raqamini kiriting');
      return;
    }
    Navigator.of(context).pop(_NewTable(zone: _selected, number: number));
  }

  @override
  Widget build(BuildContext context) {
    return Dialog(
      insetPadding: const EdgeInsets.symmetric(horizontal: 24, vertical: 24),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(18)),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 460),
        child: Padding(
          padding: const EdgeInsets.fromLTRB(22, 20, 22, 16),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const Text(
                'Yangi stol',
                style: TextStyle(
                  fontSize: 22,
                  fontWeight: FontWeight.w800,
                  color: OnDexColors.ink,
                ),
              ),
              const SizedBox(height: 4),
              const Text(
                'Avval zalni tanlang, so\'ng stol raqamini yozing.',
                style: TextStyle(color: OnDexColors.inkDim, fontSize: 13.5),
              ),
              const SizedBox(height: 20),
              _sectionTitle('Zona'),
              const SizedBox(height: 8),
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: [
                  for (final z in _zones)
                    ChoiceChip(
                      label: Text(z),
                      selected: _selected == z,
                      selectedColor: OnDexColors.primaryTint,
                      labelStyle: TextStyle(
                        fontWeight: FontWeight.w700,
                        color: _selected == z
                            ? OnDexColors.primary
                            : OnDexColors.ink,
                      ),
                      onSelected: (_) => setState(() {
                        _selected = z;
                        _error = null;
                      }),
                    ),
                ],
              ),
              const SizedBox(height: 6),
              if (_addingZone)
                Row(
                  children: [
                    Expanded(
                      child: TextField(
                        controller: _zoneCtrl,
                        autofocus: true,
                        textInputAction: TextInputAction.done,
                        decoration: const InputDecoration(
                          hintText: 'Zona nomi, masalan: Ayvon',
                          isDense: true,
                        ),
                        onSubmitted: (_) => _commitNewZone(),
                      ),
                    ),
                    IconButton(
                      tooltip: 'Qo\'shish',
                      onPressed: _commitNewZone,
                      icon: const Icon(Icons.check_rounded,
                          color: OnDexColors.success),
                    ),
                    IconButton(
                      tooltip: 'Bekor',
                      onPressed: () => setState(() {
                        _addingZone = false;
                        _zoneCtrl.clear();
                      }),
                      icon: const Icon(Icons.close_rounded),
                    ),
                  ],
                )
              else
                Align(
                  alignment: Alignment.centerLeft,
                  child: TextButton.icon(
                    onPressed: () => setState(() {
                      _addingZone = true;
                      _error = null;
                    }),
                    icon: const Icon(Icons.add_rounded, size: 20),
                    label: const Text('Yangi zona qo\'shish'),
                  ),
                ),
              const SizedBox(height: 14),
              _sectionTitle('Stol raqami'),
              const SizedBox(height: 8),
              TextField(
                controller: _numberCtrl,
                autofocus: !_addingZone,
                keyboardType: TextInputType.number,
                textInputAction: TextInputAction.done,
                decoration: const InputDecoration(
                  hintText: 'Masalan: 12',
                  prefixIcon: Icon(Icons.table_restaurant_rounded),
                ),
                onSubmitted: (_) => _save(),
              ),
              if (_error != null) ...[
                const SizedBox(height: 10),
                Text(
                  _error!,
                  style: const TextStyle(
                      color: OnDexColors.danger, fontSize: 13),
                ),
              ],
              const SizedBox(height: 22),
              Row(
                children: [
                  Expanded(
                    child: OutlinedButton(
                      onPressed: () => Navigator.of(context).pop(),
                      child: const Text('Bekor qilish'),
                    ),
                  ),
                  const SizedBox(width: 10),
                  Expanded(
                    child: FilledButton.icon(
                      onPressed: _save,
                      icon: const Icon(Icons.save_rounded, size: 18),
                      label: const Text('Saqlash'),
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

  static Widget _sectionTitle(String text) => Text(
        text.toUpperCase(),
        style: const TextStyle(
          fontSize: 11.5,
          letterSpacing: 0.7,
          fontWeight: FontWeight.w800,
          color: OnDexColors.inkFaint,
        ),
      );
}

/// Chop etish: tizim printer oynasini ochadi.
/// QR ma'lumoti — serverdagi abadiy `qr_link`, sana/vaqt qo'shilmaydi.
Future<void> printTableQr({
  required String restaurantName,
  required String zone,
  required String label,
  required String link,
}) async {
  final png = await _qrPng(link);
  await Printing.layoutPdf(
    name: '$zone · stol $label',
    format: PdfPageFormat.a6,
    onLayout: (format) async {
      final doc = pw.Document();
      final image = pw.MemoryImage(png);
      doc.addPage(
        pw.Page(
          pageFormat: format,
          margin: const pw.EdgeInsets.all(22),
          build: (_) => pw.Center(
            child: pw.Column(
              mainAxisAlignment: pw.MainAxisAlignment.center,
              children: [
                if (restaurantName.isNotEmpty)
                  pw.Text(
                    restaurantName,
                    textAlign: pw.TextAlign.center,
                    style: pw.TextStyle(
                      fontSize: 13,
                      color: PdfColor.fromInt(0xFF7A6B5C),
                    ),
                  ),
                pw.SizedBox(height: 4),
                pw.Text(
                  zone,
                  textAlign: pw.TextAlign.center,
                  style: pw.TextStyle(
                    fontSize: 14,
                    fontWeight: pw.FontWeight.bold,
                  ),
                ),
                pw.SizedBox(height: 6),
                pw.Text(
                  tableText(label).toUpperCase(),
                  textAlign: pw.TextAlign.center,
                  style: pw.TextStyle(
                    fontSize: 28,
                    fontWeight: pw.FontWeight.bold,
                  ),
                ),
                pw.SizedBox(height: 12),
                pw.Container(
                  padding: const pw.EdgeInsets.all(10),
                  decoration: pw.BoxDecoration(
                    border: pw.Border.all(
                      color: PdfColor.fromInt(0xFFEAD9C8),
                    ),
                  ),
                  child: pw.Image(image, width: 180, height: 180),
                ),
                pw.SizedBox(height: 12),
                pw.Text(
                  'Menyu uchun QR kodni skanerlang',
                  textAlign: pw.TextAlign.center,
                  style: const pw.TextStyle(fontSize: 11),
                ),
              ],
            ),
          ),
        ),
      );
      return doc.save();
    },
  );
}

Future<Uint8List> _qrPng(String data) async {
  final painter = QrPainter(
    data: data,
    version: QrVersions.auto,
    gapless: true,
    color: const Color(0xFF000000),
    emptyColor: const Color(0xFFFFFFFF),
  );
  final bytes = await painter.toImageData(512);
  if (bytes == null) {
    throw StateError('QR tasvirini chizib bo\'lmadi');
  }
  return bytes.buffer.asUint8List();
}

String _zoneOf(Map<String, dynamic> table) {
  final z = (table['zone'] as String?)?.trim() ?? '';
  return z.isEmpty ? kDefaultZone : z;
}

Future<String?> _askLabel(
  BuildContext context, {
  required String title,
  String initial = '',
}) async {
  final ctrl = TextEditingController(text: initial);
  final result = await showDialog<String>(
    context: context,
    builder: (ctx) => AlertDialog(
      title: Text(title),
      content: TextField(
        controller: ctrl,
        autofocus: true,
        keyboardType: TextInputType.number,
        decoration: const InputDecoration(
          labelText: 'Stol raqami',
          hintText: 'masalan: 5',
        ),
        onSubmitted: (v) => Navigator.of(ctx).pop(v.trim()),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(ctx).pop(),
          child: const Text('Bekor qilish'),
        ),
        FilledButton(
          onPressed: () => Navigator.of(ctx).pop(ctrl.text.trim()),
          child: const Text('Saqlash'),
        ),
      ],
    ),
  );
  ctrl.dispose();
  return (result == null || result.isEmpty) ? null : result;
}

Future<bool> _confirm(
  BuildContext context, {
  required String title,
  required String message,
}) async {
  final ok = await showDialog<bool>(
    context: context,
    builder: (ctx) => AlertDialog(
      title: Text(title),
      content: Text(message),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(ctx).pop(false),
          child: const Text('Bekor qilish'),
        ),
        FilledButton(
          onPressed: () => Navigator.of(ctx).pop(true),
          child: const Text('Davom etish'),
        ),
      ],
    ),
  );
  return ok == true;
}

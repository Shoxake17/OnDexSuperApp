import 'package:flutter/material.dart';
import 'package:qr_flutter/qr_flutter.dart';

import '../api.dart';
import '../theme.dart';

/// Stollar va ularning QR kodlari.
///
/// ┌─ QR KOD NIMA QILADI ──────────────────────────────────────────────┐
/// QR ichida `https://t.me/<bot>/<short_name>?startapp=<token>` havolasi
/// bor (hozir short_name = `ondex`, BotFather'dagi nom bilan aynan mos
/// bo'lishi SHART — serverda `miniAppShortName`).
/// Mijoz uni kamera bilan skanerlaydi → Telegram ochiladi → Mini App
/// avtomatik kirib, AYNAN shu stolning menyusini ko'rsatadi.
///
/// `<token>` — 32 baytlik tasodifiy sir. Stol ID'si EMAS: ID taxmin
/// qilinadigan bo'lsa, istalgan odam boshqa stol nomidan buyurtma
/// bera olardi.
/// └───────────────────────────────────────────────────────────────────┘
class TablesPage extends StatefulWidget {
  const TablesPage({super.key});

  @override
  State<TablesPage> createState() => _TablesPageState();
}

class _TablesPageState extends State<TablesPage> {
  List<Map<String, dynamic>> _tables = [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final list = await api.tables();
      if (!mounted) return;
      setState(() {
        _tables = list.cast<Map<String, dynamic>>();
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

  Future<void> _addTable() async {
    final label = await _askLabel(context, title: 'Yangi stol');
    if (label == null) return;
    try {
      await api.createTable(label);
      await _load();
    } catch (e) {
      _toast('$e');
    }
  }

  Future<void> _rename(Map<String, dynamic> t) async {
    final label = await _askLabel(
      context,
      title: 'Stol nomini o\'zgartirish',
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
      message: '"${t['label']}" stoli o\'chiriladi.\n\n'
          'DIQQAT: menyu varaqasiga chop etilgan QR kod ABADIY ishlamay '
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
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            const Text(
              'Stollar (QR kod)',
              style: TextStyle(fontSize: 22, fontWeight: FontWeight.bold),
            ),
            const Spacer(),
            FilledButton.icon(
              onPressed: _addTable,
              icon: const Icon(Icons.add),
              label: const Text('Stol qo\'shish'),
            ),
          ],
        ),
        const SizedBox(height: 6),
        const Text(
          'Har bir stolga QR kod chop etib, stol ustiga qo\'ying. '
          'Mijoz uni skanerlab, o\'sha stolga buyurtma beradi.',
          style: TextStyle(color: Colors.grey),
        ),
        const SizedBox(height: 16),
        if (_error != null)
          Padding(
            padding: const EdgeInsets.only(bottom: 12),
            child: Text(_error!, style: const TextStyle(color: Colors.red)),
          ),
        Expanded(
          child: _tables.isEmpty
              ? const Center(
                  child: Text(
                    'Hali stol qo\'shilmagan',
                    style: TextStyle(color: Colors.grey),
                  ),
                )
              : GridView.builder(
                  gridDelegate:
                      const SliverGridDelegateWithMaxCrossAxisExtent(
                    maxCrossAxisExtent: 280,
                    mainAxisExtent: 360,
                    crossAxisSpacing: 12,
                    mainAxisSpacing: 12,
                  ),
                  itemCount: _tables.length,
                  itemBuilder: (_, i) => _TableCard(
                    table: _tables[i],
                    onRename: () => _rename(_tables[i]),
                    onToggleActive: () => _toggleActive(_tables[i]),
                    onDelete: () => _delete(_tables[i]),
                  ),
                ),
        ),
      ],
    );
  }
}

class _TableCard extends StatelessWidget {
  const _TableCard({
    required this.table,
    required this.onRename,
    required this.onToggleActive,
    required this.onDelete,
  });

  final Map<String, dynamic> table;
  final VoidCallback onRename;
  final VoidCallback onToggleActive;
  final VoidCallback onDelete;

  @override
  Widget build(BuildContext context) {
    final active = table['active'] == true;
    final label = table['label'] as String? ?? '';
    // `qr_link` serverда bot nomi aniqlanganda qo'shiladi. Bo'lmasa —
    // QR chizmaymiz va SABABINI aytamiz: bo'sh joy ko'rsatib qo'yish
    // restoranni "nega QR yo'q?" deb o'ylantirib qo'yardi.
    final link = table['qr_link'] as String?;

    return Card(
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(14),
        side: BorderSide(
          color: active ? Colors.white24 : OnDexColors.danger.withValues(alpha: 0.6),
        ),
      ),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          children: [
            Row(
              children: [
                Text(
                  '$label-stol',
                  style: const TextStyle(
                    fontSize: 17,
                    fontWeight: FontWeight.bold,
                  ),
                ),
                const Spacer(),
                if (!active)
                  const Text(
                    'Faol emas',
                    style: TextStyle(color: Colors.orange, fontSize: 12),
                  ),
                PopupMenuButton<String>(
                  onSelected: (v) => switch (v) {
                    'rename' => onRename(),
                    'active' => onToggleActive(),
                    'delete' => onDelete(),
                    _ => null,
                  },
                  // "QR kodni yangilash" bandi ATAYLAB yo'q: QR menyu
                  // varaqasiga chop etilgan va abadiy o'zgarmaydi.
                  itemBuilder: (_) => [
                    const PopupMenuItem(
                      value: 'rename',
                      child: Text('Nomini o\'zgartirish'),
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
                    // Oq fon MAJBURIY: QR skanerlari qorong'i fonda
                    // teskari kontrastli kodni ko'pincha o'qiy olmaydi,
                    // panel esa qorong'i mavzuda.
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
              child: OutlinedButton.icon(
                onPressed: link == null
                    ? null
                    : () => _showPrintable(context, label, link),
                icon: const Icon(Icons.print, size: 18),
                label: const Text('Chop etish'),
              ),
            ),
          ],
        ),
      ),
    );
  }

  /// Chop etishga tayyor ko'rinish — katta QR va stol raqami.
  ///
  /// Brauzerning o'z chop etish oynasi ishlatiladi (Ctrl+P): alohida
  /// PDF kutubxonasi qo'shish shu bitta ekran uchun ortiqcha bo'lardi.
  static void _showPrintable(BuildContext context, String label, String link) {
    showDialog<void>(
      context: context,
      builder: (ctx) => Dialog(
        backgroundColor: Colors.white,
        child: Padding(
          padding: const EdgeInsets.all(28),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                '$label-STOL',
                style: const TextStyle(
                  fontSize: 30,
                  fontWeight: FontWeight.bold,
                  color: Colors.black,
                ),
              ),
              const SizedBox(height: 6),
              const Text(
                'Menyu uchun QR kodni skanerlang',
                style: TextStyle(color: Colors.black54),
              ),
              const SizedBox(height: 16),
              QrImageView(
                data: link,
                version: QrVersions.auto,
                size: 320,
                backgroundColor: Colors.white,
              ),
              const SizedBox(height: 16),
              const Text(
                'Kamerangizni QR kodga tuting',
                style: TextStyle(color: Colors.black87, fontSize: 15),
              ),
              const SizedBox(height: 20),
              Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  TextButton(
                    onPressed: () => Navigator.of(ctx).pop(),
                    child: const Text('Yopish'),
                  ),
                  const SizedBox(width: 12),
                  const Text(
                    'Chop etish uchun Ctrl+P',
                    style: TextStyle(color: Colors.black45, fontSize: 12),
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
        decoration: const InputDecoration(
          labelText: 'Stol nomi',
          hintText: 'masalan: 5, VIP-2, Ayvon 3',
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

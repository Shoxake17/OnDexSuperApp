import 'package:flutter/material.dart';

import '../api.dart';
import '../widgets/password_field.dart';

/// "Do'kon" bo'limi: do'konlarni ro'yxatdan o'tkazish.
///
/// Restoran kabi: do'kon o'zi ro'yxatdan o'tmaydi, akkauntni faqat
/// superadmin yaratadi. Bu OnDex akkaunti — egasi shu telefon (+ parol)
/// bilan restoran paneli ilovasiga ham, veb Merchant Panelga ham kiradi
/// (veb panel yagona kirish orqali shu akkauntni taniydi).
/// Do'kon kategoriyalari (backend `users.StoreCategory` bilan AYNAN bir xil
/// qiymatlar). Kategoriya POS panelida qaysi sahifalar ochiq ekanligini
/// belgilaydi: "Qurt" — cheklangan panel (mahsulot qo'shish/bazasi, sotuvlar,
/// kirim, statistika, qarzlar, sozlamalar); "Do'kon" — to'liq panel;
/// "Dorixona" — dorixona paneli.
const storeCategories = <({String value, String title, String hint})>[
  (
    value: 'qurt',
    title: 'Qurt',
    hint: 'Cheklangan panel: mahsulot, sotuvlar, kirim, statistika, qarzlar',
  ),
  (value: 'store', title: 'Do\'kon', hint: 'To\'liq do\'kon paneli'),
  (value: 'pharmacy', title: 'Dorixona', hint: 'Dorixona paneli'),
];

String storeCategoryTitle(String? value) => storeCategories
    .firstWhere((c) => c.value == value,
        orElse: () => (value: '', title: '—', hint: ''))
    .title;

class StoresPage extends StatefulWidget {
  const StoresPage({super.key});

  @override
  State<StoresPage> createState() => _StoresPageState();
}

class _StoresPageState extends State<StoresPage> {
  List<dynamic> _list = [];
  bool _loading = true;
  String _query = '';

  @override
  void initState() {
    super.initState();
    _load();
  }

  void _snack(String msg) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg)));
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      final list = await api.storeAccounts();
      if (mounted) setState(() => _list = list);
    } catch (e) {
      _snack('Do\'konlarni yuklab bo\'lmadi: $e');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _showCreateDialog() async {
    final name = TextEditingController();
    final phone = TextEditingController(text: '+998');
    final password = TextEditingController();
    String? category;

    final created = await showDialog<bool>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setDialogState) => AlertDialog(
          title: const Text('Yangi do\'kon qo\'shish'),
          content: SizedBox(
            width: 420,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                DropdownButtonFormField<String>(
                  initialValue: category,
                  decoration: InputDecoration(
                    labelText: 'Kategoriya',
                    helperText: category == null
                        ? 'Majburiy: panelda qaysi sahifalar ochiqligini belgilaydi'
                        : storeCategories
                            .firstWhere((c) => c.value == category)
                            .hint,
                  ),
                  items: [
                    for (final c in storeCategories)
                      DropdownMenuItem(value: c.value, child: Text(c.title)),
                  ],
                  onChanged: (v) => setDialogState(() => category = v),
                ),
                const SizedBox(height: 8),
                TextField(
                  controller: name,
                  decoration: const InputDecoration(labelText: 'Do\'kon nomi'),
                ),
                const SizedBox(height: 8),
                TextField(
                  controller: phone,
                  keyboardType: TextInputType.phone,
                  decoration: const InputDecoration(
                    labelText: 'Akkaunt telefon raqami',
                    helperText: 'Do\'kon egasi shu raqam bilan kiradi',
                  ),
                ),
                const SizedBox(height: 8),
                PasswordField(
                  controller: password,
                  outlined: false,
                  autofillHints: const [AutofillHints.newPassword],
                  label: 'Boshlang\'ich parol (ixtiyoriy)',
                  helperText:
                      'Kamida 8 belgi. Bo\'lsa — telefon + parol bilan kiradi, '
                      'bo\'lmasa faqat Telegram/SMS kod bilan',
                ),
              ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('Bekor qilish'),
            ),
            FilledButton(
              onPressed: () => Navigator.pop(ctx, true),
              child: const Text('Yaratish'),
            ),
          ],
        ),
      ),
    );
    if (created != true) return;
    if (category == null) {
      _snack('Kategoriyani tanlang');
      return;
    }
    if (name.text.trim().isEmpty) {
      _snack('Do\'kon nomini kiriting');
      return;
    }
    try {
      final res = await api.createStore(
        name: name.text.trim(),
        phone: phone.text.trim(),
        category: category!,
        password: password.text,
      );
      final acc = res['account'] as Map;
      _snack('Yaratildi! Kirish uchun: ${acc['phone']}');
      _load();
    } catch (e) {
      _snack('Xato: $e');
    }
  }

  /// Nom va kategoriyani o'zgartirish. Kategoriya POS panelida qaysi
  /// sahifalar ochiqligini belgilaydi; egasi tokenini yangilaganda kuchga kiradi.
  Future<void> _edit(Map row) async {
    final name = TextEditingController(text: '${row['name']}');
    String category = (row['category'] as String?) ?? '';
    if (!storeCategories.any((c) => c.value == category)) category = 'store';

    final saved = await showDialog<bool>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setDialogState) => AlertDialog(
          title: const Text('Do\'konni tahrirlash'),
          content: SizedBox(
            width: 420,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                TextField(
                  controller: name,
                  decoration: const InputDecoration(labelText: 'Do\'kon nomi'),
                ),
                const SizedBox(height: 8),
                DropdownButtonFormField<String>(
                  initialValue: category,
                  decoration: InputDecoration(
                    labelText: 'Kategoriya',
                    helperText: storeCategories
                        .firstWhere((c) => c.value == category)
                        .hint,
                  ),
                  items: [
                    for (final c in storeCategories)
                      DropdownMenuItem(value: c.value, child: Text(c.title)),
                  ],
                  onChanged: (v) =>
                      setDialogState(() => category = v ?? category),
                ),
              ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('Bekor qilish'),
            ),
            FilledButton(
              onPressed: () => Navigator.pop(ctx, true),
              child: const Text('Saqlash'),
            ),
          ],
        ),
      ),
    );
    if (saved != true) return;
    if (name.text.trim().isEmpty) {
      _snack('Do\'kon nomini kiriting');
      return;
    }
    try {
      await api.updateStore(row['entity_id'] as String,
          name: name.text.trim(), category: category);
      _snack('Saqlandi. POS panelida egasi tokenini yangilaganda kuchga kiradi '
          '(qayta kirsa — darhol).');
      _load();
    } catch (e) {
      _snack('Xato: $e');
    }
  }

  Future<void> _delete(Map row) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Do\'konni o\'chirish'),
        content: Text(
            '"${row['name']}" akkaunti o\'chiriladi va u hamma qurilmadan chiqariladi. '
            'Veb paneldagi do\'kon ma\'lumotlari saqlanib qoladi.'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Bekor qilish'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('O\'chirish'),
          ),
        ],
      ),
    );
    if (ok != true) return;
    try {
      await api.deleteStore(row['entity_id'] as String);
      _snack('Do\'kon o\'chirildi');
      _load();
    } catch (e) {
      _snack('Xato: $e');
    }
  }

  @override
  Widget build(BuildContext context) {
    final q = _query.trim().toLowerCase();
    final rows = _list
        .whereType<Map>()
        .where((r) =>
            q.isEmpty ||
            '${r['name']}'.toLowerCase().contains(q) ||
            '${r['phone']}'.contains(q))
        .toList();

    return Scaffold(
      floatingActionButton: FloatingActionButton.extended(
        onPressed: _showCreateDialog,
        icon: const Icon(Icons.add_business_outlined),
        label: const Text('Do\'kon qo\'shish'),
      ),
      body: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Text('Do\'konlar',
                    style: Theme.of(context).textTheme.headlineSmall),
                const Spacer(),
                SizedBox(
                  width: 260,
                  child: TextField(
                    onChanged: (v) => setState(() => _query = v),
                    decoration: const InputDecoration(
                      hintText: 'Nom yoki telefon bo\'yicha qidirish',
                      prefixIcon: Icon(Icons.search),
                      isDense: true,
                    ),
                  ),
                ),
                IconButton(
                  tooltip: 'Yangilash',
                  onPressed: _load,
                  icon: const Icon(Icons.refresh),
                ),
              ],
            ),
            const SizedBox(height: 12),
            Expanded(
              child: _loading
                  ? const Center(child: CircularProgressIndicator())
                  : rows.isEmpty
                      ? const Center(child: Text('Hozircha do\'kon yo\'q'))
                      : ListView.separated(
                          itemCount: rows.length,
                          separatorBuilder: (_, __) => const Divider(height: 1),
                          itemBuilder: (_, i) {
                            final r = rows[i];
                            return ListTile(
                              leading: const Icon(Icons.store_outlined),
                              title: Text('${r['name']}'),
                              subtitle: Text(
                                  '${r['phone']} · ${storeCategoryTitle(r['category'] as String?)}'),
                              trailing: Row(
                                mainAxisSize: MainAxisSize.min,
                                children: [
                                  IconButton(
                                    tooltip: 'Tahrirlash (nom, kategoriya)',
                                    icon: const Icon(Icons.edit_outlined),
                                    onPressed: () => _edit(r),
                                  ),
                                  IconButton(
                                    tooltip: 'O\'chirish',
                                    icon: const Icon(Icons.delete_outline),
                                    onPressed: () => _delete(r),
                                  ),
                                ],
                              ),
                            );
                          },
                        ),
            ),
          ],
        ),
      ),
    );
  }
}

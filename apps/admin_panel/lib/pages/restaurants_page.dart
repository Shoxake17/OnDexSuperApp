import 'package:flutter/material.dart';

import '../api.dart';
import '../widgets/map_picker.dart';

class RestaurantsPage extends StatefulWidget {
  const RestaurantsPage({super.key});

  @override
  State<RestaurantsPage> createState() => _RestaurantsPageState();
}

class _RestaurantsPageState extends State<RestaurantsPage> {
  List<dynamic> _list = [];
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final l = await api.restaurants();
      if (!mounted) return;
      setState(() {
        _list = l;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() => _loading = false);
      _snack('Xato: $e');
    }
  }

  void _snack(String msg) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg)));
  }

  Future<void> _toggleOpen(Map<String, dynamic> r, bool open) async {
    try {
      await api.setRestaurantOpen(r['id'], open);
      _load();
    } catch (e) {
      _snack('Xato: $e');
    }
  }

  Future<void> _confirmDelete(Map<String, dynamic> r) async {
    final yes = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        icon: const Icon(Icons.warning_amber_rounded,
            color: Colors.red, size: 40),
        title: Text('"${r['name']}" o\'chirilsinmi?'),
        content: const Text(
            'Restoran bilan birga uning menyusi va kirish akkaunti ham '
            'o\'chib ketadi. Eski buyurtmalar tarixi saqlanadi.\n\n'
            'Bu amalni ORTGA QAYTARIB BO\'LMAYDI.'),
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
      await api.deleteRestaurant(r['id']);
      _snack('"${r['name']}" o\'chirildi');
      _load();
    } catch (e) {
      _snack('Xato: $e'); // masalan: faol buyurtmalari bor
    }
  }

  Future<void> _showCreateDialog() async {
    final name = TextEditingController();
    final address = TextEditingController();
    final phone = TextEditingController(text: '+998');
    final staff = TextEditingController();
    double? lat;
    double? lng;

    final created = await showDialog<bool>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setDialogState) => AlertDialog(
          title: const Text('Yangi restoran qo\'shish'),
          content: SizedBox(
            width: 420,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                TextField(
                    controller: name,
                    decoration:
                        const InputDecoration(labelText: 'Restoran nomi')),
                const SizedBox(height: 8),
                // Manzil xaritadan tanlanadi: input bosilganda xarita ochiladi,
                // tanlangan joyning manzili va koordinatasi avtomatik to'ladi.
                TextField(
                  controller: address,
                  readOnly: true,
                  onTap: () async {
                    final picked = await showMapPicker(ctx);
                    if (picked != null) {
                      setDialogState(() {
                        lat = picked.lat;
                        lng = picked.lng;
                        address.text = picked.address.isEmpty
                            ? '${picked.lat.toStringAsFixed(5)}, ${picked.lng.toStringAsFixed(5)}'
                            : picked.address;
                      });
                    }
                  },
                  decoration: InputDecoration(
                    labelText: 'Manzil',
                    hintText: 'Bosing — xarita ochiladi',
                    helperText: lat == null
                        ? 'Joy xaritadan belgilanadi'
                        : 'Koordinata: ${lat!.toStringAsFixed(5)}, ${lng!.toStringAsFixed(5)}',
                    suffixIcon: const Icon(Icons.map_outlined),
                  ),
                ),
                const SizedBox(height: 8),
                TextField(
                    controller: phone,
                    decoration: const InputDecoration(
                        labelText: 'Akkaunt telefon raqami',
                        helperText:
                            'Restoran shu raqam bilan o\'z paneliga kiradi (SMS kod)')),
                const SizedBox(height: 8),
                TextField(
                    controller: staff,
                    decoration:
                        const InputDecoration(labelText: 'Mas\'ul shaxs ismi')),
              ],
            ),
          ),
          actions: [
            TextButton(
                onPressed: () => Navigator.pop(ctx, false),
                child: const Text('Bekor qilish')),
            FilledButton(
                onPressed: () => Navigator.pop(ctx, true),
                child: const Text('Yaratish')),
          ],
        ),
      ),
    );
    if (created != true) return;
    if (lat == null || lng == null) {
      _snack('Restoran joyi xaritadan belgilanmadi — Manzil maydonini bosing');
      return;
    }
    try {
      final res = await api.createRestaurant(
        name: name.text.trim(),
        address: address.text.trim(),
        phone: phone.text.trim(),
        staffName: staff.text.trim(),
        lat: lat!,
        lng: lng!,
      );
      final acc = res['account'] as Map;
      _snack(
          'Yaratildi! Kirish uchun: ${acc['phone']} — restoranga shu raqamni bering');
      _load();
    } catch (e) {
      _snack('Xato: $e');
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      floatingActionButton: FloatingActionButton.extended(
        onPressed: _showCreateDialog,
        icon: const Icon(Icons.add),
        label: const Text('Restoran qo\'shish'),
      ),
      body: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Text('Restoranlar',
                    style: Theme.of(context).textTheme.headlineMedium),
                const Spacer(),
                IconButton(
                    onPressed: _load, icon: const Icon(Icons.refresh)),
              ],
            ),
            const SizedBox(height: 8),
            Text(
                'Restoranlar o\'zi ro\'yxatdan o\'tmaydi — akkauntni shu yerdan yaratasiz.',
                style: Theme.of(context).textTheme.bodySmall),
            const SizedBox(height: 16),
            Expanded(
              child: _loading
                  ? const Center(child: CircularProgressIndicator())
                  : _list.isEmpty
                      ? const Center(child: Text('Hozircha restoran yo\'q'))
                      : SingleChildScrollView(
                          child: SizedBox(
                            width: double.infinity,
                            child: DataTable(
                              columns: const [
                                DataColumn(label: Text('Nomi')),
                                DataColumn(label: Text('Manzil')),
                                DataColumn(label: Text('Holat')),
                                DataColumn(label: Text('Amal')),
                              ],
                              rows: [
                                for (final r in _list.cast<Map<String, dynamic>>())
                                  DataRow(cells: [
                                    DataCell(Text(r['name'] ?? '')),
                                    DataCell(Text(r['address'] ?? '')),
                                    DataCell(Switch(
                                      value: r['open'] == true,
                                      onChanged: (v) => _toggleOpen(r, v),
                                    )),
                                    DataCell(IconButton(
                                      tooltip: 'Restoranni o\'chirish',
                                      icon: const Icon(Icons.delete_outline,
                                          color: Colors.red),
                                      onPressed: () => _confirmDelete(r),
                                    )),
                                  ]),
                              ],
                            ),
                          ),
                        ),
            ),
          ],
        ),
      ),
    );
  }
}

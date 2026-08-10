import 'dart:typed_data';

import 'package:file_picker/file_picker.dart';
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
    if (!mounted) return;
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
      _snack('Xato: $e');
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
          'Yaratildi! Kirish uchun: ${acc['phone']} — restoranga shu raqamni bering. '
          'Endi "Tahrirlash" orqali logo va banner qo\'shishingiz mumkin.');
      _load();
    } catch (e) {
      _snack('Xato: $e');
    }
  }

  Future<void> _showEditDialog(Map<String, dynamic> r) async {
    final saved = await showDialog<bool>(
      context: context,
      builder: (_) => _EditRestaurantDialog(restaurant: r),
    );
    if (saved == true) {
      _snack('Saqlandi');
      _load();
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
                          child: SingleChildScrollView(
                            scrollDirection: Axis.horizontal,
                            child: DataTable(
                              columns: const [
                                DataColumn(label: Text('Logo')),
                                DataColumn(label: Text('Nomi')),
                                DataColumn(label: Text('Manzil')),
                                DataColumn(label: Text('Holat')),
                                DataColumn(label: Text('Amal')),
                              ],
                              rows: [
                                for (final r in _list.cast<Map<String, dynamic>>())
                                  DataRow(cells: [
                                    DataCell(_LogoThumb(
                                        url: r['logo_url'] as String? ?? '')),
                                    DataCell(Text(r['name'] ?? '')),
                                    DataCell(Text(r['address'] ?? '')),
                                    DataCell(Switch(
                                      value: r['open'] == true,
                                      onChanged: (v) => _toggleOpen(r, v),
                                    )),
                                    DataCell(Row(
                                      mainAxisSize: MainAxisSize.min,
                                      children: [
                                        IconButton(
                                          tooltip: 'Tahrirlash',
                                          icon: const Icon(
                                              Icons.edit_outlined),
                                          onPressed: () =>
                                              _showEditDialog(r),
                                        ),
                                        IconButton(
                                          tooltip: 'Restoranni o\'chirish',
                                          icon: const Icon(
                                              Icons.delete_outline,
                                              color: Colors.red),
                                          onPressed: () => _confirmDelete(r),
                                        ),
                                      ],
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

class _LogoThumb extends StatelessWidget {
  final String url;
  const _LogoThumb({required this.url});

  @override
  Widget build(BuildContext context) {
    return ClipRRect(
      borderRadius: BorderRadius.circular(8),
      child: SizedBox(
        width: 40,
        height: 40,
        child: url.isEmpty
            ? Container(
                color: Colors.grey.shade200,
                child:
                    const Icon(Icons.storefront, color: Colors.grey, size: 20),
              )
            : Image.network(
                imageUrl(url),
                fit: BoxFit.cover,
                errorBuilder: (_, __, ___) => Container(
                  color: Colors.grey.shade200,
                  child: const Icon(Icons.storefront,
                      color: Colors.grey, size: 20),
                ),
              ),
      ),
    );
  }
}

/// Restoranni tahrirlash oynasi: nomi, manzili/joylashuvi (xarita orqali),
/// logo (kvadrat) va cover/banner (keng) rasmlari.
class _EditRestaurantDialog extends StatefulWidget {
  final Map<String, dynamic> restaurant;
  const _EditRestaurantDialog({required this.restaurant});

  @override
  State<_EditRestaurantDialog> createState() => _EditRestaurantDialogState();
}

class _EditRestaurantDialogState extends State<_EditRestaurantDialog> {
  late final TextEditingController _name;
  late final TextEditingController _address;
  late final TextEditingController _tags;
  late double _lat;
  late double _lng;

  Uint8List? _logoBytes;
  String? _logoName;
  late String _existingLogoUrl;

  Uint8List? _coverBytes;
  String? _coverName;
  late String _existingCoverUrl;

  bool _saving = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    final r = widget.restaurant;
    _name = TextEditingController(text: r['name'] ?? '');
    _address = TextEditingController(text: r['address'] ?? '');
    _tags = TextEditingController(text: r['tags'] ?? '');
    _lat = (r['lat'] as num?)?.toDouble() ?? 41.0030;
    _lng = (r['lng'] as num?)?.toDouble() ?? 71.2360;
    _existingLogoUrl = r['logo_url'] as String? ?? '';
    _existingCoverUrl = r['cover_url'] as String? ?? '';
  }

  Future<void> _pickLogo() async {
    final result =
        await FilePicker.platform.pickFiles(type: FileType.image, withData: true);
    final f = result?.files.firstOrNull;
    if (f?.bytes == null) return;
    setState(() {
      _logoBytes = f!.bytes;
      _logoName = f.name;
    });
  }

  Future<void> _pickCover() async {
    final result =
        await FilePicker.platform.pickFiles(type: FileType.image, withData: true);
    final f = result?.files.firstOrNull;
    if (f?.bytes == null) return;
    setState(() {
      _coverBytes = f!.bytes;
      _coverName = f.name;
    });
  }

  Future<void> _pickLocation() async {
    final picked = await showMapPicker(context);
    if (picked != null) {
      setState(() {
        _lat = picked.lat;
        _lng = picked.lng;
        if (picked.address.isNotEmpty) _address.text = picked.address;
      });
    }
  }

  Future<void> _submit() async {
    final name = _name.text.trim();
    if (name.isEmpty) {
      setState(() => _error = 'Restoran nomini kiriting');
      return;
    }
    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      var logoUrl = _existingLogoUrl;
      if (_logoBytes != null) {
        logoUrl = await api.uploadImage(_logoBytes!, _logoName ?? 'logo.jpg',
            type: 'logo');
      }
      var coverUrl = _existingCoverUrl;
      if (_coverBytes != null) {
        coverUrl = await api.uploadImage(_coverBytes!, _coverName ?? 'cover.jpg',
            type: 'cover');
      }
      await api.editRestaurant(
        id: widget.restaurant['id'],
        name: name,
        address: _address.text.trim(),
        lat: _lat,
        lng: _lng,
        logoUrl: logoUrl,
        coverUrl: coverUrl,
        tags: _tags.text.trim(),
      );
      if (!mounted) return;
      Navigator.pop(context, true);
    } on ApiException catch (e) {
      setState(() => _error = e.message);
    } catch (_) {
      setState(() => _error = 'Xato yuz berdi, qayta urinib ko\'ring');
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Restoranni tahrirlash'),
      content: SizedBox(
        width: 460,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text('Cover (banner) — mijoz ilovasida restoran sahifasi tepasida',
                  style: Theme.of(context).textTheme.labelLarge),
              const SizedBox(height: 6),
              _ImagePickerBox(
                aspectRatio: 2.4,
                pickedBytes: _coverBytes,
                existingUrl: _existingCoverUrl,
                onTap: _saving ? null : _pickCover,
                placeholderText: 'Banner rasm qo\'shish uchun bosing',
              ),
              const SizedBox(height: 20),
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  SizedBox(
                    width: 110,
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        Text('Logo', style: Theme.of(context).textTheme.labelLarge),
                        const SizedBox(height: 6),
                        _ImagePickerBox(
                          aspectRatio: 1,
                          pickedBytes: _logoBytes,
                          existingUrl: _existingLogoUrl,
                          onTap: _saving ? null : _pickLogo,
                          placeholderText: 'Logo',
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(width: 16),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        TextField(
                          controller: _name,
                          decoration: const InputDecoration(
                              labelText: 'Restoran nomi',
                              border: OutlineInputBorder()),
                        ),
                        const SizedBox(height: 8),
                        TextField(
                          controller: _address,
                          readOnly: true,
                          onTap: _saving ? null : _pickLocation,
                          decoration: InputDecoration(
                            labelText: 'Manzil',
                            border: const OutlineInputBorder(),
                            suffixIcon: const Icon(Icons.map_outlined),
                            helperText:
                                'Koordinata: ${_lat.toStringAsFixed(5)}, ${_lng.toStringAsFixed(5)}',
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 8),
              TextField(
                controller: _tags,
                decoration: const InputDecoration(
                  labelText: 'Kategoriya (ixtiyoriy)',
                  hintText: 'Masalan: Fastfud, Pishiriqlar',
                  helperText:
                      'Mijoz ilovasida restoranlar ro\'yxatida filtr sifatida ko\'rinadi',
                  border: OutlineInputBorder(),
                ),
              ),
              if (_error != null) ...[
                const SizedBox(height: 12),
                Text(_error!,
                    style: TextStyle(color: Theme.of(context).colorScheme.error)),
              ],
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: _saving ? null : () => Navigator.pop(context, false),
          child: const Text('Bekor qilish'),
        ),
        FilledButton(
          onPressed: _saving ? null : _submit,
          child: Text(_saving ? 'Saqlanmoqda...' : 'Saqlash'),
        ),
      ],
    );
  }
}

/// Qayta ishlatiladigan rasm tanlash oynasi: yumaloq burchak, chegara
/// har doim rasm ustida ko'rinadi (DecorationPosition.foreground) —
/// avval restoran panelida tuzatilgan xatoning oldini oladi.
class _ImagePickerBox extends StatelessWidget {
  final double aspectRatio;
  final Uint8List? pickedBytes;
  final String existingUrl;
  final VoidCallback? onTap;
  final String placeholderText;

  const _ImagePickerBox({
    required this.aspectRatio,
    required this.pickedBytes,
    required this.existingUrl,
    required this.onTap,
    required this.placeholderText,
  });

  @override
  Widget build(BuildContext context) {
    Widget content;
    if (pickedBytes != null) {
      content = Image.memory(pickedBytes!,
          fit: BoxFit.cover, width: double.infinity, height: double.infinity);
    } else if (existingUrl.isNotEmpty) {
      content = Image.network(
        imageUrl(existingUrl),
        fit: BoxFit.cover,
        width: double.infinity,
        height: double.infinity,
        errorBuilder: (_, __, ___) => _placeholder(),
      );
    } else {
      content = _placeholder();
    }

    return AspectRatio(
      aspectRatio: aspectRatio,
      child: ClipRRect(
        borderRadius: BorderRadius.circular(16),
        child: DecoratedBox(
          // Chegara rasm ustida — DecorationPosition.foreground bo'lmasa
          // to'g'ri qirralarda rasm foni chegarani bosib qoladi (avval
          // restoran panelida topilgan va tuzatilgan xato).
          position: DecorationPosition.foreground,
          decoration: BoxDecoration(
            border: Border.all(
                color: Theme.of(context).colorScheme.outline, width: 2),
            borderRadius: BorderRadius.circular(16),
          ),
          child: Material(
            color: Colors.white,
            child: InkWell(onTap: onTap, child: content),
          ),
        ),
      ),
    );
  }

  Widget _placeholder() {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const Icon(Icons.add_photo_alternate_outlined,
              size: 28, color: Colors.grey),
          const SizedBox(height: 4),
          Text(placeholderText,
              textAlign: TextAlign.center,
              style: const TextStyle(color: Colors.grey, fontSize: 11)),
        ],
      ),
    );
  }
}

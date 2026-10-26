import 'dart:typed_data';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';

import '../api.dart';

/// Reklama/aksiya bannerlari — mijoz ilovasining restoran sahifasidagi
/// reklama bloki shu yerdan boshqariladi (Yandex Eats/Go uslubi).
///
/// ┌─ NEGA ADMIN PANELGA KO'CHIRILDI ───────────────────────────────────┐
/// Avval bu blok mijoz ilovasi kodida QATTIQ YOZILGAN edi (matn ham,
/// rasm ham). Yangi aksiya chiqarish uchun ilovani qayta qurish kerak
/// bo'lardi. Endi banner — oddiy ma'lumot: rasm yuklanadi, ixtiyoriy
/// ravishda bitta restoranga bog'lanadi (bosilganda o'sha restoran
/// menyusi ochiladi), yoqilib-o'chiriladi va tartibi o'zgartiriladi.
/// Mijoz ilovasi serverdan FAOL bannerlarni o'qib ko'rsatadi — kod
/// o'zgarmaydi.
/// └────────────────────────────────────────────────────────────────────┘
class BannersPage extends StatefulWidget {
  const BannersPage({super.key});

  @override
  State<BannersPage> createState() => _BannersPageState();
}

class _BannersPageState extends State<BannersPage> {
  List<dynamic> _banners = [];
  List<dynamic> _restaurants = [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      // Ikkalasi parallel: restoranlar ro'yxati faqat tanlov oynasida
      // kerak, lekin bannerlar ko'rinishidan OLDIN tayyor bo'lishi
      // kerak (nom ko'rsatish uchun).
      final results = await Future.wait([api.banners(), api.restaurants()]);
      if (!mounted) return;
      setState(() {
        _banners = results[0];
        _restaurants = results[1];
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = '$e';
        _loading = false;
      });
    }
  }

  String _restaurantName(String id) {
    if (id.isEmpty) return '';
    for (final r in _restaurants) {
      if ('${r['id']}' == id) return '${r['name']}';
    }
    return id;
  }

  Future<void> _edit({Map<String, dynamic>? banner}) async {
    final saved = await showDialog<bool>(
      context: context,
      builder: (_) =>
          _BannerDialog(restaurants: _restaurants, banner: banner),
    );
    if (saved == true) await _load();
  }

  Future<void> _delete(Map<String, dynamic> banner) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Bannerni o\'chirish'),
        content: const Text('Bu banner mijoz ilovasidan olib tashlansinmi?'),
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
      await api.deleteBanner('${banner['id']}');
      await _load();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text('O\'chirilmadi: $e')));
    }
  }

  /// Faollik — ro'yxatdagi tugmadan DARHOL, dialogsiz.
  Future<void> _toggleActive(Map<String, dynamic> b, bool active) async {
    try {
      await api.updateBanner(
        '${b['id']}',
        imageUrl: '${b['image_url']}',
        restaurantId: '${b['restaurant_id'] ?? ''}',
        active: active,
        sortOrder: (b['sort_order'] as num?)?.toInt() ?? 0,
      );
      await _load();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text('Saqlanmadi: $e')));
    }
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Text('Bannerlar',
                  style:
                      TextStyle(fontSize: 26, fontWeight: FontWeight.bold)),
              const Spacer(),
              FilledButton.icon(
                onPressed: () => _edit(),
                icon: const Icon(Icons.add),
                label: const Text('Banner qo\'shish'),
              ),
            ],
          ),
          const SizedBox(height: 8),
          const Text(
            'Mijoz ilovasining restoran sahifasidagi reklama bloki. '
            'Tartib raqami kichik banner YUQORIDA chiqadi. '
            'Restoran tanlansa, banner bosilganda o\'sha restoran menyusi '
            'ochiladi — bo\'sh qoldirilsa banner shunchaki ko\'rsatish uchun.',
            style: TextStyle(color: Colors.black54),
          ),
          const SizedBox(height: 16),
          Expanded(child: _buildBody()),
        ],
      ),
    );
  }

  Widget _buildBody() {
    if (_loading) return const Center(child: CircularProgressIndicator());
    if (_error != null) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text('Bannerlar yuklanmadi:\n$_error',
                textAlign: TextAlign.center,
                style: const TextStyle(color: Colors.red)),
            const SizedBox(height: 12),
            OutlinedButton(onPressed: _load, child: const Text('Qayta urinish')),
          ],
        ),
      );
    }
    if (_banners.isEmpty) {
      return const Center(
        child: Text('Hozircha banner yo\'q',
            style: TextStyle(color: Colors.black54)),
      );
    }
    return ListView.separated(
      itemCount: _banners.length,
      separatorBuilder: (_, __) => const Divider(height: 1),
      itemBuilder: (_, i) {
        final b = Map<String, dynamic>.from(_banners[i] as Map);
        final active = b['active'] == true;
        final image = '${b['image_url'] ?? ''}';
        final restaurantId = '${b['restaurant_id'] ?? ''}';
        final sortOrder = (b['sort_order'] as num?)?.toInt() ?? 0;
        return ListTile(
          // Banner KENG (852×286 ga yaqin) — ro'yxatda ham shu nisbatda.
          leading: SizedBox(
            width: 90,
            height: 30,
            child: ClipRRect(
              borderRadius: BorderRadius.circular(6),
              child: image.isEmpty
                  ? Container(
                      color: Colors.black12,
                      child: const Icon(Icons.image_outlined,
                          size: 16, color: Colors.black38),
                    )
                  : Image.network(
                      imageUrl(image),
                      fit: BoxFit.cover,
                      errorBuilder: (_, __, ___) => Container(
                        color: Colors.black12,
                        child: const Icon(Icons.broken_image_outlined,
                            size: 16, color: Colors.black26),
                      ),
                    ),
            ),
          ),
          title: Text(
            restaurantId.isEmpty
                ? 'Havolasiz banner'
                : _restaurantName(restaurantId),
          ),
          subtitle: Text([
            'tartib: $sortOrder',
            if (!active) 'o\'chirilgan',
          ].join(' · ')),
          trailing: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Switch(value: active, onChanged: (v) => _toggleActive(b, v)),
              IconButton(
                tooltip: 'Tahrirlash',
                icon: const Icon(Icons.edit_outlined),
                onPressed: () => _edit(banner: b),
              ),
              IconButton(
                tooltip: 'O\'chirish',
                icon: const Icon(Icons.delete_outline),
                onPressed: () => _delete(b),
              ),
            ],
          ),
        );
      },
    );
  }
}

/// Banner qo'shish/tahrirlash oynasi.
class _BannerDialog extends StatefulWidget {
  const _BannerDialog({required this.restaurants, this.banner});

  final List<dynamic> restaurants;
  final Map<String, dynamic>? banner;

  @override
  State<_BannerDialog> createState() => _BannerDialogState();
}

class _BannerDialogState extends State<_BannerDialog> {
  late String _imageUrl = '${widget.banner?['image_url'] ?? ''}';
  late String? _restaurantId = (widget.banner?['restaurant_id'] as String?)
          ?.isNotEmpty ==
      true
      ? widget.banner!['restaurant_id'] as String
      : null;
  late bool _active = widget.banner?['active'] != false;
  late final _sortOrder = TextEditingController(
      text: '${(widget.banner?['sort_order'] as num?)?.toInt() ?? 0}');

  Uint8List? _pickedBytes;
  bool _uploading = false;
  bool _saving = false;
  String? _error;

  @override
  void dispose() {
    _sortOrder.dispose();
    super.dispose();
  }

  /// Rasm tanlanadi va DARROV yuklanadi — xuddi kitob muqovasi bilan
  /// bir xil naqsh (`books_page.dart`): admin natijani saqlashdan
  /// oldin ko'radi.
  Future<void> _pickImage() async {
    final res = await FilePicker.platform
        .pickFiles(type: FileType.image, withData: true);
    final bytes = res?.files.single.bytes;
    if (bytes == null) return;

    setState(() {
      _pickedBytes = bytes;
      _uploading = true;
      _error = null;
    });
    try {
      // `type: 'cover'` — server rasmni KENG banner sifatida kesadi
      // (restoran muqovasi bilan bir xil ishlov, `internal/images`).
      final url = await api.uploadImage(
        bytes,
        res!.files.single.name,
        type: 'cover',
      );
      if (!mounted) return;
      setState(() {
        _imageUrl = url;
        _uploading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = 'Rasm yuklanmadi: $e';
        _pickedBytes = null;
        _uploading = false;
      });
    }
  }

  Future<void> _save() async {
    if (_imageUrl.isEmpty) {
      setState(() => _error = 'Banner rasmi yuklanmagan');
      return;
    }
    final sortOrder = int.tryParse(_sortOrder.text.trim()) ?? 0;

    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      final existing = widget.banner;
      if (existing == null) {
        await api.createBanner(
          imageUrl: _imageUrl,
          restaurantId: _restaurantId ?? '',
          active: _active,
          sortOrder: sortOrder,
        );
      } else {
        await api.updateBanner(
          '${existing['id']}',
          imageUrl: _imageUrl,
          restaurantId: _restaurantId ?? '',
          active: _active,
          sortOrder: sortOrder,
        );
      }
      if (!mounted) return;
      Navigator.pop(context, true);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = '$e';
        _saving = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final isNew = widget.banner == null;
    final busy = _saving || _uploading;
    return AlertDialog(
      title: Text(isNew ? 'Yangi banner' : 'Bannerni tahrirlash'),
      content: SizedBox(
        width: 560,
        child: SingleChildScrollView(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              _imagePreview(),
              const SizedBox(height: 16),
              DropdownButtonFormField<String?>(
                initialValue: _restaurantId,
                decoration: const InputDecoration(
                  labelText: 'Bosilganda ochiladigan restoran (ixtiyoriy)',
                  border: OutlineInputBorder(),
                ),
                items: [
                  const DropdownMenuItem(
                    value: null,
                    child: Text('— Havolasiz (faqat ko\'rsatish) —'),
                  ),
                  for (final r in widget.restaurants)
                    DropdownMenuItem(
                      value: '${r['id']}',
                      child: Text('${r['name']}'),
                    ),
                ],
                onChanged: (v) => setState(() => _restaurantId = v),
              ),
              const SizedBox(height: 14),
              Row(
                children: [
                  Expanded(
                    child: TextField(
                      controller: _sortOrder,
                      keyboardType: TextInputType.number,
                      decoration: const InputDecoration(
                        labelText: 'Tartib raqami (kichigi yuqorida)',
                        border: OutlineInputBorder(),
                      ),
                    ),
                  ),
                  const SizedBox(width: 16),
                  Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      const Text('Faol', style: TextStyle(fontSize: 12)),
                      Switch(
                        value: _active,
                        onChanged: (v) => setState(() => _active = v),
                      ),
                    ],
                  ),
                ],
              ),
              if (_error != null) ...[
                const SizedBox(height: 12),
                Text(_error!, style: const TextStyle(color: Colors.red)),
              ],
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: busy ? null : () => Navigator.pop(context, false),
          child: const Text('Bekor qilish'),
        ),
        FilledButton(
          onPressed: busy ? null : _save,
          child: Text(_saving ? 'Saqlanmoqda...' : 'Saqlash'),
        ),
      ],
    );
  }

  /// Rasm: bosiladigan keng to'rtburchak (852:286 — serverning "cover"
  /// kesish nisbatiga yaqin, ya'ni bu yerdagi ko'rinish natijaga mos).
  Widget _imagePreview() {
    const w = 480.0;
    const h = w * 286 / 852;
    Widget inner;
    if (_uploading) {
      inner = const Center(child: CircularProgressIndicator());
    } else if (_pickedBytes != null) {
      inner = Image.memory(_pickedBytes!, fit: BoxFit.cover, width: w, height: h);
    } else if (_imageUrl.isNotEmpty) {
      inner = Image.network(
        imageUrl(_imageUrl),
        fit: BoxFit.cover,
        width: w,
        height: h,
        errorBuilder: (_, __, ___) => const Center(
            child: Icon(Icons.broken_image_outlined, color: Colors.black26)),
      );
    } else {
      inner = const Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.add_photo_alternate_outlined,
              size: 32, color: Colors.black38),
          SizedBox(height: 6),
          Text('Banner rasmi\n(keng, ~3:1)',
              textAlign: TextAlign.center,
              style: TextStyle(color: Colors.black45, fontSize: 12)),
        ],
      );
    }
    return InkWell(
      onTap: _uploading ? null : _pickImage,
      borderRadius: BorderRadius.circular(10),
      child: Container(
        width: w,
        height: h,
        decoration: BoxDecoration(
          color: Colors.black12,
          borderRadius: BorderRadius.circular(10),
          border: Border.all(color: Colors.black26),
        ),
        clipBehavior: Clip.antiAlias,
        child: inner,
      ),
    );
  }
}

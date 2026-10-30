import 'dart:typed_data';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';

import '../api.dart';

/// Reklama/aksiya bannerlari — mijoz ilovasining UCH joyidagi bannerlar shu
/// yerdan boshqariladi (har biri alohida bo'lim):
///
///   * **Menu Banner** — mijoz ilovasining BOSH SAHIFASI;
///   * **Restoran Banner** — restoranlar sahifasi;
///   * **Do'kon Banner** — do'kon (Qurt) sahifasining tepasi.
///
/// ┌─ NEGA ADMIN PANELDA ──────────────────────────────────────────────────┐
/// Avval bannerlar mijoz ilovasi kodida QATTIQ YOZILGAN edi: yangi aksiya
/// chiqarish uchun ilovani qayta qurish kerak bo'lardi. Endi banner — oddiy
/// ma'lumot: rasm yuklanadi, ixtiyoriy matn qatlami va bosilganda amal
/// belgilanadi, yoqilib-o'chiriladi va tartibi o'zgartiriladi.
/// └───────────────────────────────────────────────────────────────────────┘
class BannersPage extends StatefulWidget {
  const BannersPage({super.key});

  @override
  State<BannersPage> createState() => _BannersPageState();
}

/// Bir joylashuvning sozlamalari (backend `internal/banners` qoidalari bilan mos).
class _Spec {
  const _Spec({
    required this.placement,
    required this.tab,
    required this.hint,
    required this.uploadType,
    required this.ratio,
    required this.ratioText,
    this.overlay = false,
    this.canButton = false,
    this.canRestaurant = false,
  });

  final String placement;
  final String tab;
  final String hint;

  /// `POST /uploads?type=` — `banner` tayyor grafikani kesmaydi, `cover` 2.4:1 ga kesadi.
  final String uploadType;

  /// Ko'rinish nisbati (kenglik/balandlik) — mijoz ilovasidagi nisbatga mos.
  final double ratio;
  final String ratioText;

  /// Rasm USTIGA matn (sarlavha, tavsif) qo'yish mumkinmi.
  final bool overlay;

  /// Tugma va "Qurt sahifasi" amali mumkinmi.
  final bool canButton;

  /// Bosilganda restoran menyusi ochilishi mumkinmi.
  final bool canRestaurant;
}

const _specs = <_Spec>[
  _Spec(
    placement: 'menu',
    tab: 'Menu Banner',
    hint: 'Mijoz ilovasining BOSH SAHIFASIDAGI bannerlar (xizmatlar ostida, '
        'tartib raqami kichigi yuqorida). Rasm ustiga matn va tugma qo\'yish '
        'mumkin; tugma bosilganda Qurt sahifasi ochiladi.',
    uploadType: 'banner',
    ratio: 3.1,
    ratioText: 'keng, ~3:1',
    overlay: true,
    canButton: true,
  ),
  _Spec(
    placement: 'restaurant',
    tab: 'Restoran Banner',
    hint: 'Mijoz ilovasining RESTORANLAR sahifasidagi reklama bloki. '
        'Restoran tanlansa, banner bosilganda o\'sha restoran menyusi ochiladi — '
        'bo\'sh qoldirilsa banner shunchaki ko\'rsatish uchun.',
    uploadType: 'cover',
    ratio: 852 / 286,
    ratioText: 'keng, ~3:1',
    canRestaurant: true,
  ),
  _Spec(
    placement: 'store',
    tab: 'Do\'kon Banner',
    hint: 'Do\'kon (Qurt) sahifasining TEPASIDAGI banner. Rasm ustiga sarlavha '
        'va tavsif qo\'yish mumkin. Banner qo\'shilmasa, standart Qurt banneri '
        'ko\'rsatiladi.',
    uploadType: 'banner',
    ratio: 2.4,
    ratioText: 'keng, ~2.4:1',
    overlay: true,
  ),
];

class _BannersPageState extends State<BannersPage>
    with SingleTickerProviderStateMixin {
  late final TabController _tabs =
      TabController(length: _specs.length, vsync: this);
  List<dynamic> _banners = [];
  List<dynamic> _restaurants = [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _tabs.addListener(() {
      if (!_tabs.indexIsChanging) setState(() {});
    });
    _load();
  }

  @override
  void dispose() {
    _tabs.dispose();
    super.dispose();
  }

  _Spec get _spec => _specs[_tabs.index];

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

  /// Bannerning joylashuvi. Bo'sh (eski yozuv) — restoran sahifasi.
  static String _placementOf(Map b) {
    final p = '${b['placement'] ?? ''}';
    return p.isEmpty ? 'restaurant' : p;
  }

  List<Map<String, dynamic>> _of(String placement) => _banners
      .whereType<Map>()
      .where((b) => _placementOf(b) == placement)
      .map((b) => Map<String, dynamic>.from(b))
      .toList();

  String _restaurantName(String id) {
    if (id.isEmpty) return '';
    for (final r in _restaurants) {
      if ('${r['id']}' == id) return '${r['name']}';
    }
    return id;
  }

  Future<void> _edit(_Spec spec, {Map<String, dynamic>? banner}) async {
    final saved = await showDialog<bool>(
      context: context,
      builder: (_) => _BannerDialog(
        spec: spec,
        restaurants: _restaurants,
        banner: banner,
      ),
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

  /// Faollik — ro'yxatdagi tugmadan DARHOL, dialogsiz. To'liq holat yuboriladi
  /// (server kelmagan maydonni bo'sh deb oladi).
  Future<void> _toggleActive(Map<String, dynamic> b, bool active) async {
    try {
      await api.updateBanner(
        '${b['id']}',
        imageUrl: '${b['image_url']}',
        placement: _placementOf(b),
        title: '${b['title'] ?? ''}',
        subtitle: '${b['subtitle'] ?? ''}',
        buttonText: '${b['button_text'] ?? ''}',
        action: '${b['action'] ?? ''}',
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
    final spec = _spec;
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Text('Bannerlar',
                  style: TextStyle(fontSize: 26, fontWeight: FontWeight.bold)),
              const Spacer(),
              FilledButton.icon(
                key: const ValueKey('banner-add'),
                onPressed: () => _edit(spec),
                icon: const Icon(Icons.add),
                label: Text('${spec.tab} qo\'shish'),
              ),
            ],
          ),
          const SizedBox(height: 8),
          TabBar(
            controller: _tabs,
            isScrollable: true,
            tabAlignment: TabAlignment.start,
            tabs: [
              for (final s in _specs)
                Tab(
                  key: ValueKey('banner-tab-${s.placement}'),
                  text: '${s.tab}${_loading ? '' : ' (${_of(s.placement).length})'}',
                ),
            ],
          ),
          const SizedBox(height: 10),
          Text(spec.hint, style: const TextStyle(color: Colors.black54)),
          const SizedBox(height: 12),
          Expanded(child: _buildBody(spec)),
        ],
      ),
    );
  }

  Widget _buildBody(_Spec spec) {
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
            OutlinedButton(
                onPressed: _load, child: const Text('Qayta urinish')),
          ],
        ),
      );
    }
    final list = _of(spec.placement);
    if (list.isEmpty) {
      return Center(
        child: Text('${spec.tab} yo\'q',
            style: const TextStyle(color: Colors.black54)),
      );
    }
    return ListView.separated(
      itemCount: list.length,
      separatorBuilder: (_, __) => const Divider(height: 1),
      itemBuilder: (_, i) {
        final b = list[i];
        final active = b['active'] == true;
        final image = '${b['image_url'] ?? ''}';
        final restaurantId = '${b['restaurant_id'] ?? ''}';
        final title = '${b['title'] ?? ''}';
        final action = '${b['action'] ?? ''}';
        final sortOrder = (b['sort_order'] as num?)?.toInt() ?? 0;
        final target = restaurantId.isNotEmpty
            ? _restaurantName(restaurantId)
            : action == 'qurt'
                ? 'Qurt sahifasi'
                : 'Havolasiz';
        return ListTile(
          leading: SizedBox(
            width: 96,
            height: 96 / spec.ratio,
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
          title: Text(title.isEmpty ? target : title),
          subtitle: Text([
            if (title.isNotEmpty) 'bosilganda: $target',
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
                onPressed: () => _edit(spec, banner: b),
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

/// Banner qo'shish/tahrirlash oynasi (joylashuv [spec] bilan belgilanadi).
class _BannerDialog extends StatefulWidget {
  const _BannerDialog({
    required this.spec,
    required this.restaurants,
    this.banner,
  });

  final _Spec spec;
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
  late String _action = '${widget.banner?['action'] ?? ''}';
  late bool _active = widget.banner?['active'] != false;
  late final _sortOrder = TextEditingController(
      text: '${(widget.banner?['sort_order'] as num?)?.toInt() ?? 0}');
  late final _title =
      TextEditingController(text: '${widget.banner?['title'] ?? ''}');
  late final _subtitle =
      TextEditingController(text: '${widget.banner?['subtitle'] ?? ''}');
  late final _button =
      TextEditingController(text: '${widget.banner?['button_text'] ?? ''}');

  Uint8List? _pickedBytes;
  bool _uploading = false;
  bool _saving = false;
  String? _error;

  @override
  void dispose() {
    _sortOrder.dispose();
    _title.dispose();
    _subtitle.dispose();
    _button.dispose();
    super.dispose();
  }

  /// Rasm tanlanadi va DARROV yuklanadi — admin natijani saqlashdan
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
      final url = await api.uploadImage(
        bytes,
        res!.files.single.name,
        type: widget.spec.uploadType,
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
    final spec = widget.spec;
    final button = spec.canButton ? _button.text.trim() : '';
    final action = spec.canButton ? _action : '';
    if (button.isNotEmpty && action.isEmpty) {
      setState(() => _error = 'Tugma matni uchun "Bosilganda" amalini tanlang');
      return;
    }
    final sortOrder = int.tryParse(_sortOrder.text.trim()) ?? 0;

    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      final existing = widget.banner;
      final title = spec.overlay ? _title.text.trim() : '';
      final subtitle = spec.overlay ? _subtitle.text.trim() : '';
      final restaurantId = spec.canRestaurant ? (_restaurantId ?? '') : '';
      if (existing == null) {
        await api.createBanner(
          imageUrl: _imageUrl,
          placement: spec.placement,
          title: title,
          subtitle: subtitle,
          buttonText: button,
          action: action,
          restaurantId: restaurantId,
          active: _active,
          sortOrder: sortOrder,
        );
      } else {
        await api.updateBanner(
          '${existing['id']}',
          imageUrl: _imageUrl,
          placement: spec.placement,
          title: title,
          subtitle: subtitle,
          buttonText: button,
          action: action,
          restaurantId: restaurantId,
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
    final spec = widget.spec;
    final isNew = widget.banner == null;
    final busy = _saving || _uploading;
    return AlertDialog(
      title: Text(isNew ? 'Yangi ${spec.tab}' : '${spec.tab}ni tahrirlash'),
      content: SizedBox(
        width: 560,
        child: SingleChildScrollView(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              _imagePreview(),
              const SizedBox(height: 16),
              if (spec.overlay) ...[
                TextField(
                  controller: _title,
                  maxLength: 60,
                  decoration: const InputDecoration(
                    labelText: 'Sarlavha (ixtiyoriy, rasm ustida)',
                    helperText: 'Rasmning o\'zida matn bo\'lsa, bo\'sh qoldiring',
                    border: OutlineInputBorder(),
                  ),
                ),
                const SizedBox(height: 10),
                TextField(
                  controller: _subtitle,
                  maxLength: 140,
                  maxLines: 2,
                  decoration: const InputDecoration(
                    labelText: 'Tavsif (ixtiyoriy, rasm ustida)',
                    border: OutlineInputBorder(),
                  ),
                ),
                const SizedBox(height: 10),
              ],
              if (spec.canButton) ...[
                DropdownButtonFormField<String>(
                  isExpanded: true,
                  initialValue: _action,
                  decoration: const InputDecoration(
                    labelText: 'Bosilganda',
                    border: OutlineInputBorder(),
                  ),
                  items: const [
                    DropdownMenuItem(
                      value: '',
                      child: Text('— Havolasiz (faqat ko\'rsatish) —'),
                    ),
                    DropdownMenuItem(
                      value: 'qurt',
                      child: Text('Qurt sahifasini ochish'),
                    ),
                  ],
                  onChanged: (v) => setState(() => _action = v ?? ''),
                ),
                const SizedBox(height: 10),
                TextField(
                  controller: _button,
                  maxLength: 24,
                  decoration: const InputDecoration(
                    labelText: 'Tugma matni (ixtiyoriy)',
                    helperText: 'Masalan: Buyurtma berish. "Bosilganda" amali kerak',
                    border: OutlineInputBorder(),
                  ),
                ),
                const SizedBox(height: 10),
              ],
              if (spec.canRestaurant) ...[
                DropdownButtonFormField<String?>(
                  isExpanded: true,
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
              ],
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

  /// Rasm: bosiladigan keng to'rtburchak — nisbat mijoz ilovasidagi shu
  /// joylashuv nisbatiga yaqin, ya'ni bu yerdagi ko'rinish natijaga mos.
  Widget _imagePreview() {
    const w = 480.0;
    final h = w / widget.spec.ratio;
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
      inner = Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const Icon(Icons.add_photo_alternate_outlined,
              size: 32, color: Colors.black38),
          const SizedBox(height: 6),
          Text('Banner rasmi\n(${widget.spec.ratioText})',
              textAlign: TextAlign.center,
              style: const TextStyle(color: Colors.black45, fontSize: 12)),
        ],
      );
    }
    return InkWell(
      onTap: (_saving || _uploading) ? null : _pickImage,
      borderRadius: BorderRadius.circular(12),
      child: Container(
        width: w,
        height: h,
        clipBehavior: Clip.antiAlias,
        decoration: BoxDecoration(
          color: Colors.black.withValues(alpha: 0.04),
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: Colors.black12),
        ),
        child: inner,
      ),
    );
  }
}

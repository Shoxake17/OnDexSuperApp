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

  /// Restoran -> uning akkaunt ma'lumotlari (PostHog userID + telefon
  /// raqam + mas'ul shaxs ismi). Jadvalning Telefon ustuni va PostHog
  /// tugmasi uchun. Xatosi ro'yxatni yiqitmasligi uchun try/catch.
  Map<String, ({String userId, String phone, String name})> _accounts = {};

  /// PostHog so'rovining holati: false → tugma ko'rinadi/yo'q, true →
  /// Snack orqali foydalanuvchiga xatoni aytamiz, chunki u avval hech
  /// qanday xabar olmagan — "nima uchun PostHog ko'rinmayapti?" degan
  /// savol doim paydo bo'lardi.
  bool _accountsHadError = false;
  String? _accountsError;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final l = await api.restaurants();
      Map<String, ({String userId, String phone, String name})> acc = {};
      Object? accErr;
      try {
        acc = await api.accountsByEntity('restaurant');
      } catch (e) {
        accErr = e;
      }
      if (!mounted) return;
      setState(() {
        _list = l;
        _accounts = acc;
        _loading = false;
        _accountsHadError = accErr != null;
        _accountsError = accErr?.toString();
      });
      if (_accountsHadError && mounted) {
        _snack('Ogohlantirish: akkauntlar ro\'yxati olinmadi — PostHog va '
            'Telefon ko\'rinmay qolishi mumkin. Xato: $_accountsError');
      }
    } catch (e) {
      if (!mounted) return;
      setState(() => _loading = false);
      _snack('Xato: $e');
    }
  }

  /// ┌─ RESTORAN PANELIDA NIMA QILINGAN ────────────────────────────────┐
  /// PostHog'da "odam" — restoranning O'ZI emas, uning xodim akkaunti:
  /// tahlil `ApiClient.me()` da `identify(userId: <user.ID>)` bilan
  /// bog'lanadi. Shuning uchun avval restoran ID sidan akkaunt ID si
  /// topiladi (`GET /admin/accounts`).
  ///
  /// Panel Windows ilovasi bo'lgani uchun u yerda SEANS YOZUVI emas,
  /// HODISALAR ko'rinadi (qaysi sahifa ochilgan, qaysi amal bajarilgan)
  /// — `posthog_flutter` Windows'ni qo'llamaydi.
  ///
  /// Nima uchun ko'pincha KO'RINMAYDI:
  ///   * PostHog kaliti yo'q (`ONDEX_POSTHOG_KEY`) — butunlay o'chirilgan
  ///   * `ONDEX_POSTHOG_PROJECT` keltirilmagan — havola qurilmaydi
  ///   * `/admin/accounts` so'rovida xato — `_accountsError` orqali ko'rsatiladi
  ///   * Restoranda akkaunt umuman yo'q (yaratilmagan)
  ///   * `phone`/`name` bo'sh — PostHog emas, jadval ustunlari uchun
  /// └──────────────────────────────────────────────────────────────────┘
  Widget _posthogButton(Map<String, dynamic> r) {
    final rId = (r['id']?.toString() ?? '').trim();
    final rec = _accounts[rId];
    final userId = rec?.userId ?? '';
    final url = posthogPersonUrl(userId);
    if (url.isEmpty) {
      // Nima uchun ko'rinmasligini tooltip bilan ko'rsatamiz.
      String reason;
      if (!posthogEnabled) {
        reason = 'PostHog kaliti qo\'yilmagan (build config)';
      } else if (posthogProjectId.isEmpty) {
        reason = 'PostHog loyiha raqami qo\'yilmagan';
      } else if (_accountsHadError) {
        reason = 'Akkauntlar so\'rovida xato: $_accountsError';
      } else if (userId.isEmpty) {
        reason = 'Bu restoranga kirish akkaunti biriktirilmagan';
      } else {
        reason = '';
      }
      if (reason.isEmpty) return const SizedBox.shrink();
      return IconButton(
        tooltip: 'PostHog yoqilmagan: $reason',
        icon: Icon(Icons.play_circle_outline,
            color: Colors.grey.shade400),
        onPressed: () => _snack('PostHog: $reason'),
      );
    }
    return IconButton(
      tooltip: 'Panelda nima qilgani (PostHog)',
      icon: const Icon(Icons.play_circle_outline, color: Color(0xFF1D4AFF)),
      onPressed: () async {
        final ok = await openLegalUrl(url);
        if (!ok && mounted) _snack('Havola ochilmadi');
      },
    );
  }

  void _snack(String msg) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg)));
  }

  /// Restoran telefonini ko'rsatuvchi katak. Akkaunt ma'lumotlaridan
  /// olinadi, chunki `Catalog.Restaurant` struct'ida phone maydoni
  /// umuman yo'q — u "users" jadvaliga tegishli (akkaunt telefon raqami).
  Widget _phoneCell(Map<String, dynamic> r) {
    final rId = (r['id']?.toString() ?? '').trim();
    final rec = _accounts[rId];
    final phone = rec?.phone ?? '';
    final name = rec?.name ?? '';

    if (_accountsHadError) {
      return Tooltip(
        message: 'Xatolik: $_accountsError\n\nQayta yuklash — yuqoridagi yangilash tugmasini bosing',
        child: Text('⚠️ Yuklanmadi',
            style: TextStyle(color: Colors.orange.shade700)),
      );
    }

    if (rId.isEmpty) {
      return Text('—', style: TextStyle(color: Colors.grey.shade400));
    }

    if (rec == null) {
      return Tooltip(
        message: 'Bu restoranga kirish akkaunti hali yaratilmagan.\n\n'
            'Qanday yaratish: "Restoran qo\'shish" tugmasi orqali restoran '
            'yaratganda avtomatik telefon akkaunti ham yaratiladi. '
            'Agar eski restoran bo\'lsa — bazada users jadvalida '
            'entity_id = "$rId" bo\'lgan user yaratilganini tekshiring.',
        child: Text('📵 Akkaunt yo\'q',
            style: TextStyle(color: Colors.grey.shade600)),
      );
    }

    if (phone.isEmpty && name.isEmpty) {
      return Tooltip(
        message: 'Ushbu restoran akkaunti mavjud, lekin telefon raqami va '
            'mas\'ul shaxs ismi kiritilmagan.\n\nuserId = ${rec.userId}',
        child: Text('—', style: TextStyle(color: Colors.grey.shade400)),
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        if (phone.isNotEmpty) Text(phone),
        if (name.isNotEmpty)
          Tooltip(
            message: 'Mas\'ul shaxs',
            child: Text(name,
                style: const TextStyle(fontSize: 11, color: Colors.grey)),
          ),
      ],
    );
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
                      // ┌─ JADVAL BUTUN QATORNI EGALLASIN ──────────┐
                      // Ilgari jadval gorizontal skroll ichida
                      // to'g'ridan-to'g'ri turardi va shu sababli
                      // MAZMUNI qadar kenglik olardi — keng oynada u
                      // o'rtada tugab, o'ng tomonda katta bo'sh joy
                      // qolardi.
                      //
                      // `ConstrainedBox(minWidth: maxWidth)` eng kam
                      // enni oyna kengligiga tenglashtiradi: jadval
                      // butun qatorni egallaydi. Gorizontal skroll
                      // SAQLANADI — tor oynada ustunlar siqilib
                      // ketmasligi uchun u baribir kerak.
                      // └────────────────────────────────────────────┘
                      : LayoutBuilder(
                          builder: (context, box) => SingleChildScrollView(
                            child: SingleChildScrollView(
                              scrollDirection: Axis.horizontal,
                              child: ConstrainedBox(
                                constraints:
                                    BoxConstraints(minWidth: box.maxWidth),
                                child: DataTable(
                              columns: const [
                                DataColumn(label: Text('№')),
                                DataColumn(label: Text('Logo')),
                                DataColumn(label: Text('Nomi')),
                                DataColumn(label: Text('Telefon')),
                                DataColumn(label: Text('Manzil')),
                                DataColumn(label: Text('Holat')),
                                DataColumn(label: Text('Amal')),
                              ],
                              rows: [
                                for (var i = 0; i < _list.length; i++)
                                  DataRow(cells: [
                                    DataCell(Text('${i + 1}')),
                                    DataCell(_LogoThumb(
                                        url: (_list[i] as Map<String, dynamic>)['logo_url'] as String? ?? '')),
                                    DataCell(Text((_list[i] as Map<String, dynamic>)['name'] ?? '')),
                                    DataCell(_phoneCell(_list[i] as Map<String, dynamic>)),
                                    DataCell(Text((_list[i] as Map<String, dynamic>)['address'] ?? '')),
                                    DataCell(Switch(
                                      value: (_list[i] as Map<String, dynamic>)['open'] == true,
                                      onChanged: (v) => _toggleOpen(_list[i] as Map<String, dynamic>, v),
                                    )),
                                    DataCell(Row(
                                      mainAxisSize: MainAxisSize.min,
                                      children: [
                                        _posthogButton(_list[i] as Map<String, dynamic>),
                                        IconButton(
                                          tooltip: 'Tahrirlash',
                                          icon: const Icon(
                                              Icons.edit_outlined),
                                          onPressed: () =>
                                              _showEditDialog(_list[i] as Map<String, dynamic>),
                                        ),
                                        IconButton(
                                          tooltip: 'Restoranni o\'chirish',
                                          icon: const Icon(
                                              Icons.delete_outline,
                                              color: Colors.red),
                                          onPressed: () => _confirmDelete(_list[i] as Map<String, dynamic>),
                                        ),
                                      ],
                                    )),
                                  ]),
                                  ],
                                ),
                              ),
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
  // Mijoz ilovasi bosh sahifasidagi kartadagi ikkita chip. Bo'sh yoki 0
  // qoldirilsa chip UMUMAN chizilmaydi — soxta "0.0 ★" chiqmaydi.
  late final TextEditingController _rating;
  late final TextEditingController _ratingCount;
  late final TextEditingController _etaMin;
  late final TextEditingController _etaMax;
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
    // 0 — "kiritilmagan" degani, shuning uchun maydon BO'SH ko'rsatiladi.
    // "0" yozib qo'yilsa admin uni haqiqiy qiymat deb o'ylardi.
    _rating = TextEditingController(text: _numOrEmpty(r['rating']));
    _ratingCount = TextEditingController(text: _numOrEmpty(r['rating_count']));
    _etaMin = TextEditingController(text: _numOrEmpty(r['eta_min_minutes']));
    _etaMax = TextEditingController(text: _numOrEmpty(r['eta_max_minutes']));
    _lat = (r['lat'] as num?)?.toDouble() ?? 41.0030;
    _lng = (r['lng'] as num?)?.toDouble() ?? 71.2360;
    _existingLogoUrl = r['logo_url'] as String? ?? '';
    _existingCoverUrl = r['cover_url'] as String? ?? '';
  }

  // Oynada `dispose()` UMUMAN yo'q edi — mavjud uchta kontroller ham
  // bo'shatilmasdi. Oyna qisqa umrli bo'lgani uchun sezilmagan, lekin
  // har ochilishda kichik oqim qolardi. Yangi to'rttasi qo'shilgach
  // to'g'rilandi.
  @override
  void dispose() {
    _name.dispose();
    _address.dispose();
    _tags.dispose();
    _rating.dispose();
    _ratingCount.dispose();
    _etaMin.dispose();
    _etaMax.dispose();
    super.dispose();
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

  /// Bo'sh matn -> 0 ("ko'rsatilmasin"), noto'g'ri matn -> null (xato).
  static double? _parseOrNull(String s, {required bool isInt}) {
    final t = s.trim();
    if (t.isEmpty) return 0;
    // Vergul bilan yozilgan o'nlik ("4,8") ham qabul qilinsin — klaviatura
    // tilига qarab odam ikkalasini ham yozadi.
    final v = isInt ? int.tryParse(t)?.toDouble() : double.tryParse(t.replaceAll(',', '.'));
    if (v == null || v < 0) return null;
    return v;
  }

  /// 0 va bo'sh qiymatni BIR XIL ko'radi: ikkalasi ham "kiritilmagan".
  static String _numOrEmpty(dynamic v) {
    final n = (v as num?)?.toDouble() ?? 0;
    if (n == 0) return '';
    // Butun son ".0" siz ko'rinsin (reyting 4 bo'lsa "4", 4.8 bo'lsa "4.8").
    return n == n.roundToDouble() ? n.toInt().toString() : n.toString();
  }

  Future<void> _submit() async {
    final name = _name.text.trim();
    if (name.isEmpty) {
      setState(() => _error = 'Restoran nomini kiriting');
      return;
    }
    // Bo'sh maydon = 0 = "ko'rsatilmasin". Noto'g'ri matn kiritilsa
    // jimgina 0 ga aylanib ketmasin — aniq xato beramiz.
    final rating = _parseOrNull(_rating.text, isInt: false);
    final ratingCount = _parseOrNull(_ratingCount.text, isInt: true);
    final etaMin = _parseOrNull(_etaMin.text, isInt: true);
    final etaMax = _parseOrNull(_etaMax.text, isInt: true);
    if (rating == null || ratingCount == null || etaMin == null || etaMax == null) {
      setState(() => _error = 'Reyting va vaqt maydonlariga faqat son kiriting');
      return;
    }
    if (rating < 0 || rating > 5) {
      setState(() => _error = 'Reyting 0 va 5 orasida bo\'lsin');
      return;
    }
    if (etaMax < etaMin) {
      setState(() => _error = 'Yetkazishning eng ko\'p vaqti eng kamidan kichik bo\'lmasin');
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
        rating: rating,
        ratingCount: ratingCount.toInt(),
        etaMinMinutes: etaMin.toInt(),
        etaMaxMinutes: etaMax.toInt(),
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
              const SizedBox(height: 16),
              const Divider(),
              const SizedBox(height: 8),
              Align(
                alignment: Alignment.centerLeft,
                child: Text('Mijoz kartasidagi ko\'rsatkichlar',
                    style: Theme.of(context).textTheme.titleSmall),
              ),
              const SizedBox(height: 4),
              const Align(
                alignment: Alignment.centerLeft,
                child: Text(
                  'Bo\'sh qoldirilsa kartada umuman ko\'rsatilmaydi. Reyting '
                  'hozircha qo\'lda kiritiladi — haqiqiy baholash tizimi '
                  'qurilgach avtomatik hisoblanadi.',
                  style: TextStyle(fontSize: 12, color: Colors.grey),
                ),
              ),
              const SizedBox(height: 12),
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(
                    child: TextField(
                      controller: _rating,
                      keyboardType:
                          const TextInputType.numberWithOptions(decimal: true),
                      decoration: const InputDecoration(
                        labelText: 'Reyting',
                        hintText: '4.8',
                        helperText: '0–5',
                        border: OutlineInputBorder(),
                      ),
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: TextField(
                      controller: _ratingCount,
                      keyboardType: TextInputType.number,
                      decoration: const InputDecoration(
                        labelText: 'Baholar soni',
                        hintText: '120',
                        helperText: 'Kartada "(120+)"',
                        border: OutlineInputBorder(),
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(
                    child: TextField(
                      controller: _etaMin,
                      keyboardType: TextInputType.number,
                      decoration: const InputDecoration(
                        labelText: 'Yetkazish: eng kam',
                        hintText: '20',
                        helperText: 'daqiqa',
                        border: OutlineInputBorder(),
                      ),
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: TextField(
                      controller: _etaMax,
                      keyboardType: TextInputType.number,
                      decoration: const InputDecoration(
                        labelText: 'Yetkazish: eng ko\'p',
                        hintText: '30',
                        helperText: 'Kartada "20–30 daqiqa"',
                        border: OutlineInputBorder(),
                      ),
                    ),
                  ),
                ],
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

import 'package:flutter/material.dart';

import '../api.dart';
import '../data/cart_store.dart';
import 'address_screen.dart';
import 'catalog_screen.dart' show kBrand;
import 'tracking_screen.dart';

/// Buyurtmani rasmiylashtirish — NATIVE.
///
/// ┌─ IKKI TUR ────────────────────────────────────────────────────────┐
/// STOL (dine-in): manzil KERAK EMAS. Buyurtma `table_token` bilan
/// yuboriladi va server uni `dine_in` deb belgilaydi.
///
/// YETKAZIB BERISH: manzil MAJBURIY. Mijozning saqlangan manzili
/// ishlatiladi, kerak bo'lsa xaritadan o'zgartiriladi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// ┌─ SUMMA SERVERDAN KELADI ──────────────────────────────────────────┐
/// `quoteTiyin` — savat ekranida `POST /restaurants/{id}/quote` dan
/// olingan qiymat. Bu yerda QAYTA HISOBLANMAYDI: mijoz ko'rgan summa
/// va serverda yoziladigan summa bir xil bo'lishi shart.
/// └───────────────────────────────────────────────────────────────────┘
class CheckoutScreen extends StatefulWidget {
  final int quoteTiyin;

  const CheckoutScreen({super.key, required this.quoteTiyin});

  @override
  State<CheckoutScreen> createState() => _CheckoutScreenState();
}

class _CheckoutScreenState extends State<CheckoutScreen> {
  final _cart = CartStore.instance;

  // ┌─ IDEMPOTENTLIK KALITI BIR MARTA ─────────────────────────────┐
  // Kalit ekran ochilganda BIR MARTA generatsiya qilinadi va barcha
  // qayta urinishlarda o'zgarmaydi. Tarmoq uzilib javob kelmasa,
  // mijoz qayta bosganda server SHU kalitni ko'rib dublikat buyurtma
  // YARATMAYDI (`api.dart` dagi `newIdempotencyKey` izohiga qarang).
  //
  // Uni `_submit` ichida generatsiya qilish ENG KLASSIK xato bo'lardi:
  // har bosishda yangi kalit chiqib, himoya butunlay ishlamasdi.
  // └───────────────────────────────────────────────────────────────┘
  final String _idempotencyKey = newIdempotencyKey();

  Map<String, dynamic>? _address;
  bool _loadingAddress = true;
  bool _submitting = false;
  String? _error;
  int _partySize = 2;

  bool get _isDineIn => _cart.isDineIn;

  @override
  void initState() {
    super.initState();
    if (_isDineIn) {
      _loadingAddress = false;
    } else {
      _loadAddress();
    }
  }

  Future<void> _loadAddress() async {
    try {
      final a = await api.getMyAddress();
      if (!mounted) return;
      setState(() {
        // lat/lng == 0 — manzil hali tanlanmagan.
        _address = ((a['lat'] as num?)?.toDouble() ?? 0) != 0 ? a : null;
        _loadingAddress = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() => _loadingAddress = false);
    }
  }

  Future<void> _pickAddress() async {
    await Navigator.of(context).push(
      MaterialPageRoute(builder: (_) => const AddressScreen()),
    );
    if (!mounted) return;
    setState(() => _loadingAddress = true);
    await _loadAddress();
  }

  Future<void> _submit() async {
    if (_submitting) return;

    final rid = _cart.restaurantId;
    if (rid == null || _cart.isEmpty) return;

    // Yetkazib berishda manzilsiz yuborilmaydi — server ham rad
    // etardi, lekin mijozga sababni SHU YERDA aytish to'g'ri.
    if (!_isDineIn && _address == null) {
      setState(() => _error = 'Yetkazib berish manzilini tanlang');
      return;
    }

    setState(() {
      _submitting = true;
      _error = null;
    });

    try {
      final order = await api.createOrder(
        items: [
          for (final e in _cart.items.entries)
            {'product_id': e.key, 'qty': e.value}
        ],
        idempotencyKey: _idempotencyKey,
        lat: _isDineIn ? null : (_address!['lat'] as num).toDouble(),
        lng: _isDineIn ? null : (_address!['lng'] as num).toDouble(),
        tableToken: _isDineIn ? _cart.tableToken : null,
        partySize: _isDineIn ? _partySize : null,
      );

      if (!mounted) return;

      // Savat FAQAT muvaffaqiyatdan keyin tozalanadi. Oldin tozalansa
      // va so'rov yiqilsa, mijoz savatini yo'qotgan bo'lardi.
      _cart.clear();

      final id = (order['id'] as String?) ?? '';
      // `pushReplacement`: orqaga bosilganda checkout'ga emas,
      // katalogga qaytadi — buyurtma allaqachon berilgan, unga
      // qaytishning ma'nosi yo'q.
      Navigator.of(context).pushReplacement(
        MaterialPageRoute(builder: (_) => TrackingScreen(orderId: id)),
      );
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _submitting = false;
        _error = e is ApiException ? e.message : 'Buyurtma yuborilmadi';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Rasmiylashtirish')),
      body: _loadingAddress
          ? const Center(child: CircularProgressIndicator())
          : ListView(
              padding: const EdgeInsets.fromLTRB(16, 12, 16, 24),
              children: [
                if (_isDineIn) ..._dineInSection() else ..._deliverySection(),
                const SizedBox(height: 20),
                _TotalRow(tiyin: widget.quoteTiyin),
                if (_error != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 14),
                    child: Container(
                      padding: const EdgeInsets.all(12),
                      decoration: BoxDecoration(
                        color: const Color(0xFFFBEAE9),
                        borderRadius: BorderRadius.circular(10),
                      ),
                      child: Row(
                        children: [
                          const Icon(Icons.error_outline,
                              size: 18, color: Color(0xFFB3261E)),
                          const SizedBox(width: 8),
                          Expanded(
                            child: Text(_error!,
                                style: const TextStyle(
                                    color: Color(0xFFB3261E), fontSize: 13.5)),
                          ),
                        ],
                      ),
                    ),
                  ),
              ],
            ),
      bottomNavigationBar: SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 12),
          child: SizedBox(
            height: 52,
            child: FilledButton(
              style: FilledButton.styleFrom(
                backgroundColor: kBrand,
                shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(14)),
              ),
              onPressed: _submitting ? null : _submit,
              child: _submitting
                  ? const SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(
                          strokeWidth: 2, color: Colors.white),
                    )
                  : const Text('Buyurtma berish',
                      style: TextStyle(
                          fontSize: 16, fontWeight: FontWeight.bold)),
            ),
          ),
        ),
      ),
    );
  }

  // ── Stol ──────────────────────────────────────────────────────────

  List<Widget> _dineInSection() {
    final label = _cart.tableLabel;
    return [
      _Card(
        icon: Icons.qr_code_2,
        title: (label == null || label.isEmpty) ? 'Stol' : '$label-stol',
        subtitle: 'Buyurtma to\'g\'ridan-to\'g\'ri oshxonaga tushadi',
      ),
      const SizedBox(height: 14),
      const Text('Necha kishisiz?',
          style: TextStyle(fontWeight: FontWeight.w600)),
      const SizedBox(height: 8),
      Wrap(
        spacing: 8,
        children: [
          for (final n in [1, 2, 3, 4, 5, 6])
            ChoiceChip(
              label: Text('$n'),
              selected: _partySize == n,
              onSelected: (_) => setState(() => _partySize = n),
            ),
        ],
      ),
    ];
  }

  // ── Yetkazib berish ───────────────────────────────────────────────

  List<Widget> _deliverySection() {
    final a = _address;
    return [
      if (a == null)
        _Card(
          icon: Icons.location_off_outlined,
          title: 'Manzil tanlanmagan',
          subtitle: 'Yetkazib berish uchun manzil kerak',
          onTap: _pickAddress,
          actionText: 'Tanlash',
        )
      else
        _Card(
          icon: Icons.place_outlined,
          title: ((a['text'] as String?) ?? '').trim().isEmpty
              ? 'Tanlangan manzil'
              : (a['text'] as String),
          subtitle: _addressExtras(a),
          onTap: _pickAddress,
          actionText: 'O\'zgartirish',
        ),
    ];
  }

  /// Kvartira/qavat/domofon — bo'sh bo'lganlari ko'rsatilmaydi.
  String _addressExtras(Map<String, dynamic> a) {
    final parts = <String>[
      for (final k in ['entrance', 'floor', 'apartment'])
        if (((a[k] as String?) ?? '').trim().isNotEmpty)
          _label(k, a[k] as String),
    ];
    return parts.isEmpty ? 'Yetkazib berish manzili' : parts.join(' · ');
  }

  static String _label(String key, String value) => switch (key) {
        'entrance' => '$value-podez',
        'floor' => '$value-qavat',
        'apartment' => '$value-xonadon',
        _ => value,
      };
}

// ═══════════════════════════════════════════════════════════════════

class _Card extends StatelessWidget {
  final IconData icon;
  final String title;
  final String subtitle;
  final VoidCallback? onTap;
  final String? actionText;

  const _Card({
    required this.icon,
    required this.title,
    required this.subtitle,
    this.onTap,
    this.actionText,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(14),
      child: Container(
        padding: const EdgeInsets.all(14),
        decoration: BoxDecoration(
          border: Border.all(color: const Color(0xFFE5E5E5)),
          borderRadius: BorderRadius.circular(14),
        ),
        child: Row(
          children: [
            Icon(icon, color: kBrand),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(title,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(fontWeight: FontWeight.w600)),
                  const SizedBox(height: 2),
                  Text(subtitle,
                      style: const TextStyle(
                          fontSize: 12.5, color: Color(0xFF757575))),
                ],
              ),
            ),
            if (actionText != null)
              Text(actionText!,
                  style: const TextStyle(
                      color: kBrand, fontWeight: FontWeight.w600)),
          ],
        ),
      ),
    );
  }
}

class _TotalRow extends StatelessWidget {
  final int tiyin;
  const _TotalRow({required this.tiyin});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: const Color(0xFFF7F7F7),
        borderRadius: BorderRadius.circular(14),
      ),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          const Text('Jami',
              style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600)),
          Text(formatSum(tiyin),
              style: const TextStyle(
                  fontSize: 18, fontWeight: FontWeight.bold)),
        ],
      ),
    );
  }
}

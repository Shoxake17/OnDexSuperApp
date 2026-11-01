import 'package:flutter/material.dart';

import '../api.dart';
import '../data/address_scope.dart';
import '../data/qurt_cart.dart';
import '../widgets/sheet_page.dart';
import 'address_screen.dart';
import 'catalog_screen.dart' show kBrand;
import 'qurt_orders_screen.dart';

const _ink = Color(0xFF111827);
const _muted = Color(0xFF6B7280);
const _line = Color(0xFFEDEDED);
const _danger = Color(0xFFDC2626);

/// Qurt savati va buyurtma rasmiylashtirish (naqd, reys bilan yetkazish).
///
/// ┌─ XAVFSIZLIK ────────────────────────────────────────────────────────┐
/// * Serverga FAQAT mahsulot ID, miqdor va izoh ketadi: narx, do'kon, manzil,
///   telefon serverdan olinadi (mijoz ularni o'zgartira olmaydi).
/// * Ekrandagi jami `expected_total_tiyin` sifatida yuboriladi: narx o'zgargan
///   bo'lsa buyurtma YARATILMAYDI, mijoz yangi summani ko'rib qayta tasdiqlaydi.
/// * Bir xil savat uchun bitta `idempotency_key`: tarmoq uzilib qayta bosilsa
///   dublikat buyurtma bo'lmaydi.
/// * Tugma buyurtma yuborilayotganda qulflanadi (qo'sh bosish himoyasi).
/// └─────────────────────────────────────────────────────────────────────┘
class QurtCartScreen extends StatefulWidget {
  const QurtCartScreen({super.key, QurtCart? cart}) : _cart = cart;

  /// Faqat sinov uchun: yagona savat o'rniga berilgan.
  final QurtCart? _cart;

  @override
  State<QurtCartScreen> createState() => _QurtCartScreenState();
}

class _QurtCartScreenState extends State<QurtCartScreen> {
  late final QurtCart _cart = widget._cart ?? QurtCart.instance;
  final _comment = TextEditingController();

  Map<String, dynamic>? _delivery;
  bool _syncing = true;
  bool _busy = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _cart.addListener(_onCart);
    AddressScope.instance.addListener(_onCart);
    _init();
  }

  @override
  void dispose() {
    _cart.removeListener(_onCart);
    AddressScope.instance.removeListener(_onCart);
    _comment.dispose();
    super.dispose();
  }

  void _onCart() {
    if (mounted) setState(() {});
  }

  Future<void> _init() async {
    await _cart.ensureRestored();
    await AddressScope.instance.refresh();
    await _refresh();
  }

  /// Savatni serverdagi joriy narx/qoldiq bilan yangilaydi + reys ma'lumoti.
  /// O'zgarish bo'lsa ([announce]) mijozga aytadi; qaytadi: o'zgarishlar ro'yxati
  /// (bo'sh — savat serverga mos).
  Future<List<String>> _refresh({
    bool quiet = false,
    bool announce = true,
  }) async {
    if (!quiet && mounted) setState(() => _syncing = true);
    var notes = <String>[];
    try {
      final results = await Future.wait([
        _cart.isEmpty
            ? Future.value(<Map<String, dynamic>>[])
            : api.qurtProductsByIds(
              _cart.lines.map((l) => l.productId).toList(),
            ),
        api.qurtDeliveryInfo(),
      ]);
      notes =
          _cart.isEmpty
              ? <String>[]
              : _cart.syncWith(results[0] as List<Map<String, dynamic>>);
      if (mounted) {
        setState(() {
          _delivery = results[1] as Map<String, dynamic>;
          _error = null;
        });
        if (announce && notes.isNotEmpty) _snack(notes.join('\n'));
      }
    } catch (e) {
      if (mounted) {
        setState(() => _error = errorText(e, 'Ma\'lumotni yangilab bo\'lmadi'));
      }
    } finally {
      if (mounted) setState(() => _syncing = false);
    }
    return notes;
  }

  void _snack(String msg) {
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(
        SnackBar(content: Text(msg), duration: const Duration(seconds: 4)),
      );
  }

  Future<void> _pickAddress() async {
    await Navigator.of(context).push(sheetRoute(const AddressScreen()));
    await AddressScope.instance.refresh();
  }

  Future<void> _submit() async {
    if (_busy || _cart.isEmpty) return;
    if (!AddressScope.instance.bor || AddressScope.instance.text.isEmpty) {
      _snack('Avval yetkazib berish manzilini tanlang');
      return;
    }
    setState(() => _busy = true);
    try {
      // Eng so'nggi narx/qoldiq bilan solishtirish: o'zgargan bo'lsa buyurtma yuborilmaydi.
      final notes = await _refresh(quiet: true, announce: false);
      if (notes.isNotEmpty) {
        _snack('${notes.join('\n')}\nSummani tekshirib, qayta tasdiqlang');
        return;
      }
      if (_cart.isEmpty) return;
      final orders = await api.createQurtOrder(
        items: _cart.toOrderItems(),
        idempotencyKey: _cart.checkoutKey(),
        expectedTotalTiyin: _cart.totalTiyin,
        comment: _comment.text,
      );
      _cart.clear();
      if (!mounted) return;
      await Navigator.of(
        context,
      ).pushReplacement(sheetRoute(QurtOrderPlacedScreen(orders: orders)));
    } on ApiException catch (e) {
      if (!mounted) return;
      final code = e.data?['code'];
      if (e.statusCode == 409 && code == 'total_changed') {
        final fresh = (e.data?['total_tiyin'] as num?)?.toInt();
        await _refresh(quiet: true, announce: false);
        _snack(
          fresh == null
              ? 'Narx o\'zgardi — qayta tekshiring'
              : 'Narx o\'zgardi. Yangi jami: ${formatSum(fresh)} — tasdiqlang',
        );
      } else if (e.statusCode == 409) {
        // out_of_stock: qoldiq tugagan — savatni yangilab, sababini ko'rsatamiz.
        await _refresh(quiet: true, announce: false);
        _snack(e.message);
      } else {
        _snack(e.message);
      }
    } catch (_) {
      if (mounted) {
        _snack(
          'Buyurtma yuborilmadi. Internetni tekshirib, qayta urinib ko\'ring',
        );
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return SheetPage(
      child: Scaffold(
        backgroundColor: Colors.white,
        appBar: AppBar(
          backgroundColor: Colors.white,
          surfaceTintColor: Colors.white,
          elevation: 0,
          iconTheme: const IconThemeData(color: _ink),
          title: const Text(
            'Qurt savati',
            style: TextStyle(
              color: _ink,
              fontWeight: FontWeight.w800,
              fontSize: 18,
            ),
          ),
          actions: [
            if (!_cart.isEmpty)
              TextButton(
                onPressed: _busy ? null : _cart.clear,
                child: const Text('Tozalash', style: TextStyle(color: _muted)),
              ),
          ],
        ),
        body: _cart.isEmpty ? _empty() : _body(),
        bottomNavigationBar: _cart.isEmpty ? null : _bottom(),
      ),
    );
  }

  Widget _empty() => Center(
    child: Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        const Icon(
          Icons.shopping_basket_outlined,
          size: 60,
          color: Colors.black26,
        ),
        const SizedBox(height: 12),
        const Text(
          'Savat bo\'sh',
          style: TextStyle(color: _muted, fontSize: 16),
        ),
        const SizedBox(height: 14),
        OutlinedButton(
          onPressed: () => Navigator.of(context).maybePop(),
          child: const Text('Xaridga qaytish'),
        ),
      ],
    ),
  );

  Widget _body() {
    final date = (_delivery?['delivery_date'] ?? '').toString();
    final orderBy =
        DateTime.tryParse((_delivery?['order_by'] ?? '').toString())?.toLocal();
    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 4, 16, 24),
      children: [
        for (final l in _cart.lines)
          _LineTile(line: l, cart: _cart, enabled: !_busy),
        const SizedBox(height: 14),
        _section(
          icon: Icons.local_shipping_outlined,
          title:
              date.isEmpty ? 'Yetkazish' : 'Yetkazish: ${formatQurtDate(date)}',
          body:
              'Buyurtmalar bir necha kun to\'planib, reys bilan yetkaziladi.'
              '${orderBy == null ? '' : ' Shu reysga ${orderBy.day}-${_monthName(orderBy.month)} '
                      '${orderBy.hour.toString().padLeft(2, '0')}:${orderBy.minute.toString().padLeft(2, '0')} gacha buyurtma bering.'}',
        ),
        const SizedBox(height: 10),
        InkWell(
          key: const ValueKey('qurt-cart-address'),
          onTap: _busy ? null : _pickAddress,
          borderRadius: BorderRadius.circular(14),
          child: _section(
            icon: Icons.place_outlined,
            title: 'Manzil',
            body: AddressScope.instance.label,
            warn: !AddressScope.instance.bor,
            trailing: const Icon(Icons.chevron_right, color: _muted),
          ),
        ),
        const SizedBox(height: 10),
        _section(
          icon: Icons.payments_outlined,
          title: 'To\'lov: naqd',
          body: 'Yetkazilganda kuryerga naqd to\'laysiz.',
        ),
        const SizedBox(height: 14),
        TextField(
          controller: _comment,
          maxLength: 300,
          maxLines: 2,
          enabled: !_busy,
          decoration: InputDecoration(
            hintText: 'Izoh (ixtiyoriy)',
            filled: true,
            fillColor: const Color(0xFFF3F4F6),
            border: OutlineInputBorder(
              borderRadius: BorderRadius.circular(14),
              borderSide: BorderSide.none,
            ),
          ),
        ),
        if (_error != null) ...[
          const SizedBox(height: 8),
          Text(_error!, style: const TextStyle(color: _danger, fontSize: 12.5)),
        ],
      ],
    );
  }

  Widget _section({
    required IconData icon,
    required String title,
    required String body,
    bool warn = false,
    Widget? trailing,
  }) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: warn ? const Color(0xFFFEF2F2) : const Color(0xFFFAFAFA),
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: warn ? _danger : _line),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 22, color: warn ? _danger : kBrand),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: const TextStyle(
                    fontWeight: FontWeight.w700,
                    fontSize: 14.5,
                    color: _ink,
                  ),
                ),
                const SizedBox(height: 3),
                Text(
                  body,
                  style: TextStyle(
                    color: warn ? _danger : _muted,
                    fontSize: 12.5,
                    height: 1.3,
                  ),
                ),
              ],
            ),
          ),
          if (trailing != null) trailing,
        ],
      ),
    );
  }

  Widget _bottom() {
    final canOrder = !_busy && !_syncing && !_cart.isEmpty;
    return SafeArea(
      child: Container(
        padding: const EdgeInsets.fromLTRB(16, 10, 16, 12),
        decoration: const BoxDecoration(
          color: Colors.white,
          border: Border(top: BorderSide(color: _line)),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Row(
              children: [
                Text(
                  '${_cart.totalQty} ta mahsulot',
                  style: const TextStyle(color: _muted),
                ),
                const Spacer(),
                const Text(
                  'Yetkazish: bepul',
                  style: TextStyle(color: _muted, fontSize: 12.5),
                ),
              ],
            ),
            const SizedBox(height: 4),
            Row(
              children: [
                const Text(
                  'Jami',
                  style: TextStyle(fontWeight: FontWeight.w800, fontSize: 16),
                ),
                const Spacer(),
                Text(
                  formatSum(_cart.totalTiyin),
                  key: const ValueKey('qurt-cart-total'),
                  style: const TextStyle(
                    color: kBrand,
                    fontWeight: FontWeight.w800,
                    fontSize: 20,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 10),
            SizedBox(
              width: double.infinity,
              child: FilledButton(
                key: const ValueKey('qurt-cart-submit'),
                onPressed: canOrder ? _submit : null,
                style: FilledButton.styleFrom(
                  backgroundColor: kBrand,
                  padding: const EdgeInsets.symmetric(vertical: 16),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(16),
                  ),
                ),
                child:
                    _busy
                        ? const SizedBox(
                          width: 20,
                          height: 20,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: Colors.white,
                          ),
                        )
                        : const Text(
                          'Buyurtma berish',
                          style: TextStyle(
                            fontWeight: FontWeight.w700,
                            fontSize: 16,
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

String _monthName(int m) =>
    const [
      'yanvar',
      'fevral',
      'mart',
      'aprel',
      'may',
      'iyun',
      'iyul',
      'avgust',
      'sentabr',
      'oktabr',
      'noyabr',
      'dekabr',
    ][m - 1];

class _LineTile extends StatelessWidget {
  const _LineTile({
    required this.line,
    required this.cart,
    required this.enabled,
  });
  final QurtCartLine line;
  final QurtCart cart;
  final bool enabled;

  @override
  Widget build(BuildContext context) {
    final img = fullImageUrl(line.imageUrl);
    final atMax = line.qty >= line.maxQty || line.qty >= kQurtMaxQtyPerLine;
    return Container(
      margin: const EdgeInsets.only(top: 10),
      padding: const EdgeInsets.all(10),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: _line),
      ),
      child: Row(
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(10),
            child: Container(
              width: 76,
              height: 56,
              color: const Color(0xFFF3F4F6),
              child:
                  img.isEmpty
                      ? const Icon(Icons.bakery_dining, color: Colors.black26)
                      : Image.network(
                        img,
                        fit: BoxFit.contain,
                        errorBuilder:
                            (_, _, _) => const Icon(
                              Icons.bakery_dining,
                              color: Colors.black26,
                            ),
                      ),
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  line.name,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    fontWeight: FontWeight.w700,
                    fontSize: 14.5,
                    color: _ink,
                  ),
                ),
                if (line.storeName.isNotEmpty)
                  Text(
                    line.storeName,
                    style: const TextStyle(color: _muted, fontSize: 11.5),
                  ),
                const SizedBox(height: 4),
                Text(
                  formatSum(line.totalTiyin),
                  style: const TextStyle(
                    color: kBrand,
                    fontWeight: FontWeight.w800,
                    fontSize: 14.5,
                  ),
                ),
              ],
            ),
          ),
          _Stepper(
            qty: line.qty,
            onMinus:
                enabled
                    ? () => cart.setQty(line.productId, line.qty - 1)
                    : null,
            onPlus:
                enabled && !atMax
                    ? () => cart.setQty(line.productId, line.qty + 1)
                    : null,
          ),
        ],
      ),
    );
  }
}

/// Miqdor tanlagich: − N +.
class _Stepper extends StatelessWidget {
  const _Stepper({
    required this.qty,
    required this.onMinus,
    required this.onPlus,
  });
  final int qty;
  final VoidCallback? onMinus;
  final VoidCallback? onPlus;

  @override
  Widget build(BuildContext context) {
    Widget btn(IconData icon, VoidCallback? onTap, Key key) => InkResponse(
      key: key,
      onTap: onTap,
      radius: 20,
      child: Container(
        width: 32,
        height: 32,
        decoration: BoxDecoration(
          shape: BoxShape.circle,
          color: onTap == null ? const Color(0xFFF3F4F6) : kBrand,
        ),
        child: Icon(
          icon,
          size: 18,
          color: onTap == null ? Colors.black26 : Colors.white,
        ),
      ),
    );
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        btn(Icons.remove, onMinus, const ValueKey('qty-minus')),
        SizedBox(
          width: 30,
          child: Text(
            '$qty',
            textAlign: TextAlign.center,
            style: const TextStyle(fontWeight: FontWeight.w800, fontSize: 15),
          ),
        ),
        btn(Icons.add, onPlus, const ValueKey('qty-plus')),
      ],
    );
  }
}

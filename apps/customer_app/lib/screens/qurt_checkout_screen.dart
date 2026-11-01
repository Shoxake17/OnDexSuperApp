import 'package:flutter/material.dart';

import '../api.dart';
import '../data/address_scope.dart';
import '../data/qurt_cart.dart';
import '../widgets/order_blocks.dart';
import '../widgets/sheet_page.dart';
import 'address_screen.dart';
import 'qurt_orders_screen.dart' show formatQurtDate;

const _green = Color(0xFF16A34A);

/// Qurt buyurtmasini rasmiylashtirish — restoran rasmiylashtirishi
/// (`checkout_screen.dart`) bilan BIR XIL ko'rinish: yuqori panel
/// (`OrderPageAppBar`), kartalar (`OrderCard`/`OrderCardRow`), tafsilotlar
/// (`OrderItemTile`), jami (`OrderTotalLine`/`OrderGrandTotal`), pastki tugma
/// (`OrderSubmitBar`) — hammasi `widgets/order_blocks.dart` dan.
///
/// ┌─ XAVFSIZLIK ────────────────────────────────────────────────────────┐
/// * Serverga FAQAT mahsulot ID, miqdor va izoh ketadi: narx, do'kon,
///   manzil, telefon serverdan olinadi (mijoz ularni o'zgartira olmaydi).
/// * Ekrandagi jami `expected_total_tiyin` sifatida yuboriladi: narx
///   o'zgargan bo'lsa buyurtma YARATILMAYDI, mijoz yangi summani ko'rib qayta
///   tasdiqlaydi.
/// * Bir xil savat uchun bitta `idempotency_key`: tarmoq uzilib qayta bosilsa
///   dublikat buyurtma bo'lmaydi.
/// * Tugma yuborilayotganda qulflanadi (qo'sh bosish himoyasi).
/// └─────────────────────────────────────────────────────────────────────┘
///
/// Muvaffaqiyatda sahifa buyurtmalar ro'yxati bilan yopiladi (`pop(orders)`).
class QurtCheckoutScreen extends StatefulWidget {
  const QurtCheckoutScreen({super.key, QurtCart? cart}) : _cart = cart;

  /// Faqat sinov uchun: yagona savat o'rniga berilgan.
  final QurtCart? _cart;

  @override
  State<QurtCheckoutScreen> createState() => _QurtCheckoutScreenState();
}

class _QurtCheckoutScreenState extends State<QurtCheckoutScreen> {
  late final QurtCart _cart = widget._cart ?? QurtCart.instance;
  final _comment = TextEditingController();

  Map<String, dynamic>? _delivery;
  bool _loading = true;
  bool _submitting = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _cart.addListener(_onChange);
    AddressScope.instance.addListener(_onChange);
    _load();
  }

  @override
  void dispose() {
    _cart.removeListener(_onChange);
    AddressScope.instance.removeListener(_onChange);
    _comment.dispose();
    super.dispose();
  }

  void _onChange() {
    if (mounted) setState(() {});
  }

  Future<void> _load() async {
    await _cart.ensureRestored();
    await AddressScope.instance.refresh();
    try {
      final results = await Future.wait([
        _cart.refresh(),
        api.qurtDeliveryInfo(),
      ]);
      if (!mounted) return;
      final notes = results[0] as List<String>;
      setState(() {
        _delivery = results[1] as Map<String, dynamic>;
        _error = null;
      });
      if (notes.isNotEmpty) _snack(notes.join('\n'));
    } catch (e) {
      if (mounted) {
        setState(() => _error = errorText(e, 'Ma\'lumotni yangilab bo\'lmadi'));
      }
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  void _snack(String msg) {
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(msg)));
  }

  Future<void> _pickAddress() async {
    await Navigator.of(context).push(sheetRoute(const AddressScreen()));
    await AddressScope.instance.refresh();
  }

  bool get _hasAddress =>
      AddressScope.instance.bor && AddressScope.instance.text.isNotEmpty;

  void _add(QurtCartLine l) {
    if (l.qty >= l.maxQty || l.qty >= kQurtMaxQtyPerLine) {
      _snack('Bundan ko\'p qolmagan');
      return;
    }
    _cart.setQty(l.productId, l.qty + 1);
  }

  Future<void> _submit() async {
    if (_submitting || _cart.isEmpty) return;
    if (!_hasAddress) {
      setState(() => _error = 'Avval yetkazib berish manzilini tanlang');
      return;
    }
    setState(() {
      _submitting = true;
      _error = null;
    });
    try {
      // Eng so'nggi narx/qoldiq bilan solishtirish: o'zgargan bo'lsa buyurtma yuborilmaydi.
      final notes = await _cart.refresh();
      if (notes.isNotEmpty) {
        _snack('${notes.join('\n')}\nSummani tekshirib, qayta tasdiqlang');
        return;
      }
      if (_cart.isEmpty) {
        if (mounted) Navigator.of(context).pop();
        return;
      }
      final orders = await api.createQurtOrder(
        items: _cart.toOrderItems(),
        idempotencyKey: _cart.checkoutKey(),
        expectedTotalTiyin: _cart.totalTiyin,
        comment: _comment.text,
      );
      _cart.clear();
      if (mounted) Navigator.of(context).pop(orders);
    } on ApiException catch (e) {
      if (!mounted) return;
      final code = e.data?['code'];
      if (e.statusCode == 409 && code == 'total_changed') {
        final fresh = (e.data?['total_tiyin'] as num?)?.toInt();
        await _cart.refresh().catchError((_) => <String>[]);
        _snack(
          fresh == null
              ? 'Narx o\'zgardi — qayta tekshiring'
              : 'Narx o\'zgardi. Yangi jami: ${formatSum(fresh)} — tasdiqlang',
        );
      } else if (e.statusCode == 409) {
        // out_of_stock: qoldiq tugagan — savatni yangilab, sababini ko'rsatamiz.
        await _cart.refresh().catchError((_) => <String>[]);
        setState(() => _error = e.message);
      } else {
        setState(() => _error = e.message);
      }
    } catch (_) {
      if (mounted) {
        setState(
          () =>
              _error =
                  'Buyurtma yuborilmadi. Internetni tekshirib, qayta urinib ko\'ring',
        );
      }
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final ready = !_submitting && !_loading && !_cart.isEmpty;
    return SheetPage(
      child: Scaffold(
        backgroundColor: const Color(0xFFFAFAFA),
        appBar: const OrderPageAppBar(),
        body:
            _loading
                ? const Center(child: CircularProgressIndicator())
                : ListView(
                  padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
                  children: [
                    _addressCard(),
                    const SizedBox(height: 12),
                    _orderDetailsCard(),
                    const SizedBox(height: 12),
                    _payCard(),
                    const SizedBox(height: 12),
                    _commentCard(),
                    const SizedBox(height: 12),
                    _totalsCard(),
                    if (_error != null) ...[
                      const SizedBox(height: 12),
                      OrderErrorBox(text: _error!),
                    ],
                  ],
                ),
        bottomNavigationBar: OrderSubmitBar(
          buttonKey: const ValueKey('qurt-checkout-submit'),
          label: 'Buyurtma berish · ${formatSum(_cart.totalTiyin)}',
          ready: ready,
          submitting: _submitting,
          onPressed: _submit,
        ),
      ),
    );
  }

  Widget _addressCard() {
    final date = (_delivery?['delivery_date'] ?? '').toString();
    final orderBy =
        DateTime.tryParse((_delivery?['order_by'] ?? '').toString())?.toLocal();
    String two(int n) => n.toString().padLeft(2, '0');
    return OrderCard(
      child: Column(
        children: [
          OrderCardRow(
            icon: Icons.place_outlined,
            title: 'Yetkazib berish manzili',
            subtitle:
                _hasAddress
                    ? AddressScope.instance.text
                    : 'Yetkazib berish uchun manzil kerak',
            onTap: _submitting ? null : _pickAddress,
          ),
          const Divider(height: 1, indent: 44),
          OrderCardRow(
            icon: Icons.local_shipping_outlined,
            title: 'Yetkazish sanasi',
            subtitle: date.isEmpty ? null : formatQurtDate(date),
            note:
                orderBy == null
                    ? 'Buyurtmalar bir necha kun to\'planib, reys bilan yetkaziladi'
                    : 'Reys bilan yetkaziladi. Shu reysga '
                        '${two(orderBy.day)}.${two(orderBy.month)} '
                        '${two(orderBy.hour)}:${two(orderBy.minute)} gacha buyurtma bering',
          ),
        ],
      ),
    );
  }

  Widget _orderDetailsCard() {
    final lines = _cart.lines;
    return OrderCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const OrderDetailsHeader(),
          for (var i = 0; i < lines.length; i++) ...[
            if (i > 0) const Divider(height: 1, indent: 16, endIndent: 16),
            OrderItemTile(
              name: lines[i].name,
              description: [
                if (lines[i].weightGrams > 0) '${lines[i].weightGrams} g',
                lines[i].storeName,
              ].where((s) => s.isNotEmpty).join(' · '),
              imageUrl: lines[i].imageUrl,
              imageFit: BoxFit.contain,
              imageSize: kWideThumbSize,
              placeholderIcon: Icons.bakery_dining,
              qty: lines[i].qty,
              lineTotalTiyin: lines[i].totalTiyin,
              onAdd: () => _add(lines[i]),
              onRemove:
                  () => _cart.setQty(lines[i].productId, lines[i].qty - 1),
            ),
          ],
        ],
      ),
    );
  }

  Widget _payCard() {
    return const OrderCard(
      child: OrderCardRow(
        icon: Icons.payments_outlined,
        title: 'To\'lov usuli',
        subtitle: 'Naqd — yetkazilganda kuryerga to\'laysiz',
        trailing: Icon(Icons.check_circle, color: _green),
      ),
    );
  }

  Widget _commentCard() {
    return OrderCard(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
        child: TextField(
          controller: _comment,
          maxLength: 300,
          maxLines: 2,
          enabled: !_submitting,
          decoration: const InputDecoration(
            hintText: 'Izoh (ixtiyoriy)',
            border: InputBorder.none,
          ),
        ),
      ),
    );
  }

  Widget _totalsCard() {
    return OrderCard(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          children: [
            OrderTotalLine(
              label: 'Savatdagi tovarlar',
              value: formatSum(_cart.totalTiyin),
            ),
            const OrderTotalLine(
              label: 'Yetkazish',
              value: 'Bepul',
              color: _green,
            ),
            const Padding(
              padding: EdgeInsets.symmetric(vertical: 12),
              child: OrderDashedLine(),
            ),
            OrderGrandTotal(
              valueTiyin: _cart.totalTiyin,
              valueKey: const ValueKey('qurt-checkout-total'),
            ),
          ],
        ),
      ),
    );
  }
}

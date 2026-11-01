import 'package:flutter/material.dart';

import '../api.dart';
import '../data/qurt_cart.dart';
import '../widgets/order_blocks.dart';
import '../widgets/page_sheet.dart';
import '../widgets/sheet_page.dart';
import 'qurt_checkout_screen.dart';
import 'qurt_orders_screen.dart';

/// Qurt savati — restoran savati (`cart_screen.dart`) bilan BIR XIL ko'rinish:
/// qatorlar (`CartLineTile`), pastki "Buyurtma rasmiylashtirish" paneli
/// (`CartBottomBar`) va bo'sh savat (`EmptyCartView`) umumiy bloklardan
/// (`widgets/order_blocks.dart`). Buyurtma rasmiylashtirish keyingi sahifada
/// (`QurtCheckoutScreen`), xuddi restorandagidek.
class QurtCartScreen extends StatefulWidget {
  const QurtCartScreen({super.key, QurtCart? cart}) : _cart = cart;

  /// Faqat sinov uchun: yagona savat o'rniga berilgan.
  final QurtCart? _cart;

  @override
  State<QurtCartScreen> createState() => _QurtCartScreenState();
}

class _QurtCartScreenState extends State<QurtCartScreen> {
  late final QurtCart _cart = widget._cart ?? QurtCart.instance;
  bool _syncing = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _cart.addListener(_onCart);
    _init();
  }

  @override
  void dispose() {
    _cart.removeListener(_onCart);
    super.dispose();
  }

  void _onCart() {
    if (mounted) setState(() {});
  }

  Future<void> _init() async {
    await _cart.ensureRestored();
    await _refresh();
  }

  /// Savatni serverdagi joriy narx/qoldiq bilan yangilaydi; o'zgarsa mijozga aytadi.
  Future<void> _refresh() async {
    if (mounted) setState(() => _syncing = true);
    try {
      final notes = await _cart.refresh();
      if (!mounted) return;
      setState(() => _error = null);
      if (notes.isNotEmpty) _snack(notes.join('\n'));
    } catch (e) {
      if (mounted) {
        setState(() => _error = errorText(e, 'Narxlarni yangilab bo\'lmadi'));
      }
    } finally {
      if (mounted) setState(() => _syncing = false);
    }
  }

  void _snack(String msg) {
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(msg)));
  }

  void _add(QurtCartLine l) {
    if (l.qty >= l.maxQty || l.qty >= kQurtMaxQtyPerLine) {
      _snack('Bundan ko\'p qolmagan');
      return;
    }
    _cart.setQty(l.productId, l.qty + 1);
  }

  Future<void> _confirmClear() async {
    final yes = await showDialog<bool>(
      context: context,
      builder:
          (ctx) => AlertDialog(
            title: const Text('Savat tozalansinmi?'),
            content: const Text(
              'Barcha tanlangan mahsulotlar savatdan olib tashlanadi.',
            ),
            actions: [
              TextButton(
                onPressed: () => Navigator.pop(ctx, false),
                child: const Text('Yo\'q'),
              ),
              TextButton(
                onPressed: () => Navigator.pop(ctx, true),
                child: const Text(
                  'Ha, tozalash',
                  style: TextStyle(color: Color(0xFFDC2626)),
                ),
              ),
            ],
          ),
    );
    if (yes == true) _cart.clear();
  }

  /// Rasmiylashtirishga o'tadi; buyurtma yaratilsa (ro'yxat qaytadi) "qabul qilindi" ekrani.
  Future<void> _checkout() async {
    final orders = await Navigator.of(context).push<List<Map<String, dynamic>>>(
      sheetRoute(QurtCheckoutScreen(cart: _cart)),
    );
    if (!mounted || orders == null) return;
    await Navigator.of(
      context,
    ).pushReplacement(sheetRoute(QurtOrderPlacedScreen(orders: orders)));
  }

  @override
  Widget build(BuildContext context) {
    if (_cart.isEmpty) {
      return SheetPage(
        child: const Scaffold(
          backgroundColor: Colors.white,
          appBar: PageAppBar(title: 'Savat'),
          body: EmptyCartView(),
        ),
      );
    }
    return SheetPage(
      child: Scaffold(
        backgroundColor: Colors.white,
        appBar: PageAppBar(
          scrolledUnderElevation: 0.5,
          titleSpacing: 0,
          titleWidget: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              const Text(
                'Qurt savati',
                style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
              ),
              Text(
                formatSum(_cart.totalTiyin),
                key: const ValueKey('qurt-cart-goods-total'),
                style: const TextStyle(
                  fontSize: 13.5,
                  fontWeight: FontWeight.bold,
                  color: Color(0xFF16A34A),
                ),
              ),
            ],
          ),
          actions: [
            IconButton(
              onPressed: _confirmClear,
              icon: const Icon(Icons.delete_outline),
              tooltip: 'Savatni tozalash',
            ),
          ],
        ),
        body: ListView(
          padding: const EdgeInsets.only(bottom: 24),
          children: [
            for (final l in _cart.lines)
              CartLineTile(
                name: l.name,
                weightLabel: l.weightGrams > 0 ? '${l.weightGrams} g' : '',
                imageUrl: l.imageUrl,
                imageFit: BoxFit.contain,
                imageSize: kWideThumbSize,
                placeholderIcon: Icons.bakery_dining,
                qty: l.qty,
                lineTotalTiyin: l.totalTiyin,
                onAdd: () => _add(l),
                onRemove: () => _cart.setQty(l.productId, l.qty - 1),
              ),
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 10, 16, 0),
              child: SizedBox(
                height: 48,
                child: OutlinedButton(
                  key: const ValueKey('qurt-cart-continue'),
                  onPressed: () => Navigator.of(context).pop(),
                  style: OutlinedButton.styleFrom(
                    foregroundColor: const Color(0xFF171717),
                    side: const BorderSide(color: Color(0xFFE0E0E0)),
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(14),
                    ),
                  ),
                  child: const Text(
                    'Qurt mahsulotlarini ochish',
                    style: TextStyle(fontSize: 15, fontWeight: FontWeight.w600),
                  ),
                ),
              ),
            ),
          ],
        ),
        bottomNavigationBar: CartBottomBar(
          buttonKey: const ValueKey('qurt-cart-checkout'),
          totalKey: const ValueKey('qurt-cart-bottom-total'),
          trailing: formatSum(_cart.totalTiyin),
          loading: _syncing,
          error: _error,
          onRetry: _refresh,
          onPressed: _syncing ? null : _checkout,
        ),
      ),
    );
  }
}

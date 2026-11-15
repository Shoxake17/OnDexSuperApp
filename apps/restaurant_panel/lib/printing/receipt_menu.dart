import 'package:flutter/material.dart';

import '../api.dart';
import '../theme.dart';
import 'receipt_model.dart';
import 'receipt_printer.dart';

/// Restoran nomi (chek sarlavhasi) — bir marta o'qiladi.
class RestaurantNameCache {
  RestaurantNameCache._();
  static String _name = '';
  static Future<String>? _loading;

  static Future<String> get() async {
    if (_name.isNotEmpty) return _name;
    _loading ??= () async {
      try {
        final r = await api.myRestaurant();
        _name = '${r['name'] ?? ''}'.trim();
      } catch (_) {
        // Sarlavhasiz chek ham chiqadi ("Restoran").
      } finally {
        _loading = null;
      }
      return _name;
    }();
    return _loading!;
  }
}

/// Buyurtma kartochkasidagi chek tugmasi: oshxona yoki mijoz chekini
/// qo'lda (qayta) chiqarish.
class ReceiptMenuButton extends StatelessWidget {
  const ReceiptMenuButton({super.key, required this.order});

  final Map<String, dynamic> order;

  Future<void> _print(BuildContext context, ReceiptKind kind) async {
    final messenger = ScaffoldMessenger.maybeOf(context);
    try {
      final name = await RestaurantNameCache.get();
      await ReceiptPrinter.printOrder(order, kind, restaurantName: name);
    } catch (e) {
      messenger?.showSnackBar(SnackBar(content: Text('Chek chiqmadi: $e')));
    }
  }

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: 26,
      height: 22,
      child: PopupMenuButton<ReceiptKind>(
        key: const ValueKey('order-receipt-menu'),
        tooltip: 'Chek chiqarish',
        padding: EdgeInsets.zero,
        iconSize: 18,
        icon: const Icon(Icons.print_outlined, color: OnDexColors.inkFaint),
        onSelected: (k) => _print(context, k),
        itemBuilder: (_) => const [
          PopupMenuItem(value: ReceiptKind.kitchen, child: Text('Oshxona cheki')),
          PopupMenuItem(value: ReceiptKind.customer, child: Text('Mijoz cheki')),
        ],
      ),
    );
  }
}

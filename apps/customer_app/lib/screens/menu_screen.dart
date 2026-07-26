import 'package:flutter/material.dart';

import '../api.dart';
import 'tracking_screen.dart';

class MenuScreen extends StatefulWidget {
  final String restaurantId;
  final String restaurantName;

  const MenuScreen({
    super.key,
    required this.restaurantId,
    required this.restaurantName,
  });

  @override
  State<MenuScreen> createState() => _MenuScreenState();
}

class _MenuScreenState extends State<MenuScreen> {
  late Future<List<dynamic>> _future;
  final Map<String, int> _cart = {}; // product_id -> qty
  List<dynamic> _menu = [];
  bool _ordering = false;

  @override
  void initState() {
    super.initState();
    _future = api.menu(widget.restaurantId);
  }

  int get _totalTiyin {
    var total = 0;
    for (final p in _menu) {
      final qty = _cart[p['id']] ?? 0;
      total += (p['price_tiyin'] as int) * qty;
    }
    return total;
  }

  Future<void> _order() async {
    setState(() => _ordering = true);
    try {
      final items = _cart.entries
          .where((e) => e.value > 0)
          .map((e) => {'product_id': e.key, 'qty': e.value})
          .toList();
      // MVP: yetkazish nuqtasi sifatida Chust markazi; keyin xarita tanlovi qo'shiladi.
      final order = await api.createOrder(items, 41.0056, 71.2378);
      if (!mounted) return;
      Navigator.of(context).pushReplacement(
        MaterialPageRoute(builder: (_) => TrackingScreen(orderId: order['id'])),
      );
    } on ApiException catch (e) {
      _showError(e.message);
    } catch (_) {
      _showError('Serverga ulanib bo\'lmadi');
    } finally {
      if (mounted) setState(() => _ordering = false);
    }
  }

  void _showError(String msg) {
    ScaffoldMessenger.of(context)
        .showSnackBar(SnackBar(content: Text(msg)));
  }

  @override
  Widget build(BuildContext context) {
    final hasItems = _cart.values.any((q) => q > 0);
    return Scaffold(
      appBar: AppBar(title: Text(widget.restaurantName)),
      body: FutureBuilder<List<dynamic>>(
        future: _future,
        builder: (context, snap) {
          if (snap.connectionState != ConnectionState.done) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snap.hasError) {
            return Center(child: Text('Xato: ${snap.error}'));
          }
          _menu = snap.data ?? [];
          return ListView.builder(
            itemCount: _menu.length,
            itemBuilder: (context, i) {
              final p = _menu[i] as Map<String, dynamic>;
              final qty = _cart[p['id']] ?? 0;
              final available = p['available'] == true;
              return ListTile(
                title: Text(p['name'] ?? ''),
                subtitle: Text(available
                    ? formatSum(p['price_tiyin'] as int)
                    : 'Hozir mavjud emas'),
                enabled: available,
                trailing: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    IconButton(
                      icon: const Icon(Icons.remove_circle_outline),
                      onPressed: !available || qty == 0
                          ? null
                          : () => setState(() => _cart[p['id']] = qty - 1),
                    ),
                    Text('$qty', style: const TextStyle(fontSize: 16)),
                    IconButton(
                      icon: const Icon(Icons.add_circle_outline),
                      onPressed: !available
                          ? null
                          : () => setState(() => _cart[p['id']] = qty + 1),
                    ),
                  ],
                ),
              );
            },
          );
        },
      ),
      bottomNavigationBar: hasItems
          ? SafeArea(
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: FilledButton(
                  onPressed: _ordering ? null : _order,
                  child: Text(_ordering
                      ? 'Yuborilmoqda...'
                      : 'Buyurtma berish — ${formatSum(_totalTiyin)}'),
                ),
              ),
            )
          : null,
    );
  }
}

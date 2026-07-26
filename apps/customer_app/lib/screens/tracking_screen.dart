import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

import '../api.dart';
import 'restaurants_screen.dart';

/// Buyurtma holatini jonli kuzatish: avval GET bilan hozirgi holat olinadi,
/// keyin WebSocket'dan order_status eventlari kelib UI yangilanadi.
class TrackingScreen extends StatefulWidget {
  final String orderId;

  const TrackingScreen({super.key, required this.orderId});

  @override
  State<TrackingScreen> createState() => _TrackingScreenState();
}

class _TrackingScreenState extends State<TrackingScreen> {
  static const steps = [
    ('created', 'Buyurtma qabul qilindi', Icons.receipt_long),
    ('accepted', 'Restoran tasdiqladi', Icons.storefront),
    ('preparing', 'Tayyorlanmoqda', Icons.soup_kitchen),
    ('ready', 'Tayyor', Icons.check_circle_outline),
    ('picked_up', 'Kuryer yo\'lda', Icons.delivery_dining),
    ('delivered', 'Yetkazildi', Icons.done_all),
  ];

  String _status = 'created';
  String _courierId = '';
  WebSocketChannel? _channel;

  @override
  void initState() {
    super.initState();
    _load();
    _connect();
  }

  Future<void> _load() async {
    try {
      final o = await api.getOrder(widget.orderId);
      if (!mounted) return;
      setState(() {
        _status = o['status'] ?? 'created';
        _courierId = o['courier_id'] ?? '';
      });
    } catch (_) {
      // WebSocket baribir yangilab turadi
    }
  }

  void _connect() {
    final token = api.token;
    if (token == null) return;
    _channel = WebSocketChannel.connect(Uri.parse(wsUrl(token)));
    _channel!.stream.listen((msg) {
      final e = jsonDecode(msg as String) as Map<String, dynamic>;
      if (e['type'] == 'order_status' && e['order_id'] == widget.orderId) {
        if (!mounted) return;
        setState(() {
          _status = e['status'] ?? _status;
          _courierId = e['courier_id'] ?? _courierId;
        });
      }
    }, onError: (_) {}, cancelOnError: false);
  }

  @override
  void dispose() {
    _channel?.sink.close();
    super.dispose();
  }

  int get _currentIndex {
    final i = steps.indexWhere((s) => s.$1 == _status);
    return i < 0 ? 0 : i;
  }

  @override
  Widget build(BuildContext context) {
    final cancelled = _status == 'cancelled' || _status == 'rejected';
    final done = _status == 'delivered';
    return Scaffold(
      appBar: AppBar(title: const Text('Buyurtma holati')),
      body: Padding(
        padding: const EdgeInsets.all(24),
        child: cancelled
            ? Center(
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    const Icon(Icons.cancel_outlined,
                        size: 64, color: Colors.red),
                    const SizedBox(height: 16),
                    Text(
                      _status == 'rejected'
                          ? 'Restoran buyurtmani rad etdi'
                          : 'Buyurtma bekor qilindi',
                      style: Theme.of(context).textTheme.titleLarge,
                    ),
                  ],
                ),
              )
            : ListView(
                children: [
                  for (final (i, step) in steps.indexed)
                    ListTile(
                      leading: Icon(
                        step.$3,
                        color: i <= _currentIndex
                            ? Theme.of(context).colorScheme.primary
                            : Colors.grey,
                      ),
                      title: Text(
                        step.$2,
                        style: TextStyle(
                          fontWeight: i == _currentIndex
                              ? FontWeight.bold
                              : FontWeight.normal,
                          color: i <= _currentIndex ? null : Colors.grey,
                        ),
                      ),
                      trailing: i < _currentIndex
                          ? const Icon(Icons.check, color: Colors.green)
                          : (i == _currentIndex && !done
                              ? const SizedBox(
                                  width: 18,
                                  height: 18,
                                  child: CircularProgressIndicator(
                                      strokeWidth: 2),
                                )
                              : null),
                    ),
                  if (_courierId.isNotEmpty)
                    Padding(
                      padding: const EdgeInsets.only(top: 16),
                      child: Text('Kuryer: $_courierId',
                          textAlign: TextAlign.center,
                          style: Theme.of(context).textTheme.bodyLarge),
                    ),
                ],
              ),
      ),
      bottomNavigationBar: done || cancelled
          ? SafeArea(
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: FilledButton(
                  onPressed: () => Navigator.of(context).pushReplacement(
                    MaterialPageRoute(
                        builder: (_) => const RestaurantsScreen()),
                  ),
                  child: const Text('Bosh sahifaga qaytish'),
                ),
              ),
            )
          : null,
    );
  }
}

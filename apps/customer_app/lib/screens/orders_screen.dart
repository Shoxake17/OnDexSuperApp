import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

import '../api.dart';
import '../widgets/order_status.dart';
import '../widgets/sheet_scaffold.dart';
import 'mini_app_webview.dart';

/// "Buyurtmalarim" bo'limi — mijozning barcha (faol va tugagan)
/// buyurtmalari ro'yxati, eng yangisi tepada. Holat WebSocket orqali
/// JONLI yangilanadi (refresh tugmasini bosish shart emas) — restoran
/// panelidagi va TrackingScreen'dagi bilan bir xil naqsh: uzilib qolsa
/// 2 soniyadan keyin avtomatik qayta ulanadi, 20 soniyalik zaxira polling
/// bilan birga.
class OrdersScreen extends StatefulWidget {
  const OrdersScreen({super.key});

  @override
  State<OrdersScreen> createState() => _OrdersScreenState();
}

class _OrdersScreenState extends State<OrdersScreen> {
  List<dynamic> _orders = [];
  bool _loading = true;
  String? _error;
  WebSocketChannel? _channel;
  Timer? _pollTimer;

  @override
  void initState() {
    super.initState();
    _load();
    _connectWs();
    _pollTimer = Timer.periodic(const Duration(seconds: 20), (_) => _load());
  }

  @override
  void dispose() {
    _pollTimer?.cancel();
    _channel?.sink.close();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final l = await api.myOrders();
      if (!mounted) return;
      setState(() {
        _orders = l;
        _loading = false;
        _error = null;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        if (_orders.isEmpty) _error = '$e';
      });
    }
  }

  Future<void> _connectWs() async {
    if (api.token == null) return;
    final String ticket;
    try {
      ticket = await api.wsTicket();
    } catch (_) {
      _scheduleReconnect();
      return;
    }
    if (!mounted) return;
    _channel = WebSocketChannel.connect(Uri.parse(wsUrl(ticket)));
    _channel!.stream.listen((msg) {
      final e = jsonDecode(msg as String) as Map<String, dynamic>;
      if (e['type'] == 'order_status' || e['type'] == 'courier_assigned') {
        _load();
      }
    },
        onError: (_) => _scheduleReconnect(),
        onDone: _scheduleReconnect,
        cancelOnError: true);
  }

  void _scheduleReconnect() {
    Future.delayed(const Duration(seconds: 2), () {
      if (!mounted) return;
      _connectWs();
    });
  }

  @override
  Widget build(BuildContext context) {
    return SheetScaffold(
      title: 'Buyurtmalarim',
      child: SafeArea(
        top: false,
        child: RefreshIndicator(
          onRefresh: _load,
          child: _loading
              ? const Center(child: CircularProgressIndicator())
              : _error != null
                  ? ListView(
                      children: [
                        const SizedBox(height: 120),
                        Center(child: Text('Xato: $_error')),
                      ],
                    )
                  : _orders.isEmpty
                      ? ListView(
                          children: const [
                            SizedBox(height: 120),
                            Center(
                              child: Text('Hali buyurtmalar yo\'q',
                                  style: TextStyle(color: Colors.grey)),
                            ),
                          ],
                        )
                      : ListView.separated(
                          padding: const EdgeInsets.all(16),
                          itemCount: _orders.length,
                          separatorBuilder: (_, __) =>
                              const SizedBox(height: 12),
                          itemBuilder: (context, i) => _OrderCard(
                              order: _orders[i] as Map<String, dynamic>),
                        ),
        ),
      ),
    );
  }
}

class _OrderCard extends StatelessWidget {
  final Map<String, dynamic> order;
  const _OrderCard({required this.order});

  @override
  Widget build(BuildContext context) {
    final id = order['id'] as String;
    final status = order['status'] as String? ?? 'created';
    final (label, icon, color) = statusStyleOf(status);
    final restaurantName = order['restaurant_name'] as String? ?? '';
    final logoUrl = order['restaurant_logo_url'] as String? ?? '';
    final total = (order['total_tiyin'] ?? 0) as int;
    final items = (order['items'] as List?) ?? [];
    final itemCount =
        items.fold<int>(0, (a, i) => a + ((i['qty'] ?? 1) as int));
    final createdAt = DateTime.tryParse(order['created_at'] as String? ?? '');
    final surface = Theme.of(context).colorScheme.surfaceContainerHighest;
    final stage = stageOf(status);

    return Card(
      margin: EdgeInsets.zero,
      elevation: 0,
      color: surface.withValues(alpha: 0.4),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(20),
        side: BorderSide(color: Colors.grey.withValues(alpha: 0.12)),
      ),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: () => Navigator.of(context).push(miniAppRoute('/orders/$id')),
        child: Padding(
          padding: const EdgeInsets.all(14),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  ClipRRect(
                    borderRadius: BorderRadius.circular(16),
                    child: SizedBox(
                      width: 64,
                      height: 64,
                      child: logoUrl.isEmpty
                          ? Container(
                              color: surface,
                              child: const Icon(Icons.storefront,
                                  color: Colors.grey, size: 28))
                          : Image.network(
                              fullImageUrl(logoUrl),
                              fit: BoxFit.cover,
                              errorBuilder: (_, __, ___) => Container(
                                  color: surface,
                                  child: const Icon(Icons.storefront,
                                      color: Colors.grey, size: 28)),
                            ),
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          children: [
                            Expanded(
                              child: Text(
                                  order['order_number']?.toString() ?? '—',
                                  style: TextStyle(
                                      fontSize: 12,
                                      letterSpacing: 0.5,
                                      color: Colors.grey.shade500)),
                            ),
                            if (createdAt != null)
                              Text(_formatTime(createdAt),
                                  style: TextStyle(
                                      fontSize: 12,
                                      color: Colors.grey.shade500)),
                          ],
                        ),
                        const SizedBox(height: 3),
                        Row(
                          children: [
                            Expanded(
                              child: Text(
                                  restaurantName.isEmpty
                                      ? 'Restoran'
                                      : restaurantName,
                                  style: const TextStyle(
                                      fontWeight: FontWeight.bold,
                                      fontSize: 16),
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis),
                            ),
                            Text(formatSum(total),
                                style: const TextStyle(
                                    fontWeight: FontWeight.bold)),
                          ],
                        ),
                        const SizedBox(height: 2),
                        Text('$itemCount ta mahsulot',
                            style: TextStyle(
                                fontSize: 13, color: Colors.grey.shade500)),
                        const SizedBox(height: 8),
                        Row(
                          children: [
                            Container(
                              padding: const EdgeInsets.symmetric(
                                  horizontal: 8, vertical: 3),
                              decoration: BoxDecoration(
                                color: color.withValues(alpha: 0.16),
                                borderRadius: BorderRadius.circular(20),
                              ),
                              child: Row(
                                mainAxisSize: MainAxisSize.min,
                                children: [
                                  Icon(icon, size: 14, color: color),
                                  const SizedBox(width: 5),
                                  Text(label,
                                      style: TextStyle(
                                          color: color,
                                          fontWeight: FontWeight.w600,
                                          fontSize: 12)),
                                ],
                              ),
                            ),
                            const Spacer(),
                            Icon(Icons.chevron_right,
                                size: 18, color: Colors.grey.shade600),
                          ],
                        ),
                      ],
                    ),
                  ),
                ],
              ),
              if (stage >= 0) ...[
                const SizedBox(height: 16),
                OrderProgressStepper(stage: stage),
              ],
            ],
          ),
        ),
      ),
    );
  }
}

/// Bugungi buyurtma bo'lsa faqat vaqt, aks holda sana ham qo'shiladi —
/// "Buyurtmalarim" bir necha kunlik tarixni ko'rsatishi mumkin, shuning
/// uchun faqat vaqt ko'p kunlik ro'yxatda chalkash bo'lardi.
String _formatTime(DateTime d) {
  final local = d.toLocal();
  final now = DateTime.now();
  String two(int n) => n.toString().padLeft(2, '0');
  final time = '${two(local.hour)}:${two(local.minute)}';
  final sameDay = local.year == now.year &&
      local.month == now.month &&
      local.day == now.day;
  return sameDay ? time : '${two(local.day)}.${two(local.month)} $time';
}

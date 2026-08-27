import 'package:flutter/material.dart';

import '../api.dart';
import '../widgets/common.dart';
import '../live.dart';
import '../widgets/order_status.dart';
import '../widgets/sheet_page.dart';
import '../widgets/sheet_scaffold.dart';
import 'tracking_screen.dart';

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
  // ┌─ JONLI KANAL UMUMIY YADRODAN ─────────────────────────────────┐
  // Bu yerda avval xom `WebSocketChannel`, qo'lda yozilgan qayta
  // ulanish (qat'iy 2 soniya) va alohida 20 soniyalik taymer turardi.
  // Ayni mantiq `ondex_core` dagi `LiveBus`/`LiveRefresher` da bor va
  // restoran paneli o'shani ishlatadi — ya'ni bitta stack ichida ikki
  // nusxa edi va ular allaqachon ajralib ketgan (yadroda eksponensial
  // backoff + jitter bor, bu yerda yo'q edi).
  //
  // Endi ikkalasi ham AYNI koddan oziqlanadi.
  // └────────────────────────────────────────────────────────────────┘
  late final LiveRefresher _live;

  @override
  void initState() {
    super.initState();
    _load();
    customerLive.start();
    _live = LiveRefresher(
      bus: customerLive,
      onRefresh: _load,
      types: const {'order_status', 'courier_assigned'},
    )..start();
  }

  @override
  void dispose() {
    _live.dispose();
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
                              child: Text('Hozircha buyurtma yo\'q',
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
    // Stol (QR) buyurtmasida kuryer YO'Q — matn, ikonka va bosqichlar
    // shunga qarab o'zgaradi. Busiz zalda o'tirgan mijoz "Tayyor —
    // kuryer kutilmoqda" degan yozuvni ko'rardi.
    final isDineIn = (order['type'] as String?) == 'dine_in';
    final (label, icon, color) = statusStyleOf(status, dineIn: isDineIn);
    final restaurantName = order['restaurant_name'] as String? ?? '';
    final logoUrl = order['restaurant_logo_url'] as String? ?? '';
    final total = (order['total_tiyin'] ?? 0) as int;
    final items = (order['items'] as List?) ?? [];
    final itemCount =
        items.fold<int>(0, (a, i) => a + ((i['qty'] ?? 1) as int));
    final createdAt = DateTime.tryParse(order['created_at'] as String? ?? '');
    // Kartochka ranglari vebdagi bilan bir xil: OQ fon + neytral
    // chegara (`bg-white border-neutral-200`). Avval bu yerda
    // `surfaceContainerHighest` ishlatilardi — u mavzuga bog'liq
    // kulrang berib, veb kartochkasidan sezilarli farq qilardi.
    const surface = Color(0xFFF5F5F5);
    final stage = stageOf(status, dineIn: isDineIn);

    return Card(
      margin: EdgeInsets.zero,
      elevation: 0,
      color: Colors.white,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(16),
        side: const BorderSide(color: Color(0xFFE5E5E5)),
      ),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: () => Navigator.of(context).push(
          sheetRoute(TrackingScreen(orderId: id)),
        ),
        child: Padding(
          padding: const EdgeInsets.all(14),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  ClipRRect(
                    borderRadius: BorderRadius.circular(12),
                    child: SizedBox(
                      // Vebdagi `h-12 w-12` bilan bir xil — 64px logo
                      // kartochkani kerakdan baland qilardi.
                      width: 48,
                      height: 48,
                      // Bo'sh yo'l ham, yiqilgan rasm ham bitta
                      // o'rinbosarga tushadi (`RemoteImage`).
                      child: RemoteImage(
                        url: logoUrl,
                        placeholder: Container(
                            color: surface,
                            child: const Icon(Icons.storefront,
                                color: Colors.grey, size: 20)),
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
                        // Vebdagi bilan bir xil ibora ("ta taom") —
                        // ikkala ilovada bir narsa ikki xil atalmasin.
                        Text('$itemCount ta taom',
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
                OrderProgressStepper(stage: stage, dineIn: isDineIn),
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

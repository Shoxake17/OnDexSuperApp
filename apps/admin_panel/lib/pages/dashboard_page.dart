import 'package:flutter/material.dart';

import '../api.dart';
import '../live.dart';

class DashboardPage extends StatefulWidget {
  const DashboardPage({super.key});

  @override
  State<DashboardPage> createState() => _DashboardPageState();
}

class _DashboardPageState extends State<DashboardPage> {
  Map<String, dynamic>? _stats;
  String? _error;
  late final LiveRefresher _live;

  @override
  void initState() {
    super.initState();
    _load();
    // Ko'rsatkichlar JONLI kanaldan yangilanadi; so'rov sikli esa
    // faqat zaxira (`live.dart` dagi izoh).
    _live = LiveRefresher(
      bus: adminLive,
      onRefresh: _load,
      types: const {
        'new_order',
        'order_status',
        'courier_assigned',
        'courier_registered',
        'courier_status',
      },
    )..start();
  }

  @override
  void dispose() {
    _live.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final s = await api.stats();
      if (!mounted) return;
      setState(() {
        _stats = s;
        _error = null;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() => _error = e.toString());
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_error != null && _stats == null) {
      return Center(child: Text('Xato: $_error'));
    }
    if (_stats == null) {
      return const Center(child: CircularProgressIndicator());
    }
    final s = _stats!;
    final byStatus = Map<String, dynamic>.from(s['by_status'] ?? {});
    return Padding(
      padding: const EdgeInsets.all(24),
      child: ListView(
        children: [
          Text('Boshqaruv paneli',
              style: Theme.of(context).textTheme.headlineMedium),
          const SizedBox(height: 4),
          Text('Har 10 soniyada avtomatik yangilanadi',
              style: Theme.of(context).textTheme.bodySmall),
          const SizedBox(height: 24),
          // ┌─ BIR QATORDA AYNAN 6 TA ──────────────────────────────────┐
          // Avval `Wrap` va qat'iy `width: 220` ishlatilardi. Wrap
          // "sig'gancha joylashtir" degani — oyna eniga qarab qatorda
          // 4, 5 yoki 6 ta chiqardi va oxirgi karta pastga tushib
          // qolardi.
          //
          // Endi en HISOBLANADI: mavjud joydan oraliqlar ayirilib
          // oltiga bo'linadi. Natijada karta soni oyna o'lchamiga
          // bog'liq emas.
          //
          // Tor oynada (noutbuk, panel yonma-yon ochilgan) 6 ta karta
          // o'qib bo'lmas darajada siqilib ketardi — shuning uchun
          // 1100 px dan tor bo'lsa 3 tadan ikki qator qilinadi.
          // └────────────────────────────────────────────────────────────┘
          LayoutBuilder(
            builder: (context, box) {
              const gap = 16.0;
              final perRow = box.maxWidth < 1100 ? 3 : 6;
              final w = (box.maxWidth - gap * (perRow - 1)) / perRow;
              return Wrap(
                spacing: gap,
                runSpacing: gap,
                children: [
                  _StatCard(
                      width: w,
                      title: 'Bugungi buyurtmalar',
                      value: '${s['orders_today'] ?? 0}',
                      icon: Icons.receipt_long,
                      color: Colors.blue),
                  _StatCard(
                      width: w,
                      title: 'Bugungi tushum',
                      value: formatSum((s['revenue_today_tiyin'] ?? 0) as int),
                      icon: Icons.payments,
                      color: Colors.green),
                  _StatCard(
                      width: w,
                      title: 'Bugun yetkazildi',
                      value: '${s['delivered_today'] ?? 0}',
                      icon: Icons.done_all,
                      color: Colors.teal),
                  _StatCard(
                      width: w,
                      title: 'Kuryerlar online',
                      value: '${s['couriers_online'] ?? 0}',
                      icon: Icons.delivery_dining,
                      color: Colors.orange),
                  _StatCard(
                      width: w,
                      title: 'Tasdiq kutayotgan',
                      value: '${s['couriers_pending'] ?? 0}',
                      icon: Icons.hourglass_top,
                      color: Colors.red),
                  _StatCard(
                      width: w,
                      title: 'Restoranlar',
                      value: '${s['restaurants_total'] ?? 0}',
                      icon: Icons.storefront,
                      color: Colors.purple),
                ],
              );
            },
          ),
          const SizedBox(height: 32),
          Text('Buyurtmalar holati bo\'yicha (so\'nggi 500 ta)',
              style: Theme.of(context).textTheme.titleLarge),
          const SizedBox(height: 12),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              for (final e in byStatus.entries)
                Chip(
                  label: Text('${orderStatusLabel(e.key)}: ${e.value}'),
                ),
            ],
          ),
        ],
      ),
    );
  }
}

class _StatCard extends StatelessWidget {
  final String title;
  final String value;
  final IconData icon;
  final Color color;

  /// En tashqaridan beriladi — qatordagi karta soni `DashboardPage` da
  /// hisoblanadi (izohi o'sha yerda).
  final double width;

  const _StatCard({
    required this.title,
    required this.value,
    required this.icon,
    required this.color,
    required this.width,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Container(
        width: width,
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(icon, color: color, size: 20),
                const SizedBox(width: 8),
                Expanded(
                  // Karta tor bo'lganda uzun sarlavha ikki qatorga
                  // tushadi; uchinchisi kesiladi — kartalar bo'yi bir
                  // xil qolsin.
                  child: Text(title,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: Theme.of(context).textTheme.bodySmall),
                ),
              ],
            ),
            const SizedBox(height: 12),
            FittedBox(
              fit: BoxFit.scaleDown,
              alignment: Alignment.centerLeft,
              // Tushum summasi uzun bo'ladi ("1 250 000 so'm") va tor
              // kartada sig'masdi. `FittedBox` uni kesish o'rniga
              // kichraytiradi.
              child: Text(value,
                  style: Theme.of(context).textTheme.headlineSmall),
            ),
          ],
        ),
      ),
    );
  }
}

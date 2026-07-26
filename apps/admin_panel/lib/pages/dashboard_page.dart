import 'dart:async';

import 'package:flutter/material.dart';

import '../api.dart';

class DashboardPage extends StatefulWidget {
  const DashboardPage({super.key});

  @override
  State<DashboardPage> createState() => _DashboardPageState();
}

class _DashboardPageState extends State<DashboardPage> {
  Map<String, dynamic>? _stats;
  String? _error;
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    _load();
    _timer = Timer.periodic(const Duration(seconds: 10), (_) => _load());
  }

  @override
  void dispose() {
    _timer?.cancel();
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
          Wrap(
            spacing: 16,
            runSpacing: 16,
            children: [
              _StatCard(
                  title: 'Bugungi buyurtmalar',
                  value: '${s['orders_today'] ?? 0}',
                  icon: Icons.receipt_long,
                  color: Colors.blue),
              _StatCard(
                  title: 'Bugungi tushum',
                  value: formatSum((s['revenue_today_tiyin'] ?? 0) as int),
                  icon: Icons.payments,
                  color: Colors.green),
              _StatCard(
                  title: 'Bugun yetkazildi',
                  value: '${s['delivered_today'] ?? 0}',
                  icon: Icons.done_all,
                  color: Colors.teal),
              _StatCard(
                  title: 'Kuryerlar online',
                  value: '${s['couriers_online'] ?? 0}',
                  icon: Icons.delivery_dining,
                  color: Colors.orange),
              _StatCard(
                  title: 'Tasdiq kutayotgan kuryerlar',
                  value: '${s['couriers_pending'] ?? 0}',
                  icon: Icons.hourglass_top,
                  color: Colors.red),
              _StatCard(
                  title: 'Restoranlar',
                  value: '${s['restaurants_total'] ?? 0}',
                  icon: Icons.storefront,
                  color: Colors.purple),
            ],
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
                  label: Text('${statusLabels[e.key] ?? e.key}: ${e.value}'),
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

  const _StatCard({
    required this.title,
    required this.value,
    required this.icon,
    required this.color,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Container(
        width: 220,
        padding: const EdgeInsets.all(20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(icon, color: color),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(title,
                      style: Theme.of(context).textTheme.bodyMedium),
                ),
              ],
            ),
            const SizedBox(height: 12),
            Text(value, style: Theme.of(context).textTheme.headlineSmall),
          ],
        ),
      ),
    );
  }
}

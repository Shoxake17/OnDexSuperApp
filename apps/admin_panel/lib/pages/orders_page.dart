import 'dart:async';

import 'package:flutter/material.dart';

import '../api.dart';

class OrdersPage extends StatefulWidget {
  const OrdersPage({super.key});

  @override
  State<OrdersPage> createState() => _OrdersPageState();
}

class _OrdersPageState extends State<OrdersPage> {
  List<dynamic> _list = [];
  bool _loading = true;
  Timer? _timer;

  static const _statusColors = {
    'created': Colors.blue,
    'accepted': Colors.indigo,
    'preparing': Colors.orange,
    'ready': Colors.amber,
    'picked_up': Colors.teal,
    'delivered': Colors.green,
    'cancelled': Colors.grey,
    'rejected': Colors.red,
  };

  @override
  void initState() {
    super.initState();
    _load();
    _timer = Timer.periodic(const Duration(seconds: 5), (_) => _load());
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final l = await api.orders();
      if (!mounted) return;
      setState(() {
        _list = l;
        _loading = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() => _loading = false);
    }
  }

  String _time(String? iso) {
    if (iso == null) return '';
    final t = DateTime.tryParse(iso)?.toLocal();
    if (t == null) return '';
    two(int n) => n.toString().padLeft(2, '0');
    return '${two(t.day)}.${two(t.month)} ${two(t.hour)}:${two(t.minute)}';
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text('Buyurtmalar',
                  style: Theme.of(context).textTheme.headlineMedium),
              const Spacer(),
              Text('Har 5 soniyada yangilanadi',
                  style: Theme.of(context).textTheme.bodySmall),
            ],
          ),
          const SizedBox(height: 16),
          Expanded(
            child: _loading
                ? const Center(child: CircularProgressIndicator())
                : _list.isEmpty
                    ? const Center(child: Text('Hozircha buyurtma yo\'q'))
                    : SingleChildScrollView(
                        child: SizedBox(
                          width: double.infinity,
                          child: DataTable(
                            columns: const [
                              DataColumn(label: Text('Vaqt')),
                              DataColumn(label: Text('ID')),
                              DataColumn(label: Text('Restoran')),
                              DataColumn(label: Text('Kuryer')),
                              DataColumn(label: Text('Summa')),
                              DataColumn(label: Text('Holat')),
                            ],
                            rows: [
                              for (final o in _list.cast<Map<String, dynamic>>())
                                DataRow(cells: [
                                  DataCell(Text(_time(o['created_at']))),
                                  DataCell(Text(
                                      (o['id'] as String? ?? '').substring(0, 8))),
                                  DataCell(Text(o['restaurant_id'] ?? '')),
                                  DataCell(Text(o['courier_id'] ?? '—')),
                                  DataCell(
                                      Text(formatSum((o['total_tiyin'] ?? 0) as int))),
                                  DataCell(Chip(
                                    label: Text(
                                      statusLabels[o['status']] ??
                                          o['status'] ??
                                          '',
                                      style: const TextStyle(
                                          color: Colors.white, fontSize: 12),
                                    ),
                                    backgroundColor:
                                        _statusColors[o['status']] ??
                                            Colors.grey,
                                    padding: EdgeInsets.zero,
                                  )),
                                ]),
                            ],
                          ),
                        ),
                      ),
          ),
        ],
      ),
    );
  }
}

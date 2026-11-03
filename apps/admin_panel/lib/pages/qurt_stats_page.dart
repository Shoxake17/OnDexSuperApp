import 'package:flutter/material.dart';

import '../api.dart';

/// "Qurt statistikasi": FAQAT o'qish.
///
/// Logistikani (kuryerlar, reysga biriktirish, naqd pul) do'kon o'zi boshqaradi — OnDexPos
/// panelidagi "Logistika" bo'limida. Admin bu yerda umumiy raqamlarni va do'konlar kesimini ko'radi.
class QurtStatsPage extends StatefulWidget {
  const QurtStatsPage({super.key});

  @override
  State<QurtStatsPage> createState() => _QurtStatsPageState();
}

const _statusTitle = <String, String>{
  'new': 'Yangi',
  'confirmed': 'Qabul qilingan',
  'packed': 'Qadoqlangan',
  'collected': 'Kuryer oldi',
  'out_for_delivery': 'Yo\'lda',
  'delivered': 'Yetkazilgan',
  'failed': 'Yetkazilmadi',
  'rejected': 'Rad etilgan',
  'cancelled': 'Bekor qilingan',
};

class _QurtStatsPageState extends State<QurtStatsPage> {
  Map<String, dynamic>? _data;
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final d = await api.qurtStats();
      if (mounted) setState(() => _data = d);
    } on ApiException catch (e) {
      if (mounted) setState(() => _error = e.message);
    } catch (e) {
      if (mounted) setState(() => _error = '$e');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  int _int(Object? v) => (v as num?)?.toInt() ?? 0;

  @override
  Widget build(BuildContext context) {
    final d = _data;
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(24, 20, 24, 8),
          child: Row(children: [
            Text('Qurt statistikasi',
                style: Theme.of(context).textTheme.headlineSmall),
            const Spacer(),
            IconButton(
              tooltip: 'Yangilash',
              onPressed: _loading ? null : _load,
              icon: const Icon(Icons.refresh),
            ),
          ]),
        ),
        Expanded(
          child: _loading && d == null
              ? const Center(child: CircularProgressIndicator())
              : d == null
                  ? Center(
                      child: Column(mainAxisSize: MainAxisSize.min, children: [
                        Text(_error ?? 'Ma\'lumot yo\'q'),
                        const SizedBox(height: 12),
                        FilledButton(
                            onPressed: _load,
                            child: const Text('Qayta urinish')),
                      ]),
                    )
                  : _content(d),
        ),
      ],
    );
  }

  Widget _content(Map<String, dynamic> d) {
    final byStatus = (d['by_status'] as Map?) ?? const {};
    final stores = [
      for (final s in (d['stores'] as List? ?? const []))
        if (s is Map) Map<String, dynamic>.from(s)
    ];
    return ListView(
      padding: const EdgeInsets.fromLTRB(24, 8, 24, 32),
      children: [
        Wrap(spacing: 12, runSpacing: 12, children: [
          _tile(Icons.receipt_long_outlined, 'Buyurtmalar',
              '${_int(d['orders_total'])}'),
          _tile(Icons.payments_outlined, 'Sotilgan (yetkazilgan)',
              formatSum(_int(d['sold_tiyin']))),
          _tile(Icons.account_balance_wallet_outlined, 'Kuryerlarda naqd pul',
              formatSum(_int(d['cash_held_tiyin']))),
          _tile(Icons.verified_outlined, 'Do\'konlarga topshirilgan',
              formatSum(_int(d['cash_settled_tiyin']))),
          _tile(Icons.delivery_dining_outlined, 'Qurt kuryerlari',
              '${_int(d['couriers'])}'),
        ]),
        const SizedBox(height: 20),
        Text('Holatlar bo\'yicha',
            style: Theme.of(context).textTheme.titleMedium),
        const SizedBox(height: 8),
        Wrap(spacing: 8, runSpacing: 8, children: [
          for (final e in _statusTitle.entries)
            if (_int(byStatus[e.key]) > 0)
              Chip(label: Text('${e.value}: ${_int(byStatus[e.key])}')),
          if (byStatus.isEmpty) const Text('Hali buyurtma yo\'q'),
        ]),
        const SizedBox(height: 20),
        Text('Do\'konlar kesimida',
            style: Theme.of(context).textTheme.titleMedium),
        const SizedBox(height: 8),
        if (stores.isEmpty)
          const Text('Hali ma\'lumot yo\'q')
        else
          Card(
            child: SingleChildScrollView(
              scrollDirection: Axis.horizontal,
              child: DataTable(columns: const [
                DataColumn(label: Text('Do\'kon')),
                DataColumn(label: Text('Buyurtma'), numeric: true),
                DataColumn(label: Text('Yetkazilgan'), numeric: true),
                DataColumn(label: Text('Faol'), numeric: true),
                DataColumn(label: Text('Sotilgan'), numeric: true),
                DataColumn(label: Text('Kuryerda pul'), numeric: true),
                DataColumn(label: Text('Kuryerlar'), numeric: true),
              ], rows: [
                for (final s in stores)
                  DataRow(cells: [
                    DataCell(Text('${s['store_name'] ?? s['store_id']}')),
                    DataCell(Text('${_int(s['orders'])}')),
                    DataCell(Text('${_int(s['delivered'])}')),
                    DataCell(Text('${_int(s['active'])}')),
                    DataCell(Text(formatSum(_int(s['sold_tiyin'])))),
                    DataCell(Text(formatSum(_int(s['cash_held_tiyin'])))),
                    DataCell(Text('${_int(s['couriers'])}')),
                  ]),
              ]),
            ),
          ),
      ],
    );
  }

  Widget _tile(IconData icon, String label, String value) {
    return SizedBox(
      width: 230,
      child: Card(
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Row(children: [
            Icon(icon, size: 28),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(label, style: Theme.of(context).textTheme.bodySmall),
                    const SizedBox(height: 2),
                    Text(value,
                        style: const TextStyle(
                            fontWeight: FontWeight.w800, fontSize: 18)),
                  ]),
            ),
          ]),
        ),
      ),
    );
  }
}

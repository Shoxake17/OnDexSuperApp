import 'dart:async';

import 'package:flutter/material.dart';

import '../api.dart';

/// RS-6 API.
extension LiveOpsApi on AdminApi {
  Future<Map<String, dynamic>> restaurantsLive() async =>
      Map<String, dynamic>.from(await send('GET', '/admin/restaurants/live') as Map);
}

const _statusTitle = <String, String>{
  'created': 'Yangi',
  'accepted': 'Qabul qilingan',
  'preparing': 'Tayyorlanmoqda',
  'ready': 'Tayyor',
  'picked_up': 'Yo\'lda',
};

const _rsvTitle = <String, String>{
  'held': 'zaklad kutilmoqda',
  'confirmed': 'tasdiqlangan',
  'seated': 'keldi',
  'completed': 'yakunlangan',
  'no_show': 'kelmadi',
  'cancelled': 'bekor',
  'expired': 'to\'lanmadi',
};

/// "Jonli holat" (RS-6): barcha restoranlar — ochiq/yopiq, faol buyurtmalar
/// holati, KECHIKAYOTGANLAR (sababi bilan), bugungi bronlar, ochiq stol
/// hisoblari. 10 soniyada yangilanadi; muammolilar tepada.
class LiveOpsPage extends StatefulWidget {
  const LiveOpsPage({super.key});

  @override
  State<LiveOpsPage> createState() => _LiveOpsPageState();
}

class _LiveOpsPageState extends State<LiveOpsPage> {
  Map<String, dynamic>? _data;
  String? _error;
  Timer? _timer;
  bool _onlyIssues = false;

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
      final d = await api.restaurantsLive();
      if (mounted) {
        setState(() {
          _data = d;
          _error = null;
        });
      }
    } on ApiException catch (e) {
      if (mounted) setState(() => _error = e.statusCode == 404 ? 'Server yangilanmagan (RS-6 yo\'q)' : e.message);
    } catch (e) {
      if (mounted) setState(() => _error = '$e');
    }
  }

  int _int(Object? v) => (v as num?)?.toInt() ?? 0;

  @override
  Widget build(BuildContext context) {
    final list = [
      for (final r in (_data?['restaurants'] as List? ?? const [])) Map<String, dynamic>.from(r as Map),
    ];
    final delayedTotal = list.fold<int>(0, (s, r) => s + (r['delayed'] as List? ?? const []).length);
    final activeTotal = list.fold<int>(0, (s, r) => s + _int(r['active_total']));
    final shown = _onlyIssues ? list.where((r) => (r['delayed'] as List? ?? const []).isNotEmpty).toList() : list;
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        Row(children: [
          const Text('Jonli holat', style: TextStyle(fontSize: 24, fontWeight: FontWeight.w800)),
          const SizedBox(width: 16),
          Chip(label: Text('$activeTotal ta faol buyurtma')),
          const SizedBox(width: 8),
          Chip(
            backgroundColor: delayedTotal > 0 ? Colors.red.shade50 : null,
            label: Text('$delayedTotal ta kechikmoqda',
                style: TextStyle(color: delayedTotal > 0 ? Colors.red.shade700 : null, fontWeight: FontWeight.w700)),
          ),
          const Spacer(),
          FilterChip(
            key: const ValueKey('live-only-issues'),
            label: const Text('Faqat muammolilar'),
            selected: _onlyIssues,
            onSelected: (v) => setState(() => _onlyIssues = v),
          ),
          IconButton(tooltip: 'Yangilash', onPressed: _load, icon: const Icon(Icons.refresh)),
        ]),
        if (_error != null) Padding(padding: const EdgeInsets.only(top: 8), child: Text(_error!, style: const TextStyle(color: Colors.red))),
        const SizedBox(height: 12),
        Expanded(
          child: _data == null && _error == null
              ? const Center(child: CircularProgressIndicator())
              : ListView.separated(
                  itemCount: shown.length,
                  separatorBuilder: (_, __) => const SizedBox(height: 10),
                  itemBuilder: (_, i) => _card(shown[i]),
                ),
        ),
      ]),
    );
  }

  Widget _card(Map<String, dynamic> r) {
    final delayed = [for (final d in (r['delayed'] as List? ?? const [])) Map<String, dynamic>.from(d as Map)];
    final active = Map<String, dynamic>.from((r['active'] as Map?) ?? {});
    final rsv = Map<String, dynamic>.from((r['reservations'] as Map?) ?? {});
    final open = r['open_now'] == true;
    return Card(
      margin: EdgeInsets.zero,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: BorderSide(color: delayed.isNotEmpty ? Colors.red.shade300 : Colors.black12),
      ),
      child: Padding(
        padding: const EdgeInsets.all(14),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Row(children: [
            Icon(Icons.circle, size: 10, color: open ? Colors.green : Colors.grey),
            const SizedBox(width: 8),
            Expanded(child: Text('${r['name']}', style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w700))),
            Text(open ? 'Ochiq' : 'Yopiq', style: TextStyle(color: open ? Colors.green : Colors.grey)),
          ]),
          const SizedBox(height: 8),
          Wrap(spacing: 8, runSpacing: 6, children: [
            for (final e in _statusTitle.entries)
              if (_int(active[e.key]) > 0) Chip(visualDensity: VisualDensity.compact, label: Text('${e.value}: ${_int(active[e.key])}')),
            if (rsv.isNotEmpty)
              Chip(
                visualDensity: VisualDensity.compact,
                avatar: const Icon(Icons.event_seat, size: 16),
                label: Text('Bronlar: ${rsv.entries.map((e) => '${_rsvTitle[e.key] ?? e.key} ${e.value}').join(', ')}'),
              ),
            if (_int(r['open_bills']) > 0)
              Chip(
                visualDensity: VisualDensity.compact,
                avatar: const Icon(Icons.receipt_long, size: 16),
                label: Text('Ochiq hisob: ${_int(r['open_bills'])} · ${formatSum(_int(r['bills_due_tiyin']))}'),
              ),
          ]),
          for (final d in delayed)
            Padding(
              padding: const EdgeInsets.only(top: 6),
              child: Row(children: [
                Icon(Icons.warning_amber_rounded, size: 18, color: Colors.red.shade700),
                const SizedBox(width: 6),
                Expanded(
                  child: Text(
                    '#${'${d['order_number']}'.split('-').last} · ${d['type'] == 'dine_in' ? 'stol' : 'yetkazish'} · '
                    '${d['reason']} (${d['minutes']} daq)',
                    style: TextStyle(color: Colors.red.shade700),
                  ),
                ),
              ]),
            ),
        ]),
      ),
    );
  }
}

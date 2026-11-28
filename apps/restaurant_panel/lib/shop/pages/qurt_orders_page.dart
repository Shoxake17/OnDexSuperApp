import 'dart:async';

import 'package:flutter/material.dart';

import '../../api.dart';
import '../../theme.dart';
import '../shop_api.dart';
import '../shop_widgets.dart';
import '../store_pages.dart';

/// Qurt buyurtmasi holati: (yorliq, rang) — ShoxPos `Orders.tsx` bilan bir xil.
(String, Color) qurtStatusStyle(String s) => switch (s) {
      'new' => ('Yangi', const Color(0xFFF59E0B)),
      'confirmed' => ('Qabul qilindi', const Color(0xFF3B82F6)),
      'rejected' => ('Rad etildi', const Color(0xFF94A3B8)),
      'packed' => ('Qadoqlandi', OnDexColors.primary),
      'collected' => ('Logistika oldi', const Color(0xFF06B6D4)),
      'out_for_delivery' => ('Yo\'lda', const Color(0xFFF97316)),
      'delivered' => ('Yetkazildi', const Color(0xFF10B981)),
      'failed' => ('Yetkazib bo\'lmadi', const Color(0xFFEF4444)),
      'cancelled' => ('Bekor qilindi', const Color(0xFF94A3B8)),
      _ => (s, OnDexColors.inkDim),
    };

const _months = ['yanvar', 'fevral', 'mart', 'aprel', 'may', 'iyun', 'iyul', 'avgust', 'sentabr', 'oktabr', 'noyabr', 'dekabr'];
const _weekdays = ['dushanba', 'seshanba', 'chorshanba', 'payshanba', 'juma', 'shanba', 'yakshanba'];

/// "2026-11-30" -> "30-noyabr, yakshanba".
String deliveryDateLabel(String ymd) {
  final d = DateTime.tryParse(ymd);
  if (d == null) return ymd;
  return '${d.day}-${_months[d.month - 1]}, ${_weekdays[d.weekday - 1]}';
}

enum _Tab { fresh, active, done, closed }

const _tabStatuses = {
  _Tab.fresh: ['new'],
  _Tab.active: ['confirmed', 'packed', 'collected', 'out_for_delivery', 'failed'],
  _Tab.done: ['delivered'],
  _Tab.closed: ['rejected', 'cancelled'],
};
const _tabLabel = {_Tab.fresh: 'Yangi', _Tab.active: 'Jarayonda', _Tab.done: 'Yetkazilgan', _Tab.closed: 'Yopilgan'};

int _pieces(Map<String, dynamic> o) => (o['items'] as List? ?? const []).fold(0, (s, it) => s + intOf((it as Map)['qty']));

/// "Buyurtmalar" (qurt) — ShoxPos `Orders.tsx`.
///
/// 1:1: kartalar (Yangi, Jarayonda, Tovar (dona), Umumiy summa — rad/bekor
/// qilinganlarsiz); bo'limlar Yangi / Jarayonda / Yetkazilgan / Yopilgan;
/// qidiruv (raqam, mijoz ismi, tovar nomi); Yangi — eskisi birinchi, qolgani
/// — yangisi birinchi. Amallar: Yangi → Qabul qilish (ombordan ayiriladi,
/// yetmasa xabar) yoki Rad etish (sabab 3..200 belgi); Qabul qilingan →
/// Qadoqlandi. Ro'yxat har 20 soniyada yangilanadi.
class QurtOrdersPage extends StatefulWidget {
  const QurtOrdersPage({super.key, required this.ctx});
  final StoreContext ctx;

  @override
  State<QurtOrdersPage> createState() => _QurtOrdersPageState();
}

class _QurtOrdersPageState extends State<QurtOrdersPage> {
  List<Map<String, dynamic>> _orders = [];
  bool _loading = true;
  _Tab _tab = _Tab.fresh;
  String _q = '';
  String? _selected;
  String? _busy;
  (bool, String)? _notice;
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    _load();
    _timer = Timer.periodic(const Duration(seconds: 20), (_) => _load(quiet: true));
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  Future<void> _load({bool quiet = false}) async {
    try {
      final o = await api.qurtOrders();
      if (mounted) setState(() => _orders = o);
    } catch (e) {
      if (mounted && !quiet) setState(() => _notice = (false, errText(e)));
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  int _count(_Tab t) => _orders.where((o) => _tabStatuses[t]!.contains(o['status'])).length;

  List<Map<String, dynamic>> get _visible {
    final q = _q.trim().toLowerCase();
    final l = _orders.where((o) {
      if (!_tabStatuses[_tab]!.contains(o['status'])) return false;
      if (q.isEmpty) return true;
      return ((o['number'] as String?) ?? '').toLowerCase().contains(q) ||
          ((o['customer_name'] as String?) ?? '').toLowerCase().contains(q) ||
          (o['items'] as List? ?? const []).any((it) => (((it as Map)['name'] as String?) ?? '').toLowerCase().contains(q));
    }).toList()
      ..sort((a, b) {
        final c = ((a['created_at'] as String?) ?? '').compareTo((b['created_at'] as String?) ?? '');
        return _tab == _Tab.fresh ? c : -c;
      });
    return l;
  }

  Future<void> _act(Map<String, dynamic> o, String to, {String reason = ''}) async {
    setState(() {
      _busy = o['id'] as String;
      _notice = null;
    });
    try {
      final u = await api.send('POST', '/merchant/qurt/orders/${Uri.encodeComponent(o['id'] as String)}/transition',
          {'to': to, 'reason': reason});
      final updated = Map<String, dynamic>.from(u as Map);
      if (!mounted) return;
      setState(() {
        _orders = [for (final x in _orders) x['id'] == updated['id'] ? updated : x];
        _notice = (
          true,
          switch (to) {
            'confirmed' => '${o['number']} qabul qilindi — tovar ombordan ayirildi',
            'packed' => '${o['number']} qadoqlandi — logistika oladi',
            _ => '${o['number']} rad etildi',
          }
        );
      });
    } catch (e) {
      if (mounted) setState(() => _notice = (false, errText(e)));
      await _load(quiet: true);
    } finally {
      if (mounted) setState(() => _busy = null);
    }
  }

  Future<void> _reject(Map<String, dynamic> o) async {
    final reason = await showDialog<String>(context: context, builder: (_) => const _RejectDialog());
    if (reason != null) await _act(o, 'rejected', reason: reason);
  }

  @override
  Widget build(BuildContext context) {
    final wide = MediaQuery.sizeOf(context).width >= 1100;
    final alive = _orders.where((o) => o['status'] != 'rejected' && o['status'] != 'cancelled');
    final sum = alive.fold<int>(0, (s, o) => s + intOf(o['total_tiyin']));
    final pieces = alive.fold<int>(0, (s, o) => s + _pieces(o));
    final visible = _visible;
    if (wide && (_selected == null || !visible.any((o) => o['id'] == _selected))) {
      _selected = visible.isEmpty ? null : visible.first['id'] as String;
    }
    final selected = _orders.where((o) => o['id'] == _selected).firstOrNull;

    final list = Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
      if (visible.isEmpty)
        ShopCard(
            child: EmptyState(
                loading: _loading,
                text: _q.isNotEmpty
                    ? 'Hech narsa topilmadi'
                    : _tab == _Tab.fresh
                        ? 'Yangi buyurtma yo\'q'
                        : 'Bu bo\'limda buyurtma yo\'q'))
      else
        for (final o in visible)
          Padding(
            padding: const EdgeInsets.only(bottom: 8),
            child: _OrderRow(
              order: o,
              selected: wide && o['id'] == _selected,
              onTap: () {
                setState(() => _selected = o['id'] as String);
                if (!wide) {
                  showDialog<void>(
                    context: context,
                    builder: (c) => Dialog(
                      insetPadding: const EdgeInsets.all(12),
                      child: ConstrainedBox(
                        constraints: const BoxConstraints(maxWidth: 560, maxHeight: 720),
                        child: StatefulBuilder(
                          builder: (c, set) => _OrderDetail(
                            order: _orders.firstWhere((x) => x['id'] == o['id'], orElse: () => o),
                            busy: _busy == o['id'],
                            onAct: (to) async {
                              Navigator.pop(c);
                              if (to == 'rejected') {
                                await _reject(o);
                              } else {
                                await _act(o, to);
                              }
                            },
                          ),
                        ),
                      ),
                    ),
                  );
                }
              },
            ),
          ),
    ]);

    return ShopPage(children: [
      StatGrid(tiles: [
        StatTile(label: 'Yangi', value: '${_count(_Tab.fresh)}', icon: Icons.inbox_rounded, color: const Color(0xFFF59E0B)),
        StatTile(label: 'Jarayonda', value: '${_count(_Tab.active)}', icon: Icons.hourglass_top_rounded, color: const Color(0xFF3B82F6)),
        StatTile(label: 'Tovar (dona)', value: '$pieces', icon: Icons.inventory_2_rounded, color: OnDexColors.primary),
        StatTile(label: 'Umumiy summa', value: somFromTiyin(sum), icon: Icons.paid_rounded, color: OnDexColors.success),
      ]),
      const SizedBox(height: 14),
      if (_notice != null)
        Container(
          key: const ValueKey('orders-notice'),
          margin: const EdgeInsets.only(bottom: 12),
          padding: const EdgeInsets.all(12),
          decoration: BoxDecoration(
            color: _notice!.$1 ? OnDexColors.successBg : OnDexColors.dangerBg,
            borderRadius: BorderRadius.circular(12),
          ),
          child: Text(_notice!.$2,
              style: TextStyle(fontWeight: FontWeight.w600, color: _notice!.$1 ? OnDexColors.success : OnDexColors.danger)),
        ),
      Wrap(spacing: 10, runSpacing: 10, crossAxisAlignment: WrapCrossAlignment.center, children: [
        SegmentedButton<_Tab>(
          showSelectedIcon: false,
          segments: [
            for (final t in _Tab.values) ButtonSegment(value: t, label: Text('${_tabLabel[t]} ${_count(t)}')),
          ],
          selected: {_tab},
          onSelectionChanged: (s) => setState(() => _tab = s.first),
        ),
        SizedBox(
            width: 300,
            child: SearchBox(hint: 'Raqam, mijoz yoki tovar...', onChanged: (v) => setState(() => _q = v))),
        IconButton(tooltip: 'Yangilash', onPressed: _load, icon: const Icon(Icons.refresh_rounded)),
      ]),
      const SizedBox(height: 14),
      if (wide)
        Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
          SizedBox(width: 420, child: list),
          const SizedBox(width: 16),
          Expanded(
            child: selected == null
                ? const SizedBox.shrink()
                : ShopCard(
                    padding: EdgeInsets.zero,
                    child: _OrderDetail(
                      order: selected,
                      busy: _busy == selected['id'],
                      onAct: (to) => to == 'rejected' ? _reject(selected) : _act(selected, to),
                    ),
                  ),
          ),
        ])
      else
        list,
    ]);
  }
}

class _OrderRow extends StatelessWidget {
  const _OrderRow({required this.order, required this.selected, required this.onTap});
  final Map<String, dynamic> order;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final (label, color) = qurtStatusStyle(order['status'] as String? ?? '');
    return Material(
      color: selected ? OnDexColors.primaryTint : OnDexColors.cardBg,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(14),
        side: BorderSide(color: selected ? OnDexColors.primary : OnDexColors.cardBorder),
      ),
      child: InkWell(
        key: ValueKey('order-${order['id']}'),
        borderRadius: BorderRadius.circular(14),
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Row(children: [
            Expanded(
              child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                Text('${order['number']}', style: const TextStyle(fontWeight: FontWeight.w800)),
                Text(
                    '${order['customer_name'] ?? ''} · ${_pieces(order)} dona · ${deliveryDateLabel((order['delivery_date'] as String?) ?? '')}',
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(fontSize: 12, color: OnDexColors.inkDim)),
              ]),
            ),
            Column(crossAxisAlignment: CrossAxisAlignment.end, children: [
              Text(somFromTiyin(intOf(order['total_tiyin'])), style: const TextStyle(fontWeight: FontWeight.w800)),
              const SizedBox(height: 4),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 2),
                decoration: BoxDecoration(color: color.withValues(alpha: .14), borderRadius: BorderRadius.circular(999)),
                child: Text(label, style: TextStyle(fontSize: 11.5, fontWeight: FontWeight.w700, color: color)),
              ),
            ]),
          ]),
        ),
      ),
    );
  }
}

class _OrderDetail extends StatelessWidget {
  const _OrderDetail({required this.order, required this.busy, required this.onAct});
  final Map<String, dynamic> order;
  final bool busy;
  final ValueChanged<String> onAct;

  @override
  Widget build(BuildContext context) {
    final o = order;
    final status = o['status'] as String? ?? '';
    final (label, color) = qurtStatusStyle(status);
    final items = (o['items'] as List? ?? const []).cast<Map>();
    final history = (o['history'] as List? ?? const []).cast<Map>();
    final reason = (o['reject_reason'] as String?) ?? '';
    String hm(Object? iso) {
      final d = DateTime.tryParse((iso as String?) ?? '')?.toLocal();
      if (d == null) return '';
      String two(int v) => v.toString().padLeft(2, '0');
      return '${two(d.day)}.${two(d.month)} ${two(d.hour)}:${two(d.minute)}';
    }

    return SingleChildScrollView(
      padding: const EdgeInsets.all(16),
      child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        Row(children: [
          Expanded(child: Text('${o['number']}', style: const TextStyle(fontWeight: FontWeight.w900, fontSize: 18))),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
            decoration: BoxDecoration(color: color.withValues(alpha: .14), borderRadius: BorderRadius.circular(999)),
            child: Text(label, style: TextStyle(fontWeight: FontWeight.w700, color: color)),
          ),
        ]),
        const SizedBox(height: 4),
        Text('Mijoz: ${o['customer_name'] ?? '—'} · Reys: ${deliveryDateLabel((o['delivery_date'] as String?) ?? '')}'
            ' · To\'lov: ${(o['payment_method'] == 'cash') ? 'naqd' : (o['payment_method'] ?? '')}',
            style: const TextStyle(color: OnDexColors.inkDim, fontSize: 12.5)),
        const SizedBox(height: 14),
        Text('TOVARLAR (${items.length})',
            style: const TextStyle(fontSize: 11, fontWeight: FontWeight.w700, color: OnDexColors.inkDim, letterSpacing: .4)),
        const SizedBox(height: 8),
        ShopCard(
          padding: EdgeInsets.zero,
          child: Column(children: [
            for (final it in items)
              ListTile(
                leading: Container(
                  width: 52,
                  height: 52,
                  decoration: BoxDecoration(color: OnDexColors.pageBg, borderRadius: BorderRadius.circular(10)),
                  clipBehavior: Clip.antiAlias,
                  child: ((it['image_url'] as String?) ?? '').isEmpty
                      ? const Icon(Icons.image_not_supported_outlined, color: OnDexColors.inkFaint)
                      : Image.network(fullImageUrl(it['image_url'] as String),
                          fit: BoxFit.contain, errorBuilder: (_, __, ___) => const Icon(Icons.broken_image_outlined)),
                ),
                title: Text('${it['name']}', maxLines: 1, overflow: TextOverflow.ellipsis),
                subtitle: Text(
                    '${somFromTiyin(intOf(it['price_tiyin']))}${intOf(it['weight_grams']) > 0 ? ' · ${it['weight_grams']} g' : ''}'),
                trailing: Text('× ${it['qty']}   ${somFromTiyin(intOf(it['qty']) * intOf(it['price_tiyin']))}',
                    style: const TextStyle(fontWeight: FontWeight.w800)),
              ),
            Container(
              color: OnDexColors.pageBg,
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
              child: Row(children: [
                Text('Jami (${_pieces(o)} dona)', style: const TextStyle(color: OnDexColors.inkDim, fontWeight: FontWeight.w600)),
                const Spacer(),
                Text(somFromTiyin(intOf(o['total_tiyin'])),
                    style: const TextStyle(fontWeight: FontWeight.w900, fontSize: 18, color: OnDexColors.primary)),
              ]),
            ),
          ]),
        ),
        if (reason.isNotEmpty && (status == 'rejected' || status == 'cancelled' || status == 'failed'))
          Container(
            margin: const EdgeInsets.only(top: 12),
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(color: OnDexColors.dangerBg, borderRadius: BorderRadius.circular(12)),
            child: Text('Sabab: $reason', style: const TextStyle(color: OnDexColors.danger, fontWeight: FontWeight.w600)),
          ),
        if (history.isNotEmpty) ...[
          const SizedBox(height: 14),
          const Text('HOLATLAR TARIXI',
              style: TextStyle(fontSize: 11, fontWeight: FontWeight.w700, color: OnDexColors.inkDim, letterSpacing: .4)),
          const SizedBox(height: 8),
          for (final h in history)
            Padding(
              padding: const EdgeInsets.only(bottom: 6),
              child: Row(children: [
                Container(
                    width: 10,
                    height: 10,
                    decoration: BoxDecoration(color: qurtStatusStyle('${h['to']}').$2, shape: BoxShape.circle)),
                const SizedBox(width: 10),
                Text(qurtStatusStyle('${h['to']}').$1, style: const TextStyle(fontWeight: FontWeight.w700)),
                const Spacer(),
                Text(hm(h['at']), style: const TextStyle(fontSize: 12, color: OnDexColors.inkDim)),
              ]),
            ),
        ],
        const SizedBox(height: 16),
        if (status == 'new')
          Row(children: [
            Expanded(
              child: OutlinedButton.icon(
                key: const ValueKey('order-reject'),
                style: OutlinedButton.styleFrom(foregroundColor: OnDexColors.danger, padding: const EdgeInsets.all(14)),
                onPressed: busy ? null : () => onAct('rejected'),
                icon: const Icon(Icons.close_rounded),
                label: const Text('Rad etish'),
              ),
            ),
            const SizedBox(width: 10),
            Expanded(
              child: FilledButton.icon(
                key: const ValueKey('order-confirm'),
                style: FilledButton.styleFrom(backgroundColor: OnDexColors.success),
                onPressed: busy ? null : () => onAct('confirmed'),
                icon: const Icon(Icons.check_rounded),
                label: Text(busy ? 'Bajarilmoqda...' : 'Qabul qilish'),
              ),
            ),
          ])
        else if (status == 'confirmed')
          Row(children: [
            const Expanded(
                child: Text('Tovarni yig\'ib bo\'lgach "Qadoqlandi" ni bosing.',
                    style: TextStyle(fontSize: 12, color: OnDexColors.inkDim))),
            FilledButton.icon(
              key: const ValueKey('order-packed'),
              onPressed: busy ? null : () => onAct('packed'),
              icon: const Icon(Icons.inventory_rounded),
              label: Text(busy ? 'Bajarilmoqda...' : 'Qadoqlandi'),
            ),
          ])
        else
          Text(
            switch (status) {
              'packed' => 'Logistika reys kuni do\'kondan olib ketadi.',
              'collected' || 'out_for_delivery' => 'Buyurtma yo\'lda — do\'kondan amal talab qilinmaydi.',
              'delivered' => 'Buyurtma yetkazilgan.',
              _ => 'Bu buyurtma bo\'yicha amal yo\'q.',
            },
            style: const TextStyle(fontSize: 12.5, color: OnDexColors.inkDim),
          ),
      ]),
    );
  }
}

class _RejectDialog extends StatefulWidget {
  const _RejectDialog();

  @override
  State<_RejectDialog> createState() => _RejectDialogState();
}

class _RejectDialogState extends State<_RejectDialog> {
  final _c = TextEditingController();

  @override
  void dispose() {
    _c.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => AlertDialog(
        title: const Text('Buyurtmani rad etish'),
        content: TextField(
          key: const ValueKey('reject-reason'),
          controller: _c,
          autofocus: true,
          maxLength: 200,
          maxLines: 3,
          onChanged: (_) => setState(() {}),
          decoration: shopInput('Sabab (kamida 3 belgi) — mijozga ko\'rinadi'),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: const Text('Bekor')),
          FilledButton(
            key: const ValueKey('reject-submit'),
            style: FilledButton.styleFrom(backgroundColor: OnDexColors.danger),
            onPressed: _c.text.trim().length < 3 ? null : () => Navigator.pop(context, _c.text.trim()),
            child: const Text('Rad etish'),
          ),
        ],
      );
}

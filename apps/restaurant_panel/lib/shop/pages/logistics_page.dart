import 'package:flutter/material.dart';

import '../../api.dart';
import '../../theme.dart';
import '../shop_api.dart';
import '../shop_widgets.dart';
import '../store_pages.dart';
import 'qurt_orders_page.dart' show deliveryDateLabel, qurtStatusStyle;

/// "Logistika" (qurt) — ShoxPos `Logistics.tsx`: do'kon o'z kuryerlarini
/// boshqaradi.
///
/// 1:1: kartalar (Kuryerlar, Kuryersiz buyurtma, Yo'lda, Kuryerlarda pul);
/// "Reyslar" — faol buyurtmalar yetkazish sanasi bo'yicha guruh, butun reysni
/// yoki bitta buyurtmani kuryerga biriktirish, kuryerdan olish (faqat
/// qabul qilingan/qadoqlangan/olingan holatda); "Kuryerlar" — qo'shish (ism,
/// telefon), kuryerdagi naqd pulni qabul qilish (tasdiq bilan), olib tashlash.
class LogisticsPage extends StatefulWidget {
  const LogisticsPage({super.key, required this.ctx});
  final StoreContext ctx;

  @override
  State<LogisticsPage> createState() => _LogisticsPageState();
}

const _active = 'confirmed,packed,collected,out_for_delivery,failed';

class _LogisticsPageState extends State<LogisticsPage> {
  List<Map<String, dynamic>> _orders = [];
  List<Map<String, dynamic>> _couriers = [];
  bool _loading = true;
  bool _busy = false;
  bool _tripsView = true;
  (bool, String)? _notice;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      final r = await Future.wait([api.qurtOrders(status: _active), api.qurtCouriers()]);
      if (!mounted) return;
      setState(() {
        _orders = r[0];
        _couriers = r[1];
      });
    } catch (e) {
      if (mounted) setState(() => _notice = (false, errText(e)));
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _run(Future<String> Function() action) async {
    setState(() {
      _busy = true;
      _notice = null;
    });
    try {
      final text = await action();
      if (mounted) setState(() => _notice = (true, text));
      await _load();
    } catch (e) {
      if (mounted) setState(() => _notice = (false, errText(e)));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  String _courierName(String? id) =>
      id == null || id.isEmpty ? '' : (_couriers.where((c) => c['id'] == id).firstOrNull?['name'] as String? ?? 'Kuryer');

  int _held(Map<String, dynamic> c) => intOf((c['summary'] as Map?)?['held_tiyin']);
  int _heldOrders(Map<String, dynamic> c) => intOf((c['summary'] as Map?)?['held_orders']);

  Future<void> _assign(String title, {String? date, List<String>? orderIds}) async {
    if (_couriers.isEmpty) {
      setState(() => _notice = (false, 'Avval "Kuryerlar" bo\'limida kuryer qo\'shing'));
      return;
    }
    final picked = await showDialog<String>(
      context: context,
      builder: (c) => SimpleDialog(
        title: Text(title),
        children: [
          for (final k in _couriers)
            SimpleDialogOption(
              key: ValueKey('pick-${k['id']}'),
              onPressed: () => Navigator.pop(c, k['id'] as String),
              child: ListTile(
                leading: const Icon(Icons.delivery_dining_rounded),
                title: Text('${k['name']}'),
                subtitle: Text('${k['phone']}'),
              ),
            ),
        ],
      ),
    );
    if (picked == null) return;
    await _run(() async {
      final r = await api.qurtAssign(picked, date: date, orderIds: orderIds);
      final n = intOf(r['assigned']);
      return n > 0
          ? '$n ta buyurtma ${_courierName(picked)} ga biriktirildi'
          : 'Biriktiriladigan buyurtma yo\'q (hammasi biriktirilgan yoki holati mos emas)';
    });
  }

  Future<void> _addCourier() async {
    final r = await showDialog<(String, String)>(context: context, builder: (_) => const _CourierDialog());
    if (r == null) return;
    await _run(() async {
      await api.qurtAddCourier(r.$1, r.$2);
      return '${r.$1} kuryerlar ro\'yxatiga qo\'shildi';
    });
  }

  Future<void> _settle(Map<String, dynamic> k) async {
    if (!await confirmDialog(
        context,
        '${k['name']} sizga ${somFromTiyin(_held(k))} (${_heldOrders(k)} ta buyurtma) topshirdimi? '
        'Tasdiqlagach hisob yopiladi.',
        ok: 'Tasdiqlash',
        danger: false)) {
      return;
    }
    await _run(() async {
      final r = await api.qurtSettle(k['id'] as String);
      return '${somFromTiyin(intOf(r['total_tiyin']))} qabul qilindi (${r['orders']} ta buyurtma)';
    });
  }

  Future<void> _remove(Map<String, dynamic> k) async {
    if (!await confirmDialog(context, '${k['name']} kuryerlar ro\'yxatidan olib tashlansinmi? U ilovadan chiqariladi.',
        ok: 'Olib tashlash')) {
      return;
    }
    await _run(() async {
      await api.qurtRemoveCourier(k['id'] as String);
      return '${k['name']} olib tashlandi';
    });
  }

  @override
  Widget build(BuildContext context) {
    final unassigned = _orders.where((o) => ((o['courier_id'] as String?) ?? '').isEmpty).length;
    final onRoad = _orders.where((o) => o['status'] == 'out_for_delivery').length;
    final held = _couriers.fold<int>(0, (s, c) => s + _held(c));
    final byDate = <String, List<Map<String, dynamic>>>{};
    for (final o in _orders) {
      byDate.putIfAbsent((o['delivery_date'] as String?) ?? '', () => []).add(o);
    }
    final dates = byDate.keys.toList()..sort();

    return ShopPage(children: [
      StatGrid(tiles: [
        StatTile(label: 'Kuryerlar', value: '${_couriers.length}', icon: Icons.groups_rounded, color: OnDexColors.info),
        StatTile(
            label: 'Kuryersiz buyurtma',
            value: '$unassigned',
            icon: Icons.inventory_2_outlined,
            color: unassigned > 0 ? OnDexColors.amber : OnDexColors.success),
        StatTile(label: 'Yo\'lda', value: '$onRoad', icon: Icons.local_shipping_rounded, color: const Color(0xFFF97316)),
        StatTile(
            label: 'Kuryerlarda pul',
            value: somFromTiyin(held),
            icon: Icons.payments_rounded,
            color: held > 0 ? OnDexColors.amber : OnDexColors.success),
      ]),
      const SizedBox(height: 14),
      if (_notice != null)
        Container(
          key: const ValueKey('logistics-notice'),
          margin: const EdgeInsets.only(bottom: 12),
          padding: const EdgeInsets.all(12),
          decoration: BoxDecoration(
              color: _notice!.$1 ? OnDexColors.successBg : OnDexColors.dangerBg, borderRadius: BorderRadius.circular(12)),
          child: Text(_notice!.$2,
              style: TextStyle(fontWeight: FontWeight.w600, color: _notice!.$1 ? OnDexColors.success : OnDexColors.danger)),
        ),
      Row(children: [
        SegmentedButton<bool>(
          showSelectedIcon: false,
          segments: const [
            ButtonSegment(value: true, label: Text('Reyslar'), icon: Icon(Icons.route_rounded)),
            ButtonSegment(value: false, label: Text('Kuryerlar'), icon: Icon(Icons.delivery_dining_rounded)),
          ],
          selected: {_tripsView},
          onSelectionChanged: (s) => setState(() => _tripsView = s.first),
        ),
        const Spacer(),
        if (!_tripsView)
          FilledButton.icon(
            key: const ValueKey('courier-add'),
            onPressed: _busy ? null : _addCourier,
            icon: const Icon(Icons.person_add_alt_1_rounded, size: 18),
            label: const Text('Kuryer qo\'shish'),
          ),
        IconButton(tooltip: 'Yangilash', onPressed: _loading || _busy ? null : _load, icon: const Icon(Icons.refresh_rounded)),
      ]),
      const SizedBox(height: 14),
      if (_tripsView) ...[
        if (dates.isEmpty)
          ShopCard(child: EmptyState(loading: _loading, text: 'Faol buyurtma yo\'q'))
        else
          for (final d in dates) _trip(d, byDate[d]!),
      ] else ...[
        if (_couriers.isEmpty)
          ShopCard(child: EmptyState(loading: _loading, text: 'Hali kuryer qo\'shilmagan'))
        else
          for (final k in _couriers)
            Padding(
              padding: const EdgeInsets.only(bottom: 10),
              child: ShopCard(
                child: Row(children: [
                  const CircleAvatar(
                      backgroundColor: OnDexColors.primaryTint,
                      child: Icon(Icons.delivery_dining_rounded, color: OnDexColors.primaryPressed)),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                      Text('${k['name']}', style: const TextStyle(fontWeight: FontWeight.w800)),
                      Text('${k['phone']} · faol: ${intOf((k['summary'] as Map?)?['active_orders'])}',
                          style: const TextStyle(fontSize: 12, color: OnDexColors.inkDim)),
                    ]),
                  ),
                  Column(crossAxisAlignment: CrossAxisAlignment.end, children: [
                    Text(somFromTiyin(_held(k)),
                        style: TextStyle(
                            fontWeight: FontWeight.w800, color: _held(k) > 0 ? OnDexColors.amber : OnDexColors.ink)),
                    Text(_held(k) > 0 ? 'kuryerda · ${_heldOrders(k)} ta' : 'pul topshirilgan',
                        style: const TextStyle(fontSize: 11.5, color: OnDexColors.inkDim)),
                  ]),
                  const SizedBox(width: 12),
                  FilledButton(
                    key: ValueKey('settle-${k['id']}'),
                    onPressed: _busy || _held(k) == 0 ? null : () => _settle(k),
                    child: const Text('Pulni qabul qilish'),
                  ),
                  IconButton(
                    key: ValueKey('remove-${k['id']}'),
                    tooltip: 'Olib tashlash',
                    color: OnDexColors.danger,
                    onPressed: _busy ? null : () => _remove(k),
                    icon: const Icon(Icons.delete_outline_rounded),
                  ),
                ]),
              ),
            ),
      ],
    ]);
  }

  Widget _trip(String date, List<Map<String, dynamic>> list) {
    final sum = list.fold<int>(0, (s, o) => s + intOf(o['total_tiyin']));
    final free = list.where((o) => ((o['courier_id'] as String?) ?? '').isEmpty).length;
    return Padding(
      padding: const EdgeInsets.only(bottom: 14),
      child: ShopCard(
        child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          Row(children: [
            Expanded(
              child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                Text('Reys: ${deliveryDateLabel(date)}', style: const TextStyle(fontWeight: FontWeight.w800, fontSize: 15)),
                Text('${list.length} ta buyurtma · ${somFromTiyin(sum)} · kuryersiz: $free',
                    style: const TextStyle(fontSize: 12, color: OnDexColors.inkDim)),
              ]),
            ),
            FilledButton.icon(
              key: ValueKey('assign-trip-$date'),
              onPressed: _busy || free == 0 ? null : () => _assign('${deliveryDateLabel(date)} reysini biriktirish', date: date),
              icon: const Icon(Icons.assignment_ind_rounded, size: 18),
              label: const Text('Reysni biriktirish'),
            ),
          ]),
          const Divider(height: 20),
          for (final o in list)
            Builder(builder: (_) {
              final courier = (o['courier_id'] as String?) ?? '';
              final canTake = courier.isNotEmpty && ['confirmed', 'packed', 'collected'].contains(o['status']);
              final (label, color) = qurtStatusStyle(o['status'] as String? ?? '');
              return Padding(
                padding: const EdgeInsets.symmetric(vertical: 4),
                child: Row(children: [
                  Text('${o['number']}', style: const TextStyle(fontWeight: FontWeight.w700)),
                  const SizedBox(width: 8),
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
                    decoration: BoxDecoration(color: color.withValues(alpha: .14), borderRadius: BorderRadius.circular(999)),
                    child: Text(label, style: TextStyle(fontSize: 11.5, fontWeight: FontWeight.w700, color: color)),
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(courier.isEmpty ? 'Kuryer biriktirilmagan' : '🚚 ${_courierName(courier)}',
                        style: TextStyle(fontSize: 12.5, color: courier.isEmpty ? OnDexColors.amber : OnDexColors.inkDim)),
                  ),
                  Text(somFromTiyin(intOf(o['total_tiyin'])), style: const TextStyle(fontWeight: FontWeight.w700)),
                  const SizedBox(width: 8),
                  if (courier.isEmpty)
                    TextButton(
                      key: ValueKey('assign-${o['id']}'),
                      onPressed: _busy ? null : () => _assign('${o['number']} ni biriktirish', orderIds: [o['id'] as String]),
                      child: const Text('Biriktirish'),
                    )
                  else if (canTake)
                    TextButton(
                      key: ValueKey('unassign-${o['id']}'),
                      onPressed: _busy
                          ? null
                          : () => _run(() async {
                                await api.qurtUnassign(o['id'] as String);
                                return 'Kuryerdan olindi';
                              }),
                      child: const Text('Kuryerdan olish'),
                    ),
                ]),
              );
            }),
        ]),
      ),
    );
  }
}

class _CourierDialog extends StatefulWidget {
  const _CourierDialog();

  @override
  State<_CourierDialog> createState() => _CourierDialogState();
}

class _CourierDialogState extends State<_CourierDialog> {
  final _name = TextEditingController();
  final _phone = TextEditingController(text: '+998');
  String? _err;

  @override
  void dispose() {
    _name.dispose();
    _phone.dispose();
    super.dispose();
  }

  void _ok() {
    final name = _name.text.trim(), phone = _phone.text.trim().replaceAll(' ', '');
    if (name.isEmpty || name.length > 80) {
      setState(() => _err = 'Ismni kiriting (ko\'pi bilan 80 belgi)');
      return;
    }
    if (!RegExp(r'^\+?[0-9]{9,15}$').hasMatch(phone)) {
      setState(() => _err = 'Telefon raqami noto\'g\'ri');
      return;
    }
    Navigator.pop(context, (name, phone));
  }

  @override
  Widget build(BuildContext context) => AlertDialog(
        title: const Text('Kuryer qo\'shish'),
        content: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          const FieldLabel('Ism'),
          TextField(key: const ValueKey('courier-name'), controller: _name, decoration: shopInput('Kuryer ismi')),
          const SizedBox(height: 12),
          const FieldLabel('Telefon raqami', hint: 'Kuryer OnDex GO ilovasiga shu raqam bilan kiradi'),
          TextField(
              key: const ValueKey('courier-phone'),
              controller: _phone,
              keyboardType: TextInputType.phone,
              decoration: shopInput('+998...')),
          if (_err != null)
            Padding(
                padding: const EdgeInsets.only(top: 10),
                child: Text(_err!, style: const TextStyle(color: OnDexColors.danger))),
        ]),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: const Text('Bekor')),
          FilledButton(key: const ValueKey('courier-save'), onPressed: _ok, child: const Text('Qo\'shish')),
        ],
      );
}

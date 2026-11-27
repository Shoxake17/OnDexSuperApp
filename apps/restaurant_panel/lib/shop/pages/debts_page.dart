import 'package:flutter/material.dart';

import '../../api.dart';
import '../../theme.dart';
import '../shop_api.dart';
import '../shop_widgets.dart';
import '../store_pages.dart';

/// "Qarzlar" — ShoxPos `Qarzlar.tsx` (dorixonada `PharmQarzlar.tsx` ham shu).
///
/// 1:1: Umumiy qarz va Qarzdorlar soni; qo'shish formasi (ism, telefon "+"
/// bilan boshlanadi, summa > 0); ro'yxat (№, ism, telefon, summa, oxirgi
/// to'lov sanasi); "To'lov" oynasi — to'liq bo'lsa qarz ro'yxatdan chiqadi,
/// aks holda qoldiq ko'rsatiladi.
///
/// ShoxPos'dagi "Telegram orqali eslatma" tugmasi KO'CHIRILMADI: u chaqirgan
/// `/api/telegram/send-debt-reminder` backendda yo'q edi (doim xato berardi).
class DebtsPage extends StatefulWidget {
  const DebtsPage({super.key, required this.ctx});
  final StoreContext ctx;

  @override
  State<DebtsPage> createState() => _DebtsPageState();
}

class _DebtsPageState extends State<DebtsPage> {
  final _name = TextEditingController();
  final _phone = TextEditingController(text: '+');
  final _amount = TextEditingController();
  List<Map<String, dynamic>> _debts = [];
  bool _loading = true;
  bool _adding = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _name.dispose();
    _phone.dispose();
    _amount.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final d = await api.shopDebts();
      if (mounted) setState(() => _debts = d);
    } catch (e) {
      if (mounted) toast(context, errText(e), error: true);
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _add() async {
    final amount = tiyinFromSomText(_amount.text);
    if (_name.text.trim().isEmpty || !_phone.text.startsWith('+') || _phone.text.length < 2 || amount <= 0) {
      toast(context, 'Barcha maydonlarni to\'g\'ri to\'ldiring', error: true);
      return;
    }
    setState(() => _adding = true);
    try {
      final d = await api.shopAddDebt(_name.text.trim(), _phone.text.trim(), amount);
      if (!mounted) return;
      setState(() {
        _debts = [d, ..._debts];
        _name.clear();
        _phone.text = '+';
        _amount.clear();
      });
    } catch (e) {
      if (mounted) toast(context, errText(e), error: true);
    } finally {
      if (mounted) setState(() => _adding = false);
    }
  }

  Future<void> _pay(Map<String, dynamic> d) async {
    final paid = await showDialog<int>(context: context, builder: (_) => _PayDialog(debt: d));
    if (paid == null || !mounted) return;
    if (paid <= 0) {
      await _result(false, 'Noto\'g\'ri summa kiritildi');
      return;
    }
    try {
      final r = await api.shopPayDebt(d['id'] as String, paid);
      if (!mounted) return;
      if (r['fully_paid'] == true) {
        setState(() => _debts.removeWhere((x) => x['id'] == d['id']));
        await _result(true, 'Qarz to\'liq to\'landi!');
      } else {
        final nd = Map<String, dynamic>.from(r['debt'] as Map);
        setState(() => _debts = [for (final x in _debts) x['id'] == d['id'] ? nd : x]);
        await _result(true, 'To\'lov qabul qilindi. Qoldiq: ${somFromTiyin(intOf(nd['amount_tiyin']))}');
      }
    } catch (e) {
      if (mounted) await _result(false, errText(e));
    }
  }

  Future<void> _result(bool ok, String text) => showDialog<void>(
        context: context,
        builder: (c) => AlertDialog(
          icon: Icon(ok ? Icons.check_circle : Icons.error, color: ok ? OnDexColors.success : OnDexColors.danger, size: 40),
          title: Text(ok ? 'Muvaffaqiyatli!' : 'Xato'),
          content: Text(text, textAlign: TextAlign.center),
          actions: [FilledButton(onPressed: () => Navigator.pop(c), child: const Text('OK'))],
        ),
      );

  static String _date(Object? iso) {
    final d = DateTime.tryParse((iso as String?) ?? '')?.toLocal();
    if (d == null) return '—';
    String two(int v) => v.toString().padLeft(2, '0');
    return '${two(d.day)}.${two(d.month)}.${d.year}';
  }

  @override
  Widget build(BuildContext context) {
    final total = _debts.fold<int>(0, (s, d) => s + intOf(d['amount_tiyin']));
    final narrow = MediaQuery.sizeOf(context).width < 760;
    final form = ShopCard(
      child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        const Text('Qarz qo\'shish', style: TextStyle(fontWeight: FontWeight.w800, fontSize: 16)),
        const SizedBox(height: 14),
        const FieldLabel('Ism'),
        TextField(key: const ValueKey('debt-name'), controller: _name, decoration: shopInput('Qarzdor ismini kiriting')),
        const SizedBox(height: 12),
        const FieldLabel('Telefon raqam'),
        TextField(
          key: const ValueKey('debt-phone'),
          controller: _phone,
          keyboardType: TextInputType.phone,
          decoration: shopInput('+998...'),
          onChanged: (v) {
            if (!v.startsWith('+')) {
              _phone.value = TextEditingValue(
                  text: '+$v', selection: TextSelection.collapsed(offset: v.length + 1));
            }
          },
        ),
        const SizedBox(height: 12),
        const FieldLabel('Qarzdorlik summasi'),
        TextField(
            key: const ValueKey('debt-amount'),
            controller: _amount,
            inputFormatters: kNumberInput,
            decoration: shopInput('0', suffix: 'so\'m')),
        const SizedBox(height: 16),
        FilledButton.icon(
          key: const ValueKey('debt-add'),
          onPressed: _adding ? null : _add,
          icon: const Icon(Icons.add),
          label: const Text('Qo\'shish'),
        ),
      ]),
    );

    final list = ShopCard(
      padding: EdgeInsets.zero,
      child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        const Padding(
          padding: EdgeInsets.fromLTRB(16, 16, 16, 8),
          child: Text('Qarzdorlar ro\'yxati', style: TextStyle(fontWeight: FontWeight.w800, fontSize: 16)),
        ),
        if (_debts.isEmpty)
          EmptyState(loading: _loading, text: 'Hozircha qarzdorlar yo\'q')
        else if (narrow)
          for (final d in _debts)
            ListTile(
              title: Text((d['name'] as String?) ?? '', style: const TextStyle(fontWeight: FontWeight.w700)),
              subtitle: Text('${d['phone']} · ${_date(d['last_payment'])}'),
              trailing: Row(mainAxisSize: MainAxisSize.min, children: [
                Text(somFromTiyin(intOf(d['amount_tiyin'])),
                    style: const TextStyle(fontWeight: FontWeight.w800, color: OnDexColors.danger)),
                const SizedBox(width: 8),
                FilledButton.tonal(
                    key: ValueKey('pay-${d['id']}'), onPressed: () => _pay(d), child: const Text('To\'lov')),
              ]),
            )
        else
          SizedBox(
            width: double.infinity,
            child: DataTable(
              headingRowColor: WidgetStateProperty.all(OnDexColors.pageBg),
              headingTextStyle: const TextStyle(fontSize: 12.5, fontWeight: FontWeight.w700, color: OnDexColors.inkDim),
              columns: const [
                DataColumn(label: Text('№')),
                DataColumn(label: Text('ISM')),
                DataColumn(label: Text('TELEFON')),
                DataColumn(label: Text('SUMMA'), numeric: true),
                DataColumn(label: Text('SANA')),
                DataColumn(label: Text('AMAL')),
              ],
              rows: [
                for (var i = 0; i < _debts.length; i++)
                  DataRow(cells: [
                    DataCell(Text('${i + 1}')),
                    DataCell(Text((_debts[i]['name'] as String?) ?? '', style: const TextStyle(fontWeight: FontWeight.w600))),
                    DataCell(Text((_debts[i]['phone'] as String?) ?? '',
                        style: const TextStyle(fontFamily: 'monospace', color: OnDexColors.inkDim))),
                    DataCell(Text(somFromTiyin(intOf(_debts[i]['amount_tiyin'])),
                        style: const TextStyle(fontWeight: FontWeight.w800, color: OnDexColors.danger))),
                    DataCell(Text(_date(_debts[i]['last_payment']))),
                    DataCell(FilledButton.tonal(
                        key: ValueKey('pay-${_debts[i]['id']}'),
                        onPressed: () => _pay(_debts[i]),
                        child: const Text('To\'lov'))),
                  ]),
              ],
            ),
          ),
      ]),
    );

    return ShopPage(children: [
      StatGrid(tiles: [
        StatTile(label: 'Umumiy qarz', value: somFromTiyin(total), icon: Icons.credit_card_rounded, color: OnDexColors.danger),
        StatTile(label: 'Qarzdorlar soni', value: '${_debts.length}', icon: Icons.people_alt_rounded, color: OnDexColors.info),
      ]),
      const SizedBox(height: 16),
      if (narrow) ...[form, const SizedBox(height: 16), list]
      else
        Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
          SizedBox(width: 340, child: form),
          const SizedBox(width: 16),
          Expanded(child: list),
        ]),
    ]);
  }
}

class _PayDialog extends StatefulWidget {
  const _PayDialog({required this.debt});
  final Map<String, dynamic> debt;

  @override
  State<_PayDialog> createState() => _PayDialogState();
}

class _PayDialogState extends State<_PayDialog> {
  final _c = TextEditingController();

  @override
  void dispose() {
    _c.dispose();
    super.dispose();
  }

  void _ok() => Navigator.pop(context, tiyinFromSomText(_c.text));

  @override
  Widget build(BuildContext context) => AlertDialog(
        title: Text('${widget.debt['name']} — ${somFromTiyin(intOf(widget.debt['amount_tiyin']))}'),
        content: TextField(
          key: const ValueKey('pay-amount'),
          controller: _c,
          autofocus: true,
          inputFormatters: kNumberInput,
          decoration: shopInput('To\'lov summasini kiriting', suffix: 'so\'m'),
          onSubmitted: (_) => _ok(),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: const Text('Bekor')),
          FilledButton(
              key: const ValueKey('pay-confirm'),
              style: FilledButton.styleFrom(backgroundColor: OnDexColors.success),
              onPressed: _ok,
              child: const Text('Tasdiqlash')),
        ],
      );
}

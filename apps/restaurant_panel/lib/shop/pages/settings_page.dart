import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'package:printing/printing.dart';

import '../../api.dart';
import '../../printing/receipt_printer.dart';
import '../../screens/login_screen.dart';
import '../../theme.dart';
import '../shop_widgets.dart';
import '../store_pages.dart';

/// "Sozlamalar" — ShoxPos `Settings.tsx`: u yerda faqat "Profil" ishlardi
/// (ism, familiya, telefon). Bu yerda:
///  * Profil — ism va familiya (OnDex akkaunti, `POST /me`); telefon faqat
///    ko'rish uchun — u OnDex ID orqali o'zgaradi (kirish shu raqamga bog'liq);
///  * Chek printeri — kassadagi F8 uchun (shu kompyuterda saqlanadi);
///  * Chiqish.
class StoreSettingsPage extends StatefulWidget {
  const StoreSettingsPage({super.key, required this.ctx});
  final StoreContext ctx;

  @override
  State<StoreSettingsPage> createState() => _StoreSettingsPageState();
}

class _StoreSettingsPageState extends State<StoreSettingsPage> {
  final _first = TextEditingController();
  final _last = TextEditingController();
  String _phone = '';
  bool _saving = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _first.dispose();
    _last.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final me = await api.me();
      if (!mounted) return;
      setState(() {
        _first.text = (me['first_name'] as String?) ?? '';
        _last.text = (me['last_name'] as String?) ?? '';
        _phone = (me['phone'] as String?) ?? widget.ctx.phone;
      });
    } catch (e) {
      if (mounted) toast(context, errText(e), error: true);
    }
  }

  Future<void> _save() async {
    if (_first.text.trim().isEmpty) {
      toast(context, 'Ismni kiriting', error: true);
      return;
    }
    setState(() => _saving = true);
    try {
      await api.send('POST', '/me', {'first_name': _first.text.trim(), 'last_name': _last.text.trim()});
      if (mounted) toast(context, 'Profil saqlandi');
    } catch (e) {
      if (mounted) toast(context, errText(e), error: true);
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  Future<void> _pickPrinter() async {
    final p = await Printing.pickPrinter(context: context, title: 'Chek printerini tanlang');
    if (p != null) await ReceiptPrefs.setPrinter(p);
  }

  Future<void> _logout() async {
    if (!await confirmDialog(context, 'Hisobdan chiqasizmi?', ok: 'Chiqish')) return;
    try {
      await api.logout();
    } catch (_) {}
    await api.clearSession();
    if (!mounted) return;
    Navigator.of(context, rootNavigator: true).pushAndRemoveUntil(
      MaterialPageRoute(builder: (_) => const RestaurantLoginScreen()),
      (_) => false,
    );
  }

  String get _typeLabel => switch (widget.ctx.category) {
        'qurt' => 'Qurt do\'koni',
        'pharmacy' => 'Dorixona',
        _ => 'Do\'kon',
      };

  @override
  Widget build(BuildContext context) {
    const gap = SizedBox(height: 14);
    return ShopPage(maxWidth: 760, children: [
      ShopCard(
        padding: const EdgeInsets.all(20),
        child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          Row(children: [
            const Icon(Icons.person_rounded, color: OnDexColors.primary),
            const SizedBox(width: 10),
            const Text('Profil', style: TextStyle(fontWeight: FontWeight.w800, fontSize: 17)),
            const Spacer(),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
              decoration: BoxDecoration(color: OnDexColors.primaryTint, borderRadius: BorderRadius.circular(20)),
              child: Text(_typeLabel,
                  style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w700, color: OnDexColors.primaryPressed)),
            ),
          ]),
          const SizedBox(height: 16),
          Row(children: [
            Expanded(
              child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
                const FieldLabel('Ism'),
                TextField(key: const ValueKey('settings-first'), controller: _first, decoration: shopInput('Ism')),
              ]),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
                const FieldLabel('Familiya'),
                TextField(key: const ValueKey('settings-last'), controller: _last, decoration: shopInput('Familiya')),
              ]),
            ),
          ]),
          gap,
          const FieldLabel('Telefon raqam', hint: 'Kirish shu raqamga bog\'langan — OnDex ID orqali o\'zgaradi'),
          InputDecorator(
            decoration: shopInput('').copyWith(prefixIcon: const Icon(Icons.phone_rounded, size: 18)),
            child: Text(_phone.isEmpty ? '—' : _phone),
          ),
          const SizedBox(height: 18),
          Align(
            alignment: Alignment.centerRight,
            child: FilledButton.icon(
              key: const ValueKey('settings-save'),
              onPressed: _saving ? null : _save,
              icon: const Icon(Icons.save_rounded, size: 18),
              label: Text(_saving ? 'Saqlanmoqda...' : 'Saqlash'),
            ),
          ),
        ]),
      ),
      gap,
      if (!widget.ctx.isQurt)
        ShopCard(
          padding: const EdgeInsets.all(20),
          child: ValueListenableBuilder<String>(
            valueListenable: ReceiptPrefs.printerName,
            builder: (context, name, _) => Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
              const Row(children: [
                Icon(Icons.print_rounded, color: OnDexColors.primary),
                SizedBox(width: 10),
                Text('Chek printeri', style: TextStyle(fontWeight: FontWeight.w800, fontSize: 17)),
              ]),
              const SizedBox(height: 6),
              const Text('Savdo terminalida F8 — chek chiqarib naqd sotish. Shu kompyuterda saqlanadi.',
                  style: TextStyle(color: OnDexColors.inkDim, fontSize: 12.5)),
              const SizedBox(height: 14),
              Row(children: [
                Expanded(child: Text(name.isEmpty ? 'Tanlanmagan — tizim oynasi ochiladi' : name)),
                if (!kIsWeb) OutlinedButton(onPressed: _pickPrinter, child: const Text('Tanlash')),
              ]),
              const SizedBox(height: 12),
              ValueListenableBuilder<int>(
                valueListenable: ReceiptPrefs.widthMm,
                builder: (context, w, _) => Row(children: [
                  const Text('Lenta kengligi'),
                  const Spacer(),
                  SegmentedButton<int>(
                    showSelectedIcon: false,
                    segments: const [
                      ButtonSegment(value: 58, label: Text('58 mm')),
                      ButtonSegment(value: 80, label: Text('80 mm')),
                    ],
                    selected: {w},
                    onSelectionChanged: (s) => ReceiptPrefs.setInt(ReceiptPrefs.widthMm, s.first),
                  ),
                ]),
              ),
              const SizedBox(height: 12),
              Align(
                alignment: Alignment.centerRight,
                child: OutlinedButton.icon(
                  onPressed: () async {
                    try {
                      await ReceiptPrinter.printTest(restaurantName: widget.ctx.name);
                    } catch (e) {
                      if (context.mounted) toast(context, 'Chek chiqmadi: printerni tekshiring', error: true);
                    }
                  },
                  icon: const Icon(Icons.receipt_long_rounded, size: 18),
                  label: const Text('Sinov cheki'),
                ),
              ),
            ]),
          ),
        ),
      if (!widget.ctx.isQurt) gap,
      ShopCard(
        padding: const EdgeInsets.all(8),
        child: ListTile(
          key: const ValueKey('settings-logout'),
          leading: const Icon(Icons.logout_rounded, color: OnDexColors.danger),
          title: const Text('Chiqish', style: TextStyle(color: OnDexColors.danger, fontWeight: FontWeight.w700)),
          subtitle: const Text('Shu kompyuterdagi sessiyani yopish'),
          onTap: _logout,
        ),
      ),
    ]);
  }
}

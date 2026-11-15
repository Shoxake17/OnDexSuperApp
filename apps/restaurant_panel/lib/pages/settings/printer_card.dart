part of '../restaurant_settings_page.dart';

// ─── Chek printeri (RS-8) ─────────────────────────────────────────────────

/// Shu kompyuterdagi chek printeri: avtomatik chop, lenta kengligi,
/// nusxalar, (Windows) printer tanlash, sinov cheki, (web) kiosk yo'riqnomasi.
class _PrinterCard extends StatefulWidget {
  const _PrinterCard({required this.restaurantName});

  final String restaurantName;

  @override
  State<_PrinterCard> createState() => _PrinterCardState();
}

class _PrinterCardState extends State<_PrinterCard> {
  bool _testing = false;

  void _toast(String m) =>
      ScaffoldMessenger.maybeOf(context)?.showSnackBar(SnackBar(content: Text(m)));

  /// Avtomatik chop yoqilganda ekrandagi faol buyurtmalar "chop etilgan"
  /// deb belgilanadi — eski buyurtmalar to'dasi birdan chiqib ketmasin.
  Future<void> _setAuto(ValueNotifier<bool> which, bool v) async {
    final wasOff = !ReceiptPrefs.autoKitchen.value && !ReceiptPrefs.autoCustomer.value;
    if (v && wasOff) {
      try {
        final list = (await api.orders()).cast<Map<String, dynamic>>();
        await ReceiptPrefs.markPrinted(pendingAutoPrint(list).map((o) => '${o['id']}'));
      } catch (_) {
        _toast('Buyurtmalarni o\'qib bo\'lmadi — internetni tekshiring');
        return;
      }
    }
    await ReceiptPrefs.setBool(which, v);
  }

  Future<void> _pickPrinter() async {
    try {
      final p = await Printing.pickPrinter(context: context, title: 'Chek printerini tanlang');
      if (p != null) await ReceiptPrefs.setPrinter(p);
    } catch (e) {
      _toast('Printerlar ro\'yxatini olib bo\'lmadi: $e');
    }
  }

  Future<void> _test() async {
    setState(() => _testing = true);
    try {
      await ReceiptPrinter.printTest(restaurantName: widget.restaurantName);
    } catch (e) {
      _toast('Sinov cheki chiqmadi: $e');
    } finally {
      if (mounted) setState(() => _testing = false);
    }
  }

  Widget _switchRow({
    required ValueNotifier<bool> notifier,
    required IconData icon,
    required String title,
    required String caption,
    required Key key,
  }) =>
      ValueListenableBuilder<bool>(
        valueListenable: notifier,
        builder: (context, value, _) => _PrinterRow(
          icon: icon,
          title: title,
          caption: caption,
          trailing: Switch(
            key: key,
            value: value,
            activeTrackColor: OnDexColors.success,
            onChanged: (v) => _setAuto(notifier, v),
          ),
        ),
      );

  Widget _copies(ValueNotifier<int> which, String label) => ValueListenableBuilder<int>(
        valueListenable: which,
        builder: (context, n, _) => Row(mainAxisSize: MainAxisSize.min, children: [
          Text(label, style: const TextStyle(fontSize: 12.5, color: OnDexColors.inkDim)),
          const SizedBox(width: 6),
          DropdownButton<int>(
            value: n,
            underline: const SizedBox.shrink(),
            items: const [1, 2, 3]
                .map((v) => DropdownMenuItem(value: v, child: Text('$v nusxa')))
                .toList(),
            onChanged: (v) => v == null ? null : ReceiptPrefs.setInt(which, v),
          ),
        ]),
      );

  @override
  Widget build(BuildContext context) {
    return _SettingsCard(
      icon: Icons.print_outlined,
      title: 'Chek printeri',
      subtitle: 'Shu kompyuter uchun — printer ulangan kassada yoqing',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _switchRow(
            key: const ValueKey('print-auto-kitchen'),
            notifier: ReceiptPrefs.autoKitchen,
            icon: Icons.soup_kitchen_outlined,
            title: 'Oshxona cheki',
            caption: 'Buyurtma qabul qilinganda avtomatik chiqadi',
          ),
          _switchRow(
            key: const ValueKey('print-auto-customer'),
            notifier: ReceiptPrefs.autoCustomer,
            icon: Icons.receipt_long_outlined,
            title: 'Mijoz cheki',
            caption: 'Qabul qilinganda narxlar bilan chek ham chiqadi',
          ),
          const SizedBox(height: 8),
          Wrap(
            spacing: 16,
            runSpacing: 8,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              ValueListenableBuilder<int>(
                valueListenable: ReceiptPrefs.widthMm,
                builder: (context, w, _) => SegmentedButton<int>(
                  segments: const [
                    ButtonSegment(value: 58, label: Text('58 mm')),
                    ButtonSegment(value: 80, label: Text('80 mm')),
                  ],
                  selected: {w},
                  showSelectedIcon: false,
                  onSelectionChanged: (s) => ReceiptPrefs.setInt(ReceiptPrefs.widthMm, s.first),
                ),
              ),
              _copies(ReceiptPrefs.kitchenCopies, 'Oshxona:'),
              _copies(ReceiptPrefs.customerCopies, 'Mijoz:'),
            ],
          ),
          const SizedBox(height: 12),
          if (!kIsWeb)
            ValueListenableBuilder<String>(
              valueListenable: ReceiptPrefs.printerName,
              builder: (context, name, _) => _PrinterRow(
                icon: Icons.usb_rounded,
                title: name.isEmpty ? 'Printer tanlanmagan' : name,
                caption: name.isEmpty
                    ? 'Tanlansa chek oynasiz chiqadi, aks holda chop etish oynasi ochiladi'
                    : 'Chek shu printerga oynasiz chiqadi',
                trailing: Row(mainAxisSize: MainAxisSize.min, children: [
                  if (name.isNotEmpty)
                    IconButton(
                      tooltip: 'Tanlovni bekor qilish',
                      onPressed: () => ReceiptPrefs.setPrinter(null),
                      icon: const Icon(Icons.close_rounded, size: 18),
                    ),
                  OutlinedButton(onPressed: _pickPrinter, child: const Text('Tanlash')),
                ]),
              ),
            )
          else
            const _KioskHint(),
          const SizedBox(height: 12),
          Align(
            alignment: Alignment.centerLeft,
            child: FilledButton.icon(
              key: const ValueKey('print-test'),
              onPressed: _testing ? null : _test,
              icon: _testing
                  ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
                  : const Icon(Icons.print_rounded, size: 18),
              label: const Text('Sinov cheki'),
            ),
          ),
        ],
      ),
    );
  }
}

class _PrinterRow extends StatelessWidget {
  const _PrinterRow({required this.icon, required this.title, required this.caption, required this.trailing});

  final IconData icon;
  final String title;
  final String caption;
  final Widget trailing;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(children: [
        Container(
          width: 34,
          height: 34,
          decoration: BoxDecoration(color: OnDexColors.primaryTint, borderRadius: BorderRadius.circular(10)),
          child: Icon(icon, size: 18, color: OnDexColors.primary),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, mainAxisSize: MainAxisSize.min, children: [
            Text(title,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(fontSize: 13.5, fontWeight: FontWeight.w600, color: OnDexColors.ink)),
            Text(caption,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(fontSize: 11.5, color: OnDexColors.inkDim)),
          ]),
        ),
        const SizedBox(width: 8),
        trailing,
      ]),
    );
  }
}

/// Web: chek oynasiz chiqishi uchun kassa kompyuterida Chrome sozlamasi.
class _KioskHint extends StatelessWidget {
  const _KioskHint();

  static const _cmd =
      '"C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe" --kiosk-printing https://merchant.ondex.uz/r/';

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: OnDexColors.primaryTint,
        borderRadius: BorderRadius.circular(10),
      ),
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        const Text('Chek oynasiz chiqishi uchun (bir marta):',
            style: TextStyle(fontSize: 12.5, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
        const SizedBox(height: 6),
        const Text(
          '1. Printer drayverini o\'rnating va uni Windows\'da standart printer qiling '
          '(qog\'oz o\'lchami: 58 yoki 80 mm).\n'
          '2. Ish stolida Chrome yorlig\'ini yarating — "Obyekt" maydoniga quyidagini yozing.\n'
          '3. Panelni shu yorliqdan oching: chek chop etish oynasisiz chiqadi.',
          style: TextStyle(fontSize: 12, color: OnDexColors.inkDim, height: 1.4),
        ),
        const SizedBox(height: 8),
        Row(children: [
          const Expanded(
            child: SelectableText(_cmd,
                style: TextStyle(fontSize: 11.5, fontFamily: 'monospace', color: OnDexColors.ink)),
          ),
          IconButton(
            tooltip: 'Nusxa olish',
            icon: const Icon(Icons.copy_rounded, size: 18),
            onPressed: () {
              Clipboard.setData(const ClipboardData(text: _cmd));
              ScaffoldMessenger.maybeOf(context)
                  ?.showSnackBar(const SnackBar(content: Text('Nusxa olindi')));
            },
          ),
        ]),
      ]),
    );
  }
}

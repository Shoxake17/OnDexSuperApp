part of '../tables_page.dart';

// ─── Umumiy bloklar ───────────────────────────────────────────────────

class _KindIconTile extends StatelessWidget {
  const _KindIconTile({required this.kind, required this.status, this.size = 46});

  final TableKindInfo kind;
  final TableStatus status;
  final double size;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        color: status.background,
        borderRadius: BorderRadius.circular(size * 0.28),
      ),
      child: Icon(kind.icon, size: size * 0.52, color: status.color),
    );
  }
}

class _StatusPill extends StatelessWidget {
  const _StatusPill({required this.status});

  final TableStatus status;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(color: status.background, borderRadius: BorderRadius.circular(999)),
      child: Text(status.title,
          maxLines: 1,
          style: TextStyle(fontSize: 11.5, fontWeight: FontWeight.w700, color: status.color)),
    );
  }
}

/// Tor joyda (kartochka, ro'yxat qatori) holat yorlig'i sig'masa KESILMAYDI
/// va qatorni buzmaydi — biroz kichrayadi.
class _FitPill extends StatelessWidget {
  const _FitPill({required this.status});

  final TableStatus status;

  @override
  Widget build(BuildContext context) => ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 108),
        child: FittedBox(
          fit: BoxFit.scaleDown,
          alignment: Alignment.centerRight,
          child: _StatusPill(status: status),
        ),
      );
}

class _SectionCard extends StatelessWidget {
  const _SectionCard({required this.title, required this.child, this.subtitle, this.trailing});

  final String title;
  final String? subtitle;
  final Widget? trailing;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.fromLTRB(18, 16, 18, 18),
      decoration: BoxDecoration(
        color: OnDexColors.cardBg,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(title,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                        fontSize: 15.5, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
              ),
              if (trailing != null) trailing!,
            ],
          ),
          if (subtitle != null) ...[
            const SizedBox(height: 3),
            Text(subtitle!, style: const TextStyle(fontSize: 12.5, color: OnDexColors.inkDim)),
          ],
          const SizedBox(height: 14),
          child,
        ],
      ),
    );
  }
}

class _LinkButton extends StatelessWidget {
  const _LinkButton({required this.label, required this.onTap});

  final String label;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return TextButton(
      onPressed: onTap,
      style: TextButton.styleFrom(
        foregroundColor: OnDexColors.primary,
        padding: const EdgeInsets.symmetric(horizontal: 6),
        minimumSize: const Size(0, 30),
        tapTargetSize: MaterialTapTargetSize.shrinkWrap,
        visualDensity: VisualDensity.compact,
        textStyle: const TextStyle(fontSize: 12.5, fontWeight: FontWeight.w700),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(label),
          const SizedBox(width: 3),
          const Icon(Icons.arrow_forward_rounded, size: 15),
        ],
      ),
    );
  }
}

class _Hint extends StatelessWidget {
  const _Hint(this.text);

  final String text;

  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.symmetric(vertical: 6),
        child: Text(text, style: const TextStyle(fontSize: 13, color: OnDexColors.inkDim)),
      );
}

/// "Stol 4", "Kabina 2" — ro'yxatlarda (kartochkadagi yalang'och
/// raqamdan farqli) tur har doim yoziladi.
String _fullTitle(DiningTable t) {
  if (t.kind != 'table') return t.title;
  return t.label.toLowerCase().startsWith('stol') ? t.label : 'Stol ${t.label}';
}

// ─── QR kod ───────────────────────────────────────────────────────────

class _QrCard extends StatelessWidget {
  const _QrCard({
    required this.table,
    required this.multiZone,
    required this.onDownloadPdf,
  });

  final DiningTable? table;
  final bool multiZone;
  final VoidCallback? onDownloadPdf;

  @override
  Widget build(BuildContext context) {
    final t = table;
    return _SectionCard(
      title: 'QR kod yaratish',
      subtitle: 'Mijozlar stolga o\'tirgandan so\'ng QR kodni skanerlab buyurtma berishlari mumkin',
      child: t == null
          ? const _Hint('QR kodni ko\'rish uchun joy tanlang')
          : Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Container(
                  padding: const EdgeInsets.all(12),
                  decoration:
                      BoxDecoration(color: _panelTint, borderRadius: BorderRadius.circular(14)),
                  child: Row(
                    children: [
                      Container(
                        key: const ValueKey('qr-preview'),
                        width: 132,
                        height: 132,
                        padding: const EdgeInsets.all(8),
                        decoration: BoxDecoration(
                          color: Colors.white,
                          borderRadius: BorderRadius.circular(12),
                          border: Border.all(color: OnDexColors.cardBorder),
                        ),
                        child: t.qrLink == null
                            ? const Center(
                                child: Icon(Icons.qr_code_2_rounded,
                                    size: 48, color: OnDexColors.inkFaint))
                            : QrImageView(
                                data: t.qrLink!,
                                version: QrVersions.auto,
                                padding: EdgeInsets.zero,
                                backgroundColor: Colors.white,
                                semanticsLabel: 'QR kod: ${t.displayLabel}',
                              ),
                      ),
                      const SizedBox(width: 12),
                      const Expanded(
                        child: Column(
                          children: [
                            _Feature(icon: Icons.bolt_rounded, text: 'Tezkor buyurtma'),
                            SizedBox(height: 8),
                            _Feature(icon: Icons.menu_book_rounded, text: 'To\'g\'ridan-to\'g\'ri menyu'),
                            SizedBox(height: 8),
                            _Feature(icon: Icons.verified_user_outlined, text: 'Telegram orqali kirish'),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 12),
                Row(
                  children: [
                    Icon(t.kindInfo.icon, size: 17, color: OnDexColors.inkDim),
                    const SizedBox(width: 6),
                    Expanded(
                      child: Text(multiZone ? '${_fullTitle(t)} · ${t.zone}' : _fullTitle(t),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                              fontSize: 13.5, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
                    ),
                    const SizedBox(width: 6),
                    Tooltip(
                      message: _staticQrHint,
                      child: Container(
                        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                        decoration: BoxDecoration(
                            color: OnDexColors.successBg, borderRadius: BorderRadius.circular(999)),
                        child: const Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            Icon(Icons.lock_rounded, size: 12, color: OnDexColors.success),
                            SizedBox(width: 4),
                            Text('O\'zgarmas QR',
                                style: TextStyle(
                                    fontSize: 11, fontWeight: FontWeight.w700, color: OnDexColors.success)),
                          ],
                        ),
                      ),
                    ),
                  ],
                ),
                if (t.qrLink == null) ...[
                  const SizedBox(height: 8),
                  const Text(_noLinkText, style: TextStyle(fontSize: 12, color: OnDexColors.danger)),
                ],
                const SizedBox(height: 12),
                FilledButton.icon(
                  key: const ValueKey('qr-download'),
                  onPressed: t.qrLink == null ? null : onDownloadPdf,
                  style: FilledButton.styleFrom(
                    backgroundColor: OnDexColors.primary,
                    foregroundColor: Colors.white,
                    minimumSize: const Size.fromHeight(46),
                    shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
                    textStyle: const TextStyle(fontSize: 14, fontWeight: FontWeight.w700),
                  ),
                  icon: const Icon(Icons.download_rounded, size: 20),
                  label: const Text('QR kodni yuklab olish', maxLines: 1, overflow: TextOverflow.ellipsis),
                ),
              ],
            ),
    );
  }
}

class _Feature extends StatelessWidget {
  const _Feature({required this.icon, required this.text});

  final IconData icon;
  final String text;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 9),
      decoration: BoxDecoration(color: Colors.white, borderRadius: BorderRadius.circular(10)),
      child: Row(
        children: [
          Icon(icon, size: 17, color: OnDexColors.primary),
          const SizedBox(width: 8),
          Expanded(
            child: Text(text,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(fontSize: 12.5, fontWeight: FontWeight.w600, color: OnDexColors.ink)),
          ),
        ],
      ),
    );
  }
}

// ─── Holatlar ─────────────────────────────────────────────────────────

class _StatusCard extends StatelessWidget {
  const _StatusCard({
    required this.tables,
    required this.selected,
    required this.onStatus,
    required this.onDetails,
  });

  final List<DiningTable> tables;
  final TableStatus? selected;
  final ValueChanged<TableStatus?> onStatus;
  final VoidCallback onDetails;

  @override
  Widget build(BuildContext context) {
    int count(TableStatus s) => tables.where((t) => t.status == s).length;
    Widget tile(TableStatus s, String label) => Expanded(
          child: _StatTile(
            key: ValueKey('stat-${s.name}'),
            icon: s.icon,
            label: label,
            value: count(s),
            color: s.color,
            background: s.background,
            selected: selected == s,
            onTap: () => onStatus(s),
          ),
        );
    final inactive = count(TableStatus.inactive);
    return _SectionCard(
      title: 'Stollar holati',
      trailing: _LinkButton(label: 'Batafsil', onTap: onDetails),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              tile(TableStatus.occupied, 'Band'),
              const SizedBox(width: 8),
              tile(TableStatus.available, 'Bo\'sh'),
              const SizedBox(width: 8),
              tile(TableStatus.cleaning, 'Tozalanmoqda'),
              const SizedBox(width: 8),
              Expanded(
                child: _StatTile(
                  key: const ValueKey('stat-total'),
                  icon: Icons.grid_view_rounded,
                  label: 'Jami',
                  value: tables.length,
                  color: OnDexColors.primary,
                  background: OnDexColors.primaryTint,
                  selected: false,
                  onTap: () => onStatus(null),
                ),
              ),
            ],
          ),
          if (inactive > 0) ...[
            const SizedBox(height: 10),
            Text('Vaqtincha yopilgan: $inactive ta',
                style: const TextStyle(fontSize: 12, color: OnDexColors.inkDim)),
          ],
        ],
      ),
    );
  }
}

class _StatTile extends StatelessWidget {
  const _StatTile({
    super.key,
    required this.icon,
    required this.label,
    required this.value,
    required this.color,
    required this.background,
    required this.selected,
    required this.onTap,
  });

  final IconData icon;
  final String label;
  final int value;
  final Color color;
  final Color background;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: background,
      borderRadius: BorderRadius.circular(12),
      child: InkWell(
        borderRadius: BorderRadius.circular(12),
        onTap: onTap,
        child: Container(
          padding: const EdgeInsets.fromLTRB(10, 10, 8, 10),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(12),
            border: Border.all(color: selected ? color : Colors.transparent, width: 1.5),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              FittedBox(
                fit: BoxFit.scaleDown,
                alignment: Alignment.centerLeft,
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(icon, size: 14, color: color),
                    const SizedBox(width: 5),
                    Text(label, style: TextStyle(fontSize: 11.5, fontWeight: FontWeight.w700, color: color)),
                  ],
                ),
              ),
              const SizedBox(height: 8),
              FittedBox(
                fit: BoxFit.scaleDown,
                alignment: Alignment.centerLeft,
                child: Text('$value',
                    style: const TextStyle(
                        fontSize: 22, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

// ─── So'nggi skanerlashlar ────────────────────────────────────────────

List<DiningTable> _scannedTables(List<DiningTable> tables) =>
    tables.where((t) => t.lastScannedAt != null).toList()
      ..sort((a, b) => b.lastScannedAt!.compareTo(a.lastScannedAt!));

/// Skanerlashga tegishli buyurtma: joydagi faol buyurtma yoki skanerlash
/// atrofidagi (3 soat ichidagi) oxirgi buyurtma. Kechagi mehmonlarning
/// buyurtmasi bugungi skanerlashga yozib qo'yilmasin.
String _scanOrderText(DiningTable t) {
  final active = t.currentOrder;
  if (active != null) return '${active.items} ta mahsulot';
  final last = t.lastOrder;
  final at = last?.createdAt;
  final scan = t.lastScannedAt!;
  if (last == null || at == null || at.isBefore(scan.subtract(const Duration(hours: 3)))) {
    return 'Buyurtma berilmagan';
  }
  return '${last.items} ta mahsulot';
}

class _RecentScansCard extends StatelessWidget {
  const _RecentScansCard({
    required this.tables,
    required this.multiZone,
    required this.onOpen,
    required this.onShowAll,
  });

  final List<DiningTable> tables;
  final bool multiZone;
  final ValueChanged<DiningTable> onOpen;
  final VoidCallback onShowAll;

  static const _visible = 4;

  @override
  Widget build(BuildContext context) {
    final scanned = _scannedTables(tables);
    return _SectionCard(
      title: 'So\'nggi skanerlangan QR kodlar',
      trailing: scanned.isEmpty ? null : _LinkButton(label: 'Barchasini ko\'rish', onTap: onShowAll),
      child: scanned.isEmpty
          ? const _Hint('Hali QR kod skanerlanmagan. Mijoz joydagi QR kodni skanerlashi bilan '
              'shu yerda ko\'rinadi.')
          : Column(
              children: [
                for (var i = 0; i < scanned.length && i < _visible; i++) ...[
                  if (i > 0) Divider(height: 1, color: OnDexColors.cardBorder.withValues(alpha: 0.6)),
                  _ScanRow(table: scanned[i], multiZone: multiZone, onTap: () => onOpen(scanned[i])),
                ],
              ],
            ),
    );
  }
}

class _ScanRow extends StatelessWidget {
  const _ScanRow({required this.table, required this.multiZone, required this.onTap});

  final DiningTable table;
  final bool multiZone;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final t = table;
    return InkWell(
      borderRadius: BorderRadius.circular(10),
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 10),
        child: Row(
          children: [
            _KindIconTile(kind: t.kindInfo, status: t.status, size: 38),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(multiZone ? '${_fullTitle(t)} · ${t.zone}' : _fullTitle(t),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                          fontSize: 13.5, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
                  const SizedBox(height: 2),
                  Text('${_when(t.lastScannedAt!)} · ${_scanOrderText(t)}',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(fontSize: 12, color: OnDexColors.inkDim)),
                ],
              ),
            ),
            const SizedBox(width: 8),
            _FitPill(status: t.status),
            const Icon(Icons.chevron_right_rounded, size: 20, color: OnDexColors.inkFaint),
          ],
        ),
      ),
    );
  }
}

class _RecentScansDialog extends StatelessWidget {
  const _RecentScansDialog({required this.tables, required this.multiZone});

  final List<DiningTable> tables;
  final bool multiZone;

  @override
  Widget build(BuildContext context) {
    final scanned = _scannedTables(tables);
    return Dialog(
      backgroundColor: OnDexColors.cardBg,
      surfaceTintColor: Colors.transparent,
      insetPadding: const EdgeInsets.all(20),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(18)),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 520, maxHeight: 640),
        child: Padding(
          padding: const EdgeInsets.fromLTRB(20, 14, 12, 14),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Row(
                children: [
                  const Expanded(
                    child: Text('Skanerlangan QR kodlar',
                        style: TextStyle(fontSize: 18, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
                  ),
                  IconButton(
                    tooltip: 'Yopish',
                    onPressed: () => Navigator.of(context).pop(),
                    icon: const Icon(Icons.close_rounded),
                  ),
                ],
              ),
              Flexible(
                child: ListView(
                  shrinkWrap: true,
                  padding: const EdgeInsets.only(right: 8),
                  children: [
                    for (final t in scanned)
                      _ScanRow(table: t, multiZone: multiZone, onTap: () => Navigator.of(context).pop(t)),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _StatusBreakdownDialog extends StatelessWidget {
  const _StatusBreakdownDialog({required this.tables, required this.zones});

  final List<DiningTable> tables;
  final List<String> zones;

  static const _order = [
    TableStatus.occupied,
    TableStatus.available,
    TableStatus.cleaning,
    TableStatus.inactive,
  ];

  Widget _cell(String text, {bool head = false, bool strong = false}) => Padding(
        padding: const EdgeInsets.symmetric(vertical: 8, horizontal: 4),
        child: Text(text,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(
              fontSize: head ? 11.5 : 13,
              fontWeight: head || strong ? FontWeight.w700 : FontWeight.w500,
              color: head ? OnDexColors.inkFaint : OnDexColors.ink,
            )),
      );

  Widget _table(String heading, List<(String, List<DiningTable>)> groups) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(heading.toUpperCase(),
            style: const TextStyle(
                fontSize: 11, letterSpacing: 0.6, fontWeight: FontWeight.w800, color: OnDexColors.inkFaint)),
        const SizedBox(height: 6),
        Table(
          columnWidths: const {0: FlexColumnWidth(2.2)},
          children: [
            TableRow(
              decoration: const BoxDecoration(
                  border: Border(bottom: BorderSide(color: OnDexColors.cardBorder))),
              children: [
                _cell('', head: true),
                _cell('Jami', head: true),
                for (final s in _order) _cell(s.title, head: true),
              ],
            ),
            for (final (name, list) in groups)
              TableRow(
                children: [
                  _cell(name, strong: true),
                  _cell('${list.length}', strong: true),
                  for (final s in _order) _cell('${list.where((t) => t.status == s).length}'),
                ],
              ),
          ],
        ),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    final byKind = [
      for (final k in kTableKinds)
        if (tables.any((t) => t.kind == k.kind)) (k.title, tables.where((t) => t.kind == k.kind).toList()),
    ];
    final byZone = [
      for (final z in zones) (z, tables.where((t) => t.zone == z).toList()),
    ];
    return Dialog(
      backgroundColor: OnDexColors.cardBg,
      surfaceTintColor: Colors.transparent,
      insetPadding: const EdgeInsets.all(20),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(18)),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 600),
        child: SingleChildScrollView(
          padding: const EdgeInsets.fromLTRB(20, 14, 20, 20),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Row(
                children: [
                  const Expanded(
                    child: Text('Joylar holati — batafsil',
                        style: TextStyle(fontSize: 18, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
                  ),
                  IconButton(
                    tooltip: 'Yopish',
                    onPressed: () => Navigator.of(context).pop(),
                    icon: const Icon(Icons.close_rounded),
                  ),
                ],
              ),
              const SizedBox(height: 8),
              _table('Tur bo\'yicha', byKind),
              if (zones.length > 1) ...[
                const SizedBox(height: 18),
                _table('Zal bo\'yicha', byZone),
              ],
            ],
          ),
        ),
      ),
    );
  }
}

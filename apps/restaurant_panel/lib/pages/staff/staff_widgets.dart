part of '../staff_page.dart';

const _purple = Color(0xFF7C5CD6);
const _purpleBg = Color(0xFFEFEAFB);
const _rowDivider = Color(0xFFF1E7DC);
const _headBg = Color(0xFFFAF6F0);

BoxDecoration _cardBox() => BoxDecoration(
      color: OnDexColors.cardBg,
      borderRadius: BorderRadius.circular(16),
      border: Border.all(color: OnDexColors.cardBorder),
    );

// ─── Sarlavha ─────────────────────────────────────────────────────────

class _StaffHeader extends StatelessWidget {
  const _StaffHeader({required this.onAdd});

  final VoidCallback? onAdd;

  @override
  Widget build(BuildContext context) {
    const title = Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text('Xodimlar',
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(fontSize: 24, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
        SizedBox(height: 2),
        Text('Restorandagi xodimlar ro\'yxati va ularni boshqarish',
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(fontSize: 13.5, color: OnDexColors.inkDim)),
      ],
    );
    // Boshqa sahifalardagi asosiy tugma bilan bir xil (brend rangi).
    final add = FilledButton.icon(
      key: const ValueKey('staff-add'),
      onPressed: onAdd,
      style: FilledButton.styleFrom(
        backgroundColor: OnDexColors.primary,
        foregroundColor: Colors.white,
        minimumSize: const Size(0, 46),
        padding: const EdgeInsets.symmetric(horizontal: 18),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
        textStyle: const TextStyle(fontSize: 14, fontWeight: FontWeight.w700),
      ),
      icon: const Icon(Icons.add_rounded, size: 20),
      label: const Text('Xodim qo\'shish'),
    );
    return LayoutBuilder(builder: (context, box) {
      if (box.maxWidth >= 560) {
        return Row(children: [const Expanded(child: title), const SizedBox(width: 12), add]);
      }
      return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [title, const SizedBox(height: 12), add]);
    });
  }
}

// ─── Kartalar ─────────────────────────────────────────────────────────

class _StatsRow extends StatelessWidget {
  const _StatsRow({required this.summary});

  final StaffSummary summary;

  @override
  Widget build(BuildContext context) {
    final s = summary;
    final cards = [
      _StatCard(
        key: const ValueKey('stat-total'),
        icon: Icons.groups_rounded,
        color: OnDexColors.primary,
        bg: OnDexColors.primaryTint,
        label: 'Jami xodimlar',
        value: s.total,
      ),
      _StatCard(
        key: const ValueKey('stat-active'),
        icon: Icons.check_box_rounded,
        color: OnDexColors.success,
        bg: OnDexColors.successBg,
        label: 'Faol xodimlar',
        value: s.active,
      ),
      _StatCard(
        key: const ValueKey('stat-today'),
        icon: Icons.schedule_rounded,
        color: OnDexColors.info,
        bg: OnDexColors.infoBg,
        label: 'Bugungi ishchilar',
        value: s.workingToday,
      ),
      _StatCard(
        key: const ValueKey('stat-dismissed'),
        icon: Icons.exit_to_app_rounded,
        color: OnDexColors.danger,
        bg: OnDexColors.dangerBg,
        label: 'Ishdan bo\'shaganlar',
        value: s.dismissed,
      ),
    ];
    return LayoutBuilder(builder: (context, box) {
      const gap = 14.0;
      if (box.maxWidth >= 1000) {
        return Row(
          children: [
            for (var i = 0; i < cards.length; i++) ...[
              if (i > 0) const SizedBox(width: gap),
              Expanded(child: cards[i]),
            ],
          ],
        );
      }
      final cols = box.maxWidth >= 520 ? 2 : 1;
      final w = ((box.maxWidth - gap * (cols - 1)) / cols).floorToDouble();
      return Wrap(
        spacing: gap,
        runSpacing: gap,
        children: [for (final c in cards) SizedBox(width: w, child: c)],
      );
    });
  }
}

class _StatCard extends StatelessWidget {
  const _StatCard({
    super.key,
    required this.icon,
    required this.color,
    required this.bg,
    required this.label,
    required this.value,
  });

  final IconData icon;
  final Color color;
  final Color bg;
  final String label;
  final int value;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 16),
      decoration: _cardBox(),
      child: Row(
        children: [
          Container(
            width: 48,
            height: 48,
            decoration: BoxDecoration(color: bg, borderRadius: BorderRadius.circular(12)),
            child: Icon(icon, color: color, size: 25),
          ),
          const SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(label,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(fontSize: 13, color: OnDexColors.inkDim)),
                const SizedBox(height: 2),
                Text('$value',
                    style: const TextStyle(fontSize: 26, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

// ─── Kichik bo'laklar ─────────────────────────────────────────────────

class _Avatar extends StatelessWidget {
  const _Avatar({required this.member, this.size = 38});

  final StaffMember member;
  final double size;

  @override
  Widget build(BuildContext context) => _InitialsAvatar(id: member.id, name: member.fullName, size: size);
}

class _InitialsAvatar extends StatelessWidget {
  const _InitialsAvatar({required this.id, required this.name, this.size = 38});

  final String id;
  final String name;
  final double size;

  @override
  Widget build(BuildContext context) {
    final (fg, bg) = _avatarPalette[id.hashCode.abs() % _avatarPalette.length];
    return Container(
      width: size,
      height: size,
      alignment: Alignment.center,
      decoration: BoxDecoration(color: bg, shape: BoxShape.circle, border: Border.all(color: fg.withValues(alpha: 0.25))),
      child: Text(_initials(name), style: TextStyle(fontSize: size * 0.36, fontWeight: FontWeight.w800, color: fg)),
    );
  }
}

class _StatusPill extends StatelessWidget {
  const _StatusPill({required this.status});

  final String status;

  @override
  Widget build(BuildContext context) {
    final (label, icon, fg, bg) = switch (status) {
      'active' => ('Faol', Icons.arrow_upward_rounded, OnDexColors.success, OnDexColors.successBg),
      'on_leave' => ('Ta\'tilda', Icons.beach_access_rounded, OnDexColors.inkDim, const Color(0xFFEFEAE3)),
      'dismissed' => ('Ishdan bo\'shagan', Icons.block_rounded, OnDexColors.danger, OnDexColors.dangerBg),
      _ => (status, Icons.help_outline_rounded, OnDexColors.inkDim, OnDexColors.pageBg),
    };
    return ConstrainedBox(
      constraints: const BoxConstraints(maxWidth: 150),
      child: FittedBox(
        fit: BoxFit.scaleDown,
        alignment: Alignment.centerLeft,
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
          decoration: BoxDecoration(color: bg, borderRadius: BorderRadius.circular(999)),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(icon, size: 13, color: fg),
              const SizedBox(width: 4),
              Text(label, style: TextStyle(fontSize: 12, fontWeight: FontWeight.w700, color: fg)),
            ],
          ),
        ),
      ),
    );
  }
}

class _IconAction extends StatelessWidget {
  const _IconAction({super.key, required this.icon, required this.tooltip, required this.onTap});

  final IconData icon;
  final String tooltip;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: tooltip,
      child: Material(
        color: OnDexColors.cardBg,
        borderRadius: BorderRadius.circular(10),
        child: InkWell(
          borderRadius: BorderRadius.circular(10),
          onTap: onTap,
          child: Container(
            width: 34,
            height: 34,
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(10),
              border: Border.all(color: OnDexColors.cardBorder),
            ),
            child: Icon(icon, size: 17, color: OnDexColors.inkDim),
          ),
        ),
      ),
    );
  }
}

List<(String, String, IconData, bool)> _statusActions(String status) => switch (status) {
      'active' => [
          ('on_leave', 'Ta\'tilga chiqarish', Icons.beach_access_rounded, false),
          ('dismissed', 'Ishdan bo\'shatish', Icons.logout_rounded, true),
        ],
      'on_leave' => [
          ('active', 'Ishga qaytarish', Icons.how_to_reg_rounded, false),
          ('dismissed', 'Ishdan bo\'shatish', Icons.logout_rounded, true),
        ],
      _ => [('active', 'Qayta ishga olish', Icons.how_to_reg_rounded, false)],
    };

class _MoreMenu extends StatelessWidget {
  const _MoreMenu({super.key, required this.member, required this.onSelected});

  final StaffMember member;
  final ValueChanged<String> onSelected;

  @override
  Widget build(BuildContext context) {
    return PopupMenuButton<String>(
      tooltip: 'Boshqa amallar',
      onSelected: onSelected,
      position: PopupMenuPosition.under,
      itemBuilder: (_) => [
        for (final (key, label, icon, danger) in _statusActions(member.status))
          PopupMenuItem(
            value: key,
            child: Row(
              children: [
                Icon(icon, size: 18, color: danger ? OnDexColors.danger : OnDexColors.inkDim),
                const SizedBox(width: 10),
                Flexible(
                  child: Text(label,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(color: danger ? OnDexColors.danger : OnDexColors.ink)),
                ),
              ],
            ),
          ),
      ],
      child: Container(
        width: 34,
        height: 34,
        decoration: BoxDecoration(
          color: OnDexColors.cardBg,
          borderRadius: BorderRadius.circular(10),
          border: Border.all(color: OnDexColors.cardBorder),
        ),
        child: const Icon(Icons.more_horiz_rounded, size: 18, color: OnDexColors.inkDim),
      ),
    );
  }
}

class _SelectMenu extends StatelessWidget {
  const _SelectMenu({super.key, required this.value, required this.items, required this.onSelected});

  final String value;
  final List<(String, String)> items;
  final ValueChanged<String> onSelected;

  @override
  Widget build(BuildContext context) {
    final label = items.firstWhere((e) => e.$1 == value, orElse: () => items.first).$2;
    return PopupMenuButton<String>(
      tooltip: label,
      onSelected: onSelected,
      position: PopupMenuPosition.under,
      constraints: const BoxConstraints(minWidth: 200, maxHeight: 420),
      itemBuilder: (_) => [
        for (final (key, title) in items)
          PopupMenuItem(
            value: key,
            height: 40,
            child: Row(
              children: [
                SizedBox(
                  width: 22,
                  child: key == value ? const Icon(Icons.check_rounded, size: 17, color: OnDexColors.primary) : null,
                ),
                const SizedBox(width: 6),
                Flexible(child: Text(title, overflow: TextOverflow.ellipsis)),
              ],
            ),
          ),
      ],
      child: Container(
        height: 44,
        padding: const EdgeInsets.only(left: 12, right: 8),
        decoration: BoxDecoration(
          color: OnDexColors.cardBg,
          borderRadius: BorderRadius.circular(10),
          border: Border.all(color: value.isEmpty ? OnDexColors.cardBorder : OnDexColors.primary),
        ),
        child: Row(
          children: [
            Expanded(
              child: Text(label,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: OnDexColors.ink)),
            ),
            const Icon(Icons.keyboard_arrow_down_rounded, size: 20, color: OnDexColors.inkDim),
          ],
        ),
      ),
    );
  }
}

// ─── Xodimlar jadvali ─────────────────────────────────────────────────

class _StaffTableCard extends StatelessWidget {
  const _StaffTableCard({
    required this.members,
    required this.totalCount,
    required this.search,
    required this.positions,
    required this.position,
    required this.status,
    required this.sort,
    required this.onPosition,
    required this.onStatus,
    required this.onSort,
    required this.onClearFilters,
    required this.onAdd,
    required this.onView,
    required this.onEdit,
    required this.onStatusChange,
  });

  final List<StaffMember> members;
  final int totalCount;
  final TextEditingController search;
  final List<StaffPositionInfo> positions;
  final String position;
  final String status;
  final _Sort sort;
  final ValueChanged<String> onPosition;
  final ValueChanged<String> onStatus;
  final ValueChanged<_Sort> onSort;
  final VoidCallback onClearFilters;
  final VoidCallback onAdd;
  final ValueChanged<StaffMember> onView;
  final ValueChanged<StaffMember> onEdit;
  final void Function(StaffMember member, String status) onStatusChange;

  @override
  Widget build(BuildContext context) {
    final searchField = TextField(
      key: const ValueKey('staff-search'),
      controller: search,
      style: const TextStyle(fontSize: 13.5),
      decoration: InputDecoration(
        isDense: true,
        hintText: 'Xodim ismi, lavozimi yoki telefon raqamini qidirish...',
        hintStyle: const TextStyle(fontSize: 13, color: OnDexColors.inkFaint),
        prefixIcon: const Icon(Icons.search_rounded, size: 19, color: OnDexColors.inkFaint),
        suffixIcon: search.text.isEmpty
            ? null
            : IconButton(
                tooltip: 'Tozalash',
                icon: const Icon(Icons.close_rounded, size: 17),
                onPressed: search.clear,
              ),
        filled: true,
        fillColor: OnDexColors.cardBg,
        contentPadding: const EdgeInsets.symmetric(vertical: 13),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(10),
          borderSide: const BorderSide(color: OnDexColors.cardBorder),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(10),
          borderSide: const BorderSide(color: OnDexColors.primary),
        ),
      ),
    );
    final positionMenu = _SelectMenu(
      key: const ValueKey('filter-position'),
      value: position,
      items: [('', 'Barcha lavozimlar'), for (final p in positions) (p.key, p.title)],
      onSelected: onPosition,
    );
    final statusMenu = _SelectMenu(
      key: const ValueKey('filter-status'),
      value: status,
      items: const [('', 'Barcha holatlar'), ('active', 'Faol'), ('on_leave', 'Ta\'tilda'), ('dismissed', 'Ishdan bo\'shagan')],
      onSelected: onStatus,
    );
    final sortMenu = PopupMenuButton<_Sort>(
      key: const ValueKey('staff-sort'),
      tooltip: 'Saralash',
      onSelected: onSort,
      position: PopupMenuPosition.under,
      itemBuilder: (_) => [
        for (final (v, label) in const [
          (_Sort.number, 'Tartib raqami bo\'yicha'),
          (_Sort.name, 'Ism bo\'yicha (A–Z)'),
          (_Sort.newest, 'Yangi qo\'shilganlar'),
        ])
          CheckedPopupMenuItem(value: v, checked: v == sort, child: Text(label)),
      ],
      child: Container(
        width: 44,
        height: 44,
        decoration: BoxDecoration(
          color: OnDexColors.cardBg,
          borderRadius: BorderRadius.circular(10),
          border: Border.all(color: OnDexColors.cardBorder),
        ),
        child: const Icon(Icons.tune_rounded, size: 19, color: OnDexColors.inkDim),
      ),
    );

    return Container(
      key: const ValueKey('staff-table'),
      padding: const EdgeInsets.all(16),
      decoration: _cardBox(),
      child: LayoutBuilder(builder: (context, box) {
        final toolbar = box.maxWidth >= 700
            ? Row(
                children: [
                  Expanded(child: searchField),
                  const SizedBox(width: 12),
                  SizedBox(width: box.maxWidth >= 980 ? 180 : 150, child: positionMenu),
                  const SizedBox(width: 12),
                  SizedBox(width: box.maxWidth >= 980 ? 170 : 140, child: statusMenu),
                  const SizedBox(width: 12),
                  sortMenu,
                ],
              )
            : Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  searchField,
                  const SizedBox(height: 10),
                  Row(
                    children: [
                      Expanded(child: positionMenu),
                      const SizedBox(width: 10),
                      Expanded(child: statusMenu),
                      const SizedBox(width: 10),
                      sortMenu,
                    ],
                  ),
                ],
              );

        Widget content;
        if (totalCount == 0) {
          content = _EmptyState(
            icon: Icons.groups_2_outlined,
            title: 'Hali xodim qo\'shilmagan',
            caption: 'Oshpaz, ofitsiant, kassir va boshqa xodimlaringizni qo\'shing.',
            action: FilledButton.icon(
              onPressed: onAdd,
              style: FilledButton.styleFrom(backgroundColor: OnDexColors.primary),
              icon: const Icon(Icons.add_rounded, size: 18),
              label: const Text('Xodim qo\'shish'),
            ),
          );
        } else if (members.isEmpty) {
          content = _EmptyState(
            icon: Icons.search_off_rounded,
            title: 'Xodim topilmadi',
            caption: 'Qidiruv yoki filtr bo\'yicha mos xodim yo\'q.',
            action: TextButton(onPressed: onClearFilters, child: const Text('Filtrlarni tozalash')),
          );
        } else {
          content = _StaffTable(members: members, onView: onView, onEdit: onEdit, onStatusChange: onStatusChange);
        }

        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            toolbar,
            const SizedBox(height: 14),
            Expanded(child: content),
            const SizedBox(height: 10),
            Text(
              members.length == totalCount
                  ? 'Jami $totalCount ta xodim'
                  : '$totalCount ta xodimdan ${members.length} tasi ko\'rsatilmoqda',
              style: const TextStyle(fontSize: 12, color: OnDexColors.inkFaint),
            ),
          ],
        );
      }),
    );
  }
}

/// Qaysi ustunlar sig'adi. Yon tomonga aylantirish YO'Q: tor joyda
/// avval ish vaqti, keyin telefon, eng oxirida lavozim va tartib raqami
/// yashiriladi (lavozim ism ostiga ko'chadi).
class _StaffCols {
  const _StaffCols(this.width);

  final double width;

  bool get index => width >= 560;
  bool get position => width >= 560;
  bool get phone => width >= 700;
  bool get hours => width >= 860;
}

/// Qat'iy sarlavhali jadval: qatorlar ichkarida aylanadi.
class _StaffTable extends StatelessWidget {
  const _StaffTable({required this.members, required this.onView, required this.onEdit, required this.onStatusChange});

  static const _head = TextStyle(fontSize: 12.5, fontWeight: FontWeight.w600, color: OnDexColors.inkDim);
  static const actionsWidth = 84.0;

  final List<StaffMember> members;
  final ValueChanged<StaffMember> onView;
  final ValueChanged<StaffMember> onEdit;
  final void Function(StaffMember member, String status) onStatusChange;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(builder: (context, c) {
      final cols = _StaffCols(c.maxWidth);
      return Column(
        children: [
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 12),
            decoration: BoxDecoration(color: _headBg, borderRadius: BorderRadius.circular(10)),
            child: Row(
              children: [
                if (cols.index) const SizedBox(width: 36, child: Text('#', style: _head)),
                const Expanded(flex: 26, child: Text('Foydalanuvchi', maxLines: 1, overflow: TextOverflow.ellipsis, style: _head)),
                if (cols.position)
                  const Expanded(flex: 18, child: Text('Lavozimi', maxLines: 1, overflow: TextOverflow.ellipsis, style: _head)),
                if (cols.phone)
                  const Expanded(flex: 16, child: Text('Telefon', maxLines: 1, overflow: TextOverflow.ellipsis, style: _head)),
                if (cols.hours)
                  const Expanded(flex: 14, child: Text('Ish vaqti', maxLines: 1, overflow: TextOverflow.ellipsis, style: _head)),
                const Expanded(flex: 15, child: Text('Holati', maxLines: 1, overflow: TextOverflow.ellipsis, style: _head)),
                const SizedBox(width: actionsWidth, child: Text('Amallar', maxLines: 1, overflow: TextOverflow.ellipsis, style: _head)),
              ],
            ),
          ),
          Expanded(
            child: ListView.separated(
              key: const ValueKey('staff-rows'),
              primary: false,
              padding: EdgeInsets.zero,
              itemCount: members.length,
              separatorBuilder: (_, __) => const Divider(height: 1, color: _rowDivider),
              itemBuilder: (_, i) => _StaffRow(
                index: i + 1,
                cols: cols,
                member: members[i],
                onView: onView,
                onEdit: onEdit,
                onStatusChange: onStatusChange,
              ),
            ),
          ),
        ],
      );
    });
  }
}

class _StaffRow extends StatelessWidget {
  const _StaffRow({
    required this.index,
    required this.cols,
    required this.member,
    required this.onView,
    required this.onEdit,
    required this.onStatusChange,
  });

  final int index;
  final _StaffCols cols;
  final StaffMember member;
  final ValueChanged<StaffMember> onView;
  final ValueChanged<StaffMember> onEdit;
  final void Function(StaffMember member, String status) onStatusChange;

  @override
  Widget build(BuildContext context) {
    final m = member;
    const cell = TextStyle(fontSize: 13, color: OnDexColors.ink);
    final subtitle = cols.position ? 'ID: #${m.code}' : '#${m.code} · ${m.positionTitle}';
    return InkWell(
      key: ValueKey('row-${m.id}'),
      // Qatorni bosish — xodim kartochkasi (alohida "ko'rish" belgisi yo'q).
      onTap: () => onView(m),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 9),
        child: Opacity(
          opacity: m.status == 'dismissed' ? 0.72 : 1,
          child: Row(
            children: [
              if (cols.index)
                SizedBox(width: 36, child: Text('$index', style: const TextStyle(fontSize: 13, color: OnDexColors.inkDim))),
              Expanded(
                flex: 26,
                child: Row(
                  children: [
                    _Avatar(member: m),
                    const SizedBox(width: 10),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Text(m.fullName,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: const TextStyle(fontSize: 13.5, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
                          Text(subtitle,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: const TextStyle(fontSize: 11.5, color: OnDexColors.inkFaint)),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
              if (cols.position)
                Expanded(
                  flex: 18,
                  child: Row(
                    children: [
                      Icon(positionIcon(m.position), size: 17, color: OnDexColors.inkDim),
                      const SizedBox(width: 8),
                      Flexible(child: Text(m.positionTitle, maxLines: 1, overflow: TextOverflow.ellipsis, style: cell)),
                      if (m.appAccessActive) ...[
                        const SizedBox(width: 4),
                        Tooltip(
                          message: '${staffAppName(m.position)} ilovasiga kirish ochiq',
                          child: const Icon(Icons.phone_iphone_rounded, size: 14, color: OnDexColors.success),
                        ),
                      ],
                    ],
                  ),
                ),
              if (cols.phone)
                Expanded(
                  flex: 16,
                  child: Text(formatUzPhone(m.phone), maxLines: 1, overflow: TextOverflow.ellipsis, style: cell),
                ),
              if (cols.hours)
                Expanded(
                  flex: 14,
                  child: Text(m.schedule?.range ?? '—', maxLines: 1, overflow: TextOverflow.ellipsis, style: cell),
                ),
              Expanded(flex: 15, child: Align(alignment: Alignment.centerLeft, child: _StatusPill(status: m.status))),
              SizedBox(
                width: _StaffTable.actionsWidth,
                child: Row(
                  children: [
                    _IconAction(
                        key: ValueKey('edit-${m.id}'),
                        icon: Icons.edit_outlined,
                        tooltip: 'Tahrirlash',
                        onTap: () => onEdit(m)),
                    const SizedBox(width: 6),
                    _MoreMenu(key: ValueKey('more-${m.id}'), member: m, onSelected: (s) => onStatusChange(m, s)),
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

class _EmptyState extends StatelessWidget {
  const _EmptyState({required this.icon, required this.title, required this.caption, required this.action});

  final IconData icon;
  final String title;
  final String caption;
  final Widget action;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: SingleChildScrollView(
        primary: false,
        padding: const EdgeInsets.symmetric(vertical: 24, horizontal: 12),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 44, color: OnDexColors.inkFaint),
            const SizedBox(height: 10),
            Text(title,
                textAlign: TextAlign.center,
                style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
            const SizedBox(height: 4),
            Text(caption, textAlign: TextAlign.center, style: const TextStyle(fontSize: 13, color: OnDexColors.inkDim)),
            const SizedBox(height: 14),
            action,
          ],
        ),
      ),
    );
  }
}

// ─── O'ng ustun ───────────────────────────────────────────────────────

class _SideCard extends StatelessWidget {
  const _SideCard({required this.icon, required this.title, required this.child});

  final IconData icon;
  final String title;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.fromLTRB(18, 16, 18, 18),
      decoration: _cardBox(),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              Container(
                width: 30,
                height: 30,
                decoration: BoxDecoration(color: OnDexColors.primaryTint, borderRadius: BorderRadius.circular(9)),
                child: Icon(icon, size: 17, color: OnDexColors.primary),
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Text(title,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
              ),
            ],
          ),
          const SizedBox(height: 14),
          child,
        ],
      ),
    );
  }
}

const _chartPalette = [
  OnDexColors.primary,
  Color(0xFF3B82F6),
  Color(0xFF22A06B),
  _purple,
  Color(0xFFF0B429),
];

class _DistributionCard extends StatelessWidget {
  const _DistributionCard({required this.summary, required this.positions});

  final StaffSummary summary;
  final List<StaffPositionInfo> positions;

  @override
  Widget build(BuildContext context) {
    String titleOf(String key) => positions.where((p) => p.key == key).firstOrNull?.title ?? key;
    final entries = summary.byPosition.entries.where((e) => e.value > 0 && e.key != 'other').toList()
      ..sort((a, b) => b.value != a.value ? b.value.compareTo(a.value) : titleOf(a.key).compareTo(titleOf(b.key)));
    final total = summary.byPosition.values.fold<int>(0, (s, v) => s + v);
    final slices = <(String, int, Color)>[];
    for (var i = 0; i < entries.length && i < 4; i++) {
      slices.add((titleOf(entries[i].key), entries[i].value, _chartPalette[i]));
    }
    final rest = total - slices.fold<int>(0, (s, e) => s + e.$2);
    if (rest > 0) slices.add(('Boshqa', rest, _chartPalette[4]));

    return _SideCard(
      icon: Icons.groups_rounded,
      title: 'Lavozimlar bo\'yicha taqsimot',
      child: total == 0
          ? const Padding(
              padding: EdgeInsets.symmetric(vertical: 12),
              child: Text('Hali ishlayotgan xodim yo\'q', style: TextStyle(fontSize: 13, color: OnDexColors.inkDim)),
            )
          : LayoutBuilder(builder: (context, box) {
              final donut = SizedBox(
                key: const ValueKey('staff-donut'),
                width: 124,
                height: 124,
                child: CustomPaint(
                  painter: _DonutPainter([for (final s in slices) (s.$2.toDouble(), s.$3)]),
                  child: Center(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        // Ishdan bo'shaganlar taqsimotga kirmaydi — "Jami xodimlar"
                        // kartasi bilan chalkashmasin.
                        const Text('Ishda', style: TextStyle(fontSize: 12, color: OnDexColors.inkDim)),
                        Text('$total',
                            style: const TextStyle(fontSize: 22, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
                      ],
                    ),
                  ),
                ),
              );
              final legend = Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  for (final (title, count, color) in slices)
                    Padding(
                      padding: const EdgeInsets.symmetric(vertical: 5),
                      child: Row(
                        children: [
                          Container(width: 10, height: 10, decoration: BoxDecoration(color: color, shape: BoxShape.circle)),
                          const SizedBox(width: 8),
                          Expanded(
                            child: Text(title,
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: const TextStyle(fontSize: 12.5, color: OnDexColors.ink)),
                          ),
                          const SizedBox(width: 6),
                          Text('$count', style: const TextStyle(fontSize: 12.5, fontWeight: FontWeight.w700)),
                          SizedBox(
                            width: 42,
                            child: Text('${(count * 100 / total).round()}%',
                                textAlign: TextAlign.right,
                                style: const TextStyle(fontSize: 12, color: OnDexColors.inkFaint)),
                          ),
                        ],
                      ),
                    ),
                ],
              );
              if (box.maxWidth >= 280) {
                return Row(children: [donut, const SizedBox(width: 14), Expanded(child: legend)]);
              }
              return Column(children: [donut, const SizedBox(height: 12), legend]);
            }),
    );
  }
}

class _DonutPainter extends CustomPainter {
  _DonutPainter(this.values);

  final List<(double, Color)> values;

  @override
  void paint(Canvas canvas, Size size) {
    const stroke = 20.0;
    final total = values.fold<double>(0, (s, v) => s + v.$1);
    if (total <= 0) return;
    final rect = Rect.fromLTWH(stroke / 2, stroke / 2, size.width - stroke, size.height - stroke);
    final gap = values.length > 1 ? 0.03 : 0.0;
    var start = -math.pi / 2;
    for (final (v, color) in values) {
      final sweep = v / total * 2 * math.pi;
      canvas.drawArc(
        rect,
        start + gap / 2,
        math.max(0.0, sweep - gap),
        false,
        Paint()
          ..style = PaintingStyle.stroke
          ..strokeWidth = stroke
          ..color = color,
      );
      start += sweep;
    }
  }

  @override
  bool shouldRepaint(_DonutPainter old) => old.values != values;
}

class _QuickActionsCard extends StatelessWidget {
  const _QuickActionsCard({required this.onSchedule, required this.onReport});

  final VoidCallback onSchedule;
  final VoidCallback onReport;

  @override
  Widget build(BuildContext context) {
    return _SideCard(
      icon: Icons.bolt_rounded,
      title: 'Tezkor amallar',
      child: Row(
        children: [
          Expanded(
            child: _QuickButton(
              key: const ValueKey('quick-schedule'),
              icon: Icons.calendar_month_rounded,
              label: 'Ish jadvali',
              fg: OnDexColors.info,
              bg: OnDexColors.infoBg,
              onTap: onSchedule,
            ),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: _QuickButton(
              key: const ValueKey('quick-report'),
              icon: Icons.insights_rounded,
              label: 'Hisobot',
              fg: _purple,
              bg: _purpleBg,
              onTap: onReport,
            ),
          ),
        ],
      ),
    );
  }
}

class _QuickButton extends StatelessWidget {
  const _QuickButton({
    super.key,
    required this.icon,
    required this.label,
    required this.fg,
    required this.bg,
    required this.onTap,
  });

  final IconData icon;
  final String label;
  final Color fg;
  final Color bg;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: bg,
      borderRadius: BorderRadius.circular(12),
      child: InkWell(
        borderRadius: BorderRadius.circular(12),
        onTap: onTap,
        child: Container(
          height: 48,
          padding: const EdgeInsets.symmetric(horizontal: 12),
          child: Row(
            children: [
              Icon(icon, size: 19, color: fg),
              const SizedBox(width: 8),
              Expanded(
                child: Text(label,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(fontSize: 13, fontWeight: FontWeight.w700, color: fg)),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _ActivityCard extends StatelessWidget {
  const _ActivityCard({required this.events});

  final List<StaffEvent> events;

  @override
  Widget build(BuildContext context) {
    return _SideCard(
      icon: Icons.history_rounded,
      title: 'So\'nggi faoliyat',
      child: events.isEmpty
          ? const Padding(
              padding: EdgeInsets.symmetric(vertical: 8),
              child: Text('Hali faoliyat yo\'q', style: TextStyle(fontSize: 13, color: OnDexColors.inkDim)),
            )
          : Column(children: [for (final e in events.take(8)) _EventRow(event: e)]),
    );
  }
}

class _EventRow extends StatelessWidget {
  const _EventRow({required this.event});

  final StaffEvent event;

  @override
  Widget build(BuildContext context) {
    final (icon, fg, bg) = event.look;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 30,
            height: 30,
            decoration: BoxDecoration(color: bg, shape: BoxShape.circle),
            child: Icon(icon, size: 16, color: fg),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(event.title,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(fontSize: 12.5, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
                Text(event.subtitle,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(fontSize: 11.5, color: OnDexColors.inkDim)),
              ],
            ),
          ),
          const SizedBox(width: 8),
          Text(_eventTime(event.at), style: const TextStyle(fontSize: 11, color: OnDexColors.inkFaint)),
        ],
      ),
    );
  }
}

part of '../notifications_page.dart';

BoxDecoration _card() => BoxDecoration(
      color: OnDexColors.cardBg,
      borderRadius: BorderRadius.circular(16),
      border: Border.all(color: OnDexColors.cardBorder),
    );

// ─── Sarlavha ─────────────────────────────────────────────────────────

class _AlertsHeader extends StatelessWidget {
  const _AlertsHeader({
    required this.busy,
    required this.onReadAll,
    required this.onRefresh,
    required this.onSettings,
  });

  final bool busy;
  final VoidCallback? onReadAll;
  final VoidCallback onRefresh;
  final VoidCallback? onSettings;

  @override
  Widget build(BuildContext context) {
    const title = Row(
      children: [
        Flexible(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text('Bildirishnomalar',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(fontSize: 24, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
              SizedBox(height: 2),
              Text('Tizim xabarlari, muhim yangiliklar va bildirishnomalar',
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(fontSize: 13.5, color: OnDexColors.inkDim)),
            ],
          ),
        ),
      ],
    );
    final actions = Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        OutlinedButton.icon(
          key: const ValueKey('alerts-read-all'),
          onPressed: busy ? null : onReadAll,
          style: OutlinedButton.styleFrom(
            foregroundColor: OnDexColors.ink,
            backgroundColor: OnDexColors.cardBg,
            side: const BorderSide(color: OnDexColors.cardBorder),
            minimumSize: const Size(0, 44),
            padding: const EdgeInsets.symmetric(horizontal: 16),
            shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
            textStyle: const TextStyle(fontSize: 13.5, fontWeight: FontWeight.w600),
          ),
          icon: busy
              ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
              : const Icon(Icons.done_all_rounded, size: 18),
          label: const Text('Barchasini o\'qish'),
        ),
        const SizedBox(width: 10),
        PopupMenuButton<String>(
          key: const ValueKey('alerts-more'),
          tooltip: 'Boshqa amallar',
          position: PopupMenuPosition.under,
          onSelected: (v) => v == 'refresh' ? onRefresh() : onSettings?.call(),
          itemBuilder: (_) => [
            const PopupMenuItem(
              value: 'refresh',
              child: Row(children: [Icon(Icons.refresh_rounded, size: 18), SizedBox(width: 10), Text('Yangilash')]),
            ),
            PopupMenuItem(
              value: 'settings',
              enabled: onSettings != null,
              child: const Row(children: [
                Icon(Icons.tune_rounded, size: 18),
                SizedBox(width: 10),
                Flexible(child: Text('Bildirishnoma sozlamalari', overflow: TextOverflow.ellipsis)),
              ]),
            ),
          ],
          child: Container(
            width: 44,
            height: 44,
            decoration: BoxDecoration(
              color: OnDexColors.cardBg,
              borderRadius: BorderRadius.circular(12),
              border: Border.all(color: OnDexColors.cardBorder),
            ),
            child: const Icon(Icons.more_horiz_rounded, color: OnDexColors.inkDim),
          ),
        ),
      ],
    );
    return LayoutBuilder(builder: (context, box) {
      if (box.maxWidth >= 640) {
        return Row(children: [const Expanded(child: title), const SizedBox(width: 12), actions]);
      }
      return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [title, const SizedBox(height: 12), actions]);
    });
  }
}

// ─── Ro'yxat ──────────────────────────────────────────────────────────

class _AlertListCard extends StatelessWidget {
  const _AlertListCard({
    required this.items,
    required this.loading,
    required this.loadingMore,
    required this.error,
    required this.hasFilters,
    required this.controller,
    required this.onOpen,
    required this.onRetry,
    required this.onClearFilters,
  });

  final List<AlertItem> items;
  final bool loading;
  final bool loadingMore;
  final String? error;
  final bool hasFilters;
  final ScrollController controller;
  final ValueChanged<AlertItem> onOpen;
  final VoidCallback onRetry;
  final VoidCallback onClearFilters;

  @override
  Widget build(BuildContext context) {
    Widget content;
    if (loading) {
      content = const Center(child: CircularProgressIndicator());
    } else if (error != null && items.isEmpty) {
      content = _Empty(
        icon: Icons.cloud_off_rounded,
        title: error!,
        caption: '',
        action: FilledButton(onPressed: onRetry, child: const Text('Qayta urinish')),
      );
    } else if (items.isEmpty) {
      content = hasFilters
          ? _Empty(
              icon: Icons.search_off_rounded,
              title: 'Filtr bo\'yicha bildirishnoma topilmadi',
              caption: 'Boshqa tur yoki davrni tanlab ko\'ring.',
              action: TextButton(onPressed: onClearFilters, child: const Text('Filtrlarni tozalash')),
            )
          : const _Empty(
              icon: Icons.notifications_none_rounded,
              title: 'Hozircha bildirishnoma yo\'q',
              caption: 'Yangi buyurtma, to\'lov va xodimlar bo\'yicha xabarlar shu yerda darhol paydo bo\'ladi.',
              action: SizedBox.shrink(),
            );
    } else {
      content = ListView.separated(
        key: const ValueKey('alerts-list'),
        controller: controller,
        primary: false,
        padding: const EdgeInsets.all(10),
        itemCount: items.length + (loadingMore ? 1 : 0),
        separatorBuilder: (_, __) => const SizedBox(height: 8),
        itemBuilder: (_, i) {
          if (i >= items.length) {
            return const Padding(
              padding: EdgeInsets.all(12),
              child: Center(child: SizedBox(width: 22, height: 22, child: CircularProgressIndicator(strokeWidth: 2))),
            );
          }
          return _AlertTile(item: items[i], onTap: () => onOpen(items[i]));
        },
      );
    }
    return Container(key: const ValueKey('alerts-card'), decoration: _card(), child: content);
  }
}

class _CategoryPill extends StatelessWidget {
  const _CategoryPill({required this.category});

  final String category;

  @override
  Widget build(BuildContext context) {
    final (fg, bg) = categoryColors(category);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(color: bg, borderRadius: BorderRadius.circular(999)),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(width: 7, height: 7, decoration: BoxDecoration(color: fg, shape: BoxShape.circle)),
          const SizedBox(width: 5),
          Text(categoryTitle(category), style: TextStyle(fontSize: 11.5, fontWeight: FontWeight.w700, color: fg)),
        ],
      ),
    );
  }
}

class _AlertTile extends StatelessWidget {
  const _AlertTile({required this.item, required this.onTap});

  final AlertItem item;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final (fg, bg) = categoryColors(item.category);
    final unread = !item.read;
    return Material(
      color: unread ? const Color(0xFFFFF3EA) : OnDexColors.cardBg,
      borderRadius: BorderRadius.circular(14),
      child: InkWell(
        key: ValueKey('alert-${item.id}'),
        borderRadius: BorderRadius.circular(14),
        onTap: onTap,
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(14),
            border: Border.all(color: unread ? OnDexColors.primary.withValues(alpha: 0.35) : OnDexColors.cardBorder),
          ),
          child: LayoutBuilder(builder: (context, c) {
            final narrow = c.maxWidth < 520;
            final time = Text(alertTime(item.createdAt),
                style: const TextStyle(fontSize: 12, color: OnDexColors.inkFaint));
            final texts = Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Row(
                  children: [
                    if (unread) ...[
                      Container(
                        key: ValueKey('alert-unread-${item.id}'),
                        width: 8,
                        height: 8,
                        decoration: const BoxDecoration(color: OnDexColors.primary, shape: BoxShape.circle),
                      ),
                      const SizedBox(width: 6),
                    ],
                    Flexible(
                      child: Text(item.title,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(fontSize: 14.5, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
                    ),
                  ],
                ),
                const SizedBox(height: 3),
                Text.rich(
                  TextSpan(children: richBody(item.body)),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontSize: 13, color: OnDexColors.inkDim, height: 1.35),
                ),
                if (narrow) ...[
                  const SizedBox(height: 8),
                  Wrap(
                    spacing: 10,
                    runSpacing: 4,
                    crossAxisAlignment: WrapCrossAlignment.center,
                    children: [_CategoryPill(category: item.category), time],
                  ),
                ],
              ],
            );
            return Row(
              children: [
                Container(
                  width: 48,
                  height: 48,
                  decoration: BoxDecoration(color: bg, borderRadius: BorderRadius.circular(12)),
                  child: Icon(kindIcon(item.kind), color: fg, size: 24),
                ),
                const SizedBox(width: 14),
                Expanded(child: texts),
                if (!narrow) ...[
                  const SizedBox(width: 12),
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.end,
                    mainAxisSize: MainAxisSize.min,
                    children: [time, const SizedBox(height: 6), _CategoryPill(category: item.category)],
                  ),
                ],
                const SizedBox(width: 8),
                const Icon(Icons.chevron_right_rounded, color: OnDexColors.inkFaint),
              ],
            );
          }),
        ),
      ),
    );
  }
}

class _Empty extends StatelessWidget {
  const _Empty({required this.icon, required this.title, required this.caption, required this.action});

  final IconData icon;
  final String title;
  final String caption;
  final Widget action;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: SingleChildScrollView(
        primary: false,
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 46, color: OnDexColors.inkFaint),
            const SizedBox(height: 10),
            Text(title,
                textAlign: TextAlign.center,
                style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
            if (caption.isNotEmpty) ...[
              const SizedBox(height: 4),
              Text(caption, textAlign: TextAlign.center, style: const TextStyle(fontSize: 13, color: OnDexColors.inkDim)),
            ],
            const SizedBox(height: 14),
            action,
          ],
        ),
      ),
    );
  }
}

// ─── O'ng ustun ───────────────────────────────────────────────────────

class _SummaryCard extends StatelessWidget {
  const _SummaryCard({required this.total, required this.unread, required this.byCategory});

  final int total;
  final int unread;
  final Map<String, int> byCategory;

  @override
  Widget build(BuildContext context) {
    return Container(
      key: const ValueKey('alerts-summary'),
      padding: const EdgeInsets.fromLTRB(18, 16, 18, 16),
      decoration: _card(),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              Stack(
                clipBehavior: Clip.none,
                children: [
                  Container(
                    width: 50,
                    height: 50,
                    decoration: BoxDecoration(color: OnDexColors.primaryTint, borderRadius: BorderRadius.circular(14)),
                    child: const Icon(Icons.notifications_rounded, color: OnDexColors.primary, size: 26),
                  ),
                  if (unread > 0)
                    Positioned(
                      right: -6,
                      top: -6,
                      child: Container(
                        key: const ValueKey('alerts-unread-badge'),
                        padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                        constraints: const BoxConstraints(minWidth: 20),
                        decoration: BoxDecoration(
                          color: OnDexColors.danger,
                          borderRadius: BorderRadius.circular(999),
                          border: Border.all(color: Colors.white, width: 1.5),
                        ),
                        child: Text(unread > 99 ? '99+' : '$unread',
                            textAlign: TextAlign.center,
                            style: const TextStyle(color: Colors.white, fontSize: 11, fontWeight: FontWeight.w800)),
                      ),
                    ),
                ],
              ),
              const SizedBox(width: 14),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const Text('Jami bildirishnomalar',
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(fontSize: 13, color: OnDexColors.inkDim)),
                    Text('$total ta',
                        style: const TextStyle(fontSize: 24, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),
          for (final (key, title) in alertCategories)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 5),
              child: Row(
                children: [
                  Container(
                    width: 10,
                    height: 10,
                    decoration: BoxDecoration(color: categoryColors(key).$1, shape: BoxShape.circle),
                  ),
                  const SizedBox(width: 10),
                  Expanded(
                    child: Text(title,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(fontSize: 13, color: OnDexColors.ink)),
                  ),
                  Text('${byCategory[key] ?? 0}',
                      key: ValueKey('alerts-count-$key'),
                      style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
                ],
              ),
            ),
        ],
      ),
    );
  }
}

class _AlertSelect extends StatelessWidget {
  const _AlertSelect({super.key, required this.value, required this.items, required this.onSelected});

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
      constraints: const BoxConstraints(minWidth: 220, maxHeight: 420),
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

class _FilterCard extends StatelessWidget {
  const _FilterCard({
    required this.category,
    required this.period,
    required this.search,
    required this.onCategory,
    required this.onPeriod,
  });

  final String category;
  final String period;
  final TextEditingController search;
  final ValueChanged<String> onCategory;
  final ValueChanged<String> onPeriod;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.fromLTRB(18, 16, 18, 18),
      decoration: _card(),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          const Row(
            children: [
              Icon(Icons.filter_alt_outlined, size: 20, color: OnDexColors.ink),
              SizedBox(width: 8),
              Text('Filtrlash', style: TextStyle(fontSize: 15, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
            ],
          ),
          const SizedBox(height: 12),
          _AlertSelect(
            key: const ValueKey('alert-filter-category'),
            value: category,
            items: const [('', 'Barchasi'), ...alertCategories],
            onSelected: onCategory,
          ),
          const SizedBox(height: 10),
          _AlertSelect(
            key: const ValueKey('alert-filter-period'),
            value: period,
            items: const [('', 'Barcha vaqt'), ('today', 'Bugun'), ('week', 'Oxirgi 7 kun'), ('month', 'Oxirgi 30 kun')],
            onSelected: onPeriod,
          ),
          const SizedBox(height: 10),
          TextField(
            key: const ValueKey('alert-search'),
            controller: search,
            maxLength: 100,
            style: const TextStyle(fontSize: 13.5),
            decoration: InputDecoration(
              counterText: '',
              isDense: true,
              hintText: 'Bildirishnoma qidirish...',
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
          ),
        ],
      ),
    );
  }
}


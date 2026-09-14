import 'package:flutter/material.dart';

import '../format.dart';
import '../models/waiter_order.dart';
import '../models/waiter_table.dart';
import '../state/waiter_store.dart';
import '../theme.dart';
import '../widgets/common.dart';
import 'table_detail_screen.dart';

/// Restoranning BARCHA joylari — zallar bo'yicha, jonli holati bilan.
///
/// ┌─ MA'LUMOT MANBAI ─────────────────────────────────────────────────┐
/// Avval bu ekran faqat FAOL BUYURTMALARDAN yig'ilardi, chunki server
/// affitsiantga joylar ro'yxatini bermasdi — bo'sh stollar umuman
/// ko'rinmasdi va affitsiant ularga buyurtma kirita olmasdi.
///
/// Endi `GET /waiter/tables` joylarni holati bilan beradi (QR tokensiz),
/// ya'ni "bo'sh" degani serverdagi haqiqiy holat — taxmin emas.
/// └───────────────────────────────────────────────────────────────────┘
class TablesScreen extends StatelessWidget {
  const TablesScreen({super.key, required this.store});

  final WaiterStore store;

  @override
  Widget build(BuildContext context) {
    if (!store.tablesLoaded) {
      return const Center(child: CircularProgressIndicator());
    }

    final tables = [...store.allTables]..sort(_compareTables);
    if (tables.isEmpty) {
      return RefreshIndicator(
        onRefresh: store.refresh,
        color: kBrandColor,
        backgroundColor: kSurface,
        child: ListView(
          children: [
            if (store.tablesError != null)
              ErrorRetry(message: store.tablesError!, onRetry: store.refresh)
            else
              const EmptyState(
                icon: Icons.table_restaurant_outlined,
                title: 'Joylar hali qo\'shilmagan',
                subtitle: 'Restoran panelidagi "Stollar (QR)" bo\'limida '
                    'joy qo\'shilgach, u shu yerda ko\'rinadi.',
              ),
          ],
        ),
      );
    }

    final zones = <String, List<WaiterTable>>{};
    for (final t in tables) {
      zones.putIfAbsent(t.zone.isEmpty ? 'Asosiy zal' : t.zone, () => []).add(t);
    }
    // Katak balandligi shrift o'lchamiga moslanadi: telefonda katta
    // shrift yoqilgan bo'lsa ham yozuvlar kataklardan chiqib ketmasin.
    final extent = 64 + MediaQuery.textScalerOf(context).scale(70);

    return RefreshIndicator(
      onRefresh: store.refresh,
      color: kBrandColor,
      backgroundColor: kSurface,
      child: CustomScrollView(
        slivers: [
          if (store.tablesError != null)
            SliverToBoxAdapter(child: _StaleBanner(message: store.tablesError!)),
          for (final entry in zones.entries) ...[
            SliverPadding(
              padding: const EdgeInsets.fromLTRB(12, 0, 12, 0),
              sliver: SliverToBoxAdapter(
                child: SectionHeader(title: entry.key, count: entry.value.length),
              ),
            ),
            SliverPadding(
              padding: const EdgeInsets.symmetric(horizontal: 12),
              sliver: SliverGrid(
                gridDelegate: SliverGridDelegateWithMaxCrossAxisExtent(
                  maxCrossAxisExtent: 240,
                  mainAxisSpacing: 12,
                  crossAxisSpacing: 12,
                  mainAxisExtent: extent,
                ),
                delegate: SliverChildBuilderDelegate(
                  (context, i) {
                    final table = entry.value[i];
                    return _TableTile(
                      table: table,
                      orders: store.ordersForTable(table),
                      onTap: () => Navigator.of(context).push(
                        MaterialPageRoute(
                          builder: (_) => TableDetailScreen(
                            tableId: table.id,
                            store: store,
                          ),
                        ),
                      ),
                    );
                  },
                  childCount: entry.value.length,
                ),
              ),
            ),
          ],
          const SliverToBoxAdapter(child: SizedBox(height: 24)),
        ],
      ),
    );
  }

  /// Tartib: tur (stol, kabina...), keyin nom INSON tartibida
  /// ("2" < "10").
  static int _compareTables(WaiterTable a, WaiterTable b) {
    final byZone = a.zone.toLowerCase().compareTo(b.zone.toLowerCase());
    if (byZone != 0) return byZone;
    final byKind = a.kind.compareTo(b.kind);
    if (byKind != 0) return byKind;
    return TableGroup.compareLabels(a.label, b.label);
  }
}

class _StaleBanner extends StatelessWidget {
  const _StaleBanner({required this.message});

  final String message;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.fromLTRB(12, 12, 12, 0),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        color: kWaitingColor.withValues(alpha: 0.14),
        borderRadius: BorderRadius.circular(kRadiusButton),
      ),
      child: Row(
        children: [
          const Icon(Icons.cloud_off, size: 16, color: kWaitingColor),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              'Ro\'yxat yangilanmadi: $message',
              style: const TextStyle(color: kWaitingColor, fontSize: 12.5),
            ),
          ),
        ],
      ),
    );
  }
}

class _TableTile extends StatelessWidget {
  const _TableTile({
    required this.table,
    required this.orders,
    required this.onTap,
  });

  final WaiterTable table;
  final List<WaiterOrder> orders;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    // Rang ustuvorligi: TAYYOR (harakat kerak) > band > tozalanmoqda >
    // bo'sh > yopiq. Affitsiant birinchi navbatda "qayerga borishim
    // kerak"ni ko'rishi kerak.
    final ready = orders.any((o) => o.isReady);
    final (String statusText, Color accent) = ready
        ? ('Yetkazish kerak', kReadyColor)
        : table.isOccupied
            ? ('Band', kWaitingColor)
            : table.isCleaning
                ? ('Tozalanmoqda', kInkFaint)
                : table.isInactive
                    ? ('Yopiq', kInkGhost)
                    : ('Bo\'sh', kReadyColor);
    final dim = table.isInactive && !table.isOccupied;

    return Opacity(
      opacity: dim ? 0.55 : 1,
      child: Material(
        color: ready ? const Color(0x142E9E4F) : kSurface,
        borderRadius: BorderRadius.circular(kRadiusCard),
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(kRadiusCard),
          child: Container(
            padding: const EdgeInsets.all(14),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(kRadiusCard),
              border: Border.all(
                color: ready ? kReadyColor : kBorder,
                width: ready ? 1.6 : 1,
              ),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Expanded(
                      child: Text(
                        tableText(table.shortName),
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                          fontSize: 16,
                          height: 1.2,
                          fontWeight: FontWeight.bold,
                          color: kInk,
                        ),
                      ),
                    ),
                    const SizedBox(width: 6),
                    Padding(
                      padding: const EdgeInsets.only(top: 5),
                      child: Container(
                        width: 9,
                        height: 9,
                        decoration: BoxDecoration(
                          shape: BoxShape.circle,
                          color: accent,
                        ),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 4),
                Text(
                  statusText,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontSize: 12.5,
                    fontWeight: FontWeight.w600,
                    color: accent,
                  ),
                ),
                const Spacer(),
                Row(
                  children: [
                    if (table.capacity != null) ...[
                      const Icon(Icons.people_outline, size: 13, color: kInkFaint),
                      const SizedBox(width: 3),
                      Text(
                        '${table.capacity}',
                        style: const TextStyle(color: kInkFaint, fontSize: 12),
                      ),
                      const SizedBox(width: 10),
                    ],
                    if (orders.isNotEmpty) ...[
                      const Icon(Icons.receipt_long, size: 13, color: kInkFaint),
                      const SizedBox(width: 3),
                      Text(
                        '${orders.length}',
                        style: const TextStyle(color: kInkFaint, fontSize: 12),
                      ),
                    ],
                  ],
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

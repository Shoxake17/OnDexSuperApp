import 'package:flutter/material.dart';
import 'package:ondex_core/ondex_core.dart' show formatSum;

import '../format.dart';
import '../models/waiter_table.dart';
import '../state/waiter_store.dart';
import '../theme.dart';
import '../widgets/common.dart';
import '../widgets/order_card.dart';
import 'new_order_screen.dart';
import 'order_detail_screen.dart';

/// Bitta joy: faol buyurtmalari, umumiy hisobi va "Buyurtma berish".
///
/// ┌─ NEGA KERAK ──────────────────────────────────────────────────────┐
/// Bitta stol o'tirish davomida bir necha marta buyurtma beradi (avval
/// ovqat, keyin choy/desert). Affitsiant "shu stolga yana nima ketishi
/// kerak" va "hisob qancha" degan savollarga shu yerda javob topadi va
/// mehmon og'zaki aytgan buyurtmani shu yerdan oshxonaga yuboradi.
/// └───────────────────────────────────────────────────────────────────┘
class TableDetailScreen extends StatelessWidget {
  const TableDetailScreen({
    super.key,
    required this.tableId,
    required this.store,
  });

  final String tableId;
  final WaiterStore store;

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: store,
      builder: (context, _) {
        // Joy jonli ro'yxatdan olinadi: restoran uni yopsa yoki
        // o'chirsa, ekran o'zi yangilanadi.
        final table = store.tableById(tableId);
        if (table == null) {
          return Scaffold(
            appBar: AppBar(title: const Text('Joy')),
            body: ListView(
              children: [
                const EmptyState(
                  icon: Icons.table_restaurant_outlined,
                  title: 'Bu joy topilmadi',
                  subtitle: 'Restoran uni o\'chirgan bo\'lishi mumkin.',
                ),
                Center(
                  child: OutlinedButton(
                    onPressed: () => Navigator.of(context).pop(),
                    child: const Text('Stollarga qaytish'),
                  ),
                ),
              ],
            ),
          );
        }

        final orders = store.ordersForTable(table);
        final total = orders.fold<int>(0, (s, o) => s + o.totalTiyin);
        final readyCount = orders.where((o) => o.isReady).length;

        return Scaffold(
          appBar: AppBar(
            toolbarHeight: 64,
            title: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  tableText(table.displayLabel),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontSize: 17, fontWeight: FontWeight.bold),
                ),
                if (table.capacity != null)
                  Text(
                    '${table.capacity} kishilik',
                    style: const TextStyle(fontSize: 12, color: kInkFaint),
                  ),
              ],
            ),
          ),
          body: Column(
            children: [
              if (readyCount > 0)
                Container(
                  width: double.infinity,
                  margin: const EdgeInsets.fromLTRB(12, 8, 12, 0),
                  padding:
                      const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
                  decoration: BoxDecoration(
                    color: kReadyColor.withValues(alpha: 0.12),
                    borderRadius: BorderRadius.circular(kRadiusCard),
                    border: Border.all(color: kReadyColor.withValues(alpha: 0.4)),
                  ),
                  child: Row(
                    children: [
                      const Icon(Icons.room_service, size: 18, color: kReadyColor),
                      const SizedBox(width: 10),
                      Expanded(
                        child: Text(
                          readyCount == 1
                              ? 'Bitta buyurtma yetkazishni kutmoqda'
                              : '$readyCount ta buyurtma yetkazishni kutmoqda',
                          style: const TextStyle(
                            color: kReadyColor,
                            fontWeight: FontWeight.w600,
                            fontSize: 13,
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              Expanded(
                child: orders.isEmpty
                    ? ListView(
                        children: [
                          EmptyState(
                            icon: Icons.event_seat_outlined,
                            title: 'Stolda faol buyurtma yo\'q',
                            subtitle: table.canOrder
                                ? 'Mehmon buyurtmasini pastdagi tugma orqali kiriting.'
                                : 'Joy vaqtincha yopilgan — restoran uni '
                                    'panelda qayta ochishi kerak.',
                          ),
                        ],
                      )
                    : ListView(
                        padding: const EdgeInsets.fromLTRB(12, 12, 12, 24),
                        children: [
                          for (final o in orders)
                            OrderCard(
                              order: o,
                              store: store,
                              showTable: false,
                              onTap: () => Navigator.of(context).push(
                                MaterialPageRoute(
                                  builder: (_) => OrderDetailScreen(
                                    orderId: o.id,
                                    store: store,
                                  ),
                                ),
                              ),
                            ),
                        ],
                      ),
              ),
              _BottomPanel(
                table: table,
                orderCount: orders.length,
                totalTiyin: total,
                onOrder: () => Navigator.of(context).push(
                  MaterialPageRoute(
                    builder: (_) => NewOrderScreen(table: table, store: store),
                  ),
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}

class _BottomPanel extends StatelessWidget {
  const _BottomPanel({
    required this.table,
    required this.orderCount,
    required this.totalTiyin,
    required this.onOrder,
  });

  final WaiterTable table;
  final int orderCount;
  final int totalTiyin;
  final VoidCallback onOrder;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.fromLTRB(16, 14, 16, 14),
      decoration: const BoxDecoration(
        color: kSurface,
        border: Border(top: BorderSide(color: kBorder)),
      ),
      child: SafeArea(
        top: false,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            if (orderCount > 0) ...[
              Row(
                children: [
                  const Text(
                    'Stol hisobi',
                    style: TextStyle(
                      color: kInk,
                      fontWeight: FontWeight.bold,
                      fontSize: 15,
                    ),
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      '$orderCount ta buyurtma',
                      style: const TextStyle(color: kInkGhost, fontSize: 12),
                    ),
                  ),
                  Text(
                    formatSum(totalTiyin),
                    style: const TextStyle(
                      color: kBrandColor,
                      fontWeight: FontWeight.bold,
                      fontSize: 18,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 4),
              const Text(
                // To'lov ilovada QABUL QILINMAYDI — bu ataylab: naqd/karta
                // affitsiantning qo'lidan o'tadi, ilova esa faqat raqamni
                // ko'rsatadi.
                'To\'lov naqd yoki karta orqali qabul qilinadi',
                style: TextStyle(color: kInkGhost, fontSize: 11.5),
              ),
              const SizedBox(height: 12),
            ],
            FilledButton.icon(
              key: const ValueKey('table-new-order'),
              onPressed: table.canOrder ? onOrder : null,
              icon: const Icon(Icons.add_shopping_cart_rounded, size: 20),
              label: Text(table.canOrder ? 'Buyurtma berish' : 'Joy yopilgan'),
            ),
          ],
        ),
      ),
    );
  }
}

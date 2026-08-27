import 'package:flutter/material.dart';
import 'package:ondex_core/ondex_core.dart' show formatSum;

import '../format.dart';
import '../state/waiter_store.dart';
import '../theme.dart';
import '../widgets/common.dart';
import '../widgets/order_card.dart';
import 'order_detail_screen.dart';

/// Bitta stolning BARCHA faol buyurtmalari va umumiy summasi.
///
/// ┌─ NEGA KERAK ──────────────────────────────────────────────────────┐
/// Bitta stol o'tirish davomida bir necha marta buyurtma beradi (avval
/// ovqat, keyin choy/desert). Buyurtmalar ro'yxatida ular alohida
/// kartochkalar bo'lib turadi va affitsiant "shu stolga yana nima
/// ketishi kerak" degan savolga javob topa olmasdi — ro'yxatni ko'zi
/// bilan qidirib chiqardi.
///
/// Umumiy summa esa hisob so'ralganda kerak: OnDex'da stol buyurtmasi
/// naqd/karta bilan AFFITSIANTGA to'lanadi (ilovada to'lov integratsiyasi
/// ataylab yo'q), shuning uchun u umumiy raqamni bilishi shart.
/// └───────────────────────────────────────────────────────────────────┘
class TableDetailScreen extends StatelessWidget {
  const TableDetailScreen({
    super.key,
    required this.label,
    required this.store,
  });

  final String label;
  final WaiterStore store;

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: store,
      builder: (context, _) {
        final orders = store.ordersForTable(label);
        final total = orders.fold<int>(0, (s, o) => s + o.totalTiyin);
        final party = orders.fold<int>(
          0,
          (m, o) => o.partySize > m ? o.partySize : m,
        );
        final readyCount = orders.where((o) => o.isReady).length;

        return Scaffold(
          appBar: AppBar(
            title: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  tableText(label),
                  style: const TextStyle(
                      fontSize: 18, fontWeight: FontWeight.bold),
                ),
                if (party > 0)
                  Text(
                    '$party kishi',
                    style: const TextStyle(fontSize: 12, color: kInkFaint),
                  ),
              ],
            ),
          ),
          body: orders.isEmpty
              ? ListView(
                  children: [
                    const EmptyState(
                      icon: Icons.task_alt,
                      title: 'Stolda faol buyurtma qolmadi',
                      subtitle: 'Barcha buyurtmalar yakunlangan.',
                    ),
                    Center(
                      child: OutlinedButton(
                        onPressed: () => Navigator.of(context).pop(),
                        child: const Text('Stollarga qaytish'),
                      ),
                    ),
                  ],
                )
              : Column(
                  children: [
                    if (readyCount > 0)
                      Container(
                        width: double.infinity,
                        margin: const EdgeInsets.fromLTRB(12, 8, 12, 0),
                        padding: const EdgeInsets.symmetric(
                            horizontal: 14, vertical: 12),
                        decoration: BoxDecoration(
                          color: kReadyColor.withValues(alpha: 0.12),
                          borderRadius: BorderRadius.circular(kRadiusCard),
                          border: Border.all(
                              color: kReadyColor.withValues(alpha: 0.4)),
                        ),
                        child: Row(
                          children: [
                            const Icon(Icons.room_service,
                                size: 18, color: kReadyColor),
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
                      child: ListView(
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
                    // ── Umumiy hisob ──
                    Container(
                      padding: const EdgeInsets.fromLTRB(16, 14, 16, 14),
                      decoration: const BoxDecoration(
                        color: kSurface,
                        border: Border(top: BorderSide(color: kBorder)),
                      ),
                      child: SafeArea(
                        top: false,
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
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
                                Text(
                                  '${orders.length} ta buyurtma',
                                  style: const TextStyle(
                                      color: kInkGhost, fontSize: 12),
                                ),
                                const Spacer(),
                                Text(
                                  formatSum(total),
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
                              // To'lov ilovada QABUL QILINMAYDI — bu
                              // ataylab: naqd/karta affitsiantning
                              // qo'lidan o'tadi, ilova esa faqat
                              // raqamni ko'rsatadi.
                              'To\'lov naqd yoki karta orqali qabul qilinadi',
                              style: TextStyle(color: kInkGhost, fontSize: 11.5),
                            ),
                          ],
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

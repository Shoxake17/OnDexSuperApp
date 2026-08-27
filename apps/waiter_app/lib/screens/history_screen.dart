import 'package:flutter/material.dart';
import 'package:ondex_core/ondex_core.dart' show formatSum;

import '../format.dart';
import '../models/activity.dart';
import '../state/waiter_store.dart';
import '../theme.dart';
import '../widgets/common.dart';

/// Smena tarixi — "bugun nechta stolga xizmat qildim".
///
/// ┌─ NEGA QURILMADA ──────────────────────────────────────────────────┐
/// `/waiter/orders` FAQAT faol buyurtmalarni qaytaradi — yakunlangani
/// ro'yxatni to'ldirib yubormasligi uchun ataylab shunday
/// (`routes_waiter.go`). Ya'ni server "bugun nima yetkazdim" degan
/// savolga javob bermaydi.
///
/// Shuning uchun "Yetkazdim" bosilgan har bir buyurtma shu telefonda
/// yozib boriladi. Bu — o'zini tekshirish uchun shaxsiy ro'yxat, rasmiy
/// hisobot EMAS. Cheklov ekranda ochiq yozilgan: ilova qayta
/// o'rnatilsa yoki boshqa telefondan kirilsa yozuvlar qolmaydi.
/// └───────────────────────────────────────────────────────────────────┘
class HistoryScreen extends StatelessWidget {
  const HistoryScreen({super.key, required this.store});

  final WaiterStore store;

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: store,
      builder: (context, _) {
        final today = store.todayHistory;
        final older = store.history.where((h) => !h.isToday).toList();

        return Scaffold(
          appBar: AppBar(
            title: const Text(
              'Mening tarixim',
              style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
            ),
          ),
          body: store.history.isEmpty
              ? ListView(
                  children: const [
                    EmptyState(
                      icon: Icons.history,
                      title: 'Tarix bo\'sh',
                      subtitle: 'Siz "Yetkazdim" deb belgilagan buyurtmalar '
                          'shu yerda yig\'iladi.',
                    ),
                  ],
                )
              : ListView(
                  padding: const EdgeInsets.fromLTRB(12, 8, 12, 24),
                  children: [
                    _TodayCard(
                      count: today.length,
                      totalTiyin: store.todayTotalTiyin,
                    ),
                    if (today.isNotEmpty) ...[
                      const SectionHeader(title: 'Bugun'),
                      for (final h in today) _HistoryTile(record: h),
                    ],
                    if (older.isNotEmpty) ...[
                      const SectionHeader(title: 'Avvalgi kunlar'),
                      for (final h in older) _HistoryTile(record: h),
                    ],
                    const SizedBox(height: 16),
                    const Text(
                      'Bu ro\'yxat faqat shu telefonda saqlanadi va rasmiy '
                      'hisobot emas.',
                      textAlign: TextAlign.center,
                      style: TextStyle(color: kInkGhost, fontSize: 11.5),
                    ),
                  ],
                ),
        );
      },
    );
  }
}

class _TodayCard extends StatelessWidget {
  const _TodayCard({required this.count, required this.totalTiyin});

  final int count;
  final int totalTiyin;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: kSurface,
        borderRadius: BorderRadius.circular(kRadiusCard),
        border: Border.all(color: kBorder),
      ),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  '$count',
                  style: const TextStyle(
                    fontSize: 26,
                    fontWeight: FontWeight.bold,
                    color: kBrandColor,
                  ),
                ),
                const Text(
                  'buyurtma yetkazdim',
                  style: TextStyle(color: kInkFaint, fontSize: 12.5),
                ),
              ],
            ),
          ),
          Container(width: 1, height: 42, color: kBorder),
          Expanded(
            child: Padding(
              padding: const EdgeInsets.only(left: 16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    formatSum(totalTiyin),
                    style: const TextStyle(
                      fontSize: 17,
                      fontWeight: FontWeight.bold,
                      color: kInk,
                    ),
                  ),
                  const Text(
                    'jami chek summasi',
                    style: TextStyle(color: kInkFaint, fontSize: 12.5),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _HistoryTile extends StatelessWidget {
  const _HistoryTile({required this.record});

  final ServedRecord record;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: 8),
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      decoration: BoxDecoration(
        color: kSurface,
        borderRadius: BorderRadius.circular(kRadiusCard),
        border: Border.all(color: kBorder),
      ),
      child: Row(
        children: [
          SizedBox(
            width: 46,
            child: Text(
              formatClock(record.servedAt),
              style: const TextStyle(color: kInkFaint, fontSize: 12.5),
            ),
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  tableText(record.tableLabel),
                  style: const TextStyle(
                    color: kInk,
                    fontWeight: FontWeight.w600,
                    fontSize: 14,
                  ),
                ),
                Text(
                  '${record.shortNumber} · ${record.itemCount} ta mahsulot',
                  style: const TextStyle(color: kInkGhost, fontSize: 12),
                ),
              ],
            ),
          ),
          Text(
            formatSum(record.totalTiyin),
            style: const TextStyle(
              color: kInkDim,
              fontWeight: FontWeight.w600,
              fontSize: 13,
            ),
          ),
        ],
      ),
    );
  }
}

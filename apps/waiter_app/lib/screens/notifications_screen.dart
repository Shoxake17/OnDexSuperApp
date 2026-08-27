import 'package:flutter/material.dart';

import '../format.dart';
import '../models/activity.dart';
import '../state/waiter_store.dart';
import '../theme.dart';
import '../widgets/common.dart';

/// Bildirishnomalar tasmasi — smena davomidagi hodisalar.
///
/// ┌─ QAYERDAN KELADI ─────────────────────────────────────────────────┐
/// Yozuvlar WebSocket hodisalaridan va ro'yxat yangilanishidan yig'iladi
/// (`WaiterStore._pushFeed`), server tarafda affitsiant uchun
/// bildirishnoma tarixi saqlanmaydi. Ya'ni tasma SHU QURILMANIKI.
///
/// Bu ataylab: affitsiant "ovoz eshitdim, lekin qaysi stol ekanini
/// ko'rmay qoldim" holatida orqaga qarab bilishi kerak — buning uchun
/// server tarixi shart emas, ilova o'zi ko'rgan hodisalarni eslab
/// qolsa kifoya.
/// └───────────────────────────────────────────────────────────────────┘
class NotificationsScreen extends StatelessWidget {
  const NotificationsScreen({super.key, required this.store});

  final WaiterStore store;

  @override
  Widget build(BuildContext context) {
    final feed = store.feed;

    if (feed.isEmpty) {
      return ListView(
        children: const [
          EmptyState(
            icon: Icons.notifications_none,
            title: 'Xabarlar yo\'q',
            subtitle: 'Yangi buyurtma kelganda va taom tayyor bo\'lganda '
                'shu yerda yoziladi.',
          ),
        ],
      );
    }

    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 4, 8, 0),
          child: Row(
            children: [
              const Expanded(
                child: Text(
                  'Faqat shu qurilmada saqlanadi',
                  style: TextStyle(color: kInkGhost, fontSize: 11.5),
                ),
              ),
              TextButton(
                onPressed: store.clearFeed,
                style: TextButton.styleFrom(foregroundColor: kInkFaint),
                child: const Text('Tozalash', style: TextStyle(fontSize: 12.5)),
              ),
            ],
          ),
        ),
        Expanded(
          child: ListView.separated(
            padding: const EdgeInsets.fromLTRB(12, 4, 12, 24),
            itemCount: feed.length,
            separatorBuilder: (_, _) => const SizedBox(height: 8),
            itemBuilder: (_, i) => _FeedTile(item: feed[i]),
          ),
        ),
      ],
    );
  }
}

class _FeedTile extends StatelessWidget {
  const _FeedTile({required this.item});

  final FeedItem item;

  @override
  Widget build(BuildContext context) {
    final (icon, color) = switch (item.kind) {
      'ready' => (Icons.room_service, kReadyColor),
      'new' => (Icons.receipt_long, kBrandColor),
      'served' => (Icons.check_circle, kReadyColor),
      _ => (Icons.info_outline, kInkFaint),
    };

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: kSurface,
        borderRadius: BorderRadius.circular(kRadiusCard),
        border: Border.all(color: kBorder),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 34,
            height: 34,
            decoration: BoxDecoration(
              shape: BoxShape.circle,
              color: color.withValues(alpha: 0.14),
            ),
            child: Icon(icon, size: 17, color: color),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        item.title,
                        style: const TextStyle(
                          color: kInk,
                          fontWeight: FontWeight.w600,
                          fontSize: 14,
                        ),
                      ),
                    ),
                    Text(
                      formatAgo(item.at),
                      style: const TextStyle(color: kInkGhost, fontSize: 11.5),
                    ),
                  ],
                ),
                const SizedBox(height: 3),
                Text(
                  item.body,
                  style: const TextStyle(color: kInkFaint, fontSize: 12.5),
                ),
              ],
            ),
          ),
          if (!item.read) ...[
            const SizedBox(width: 8),
            Container(
              width: 7,
              height: 7,
              margin: const EdgeInsets.only(top: 5),
              decoration: const BoxDecoration(
                shape: BoxShape.circle,
                color: kBrandColor,
              ),
            ),
          ],
        ],
      ),
    );
  }
}

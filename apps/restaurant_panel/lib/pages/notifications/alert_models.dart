part of '../notifications_page.dart';

String _str(Object? v) => v is String ? v : '';
int _int(Object? v) => v is num ? v.toInt() : 0;

/// Server bilan bir xil tartib (`alerts.Categories`).
const alertCategories = <(String, String)>[
  ('new', 'Yangi'),
  ('success', 'Muvaffaqiyatli'),
  ('info', 'Ma\'lumot'),
  ('important', 'Muhim'),
  ('report', 'Hisobot'),
  ('update', 'Yangilanish'),
  ('activity', 'Faoliyat'),
  ('reminder', 'Eslatma'),
];

String categoryTitle(String key) =>
    alertCategories.where((c) => c.$1 == key).map((c) => c.$2).firstOrNull ?? key;

/// (asosiy rang, fon rangi).
(Color, Color) categoryColors(String category) => switch (category) {
      'new' => (OnDexColors.primary, OnDexColors.primaryTint),
      'success' => (OnDexColors.success, OnDexColors.successBg),
      'info' => (const Color(0xFF3B82F6), const Color(0xFFE3EDFD)),
      'important' => (OnDexColors.danger, OnDexColors.dangerBg),
      'report' => (const Color(0xFF7C5CD6), const Color(0xFFEFEAFB)),
      'update' => (OnDexColors.info, OnDexColors.infoBg),
      'activity' => (const Color(0xFF22A06B), const Color(0xFFDFF3E8)),
      _ => (OnDexColors.inkDim, const Color(0xFFEFEAE3)),
    };

IconData kindIcon(String kind) => switch (kind) {
      'new_order' => Icons.shopping_cart_rounded,
      'order_cancelled' => Icons.remove_shopping_cart_rounded,
      'order_waiting' => Icons.timer_rounded,
      'payment_received' => Icons.account_balance_wallet_rounded,
      'dispatch_failed' => Icons.delivery_dining_rounded,
      'courier_not_found' => Icons.person_search_rounded,
      'staff_added' => Icons.group_add_rounded,
      'staff_status' => Icons.groups_rounded,
      'staff_position' => Icons.swap_horiz_rounded,
      'daily_report' => Icons.insights_rounded,
      'platform_update' => Icons.settings_rounded,
      _ => Icons.notifications_rounded,
    };

class AlertItem {
  const AlertItem({
    required this.id,
    required this.seq,
    required this.kind,
    required this.category,
    required this.title,
    required this.body,
    required this.read,
    required this.createdAt,
  });

  final String id;
  final int seq;
  final String kind;
  final String category;
  final String title;
  final String body;
  final bool read;
  final DateTime createdAt;

  AlertItem copyWith({bool? read}) => AlertItem(
        id: id,
        seq: seq,
        kind: kind,
        category: category,
        title: title,
        body: body,
        read: read ?? this.read,
        createdAt: createdAt,
      );

  factory AlertItem.fromJson(Map<String, dynamic> j) => AlertItem(
        id: _str(j['id']),
        seq: _int(j['seq']),
        kind: _str(j['kind']),
        category: _str(j['category']),
        title: _str(j['title']),
        body: _str(j['body']),
        read: j['read'] == true,
        createdAt: DateTime.tryParse(_str(j['created_at']))?.toLocal() ?? DateTime.now(),
      );
}

class AlertPage {
  const AlertPage({
    required this.items,
    required this.nextBefore,
    required this.total,
    required this.unread,
    required this.byCategory,
    required this.latestSeq,
  });

  final List<AlertItem> items;
  final int nextBefore;
  final int total;
  final int unread;
  final Map<String, int> byCategory;
  final int latestSeq;

  factory AlertPage.fromJson(Map<String, dynamic> j) {
    final counts = j['counts'] is Map ? j['counts'] as Map : const {};
    final by = <String, int>{};
    if (counts['by_category'] is Map) {
      (counts['by_category'] as Map).forEach((k, v) {
        if (k is String && v is num) by[k] = v.toInt();
      });
    }
    return AlertPage(
      items: [
        for (final it in (j['items'] as List? ?? const []))
          if (it is Map) AlertItem.fromJson(Map<String, dynamic>.from(it)),
      ],
      nextBefore: _int(j['next_before']),
      total: _int(counts['total']),
      unread: _int(j['unread']),
      byCategory: by,
      latestSeq: _int(j['latest_seq']),
    );
  }
}

String _two(int v) => v.toString().padLeft(2, '0');

/// Bugun — "16:42", kecha — "Kecha 16:42", aks holda "12.09 16:42".
String alertTime(DateTime at, {DateTime? now}) {
  final n = now ?? DateTime.now();
  final t = '${_two(at.hour)}:${_two(at.minute)}';
  final today = DateTime(n.year, n.month, n.day);
  final day = DateTime(at.year, at.month, at.day);
  if (day == today) return t;
  if (day == today.subtract(const Duration(days: 1))) return 'Kecha $t';
  return '${_two(at.day)}.${_two(at.month)} $t';
}

final _emphasis = RegExp(r"#[0-9A-Za-z\-]+|\d{1,3}(?: \d{3})*(?:[.,]\d+)? so'm");

/// Buyurtma raqami va summalar qalin — rasmdagidek. Faqat matn (HTML emas).
List<TextSpan> richBody(String body) {
  final spans = <TextSpan>[];
  var last = 0;
  for (final m in _emphasis.allMatches(body)) {
    if (m.start > last) spans.add(TextSpan(text: body.substring(last, m.start)));
    spans.add(TextSpan(
        text: m.group(0), style: const TextStyle(fontWeight: FontWeight.w700, color: OnDexColors.ink)));
    last = m.end;
  }
  if (last < body.length) spans.add(TextSpan(text: body.substring(last)));
  return spans;
}

/// Qurilmada saqlanadigan yozuvlar: bildirishnomalar tasmasi va
/// "men yetkazdim" tarixi.
///
/// ┌─ NEGA QURILMADA ──────────────────────────────────────────────────┐
/// Backend affitsiantga faqat FAOL buyurtmalarni beradi
/// (`routes_waiter.go` — yakunlangani ro'yxatni to'ldirib yubormasligi
/// uchun ataylab). Ya'ni "bugun nechta stolga xizmat qildim" degan
/// savolga server javob bermaydi.
///
/// Shuning uchun affitsiant "Yetkazdim" bosgan har bir buyurtma SHU
/// QURILMADA yozib boriladi. Bu — smena davomidagi shaxsiy hisob, rasmiy
/// hisobot emas: ilova o'chirilsa yoki boshqa telefonga kirilsa yozuv
/// qolmaydi. Shu cheklov ekranda ham ochiq yozilgan — foydalanuvchi buni
/// buxgalteriya hisoboti deb o'ylab qolmasligi kerak.
/// └───────────────────────────────────────────────────────────────────┘
library;

/// Bildirishnomalar tasmasidagi bitta yozuv.
class FeedItem {
  const FeedItem({
    required this.id,
    required this.kind,
    required this.title,
    required this.body,
    required this.at,
    required this.read,
  });

  final String id;

  /// `ready` — taom tayyor (eng muhim), `new` — yangi buyurtma keldi,
  /// `served` — affitsiant yetkazdi, `info` — boshqa holat o'zgarishi.
  final String kind;
  final String title;
  final String body;
  final DateTime at;
  final bool read;

  FeedItem copyWith({bool? read}) => FeedItem(
        id: id,
        kind: kind,
        title: title,
        body: body,
        at: at,
        read: read ?? this.read,
      );

  Map<String, dynamic> toJson() => {
        'id': id,
        'kind': kind,
        'title': title,
        'body': body,
        'at': at.toIso8601String(),
        'read': read,
      };

  static FeedItem? fromJson(Map<String, dynamic> j) {
    final at = DateTime.tryParse(j['at'] as String? ?? '');
    if (at == null) return null;
    return FeedItem(
      id: j['id'] as String? ?? '',
      kind: j['kind'] as String? ?? 'info',
      title: j['title'] as String? ?? '',
      body: j['body'] as String? ?? '',
      at: at,
      read: j['read'] as bool? ?? false,
    );
  }
}

/// "Yetkazdim" bosilgan buyurtma — smena tarixi uchun.
class ServedRecord {
  const ServedRecord({
    required this.orderId,
    required this.shortNumber,
    required this.tableLabel,
    required this.totalTiyin,
    required this.itemCount,
    required this.servedAt,
  });

  final String orderId;
  final String shortNumber;
  final String tableLabel;
  final int totalTiyin;
  final int itemCount;
  final DateTime servedAt;

  Map<String, dynamic> toJson() => {
        'order_id': orderId,
        'short_number': shortNumber,
        'table_label': tableLabel,
        'total_tiyin': totalTiyin,
        'item_count': itemCount,
        'served_at': servedAt.toIso8601String(),
      };

  static ServedRecord? fromJson(Map<String, dynamic> j) {
    final at = DateTime.tryParse(j['served_at'] as String? ?? '');
    if (at == null) return null;
    return ServedRecord(
      orderId: j['order_id'] as String? ?? '',
      shortNumber: j['short_number'] as String? ?? '',
      tableLabel: j['table_label'] as String? ?? '',
      totalTiyin: (j['total_tiyin'] as num?)?.toInt() ?? 0,
      itemCount: (j['item_count'] as num?)?.toInt() ?? 0,
      servedAt: at,
    );
  }

  bool get isToday {
    final now = DateTime.now();
    return servedAt.year == now.year &&
        servedAt.month == now.month &&
        servedAt.day == now.day;
  }
}

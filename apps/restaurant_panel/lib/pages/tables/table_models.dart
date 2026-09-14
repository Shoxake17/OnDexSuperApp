import 'package:flutter/material.dart';

import '../../theme.dart';

const kDefaultZone = 'Asosiy zal';

/// Joy turi tavsifi.
class TableKindInfo {
  const TableKindInfo(this.kind, this.title, this.icon, this.hint);

  final String kind;
  final String title;
  final IconData icon;
  final String hint;
}

/// Joy turlari — serverdagi `internal/tables/kind.go` bilan AYNAN bir
/// xil tartib va nomlarda. Go testi (`TestKindsMatchMigrationAndPanel`)
/// shu faylni o'qib solishtiradi: bir tomonga qo'shib ikkinchisini
/// unutish mumkin emas.
const kTableKinds = <TableKindInfo>[
  TableKindInfo('table', 'Stol', Icons.table_restaurant_rounded, 'Zal yoki ayvondagi oddiy stol'),
  TableKindInfo('cabin', 'Kabina', Icons.meeting_room_rounded, 'Yopiq yoki yarim yopiq kabina'),
  TableKindInfo('vip_room', 'VIP xona', Icons.workspace_premium_rounded, 'Alohida xizmat ko\'rsatiladigan xona'),
  TableKindInfo('tapchan', 'Topchan', Icons.deck_rounded, 'Choyxona so\'risi yoki topchan'),
  TableKindInfo('bar_counter', 'Bar stoyka', Icons.local_bar_rounded, 'Bar peshtaxtasi o\'rindiqlari'),
  TableKindInfo('lounge', 'Lounge', Icons.weekend_rounded, 'Divanli dam olish zonasi'),
  TableKindInfo('banquet_hall', 'Banket zali', Icons.celebration_rounded, 'To\'y va ziyofatlar uchun zal'),
];

/// Noma'lum tur (server yangilangan, panel eski) — oddiy stol sifatida
/// chiziladi, sahifa yiqilmaydi.
TableKindInfo tableKindInfo(String kind) =>
    kTableKinds.firstWhere((k) => k.kind == kind, orElse: () => kTableKinds.first);

/// Yangi joy formasida TAKLIF qilinadigan sig'im — foydalanuvchi ko'rib,
/// o'zgartirib saqlaydi (jimgina yozilmaydi).
int suggestedCapacity(String kind) => switch (kind) {
      'cabin' => 6,
      'vip_room' => 10,
      'tapchan' => 6,
      'bar_counter' => 2,
      'lounge' => 4,
      'banquet_hall' => 50,
      _ => 4,
    };

/// Kartochkadagi nom: stol — raqamning o'zi ("12"), boshqa turlar — tur
/// bilan ("Kabina 3"). Nom tur bilan boshlansa takrorlanmaydi. Serverdagi
/// `Table.DisplayLabel` bilan bir xil qoida.
String displayTitle(String kind, String label) {
  if (kind == 'table') return label;
  final title = tableKindInfo(kind).title;
  return label.toLowerCase().startsWith(title.toLowerCase()) ? label : '$title $label';
}

enum TableStatus { occupied, available, cleaning, inactive }

extension TableStatusView on TableStatus {
  String get title => switch (this) {
        TableStatus.occupied => 'Band',
        TableStatus.available => 'Bo\'sh',
        TableStatus.cleaning => 'Tozalanmoqda',
        TableStatus.inactive => 'Yopiq',
      };

  Color get color => switch (this) {
        TableStatus.occupied => OnDexColors.success,
        TableStatus.available => OnDexColors.info,
        TableStatus.cleaning => const Color(0xFF6B6259),
        TableStatus.inactive => OnDexColors.danger,
      };

  Color get background => switch (this) {
        TableStatus.occupied => OnDexColors.successBg,
        TableStatus.available => OnDexColors.infoBg,
        TableStatus.cleaning => const Color(0xFFEFEAE4),
        TableStatus.inactive => OnDexColors.dangerBg,
      };

  IconData get icon => switch (this) {
        TableStatus.occupied => Icons.restaurant_rounded,
        TableStatus.available => Icons.event_seat_rounded,
        TableStatus.cleaning => Icons.cleaning_services_rounded,
        TableStatus.inactive => Icons.block_rounded,
      };
}

TableStatus parseTableStatus(Object? v) => switch (v) {
      'occupied' => TableStatus.occupied,
      'cleaning' => TableStatus.cleaning,
      'inactive' => TableStatus.inactive,
      _ => TableStatus.available,
    };

// JSON yordamchilari hech qachon xato tashlamaydi: kutilmagan tur
// sahifani "internet yo'q" degan chalg'ituvchi xabarga aylantirmasin.
String _str(Object? v) => v is String ? v : '';
int _int(Object? v) => v is num ? v.toInt() : 0;
int? _intOrNull(Object? v) => v is num ? v.toInt() : null;
DateTime? _time(Object? v) => v is String ? DateTime.tryParse(v)?.toLocal() : null;

/// Joyga bog'langan buyurtma (mijoz ma'lumotisiz).
class TableOrderSnap {
  const TableOrderSnap({
    required this.id,
    required this.orderNumber,
    required this.status,
    required this.items,
    required this.totalTiyin,
    required this.createdAt,
  });

  final String id;
  final String orderNumber;
  final String status;
  final int items;
  final int totalTiyin;
  final DateTime? createdAt;

  /// "#1024" — buyurtma raqamining oxirgi bo'lagi.
  String get shortNumber =>
      orderNumber.isEmpty ? '' : '#${orderNumber.split('-').last}';

  factory TableOrderSnap.fromJson(Map<String, dynamic> j) => TableOrderSnap(
        id: _str(j['id']),
        orderNumber: _str(j['order_number']),
        status: _str(j['status']),
        items: _int(j['items']),
        totalTiyin: _int(j['total_tiyin']),
        createdAt: _time(j['created_at']),
      );
}

class DiningTable {
  const DiningTable({
    required this.id,
    required this.zone,
    required this.kind,
    required this.label,
    required this.displayLabel,
    required this.capacity,
    required this.active,
    required this.qrToken,
    required this.qrLink,
    required this.status,
    required this.cleaningSince,
    required this.lastScannedAt,
    required this.createdAt,
    required this.activeOrders,
    required this.lastOrder,
  });

  final String id;
  final String zone;
  final String kind;
  final String label;
  final String displayLabel;
  final int? capacity;
  final bool active;
  final String qrToken;

  /// `null` — serverda Telegram bot sozlanmagan, QR chizib bo'lmaydi.
  final String? qrLink;
  final TableStatus status;
  final DateTime? cleaningSince;
  final DateTime? lastScannedAt;
  final DateTime? createdAt;
  final List<TableOrderSnap> activeOrders;
  final TableOrderSnap? lastOrder;

  TableKindInfo get kindInfo => tableKindInfo(kind);

  String get title => displayTitle(kind, label);

  TableOrderSnap? get currentOrder => activeOrders.isEmpty ? null : activeOrders.first;

  factory DiningTable.fromJson(Map<String, dynamic> j) {
    final zone = _str(j['zone']).trim();
    final label = _str(j['label']);
    final kind = _str(j['kind']).isEmpty ? 'table' : _str(j['kind']);
    final resolvedZone = zone.isEmpty ? kDefaultZone : zone;
    final link = _str(j['qr_link']);
    final last = j['last_order'];
    return DiningTable(
      id: _str(j['id']),
      zone: resolvedZone,
      kind: kind,
      label: label,
      displayLabel: _str(j['display_label']).isEmpty
          ? '$resolvedZone · ${displayTitle(kind, label)}'
          : _str(j['display_label']),
      capacity: _intOrNull(j['capacity']),
      active: j['active'] == true,
      qrToken: _str(j['qr_token']),
      qrLink: link.isEmpty ? null : link,
      status: parseTableStatus(j['status']),
      cleaningSince: _time(j['cleaning_since']),
      lastScannedAt: _time(j['last_scanned_at']),
      createdAt: _time(j['created_at']),
      activeOrders: [
        for (final o in (j['active_orders'] is List ? j['active_orders'] as List : const []))
          if (o is Map) TableOrderSnap.fromJson(Map<String, dynamic>.from(o)),
      ],
      lastOrder: last is Map ? TableOrderSnap.fromJson(Map<String, dynamic>.from(last)) : null,
    );
  }

  /// Qidiruv: raqam/nom, zal, tur, buyurtma raqami yoki QR havolasi
  /// (skanerlab olingan havolani joylashtirib qaysi joy ekanini topish).
  bool matches(String query) {
    final q = query.trim().toLowerCase();
    if (q.isEmpty) return true;
    if (title.toLowerCase().contains(q) ||
        displayLabel.toLowerCase().contains(q) ||
        kindInfo.title.toLowerCase().contains(q)) {
      return true;
    }
    if (activeOrders.any((o) => o.orderNumber.toLowerCase().contains(q))) return true;
    // QR bo'yicha — faqat yetarlicha uzun bo'lak: "1" deb yozilganda
    // tokenida "1" bor hamma joy chiqib ketmasin.
    return q.length >= 8 && (qrToken.contains(q) || (qrLink?.toLowerCase().contains(q) ?? false));
  }
}

/// Tabiiy tartib: "2" < "10", "A-9" < "A-10".
int compareLabels(String a, String b) {
  final re = RegExp(r'\d+|\D+');
  final pa = re.allMatches(a.toLowerCase()).map((m) => m.group(0)!).toList();
  final pb = re.allMatches(b.toLowerCase()).map((m) => m.group(0)!).toList();
  for (var i = 0; i < pa.length && i < pb.length; i++) {
    final na = int.tryParse(pa[i]);
    final nb = int.tryParse(pb[i]);
    final c = na != null && nb != null ? na.compareTo(nb) : pa[i].compareTo(pb[i]);
    if (c != 0) return c;
  }
  return pa.length.compareTo(pb.length);
}

int compareZones(String a, String b) {
  if (a == b) return 0;
  if (a == kDefaultZone) return -1;
  if (b == kDefaultZone) return 1;
  return a.toLowerCase().compareTo(b.toLowerCase());
}

/// Zal ("Asosiy zal" birinchi) → tur (ro'yxat tartibida) → nom.
int compareTables(DiningTable a, DiningTable b) {
  final z = compareZones(a.zone, b.zone);
  if (z != 0) return z;
  final ka = kTableKinds.indexWhere((k) => k.kind == a.kind);
  final kb = kTableKinds.indexWhere((k) => k.kind == b.kind);
  if (ka != kb) return ka.compareTo(kb);
  return compareLabels(a.label, b.label);
}

/// Restoran joyi (stol, kabina, VIP xona...) — affitsiant ko'radigan
/// ko'rinish (`GET /waiter/tables`).
///
/// QR token bu modelda UMUMAN yo'q: server uni affitsiantga bermaydi
/// (`routes_waiter.go` — `waiterTableView`).
library;

class WaiterTable {
  const WaiterTable({
    required this.id,
    required this.zone,
    required this.kind,
    required this.kindTitle,
    required this.label,
    required this.displayLabel,
    required this.capacity,
    required this.active,
    required this.status,
    required this.activeOrders,
  });

  final String id;
  final String zone;
  final String kind;
  final String kindTitle;
  final String label;

  /// To'liq nom: "Asosiy zal · Kabina 3" — buyurtmaga shu yoziladi.
  final String displayLabel;
  final int? capacity;
  final bool active;

  /// Serverdagi `tables.Status`: available | occupied | cleaning | inactive.
  final String status;
  final int activeOrders;

  bool get isOccupied => status == 'occupied';
  bool get isCleaning => status == 'cleaning';
  bool get isInactive => status == 'inactive';

  /// Buyurtma kiritsa bo'ladimi. Yopiq joyga server ham rad etadi —
  /// bu faqat tugmani oldindan o'chirish uchun.
  bool get canOrder => active;

  /// Zal ichidagi qisqa nom: stol uchun raqam, boshqa turlarda tur
  /// nomi bilan ("Kabina 3"). Zal nomi bo'lim sarlavhasida turadi.
  String get shortName {
    if (kind == 'table' || kind.isEmpty) return label;
    final lower = label.toLowerCase();
    if (kindTitle.isNotEmpty && !lower.startsWith(kindTitle.toLowerCase())) {
      return '$kindTitle $label';
    }
    return label;
  }

  factory WaiterTable.fromJson(Map<String, dynamic> j) {
    final cap = j['capacity'];
    return WaiterTable(
      id: j['id'] as String? ?? '',
      zone: j['zone'] as String? ?? '',
      kind: j['kind'] as String? ?? 'table',
      kindTitle: j['kind_title'] as String? ?? '',
      label: j['label'] as String? ?? '',
      displayLabel: j['display_label'] as String? ?? '',
      capacity: cap is int ? cap : (cap is double ? cap.toInt() : null),
      active: j['active'] as bool? ?? false,
      status: j['status'] as String? ?? 'available',
      activeOrders: j['active_orders'] is int ? j['active_orders'] as int : 0,
    );
  }
}

/// Menyudagi bitta taom (`GET /restaurants/{id}/menu`, ochiq javob).
class MenuProduct {
  const MenuProduct({
    required this.id,
    required this.name,
    required this.category,
    required this.priceTiyin,
    required this.discountPriceTiyin,
    required this.imageUrl,
    required this.available,
  });

  final String id;
  final String name;
  final String category;
  final int priceTiyin;
  final int discountPriceTiyin;
  final String imageUrl;
  final bool available;

  /// Ko'rsatiladigan narx. Yakuniy summani baribir SERVER hisoblaydi
  /// (aksiyalar ham qo'llanadi) — bu faqat taxminiy ko'rsatkich.
  int get unitPriceTiyin =>
      discountPriceTiyin > 0 && discountPriceTiyin < priceTiyin
          ? discountPriceTiyin
          : priceTiyin;

  factory MenuProduct.fromJson(Map<String, dynamic> j) {
    int asInt(dynamic v) => v is int ? v : (v is double ? v.toInt() : 0);
    return MenuProduct(
      id: j['id'] as String? ?? '',
      name: j['name'] as String? ?? '',
      category: j['category'] as String? ?? '',
      priceTiyin: asInt(j['price_tiyin']),
      discountPriceTiyin: asInt(j['discount_price_tiyin']),
      imageUrl: j['image_url'] as String? ?? '',
      available: j['available'] as bool? ?? false,
    );
  }
}

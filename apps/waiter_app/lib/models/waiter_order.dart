/// Affitsiant ko'radigan buyurtma — TIPLASHTIRILGAN model.
///
/// ┌─ NEGA Map EMAS ───────────────────────────────────────────────────┐
/// Ilgari ekranlar to'g'ridan-to'g'ri `Map<String, dynamic>` bilan
/// ishlardi: `o['status']`, `o['table_label'] as String?`. Bunda
/// serverdagi maydon nomi o'zgarsa yoki tur mos kelmasa — xato
/// KOMPILYATSIYADA emas, foydalanuvchi ekranida chiqadi (bo'sh joy yoki
/// qulash). Bitta joyda parse qilinsa, qolgan hamma joy xavfsiz bo'ladi.
/// └───────────────────────────────────────────────────────────────────┘
library;

class WaiterOrderItem {
  const WaiterOrderItem({
    required this.name,
    required this.qty,
    required this.priceTiyin,
  });

  final String name;
  final int qty;
  final int priceTiyin;

  /// Shu qator uchun jami (miqdor × narx).
  int get lineTotalTiyin => priceTiyin * qty;

  factory WaiterOrderItem.fromJson(Map<String, dynamic> j) {
    // `discount_price_tiyin` bo'lsa mijoz AYNAN shuni to'lagan —
    // affitsiant ko'radigan raqam chekdagi raqam bilan bir xil
    // bo'lishi shart, aks holda stolda summa bo'yicha bahs chiqadi.
    final discount = _int(j['discount_price_tiyin']);
    final price = _int(j['price_tiyin']);
    return WaiterOrderItem(
      name: j['name'] as String? ?? '—',
      qty: _int(j['qty']),
      priceTiyin: discount > 0 ? discount : price,
    );
  }
}

class WaiterOrder {
  const WaiterOrder({
    required this.id,
    required this.orderNumber,
    required this.tableId,
    required this.tableLabel,
    required this.partySize,
    required this.items,
    required this.totalTiyin,
    required this.status,
    required this.createdAt,
    required this.readyAt,
    this.placedByWaiter = false,
  });

  final String id;
  final String orderNumber;

  /// Joy ID'si — stol sahifasida buyurtmalar SHU bo'yicha guruhlanadi
  /// (nom qayta nomlanishi mumkin, ID o'zgarmaydi).
  final String tableId;
  final String tableLabel;
  final int partySize;
  final List<WaiterOrderItem> items;
  final int totalTiyin;
  final String status;
  final DateTime? createdAt;
  final DateTime? readyAt;

  /// Buyurtmani affitsiant kiritgan (mijoz QR orqali emas).
  final bool placedByWaiter;

  /// Affitsiant harakat qilishi kerakmi (yagona harakat — "Yetkazdim").
  bool get isReady => status == 'ready';

  /// Oshxonada, kutilmoqda.
  bool get isPreparing => status == 'preparing';

  /// Hali oshxona qabul qilmagan / endi qabul qilgan.
  bool get isNew => status == 'created' || status == 'accepted';

  /// Buyurtmadagi jami mahsulot soni (2× osh + 1× salat = 3).
  int get itemCount => items.fold(0, (sum, it) => sum + it.qty);

  /// Ro'yxatda ko'rsatiladigan qisqa raqam.
  ///
  /// `order_number` "300726-0000123" ko'rinishida — telefon ekranida
  /// to'liq ko'rsatish joy egallaydi va o'qilmaydi, shuning uchun
  /// oxirgi bo'lagi olinadi (kuryer ilovasida ham shu naqsh:
  /// restoranda buyurtma oxirgi raqamlar bo'yicha topiladi).
  String get shortNumber {
    final parts = orderNumber.split('-');
    final tail = parts.isNotEmpty ? parts.last : orderNumber;
    final trimmed = tail.replaceFirst(RegExp(r'^0+'), '');
    return '#${trimmed.isEmpty ? tail : trimmed}';
  }

  /// "Tayyor" bo'lganiga qancha vaqt o'tdi — taom sovimoqda.
  ///
  /// Affitsiant uchun eng muhim raqam shu: 1 daqiqa oldin tayyor
  /// bo'lgan va 15 daqiqa oldin tayyor bo'lgan buyurtma bir xil
  /// ko'rinmasligi kerak.
  Duration? get waitingSinceReady {
    final r = readyAt;
    if (r == null || !isReady) return null;
    final d = DateTime.now().difference(r);
    return d.isNegative ? Duration.zero : d;
  }

  factory WaiterOrder.fromJson(Map<String, dynamic> j) {
    final rawItems = j['items'];
    return WaiterOrder(
      id: j['id'] as String? ?? '',
      orderNumber: j['order_number'] as String? ?? '',
      tableId: j['table_id'] as String? ?? '',
      tableLabel: j['table_label'] as String? ?? '',
      partySize: _int(j['party_size']),
      items: rawItems is List
          ? rawItems
                .whereType<Map>()
                .map((e) => WaiterOrderItem.fromJson(
                      Map<String, dynamic>.from(e),
                    ))
                .toList()
          : const [],
      totalTiyin: _int(j['total_tiyin']),
      status: j['status'] as String? ?? '',
      createdAt: _time(j['created_at']),
      readyAt: _time(j['ready_at']),
      placedByWaiter: j['placed_by_waiter'] == true,
    );
  }
}

/// Bitta stol — o'sha stolning barcha FAOL buyurtmalari.
///
/// Buyurtmalar ekranidagi guruhlash uchun. "Stollar" bo'limi esa endi
/// restoranning BARCHA joylarini serverdan oladi (`WaiterTable`).
class TableGroup {
  TableGroup({required this.label, required this.orders});

  final String label;
  final List<WaiterOrder> orders;

  /// Stolda tayyor, lekin hali olib borilmagan buyurtma bormi.
  bool get hasReady => orders.any((o) => o.isReady);

  /// Oshxonada tayyorlanayotgani bormi.
  bool get hasPreparing => orders.any((o) => o.isPreparing);

  /// Stoldagi odamlar soni — buyurtmalardagi eng katta qiymat
  /// (bir stolga ikki marta buyurtma berilsa, ikkinchisida son 0
  /// bo'lishi mumkin).
  int get partySize =>
      orders.fold(0, (m, o) => o.partySize > m ? o.partySize : m);

  /// Stolning faol buyurtmalari jami summasi.
  int get totalTiyin => orders.fold(0, (s, o) => s + o.totalTiyin);

  /// Eng uzoq kutayotgan tayyor buyurtma vaqti — stollar ro'yxatini
  /// shu bo'yicha tartiblaymiz (eng ko'p kutgan tepada).
  Duration? get longestReadyWait {
    Duration? worst;
    for (final o in orders) {
      final w = o.waitingSinceReady;
      if (w == null) continue;
      if (worst == null || w > worst) worst = w;
    }
    return worst;
  }

  /// Buyurtmalar ro'yxatidan stol bo'yicha guruhlar yasaydi.
  ///
  /// Tartib: avval tayyor kutayotganlar (eng uzoq kutgani birinchi),
  /// keyin tayyorlanayotganlar, keyin qolganlari. Sabab — affitsiant
  /// ekranga qaraganda birinchi ko'rgan narsasi "hozir nima qilishim
  /// kerak" bo'lishi kerak, stol raqami emas.
  static List<TableGroup> from(List<WaiterOrder> orders) {
    final byLabel = <String, List<WaiterOrder>>{};
    for (final o in orders) {
      final key = o.tableLabel.isEmpty ? '—' : o.tableLabel;
      byLabel.putIfAbsent(key, () => []).add(o);
    }
    final groups = byLabel.entries
        .map((e) => TableGroup(label: e.key, orders: e.value))
        .toList();

    groups.sort((a, b) {
      if (a.hasReady != b.hasReady) return a.hasReady ? -1 : 1;
      if (a.hasReady && b.hasReady) {
        final aw = a.longestReadyWait ?? Duration.zero;
        final bw = b.longestReadyWait ?? Duration.zero;
        final cmp = bw.compareTo(aw);
        if (cmp != 0) return cmp;
      }
      if (a.hasPreparing != b.hasPreparing) return a.hasPreparing ? -1 : 1;
      return compareLabels(a.label, b.label);
    });
    return groups;
  }

  /// Nomlarni INSON tartibida solishtiradi: "Stol 2" < "Stol 10".
  /// Oddiy matn solishtiruvida "10" "2" dan oldin kelib qolardi.
  static int compareLabels(String a, String b) {
    final na = int.tryParse(RegExp(r'\d+').firstMatch(a)?.group(0) ?? '');
    final nb = int.tryParse(RegExp(r'\d+').firstMatch(b)?.group(0) ?? '');
    if (na != null && nb != null && na != nb) return na.compareTo(nb);
    return a.toLowerCase().compareTo(b.toLowerCase());
  }
}

int _int(dynamic v) {
  if (v is int) return v;
  if (v is double) return v.toInt();
  if (v is String) return int.tryParse(v) ?? 0;
  return 0;
}

DateTime? _time(dynamic v) {
  if (v is! String || v.isEmpty) return null;
  // Server UTC beradi — `toLocal()` shart, aks holda "5 daqiqa oldin"
  // o'rniga "5 soat oldin" ko'rinardi.
  return DateTime.tryParse(v)?.toLocal();
}

import 'package:flutter/foundation.dart';

import '../api.dart';

/// Shaddiy qaysi amallarni bajara olishi — foydalanuvchi ruxsatlari.
///
/// ┌─ RO'YXAT SERVERDAN KELADI ─────────────────────────────────────────┐
/// Amal nomlari bu yerda QO'LDA yozilmaydi: backend'da yangi amal
/// qo'shilsa u shu zahoti ro'yxatda paydo bo'ladi. Aks holda yangi
/// imkoniyat ilovada ko'rinmasdi va foydalanuvchi uni boshqara
/// olmasdi — ya'ni "ruxsatlar" ro'yxati to'liq bo'lmasdi.
///
/// Ko'rsatiladigan NOM va IZOH esa ilovada: ular foydalanuvchi tili,
/// backend'ning ichki nomlari emas. Notanish amal uchun nomning o'zi
/// ko'rsatiladi — u yashirilib qolmasligi kerak.
/// └────────────────────────────────────────────────────────────────────┘
///
/// ┌─ TEKSHIRUV SERVERDA ───────────────────────────────────────────────┐
/// Bu sinf faqat KO'RINISH va o'zgartirish uchun. O'chirilgan amal
/// serverda modelga umuman e'lon qilinmaydi va chaqirilsa ham
/// bajarilmaydi (`internal/assistant/tools.go`). Ya'ni ilovani
/// o'zgartirgan odam ham o'chirilgan amalni qayta yoqa olmaydi.
/// └────────────────────────────────────────────────────────────────────┘
class AiTools extends ChangeNotifier {
  AiTools._();
  static final AiTools instance = AiTools._();

  /// Amal nomi -> yoqilganmi.
  Map<String, bool> _state = {};
  Map<String, bool> get state => Map.unmodifiable(_state);

  bool _loading = false;
  bool get loading => _loading;

  String? _error;
  String? get error => _error;

  /// Yuklab bo'lingan bo'lsa `true` — ro'yxat bo'sh bo'lishi ham
  /// mumkin (server eski bo'lsa), bu holat xatodan farq qiladi.
  bool _loaded = false;
  bool get loaded => _loaded;

  /// Ilova ekranlar bo'ylab yurib, buyurtmani to'lov ekranigacha
  /// olib bora oladimi.
  ///
  /// ┌─ RUXSAT NOMA'LUM BO'LSA — RUXSAT YO'Q ─────────────────────────┐
  /// Ro'yxat hali yuklanmagan bo'lsa `false` qaytadi. Teskarisi
  /// xavfli bo'lardi: tarmoq sekin bo'lgani uchun ilova o'z-o'zidan
  /// yura boshlagan bo'lardi — foydalanuvchi esa buni O'CHIRIB
  /// qo'ygan bo'lishi mumkin.
  /// └────────────────────────────────────────────────────────────────┘
  static const capCheckout = 'checkout_flow';
  bool get checkoutAllowed => _loaded && (_state[capCheckout] ?? false);

  Future<void> load() async {
    _loading = true;
    _error = null;
    notifyListeners();
    try {
      final list = await api.aiTools();
      final next = <String, bool>{};
      for (final t in list) {
        final name = (t['name'] ?? '').toString();
        if (name.isEmpty) continue;
        next[name] = t['enabled'] != false;
      }
      _state = next;
      _loaded = true;
    } catch (e) {
      _error = errorText(e, 'Ruxsatlarni yuklab bo\'lmadi.');
    } finally {
      _loading = false;
      notifyListeners();
    }
  }

  /// Bitta amalni yoqadi/o'chiradi.
  ///
  /// Serverga BUTUN ro'yxat yuboriladi (yoqilganlar), chunki qisman
  /// yangilash ikki qurilma bir vaqtda o'zgartirganda holatni
  /// chalkashtirardi.
  ///
  /// Xato bo'lsa o'zgarish QAYTARILADI: ekranda yoqilgan, serverda
  /// o'chiq holat eng yomon variant bo'lardi — foydalanuvchi
  /// ruxsat berdim deb o'ylardi.
  Future<void> setEnabled(String name, bool enabled) async {
    final prev = _state[name];
    if (prev == null) return;
    _state = {..._state, name: enabled};
    _error = null;
    notifyListeners();

    try {
      await api.setAiTools(
        _state.entries.where((e) => e.value).map((e) => e.key).toList(),
      );
    } catch (e) {
      _state = {..._state, name: prev};
      _error = errorText(e, 'Saqlanmadi — qayta urining.');
      notifyListeners();
    }
  }
}

/// Amalning foydalanuvchi tilidagi nomi va izohi.
///
/// Izohlar AYNAN nima bo'lishini aytadi: "ruxsat" degan so'z faqat
/// oqibati tushunarli bo'lgandagina ma'noga ega.
class AiToolInfo {
  const AiToolInfo(this.title, this.subtitle);
  final String title;
  final String subtitle;

  static const _map = <String, AiToolInfo>{
    'search_food': AiToolInfo(
      'Taom qidirish',
      'Barcha restoranlardan taom topadi. O\'chirilsa Shaddiy taom '
          'nomini eshitsa ham hech narsa topa olmaydi.',
    ),
    'list_restaurants': AiToolInfo(
      'Restoranlar ro\'yxati',
      'Ochiq restoranlarni ko\'radi.',
    ),
    'restaurant_menu': AiToolInfo(
      'Restoran menyusi',
      'Bitta restoranning taomlari va narxlarini o\'qiydi.',
    ),
    'propose_order': AiToolInfo(
      'Savat tayyorlash',
      'Taomlarni tanlab, jami summani hisoblaydi va buyurtmani '
          'to\'lov ekranigacha olib boradi. Buyurtmani baribir SIZ '
          'tasdiqlaysiz.',
    ),
    'my_orders': AiToolInfo(
      'Buyurtmalarim',
      'Oxirgi buyurtmalaringiz va ularning holatini ko\'radi.',
    ),
    'cancel_order': AiToolInfo(
      'Buyurtmani bekor qilish',
      'Hali tayyorlanmagan buyurtmani bekor qila oladi.',
    ),
    'checkout_flow': AiToolInfo(
      'Rasmiylashtirish va to\'lov',
      'Shaddiy menyuni ochib, taomlarni savatga qo\'shib, sizni to\'lov '
          'ekranigacha olib boradi — hammasi ekranda ko\'rinadi va '
          'istalgan payt to\'xtatiladi. To\'lovni SIZ tasdiqlaysiz. '
          'O\'chirilsa Shaddiy faqat savat taklifini aytadi.',
    ),
  };

  static AiToolInfo of(String name) =>
      _map[name] ?? AiToolInfo(name, 'Yordamchining amali.');
}

/// Ruxsatlar QAYSI XIZMATGA tegishli.
///
/// ┌─ NEGA GURUHLANADI ─────────────────────────────────────────────────┐
/// Hozir OnDex'da faqat restoran/kafe ishlaydi, lekin bosh sahifada
/// "Do'kon", "Computer Club", "Uy Joy", "Taksi" ham turibdi. Ular
/// qo'shilganda yordamchiga yangi amallar keladi va ruxsatlar bitta
/// tekis ro'yxatga aralashib ketardi — foydalanuvchi qaysi ruxsat
/// qaysi xizmat uchun ekanini ajrata olmasdi.
///
/// Shuning uchun guruh HOZIRDAN kiritiladi: keyin qo'shish emas,
/// keyin AJRATISH qiyin bo'ladi (odam allaqachon o'rgangan tartib
/// buziladi).
/// └────────────────────────────────────────────────────────────────────┘
class AiToolGroup {
  const AiToolGroup(this.title, this.subtitle);
  final String title;
  final String subtitle;

  static const restaurant = AiToolGroup(
    'Restoran va kafe',
    'Taom buyurtma qilish bilan bog\'liq amallar.',
  );
  static const other = AiToolGroup(
    'Boshqa',
    'Yangi xizmat amallari.',
  );

  /// Amal qaysi guruhga tegishli.
  ///
  /// Notanish amal "Boshqa" ga tushadi va YASHIRILMAYDI: backend'da
  /// yangi amal paydo bo'lsa, foydalanuvchi uni baribir ko'radi va
  /// boshqara oladi.
  static AiToolGroup of(String name) {
    const restaurantTools = {
      'search_food',
      'list_restaurants',
      'restaurant_menu',
      'propose_order',
      'my_orders',
      'cancel_order',
      'checkout_flow',
      'confirm_order',
    };
    return restaurantTools.contains(name) ? restaurant : other;
  }

  /// Ro'yxatni guruhlarga ajratadi (tartib saqlanadi).
  static List<MapEntry<AiToolGroup, List<String>>> split(List<String> names) {
    final byGroup = <AiToolGroup, List<String>>{};
    for (final n in names) {
      byGroup.putIfAbsent(of(n), () => []).add(n);
    }
    // Restoran BIRINCHI: hozir yagona ishlaydigan xizmat.
    final out = <MapEntry<AiToolGroup, List<String>>>[];
    if (byGroup[restaurant] != null) {
      out.add(MapEntry(restaurant, byGroup[restaurant]!));
    }
    for (final e in byGroup.entries) {
      if (e.key != restaurant) out.add(MapEntry(e.key, e.value));
    }
    return out;
  }
}

import 'dart:async';

import 'package:flutter/material.dart';

import '../screens/cart_screen.dart';
import '../screens/menu_screen.dart';
import 'ai_tools.dart';

/// Ilovaning ILDIZ navigatori.
///
/// Yordamchi ekrani yopilgandan keyin ham yo'l ochish kerak, ya'ni
/// o'sha ekranning `BuildContext` iga tayanib bo'lmaydi — u
/// `dispose` bo'ladi.
final appNavigatorKey = GlobalKey<NavigatorState>();

/// Ekranlarning "boshqarish nuqtalari" ro'yxati.
///
/// ┌─ NEGA REGISTR KERAK ───────────────────────────────────────────────┐
/// Shaddiy buyurtmani odam qanday bersa, xuddi shunday berishi kerak:
/// menyuni ochib, taomni topib, "+" ni bosib, savatdan rasmiylashtirish
/// tugmasini bosib.
///
/// Buning uchun boshqaruvchi ekranlarning ICHIGA kira olmaydi — ular
/// o'zlari "mana mening skrollim, mana taomlarning joylari, mana
/// qo'shish amali" deb ro'yxatdan o'tadi. Shu bilan boshqaruvchi
/// ekranlarning ichki tuzilishiga bog'lanmaydi.
///
/// ENG MUHIMI: bu yerda ro'yxatdan o'tadigan amallar — AYNAN
/// foydalanuvchi bosganda chaqiriladigan amallar. Boshqaruvchi yangi
/// yo'l ochmaydi, faqat mavjud tugmalarni bosadi. Shuning uchun u
/// buyurtma ham yarata olmaydi, pul ham sarflay olmaydi: to'lov
/// ekranida to'xtaydi.
/// └────────────────────────────────────────────────────────────────────┘
class AgentStage {
  AgentStage._();
  static final AgentStage instance = AgentStage._();

  // ── Menyu ekrani ──
  String? menuRestaurantId;
  ScrollController? menuScroll;

  /// Taom kartochkalarining joylari — skroll va "barmoq" uchun.
  final Map<String, GlobalKey> productKeys = {};

  /// Taomni savatga qo'shish — menyu ekranidagi "+" bilan AYNAN bir xil.
  void Function(String productId)? addProduct;

  void registerMenu({
    required String restaurantId,
    required ScrollController scroll,
    required void Function(String productId) add,
  }) {
    menuRestaurantId = restaurantId;
    menuScroll = scroll;
    addProduct = add;
    productKeys.clear();
    addKeys.clear();
  }

  void unregisterMenu(String restaurantId) {
    if (menuRestaurantId != restaurantId) return;
    menuRestaurantId = null;
    menuScroll = null;
    addProduct = null;
    productKeys.clear();
    addKeys.clear();
  }

  /// Taom kartochkasining kaliti — `menu_screen` har chizishda beradi.
  /// SKROLL uchun ishlatiladi (butun kartochka ko'rinishi kerak).
  GlobalKey productKey(String productId) =>
      productKeys.putIfAbsent(productId, () => GlobalKey());

  /// "+" tugmalarining kalitlari — BOSISH aynan shu joyda bo'ladi.
  final Map<String, GlobalKey> addKeys = {};

  /// Taomning "+" tugmasi kaliti.
  ///
  /// Kartochka kaliti bilan ALMASHTIRIB bo'lmaydi: kartochka markazi
  /// miqdor raqamining ustiga to'g'ri keladi va "barmoq" noto'g'ri
  /// tugmani bosayotgandek ko'rinardi.
  GlobalKey addKey(String productId) =>
      addKeys.putIfAbsent(productId, () => GlobalKey());

  // ── Savat ekrani ──

  /// "Rasmiylashtirish" tugmasi — FAQAT narx hisoblanib, tugma faol
  /// bo'lganda ro'yxatdan o'tadi.
  VoidCallback? cartCheckout;
  GlobalKey? cartCheckoutKey;

  void registerCartCheckout(VoidCallback? onTap, GlobalKey key) {
    cartCheckout = onTap;
    cartCheckoutKey = key;
  }

  void unregisterCartCheckout() {
    cartCheckout = null;
    cartCheckoutKey = null;
  }

  // ── Rasmiylashtirish ekrani ──

  /// Naqd to'lov bilan buyurtma berish — checkout ekranidagi AYNAN
  /// o'sha tugma. Tugma tayyor bo'lmasa `null`.
  VoidCallback? cashConfirm;
  GlobalKey? cashConfirmKey;

  void registerCashConfirm(VoidCallback? onTap, GlobalKey key) {
    cashConfirm = onTap;
    cashConfirmKey = key;
  }

  void unregisterCashConfirm() {
    cashConfirm = null;
    cashConfirmKey = null;
  }
}

/// Shaddiy buyurtmani KO'RSATIB beradigan boshqaruvchi.
///
/// ┌─ XAVFSIZLIK CHEGARALARI ───────────────────────────────────────────┐
///  1. Faqat foydalanuvchining o'z tugmalari bosiladi. Boshqaruvchi
///     `api.createOrder` ni CHAQIRMAYDI va to'lovga tegmaydi.
///  2. To'lov ekranida TO'XTAYDI. Oxirgi tugmani — pul sarflaydigan
///     yagona tugmani — odam bosadi.
///  3. Taom va restoran ID lari serverdan kelgan taklifdan olinadi
///     (`propose_order`), ya'ni narx ham, mavjudlik ham allaqachon
///     serverda tekshirilgan.
///  4. Istalgan payt to'xtatiladi: lentadagi tugma bilan ham, ekranga
///     tegish bilan ham. Har qadamda bekor qilinganlik tekshiriladi.
///  5. Hech narsa "ko'rinmas" bajarilmaydi: har amal ekranda, odam
///     ko'radigan tezlikda ketadi.
/// └────────────────────────────────────────────────────────────────────┘
class AgentDriver extends ChangeNotifier {
  AgentDriver._();
  static final AgentDriver instance = AgentDriver._();

  final _stage = AgentStage.instance;

  bool _running = false;
  bool get running => _running;

  bool _cancelled = false;

  /// Foydalanuvchiga ko'rsatiladigan hozirgi qadam.
  String _status = '';
  String get status => _status;

  /// "Barmoq" ning ekrandagi joyi. `null` — ko'rsatilmaydi.
  Offset? _pointer;
  Offset? get pointer => _pointer;

  /// Barmoq AYNAN hozir bosyaptimi (halqa animatsiyasi).
  bool _tapping = false;
  bool get tapping => _tapping;

  /// Rasmiylashtirish ekraniga yetib borildi.
  ///
  /// ┌─ NEGA CALLBACK, ICHKARIDA CHAQIRUV EMAS ───────────────────────┐
  /// Boshqaruvchi ovozli seans haqida hech narsa bilmaydi — u faqat
  /// ekranlarni boshqaradi. Ovoz esa `AiLive` da. Ular bir-biriga
  /// to'g'ridan-to'g'ri bog'lansa, boshqaruvni ovozsiz (matnli
  /// rejimda) ishlatib bo'lmasdi.
  ///
  /// Shu sabab boshqaruvchi shunchaki "yetib bordim" deb aytadi,
  /// nima qilish kerakligini esa yordamchi ekrani hal qiladi.
  /// └────────────────────────────────────────────────────────────────┘
  void Function(String restaurantName, int totalTiyin)? onCheckoutReady;

  /// Odam ekranga tegdi — boshqaruv unga qaytadi.
  ///
  /// Bu shunchaki qulaylik emas, XAVFSIZLIK qoidasi: foydalanuvchi
  /// istagan payt jarayonni to'xtata olishi kerak va buning eng tabiiy
  /// usuli — ekranga tegish.
  void userTouched() {
    if (_running) cancel();
  }

  void cancel() {
    if (!_running) return;
    _cancelled = true;
    _set('To\'xtatildi', pointer: null);
  }

  void _set(String status, {Offset? pointer, bool tapping = false}) {
    _status = status;
    _pointer = pointer;
    _tapping = tapping;
    notifyListeners();
  }

  /// Taklifni bajaradi: menyuni ochib, taomlarni qo'shib, savatdan
  /// to'lov ekraniga olib boradi.
  ///
  /// Xato tashlamaydi — nosozlik `status` da ko'rinadi va jarayon
  /// to'xtaydi. Foydalanuvchi qolgan qadamlarni o'zi bajara oladi,
  /// chunki u odatdagi ekranlarda turadi.
  Future<void> run(Map<String, dynamic> proposal) async {
    if (_running) return;

    // ★ RUXSAT. Foydalanuvchi "Rasmiylashtirish va to'lov" ni
    // o'chirgan bo'lsa ilova ekranlar bo'ylab YURMAYDI — taklif
    // kartasi suhbatda qoladi va u odatdagidek o'zi davom etadi.
    //
    // Ruxsat serverda saqlanadi (`/me/ai-tools`), ya'ni uni ikkinchi
    // qurilmada ham, ilova qayta o'rnatilgandan keyin ham eslab
    // qolinadi. Noma'lum bo'lsa — YURMAYDI (`checkoutAllowed`).
    if (!AiTools.instance.checkoutAllowed) return;

    final rid = (proposal['restaurant_id'] ?? '').toString();
    final items = (proposal['items'] as List? ?? const [])
        .cast<Map<String, dynamic>>();
    if (rid.isEmpty || items.isEmpty) return;

    _running = true;
    _cancelled = false;
    final rname = (proposal['restaurant_name'] ?? '').toString();
    final total = (proposal['total_tiyin'] as num?)?.toInt() ?? 0;
    try {
      if (await _drive(rid, rname, items)) {
        // Yetib bordik — endi to'lov savolini yordamchi beradi.
        // Bekor qilingan yoki yiqilgan holatda CHAQIRILMAYDI:
        // bo'lmagan ekran uchun to'lov so'rash aldash bo'lardi.
        onCheckoutReady?.call(rname, total);
      }
    } finally {
      _running = false;
      _pointer = null;
      _tapping = false;
      notifyListeners();
    }
  }

  /// `true` — rasmiylashtirish ekraniga yetib borildi.
  Future<bool> _drive(
      String rid, String rname, List<Map<String, dynamic>> items) async {
    final nav = appNavigatorKey.currentState;
    if (nav == null) return false;

    _set('Restoran ochilyapti...');
    await _pause(700);
    if (_stop()) return false;

    // ┌─ YORDAMCHI EKRANI YOPILMAYDI ─────────────────────────────────┐
    // Ilgari bu yerda `nav.popUntil((r) => r.isFirst)` turardi.
    // U yordamchi ekranini stekdan olib tashlardi, ekran esa
    // `dispose` bo'lib ovozli seansni ham yopardi — Shaddiy
    // rasmiylashtirishga yetib borib JIM bo'lib qolardi.
    //
    // Endi keyingi ekranlar yordamchining USTIGA ochiladi: seans
    // tirik qoladi va u oxirida to'lovni so'ray oladi. Foydalanuvchi
    // "orqaga" bossa suhbatga qaytadi.
    // └───────────────────────────────────────────────────────────────┘
    await _pause(200);
    if (_stop()) return false;

    // ┌─ NAVIGATOR HAR SAFAR QAYTA OLINADI ───────────────────────────┐
    // Kutishlar orasida ilova yopilishi yoki qayta qurilishi mumkin.
    // Eski `NavigatorState` ga murojaat qilish "BuildContext across
    // async gap" xatosi — ya'ni o'lgan daraxtga yozish.
    // └───────────────────────────────────────────────────────────────┘
    final navMenu = appNavigatorKey.currentState;
    if (navMenu == null || !navMenu.mounted) return false;
    // Menyu — AYNAN katalogdagi kartochka bosilganda ochiladigan yo'l.
    unawaited(MenuScreen.open(navMenu.context, rid, fallbackName: rname));
    if (!await _waitFor(() => _stage.menuRestaurantId == rid,
        timeout: const Duration(seconds: 12))) {
      _set('Menyuni ochib bo\'lmadi');
      return false;
    }
    await _pause(600);

    for (final it in items) {
      if (_stop()) return false;
      final pid = (it['product_id'] ?? '').toString();
      final qty = (it['qty'] as num?)?.toInt() ?? 1;
      final name = (it['name'] ?? '').toString();
      if (pid.isEmpty || qty <= 0) continue;

      _set(name.isEmpty ? 'Taom qidirilyapti...' : '"$name" qidirilyapti...');

      // Taom ro'yxatda chizilishini kutamiz: menyu tarmoqdan keladi
      // va birinchi kadrda hali bo'sh bo'lishi mumkin.
      final key = _stage.productKey(pid);
      if (!await _waitFor(() => key.currentContext != null,
          timeout: const Duration(seconds: 10))) {
        _set('"$name" menyuda topilmadi');
        return false;
      }
      if (_stop()) return false;

      // Ko'rinadigan skroll — odam qidirgandek.
      //
      // `mounted` tekshiruvi SHART: yuqoridagi kutish davomida
      // foydalanuvchi ekranni yopgan bo'lishi mumkin va o'lgan
      // daraxtga skroll qilish istisno tashlardi.
      final ctx = key.currentContext;
      if (ctx != null && ctx.mounted) {
        await Scrollable.ensureVisible(
          ctx,
          alignment: 0.35,
          duration: const Duration(milliseconds: 700),
          curve: Curves.easeInOut,
        );
      }
      await _pause(250);
      if (_stop()) return false;

      // "Barmoq" AYNAN "+" tugmasi ustida. Kartochka markazi miqdor
      // raqamiga to'g'ri kelardi va noto'g'ri tugma bosilayotgandek
      // ko'rinardi.
      final plus = _stage.addKey(pid);
      _set(
          name.isEmpty
              ? 'Savatga qo\'shilyapti...'
              : '"$name" savatga qo\'shilyapti...',
          pointer: _centerOf(plus) ?? _centerOf(key));

      for (var i = 0; i < qty; i++) {
        if (_stop()) return false;
        // Joy HAR bosishdan oldin qayta o'lchanadi: birinchi
        // qo'shishdan keyin "+" miqdor qatoriga ko'chadi.
        await _tapAt(_stage.addKey(pid), fallback: key);
        _stage.addProduct?.call(pid);
        await _pause(450);
      }
      await _pause(300);
    }

    if (_stop()) return false;
    _set('Savat ochilyapti...');
    await _pause(500);

    final navCart = appNavigatorKey.currentState;
    if (navCart == null || !navCart.mounted) return false;
    unawaited(
        navCart.push(MaterialPageRoute(builder: (_) => const CartScreen())));

    // Rasmiylashtirish tugmasi narx hisoblangach faollashadi — uni
    // kutamiz. Kutmasak "bosish" hech narsa qilmasdi.
    if (!await _waitFor(() => _stage.cartCheckout != null,
        timeout: const Duration(seconds: 15))) {
      _set('Savat tayyor — rasmiylashtirishni o\'zingiz bosing');
      return false;
    }
    await _pause(600);
    if (_stop()) return false;

    final key = _stage.cartCheckoutKey;
    if (key != null) {
      _set('Rasmiylashtirishga o\'tilyapti...', pointer: _centerOf(key));
      await _tapAt(key);
    }
    _stage.cartCheckout?.call();

    // ┌─ SHU YERDA TO'XTAYMIZ ─────────────────────────────────────────┐
    // Keyingi tugma — pul sarflaydigan yagona tugma. Uni ODAM bosadi.
    // Boshqaruvchi manzilni ham, to'lov usulini ham tanlamaydi:
    // ularni foydalanuvchi ko'rib chiqishi kerak.
    // └────────────────────────────────────────────────────────────────┘
    await _pause(900);
    _set('Tayyor — to\'lovni tasdiqlang', pointer: null);
    await _pause(1200);
    return true;
  }

  /// Og'zaki tasdiqdan keyin buyurtmani NAQD to'lov bilan beradi.
  ///
  /// ┌─ TO'RTTA SHART ────────────────────────────────────────────────┐
  ///  1. Ruxsat berilgan bo'lishi kerak (`checkoutAllowed`).
  ///  2. Rasmiylashtirish ekrani OCHIQ bo'lishi kerak — tugma
  ///     ro'yxatdan o'tgan bo'lsa shunday.
  ///  3. Tugma FAOL bo'lishi kerak (narx hisoblangan, summa > 0);
  ///     aks holda `onTap` `null` bo'ladi.
  ///  4. Boshqa boshqaruv ketayotgan bo'lmasligi kerak.
  ///
  /// Bittasi bajarilmasa hech narsa qilinmaydi va `false` qaytadi —
  /// chaqiruvchi buni foydalanuvchiga aytadi. Jimgina o'tib ketish
  /// eng yomon variant bo'lardi: model "tasdiqladim" degan, ilova
  /// esa hech narsa qilmagan bo'lardi.
  /// └────────────────────────────────────────────────────────────────┘
  /// Hozir tasdiqlash MUMKINmi — hech narsa qilmasdan tekshiradi.
  ///
  /// Ovozli rejim tasdiq dialogini ochishdan OLDIN shuni so'raydi
  /// (`assistant_screen.dart`, bug.md 11-band): dialogni ochib, keyin
  /// "bajarib bo'lmadi" deyish foydalanuvchini bekorga bezovta
  /// qilardi. Shartlar `confirmCashOrder()` bilan bir xil.
  bool get canConfirmCashOrder =>
      !_running &&
      AiTools.instance.checkoutAllowed &&
      _stage.cashConfirm != null;

  Future<bool> confirmCashOrder() async {
    if (_running) return false;
    if (!AiTools.instance.checkoutAllowed) return false;
    final onTap = _stage.cashConfirm;
    if (onTap == null) return false;

    _running = true;
    _cancelled = false;
    try {
      final key = _stage.cashConfirmKey;
      _set('Buyurtma tasdiqlanyapti...',
          pointer: key == null ? null : _centerOf(key));
      await _pause(500);
      if (_cancelled) return false;
      if (key != null) await _tapAt(key);

      // Ro'yxat shu orada o'zgargan bo'lishi mumkin (ekran yopildi,
      // narx qayta hisoblandi) — QAYTA o'qiymiz.
      final fresh = _stage.cashConfirm;
      if (fresh == null) return false;
      fresh();
      await _pause(600);
      return true;
    } finally {
      _running = false;
      _pointer = null;
      _tapping = false;
      notifyListeners();
    }
  }

  // ── Yordamchilar ──

  bool _stop() {
    if (_cancelled) {
      _pointer = null;
      notifyListeners();
    }
    return _cancelled;
  }

  Future<void> _pause(int ms) =>
      Future<void>.delayed(Duration(milliseconds: ms));

  /// Shart bajarilishini kutadi (yoki muddat tugaydi).
  Future<bool> _waitFor(bool Function() cond,
      {required Duration timeout}) async {
    final deadline = DateTime.now().add(timeout);
    while (DateTime.now().isBefore(deadline)) {
      if (_cancelled) return false;
      if (cond()) return true;
      await _pause(120);
    }
    return false;
  }

  Offset? _centerOf(GlobalKey key) {
    final ctx = key.currentContext;
    final box = ctx?.findRenderObject();
    if (box is! RenderBox || !box.hasSize) return null;
    return box.localToGlobal(box.size.center(Offset.zero));
  }

  /// "Bosish" animatsiyasi — amalning O'ZI chaqiruvchida bajariladi.
  ///
  /// Ataylab shunday bo'lingan: animatsiya bo'lmasa ham amal bajariladi
  /// va aksincha — animatsiya hech qachon amalni almashtirmaydi.
  Future<void> _tapAt(GlobalKey key, {GlobalKey? fallback}) async {
    final p = _centerOf(key) ?? (fallback == null ? null : _centerOf(fallback));
    if (p == null) return;
    _set(_status, pointer: p, tapping: true);
    await _pause(260);
    _set(_status, pointer: p);
  }
}

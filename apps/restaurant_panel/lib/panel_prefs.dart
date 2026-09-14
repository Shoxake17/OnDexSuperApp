import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// Shu kompyuterdagi panel sozlamalari (bildirishnomalar).
///
/// ┌─ NEGA SERVERDA EMAS ──────────────────────────────────────────────┐
/// Bitta restoranning bir nechta kompyuteri bo'ladi: oshxonadagisi yangi
/// buyurtmada jiringlashi, menejer noutbuki esa jim turishi kerak. Server
/// sozlamasi hammasini birdek qilib qo'yardi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// Qiymatlar HAQIQATAN ishlatiladi: ovoz — `RingSound`, xabarlar —
/// buyurtmalar sahifasi (`orders_page.dart`).
class PanelPrefs {
  PanelPrefs._();

  static const _kSound = 'panel.notify.new_order_sound';
  static const _kBanner = 'panel.notify.new_order_banner';
  static const _kCourier = 'panel.notify.courier_alerts';

  /// Qabul qilinmagan buyurtma bo'lganda qo'ng'iroq ovozi.
  static final newOrderSound = ValueNotifier<bool>(true);

  /// "YANGI BUYURTMA!" ekrandagi xabari.
  static final newOrderBanner = ValueNotifier<bool>(true);

  /// Kuryer topishda xato bo'lganda ogohlantirish.
  static final courierAlerts = ValueNotifier<bool>(true);

  static bool _loaded = false;

  static Future<void> load() async {
    if (_loaded) return;
    try {
      final p = await SharedPreferences.getInstance();
      newOrderSound.value = p.getBool(_kSound) ?? true;
      newOrderBanner.value = p.getBool(_kBanner) ?? true;
      courierAlerts.value = p.getBool(_kCourier) ?? true;
      _loaded = true;
    } catch (_) {
      // Saqlash mavjud bo'lmasa — standart (hammasi yoqiq), panel ishlayveradi.
    }
  }

  static Future<void> set(ValueNotifier<bool> which, bool value) async {
    which.value = value;
    final key = identical(which, newOrderSound)
        ? _kSound
        : identical(which, newOrderBanner)
            ? _kBanner
            : _kCourier;
    try {
      final p = await SharedPreferences.getInstance();
      await p.setBool(key, value);
    } catch (_) {}
  }
}

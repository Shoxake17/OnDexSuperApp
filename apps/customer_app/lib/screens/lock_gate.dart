import 'package:flutter/material.dart';

import '../api.dart';
import '../services/app_lock.dart';
import '../services/app_pin.dart';
import '../services/push.dart';
import '../session.dart';
import 'home_shell.dart';
import 'login_screen.dart';
import 'mini_app_webview.dart';
import 'pin_reset_screen.dart';
import 'pin_screen.dart';

/// Qulf darvozasi — ichidagi ekranni faqat shaxs tasdiqlangandan keyin
/// ko'rsatadi.
///
/// ── IKKI QATLAM (Click/Payme naqshi) ───────────────────────────────
/// Asosiy qatlam — OnDex'ning O'Z PIN kodi. Uning USTIDA qurilmaning
/// tizim oynasi (barmoq izi / yuz / qurilma kaliti) avtomatik
/// ochiladi.
///
///	biometrika muvaffaqiyatli -> darhol kiradi;
///	foydalanuvchi oynani yopdi -> orqada PIN ekrani turadi, terib
///	kiradi;
///	qurilmada biometrika yo'q -> faqat PIN.
///
/// Ya'ni biometrika QULAYLIK, PIN esa KAFOLAT: biometrikasiz ham,
/// sensor nosoz bo'lganda ham foydalanuvchi ilovaga kira oladi.
///
/// ── QACHON QULFLANADI ──────────────────────────────────────────────
///   * ilova SOVUQ ishga tushganda (har safar);
///   * fondan qaytganda, agar fonda `AppLock.lockAfterBackground` dan
///     uzoq turgan bo'lsa.
///
/// ── NEGA FAQAT KIRGAN FOYDALANUVCHI UCHUN ──────────────────────────
/// Darvoza `HomeShell` ni (ya'ni sessiya bor holatni) o'raydi, kirish
/// ekranini EMAS. Ikki sabab:
///
///   1. himoya qiladigan narsa aynan sessiya — kirmagan odamda
///      yashiradigan ma'lumot yo'q;
///   2. kirish oqimlari (Telegram, Google) ilovani ATAYLAB fonga
///      chiqaradi va qaytaradi. Qulf o'sha ekranlarni ham qamrasa,
///      Telegram bilan kirish har safar qulf so'rab uzilib qolardi.
///
/// ── EKRAN SURATI ───────────────────────────────────────────────────
/// Qulflangan holatda ichki daraxt umuman QURILMAYDI — ya'ni ilovalar
/// ro'yxatidagi ko'rinishda ham (recent apps) mijoz ma'lumoti
/// ko'rinmaydi.
class LockGate extends StatefulWidget {
  const LockGate({super.key, required this.child, this.startUnlocked = false});

  final Widget child;

  /// Boshida qulf SO'RALMAYDI.
  ///
  /// Kirish oqimidan keyin ishlatiladi: foydalanuvchi hozirgina parol
  /// yoki Telegram/Google orqali shaxsini tasdiqladi, darhol yana
  /// barmoq izi so'rash mantiqsiz bo'lardi. Fonga chiqib qaytganda
  /// qulf baribir ishlaydi.
  final bool startUnlocked;

  @override
  State<LockGate> createState() => _LockGateState();
}

/// Kirishdan keyingi asosiy ekran — HAR DOIM qulf ostida.
///
/// NEGA ALOHIDA WIDGET: `HomeShell` beshta joydan ochiladi (kirish,
/// ro'yxatdan o'tish, parol tiklash, Google, Telegram). Har birida
/// qo'lda `LockGate` bilan o'rash kerak bo'lsa, bitta joyni unutish
/// qulfni o'sha yo'l uchun BUTUNLAY o'chirib qo'yardi va buni sezish
/// qiyin bo'lardi. Endi hamma yo'l shu widgetga boradi.
class LockedHome extends StatelessWidget {
  const LockedHome({super.key});

  @override
  Widget build(BuildContext context) =>
      const LockGate(startUnlocked: true, child: HomeShell());
}

enum _Stage { checking, needsPin, locked, resetting, open }

class _LockGateState extends State<LockGate> with WidgetsBindingObserver {
  _Stage _stage = _Stage.checking;

  /// Biometrika so'rovi hozir ochiqmi. Busiz fon/old almashinuvi
  /// ikkinchi so'rovni ochib yuborardi (Android'da bu birinchisini
  /// bekor qiladi va foydalanuvchi tugab bo'lmas siklga tushardi).
  bool _asking = false;

  bool _biometricAvailable = false;

  /// Saqlangan PIN eski uzunlikda (kiritib bo'lmaydi). Bunda tiklashni
  /// BEKOR QILISH mumkin emas — qaytadigan ishlaydigan ekran yo'q,
  /// yagona muqobil — hisobdan chiqish.
  bool _legacyPin = false;
  int _attemptsLeft = AppPin.maxAttempts;
  DateTime? _hiddenAt;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _start();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  Future<void> _start() async {
    _biometricAvailable = await AppLock.isAvailable();
    final pinSet = await AppPin.isSet();
    _attemptsLeft = await AppPin.attemptsLeft();
    if (!mounted) return;

    // PIN hali yaratilmagan — uni yaratmasdan ichkariga o'tkazmaymiz.
    // Kirishdan keyin ham (`startUnlocked`) shu qadam bajariladi:
    // qulfning butun mantig'i PIN mavjudligiga tayanadi.
    if (!pinSet) {
      setState(() => _stage = _Stage.needsPin);
      return;
    }
    // ESKI UZUNLIKDAGI PIN (masalan 4 xonali) — uni kiritib bo'lmaydi,
    // chunki ekran 6 xona to'lishini kutadi. Foydalanuvchi ilovaga
    // umuman kira olmay qolmasligi uchun TIKLASH oqimiga yuboriladi
    // (`AppPin._kLen` izohiga qarang). Bu holat faqat ilova yangilangan
    // qurilmalarda bir marta uchraydi.
    if (!await AppPin.matchesCurrentLength()) {
      if (!mounted) return;
      setState(() {
        _legacyPin = true;
        _stage = _Stage.resetting;
      });
      return;
    }
    if (widget.startUnlocked) {
      setState(() => _stage = _Stage.open);
      return;
    }
    setState(() => _stage = _Stage.locked);
    _askBiometric();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    switch (state) {
      case AppLifecycleState.paused:
      case AppLifecycleState.hidden:
        _hiddenAt = DateTime.now();
      case AppLifecycleState.resumed:
        _maybeRelock();
      case AppLifecycleState.inactive:
      case AppLifecycleState.detached:
        break;
    }
  }

  Future<void> _maybeRelock() async {
    if (_stage != _Stage.open || _asking) return;
    final hiddenAt = _hiddenAt;
    if (hiddenAt == null) return;
    if (DateTime.now().difference(hiddenAt) < AppLock.lockAfterBackground) {
      return;
    }
    if (!await AppPin.isSet()) return;
    _attemptsLeft = await AppPin.attemptsLeft();
    if (!mounted) return;
    setState(() => _stage = _Stage.locked);
    _askBiometric();
  }

  /// Tizim oynasini ochadi. Rad etilsa HECH NARSA qilmaymiz —
  /// orqadagi PIN ekrani o'z ishini davom ettiradi.
  Future<void> _askBiometric() async {
    if (_asking || !_biometricAvailable) return;
    if (!await AppLock.isEnabled()) return;
    _asking = true;
    // ┌─ NEGA KUTAMIZ ────────────────────────────────────────────────┐
    // `BiometricPrompt` — bu Android FRAGMENTI va u faqat Activity
    // RESUMED holatga o'tgandan keyin ko'rsatilishi mumkin.
    // `initState` dan darhol chaqirilsa Activity hali tayyor emas:
    // plagin xato qaytaradi va oyna UMUMAN OCHILMAYDI. Jonli
    // qurilmada aynan shu holat kuzatilgan.
    // └───────────────────────────────────────────────────────────────┘
    await WidgetsBinding.instance.endOfFrame;
    await Future<void>.delayed(const Duration(milliseconds: 250));
    if (!mounted) {
      _asking = false;
      return;
    }
    final ok = await AppLock.unlock();
    _asking = false;
    if (!mounted || !ok) return;
    // Biometrika muvaffaqiyatli — PIN urinishlari ham tozalanadi.
    await AppPin.resetAttemptsAfterBiometric();
    if (!mounted) return;
    setState(() {
      _stage = _Stage.open;
      _hiddenAt = null;
      _attemptsLeft = AppPin.maxAttempts;
    });
  }

  Future<bool> _submitPin(String pin) async {
    final res = await AppPin.verify(pin);
    if (!mounted) return false;
    if (res.ok) {
      setState(() {
        _stage = _Stage.open;
        _hiddenAt = null;
        _attemptsLeft = AppPin.maxAttempts;
      });
      return true;
    }
    if (res.sessionWiped) {
      // Urinishlar tugadi — sessiya butunlay o'chiriladi.
      await _signOut(
          'PIN bir necha marta noto\'g\'ri kiritildi. Xavfsizlik uchun '
          'hisobdan chiqarildi.');
      return false;
    }
    setState(() => _attemptsLeft = res.attemptsLeft);
    return false;
  }

  /// Qulf ekranidagi chiqish tugmasi — tasdiq so'raydi.
  ///
  /// TASDIQ NEGA KERAK: tugma o'ng yuqorida, "orqaga" odati bilan
  /// tasodifan bosilishi oson. Chiqish esa qaytarib bo'lmaydigan
  /// amal — foydalanuvchi qaytadan to'liq kirishga majbur bo'lardi.
  Future<void> _confirmLogout() async {
    final yes = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Hisobdan chiqasizmi?'),
        content: const Text(
            'Qaytadan kirish uchun telefon raqamingiz va parolingiz '
            '(yoki Telegram/Google) kerak bo\'ladi.'),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('Yo\'q')),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Ha, chiqish'),
          ),
        ],
      ),
    );
    if (yes != true) return;
    await _signOut('Hisobdan chiqdingiz.');
  }

  /// Chiqish — urinishlar tugagan holat va chiqish tugmasi uchun.
  ///
  /// Serverga ham xabar beriladi: qurilmadagi nusxani o'chirishning
  /// o'zi yetarli emas, o'g'irlangan token 30 kun ishlayverardi.
  Future<void> _signOut(String reason) async {
    // Push tokeni logout'dan OLDIN — sabab profile_screen.dart'dagi
    // izohda (keyin so'rov 401 olardi va yozuv serverda qolardi).
    await PushService.instance.stop();
    try {
      await api.logout();
    } catch (_) {
      // Tarmoq bo'lmasa ham lokal tozalash BAJARILADI.
    }
    await tokenStore.clear();
    api.token = null;
    await AppPin.clear();
    await clearMiniAppSession();
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      content: Text(reason),
      behavior: SnackBarBehavior.floating,
    ));
    Navigator.of(context).pushAndRemoveUntil(
      MaterialPageRoute(builder: (_) => const LoginScreen()),
      (route) => false,
    );
  }

  @override
  Widget build(BuildContext context) {
    switch (_stage) {
      case _Stage.checking:
        return const Scaffold(body: Center(child: CircularProgressIndicator()));
      case _Stage.needsPin:
        return PinSetupScreen(onDone: () {
          setState(() => _stage = _Stage.open);
        });
      case _Stage.locked:
        return PinEntryScreen(
          onSubmit: _submitPin,
          attemptsLeft: _attemptsLeft,
          onBiometric: _biometricAvailable ? _askBiometric : null,
          // "Unutdim" HISOBDAN CHIQARMAYDI: raqamga kod yuboriladi va
          // tasdiqlangach yangi PIN qo'yiladi (`PinResetScreen`).
          // Chiqish alohida — o'ng yuqoridagi tugma.
          onForgot: () => setState(() => _stage = _Stage.resetting),
          onLogout: _confirmLogout,
        );
      case _Stage.resetting:
        return PinResetScreen(
          notice: _legacyPin
              ? 'PIN kod endi 6 xonali. Xavfsizlik uchun yangi PIN '
                  'yaratishdan oldin raqamingizni tasdiqlang.'
              : null,
          onDone: () async {
            // Yangi PIN o'rnatildi — urinishlar hisobi ham tozalanadi
            // (`AppPin.set` buni bajaradi).
            if (!mounted) return;
            setState(() {
              _legacyPin = false;
              _stage = _Stage.open;
              _hiddenAt = null;
              _attemptsLeft = AppPin.maxAttempts;
            });
          },
          // Eski PIN holatida qaytadigan ishlaydigan ekran YO'Q —
          // yagona muqobil chiqish (yuqoridagi `_legacyPin` izohi).
          onCancel: _legacyPin
              ? _confirmLogout
              : () => setState(() => _stage = _Stage.locked),
        );
      case _Stage.open:
        return widget.child;
    }
  }
}

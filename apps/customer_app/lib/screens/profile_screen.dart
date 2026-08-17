import 'package:flutter/material.dart';

import '../api.dart';
import '../data/catalog_repository.dart';
import '../services/app_lock.dart';
import '../services/app_pin.dart';
import '../services/firebase_phone.dart';
import '../services/google_auth.dart';
import '../services/push.dart';
import '../session.dart';
import '../widgets/sheet_scaffold.dart';
import 'address_screen.dart';
import 'login_screen.dart';
import 'web_session.dart';
import 'pin_screen.dart';

/// "Profil" bo'limi — foydalanuvchi ma'lumotlari, manzil boshqaruvi va
/// chiqish (logout). Logout endi shu yerda — avval RestaurantsScreen'ning
/// AppBar'ida edi, endi pastki menyuga ko'chirilgani sababli mantiqan
/// Profil bo'limiga tegishli (Yandex Eats/Wolt'dagi kabi).
class ProfileScreen extends StatefulWidget {
  const ProfileScreen({super.key});

  @override
  State<ProfileScreen> createState() => _ProfileScreenState();
}

class _ProfileScreenState extends State<ProfileScreen> {
  late Future<Map<String, dynamic>> _future;

  bool _lockAvailable = false;
  bool _lockEnabled = false;

  @override
  void initState() {
    super.initState();
    _future = api.me();
    _loadLockState();
  }

  /// PIN o'zgartirish — avval JORIY PIN so'raladi.
  ///
  /// NEGA JORIY PIN SHART: ilova ochiq qolgan telefonni qo'lga
  /// kiritgan odam yangi PIN qo'yib, egasini o'z hisobidan
  /// chiqarib yuborardi. Bu — parol o'zgartirishdagi
  /// `current_password` talabining aynan o'zi.
  Future<void> _changePin() async {
    // Urinishlar SHU YERDA ham tugashi mumkin — `AppPin.verify`
    // hisobni oshiradi. Bunda PIN o'chadi va sessiya OCHIQ qolsa,
    // qulf butunlay yo'qolardi (keyingi qulflashda "PIN yo'q" holati
    // hech narsa so'ramaydi). Shuning uchun bu holat chiqishga
    // olib keladi — qulf ekranidagi bilan bir xil qoida.
    var wiped = false;
    final verified = await Navigator.of(context).push<bool>(
      MaterialPageRoute(
        builder: (ctx) => PinEntryScreen(
          title: 'Joriy PIN kodni kiriting',
          subtitle: 'Yangi PIN o\'rnatishdan oldin',
          onSubmit: (pin) async {
            final res = await AppPin.verify(pin);
            if (res.sessionWiped) {
              wiped = true;
              if (ctx.mounted) Navigator.of(ctx).pop(false);
              return false;
            }
            if (res.ok && ctx.mounted) Navigator.of(ctx).pop(true);
            return res.ok;
          },
          onForgot: () => Navigator.of(ctx).pop(false),
          forgotLabel: 'Bekor qilish',
        ),
      ),
    );
    if (wiped) {
      await _logout(confirm: false);
      return;
    }
    if (verified != true || !mounted) return;
    await Navigator.of(context).push<void>(
      MaterialPageRoute(
        builder: (ctx) => PinSetupScreen(
          onDone: () => Navigator.of(ctx).pop(),
          onCancel: () => Navigator.of(ctx).pop(),
        ),
      ),
    );
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(const SnackBar(
      content: Text('PIN kod yangilandi'),
      behavior: SnackBarBehavior.floating,
    ));
  }

  Future<void> _loadLockState() async {
    final available = await AppLock.isAvailable();
    final enabled = await AppLock.isEnabled();
    if (!mounted) return;
    setState(() {
      _lockAvailable = available;
      _lockEnabled = enabled;
    });
  }

  /// `confirm: false` — tasdiq so'ralmaydi (xavfsizlik sababli
  /// majburiy chiqish: PIN urinishlari tugagan holat).
  Future<void> _logout({bool confirm = true}) async {
    if (!confirm) {
      await _doLogout();
      return;
    }
    final yes = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Chiqishni tasdiqlaysizmi?'),
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
    await _doLogout();
  }

  Future<void> _doLogout() async {
    // Push tokeni ENG BIRINCHI o'chiriladi — `api.logout()` dan ham
    // oldin. Logout barcha sessiyalarni bekor qilgandan keyin
    // `DELETE /me/push-token` 401 olardi va yozuv serverda qolib
    // ketardi: bu telefonga keyingi egasining buyurtmalari haqidagi
    // xabarlar kelaverardi (push.dart izohiga qarang).
    await PushService.instance.stop();
    // Avval SERVERGA xabar beramiz — token o'chirilishidan OLDIN, chunki
    // so'rov aynan shu token bilan yuboriladi. Busiz "chiqish" faqat
    // qurilmadagi nusxani o'chirardi va o'g'irlangan token yana 30 kun
    // ishlayverardi (backend: POST /auth/logout).
    await api.logout();
    await tokenStore.clear();
    api.token = null;
    // Google va Firebase O'Z sessiyalarini alohida saqlaydi — ular
    // tozalanmasa "chiqish" faqat yarim ish bo'lardi:
    //   * Google: keyingi safar hisob tanlash oynasi KO'RSATILMAY,
    //     eskisi jimgina qayta ishlatilardi. Umumiy qurilmada bu
    //     to'g'ridan-to'g'ri begona odamning akkauntiga kirish demakdir;
    //   * Firebase: chiqqan foydalanuvchi Firebase'da kirgan bo'lib
    //     qolardi va keyingi telefon tasdiqlashda eski hisob
    //     ishlatilishi mumkin edi.
    // Ikkalasi ham xatoni yutadi — chiqishga xalaqit bermaydi.
    await GoogleAuth.signOut();
    await FirebasePhoneAuth.signOut();
    // PIN ham o'chiriladi: u SHU sessiyaga tegishli. Qolib ketsa,
    // qurilmadan chiqqan odamning PIN'i keyingi foydalanuvchiga
    // qolardi va u boshqa hisobning qulfini ochardi.
    await AppPin.clear();
    // WebView'dagi `chust_session` cookie'si ham tozalanadi — busiz
    // chiqqandan keyin ham web tomondagi sessiya 30 kun ochiq qolardi
    // (web_session.dart'dagi izohga qarang).
    await clearMiniAppSession();
    // Shaxsiy kesh (buyurtmalar, sevimlilar) SHIFRLANGAN holda
    // diskda yotadi — u ham tozalanishi SHART. Busiz qurilmadan
    // chiqqan odamning buyurtmalari va manzili keyingi
    // foydalanuvchiga ko'rinardi.
    //
    // Katalog keshi ATAYLAB qoldiriladi: u shaxsiy emas va keyingi
    // kirishda ilova bir zumda ochilishini ta'minlaydi
    // (`data/catalog_repository.dart` izohiga qarang).
    await Repos.clearOnLogout();
    if (!mounted) return;
    Navigator.of(context).pushAndRemoveUntil(
      MaterialPageRoute(builder: (_) => const LoginScreen()),
      (route) => false,
    );
  }

  @override
  Widget build(BuildContext context) {
    return SheetScaffold(
      title: 'Profil',
      child: SafeArea(
        top: false,
        child: FutureBuilder<Map<String, dynamic>>(
          future: _future,
          builder: (context, snap) {
            final phone = snap.data?['phone'] as String? ?? '';
            final name = snap.data?['name'] as String? ?? '';
            return ListView(
              padding: const EdgeInsets.symmetric(vertical: 16),
              children: [
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 16),
                  child: Row(
                    children: [
                      CircleAvatar(
                        radius: 28,
                        backgroundColor: Theme.of(context)
                            .colorScheme
                            .surfaceContainerHighest,
                        child: const Icon(Icons.person, size: 32),
                      ),
                      const SizedBox(width: 16),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(name.isEmpty ? 'Mijoz' : name,
                                style: const TextStyle(
                                    fontSize: 18, fontWeight: FontWeight.bold)),
                            const SizedBox(height: 2),
                            Text(
                                phone.isEmpty
                                    ? (snap.connectionState ==
                                            ConnectionState.waiting
                                        ? 'Yuklanmoqda...'
                                        : '')
                                    : phone,
                                style: const TextStyle(color: Colors.grey)),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 24),
                const Divider(height: 1),
                ListTile(
                  leading: const Icon(Icons.location_on_outlined),
                  title: const Text('Yetkazib berish manzili'),
                  trailing: const Icon(Icons.chevron_right),
                  onTap: () => Navigator.of(context).push(
                    MaterialPageRoute(builder: (_) => const AddressScreen()),
                  ),
                ),
                const Divider(height: 1),
                // Qulf sozlamasi FAQAT qurilmada himoya bo'lsa
                // ko'rsatiladi — PIN/barmoq izi umuman qo'yilmagan
                // telefonda bu tugma hech nima qila olmasdi va
                // foydalanuvchini chalg'itardi.
                ListTile(
                  leading: const Icon(Icons.pin_outlined),
                  title: const Text('PIN kodni o\'zgartirish'),
                  trailing: const Icon(Icons.chevron_right),
                  onTap: _changePin,
                ),
                const Divider(height: 1),
                if (_lockAvailable)
                  SwitchListTile(
                    secondary: const Icon(Icons.fingerprint),
                    title: const Text('Ilova qulfi'),
                    subtitle: const Text(
                        'Ochilganda barmoq izi, yuz yoki qurilma kaliti '
                        'so\'raladi'),
                    value: _lockEnabled,
                    onChanged: (v) async {
                      await AppLock.setEnabled(v);
                      if (!mounted) return;
                      setState(() => _lockEnabled = v);
                    },
                  ),
                if (_lockAvailable) const Divider(height: 1),
                ListTile(
                  leading: const Icon(Icons.logout, color: Colors.red),
                  title: const Text('Chiqish',
                      style: TextStyle(color: Colors.red)),
                  onTap: _logout,
                ),
              ],
            );
          },
        ),
      ),
    );
  }
}

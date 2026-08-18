import 'package:flutter/material.dart';

import '../api.dart';
import 'catalog_screen.dart' show kBrand;
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

/// Profil kartochkalarining umumiy bezagi — vebdagi
/// `rounded-2xl border border-neutral-200 bg-white` bilan bir xil.
final _cardDecoration = BoxDecoration(
  color: Colors.white,
  borderRadius: BorderRadius.circular(16),
  border: Border.all(color: const Color(0xFFE5E5E5)),
);

/// Ikon + kichik yorliq + qiymat qatori. `onTap` berilsa o'ngda
/// ko'rsatkich chiziladi va qator bosiladi.
class _InfoRow extends StatelessWidget {
  final IconData icon;
  final String label;
  final String value;
  final VoidCallback? onTap;

  const _InfoRow({
    required this.icon,
    required this.label,
    required this.value,
    this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final row = Container(
      padding: const EdgeInsets.all(14),
      decoration: _cardDecoration,
      child: Row(
        children: [
          Icon(icon, size: 19, color: const Color(0xFF9E9E9E)),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(label,
                    style: const TextStyle(
                        fontSize: 12, color: Color(0xFF757575))),
                Text(value,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(fontSize: 15)),
              ],
            ),
          ),
          if (onTap != null)
            const Icon(Icons.chevron_right,
                size: 18, color: Color(0xFF9E9E9E)),
        ],
      ),
    );

    if (onTap == null) return row;
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(16),
      child: row,
    );
  }
}

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
            final me = snap.data;
            final loading = snap.connectionState == ConnectionState.waiting;

            // Ism vebdagi `displayName()` bilan bir xil tartibda
            // tanlanadi: `name` -> `first_name last_name` -> "Mijoz".
            final full = [
              (me?['first_name'] as String?) ?? '',
              (me?['last_name'] as String?) ?? '',
            ].where((s) => s.trim().isNotEmpty).join(' ').trim();
            final name = ((me?['name'] as String?) ?? '').trim().isNotEmpty
                ? (me!['name'] as String).trim()
                : (full.isNotEmpty ? full : 'Mijoz');
            final phone = (me?['phone'] as String?) ?? '';
            final email = ((me?['email'] as String?) ?? '').trim();
            final address =
                (((me?['address'] as Map?)?['text'] as String?) ?? '').trim();

            return ListView(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
              children: [
                // ── Foydalanuvchi kartasi ──────────────────────────
                Container(
                  padding: const EdgeInsets.all(16),
                  decoration: _cardDecoration,
                  child: Row(
                    children: [
                      Container(
                        width: 56,
                        height: 56,
                        alignment: Alignment.center,
                        decoration: const BoxDecoration(
                          shape: BoxShape.circle,
                          color: kBrand,
                        ),
                        // Vebdagi kabi ism harfi — umumiy "odam"
                        // ikonkasidan ko'ra shaxsiyroq ko'rinadi.
                        child: Text(
                          name.characters.first.toUpperCase(),
                          style: const TextStyle(
                              fontSize: 22,
                              fontWeight: FontWeight.bold,
                              color: Colors.white),
                        ),
                      ),
                      const SizedBox(width: 14),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(name,
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: const TextStyle(
                                    fontSize: 17,
                                    fontWeight: FontWeight.bold)),
                            const SizedBox(height: 2),
                            Text(
                              phone.isEmpty
                                  ? (loading ? 'Yuklanmoqda…' : '')
                                  : phone,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: const TextStyle(
                                  fontSize: 13.5, color: Color(0xFF757575)),
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 16),

                // Telefon bloki ATAYLAB yo'q: raqam yuqoridagi kartada,
                // ism ostida allaqachon turibdi.
                if (email.isNotEmpty) ...[
                  _InfoRow(
                      icon: Icons.mail_outline, label: 'Email', value: email),
                  const SizedBox(height: 8),
                ],
                _InfoRow(
                  icon: Icons.place_outlined,
                  label: 'Yetkazish manzili',
                  // Manzil MATNI ko'rsatiladi — avval faqat "Yetkazib
                  // berish manzili" yozuvi turardi va mijoz qaysi
                  // manzil saqlanganini ochmasdan bilolmasdi.
                  value: address.isEmpty ? 'Tanlanmagan' : address,
                  onTap: () async {
                    await Navigator.of(context).push(
                      MaterialPageRoute(
                          builder: (_) => const AddressScreen()),
                    );
                    // Manzil o'zgargan bo'lishi mumkin — qayta o'qiymiz.
                    if (mounted) setState(() => _future = api.me());
                  },
                ),

                // ── Faqat native sozlamalar ────────────────────────
                //
                // PIN va ilova qulfi vebda YO'Q va bo'lishi ham mumkin
                // emas: brauzerda qurilma kaliti/barmoq izi bilan
                // ilovani qulflash tushunchasi yo'q.
                const SizedBox(height: 8),
                _InfoRow(
                  icon: Icons.pin_outlined,
                  label: 'Xavfsizlik',
                  value: 'PIN kodni o\'zgartirish',
                  onTap: _changePin,
                ),
                // Qulf sozlamasi FAQAT qurilmada himoya bo'lsa
                // ko'rsatiladi — PIN/barmoq izi umuman qo'yilmagan
                // telefonda bu tugma hech nima qila olmasdi va
                // foydalanuvchini chalg'itardi.
                if (_lockAvailable) ...[
                  const SizedBox(height: 8),
                  Container(
                    padding: const EdgeInsets.fromLTRB(14, 4, 8, 4),
                    decoration: _cardDecoration,
                    child: Row(
                      children: [
                        const Icon(Icons.fingerprint,
                            size: 19, color: Color(0xFF9E9E9E)),
                        const SizedBox(width: 12),
                        const Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text('Ilova qulfi',
                                  style: TextStyle(
                                      fontSize: 12, color: Color(0xFF757575))),
                              Text(
                                  'Ochilganda barmoq izi yoki qurilma kaliti',
                                  style: TextStyle(fontSize: 14)),
                            ],
                          ),
                        ),
                        Switch(
                          value: _lockEnabled,
                          activeThumbColor: kBrand,
                          onChanged: (v) async {
                            await AppLock.setEnabled(v);
                            if (!mounted) return;
                            setState(() => _lockEnabled = v);
                          },
                        ),
                      ],
                    ),
                  ),
                ],

                // ── Chiqish ────────────────────────────────────────
                const SizedBox(height: 28),
                SizedBox(
                  height: 52,
                  child: OutlinedButton.icon(
                    onPressed: _logout,
                    icon: const Icon(Icons.logout, size: 18),
                    label: const Text('Chiqish',
                        style: TextStyle(
                            fontSize: 15.5, fontWeight: FontWeight.bold)),
                    style: OutlinedButton.styleFrom(
                      foregroundColor: const Color(0xFFDC2626),
                      side: const BorderSide(color: Color(0xFFE5E5E5)),
                      shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(14)),
                    ),
                  ),
                ),
              ],
            );
          },
        ),
      ),
    );
  }
}

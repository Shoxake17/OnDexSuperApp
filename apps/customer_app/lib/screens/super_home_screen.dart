import 'package:flutter/material.dart';

import '../api.dart';
import '../data/address_scope.dart';
import '../widgets/header_action.dart';
import '../widgets/sheet_page.dart';
import 'address_screen.dart';
import 'catalog_screen.dart' show kBrand;
import 'coming_soon_screen.dart';
import '../widgets/promo_banners.dart';
import 'qurt_screen.dart';
import 'menu_screen.dart';
import 'notifications_screen.dart';
import 'ondex_map_screen.dart';
import 'restaurant_shell.dart';
import 'wallet_screen.dart';

/// OnDex Super App — ilovaning BOSH sahifasi (maket: `image/main.png`).
///
/// ┌─ NEGA KATALOG EMAS ────────────────────────────────────────────────┐
/// Ilgari kirgandan keyin darhol RESTORANLAR ro'yxati ochilardi, ya'ni
/// ilova "ovqat yetkazish ilovasi" bo'lib ko'rinardi. OnDex esa super
/// app: restoran uning xizmatlaridan BITTASI. Bosh sahifa endi barcha
/// xizmatlarni ko'rsatadi, restoranlar esa "Restoran" kartasi orqali
/// ochiladi.
///
/// Karta `RestaurantShell` ni ochadi — restoran bo'limi O'Z pastki
/// menyusi bilan keladi (Bosh sahifa · Savat · QR · Sevimlilar ·
/// Buyurtmalar). Shu sababdan super menyuda ovqatga tegishli bo'limlar
/// YO'Q: ular faqat shu bo'lim ichida ma'noga ega.
/// └────────────────────────────────────────────────────────────────────┘
///
/// ┌─ MAKET TELEFON KENGLIGIDA CHIZILMAGAN ─────────────────────────────┐
/// `image/main.png` — 907px kanvas. Undagi o'lchamlarni 1:1 ko'chirish
/// MUMKIN EMAS: tavsif shrifti telefonda ~9pt bo'lib o'qilmay qolardi.
///
/// Shuning uchun maketdan AYNAN ko'chirilgani: rasmlar, ranglar,
/// burchak radiuslari, elementlar tartibi va nisbatlari. Shrift
/// o'lchamlari esa telefon uchun moslashtirilgan — maket "qanday
/// ko'rinishi" haqida, "necha piksel" haqida emas.
/// └────────────────────────────────────────────────────────────────────┘
///
/// ┌─ SOXTA TUGMA YO'Q ─────────────────────────────────────────────────┐
/// Maketdagi sakkizta xizmatning hech biri "ishlaydigandek" ko'rinib,
/// bosilganda hech narsa bo'lmaydigan tugma emas: backend'da hali yo'q
/// bo'lganlari (Do'kon, Computer Club, Uy Joy, Moshina bozori, Ish joy
/// e'lonlari, OnDex Taxi) `ComingSoonScreen` ni ochadi — holat OCHIQ
/// aytiladi, foydalanuvchi ilovani buzuq deb o'ylamaydi.
/// └────────────────────────────────────────────────────────────────────┘
class SuperHomeScreen extends StatefulWidget {
  /// Stol QR kodini skanerlash — qidiruv maydonidagi belgi.
  ///
  /// Oqimning o'zi `table_qr_flow.dart` da (skaner → savatga stol
  /// seansi → menyu), shuning uchun bu ekran uni CHAQIRADI, o'zi
  /// bajarmaydi.
  final Future<void> Function() onScanQr;

  const SuperHomeScreen({super.key, required this.onScanQr});

  @override
  State<SuperHomeScreen> createState() => _SuperHomeScreenState();
}

// ── Maket ranglari (`image/main.png`) ──
const _pageBg = Color(0xFFFFFFFF);
const _titleDark = Color(0xFF111827);
const _muted = Color(0xFF6B7280);
const _tileBg = Color(0xFFF5F5F6);
const _bannerBg = Color(0xFFFDF1E7);
const _fieldBorder = Color(0xFFE5E7EB);

class _Service {
  final String title;

  /// Rasm topilmasa o'rnini bosadigan belgi. Ekranda KO'RINMAYDI —
  /// faqat zaxira.
  final IconData icon;
  final Color color;

  /// Maketdan kesib olingan rasm (`assets/services/`). Haqiqiy foto
  /// tayyor bo'lganda SHU NOM bilan almashtiriladi — kodga tegilmaydi.
  final String image;

  const _Service({
    required this.title,
    required this.icon,
    required this.color,
    required this.image,
  });
}

// ┌─ KARTADA FAQAT RASM VA NOM ────────────────────────────────────────┐
// Ilgari har kartada rangli ikonka, ikki qatorli tavsif va tugma
// (yoki "Tez orada") turardi. Telefon kengligida bularning hammasi
// bir vaqtda sig'masdi: matn kesilardi, rasm kichrayardi.
//
// Endi kartada IKKI narsa bor — rasm va nom. Rasm butun blokni
// egallaydi, nom uning ostida. Xizmat ishlamasa ham karta bir xil
// ko'rinadi; bosilganda "tez orada" deb aytiladi.
// └────────────────────────────────────────────────────────────────────┘
const _services = <_Service>[
  _Service(
    title: 'Restoran',
    icon: Icons.restaurant,
    color: kBrand,
    image: 'assets/services/restoran.png',
  ),
  _Service(
    title: 'Do\'kon',
    icon: Icons.storefront,
    color: Color(0xFF1E6BF1),
    image: 'assets/services/dokon.png',
  ),
  _Service(
    title: 'Computer Club',
    icon: Icons.sports_esports,
    color: Color(0xFF7C3AED),
    image: 'assets/services/club.png',
  ),
  _Service(
    title: 'Uy Joy',
    icon: Icons.home_work,
    color: Color(0xFF0EA5A5),
    image: 'assets/services/uyjoy.png',
  ),
];

const _tiles = <_Service>[
  _Service(
    title: 'Moshina\nbozori',
    icon: Icons.directions_car,
    color: Color(0xFF334155),
    image: 'assets/services/moshina.png',
  ),
  _Service(
    title: 'OnDex Xarita',
    icon: Icons.place,
    color: kBrand,
    image: 'assets/services/map.png',
  ),
  _Service(
    title: 'Ish joy\ne\'lonlari',
    icon: Icons.work,
    color: Color(0xFF92400E),
    image: 'assets/services/ishjoy.png',
  ),
  _Service(
    title: 'OnDex Taxi\nxizmati',
    icon: Icons.local_taxi,
    color: Color(0xFFF59E0B),
    image: 'assets/services/taxi.png',
  ),
];

class _SuperHomeScreenState extends State<SuperHomeScreen> {
  /// Pull-to-refresh: o'zgarganda menu bannerlari serverdan majburan yangilanadi.
  int _bannerTick = 0;

  int _unread = 0;
  // ⚠️ Boshlang'ich qiymat `AddressScope` dan OLINADI, `kNoAddressLabel`
  // dan emas. Avval bu yerda doimiy turardi va ekran ochilganda manzil
  // allaqachon ma'lum bo'lsa ham "Manzilni tanlang" ko'rinardi:
  // `initState` dagi `refresh()` serverdan AYNAN o'sha manzilni olardi,
  // o'zgarish bo'lmagani uchun xabar kelmasdi va `_onAddressChanged`
  // hech qachon ishlamasdi.
  String _city = AddressScope.instance.label;
  List<dynamic> _restaurants = const [];
  bool _loadingRestaurants = true;

  @override
  void initState() {
    super.initState();
    // Manzil BITTA manbadan keladi va o'zgarganda o'zi xabar beradi —
    // shunda restoran sahifasi bilan hech qachon ajralib qolmaydi
    // (`AddressScope` izohiga qarang).
    AddressScope.instance.addListener(_onAddressChanged);
    _loadUnread();
    _loadAddress();
    _loadRestaurants();
  }

  @override
  void dispose() {
    AddressScope.instance.removeListener(_onAddressChanged);
    super.dispose();
  }

  /// Bildirishnoma belgisi — xatosi JIM yutiladi (bezak ma'lumoti,
  /// usiz ham sahifa to'liq ishlaydi).
  Future<void> _loadUnread() async {
    try {
      final n = await api.unreadNotificationCount();
      if (mounted) setState(() => _unread = n);
    } catch (_) {}
  }

  Future<void> _loadAddress() => AddressScope.instance.refresh();

  /// Manzil o'zgardi — sarlavha VA ro'yxat birga yangilanadi.
  ///
  /// Ro'yxat ham shu yerda qayta so'raladi: server uni manzil shahri
  /// bilan cheklaydi (`?for_me=1`), ya'ni manzil o'zgarsa javob ham
  /// boshqacha bo'ladi. Busiz yangi ro'yxat faqat ilova qayta ishga
  /// tushganda ko'rinardi.
  void _onAddressChanged() {
    if (!mounted) return;
    setState(() => _city = AddressScope.instance.label);
    _loadRestaurants();
  }

  Future<void> _loadRestaurants() async {
    try {
      final list = await api.restaurants();
      if (mounted) {
        setState(() {
          _restaurants = list;
          _loadingRestaurants = false;
        });
      }
    } catch (_) {
      if (mounted) setState(() => _loadingRestaurants = false);
    }
  }

  Future<void> _push(Widget screen) async {
    await Navigator.of(context).push(sheetRoute(screen));
    if (mounted) {
      _loadUnread();
      // ⚠️ Manzil ham qayta o'qiladi. Busiz manzil ekranidan
      // qaytilganda sarlavhada ESKI nom qolardi, restoran sahifasi
      // esa (`catalog_screen.dart`, AYNI manbadan o'qiydi) yangisini
      // ko'rsatardi — bitta ilovada ikki xil manzil.
      //
      // O'zgargan bo'lsa `AddressScope` o'zi xabar beradi va
      // `_onAddressChanged` ro'yxatni ham yangilaydi.
      _loadAddress();
    }
  }

  /// Banner bosilganda: server yuborgan ction (backend internal/banners).
  /// Noma'lum amal e'tiborsiz qoldiriladi (yangi turdagi banner eski ilovani buzmaydi).
  void _openBannerAction(String action) {
    switch (action) {
      case 'qurt':
        _push(const QurtScreen());
    }
  }

  void _openService(_Service s) {
    // Ishlaydigan xizmatlar soni ortgani uchun endi nom bo'yicha
    // tanlanadi. Ilgari bitta xizmat bor edi va `_push` to'g'ridan-
    // to'g'ri restoranni ochardi — yangi xizmat qo'shilganda bu
    // jimgina noto'g'ri ekranga olib borardi.
    switch (s.title) {
      case 'OnDex Xarita':
        _push(const OndexMapScreen());
      case 'Do\'kon':
        // Do'kon bo'limida hozircha bitta ishlaydigan toifa bor — Qurt. Do'kon
        // bannerlari (admin panel) shu sahifa tepasida chiqadi.
        _push(const QurtScreen());
      case 'Computer Club':
        _push(const ComingSoonScreen(
          title: 'Computer Club',
          heading: 'Computer Club tez orada ochiladi',
          subtitle: 'Zamonaviy o\'yin zonalari tez orada sizga '
              'yaqin bo\'ladi! \u{1F3AE}',
          mascotAsset: 'assets/illustrations/gameclub_mascot.png',
        ));
      case 'Uy Joy':
        _push(const ComingSoonScreen(
          title: 'Uy Joy',
          heading: 'Uy-joy xizmati tez orada ishga tushadi',
          subtitle: 'Kvartira va uylarni qidirish tez orada '
              'oson bo\'ladi! \u{1F3E0}',
          mascotAsset: 'assets/illustrations/uyjoy_mascot.png',
        ));
      case 'Moshina\nbozori':
        _push(const ComingSoonScreen(
          title: 'Moshina bozori',
          heading: 'Moshina bozori tez orada ochiladi',
          subtitle: 'Yangi va haydalgan avtomobillarni qidirish '
              'tez orada oson bo\'ladi! \u{1F697}',
          mascotAsset: 'assets/illustrations/moshinabozori_mascot.png',
        ));
      case 'Ish joy\ne\'lonlari':
        _push(const ComingSoonScreen(
          title: 'Ish joy e\'lonlari',
          heading: 'Ish joy e\'lonlari tez orada ochiladi',
          subtitle: 'Mos vakansiyalarni topish va CV yuborish '
              'tez orada oson bo\'ladi! \u{1F4BC}',
          mascotAsset: 'assets/illustrations/ishjoy_mascot.png',
        ));
      case 'OnDex Taxi\nxizmati':
        _push(const ComingSoonScreen(
          title: 'OnDex Taxi',
          heading: 'OnDex Taxi xizmati tez orada ishga tushadi',
          subtitle: 'Shahar bo\'ylab tezkor taksi chaqirish '
              'tez orada mavjud bo\'ladi! \u{1F695}',
          mascotAsset: 'assets/illustrations/taxi_mascot.png',
        ));
      default:
        _push(const RestaurantShell());
    }
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      color: _pageBg,
      child: RefreshIndicator(
        color: kBrand,
        onRefresh: () async {
          setState(() => _bannerTick++);
          await Future.wait([
            _loadRestaurants(),
            _loadUnread(),
            _loadAddress(),
          ]);
        },
        child: ListView(
          // Pastdagi 96 — suzuvchi pastki menyu uchun. Usiz oxirgi
          // blok menyu ostida qolib ketardi.
          padding: const EdgeInsets.fromLTRB(16, 4, 16, 96),
          children: [
            _header(),
            const SizedBox(height: 10),
            _wordmark(),
            const SizedBox(height: 14),
            _search(),
            const SizedBox(height: 14),
            _serviceGrid(),
            const SizedBox(height: 14),
            _tileRow(),
            const SizedBox(height: 18),
            // Admin panelidagi "Menu Banner" bo'limi. Bo'sh bo'lsa butunlay yashirin.
            PromoBanners(
              placement: 'menu',
              aspectRatio: 3.1,
              tick: _bannerTick,
              onAction: _openBannerAction,
              spacing: 14,
              bottomGap: 14,
            ),
            _banner(),
            const SizedBox(height: 20),
            _restaurantsSection(),
          ],
        ),
      ),
    );
  }

  // ── Sarlavha: manzil + bildirishnoma + hamyon ──
  Widget _header() {
    return Row(
      children: [
        Expanded(
          child: InkWell(
            onTap: () => _push(const AddressScreen()),
            borderRadius: BorderRadius.circular(10),
            child: Padding(
              padding: const EdgeInsets.symmetric(vertical: 6),
              child: Row(
                children: [
                  const Icon(Icons.place_outlined, size: 19, color: _titleDark),
                  const SizedBox(width: 5),
                  Flexible(
                    child: Text(
                      _city,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        fontSize: 15.5,
                        fontWeight: FontWeight.w600,
                        color: _titleDark,
                      ),
                    ),
                  ),
                  const Icon(
                    Icons.keyboard_arrow_down,
                    size: 20,
                    color: _titleDark,
                  ),
                ],
              ),
            ),
          ),
        ),
        // Restoran bo'limidagi AYNAN o'sha tugmalar
        // (`widgets/header_action.dart`). Bu ekran o'zinikini qayta
        // chizmaydi — aks holda bitta ilovada bir xil tugma ikki xil
        // ko'rinardi.
        HeaderActionButton(
          icon: Icons.account_balance_wallet_outlined,
          label: 'Hamyon',
          onTap: () => _push(const WalletScreen()),
        ),
        const SizedBox(width: 8),
        HeaderActionButton(
          icon: Icons.notifications_none,
          label: 'Bildirishnomalar',
          badge: _unread,
          onTap: () => _push(const NotificationsScreen()),
        ),
      ],
    );
  }

  Widget _wordmark() {
    return const Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text.rich(
          TextSpan(
            children: [
              TextSpan(text: 'On', style: TextStyle(color: _titleDark)),
              TextSpan(text: 'Dex', style: TextStyle(color: kBrand)),
            ],
          ),
          style: TextStyle(
            fontSize: 34,
            fontWeight: FontWeight.w800,
            height: 1.0,
            letterSpacing: -0.8,
          ),
        ),
        SizedBox(height: 1),
        // Kirish ekranidagi (`widgets/auth_ui.dart`) shior bilan AYNAN
        // bir xil bo'lishi kerak — ikki joyda ikki xil tavsif turmasin.
        Text(
          'Xalq ilovasi',
          style: TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w500,
            color: _muted,
          ),
        ),
      ],
    );
  }

  // ── Qidiruv + QR skaneri ──
  //
  // Skaner belgisi AYNAN shu yerda (maketdagidek). Ilgari u pastki
  // menyuning markazidagi tugma edi; o'sha joy endi Shaddiy yordamchisi
  // uchun. Stol QR kodi baribir qidiruv bilan bir xil "biror narsa
  // topish" harakati, shuning uchun joyi mantiqan to'g'ri.
  Widget _search() {
    return Container(
      height: 46,
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(13),
        border: Border.all(color: _fieldBorder),
      ),
      child: Row(
        children: [
          const SizedBox(width: 12),
          const Icon(Icons.search, size: 20, color: _muted),
          const SizedBox(width: 8),
          Expanded(
            child: InkWell(
              onTap: () => _push(const RestaurantShell()),
              child: const Align(
                alignment: Alignment.centerLeft,
                child: Text(
                  'Xizmat yoki mahsulot qidiring...',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(fontSize: 13.5, color: _muted),
                ),
              ),
            ),
          ),
          InkResponse(
            radius: 22,
            onTap: widget.onScanQr,
            child: const Padding(
              padding: EdgeInsets.symmetric(horizontal: 12, vertical: 8),
              child: Icon(Icons.qr_code_scanner, size: 20, color: _muted),
            ),
          ),
        ],
      ),
    );
  }

  // ── Xizmat kartalari (2x2) ──
  //
  // `childAspectRatio` maketdan EMAS, tarkibdan olingan: telefonda
  // karta kengligi ~170pt bo'ladi va unga belgi + sarlavha + ikki
  // qatorli tavsif + tugma sig'ishi kerak. Maketning nisbatini
  // ko'chirsak, matn kesilardi.
  Widget _serviceGrid() {
    return GridView.count(
      crossAxisCount: 2,
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      // `padding: zero` SHART: `GridView` standart holatda MediaQuery
      // paddingini oladi va qidiruv maydoni bilan kartalar orasida
      // keraksiz bo'shliq paydo bo'ladi.
      padding: EdgeInsets.zero,
      mainAxisSpacing: 14,
      crossAxisSpacing: 12,
      // ┌─ ENI KENG, BO'YI PAST (maket: `image/image.png`) ──────────┐
      // Kvadrat kartalar ekranning yarmini yeb qo'yardi va pastdagi
      // bloklar ko'rinmasdi. Endi blok yassi: rasm kulrang maydon
      // ichida, nom esa uning OSTIDA, maydondan tashqarida.
      //
      // Rasm blokning TEPASIDAN chiqib turadi, shuning uchun
      // katakcha bir oz balandroq: 1.7 -> 1.58.
      // └────────────────────────────────────────────────────────────┘
      childAspectRatio: 1.58,
      children: [
        for (final s in _services)
          _ServiceCard(service: s, onTap: () => _openService(s)),
      ],
    );
  }

  // ── To'rtta kichik plitka ──
  Widget _tileRow() {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        for (var i = 0; i < _tiles.length; i++) ...[
          if (i > 0) const SizedBox(width: 10),
          Expanded(
            child: _Tile(
              service: _tiles[i],
              onTap: () => _openService(_tiles[i]),
            ),
          ),
        ],
      ],
    );
  }

  // ── Reklama bloki (`image/OnDexGameBar.png` maketi) ──
  //
  // ┌─ RASM MATN ORQASIDA, YONIDA EMAS ───────────────────────────────┐
  // Ilgari matn va rasm Row ichida YONMA-YON turardi (ikkita ustun).
  // Yangi maket (`OnDexGame.png`) esa BUTUN blokni qoplaydigan fon —
  // bayroq va Registon chap tomondagi bo'sh joy ustiga matn YOZILADI.
  // Shuning uchun endi `DecorationImage` orqa fon, matn esa uning
  // USTIDA (Stack emas, `Container.decoration` — bitta qatlam yetarli).
  // └────────────────────────────────────────────────────────────────┘
  // Eski bannerning balandligi — bo'yi "to'rtburchak"ka aylanmasin,
  // matn uch bo'lak (sarlavha, tavsif, tugma) shu balandlikka
  // `spaceBetween` bilan taqsimlanadi.
  static const _bannerHeight = 150.0;

  Widget _banner() {
    return ClipRRect(
      borderRadius: BorderRadius.circular(20),
      child: Container(
        height: _bannerHeight,
        decoration: const BoxDecoration(
          color: _bannerBg,
          image: DecorationImage(
            image: AssetImage('assets/services/ondex_game.png'),
            fit: BoxFit.cover,
          ),
        ),
        padding: const EdgeInsets.fromLTRB(16, 14, 16, 14),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            // "On" va "Dex" ikki xil rangda — xuddi `_wordmark()`dagi
            // brend yozuvi kabi (shu fayldagi yagona joy emas, lekin
            // bannerda ham brend darhol tanilsin).
            const Text.rich(
              TextSpan(
                children: [
                  TextSpan(text: 'On', style: TextStyle(color: _titleDark)),
                  TextSpan(text: 'Dex', style: TextStyle(color: kBrand)),
                  TextSpan(
                    text: ' Simulatsiya\nO\'yini',
                    style: TextStyle(color: _titleDark),
                  ),
                ],
              ),
              style: TextStyle(
                fontSize: 16.5,
                fontWeight: FontWeight.bold,
                height: 1.2,
              ),
            ),
            const Text(
              'Shahringni qur, biznesingni boshqar\nva g\'alabaga erish!',
              style: TextStyle(fontSize: 11.5, color: _muted, height: 1.25),
            ),
            SizedBox(
              height: 32,
              child: FilledButton(
                style: FilledButton.styleFrom(
                  backgroundColor: kBrand,
                  padding: const EdgeInsets.symmetric(horizontal: 18),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(16),
                  ),
                ),
                onPressed: () => _push(const ComingSoonScreen(
                  title: 'OnDex Simulatsiya O\'yini',
                  heading: 'O\'yin tez orada ishga tushadi',
                  subtitle: 'O\'z biznesingni ochish imkoniyati '
                      'tez orada mavjud bo\'ladi! \u{1F1FA}\u{1F1FF}',
                  mascotAsset: 'assets/services/ondex_game.png',
                  mascotWidth: 280,
                )),
                child: const Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(
                      'O\'ynash',
                      style: TextStyle(
                        fontSize: 12.5,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    SizedBox(width: 5),
                    Icon(Icons.arrow_forward, size: 15),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  // ── Restoranlar ──
  //
  // ┌─ MAKETDAGI "AKTUAL TAKLIFLAR" DAN FARQI ───────────────────────┐
  // Maketda bu joyda chegirma kartalari turadi ("-20% Pizza", "1+1"...).
  // Ular HAQIQIY bo'lishi uchun barcha restoranlar bo'yicha umumiy
  // aksiyalar endpointi kerak — backend'da esa faqat
  // `GET /restaurants/{id}/active-promotions` bor, ya'ni har restoran
  // uchun alohida so'rov.
  //
  // Yozib qo'yilgan "-20%" kabi raqamlarni chizish MUMKIN EMAS: mijoz
  // ularni haqiqiy chegirma deb o'qiydi. Shuning uchun bu blok
  // maketning SHAKLINI (gorizontal kartalar qatori) saqlaydi, lekin
  // HAQIQIY ma'lumot — ochiq restoranlarni ko'rsatadi.
  // └────────────────────────────────────────────────────────────────┘
  Widget _restaurantsSection() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            const Text(
              'Restoranlar',
              style: TextStyle(
                fontSize: 18.5,
                fontWeight: FontWeight.bold,
                color: _titleDark,
              ),
            ),
            InkWell(
              onTap: () => _push(const RestaurantShell()),
              child: const Row(
                children: [
                  Text(
                    'Barchasini ko\'rish',
                    style: TextStyle(fontSize: 13.5, color: _muted),
                  ),
                  Icon(Icons.chevron_right, size: 18, color: _muted),
                ],
              ),
            ),
          ],
        ),
        const SizedBox(height: 12),
        SizedBox(
          height: 152,
          child:
              _loadingRestaurants
                  ? const Center(
                    child: SizedBox(
                      width: 22,
                      height: 22,
                      child: CircularProgressIndicator(
                        strokeWidth: 2,
                        color: kBrand,
                      ),
                    ),
                  )
                  : _restaurants.isEmpty
                  // ⚠️ Ro'yxat SERVERDA manzil shahri bo'yicha
                  // filtrlanadi (`visibleRestaurants`). Manzil
                  // tanlangan bo'lsa bo'sh ro'yxat "umuman yo'q"
                  // degani emas — "shu shaharda yo'q" degani, va
                  // matn buni aytishi kerak.
                  ? Center(
                    child: Padding(
                      padding: const EdgeInsets.symmetric(horizontal: 24),
                      child: Text(
                        _city == kNoAddressLabel
                            ? 'Hozircha restoran yo\'q'
                            : 'Manzilingiz hududida hali restoran yo\'q',
                        textAlign: TextAlign.center,
                        style: const TextStyle(fontSize: 13.5, color: _muted),
                      ),
                    ),
                  )
                  : ListView.separated(
                    scrollDirection: Axis.horizontal,
                    itemCount: _restaurants.length,
                    separatorBuilder: (_, _) => const SizedBox(width: 12),
                    itemBuilder: (_, i) {
                      final r = Map<String, dynamic>.from(
                        _restaurants[i] as Map,
                      );
                      return _RestaurantCard(
                        restaurant: r,
                        onTap:
                            () => MenuScreen.open(
                              context,
                              (r['id'] ?? '').toString(),
                              fallbackName: (r['name'] ?? '').toString(),
                            ),
                      );
                    },
                  ),
        ),
      ],
    );
  }
}

// ── Qismlar ──

class _ServiceCard extends StatelessWidget {
  final _Service service;
  final VoidCallback onTap;

  const _ServiceCard({required this.service, required this.onTap});

  @override
  Widget build(BuildContext context) {
    // ┌─ RASM BLOKDAN CHIQIB TURADI ───────────────────────────────────┐
    // Maketdagi (`image/image.png`) uslub: kulrang yassi maydon, nom
    // uning OSTIDA, rasm esa maydonning tepasidan biroz CHIQIB turadi
    // — shu tufayli u kattaroq ko'rinadi va blok baland bo'lmaydi.
    //
    // Buning uchun maydon yuqoridan `_overhang` piksel pastda
    // boshlanadi, rasm esa katakning eng tepasidan chiziladi. Ikkalasi
    // ham katak ICHIDA qoladi, ya'ni `GridView` hech narsani kesmaydi.
    //
    // Rasmlar shaffof fonli (`assets/services/`), shuning uchun
    // chiqib turgan qismi maydondan tashqarida ham toza ko'rinadi.
    // └────────────────────────────────────────────────────────────────┘
    const overhang = 14.0;

    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(18),
      child: Column(
        children: [
          Expanded(
            child: Stack(
              children: [
                Positioned(
                  left: 0,
                  right: 0,
                  top: overhang,
                  bottom: 0,
                  child: Container(
                    decoration: BoxDecoration(
                      color: _tileBg,
                      borderRadius: BorderRadius.circular(18),
                    ),
                  ),
                ),
                Positioned(
                  left: 10,
                  right: 10,
                  top: 0,
                  bottom: 6,
                  child: Image.asset(
                    service.image,
                    fit: BoxFit.contain,
                    // Rasm topilmasa blok bo'sh qolmasin — xizmat
                    // belgisi o'rnini bosadi.
                    errorBuilder:
                        (_, _, _) =>
                            Icon(service.icon, size: 40, color: service.color),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 7),
          Text(
            service.title,
            textAlign: TextAlign.center,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: const TextStyle(
              fontSize: 13.5,
              fontWeight: FontWeight.w600,
              color: _titleDark,
            ),
          ),
        ],
      ),
    );
  }
}

class _Tile extends StatelessWidget {
  final _Service service;
  final VoidCallback onTap;

  const _Tile({required this.service, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(16),
      child: Column(
        children: [
          Container(
            height: 72,
            padding: const EdgeInsets.all(9),
            decoration: BoxDecoration(
              color: _tileBg,
              borderRadius: BorderRadius.circular(16),
            ),
            child: Image.asset(
              service.image,
              fit: BoxFit.contain,
              errorBuilder:
                  (_, _, _) =>
                      Icon(service.icon, size: 28, color: service.color),
            ),
          ),
          const SizedBox(height: 7),
          Text(
            service.title,
            textAlign: TextAlign.center,
            maxLines: 2,
            style: const TextStyle(
              fontSize: 11.5,
              height: 1.28,
              fontWeight: FontWeight.w500,
              color: _titleDark,
            ),
          ),
        ],
      ),
    );
  }
}

class _RestaurantCard extends StatelessWidget {
  final Map<String, dynamic> restaurant;
  final VoidCallback onTap;

  const _RestaurantCard({required this.restaurant, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final name = (restaurant['name'] ?? '').toString();
    // "Ochiq/Yopiq" — ish vaqti bilan (server hisobi, `ondex_core`).
    final open =
        RestaurantOpenStatus.fromJson(restaurant).at(DateTime.now()).open;
    final cover = fullImageUrl((restaurant['cover_url'] ?? '').toString());

    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(16),
      child: SizedBox(
        width: 210,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(
              // ┌─ KARTA CHEGARASI KO'RINIB TURSIN ──────────────────────┐
              // Restoranlar muqova sifatida ko'pincha OQ fonli logotip
              // qo'yadi. Chegarasiz bunday rasm oq sahifa foniga qo'shilib
              // ketadi: karta yo'qoladi va logotip ekranda "osilib" qolgandek
              // ko'rinadi (Book Cafe'da aynan shunday edi).
              // └────────────────────────────────────────────────────────┘
              child: DecoratedBox(
                decoration: BoxDecoration(
                  color: _tileBg,
                  borderRadius: BorderRadius.circular(16),
                  border: Border.all(color: const Color(0xFFE5E7EB)),
                ),
                child: Stack(
                  fit: StackFit.expand,
                  children: [
                    ClipRRect(
                      borderRadius: BorderRadius.circular(15),
                      child:
                          cover.isEmpty
                              ? Container(color: _tileBg)
                              : Image.network(
                                cover,
                                fit: BoxFit.cover,
                                errorBuilder:
                                    (_, _, _) => Container(color: _tileBg),
                              ),
                    ),
                    Positioned(
                      top: 8,
                      right: 8,
                      child: Container(
                        padding: const EdgeInsets.symmetric(
                          horizontal: 10,
                          vertical: 4,
                        ),
                        decoration: BoxDecoration(
                          color: Colors.white,
                          borderRadius: BorderRadius.circular(20),
                          // Oq muqovada oq yorliq ko'rinmay qolmasin.
                          border: Border.all(color: const Color(0xFFE5E7EB)),
                        ),
                        child: Text(
                          open ? 'Ochiq' : 'Yopiq',
                          style: TextStyle(
                            fontSize: 11.5,
                            fontWeight: FontWeight.w600,
                            color:
                                open
                                    ? const Color(0xFF16A34A)
                                    : const Color(0xFF9CA3AF),
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
            const SizedBox(height: 7),
            Text(
              name,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(
                fontSize: 14,
                fontWeight: FontWeight.w600,
                color: _titleDark,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

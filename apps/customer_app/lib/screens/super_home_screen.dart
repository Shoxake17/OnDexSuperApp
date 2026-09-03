import 'package:flutter/material.dart';

import '../api.dart';
import '../widgets/header_action.dart';
import '../widgets/sheet_page.dart';
import 'address_screen.dart';
import 'catalog_screen.dart' show kBrand;
import 'menu_screen.dart';
import 'notifications_screen.dart';
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
/// Maketda sakkizta xizmat bor, lekin bugungi kunda backend'da FAQAT
/// restoran oqimi mavjud. Qolganlari ko'rinadi (maket buzilmasin), lekin
/// "Tez orada" belgisi bilan va bosilganda shuni aytadi.
///
/// Ular uchun "ishlaydigandek" tugma qo'yish eng yomon variant bo'lardi:
/// foydalanuvchi bosadi, hech narsa bo'lmaydi va ilova buzuq deb
/// o'ylaydi. `_Service.soon` maydoni shu farqni bitta joyda saqlaydi —
/// xizmat ishga tushganda `onTap` beriladi va bayroq olib tashlanadi.
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

  /// Xizmat hali ishga tushmagan. Kartada BELGILANMAYDI (ko'rinish
  /// hamma xizmatda bir xil), lekin bosilganda aniq aytiladi.
  final bool soon;

  const _Service({
    required this.title,
    required this.icon,
    required this.color,
    required this.image,
    this.soon = false,
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
    soon: true,
  ),
  _Service(
    title: 'Computer Club',
    icon: Icons.sports_esports,
    color: Color(0xFF7C3AED),
    image: 'assets/services/club.png',
    soon: true,
  ),
  _Service(
    title: 'Uy Joy',
    icon: Icons.home_work,
    color: Color(0xFF0EA5A5),
    image: 'assets/services/uyjoy.png',
    soon: true,
  ),
];

const _tiles = <_Service>[
  _Service(
    title: 'Moshina\nbozori',
    icon: Icons.directions_car,
    color: Color(0xFF334155),
    image: 'assets/services/moshina.png',
    soon: true,
  ),
  _Service(
    title: 'OnDex Xarita',
    icon: Icons.place,
    color: kBrand,
    image: 'assets/services/map.png',
    soon: true,
  ),
  _Service(
    title: 'Ish joy\ne\'lonlari',
    icon: Icons.work,
    color: Color(0xFF92400E),
    image: 'assets/services/ishjoy.png',
    soon: true,
  ),
  _Service(
    title: 'OnDex Taxi\nxizmati',
    icon: Icons.local_taxi,
    color: Color(0xFFF59E0B),
    image: 'assets/services/taxi.png',
    soon: true,
  ),
];

class _SuperHomeScreenState extends State<SuperHomeScreen> {
  int _unread = 0;
  String _city = 'Chust';
  List<dynamic> _restaurants = const [];
  bool _loadingRestaurants = true;

  @override
  void initState() {
    super.initState();
    _loadUnread();
    _loadAddress();
    _loadRestaurants();
  }

  /// Bildirishnoma belgisi — xatosi JIM yutiladi (bezak ma'lumoti,
  /// usiz ham sahifa to'liq ishlaydi).
  Future<void> _loadUnread() async {
    try {
      final n = await api.unreadNotificationCount();
      if (mounted) setState(() => _unread = n);
    } catch (_) {}
  }

  Future<void> _loadAddress() async {
    try {
      final a = await api.getMyAddress();
      final t = (a['text'] ?? '').toString().trim();
      if (mounted && t.isNotEmpty) setState(() => _city = t);
    } catch (_) {}
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
    if (mounted) _loadUnread();
  }

  void _soon(String name) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text('$name — tez orada ishga tushadi'),
        behavior: SnackBarBehavior.floating,
      ),
    );
  }

  void _openService(_Service s) {
    if (s.soon) {
      _soon(s.title.replaceAll('\n', ' '));
      return;
    }
    _push(const RestaurantShell());
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      color: _pageBg,
      child: RefreshIndicator(
        color: kBrand,
        onRefresh: () async {
          await Future.wait(
              [_loadRestaurants(), _loadUnread(), _loadAddress()]);
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
                  const Icon(Icons.place_outlined,
                      size: 19, color: _titleDark),
                  const SizedBox(width: 5),
                  Flexible(
                    child: Text(
                      _city,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                          fontSize: 15.5,
                          fontWeight: FontWeight.w600,
                          color: _titleDark),
                    ),
                  ),
                  const Icon(Icons.keyboard_arrow_down,
                      size: 20, color: _titleDark),
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
              letterSpacing: -0.8),
        ),
        SizedBox(height: 1),
        Text('Super App',
            style: TextStyle(
                fontSize: 14, fontWeight: FontWeight.w500, color: _muted)),
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
                child: Text('Xizmat yoki mahsulot qidiring...',
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(fontSize: 13.5, color: _muted)),
              ),
            ),
          ),
          InkResponse(
            radius: 22,
            onTap: widget.onScanQr,
            child: const Padding(
              padding: EdgeInsets.symmetric(horizontal: 12, vertical: 8),
              child:
                  Icon(Icons.qr_code_scanner, size: 20, color: _muted),
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

  // ── Reklama bloki ──
  Widget _banner() {
    return ClipRRect(
      borderRadius: BorderRadius.circular(20),
      child: Container(
        color: _bannerBg,
        padding: const EdgeInsets.fromLTRB(18, 18, 0, 18),
        child: Row(
          children: [
            Expanded(
              flex: 52,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Text.rich(
                    TextSpan(
                      children: [
                        TextSpan(
                            text: 'OnDex ', style: TextStyle(color: kBrand)),
                        TextSpan(
                            text: 'bilan\nhayotingiz oson!',
                            style: TextStyle(color: _titleDark)),
                      ],
                    ),
                    style: TextStyle(
                        fontSize: 19,
                        fontWeight: FontWeight.bold,
                        height: 1.28),
                  ),
                  const SizedBox(height: 7),
                  const Text('Barcha xizmatlar bir joyda',
                      style: TextStyle(fontSize: 13, color: _muted)),
                  const SizedBox(height: 14),
                  SizedBox(
                    height: 38,
                    child: FilledButton(
                      style: FilledButton.styleFrom(
                        backgroundColor: kBrand,
                        padding: const EdgeInsets.symmetric(horizontal: 22),
                        shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.circular(19)),
                      ),
                      onPressed: () => _push(const RestaurantShell()),
                      child: const Text('Batafsil',
                          style: TextStyle(
                              fontSize: 13.5, fontWeight: FontWeight.w600)),
                    ),
                  ),
                ],
              ),
            ),
            Expanded(
              flex: 48,
              child: Image.asset('assets/services/banner.png',
                  fit: BoxFit.contain),
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
            const Text('Restoranlar',
                style: TextStyle(
                    fontSize: 18.5,
                    fontWeight: FontWeight.bold,
                    color: _titleDark)),
            InkWell(
              onTap: () => _push(const RestaurantShell()),
              child: const Row(
                children: [
                  Text('Barchasini ko\'rish',
                      style: TextStyle(fontSize: 13.5, color: _muted)),
                  Icon(Icons.chevron_right, size: 18, color: _muted),
                ],
              ),
            ),
          ],
        ),
        const SizedBox(height: 12),
        SizedBox(
          height: 152,
          child: _loadingRestaurants
              ? const Center(
                  child: SizedBox(
                    width: 22,
                    height: 22,
                    child: CircularProgressIndicator(
                        strokeWidth: 2, color: kBrand),
                  ),
                )
              : _restaurants.isEmpty
                  ? const Center(
                      child: Text('Hozircha restoran yo\'q',
                          style: TextStyle(fontSize: 13.5, color: _muted)),
                    )
                  : ListView.separated(
                      scrollDirection: Axis.horizontal,
                      itemCount: _restaurants.length,
                      separatorBuilder: (_, __) => const SizedBox(width: 12),
                      itemBuilder: (_, i) {
                        final r =
                            Map<String, dynamic>.from(_restaurants[i] as Map);
                        return _RestaurantCard(
                          restaurant: r,
                          onTap: () => MenuScreen.open(
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
                    errorBuilder: (_, __, ___) =>
                        Icon(service.icon, size: 40, color: service.color),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 7),
          Text(service.title,
              textAlign: TextAlign.center,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(
                  fontSize: 13.5,
                  fontWeight: FontWeight.w600,
                  color: _titleDark)),
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
              errorBuilder: (_, __, ___) =>
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
                color: _titleDark),
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
    final open = restaurant['open'] == true;
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
              child: Stack(
                fit: StackFit.expand,
                children: [
                  ClipRRect(
                    borderRadius: BorderRadius.circular(16),
                    child: cover.isEmpty
                        ? Container(color: _tileBg)
                        : Image.network(cover,
                            fit: BoxFit.cover,
                            errorBuilder: (_, __, ___) =>
                                Container(color: _tileBg)),
                  ),
                  Positioned(
                    top: 8,
                    right: 8,
                    child: Container(
                      padding: const EdgeInsets.symmetric(
                          horizontal: 10, vertical: 4),
                      decoration: BoxDecoration(
                        color: Colors.white,
                        borderRadius: BorderRadius.circular(20),
                      ),
                      child: Text(
                        open ? 'Ochiq' : 'Yopiq',
                        style: TextStyle(
                          fontSize: 11.5,
                          fontWeight: FontWeight.w600,
                          color: open
                              ? const Color(0xFF16A34A)
                              : const Color(0xFF9CA3AF),
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 7),
            Text(name,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                    color: _titleDark)),
          ],
        ),
      ),
    );
  }
}

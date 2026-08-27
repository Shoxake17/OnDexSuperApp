import 'package:flutter/material.dart';
// `SystemUiOverlayStyle` uchun — tizim paneli uslubi shu ekranda
// belgilanadi (izohga qarang).
import 'package:flutter/services.dart';

import '../data/cart_store.dart';
import '../services/push.dart';
import '../widgets/page_sheet.dart';
import 'catalog_screen.dart';
import 'favorites_screen.dart';
import 'menu_screen.dart';
import 'orders_screen.dart';
import 'profile_screen.dart';
import 'qr_scan_screen.dart';

/// Ilovaning pastki menyu (bottom navigation) qobig'i — Yandex Eats/Wolt
/// uslubida to'rtta bo'lim: Bosh sahifa (restoranlar), Istaklarim (yurak
/// belgisi bilan saqlangan taomlar), Buyurtmalarim (buyurtmalar tarixi va
/// holati), Profil. Har bir bo'lim `IndexedStack` ichida saqlanadi — tab
/// almashtirilganda oldingi holat (masalan restoranlar ro'yxatining
/// skroll pozitsiyasi) yo'qolmaydi.
class HomeShell extends StatefulWidget {
  const HomeShell({super.key});

  @override
  State<HomeShell> createState() => _HomeShellState();
}

class _HomeShellState extends State<HomeShell> {
  static const _favoritesTabIndex = 1;

  int _index = 0;
  final _favoritesKey = GlobalKey<FavoritesScreenState>();

  // `_miniAppKey` va `_webViewUrl` OLIB TASHLANDI (2-bosqich): bosh
  // sahifa native bo'lgach doimiy WebView qolmadi. Menyu, savat va
  // checkout endi alohida PUSH qilingan ekranlar sifatida ochiladi —
  // ular o'z `Scaffold`iga ega, ya'ni pastki menyuni qo'lda yashirish
  // ham kerak emas (eski `_hideBottomBar` mantig'i shu sabab o'chdi).

  // IndexedStack BARCHA tab'larni doim "tirik" ushlab turadi — tab
  // almashtirilganda initState() qayta chaqirilmaydi. Shuning uchun
  // boshqa ekranda (masalan menyu sahifasida) bosilgan yurak belgisi
  // "Istaklarim" tab'ida ko'rinmay, faqat ilova to'liq qayta ishga
  // tushganda yangilanardi. Endi _tabs GlobalKey bilan quriladi va
  // onDestinationSelected() Istaklarim tab'iga o'tilganda ANIQ shu
  // ekranni majburan qayta yuklaydi — real vaqtda, refreshsiz.
  // Home tab endi apps/web (Next.js) mini-app'ini WebView orqali ochadi —
  // /api/bridge tokenni bir martalik httpOnly cookie'ga o'tkazadi, shundan
  // keyin Home/Menyu/Savat/Checkout/Active-Order — hammasi shu WebView
  // ichida, Next.js'ning o'z router'i orqali (native Navigator'siz).
  late final _tabs = [
    // 2-BOSQICH: bosh sahifa endi NATIVE (`catalog_screen.dart`).
    // Avval bu yerda `MiniAppWebView` turardi va ilova har ochilganda
    // butun Next.js sahifasini tarmoqdan yuklardi.
    const CatalogScreen(),
    FavoritesScreen(key: _favoritesKey),
    const OrdersScreen(),
    const ProfileScreen(),
  ];

  @override
  void initState() {
    super.initState();
    // Push tokenini ro'yxatdan o'tkazish AYNAN SHU YERDA.
    //
    // NEGA bu joy: `HomeShell` — sessiya ochilganidan keyingi YAGONA
    // kirish nuqtasi. Kirishning beshta yo'li bor (parol, SMS kodi,
    // Telegram, Google, parolni tiklash) va ularning har biriga
    // alohida chaqiruv qo'yilsa, kelajakda qo'shiladigan oltinchi yo'l
    // jimgina unutilardi — foydalanuvchi push olmay qo'yardi va buni
    // hech kim sezmasdi.
    //
    // `await` qilinmaydi: tarmoq so'rovi ekran chizilishini kutib
    // turmasligi kerak.
    PushService.instance.start();
  }

  void _selectTab(int i) {
    setState(() => _index = i);
    if (i == _favoritesTabIndex) {
      _favoritesKey.currentState?.reload(silent: true);
    }
  }

  @override
  Widget build(BuildContext context) {
    // ┌─ TIZIM PANELI USLUBI SHU YERDA ─────────────────────────────┐
    // Ilgari uni har ekran o'zi belgilardi, lekin ular `SafeArea`
    // ICHIDA edi — ya'ni belgilangan hudud ekranning eng tepasiga
    // YETMASDI va Flutter uslubni umuman qo'llamasdi. Natijada bosh
    // sahifada soat va Wi-Fi belgilar oq holicha qolib, oq fonda
    // ko'rinmay ketardi.
    //
    // Bu yerda `AnnotatedRegion` butun ekranni qamraydi va menyudan
    // qaytilganda uslub o'z-o'zidan tiklanadi.
    // └─────────────────────────────────────────────────────────────┘
    return AnnotatedRegion<SystemUiOverlayStyle>(
      // Barcha tab'lar bir xil: OQ tizim paneli, qora belgilar. Ular
      // dumaloq kartasiz, oq fon bilan boshlanadi. Qora panel faqat
      // PUSH qilingan sahifalarda (`PageSheet`).
      value: PageSheet.light,
      child: Scaffold(
      // ┌─ FON HAR DOIM OQ ───────────────────────────────────────────┐
      // Ilgari bu rang tab'ga qarab QORA bo'lardi — u tizim paneli
      // ostidagi chiziqni bo'yash uchun edi. Lekin Scaffold foni
      // BUTUN ekranni bo'yaydi, shu jumladan pastki menyuning
      // ko'tarilgan QR tugmasi uchun qoldirilgan shaffof joyni ham.
      // Natijada menyu ustida QORA chiziq paydo bo'lgan edi.
      //
      // Endi tizim paneli ostidagi chiziq `body` ichida ALOHIDA
      // chiziladi (pastga qarang) va pastki menyuga tegmaydi.
      // └─────────────────────────────────────────────────────────────┘
      backgroundColor: Colors.white,
      // MUHIM (haqiqiy Android qurilmada topilgan bug): IndexedStack
      // WebView (platform view) bilan ishlatilganda — hatto faqat BITTA
      // tab platform view bo'lsa ham — WebView'ga teginish UMUMAN
      // yetib bormay qoladi (Flutter/webview_flutter'ning tanilgan
      // muammosi). IndexedStack o'rniga Stack+Visibility(maintainState)
      // ishlatildi — bir xil holat saqlash xususiyati, lekin
      // hit-testing to'g'ri ishlaydi (nofaol tab'lar chindan ham
      // teginishdan chiqarib tashlanadi, IndexedStack'dagidek emas).
      // MUHIM (foydalanuvchi aniq so'radi): WebView avval EDGE-TO-EDGE
      // chizilardi — status-bar/pastki gesture-bar ORQASIDA ham ilova
      // fonini (CSS orqali) chizib, tizim panellari "shaffof" bo'lib
      // ko'rinardi. Endi butun Stack `SafeArea` bilan o'raladi — bu
      // WebView'ni FAQAT tizim panellari EGALLAMAGAN hududda chizadi,
      // ularning orqasi Android'ning o'z (ilova chizmagan) foniga
      // qaytadi. `bottomNavigationBar` mavjud bo'lganda Scaffold
      // `body`ning pastki MediaQuery paddingini avtomatik nolga
      // tushiradi (allaqachon hisobga olingan) — shuning uchun
      // SafeArea'ning pastki paddingi FAQAT `bottomNavigationBar` yo'q
      // holatda (Menyu/Savat/Checkout — _hideBottomBar=true) haqiqiy
      // ishlaydi, ikki marta qo'shilib ketmaydi.
      body: Column(
        children: [
          // Tizim paneli egallagan chiziq — barcha tab'larda OQ.
          //
          // `SafeArea` o'rniga aniq balandlik: SafeArea butun `body`ni
          // qamrab, pastki menyuga ham ta'sir qilardi.
          Container(
            height: MediaQuery.of(context).padding.top,
            color: Colors.white,
          ),
          Expanded(
            child: Stack(
              children: [
                for (var i = 0; i < _tabs.length; i++)
                  Visibility(
                    visible: i == _index,
                    maintainState: true,
                    child: _tabs[i],
                  ),
              ],
            ),
          ),
        ],
      ),
      // Pastki menyu endi HAR DOIM ko'rinadi. "Chuqurlashgan" oqim
      // (menyu, savat, checkout) alohida PUSH qilingan ekranlarda
      // ochiladi — ular bu Scaffold'ni butunlay bosib turadi, ya'ni
      // menyuni qo'lda yashirish kerak emas.
      // ┌─ QR TUGMASI — SUZUVCHI ─────────────────────────────────────┐
      // Ilgari u pastki panelning ICHIDA edi va panel uning
      // ko'tarilishi uchun 28px qo'shimcha balandlik ajratardi. O'sha
      // bo'sh oq chiziq tarkibni bosib turardi.
      //
      // Endi tugma `floatingActionButton` — u panel ustida SUZADI,
      // hech qanday joy egallamaydi. `centerDocked` uni panelning
      // yuqori chetiga aniq markazlaydi.
      // └─────────────────────────────────────────────────────────────┘
      floatingActionButton: _QrButton(onTap: _scanTableQr),
      floatingActionButtonLocation: FloatingActionButtonLocation.centerDocked,
      bottomNavigationBar: _OndexBottomBar(
        currentTab: _index,
        onSelectTab: _selectTab,
      ),
    ));
  }

  /// Markaziy QR tugmasi — stol skanerini ochadi.
  ///
  /// ┌─ ISH TAQSIMOTI ───────────────────────────────────────────────┐
  /// `QrScanScreen`  — kamera, QR o'qish va tokenni SERVERDA yechish
  ///                   (`GET /tables/resolve`). Xato matnlari native
  ///                   ekranda ko'rsatiladi.
  /// Bu metod        — natijani Home tab'idagi mini-app sahifasiga
  ///                   uzatadi.
  /// Sahifa          — seansni yozadi, savat qoidasini qo'llaydi va
  ///                   menyuni ochadi (`lib/open-table.ts`) — Telegram
  ///                   Mini App bilan AYNAN bir xil kod.
  /// └───────────────────────────────────────────────────────────────┘
  ///
  /// Tab 0 ga o'tish MAJBURIY va uzatishdan OLDIN bajariladi: mini-app
  /// WebView'i faqat o'sha tab'da yashaydi. Foydalanuvchi skanerni
  /// "Profil" tab'ida turib ochsa, menyu ko'rinmas tab'da ochilib,
  /// ekranda hech narsa o'zgarmagandek tuyulardi.
  Future<void> _scanTableQr() async {
    final result = await Navigator.of(context).push<TableScanResult>(
      MaterialPageRoute(builder: (_) => const QrScanScreen()),
    );
    if (result == null || !mounted) return; // foydalanuvchi yopdi

    // Menyu endi NATIVE — JS ko'prigi umuman ishlatilmaydi.
    //
    // Stol seansi savatga yoziladi: shu paytdan boshlab buyurtma
    // `table_token` bilan yuboriladi va server uni `dine_in` deb
    // belgilaydi (`routes_orders.go`). Savat boshqa restoranniki
    // bo'lsa `startTableSession` uni o'zi tozalaydi.
    CartStore.instance.startTableSession(
      restaurantId: result.restaurantId,
      token: result.token,
      tableLabel: result.tableLabel,
      restaurantName: result.restaurantName,
    );

    await MenuScreen.open(
      context,
      result.restaurantId,
      fallbackName: result.restaurantName,
    );
  }
}

const _kBrand = Color(0xFFF4511E);

/// Maketdagi (image/restarant.png) pastki menyu: to'rtta bo'lim va
/// markazda ko'tarilib turgan QR tugmasi.
///
/// `NavigationBar` O'RNIGA qo'lda qurilgan — Material'ning
/// `NavigationBar`i markazga tugma qo'ya olmaydi, `BottomAppBar`ning
/// o'yig'i (notch) esa faqat `Scaffold.floatingActionButton` bilan
/// ishlaydi va u SafeArea/`_hideBottomBar` mantig'ini murakkablashtirardi.
///
/// MUHIM: bo'limlar ko'rinish tartibi tab'lar tartibidan FARQ QILADI.
/// Maketda "Buyurtmalar" ikkinchi, "Sevimlilar" to'rtinchi; `_tabs`
/// ro'yxatida esa Sevimlilar 1, Buyurtmalar 2. Ro'yxatni qayta
/// tartiblash `_favoritesTabIndex` va holat saqlash mantig'iga tegardi,
/// shuning uchun faqat KO'RINISH tartibi mos keladi — har tugma o'z
/// tab indeksini olib yuradi.
class _OndexBottomBar extends StatelessWidget {
  final int currentTab;
  final ValueChanged<int> onSelectTab;

  const _OndexBottomBar({
    required this.currentTab,
    required this.onSelectTab,
  });

  /// Panelning o'z balandligi (tizim chekinishisiz).
  static const _barHeight = 64.0;

  @override
  Widget build(BuildContext context) => _bar(Theme.of(context));

  Widget _bar(ThemeData theme) {
    return Material(
      // Panel rangi ATAYLAB qat'iy oq: `colorScheme.surface` mavzuga
      // bog'liq va qurilma sozlamasiga qarab kulrang tusga kirardi.
      color: Colors.white,
      elevation: 8,
      // ┌─ SAFEAREA SHART ────────────────────────────────────────────┐
      // Busiz menyu ekranning eng pastiga chizilardi va Android'ning
      // TIZIM navigatsiya paneli uning ustiga tushardi: uch tugmali
      // navigatsiyada (48dp) "Bosh sahifa"/"Buyurtmalar" yozuvlari
      // tizim tugmalari ostida qolib, QR tugmasi yarim yashirinardi.
      //
      // Jest navigatsiyali telefonda past chiziq atigi ~24dp bo'lgani
      // uchun bu deyarli sezilmasdi — xato AYNAN uch tugmali qurilmada
      // ko'rindi.
      //
      // `top: false` — yuqori chekinish bu yerda keraksiz, uni
      // `Scaffold.body` dagi SafeArea allaqachon qo'llaydi.
      // └─────────────────────────────────────────────────────────────┘
      child: SafeArea(
        top: false,
        child: SizedBox(
          height: _barHeight,
          child: Row(
            children: [
              _NavItem(
                icon: Icons.home_outlined,
                activeIcon: Icons.home,
                label: 'Bosh sahifa',
                selected: currentTab == 0,
                onTap: () => onSelectTab(0),
              ),
              _NavItem(
                icon: Icons.receipt_long_outlined,
                activeIcon: Icons.receipt_long,
                label: 'Buyurtmalar',
                selected: currentTab == 2,
                onTap: () => onSelectTab(2),
              ),
              // Markazdagi bo'sh joy — QR tugmasi bu qatorda EMAS,
              // u Stack orqali tepadan tushib turadi.
              const Expanded(child: SizedBox.shrink()),
              _NavItem(
                icon: Icons.favorite_border,
                activeIcon: Icons.favorite,
                label: 'Sevimlilar',
                selected: currentTab == 1,
                onTap: () => onSelectTab(1),
              ),
              _NavItem(
                icon: Icons.person_outline,
                activeIcon: Icons.person,
                label: 'Profil',
                selected: currentTab == 3,
                onTap: () => onSelectTab(3),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _QrButton extends StatelessWidget {
  final VoidCallback onTap;

  const _QrButton({required this.onTap});

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: 'Stol QR kodi',
      child: InkResponse(
        onTap: onTap,
        radius: 38,
        child: Container(
          width: 64,
          height: 64,
          // Oq halqa — tugma panel ustiga "o'yib" qo'yilgandek
          // ko'rinadi. Busiz u shunchaki panelga yopishgan doira
          // bo'lib qolardi.
          padding: const EdgeInsets.all(4),
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            color: Colors.white,
            boxShadow: [
              BoxShadow(
                color: _kBrand.withValues(alpha: 0.35),
                blurRadius: 14,
                offset: const Offset(0, 5),
              ),
            ],
          ),
          child: Container(
            decoration: const BoxDecoration(
              shape: BoxShape.circle,
              gradient: LinearGradient(
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
                colors: [Color(0xFFFF7043), _kBrand],
              ),
            ),
            child: const Icon(Icons.qr_code_2, color: Colors.white, size: 30),
          ),
        ),
      ),
    );
  }
}

class _NavItem extends StatelessWidget {
  final IconData icon;
  final IconData activeIcon;
  final String label;
  final bool selected;
  final VoidCallback onTap;

  const _NavItem({
    required this.icon,
    required this.activeIcon,
    required this.label,
    required this.selected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final color = selected ? _kBrand : Theme.of(context).colorScheme.outline;
    return Expanded(
      child: InkResponse(
        onTap: onTap,
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(selected ? activeIcon : icon, size: 24, color: color),
            const SizedBox(height: 2),
            // `maxLines: 1` + kichik o'lcham — "Bosh sahifa" tor
            // ekranlarda ikki qatorga bo'linib, balandlikni buzardi.
            Text(
              label,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                fontSize: 11,
                fontWeight: selected ? FontWeight.w600 : FontWeight.w500,
                color: color,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

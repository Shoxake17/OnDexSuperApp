import 'package:flutter/material.dart';

import '../services/push.dart';
import 'favorites_screen.dart';
import 'mini_app_webview.dart';
import 'orders_screen.dart';
import 'profile_screen.dart';

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
  String? _webViewUrl;

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
    MiniAppWebView(
      onUrlChanged: (url) => setState(() => _webViewUrl = url),
    ),
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

  // Bosh sahifa (restoranlar ro'yxati) — pastki bar ko'rinadi. Menyu/
  // Savat/Checkout — bular Yandex Eats uslubidagi "chuqurlashgan" oqim
  // (o'z <- orqaga tugmasi bilan), pastki bar ULARDA YASHIRILADI (aks
  // holda foydalanuvchi tasodifan boshqa tab bosib, savatini yo'qotishi
  // mumkin edi).
  bool get _hideBottomBar {
    if (_index != 0) return false;
    final path = Uri.tryParse(_webViewUrl ?? '')?.path ?? '';
    return path.startsWith('/restaurants/') ||
        path == '/cart' ||
        path.startsWith('/cart/') ||
        path == '/address' ||
        path.startsWith('/address/') ||
        path == '/checkout' ||
        path.startsWith('/checkout/');
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
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
      body: SafeArea(
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
      bottomNavigationBar: _hideBottomBar
          ? null
          : NavigationBar(
              selectedIndex: _index,
              onDestinationSelected: _selectTab,
              destinations: const [
                NavigationDestination(
                  icon: Icon(Icons.storefront_outlined),
                  selectedIcon: Icon(Icons.storefront),
                  label: 'Bosh sahifa',
                ),
                NavigationDestination(
                  icon: Icon(Icons.favorite_border),
                  selectedIcon: Icon(Icons.favorite),
                  label: 'Istaklarim',
                ),
                NavigationDestination(
                  icon: Icon(Icons.receipt_long_outlined),
                  selectedIcon: Icon(Icons.receipt_long),
                  label: 'Buyurtmalarim',
                ),
                NavigationDestination(
                  icon: Icon(Icons.person_outline),
                  selectedIcon: Icon(Icons.person),
                  label: 'Profil',
                ),
              ],
            ),
    );
  }
}

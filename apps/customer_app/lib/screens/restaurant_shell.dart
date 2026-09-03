import 'package:flutter/material.dart';

import '../data/cart_store.dart';
import '../widgets/bottom_nav.dart';
import '../widgets/sheet_page.dart';
import 'cart_screen.dart';
import 'catalog_screen.dart';
import 'favorites_screen.dart';
import 'orders_screen.dart';
import 'table_qr_flow.dart';

/// RESTORAN bo'limining o'z qobig'i — o'z pastki menyusi bilan.
///
/// ┌─ NEGA ALOHIDA QOBIQ ──────────────────────────────────────────────┐
/// OnDex — super ilova. Uning bosh sahifasida bank, taksi, dorixona
/// va restoran yonma-yon turadi, shuning uchun SUPER menyuda faqat
/// hamma xizmatga tegishli narsa qoladi:
///
///     Bosh sahifa · [Shaddiy] · Profil
///
/// "Savat", "Sevimlilar", "Buyurtmalar" va stol QR kodi esa FAQAT
/// ovqatga tegishli — taksi chaqirayotgan odamga ular mazmunsiz.
/// Shuning uchun ular super menyudan olinib, aynan shu qobiqqa
/// ko'chirildi:
///
///     Bosh sahifa · Savat · [QR] · Sevimlilar · Buyurtmalar
///
/// Bu qobiq super sahifadan `sheetRoute` bilan PUSH qilinadi — ya'ni
/// u super menyuning USTIGA to'liq yopiladi va ikkita menyu bir vaqtda
/// hech qachon ko'rinmaydi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// ┌─ MATERIAL SHU YERDA BERILADI ─────────────────────────────────────┐
/// Ichkaridagi ekranlar (`CatalogScreen`, `FavoritesScreen`,
/// `OrdersScreen`) o'z `Scaffold`iga EGA EMAS — ular tab sifatida
/// yozilgan. Ularning `InkResponse` tugmalari Material ancestor
/// talab qiladi va uni AYNAN shu yerdagi `Scaffold` beradi. Busiz
/// ekran "No Material widget found" bilan qizarardi.
/// └───────────────────────────────────────────────────────────────────┘
class RestaurantShell extends StatefulWidget {
  const RestaurantShell({super.key});

  @override
  State<RestaurantShell> createState() => _RestaurantShellState();
}

class _RestaurantShellState extends State<RestaurantShell> {
  static const _favoritesTabIndex = 2;

  int _index = 0;

  /// Sevimlilar ekranini majburan yangilash uchun.
  ///
  /// Tab'lar `Visibility(maintainState: true)` bilan doim "tirik"
  /// turadi — `initState()` qayta chaqirilmaydi. Shuning uchun menyu
  /// sahifasida bosilgan yurak belgisi bu tab'da o'z-o'zidan
  /// ko'rinmasdi.
  final _favoritesKey = GlobalKey<FavoritesScreenState>();

  late final _tabs = <Widget>[
    const CatalogScreen(),
    // `embedded: true` — savat o'zining `SheetPage` qobig'ini
    // OLMAYDI. Aks holda bu qobiqning dumaloq burchagi va qora
    // tizim paneli chizig'i IKKI marta chizilardi.
    const CartScreen(embedded: true),
    FavoritesScreen(key: _favoritesKey),
    const OrdersScreen(),
  ];

  void _selectTab(int i) {
    setState(() => _index = i);
    if (i == _favoritesTabIndex) {
      _favoritesKey.currentState?.reload(silent: true);
    }
  }

  @override
  Widget build(BuildContext context) {
    return SheetPage(
      child: Scaffold(
        backgroundColor: Colors.white,
        body: Stack(
          children: [
            for (var i = 0; i < _tabs.length; i++)
              Visibility(
                visible: i == _index,
                maintainState: true,
                child: _tabs[i],
              ),
          ],
        ),
        // Markazdagi tugma panelning ICHIDA emas — u `centerDocked`
        // bilan uning yuqori chetida SUZADI va menyu balandligiga
        // ta'sir qilmaydi.
        floatingActionButton: QrFab(onTap: () => scanTableQr(context)),
        floatingActionButtonLocation: FloatingActionButtonLocation.centerDocked,
        bottomNavigationBar: AnimatedBuilder(
          // Savatdagi taomlar soni menyudan ham o'zgaradi — belgi
          // o'zi yangilanishi uchun savat oqimiga obuna bo'ladi.
          animation: CartStore.instance,
          builder: (_, __) => OndexBottomBar(
            left: [
              NavSpec(
                icon: Icons.storefront_outlined,
                activeIcon: Icons.storefront,
                label: 'Bosh sahifa',
                selected: _index == 0,
                onTap: () => _selectTab(0),
              ),
              NavSpec(
                icon: Icons.shopping_bag_outlined,
                activeIcon: Icons.shopping_bag,
                label: 'Savat',
                selected: _index == 1,
                badge: CartStore.instance.totalQty,
                onTap: () => _selectTab(1),
              ),
            ],
            right: [
              NavSpec(
                icon: Icons.favorite_border,
                activeIcon: Icons.favorite,
                label: 'Sevimlilar',
                selected: _index == _favoritesTabIndex,
                onTap: () => _selectTab(_favoritesTabIndex),
              ),
              NavSpec(
                icon: Icons.receipt_long_outlined,
                activeIcon: Icons.receipt_long,
                label: 'Buyurtmalar',
                selected: _index == 3,
                onTap: () => _selectTab(3),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

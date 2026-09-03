import 'package:flutter/material.dart';

import '../data/cart_store.dart';
import 'menu_screen.dart';
import 'qr_scan_screen.dart';

/// Stol QR kodini skanerlash oqimi — skanerdan menyugacha.
///
/// ┌─ NEGA ALOHIDA FUNKSIYA ───────────────────────────────────────────┐
/// Bu oqim IKKI joydan chaqiriladi:
///   * restoran qobig'ining markazdagi QR tugmasidan
///     (`restaurant_shell.dart`);
///   * super ilova bosh sahifasidagi qidiruv maydonidan
///     (`super_home_screen.dart` → `home_shell.dart`).
///
/// Ilgari kod `home_shell.dart` ichida edi. U yerda qolsa, restoran
/// qobig'i uni nusxalashga majbur bo'lardi — va ikki nusxadan biri
/// stol seansini savatga yozishni unutib qo'ysa, buyurtma jimgina
/// `delivery` bo'lib ketardi (`routes_orders.go` `table_token` ga
/// qarab hal qiladi). Shuning uchun mantiq BITTA joyda.
/// └───────────────────────────────────────────────────────────────────┘
///
/// ┌─ ISH TAQSIMOTI ───────────────────────────────────────────────────┐
/// `QrScanScreen`  — kamera, QR o'qish va tokenni SERVERDA yechish
///                   (`GET /tables/resolve`). Xato matnlari o'sha
///                   ekranda ko'rsatiladi.
/// Bu funksiya     — natijani savatga yozadi va menyuni ochadi.
/// └───────────────────────────────────────────────────────────────────┘
Future<void> scanTableQr(BuildContext context) async {
  final navigator = Navigator.of(context);
  final result = await navigator.push<TableScanResult>(
    MaterialPageRoute(builder: (_) => const QrScanScreen()),
  );
  if (result == null || !context.mounted) return; // foydalanuvchi yopdi

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

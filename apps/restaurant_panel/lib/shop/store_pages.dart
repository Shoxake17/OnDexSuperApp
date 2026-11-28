import 'package:flutter/material.dart';

import 'pages/add_product_page.dart';
import 'pages/dashboard_page.dart';
import 'pages/debts_page.dart';
import 'pages/incoming_page.dart';
import 'pages/logistics_page.dart';
import 'pages/pharm_add_page.dart';
import 'pages/pharm_base_page.dart';
import 'pages/pharm_info_page.dart';
import 'pages/products_page.dart';
import 'pages/qurt_orders_page.dart';
import 'pages/reports_page.dart';
import 'pages/sales_page.dart';
import 'pages/scanner_page.dart';
import 'pages/settings_page.dart';

/// Do'kon panelining sahifalari (ShoxPos marshrutlari bilan 1:1).
enum StorePage {
  // Oddiy do'kon / qurt
  dashboard, // /menu
  products, // /products
  addProduct, // /add-product (?edit=<shtrix-kod> — qurt tahriri)
  orders, // /orders (qurt)
  logistics, // /logistics (qurt)
  scanner, // /scanner
  sales, // /sales
  incoming, // /kirim-bazasi
  reports, // /reports
  debts, // /debts
  settings, // /settings
  // Dorixona
  pharmDashboard, // /pharm-menu
  pharmAdd, // /pharm-add
  pharmBase, // /pharm-base
  pharmScanner, // /pharm-scanner
  pharmSales, // /pharm-sale
  pharmReports, // /pharm-hisobotlar
  pharmIncoming, // /pharm-kirim-base
  pharmInfo, // /pharm-info
  pharmDebts, // /pharm-qarzlar
}

/// Sahifalarga beriladigan umumiy holat.
class StoreContext {
  const StoreContext(
      {required this.category, required this.name, required this.phone, required this.go, this.arg});

  /// `store` | `qurt` | `pharmacy`.
  final String category;
  final String name;
  final String phone;

  /// Boshqa sahifaga o'tish; [arg] — sahifa parametri (masalan tahrir uchun shtrix-kod).
  final void Function(StorePage page, {String? arg}) go;

  /// Joriy sahifa parametri (go(..., arg:) dan).
  final String? arg;

  bool get isQurt => category == 'qurt';
  bool get isPharmacy => category == 'pharmacy';
}

/// Sahifa vidjeti. `switch` TO'LIQ (`_` yo'q): yangi [StorePage] qo'shilsa,
/// uning sahifasi yozilmaguncha kompilyatsiya o'tmaydi — joy egasi qolmaydi.
Widget buildStorePage(StorePage p, StoreContext ctx) {
  return switch (p) {
    StorePage.products => ProductsPage(ctx: ctx),
    StorePage.addProduct => AddProductPage(ctx: ctx),
    StorePage.scanner || StorePage.pharmScanner => ScannerPage(ctx: ctx),
    StorePage.sales || StorePage.pharmSales => SalesPage(ctx: ctx),
    StorePage.debts || StorePage.pharmDebts => DebtsPage(ctx: ctx),
    StorePage.reports || StorePage.pharmReports => ReportsPage(ctx: ctx),
    StorePage.incoming || StorePage.pharmIncoming => IncomingPage(ctx: ctx),
    StorePage.pharmAdd => PharmAddPage(ctx: ctx),
    StorePage.pharmBase => PharmBasePage(ctx: ctx),
    StorePage.pharmInfo => PharmInfoPage(ctx: ctx),
    StorePage.dashboard || StorePage.pharmDashboard => DashboardPage(ctx: ctx),
    StorePage.settings => StoreSettingsPage(ctx: ctx),
    StorePage.orders => QurtOrdersPage(ctx: ctx),
    StorePage.logistics => LogisticsPage(ctx: ctx),
  };
}

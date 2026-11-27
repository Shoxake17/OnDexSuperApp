import 'package:flutter/material.dart';

import '../theme.dart';
import 'pages/add_product_page.dart';
import 'pages/debts_page.dart';
import 'pages/incoming_page.dart';
import 'pages/products_page.dart';
import 'pages/reports_page.dart';
import 'pages/sales_page.dart';
import 'pages/scanner_page.dart';

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

/// Sahifa vidjeti. Hali ko'chirilmagan sahifa — aniq belgilangan bo'sh joy
/// (POS-3 davomida bittalab almashtiriladi; deploy'dan oldin bittasi ham
/// qolmasligi `store_pages_test.dart` bilan tekshiriladi).
Widget buildStorePage(StorePage p, StoreContext ctx) {
  return switch (p) {
    StorePage.products => ProductsPage(ctx: ctx),
    StorePage.addProduct => AddProductPage(ctx: ctx),
    StorePage.scanner => ScannerPage(ctx: ctx),
    StorePage.sales => SalesPage(ctx: ctx),
    StorePage.debts || StorePage.pharmDebts => DebtsPage(ctx: ctx),
    StorePage.reports => ReportsPage(ctx: ctx),
    StorePage.incoming => IncomingPage(ctx: ctx),
    _ => PortingPlaceholder(page: p),
  };
}

class PortingPlaceholder extends StatelessWidget {
  const PortingPlaceholder({super.key, required this.page});
  final StorePage page;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(mainAxisSize: MainAxisSize.min, children: [
        const Icon(Icons.construction_rounded, size: 48, color: OnDexColors.inkFaint),
        const SizedBox(height: 12),
        Text('Bo\'lim ko\'chirilmoqda: ${page.name}', style: const TextStyle(color: OnDexColors.inkDim)),
      ]),
    );
  }
}

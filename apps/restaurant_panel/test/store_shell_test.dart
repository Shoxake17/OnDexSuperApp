import 'dart:convert';

import 'package:chust_restaurant/shop/store_shell.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

// Do'kon paneli qobig'i (POS-2).
//
// ┌─ NIMA QULFLANADI ──────────────────────────────────────────────────┐
//  1. Menyu ShoxPos bilan 1:1 kategoriya bo'yicha: qurtda kassa va
//     boshqaruv paneli YO'Q, Buyurtmalar/Logistika FAQAT qurtda,
//     dorixona menyusi alohida;
//  2. Xizmat almashtirish (Restoran | Do'kon) pastki profil kartasi menyusida
//     (admin paneldagi kabi); ulanmagan xizmat qulf; Chiqish shu menyuda;
//  3. Kategoriya serverdan (`/merchant/shop/me`) olinadi.
// └────────────────────────────────────────────────────────────────────┘

http.Response _json(Object body, [int status = 200]) => http.Response.bytes(
      utf8.encode(jsonEncode(body)),
      status,
      headers: {'content-type': 'application/json; charset=utf-8'},
    );

Future<void> _open(WidgetTester tester, String category, Future<void> Function() body) async {
  tester.view.physicalSize = const Size(1440, 1000);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  await http.runWithClient(
    () async {
      await tester.pumpWidget(const MaterialApp(home: StoreShell(name: 'Baraka market', phone: '+998901112233')));
      await tester.pumpAndSettle();
      await body();
      await tester.pumpWidget(const SizedBox());
    },
    () => MockClient((r) async {
      if (r.url.path == '/merchant/shop/me') {
        return _json({'store_id': 's1', 'category': category, 'shop_type': 'x', 'sections': []});
      }
      if (r.url.path.endsWith('base-count')) return _json({'count': 0});
      return _json([]);
    }),
  );
}

bool _has(String label) => find.text(label).evaluate().isNotEmpty;

void main() {
  testWidgets('oddiy do\'kon: kassa bor, buyurtma/logistika yo\'q', (tester) async {
    await _open(tester, 'store', () async {
      expect(find.byKey(const ValueKey('store-nav-scanner')), findsOneWidget);
      expect(find.byKey(const ValueKey('store-nav-dashboard')), findsOneWidget);
      expect(find.byKey(const ValueKey('store-nav-orders')), findsNothing);
      expect(find.byKey(const ValueKey('store-nav-logistics')), findsNothing);
      expect(find.byKey(const ValueKey('store-nav-pharmBase')), findsNothing);
      expect(_has('Do\'kon paneli'), isTrue);
    });
  });

  testWidgets('qurt: kassa va boshqaruv paneli yo\'q, buyurtmalar bor', (tester) async {
    await _open(tester, 'qurt', () async {
      expect(find.byKey(const ValueKey('store-nav-scanner')), findsNothing);
      expect(find.byKey(const ValueKey('store-nav-dashboard')), findsNothing);
      expect(find.byKey(const ValueKey('store-nav-orders')), findsOneWidget);
      expect(find.byKey(const ValueKey('store-nav-logistics')), findsOneWidget);
      expect(_has('Qurt paneli'), isTrue);
    });
  });

  testWidgets('dorixona: alohida menyu', (tester) async {
    await _open(tester, 'pharmacy', () async {
      for (final k in ['pharmDashboard', 'pharmAdd', 'pharmBase', 'pharmScanner', 'pharmInfo', 'pharmDebts']) {
        expect(find.byKey(ValueKey('store-nav-$k')), findsOneWidget, reason: k);
      }
      expect(find.byKey(const ValueKey('store-nav-scanner')), findsNothing);
    });
  });

  testWidgets('xizmat almashtirish pastki kartada: Do\'kon faol, Restoran qulf, Chiqish', (tester) async {
    await _open(tester, 'store', () async {
      // Tepada blok yo'q — faqat pastki karta menyusida.
      expect(find.byKey(const ValueKey('service-restaurant')), findsNothing);
      await tester.tap(find.byKey(const ValueKey('account-menu')));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('service-restaurant')), findsOneWidget);
      expect(find.byKey(const ValueKey('service-store')), findsOneWidget);
      expect(find.byKey(const ValueKey('menu-logout')), findsOneWidget);
      expect(find.byIcon(Icons.lock_outline_rounded), findsOneWidget);
      expect(find.text('Ulanmagan'), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('service-restaurant')));
      await tester.pumpAndSettle();
      expect(find.textContaining('restoran ulanmagan'), findsWidgets);
    });
  });
  testWidgets('bo\'limlar orasida o\'tish', (tester) async {
    await _open(tester, 'store', () async {
      await tester.tap(find.byKey(const ValueKey('store-nav-debts')));
      await tester.pumpAndSettle();
      // Yuqori panel sarlavhasi + sidebar bandi.
      expect(find.text('Qarzlar'), findsNWidgets(2));
    });
  });
}

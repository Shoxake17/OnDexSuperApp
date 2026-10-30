import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:ondex_admin/pages/banners_page.dart';

// Admin panel > Bannerlar: uch bo'lim (Menu / Restoran / Do'kon), har biri o'z
// joylashuvidagi bannerlarni ko'rsatadi va shu joylashuv bilan yaratadi.

http.Response _json(Object body, [int status = 200]) => http.Response(
    jsonEncode(body), status,
    headers: {'content-type': 'application/json; charset=utf-8'});

Map<String, dynamic> _b(String id, String placement, {String title = '', String? action, String? rest}) => {
      'id': id,
      'image_url': '',
      'placement': placement,
      'title': title,
      'action': action ?? '',
      'restaurant_id': rest ?? '',
      'active': true,
      'sort_order': 0,
    };

void main() {
  testWidgets('uchta bo\'lim: har biri faqat o\'z bannerlarini ko\'rsatadi', (tester) async {
    tester.view.physicalSize = const Size(1400, 1000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    await http.runWithClient(() async {
      await tester.pumpWidget(const MaterialApp(home: Scaffold(body: BannersPage())));
      await tester.pumpAndSettle();

      // Tab nomlari (soni bilan)
      expect(find.textContaining('Menu Banner (2)'), findsOneWidget);
      expect(find.textContaining('Restoran Banner (1)'), findsOneWidget);
      expect(find.textContaining('Do\'kon Banner (1)'), findsOneWidget);

      // 1-bo'lim: Menu
      expect(find.text('Qurt banneri'), findsOneWidget);
      expect(find.text('Ikkinchi menu'), findsOneWidget);
      expect(find.text('Do\'kon tepasi'), findsNothing);
      expect(find.textContaining('Qurt sahifasi'), findsWidgets);

      await tester.tap(find.byKey(const ValueKey('banner-tab-restaurant')));
      await tester.pumpAndSettle();
      expect(find.text('Chust Osh'), findsOneWidget); // restoran nomi
      expect(find.text('Qurt banneri'), findsNothing);

      await tester.tap(find.byKey(const ValueKey('banner-tab-store')));
      await tester.pumpAndSettle();
      expect(find.text('Do\'kon tepasi'), findsOneWidget);
      expect(find.text('Chust Osh'), findsNothing);
    }, () => MockClient((req) async {
          switch (req.url.path) {
            case '/admin/banners':
              return _json([
                _b('1', 'menu', title: 'Qurt banneri', action: 'qurt'),
                _b('2', 'menu', title: 'Ikkinchi menu'),
                _b('3', 'restaurant', rest: 'r1'),
                _b('4', 'store', title: 'Do\'kon tepasi'),
              ]);
            case '/restaurants':
              return _json([
                {'id': 'r1', 'name': 'Chust Osh'}
              ]);
          }
          return _json({});
        }));
  });

  testWidgets('eski bannerlar (joylashuvi bo\'sh) — Restoran bo\'limida qoladi', (tester) async {
    tester.view.physicalSize = const Size(1400, 1000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    await http.runWithClient(() async {
      await tester.pumpWidget(const MaterialApp(home: Scaffold(body: BannersPage())));
      await tester.pumpAndSettle();
      expect(find.textContaining('Restoran Banner (1)'), findsOneWidget);
      expect(find.textContaining('Menu Banner (0)'), findsOneWidget);
    }, () => MockClient((req) async {
          if (req.url.path == '/admin/banners') {
            final legacy = _b('1', '');
            legacy.remove('placement'); // eski server maydonni umuman bermagan
            return _json([legacy]);
          }
          return _json(<Object>[]);
        }));
  });

  testWidgets('yangi banner tanlangan bo\'lim joylashuvi bilan yuboriladi', (tester) async {
    tester.view.physicalSize = const Size(1400, 1000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    // Rasm tanlash fayl oynasini ochadi — bu yerda faqat oyna maydonlari tekshiriladi.
    await http.runWithClient(() async {
      await tester.pumpWidget(const MaterialApp(home: Scaffold(body: BannersPage())));
      await tester.pumpAndSettle();

      // Menu bo'limi: sarlavha, tavsif, tugma va "Bosilganda" bor
      await tester.tap(find.byKey(const ValueKey('banner-add')));
      await tester.pumpAndSettle();
      expect(find.text('Yangi Menu Banner'), findsOneWidget);
      expect(find.textContaining('Sarlavha'), findsOneWidget);
      expect(find.textContaining('Tugma matni'), findsOneWidget);
      expect(find.text('Bosilganda'), findsOneWidget);
      // Rasm yo'q — saqlab bo'lmaydi
      await tester.tap(find.text('Saqlash'));
      await tester.pumpAndSettle();
      expect(find.text('Banner rasmi yuklanmagan'), findsOneWidget);
      await tester.tap(find.text('Bekor qilish'));
      await tester.pumpAndSettle();

      // Do'kon bo'limi: sarlavha/tavsif bor, tugma va amal YO'Q
      await tester.tap(find.byKey(const ValueKey('banner-tab-store')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('banner-add')));
      await tester.pumpAndSettle();
      expect(find.text('Yangi Do\'kon Banner'), findsOneWidget);
      expect(find.textContaining('Sarlavha'), findsOneWidget);
      expect(find.textContaining('Tugma matni'), findsNothing);
      expect(find.text('Bosilganda'), findsNothing);
      await tester.tap(find.text('Bekor qilish'));
      await tester.pumpAndSettle();

      // Restoran bo'limi: faqat restoran tanlovi (matn qatlami yo'q)
      await tester.tap(find.byKey(const ValueKey('banner-tab-restaurant')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('banner-add')));
      await tester.pumpAndSettle();
      expect(find.textContaining('Bosilganda ochiladigan restoran'), findsOneWidget);
      expect(find.textContaining('Sarlavha'), findsNothing);
    }, () => MockClient((req) async {
          if (req.url.path == '/admin/banners') return _json(<Object>[]);
          return _json(<Object>[]);
        }));
  });
}

import 'package:chust_restaurant/api.dart';
import 'package:chust_restaurant/merchant_route.dart';
import 'package:chust_restaurant/theme.dart';
import 'package:chust_restaurant/widgets/charts.dart';
import 'package:chust_restaurant/widgets/service_switcher.dart';
import 'package:chust_restaurant/widgets/sidebar.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

// ┌─ QO'RIQCHI TESTLAR: RESTORAN TOMONI (ONDEXECOSY-498) ─────────────────┐
// Do'kon (POS) kodi `lib/shop/` da alohida, lekin restoran bilan 6 ta
// umumiy faylni ishlatadi: api.dart, printing/*, theme.dart,
// widgets/charts.dart, login_screen/main (kirish yo'nalishi),
// widgets/service_switcher.dart. Do'kon uchun shu fayllardan biri
// o'zgartirilsa va RESTORAN xatti-harakati buzilsa — shu testlar qizil.
//
// Chek printeri: `receipt_test.dart` (oshxona/mijoz cheki, 58/80 mm PDF,
// avto-chop) — o'sha fayl ham qo'riqchi.
// └──────────────────────────────────────────────────────────────────────┘

Future<void> _pumpSidebar(WidgetTester tester,
    {required VoidCallback onLogout, int selected = 0}) async {
  tester.view.physicalSize = const Size(1200, 1000);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(MaterialApp(
    home: Scaffold(
      body: Row(children: [
        Sidebar(
          selectedIndex: selected,
          onSelect: (_) {},
          restaurantName: 'Chust Osh',
          restaurantAddress: 'Chust, Navoiy 5',
          restaurantLogoUrl: '',
          newOrdersCount: 3,
          notificationsCount: 2,
          supportCount: 1,
          onLogout: onLogout,
        ),
        const Expanded(child: SizedBox()),
      ]),
    ),
  ));
  await tester.pumpAndSettle();
}

void main() {
  group('kirish yo\'nalishi (main.dart + login_screen.dart)', () {
    test('restoran egasi -> restoran paneli', () {
      expect(merchantHomeFor({'role': 'restaurant', 'entity_id': 'r1'}),
          MerchantHome.restaurant);
    });
    test('restoran, lekin entity_id yo\'q -> kiritilmaydi', () {
      expect(merchantHomeFor({'role': 'restaurant'}), MerchantHome.none);
      expect(merchantHomeFor({'role': 'restaurant', 'entity_id': ''}),
          MerchantHome.none);
    });
    test('do\'kon -> do\'kon paneli', () {
      expect(merchantHomeFor({'role': 'store', 'entity_id': 's1'}),
          MerchantHome.store);
    });
    test('boshqa rollar -> kiritilmaydi', () {
      for (final r in ['', 'customer', 'courier', 'waiter', 'admin']) {
        expect(
            merchantHomeFor({'role': r, 'entity_id': 'x'}), MerchantHome.none,
            reason: r);
      }
    });
  });

  group('restoran sidebar', () {
    testWidgets('13 ta bo\'lim aynan shu tartibda', (tester) async {
      await _pumpSidebar(tester, onLogout: () {});
      const labels = [
        'Bosh sahifa',
        'Buyurtmalar',
        'Menyu',
        'Aksiyalar',
        'Stollar (QR)',
        'Bronlar',
        'Statistika',
        'Moliya',
        'Restoran sozlamalari',
        'Xodimlar',
        'Bildirishnomalar',
        'Yordam markazi',
        'Chat markazi',
      ];
      double lastY = -1;
      for (final l in labels) {
        expect(find.text(l), findsOneWidget, reason: l);
        final y = tester.getTopLeft(find.text(l)).dy;
        expect(y, greaterThan(lastY), reason: 'tartib: $l');
        lastY = y;
      }
      expect(find.text('Restoran paneli'), findsOneWidget);
      expect(find.text('Chust Osh'), findsOneWidget);
    });

    testWidgets('rozetkalar: buyurtmalar, bildirishnomalar, chat',
        (tester) async {
      await _pumpSidebar(tester, onLogout: () {});
      expect(find.text('3'), findsOneWidget);
      expect(find.text('2'), findsOneWidget);
      expect(find.text('1'), findsOneWidget);
    });

    testWidgets(
        'profil kartasi menyusi: Restoran faol, Do\'kon qulf, Chiqish ishlaydi',
        (tester) async {
      var loggedOut = 0;
      await _pumpSidebar(tester, onLogout: () => loggedOut++);
      await tester.tap(find.byKey(const ValueKey('account-menu')));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('service-restaurant')), findsOneWidget);
      expect(find.byKey(const ValueKey('service-store')), findsOneWidget);
      expect(find.byIcon(Icons.check_rounded),
          findsOneWidget); // faqat Restoran faol
      expect(find.byIcon(Icons.lock_outline_rounded),
          findsOneWidget); // Do'kon ulanmagan

      // Ulanmagan Do'kon: o'tmaydi, xabar chiqadi, chiqish bo'lmaydi.
      await tester.tap(find.byKey(const ValueKey('service-store')));
      await tester.pumpAndSettle();
      expect(find.textContaining('do\'kon ulanmagan'), findsOneWidget);
      expect(loggedOut, 0);

      // Xabar pastki kartani yopib turadi — yopib, menyuni qayta ochamiz.
      ScaffoldMessenger.of(tester.element(find.byType(Sidebar)))
          .removeCurrentSnackBar();
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('account-menu')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('menu-logout')));
      await tester.pumpAndSettle();
      expect(loggedOut, 1);
    });
  });

  group('xizmat menyusi (service_switcher.dart)', () {
    test('qiymatlar: ikki xizmat + chiqish', () {
      final items = merchantMenuItems(active: MerchantService.restaurant)
          .whereType<PopupMenuItem<String>>();
      expect(items.map((i) => i.value),
          ['service:restaurant', 'service:store', 'logout']);
    });

    testWidgets(
        'ikkala xizmat ulangan bo\'lsa almashtiradi, faolini qayta tanlash hech narsa qilmaydi',
        (tester) async {
      late BuildContext ctx;
      await tester
          .pumpWidget(MaterialApp(home: Scaffold(body: Builder(builder: (c) {
        ctx = c;
        return const SizedBox();
      }))));
      final switched = <MerchantService>[];
      var out = 0;
      void run(String v) => handleMerchantMenu(ctx, v,
          active: MerchantService.restaurant,
          available: const {MerchantService.store},
          onLogout: () => out++,
          onSwitch: switched.add);
      run('service:restaurant');
      run('service:store');
      run('logout');
      expect(switched, [MerchantService.store]);
      expect(out, 1);
    });
  });

  group('umumiy yadro', () {
    test(
        'rasm manzili: to\'liq URL o\'zgarmaydi, nisbiy yo\'lga server qo\'shiladi',
        () {
      expect(fullImageUrl('https://cdn.ondex.uz/logos/a.webp'),
          'https://cdn.ondex.uz/logos/a.webp');
      expect(fullImageUrl('/uploads/a.jpg'), '$apiBaseUrl/uploads/a.jpg');
      expect(fullImageUrl(''), '');
      expect(fullImageUrl(null), '');
    });

    test('pul formati', () {
      expect(formatSum(4500000 ~/ 100), isNotEmpty);
    });

    test('restoran brend ranglari (OnDex Merchant, binafsha emas)', () {
      expect(OnDexColors.primary, const Color(0xFFF2650F));
      expect(OnDexColors.sidebarBg, const Color(0xFF1B140F));
      expect(OnDexColors.sidebarBgActive, const Color(0xFF2A2018));
      expect(OnDexColors.pageBg, const Color(0xFFF7F1E7));
      expect(OnDexColors.danger, const Color(0xFFE5484D));
    });
  });

  group('statistika grafigi (charts.dart)', () {
    Future<void> pump(WidgetTester t, List<ChartPoint> pts) =>
        t.pumpWidget(MaterialApp(
          home: Scaffold(
            body: SizedBox(
              width: 600,
              height: 300,
              child: OnDexLineChart(
                points: pts,
                color: OnDexColors.primary,
                axisLabel: (v) => v.toStringAsFixed(0),
                emptyText: 'Ma\'lumot yo\'q',
                integerAxis: true,
              ),
            ),
          ),
        ));

    testWidgets('hammasi nol -> bo\'sh matn', (t) async {
      await pump(t, [
        for (var i = 0; i < 7; i++)
          ChartPoint(
              axisLabel: 'D$i',
              value: 0,
              tooltipTitle: 'D$i',
              tooltipValue: '0')
      ]);
      expect(find.text('Ma\'lumot yo\'q'), findsOneWidget);
    });

    testWidgets('qiymat bor -> chiziladi, xato yo\'q', (t) async {
      await pump(t, [
        for (var i = 0; i < 7; i++)
          ChartPoint(
              axisLabel: 'D$i',
              value: i * 3.0,
              tooltipTitle: 'D$i',
              tooltipValue: '$i')
      ]);
      expect(find.text('Ma\'lumot yo\'q'), findsNothing);
      expect(find.byType(CustomPaint), findsWidgets);
      expect(t.takeException(), isNull);
    });
  });
}

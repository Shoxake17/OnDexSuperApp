import 'dart:convert';

import 'package:chust_admin/pages/restaurant_module.dart';
import 'package:chust_admin/screens/shell.dart';
import 'package:chust_admin/support_inbox.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

// "Restoran" bo'limi: cover rasmli kartalar ro'yxati -> tanlangan restoranning
// ALOHIDA moduli (Buyurtmalar, Kuryerlar, Affitsiantlar, Kutubxona, Chat).
// Har bo'lim FAQAT shu restoran ma'lumotini ko'rsatadi.

http.Response _json(Object body, [int status = 200]) => http.Response.bytes(
      utf8.encode(jsonEncode(body)),
      status,
      headers: {'content-type': 'application/json; charset=utf-8'},
    );

Map<String, dynamic> _restaurant(String id, String name, {bool open = true}) => {
      'id': id,
      'name': name,
      'address': '$name manzili',
      'open': open,
      'open_now': open,
      'logo_url': '',
      'cover_url': 'covers/$id.jpg',
    };

Map<String, dynamic> _order(String id, int number, String rid) => {
      'id': id,
      'order_number': number,
      'created_at': DateTime(2026, 9, 21, 12).toUtc().toIso8601String(),
      'status': 'created',
      'total_tiyin': 5000000,
      'restaurant_id': rid,
      'restaurant_name': rid == 'rest-a' ? 'Book Cafe' : 'Osh Markazi',
      'customer_name': 'Mijoz $number',
      'customer_phone': '+99890000$number',
    };

Map<String, dynamic> _thread(String rid, {int unread = 0}) => {
      'restaurant_id': rid,
      'message_count': unread == 0 ? 0 : 2,
      'last_seq': unread,
      'last_sender': 'restaurant',
      'last_body': '',
      'read_seq': 0,
      'peer_read_seq': 0,
      'unread': unread,
    };

/// Har so'rov shu ro'yxatga yoziladi: qaysi bo'lim qaysi so'rovni yuborgani
/// tekshiriladi (filtr serverga borishi shart).
Future<void> _run(WidgetTester tester, Widget home,
    Future<void> Function(List<http.Request> requests) body,
    {Size size = const Size(1600, 1000), bool oldServer = false}) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  final requests = <http.Request>[];
  await http.runWithClient(
    () async {
      await tester.pumpWidget(MaterialApp(
        theme: ThemeData(
            colorScheme: ColorScheme.fromSeed(seedColor: const Color(0xFF0B5D1E)),
            useMaterial3: true),
        home: Scaffold(body: home),
      ));
      await tester.pumpAndSettle();
      await body(requests);
      // Sahifalar dispose bo'lib, jonli yangilash taymerlari to'xtaydi.
      await tester.pumpWidget(const SizedBox());
    },
    () => MockClient((request) async {
      requests.add(request);
      final path = request.url.path;
      final q = request.url.queryParameters;
      if (request.method != 'GET') {
        return _json({'error': 'kutilmagan so\'rov: ${request.method} $path'}, 404);
      }
      switch (path) {
        case '/restaurants':
          return _json([
            _restaurant('rest-a', 'Book Cafe'),
            _restaurant('rest-b', 'Osh Markazi', open: false),
          ]);
        case '/admin/accounts':
          return _json([
            {'entity_id': 'rest-a', 'user_id': 'u-a', 'name': 'Aziz', 'phone': '+998901111111'},
          ]);
        case '/admin/support/threads':
          return _json({
            'items': [
              {
                ..._thread('rest-a', unread: 2),
                'restaurant': {'id': 'rest-a', 'name': 'Book Cafe', 'address': 'Book Cafe manzili', 'logo_url': ''},
                'restaurant_online': true,
              },
            ],
            'unread_total': 2,
          });
        case '/admin/stats':
          return _json({'orders_today': 7, 'revenue_today_tiyin': 12300000, 'delivered_today': 5, 'by_status': {}});
        case '/admin/customers':
          return _json({
            'count': 1,
            'items': [
              {'id': 'cu1', 'name': 'Sardor Mijoz', 'phone': '+998977777777', 'phone_verified': true, 'devices': []},
            ],
          });
        case '/support/contacts':
          return _json({'phone': '+998901234567', 'phone_hours': '09:00', 'telegram': 'ondex_support', 'email': 'support@ondex.uz', 'email_note': '', 'configured': true});
        case '/admin/support/summary':
          return _json({'unread': 2});
        case '/admin/orders':
          final all = [
            _order('o1', 101, 'rest-a'),
            _order('o2', 102, 'rest-a'),
            _order('o3', 201, 'rest-b'),
          ];
          final rid = q['restaurant_id'];
          // oldServer: filtr parametrini e'tiborsiz qoldiradigan eski backend.
          return _json(rid == null || oldServer ? all : all.where((o) => o['restaurant_id'] == rid).toList());
        case '/admin/couriers':
          // Server hammasini qaytaradi; ajratish panelda. Bittasi OnDex
          // umumiy kuryeri (restaurant_id yo'q) — restoran modulida chiqmasligi kerak.
          return _json([
            {'id': 'c1', 'name': 'Anvar Kuryer', 'phone': '+998911111111', 'approved': true, 'available': true, 'restaurant_id': 'rest-a'},
            {'id': 'c2', 'name': 'Boshqa Kuryer', 'phone': '+998922222222', 'approved': true, 'available': false, 'restaurant_id': 'rest-b'},
            {'id': 'c3', 'name': 'Umumiy Kuryer', 'phone': '+998933333333', 'approved': true, 'available': false, 'restaurant_id': ''},
          ]);
        case '/admin/waiters':
          return _json({
            'count': 2,
            'items': [
              {'id': 'w1', 'name': 'Olim Ofitsiant', 'phone': '+998944444444', 'restaurant_id': 'rest-a', 'restaurant_name': 'Book Cafe', 'devices': []},
              {'id': 'w2', 'name': 'Begona Ofitsiant', 'phone': '+998955555555', 'restaurant_id': 'rest-b', 'restaurant_name': 'Osh Markazi', 'devices': []},
            ],
          });
        case '/restaurants/rest-a/books/all':
          return _json([
            {'id': 'b1', 'title': 'Kitob A', 'author': 'Muallif', 'active': true, 'cover_url': ''},
          ]);
        case '/restaurants/rest-b/books/all':
          return _json([
            {'id': 'b2', 'title': 'Kitob B', 'author': 'Muallif', 'active': true, 'cover_url': ''},
          ]);
        case '/admin/support/threads/rest-a/messages':
          return _json({
            'items': [
              {
                'id': 'm1',
                'seq': 1,
                'sender': 'restaurant',
                'sender_name': 'Book Cafe',
                'body': 'Salom admin',
                'client_id': 'client-1-abcdefgh',
                'created_at': DateTime.now().toUtc().toIso8601String(),
              },
            ],
            'next_before': 0,
            'thread': {..._thread('rest-a', unread: 2), 'last_seq': 1},
            'restaurant': {'id': 'rest-a', 'name': 'Book Cafe'},
            'restaurant_online': true,
          });
      }
      return _json({'error': 'kutilmagan so\'rov: ${request.method} $path'}, 404);
    }),
  );
}

Future<void> _openRestaurant(WidgetTester tester, String id) async {
  await tester.tap(find.byKey(ValueKey('restaurant-open-$id')));
  await tester.pumpAndSettle();
}

Future<void> _tab(WidgetTester tester, String screen) async {
  await tester.tap(find.byKey(ValueKey('restaurant-tab-$screen')));
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('ro\'yxat: cover kartalar, holat va o\'qilmagan rozetkasi', (tester) async {
    await _run(tester, const RestaurantsSection(), (requests) async {
      expect(tester.takeException(), isNull);
      expect(find.byKey(const ValueKey('restaurant-card-rest-a')), findsOneWidget);
      expect(find.byKey(const ValueKey('restaurant-card-rest-b')), findsOneWidget);
      expect(find.text('Book Cafe'), findsOneWidget);
      expect(find.text('Osh Markazi'), findsOneWidget);
      // Holat: birinchisi ochiq, ikkinchisi yopiq.
      expect(find.text('Ochiq'), findsOneWidget);
      expect(find.text('Yopiq'), findsOneWidget);
      // Faqat o'qilmagan xabari bor restoranda rozetka.
      expect(find.byKey(const ValueKey('restaurant-unread-rest-a')), findsOneWidget);
      expect(find.byKey(const ValueKey('restaurant-unread-rest-b')), findsNothing);
      // Akkaunt telefoni kartada.
      expect(find.text('+998901111111'), findsOneWidget);
      // Ro'yxat bosqichida modulning tablari yo'q.
      expect(find.byKey(const ValueKey('restaurant-module-tabs')), findsNothing);

      // Qidiruv.
      await tester.enterText(find.byKey(const ValueKey('restaurants-search')), 'osh');
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('restaurant-card-rest-a')), findsNothing);
      expect(find.byKey(const ValueKey('restaurant-card-rest-b')), findsOneWidget);
      expect(tester.takeException(), isNull);
    });
  });

  testWidgets('restoranni bosish -> alohida modul; har bo\'lim faqat o\'sha restoranniki', (tester) async {
    await _run(tester, const RestaurantsSection(), (requests) async {
      await _openRestaurant(tester, 'rest-a');
      expect(tester.takeException(), isNull);
      expect(find.byKey(const ValueKey('restaurant-module-title')), findsOneWidget);
      expect(
          tester.widget<Text>(find.byKey(const ValueKey('restaurant-module-title'))).data, 'Book Cafe');
      // Modulda ro'yxat kartalari yo'q.
      expect(find.byKey(const ValueKey('restaurant-card-rest-b')), findsNothing);

      // 1) Buyurtmalar (birinchi tab): filtr SERVERGA ketadi, "Restoran" ustuni yo'q.
      final ordersReq = requests.lastWhere((r) => r.url.path == '/admin/orders');
      expect(ordersReq.url.queryParameters['restaurant_id'], 'rest-a');
      expect(find.text('101'), findsOneWidget);
      expect(find.text('102'), findsOneWidget);
      expect(find.text('201'), findsNothing);
      expect(find.text('Restoran'), findsNothing);

      // 2) Kuryerlar: faqat shu restoranning o'z kuryeri.
      await _tab(tester, 'Couriers');
      expect(find.text('Anvar Kuryer'), findsOneWidget);
      expect(find.text('Boshqa Kuryer'), findsNothing);
      expect(find.text('Umumiy Kuryer'), findsNothing);

      // 3) Affitsiantlar: faqat shu restoranniki, "Restoran" ustuni yo'q.
      await _tab(tester, 'Waiters');
      expect(find.text('Olim Ofitsiant'), findsOneWidget);
      expect(find.text('Begona Ofitsiant'), findsNothing);
      expect(find.text('Restoran'), findsNothing);

      // 4) Kutubxona: shu restoranning kitoblari, restoran tanlash ro'yxati yo'q.
      await _tab(tester, 'Books');
      expect(find.text('Kitob A'), findsOneWidget);
      expect(find.text('Kitob B'), findsNothing);
      expect(find.byType(DropdownButton<String>), findsNothing);
      expect(requests.any((r) => r.url.path == '/restaurants/rest-a/books/all'), isTrue);
      // Restoranlar ro'yxati kutubxona uchun QAYTA so'ralmaydi.
      expect(requests.where((r) => r.url.path == '/restaurants/rest-b/books/all'), isEmpty);

      // 5) Chat: faqat shu restoran bilan yozishma, suhbatlar ro'yxati va "Yangi suhbat" yo'q.
      await _tab(tester, 'Chat');
      expect(find.text('Salom admin'), findsOneWidget);
      expect(find.text('Yangi suhbat'), findsNothing);
      expect(find.byKey(const ValueKey('admin-support-search')), findsNothing);
      expect(requests.any((r) => r.url.path == '/admin/support/threads/rest-a/messages'), isTrue);
      expect(requests.any((r) => r.url.path.contains('/threads/rest-b/')), isFalse);
      expect(tester.takeException(), isNull);
    });
  });

  testWidgets('bo\'limlar birinchi ochilganda yuklanadi va saqlanadi', (tester) async {
    await _run(tester, const RestaurantsSection(), (requests) async {
      await _openRestaurant(tester, 'rest-a');
      int count(String path) => requests.where((r) => r.url.path == path).length;
      // Faqat ochilgan bo'limning so'rovi ketgan (Buyurtmalar kuryer
      // NOMLARI uchun /admin/couriers ni o'zi ham so'raydi — shuning
      // uchun kuryerlar bo'yicha o'sishni o'lchaymiz).
      expect(count('/admin/waiters'), 0);
      expect(count('/restaurants/rest-a/books/all'), 0);
      expect(count('/admin/support/threads/rest-a/messages'), 0);
      final couriersBefore = count('/admin/couriers');
      await _tab(tester, 'Couriers');
      expect(count('/admin/couriers'), couriersBefore + 1);
      await _tab(tester, 'Orders');
      await _tab(tester, 'Couriers');
      // Qaytib kelganda qayta yuklanmaydi (holat saqlangan).
      expect(count('/admin/couriers'), couriersBefore + 1);
    });
  });

  testWidgets('boshqa restoran — alohida modul, ma\'lumot aralashmaydi; orqaga qaytish', (tester) async {
    await _run(tester, const RestaurantsSection(), (requests) async {
      await _openRestaurant(tester, 'rest-b');
      expect(
          tester.widget<Text>(find.byKey(const ValueKey('restaurant-module-title'))).data, 'Osh Markazi');
      final ordersReq = requests.lastWhere((r) => r.url.path == '/admin/orders');
      expect(ordersReq.url.queryParameters['restaurant_id'], 'rest-b');
      expect(find.text('201'), findsOneWidget);
      expect(find.text('101'), findsNothing);

      await _tab(tester, 'Books');
      expect(find.text('Kitob B'), findsOneWidget);
      expect(find.text('Kitob A'), findsNothing);

      // Orqaga: ro'yxat qaytadi.
      await tester.tap(find.byKey(const ValueKey('restaurant-module-back')));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('restaurant-card-rest-a')), findsOneWidget);
      expect(find.byKey(const ValueKey('restaurant-module-tabs')), findsNothing);

      // Boshqa restoranni ochsak — yangidan boshlanadi.
      await _openRestaurant(tester, 'rest-a');
      expect(find.text('101'), findsOneWidget);
      expect(find.text('201'), findsNothing);
      expect(tester.takeException(), isNull);
    });
  });

  testWidgets('Chat tabida shu restoranning o\'qilmagan soni rozetkada', (tester) async {
    await _run(tester, const RestaurantsSection(), (requests) async {
      await _openRestaurant(tester, 'rest-a');
      final chatTab = find.byKey(const ValueKey('restaurant-tab-Chat'));
      expect(find.descendant(of: chatTab, matching: find.text('2')), findsOneWidget);
    });
  });

  testWidgets('Sidebar: Restoran, Statistika, Mijozlar, OnDex kuryerlari, Sozlamalar; pastda OnDexMap', (tester) async {
    await _run(tester, const AdminShell(), (requests) async {
      expect(tester.takeException(), isNull);
      int count(String path) => requests.where((r) => r.url.path == path).length;
      Finder nav(String name) => find.byKey(ValueKey('nav-$name'));

      // Sidebar'da AYNAN oltita band; boshqa (yuqori tablar kabi) navigatsiya yo'q.
      final navItems = find.byWidgetPredicate((w) =>
          w.key is ValueKey<String> && (w.key as ValueKey<String>).value.startsWith('nav-'));
      expect(navItems, findsNWidgets(6));
      for (final label in const [
        'Restoran', 'Statistika', 'Mijozlar', 'OnDex kuryerlari', 'Sozlamalar', 'OnDexMap'
      ]) {
        expect(find.text(label), findsOneWidget, reason: '"$label" sidebar\'da bo\'lishi kerak');
      }

      // Tartib: yuqoridan pastga; OnDexMap eng pastda.
      final order = ['restaurants', 'stats', 'customers', 'couriers', 'settings', 'ondexmap'];
      for (var i = 1; i < order.length; i++) {
        expect(tester.getTopLeft(nav(order[i])).dy, greaterThan(tester.getTopLeft(nav(order[i - 1])).dy),
            reason: '${order[i]} ${order[i - 1]} dan pastda bo\'lishi kerak');
      }

      // Boshlang'ich bo'lim — restoranlar ro'yxati; o'qilmagan chat "Restoran" bandida.
      expect(find.byKey(const ValueKey('restaurant-card-rest-a')), findsOneWidget);
      expect(find.descendant(of: nav('restaurants'), matching: find.text('2')), findsOneWidget);
      // Ochilmagan bo'limlar so'rov yubormaydi.
      expect(count('/admin/stats'), 0);
      expect(count('/admin/customers'), 0);
      expect(count('/support/contacts'), 0);

      // Restoran ochiladi.
      await _openRestaurant(tester, 'rest-a');
      expect(find.byKey(const ValueKey('restaurant-module-tabs')), findsOneWidget);

      // Statistika.
      await tester.tap(nav('stats'));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('stats-period')), findsOneWidget);
      expect(count('/admin/stats'), 1);
      expect(requests.lastWhere((r) => r.url.path == '/admin/stats').url.queryParameters['period'], '30d');

      // Mijozlar.
      await tester.tap(nav('customers'));
      await tester.pumpAndSettle();
      expect(find.text('Sardor Mijoz'), findsOneWidget);

      // OnDex kuryerlari: faqat restoranga BOG'LANMAGAN kuryer.
      await tester.tap(nav('couriers'));
      await tester.pumpAndSettle();
      expect(find.text('Umumiy Kuryer'), findsOneWidget);
      expect(find.text('Anvar Kuryer'), findsNothing);
      expect(find.text('Boshqa Kuryer'), findsNothing);

      // Sozlamalar.
      await tester.tap(nav('settings'));
      await tester.pumpAndSettle();
      expect(count('/support/contacts'), 1);

      // OnDexMap.
      await tester.tap(nav('ondexmap'));
      await tester.pumpAndSettle();

      // Restoran'ga qaytilsa ochiq restoran o'z joyida; ko'rilgan bo'limlar qayta yuklanmaydi.
      await tester.tap(nav('restaurants'));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('restaurant-module-tabs')), findsOneWidget);
      expect(find.byKey(const ValueKey('restaurant-module-title')), findsOneWidget);
      await tester.tap(nav('stats'));
      await tester.pumpAndSettle();
      expect(count('/admin/stats'), 1);
      expect(tester.takeException(), isNull);

      // Global chat qutisi (AdminShell ishga tushirgan) taymeri to'xtatiladi.
      // stop() haqiqiy asinxron kutadi — soxta soatdan tashqarida.
      await tester.runAsync(supportInbox.stop);
    });
  });

  testWidgets('sidebar past oynada (768 px) sig\'adi va overflow bermaydi', (tester) async {
    await _run(tester, const AdminShell(), (requests) async {
      expect(tester.takeException(), isNull);
      expect(find.byKey(const ValueKey('nav-ondexmap')), findsOneWidget);
      expect(find.byTooltip('Chiqish'), findsOneWidget);
      await tester.runAsync(supportInbox.stop);
    }, size: const Size(1366, 600));
  });

  testWidgets('eski server filtrni bilmasa ham begona restoran buyurtmasi chiqmaydi', (tester) async {
    await _run(tester, const RestaurantsSection(), (requests) async {
      await _openRestaurant(tester, 'rest-a');
      expect(find.text('101'), findsOneWidget);
      expect(find.text('102'), findsOneWidget);
      expect(find.text('201'), findsNothing);
    }, oldServer: true);
  });

  testWidgets('tor oynada tablar sig\'adi (overflow yo\'q)', (tester) async {
    await _run(tester, const RestaurantsSection(), (requests) async {
      await _openRestaurant(tester, 'rest-a');
      expect(tester.takeException(), isNull);
      await _tab(tester, 'Chat');
      expect(tester.takeException(), isNull);
    }, size: const Size(1000, 700));
  });
}

import 'dart:convert';

import 'package:chust_restaurant/api.dart';
import 'package:chust_restaurant/notification_center.dart';
import 'package:chust_restaurant/pages/notifications_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

// "Bildirishnomalar": namuna bo'yicha ko'rinish (har kenglikda, yon
// aylantirishsiz), jonli yangi xabar (takrorsiz), o'qish va "barchasini
// o'qish", boshqa kompyuterdagi o'qish, filtrlar, sahifalash va markazning
// qayta ulanishdagi to'ldirishi.

const _titles = {
  'new': 'Yangi',
  'success': 'Muvaffaqiyatli',
  'info': 'Ma\'lumot',
  'important': 'Muhim',
  'report': 'Hisobot',
  'update': 'Yangilanish',
  'activity': 'Faoliyat',
  'reminder': 'Eslatma',
};

Map<String, dynamic> _alert(int seq, String kind, String category, String title, String body, {bool read = false}) => {
      'id': 'n$seq',
      'seq': seq,
      'kind': kind,
      'category': category,
      'category_title': _titles[category],
      'title': title,
      'body': body,
      'data': {},
      'read': read,
      'created_at': DateTime.now().toUtc().subtract(Duration(minutes: 100 - seq)).toIso8601String(),
    };

List<Map<String, dynamic>> _fixture() => [
      _alert(9, 'new_order', 'new', 'Yangi buyurtma', '#140926-0000124 buyurtma tushdi · Stol: 4. Jami: 180 000 so\'m'),
      _alert(8, 'payment_received', 'success', 'To\'lov qabul qilindi',
          'Karta orqali 140 000 so\'m to\'lov amalga oshirildi. Buyurtma #140926-0000123'),
      _alert(7, 'staff_added', 'info', 'Yangi xodim qo\'shildi', 'Malika To\'xtayeva (Ofitsiant) xodimlar ro\'yxatiga qo\'shildi.'),
      _alert(6, 'order_cancelled', 'important', 'Buyurtma bekor qilindi', 'Mijoz #140926-0000120 buyurtmani bekor qildi. Summa: 45 000 so\'m',
          read: true),
      _alert(5, 'daily_report', 'report', 'Kunlik hisobot tayyor',
          '13-sentabr: 34 ta buyurtma, 31 tasi bajarildi, 3 tasi bekor qilindi. Savdo: 4 560 000 so\'m. Ko\'rish uchun bosing.',
          read: true),
      _alert(4, 'platform_update', 'update', 'Tizim yangilanishi', 'Restoran panelida yangi funksiyalar qo\'shildi.', read: true),
      _alert(3, 'staff_status', 'activity', 'Xodim ta\'tilga chiqdi', 'Jasurbek Yo\'ldoshev (Ofitsiant) — holati: Ta\'tilda.',
          read: true),
      _alert(2, 'order_waiting', 'reminder', 'Buyurtma qabul qilinmagan', '#140926-0000119 buyurtma 6 daqiqadan beri javob kutmoqda.',
          read: true),
      _alert(1, 'dispatch_failed', 'important', 'Kuryer topishda xatolik', 'Kuryer qidirishda tizim xatosi yuz berdi.', read: true),
    ];

Map<String, dynamic> _page(List<Map<String, dynamic>> items, {int nextBefore = 0, int unread = 3}) {
  final by = <String, int>{};
  for (final it in items) {
    by[it['category'] as String] = (by[it['category'] as String] ?? 0) + 1;
  }
  return {
    'items': items,
    'next_before': nextBefore,
    'counts': {'total': items.length, 'unread': unread, 'by_category': by},
    'unread': unread,
    'latest_seq': 9,
    'categories': [],
  };
}

http.Response _json(Object body, [int status = 200]) => http.Response.bytes(
      utf8.encode(jsonEncode(body)),
      status,
      headers: {'content-type': 'application/json; charset=utf-8'},
    );

NotificationCenter _center() =>
    NotificationCenter(bus: LiveBus(ticketProvider: () async => 't', urlBuilder: (t) => 'ws://localhost/ws?ticket=$t'));

Future<void> _run(
  WidgetTester tester, {
  Size size = const Size(1600, 1000),
  NotificationCenter? center,
  http.Response? Function(http.Request request)? respond,
  VoidCallback? onOrders,
  VoidCallback? onSettings,
  required Future<void> Function(List<http.Request> requests, NotificationCenter center) body,
}) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  api.rid = 'rest-a';
  final requests = <http.Request>[];
  final c = center ?? _center();
  await http.runWithClient(
    () async {
      await tester.pumpWidget(MaterialApp(
        home: Scaffold(
          body: NotificationsPage(center: c, onOpenOrders: onOrders, onOpenSettings: onSettings),
        ),
      ));
      await tester.pumpAndSettle();
      await body(requests, c);
      await tester.pumpWidget(const SizedBox());
    },
    () => MockClient((request) async {
      requests.add(request);
      final custom = respond?.call(request);
      if (custom != null) return custom;
      final path = request.url.path;
      if (request.method == 'GET' && path == '/restaurants/rest-a/notifications') return _json(_page(_fixture()));
      if (request.method == 'POST' && path.endsWith('/read')) return _json({'unread': 2});
      if (request.method == 'POST' && path.endsWith('/read-all')) return _json({'updated': 3, 'unread': 0});
      if (path.endsWith('/summary')) return _json({'unread': 3, 'latest_seq': 9});
      return _json({'error': 'kutilmagan so\'rov'}, 404);
    }),
  );
}

Finder _horizontalScrollables() => find.byWidgetPredicate((w) =>
    (w is ScrollView && w.scrollDirection == Axis.horizontal) ||
    (w is SingleChildScrollView && w.scrollDirection == Axis.horizontal));

void main() {
  for (final size in const [Size(1600, 1000), Size(1180, 900), Size(820, 1000), Size(420, 1000)]) {
    testWidgets('namuna bo\'yicha ko\'rinish, overflowsiz — ${size.width.toInt()}px', (tester) async {
      await _run(tester, size: size, body: (_, __) async {
        expect(tester.takeException(), isNull);
        for (final t in ['Bildirishnomalar', 'Barchasini o\'qish', 'Jami bildirishnomalar', 'Filtrlash']) {
          expect(find.text(t), findsOneWidget, reason: t);
        }
        // "Muhim yangiliklar" bloki va sarlavha oldidagi belgi yo'q.
        expect(find.text('Muhim yangiliklar'), findsNothing);
        expect(tester.getTopLeft(find.text('Bildirishnomalar')).dx, closeTo(24, 0.5));
        expect(find.text('9 ta'), findsOneWidget);
        expect(find.byKey(const ValueKey('alert-n9')), findsOneWidget);
        expect(find.byKey(const ValueKey('alert-unread-n9')), findsOneWidget);
        expect(find.byKey(const ValueKey('alert-unread-n6')), findsNothing);
        expect(_horizontalScrollables(), findsNothing);
      });
    });
  }

  testWidgets('jonli yangi xabar darhol ro\'yxat boshiga, takrorsiz', (tester) async {
    await _run(tester, body: (requests, center) async {
      final event = {
        'type': 'restaurant_notification',
        'notification': _alert(10, 'new_order', 'new', 'Yangi buyurtma', '#140926-0000125 buyurtma tushdi. Jami: 95 000 so\'m'),
        'unread': 4,
      };
      center.handleEvent(event);
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('alert-n10')), findsOneWidget);
      expect(tester.getTopLeft(find.byKey(const ValueKey('alert-n10'))).dy,
          lessThan(tester.getTopLeft(find.byKey(const ValueKey('alert-n9'))).dy));
      expect(find.text('10 ta'), findsOneWidget);
      expect(tester.widget<Text>(find.byKey(const ValueKey('alerts-count-new'))).data, '2');

      // Xuddi shu xabar ikkinchi yo'ldan (qayta ulanish) — bir marta.
      center.handleEvent(event);
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('alert-n10')), findsOneWidget);
      expect(find.text('10 ta'), findsOneWidget);
      // Serverga qo'shimcha so'rov yuborilmaydi — jonli qo'shildi.
      expect(requests.where((r) => r.method == 'GET'), hasLength(1));
    });
  });

  testWidgets('bosish: o\'qilgan bo\'ladi va kerakli sahifa ochiladi', (tester) async {
    var opened = 0;
    await _run(tester, onOrders: () => opened++, body: (requests, _) async {
      await tester.tap(find.byKey(const ValueKey('alert-n9')));
      await tester.pumpAndSettle();
      expect(requests.where((r) => r.method == 'POST' && r.url.path == '/restaurants/rest-a/notifications/n9/read'),
          hasLength(1));
      expect(find.byKey(const ValueKey('alert-unread-n9')), findsNothing);
      expect(opened, 1);
    });
  });

  testWidgets('barchasini o\'qish — ko\'rilgan oxirgi raqamgacha', (tester) async {
    await _run(tester, body: (requests, _) async {
      await tester.tap(find.byKey(const ValueKey('alerts-read-all')));
      await tester.pumpAndSettle();
      final req = requests.singleWhere((r) => r.url.path.endsWith('/read-all'));
      expect(jsonDecode(req.body), {'up_to_seq': 9});
      for (final id in ['n9', 'n8', 'n7']) {
        expect(find.byKey(ValueKey('alert-unread-$id')), findsNothing);
      }
      expect(find.text('Barcha bildirishnomalar o\'qildi'), findsOneWidget);
    });
  });

  testWidgets('boshqa kompyuterda o\'qildi — bu yerda ham belgi o\'chadi', (tester) async {
    await _run(tester, body: (_, center) async {
      center.handleEvent({'type': 'restaurant_notifications_read', 'id': 'n8', 'unread': 2});
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('alert-unread-n8')), findsNothing);
      expect(find.byKey(const ValueKey('alert-unread-n9')), findsOneWidget);
      center.handleEvent({'type': 'restaurant_notifications_read', 'up_to_seq': 9, 'unread': 0});
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('alert-unread-n9')), findsNothing);
      expect(find.byKey(const ValueKey('alert-unread-n7')), findsNothing);
    });
  });

  testWidgets('filtrlar va qidiruv serverga yuboriladi', (tester) async {
    await _run(tester, body: (requests, _) async {
      await tester.tap(find.byKey(const ValueKey('alert-filter-category')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Muhim').last);
      await tester.pumpAndSettle();
      expect(requests.last.url.queryParameters['category'], 'important');

      await tester.tap(find.byKey(const ValueKey('alert-filter-period')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Bugun').last);
      await tester.pumpAndSettle();
      expect(requests.last.url.queryParameters['period'], 'today');

      await tester.enterText(find.byKey(const ValueKey('alert-search')), 'Lavash');
      await tester.pump(const Duration(milliseconds: 200));
      expect(requests.last.url.queryParameters['q'], isNull, reason: 'har harfda so\'rov yuborilmasin');
      await tester.pump(const Duration(milliseconds: 400));
      await tester.pumpAndSettle();
      expect(requests.last.url.queryParameters['q'], 'Lavash');
    });
  });

  testWidgets('sahifalash: pastga aylantirilganda eskiroqlari', (tester) async {
    final first = _fixture().take(6).toList();
    final rest = _fixture().skip(6).toList();
    await _run(
      tester,
      size: const Size(1600, 700),
      respond: (r) {
        if (r.method != 'GET' || !r.url.path.endsWith('/notifications')) return null;
        return r.url.queryParameters['before'] == '4' ? _json(_page(rest)) : _json(_page(first, nextBefore: 4));
      },
      body: (requests, _) async {
        await tester.drag(find.byKey(const ValueKey('alerts-list')), const Offset(0, -2000));
        await tester.pumpAndSettle();
        expect(requests.any((r) => r.url.queryParameters['before'] == '4'), isTrue);
        await tester.drag(find.byKey(const ValueKey('alerts-list')), const Offset(0, -2000));
        await tester.pumpAndSettle();
        expect(find.byKey(const ValueKey('alert-n1')), findsOneWidget);
      },
    );
  });

  testWidgets('"..." menyusidan bildirishnoma sozlamalari', (tester) async {
    var opened = 0;
    await _run(tester, onSettings: () => opened++, body: (_, __) async {
      await tester.tap(find.byKey(const ValueKey('alerts-more')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Bildirishnoma sozlamalari'));
      await tester.pumpAndSettle();
      expect(opened, 1);
    });
  });

  test('markaz: qayta ulanishda o\'tkazib yuborilganlar tartibda va takrorsiz', () async {
    final center = _center();
    final got = <int>[];
    final sub = center.incoming.listen((n) => got.add(n['seq'] as int));
    await http.runWithClient(() async {
      center.handleEvent({'type': 'restaurant_notification', 'notification': _alert(5, 'new_order', 'new', 't', 'b'), 'unread': 1});
      expect(center.latestSeq, 5);
      await center.sync();
      // Jonli kanal orqali ham 6 kelsa — ikkinchi marta chiqmaydi.
      center.handleEvent({'type': 'restaurant_notification', 'notification': _alert(6, 'new_order', 'new', 't', 'b'), 'unread': 3});
    }, () => MockClient((request) async {
      if (request.url.queryParameters['after'] == '5') {
        return _json({
          'items': [_alert(6, 'new_order', 'new', 't', 'b'), _alert(7, 'staff_added', 'info', 't', 'b')],
          'unread': 3,
        });
      }
      return _json({'items': [], 'unread': 3});
    }));
    await Future<void>.delayed(Duration.zero);
    expect(got, [5, 6, 7]);
    expect(center.latestSeq, 7);
    expect(center.unread, 3);
    await sub.cancel();
  });
}

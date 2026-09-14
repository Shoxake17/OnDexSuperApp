import 'dart:convert';

import 'package:chust_restaurant/api.dart';
import 'package:chust_restaurant/pages/statistics_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

// "Statistika" sahifasi testlari.
//
// ┌─ NEGA WIDGET TESTI ────────────────────────────────────────────────┐
// Sahifada qat'iy balandlikdagi kartochkalar va ko'p ustunli jadvallar
// bor — "overflow" faqat MA'LUM oyna kengligida chiqadi va ko'z bilan
// tekshirib topish ishonchsiz. Widget testida u oddiy xatoga aylanadi.
//
// Server so'rovi `http.runWithClient` bilan ushlanadi: `ApiClient.send`
// yuqori darajadagi `http.get` dan foydalanadi, ya'ni ilova kodiga soxta
// klient kiritish shart emas.
// └────────────────────────────────────────────────────────────────────┘

/// Test daraxtida yuqori panel o'rnini egallaydigan qator balandligi.
const _toolbarHeight = 60.0;

String _date(DateTime d) =>
    '${d.year}-${d.month.toString().padLeft(2, '0')}-${d.day.toString().padLeft(2, '0')}';

Map<String, dynamic> _summary({
  int revenue = 0,
  int orders = 0,
  int completed = 0,
  int inProgress = 0,
  int inProgressTiyin = 0,
  int cancelled = 0,
  int newCustomers = 0,
  int returning = 0,
  int? avgOrder,
  int? prep,
}) =>
    {
      'revenue_tiyin': revenue,
      'orders': orders,
      'completed': completed,
      'in_progress': inProgress,
      'in_progress_tiyin': inProgressTiyin,
      'cancelled': cancelled,
      'new_customers': newCustomers,
      'returning_customers': returning,
      'avg_order_tiyin': avgOrder,
      'avg_prep_minutes': prep,
    };

Map<String, dynamic> _fullStats() => {
      'from': '2026-08-15',
      'to': '2026-09-13',
      'previous_from': '2026-07-16',
      'previous_to': '2026-08-14',
      'granularity': 'day',
      'timezone': 'Asia/Tashkent',
      'current': _summary(
          revenue: 2418000000,
          orders: 1248,
          completed: 1156,
          inProgress: 64,
          inProgressTiyin: 135000000,
          cancelled: 28,
          newCustomers: 156,
          returning: 342,
          avgOrder: 2091695,
          prep: 18),
      'previous': _summary(
          revenue: 2049152500,
          orders: 1114,
          completed: 1014,
          inProgress: 70,
          cancelled: 30,
          newCustomers: 128,
          returning: 297,
          avgOrder: 2020862,
          prep: 21),
      'series': [
        for (var i = 0; i < 30; i++)
          {
            'start': _date(DateTime(2026, 8, 15 + i)),
            'end': _date(DateTime(2026, 8, 15 + i)),
            'revenue_tiyin': (i % 7 + 1) * 45000000,
            'orders': i % 9 + 3,
          },
      ],
      'categories': [
        for (final (i, name) in [
          'Lavashlar',
          'Burgerlar',
          'Pizzalar',
          'Ichimliklar',
          'Salatlar',
          'Desertlar',
          'Кока-кола',
          'Turkumsiz',
        ].indexed)
          {'name': name, 'qty': 500 - i * 50, 'sales_tiyin': (500 - i * 50) * 1800000},
      ],
      'top_products': [
        for (var i = 0; i < 12; i++)
          {
            'product_id': 'p$i',
            // Juda uzun nom ham qatorni buzmasligi kerak.
            'name': i == 0 ? 'Lavash mini juda uzun nomli maxsus taom ikki kishilik' : 'Taom $i',
            'category': i.isEven ? 'Fast Food' : 'Juda uzun nomli turkum',
            'image_url': '',
            'qty': 256 - i * 10,
            'sales_tiyin': (256 - i * 10) * 1800000,
          },
      ],
      'top_products_total': 60,
      'busiest_hours': {'start_hour': 12, 'end_hour': 14, 'orders': 312},
      'busiest_weekday': {'weekday': 5, 'avg_orders': 48.5, 'orders': 194},
    };

Map<String, dynamic> _emptyStats() => {
      'from': '2026-09-07',
      'to': '2026-09-13',
      'previous_from': '2026-08-31',
      'previous_to': '2026-09-06',
      'granularity': 'day',
      'timezone': 'Asia/Tashkent',
      'current': _summary(),
      'previous': _summary(),
      'series': [
        for (var i = 0; i < 7; i++)
          {
            'start': _date(DateTime(2026, 9, 7 + i)),
            'end': _date(DateTime(2026, 9, 7 + i)),
            'revenue_tiyin': 0,
            'orders': 0,
          },
      ],
      'categories': <Object>[],
      'top_products': <Object>[],
      'top_products_total': 0,
      'busiest_hours': null,
      'busiest_weekday': null,
    };

List<Map<String, dynamic>> _recentOrders() {
  final now = DateTime.now().toUtc();
  String at(int minutesAgo) => now.subtract(Duration(minutes: minutesAgo)).toIso8601String();
  return [
    // Ataylab tartibsiz: jadval o'zi saralashi kerak.
    {'order_number': '130926-5607001', 'created_at': at(300), 'customer_phone': '+998900000001',
      'items': [{'qty': 1, 'name': 'Eski'}], 'total_tiyin': 100000, 'status': 'delivered'},
    {'order_number': '130926-4384732', 'created_at': at(5), 'customer_phone': '+998902784207',
      'items': [{'qty': 2, 'name': 'HotDog Mini'}], 'total_tiyin': 3600000, 'status': 'delivered'},
    {'order_number': '130926-5608152', 'created_at': at(15), 'type': 'dine_in', 'table_label': '5',
      'items': [{'qty': 1, 'name': 'Lavash'}, {'qty': 3, 'name': 'Cola'}], 'total_tiyin': 2800000,
      'status': 'preparing'},
    {'order_number': '130926-5607993', 'created_at': at(40), 'customer_phone': '+998909876543',
      'items': [{'qty': 1, 'name': 'Salat'}], 'total_tiyin': 1800000, 'status': 'cancelled'},
    {'order_number': '130926-5608017', 'created_at': at(60), 'customer_phone': '+998901234567',
      'items': [{'qty': 3, 'name': 'Ichimlik'}], 'total_tiyin': 4500000, 'status': 'served'},
    {'order_number': '130926-5608121', 'created_at': at(90), 'customer_phone': '+998974521003',
      'items': <Object>[], 'total_tiyin': 2800000, 'status': 'accepted'},
  ];
}

/// "Barcha buyurtmalar" sahifasi uchun buyurtma: [i] kattalashgan sari eskiroq.
Map<String, dynamic> _historyOrder(int i, {String status = 'delivered'}) => {
      'id': 'o$i',
      'order_number': '010326-${9000 + i}',
      'created_at': DateTime.utc(2026, 9, 1).subtract(Duration(hours: i)).toIso8601String(),
      'customer_phone': '+99890${(1000000 + i).toString()}',
      'items': [
        {'qty': 2, 'name': 'Taom $i'},
        {'qty': 1, 'name': 'Cola'},
      ],
      'total_tiyin': 1800000,
      'status': status,
    };

Map<String, dynamic> _lifetime() => {
      'orders': 35,
      'completed': 30,
      'in_progress': 2,
      'cancelled': 3,
      'revenue_tiyin': 5400000000,
      'in_progress_tiyin': 7200000,
      'avg_order_tiyin': 180000000,
      'first_order_at': '2026-03-01T07:00:00Z',
      'last_order_at': '2026-09-01T00:00:00Z',
    };

String _dayKey(DateTime d) =>
    'range-day-${d.year}-${d.month.toString().padLeft(2, '0')}-${d.day.toString().padLeft(2, '0')}';

http.Response _json(Object body, [int status = 200]) => http.Response.bytes(
      utf8.encode(jsonEncode(body)),
      status,
      headers: {'content-type': 'application/json; charset=utf-8'},
    );

/// Sahifani (yuqori panel tugmalari bilan birga) berilgan o'lchamda ochadi.
/// Butun tana soxta klient zonasi ICHIDA bajariladi — bosish natijasidagi
/// keyingi so'rovlar ham ushlanadi.
Future<void> _run(
  WidgetTester tester, {
  required Size size,
  required http.Response Function(http.Request request) respond,
  List<Map<String, dynamic>>? recent,
  required Future<void> Function(StatisticsController controller) body,
}) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  api.rid = 'rest-a';
  final controller = StatisticsController();
  addTearDown(controller.dispose);
  controller.setRecentOrders(recent ?? _recentOrders());

  await http.runWithClient(
    () async {
      await tester.pumpWidget(MaterialApp(
        home: Scaffold(
          body: Column(
            children: [
              // Toolbar'ga CHEKLANGAN kenglik beriladi: haqiqiy `TopBar` da
              // u restoran nomi ustunidan qolgan joyni oladi. `Row` ning
              // egiluvchan bo'lmagan bolasi sifatida cheksiz kenglik
              // olganda sana yorlig'i qisqara olmasdi va tor test
              // oynasida yolg'on overflow chiqardi.
              SizedBox(
                height: _toolbarHeight,
                child: Align(
                  alignment: Alignment.centerRight,
                  child: StatisticsToolbar(controller: controller, restaurantName: 'Test restoran'),
                ),
              ),
              Expanded(child: StatisticsPage(controller: controller)),
            ],
          ),
        ),
      ));
      await tester.pumpAndSettle();
      await body(controller);
      // Daraxt kontrollerdan OLDIN yig'ishtiriladi.
      await tester.pumpWidget(const SizedBox());
    },
    () => MockClient((request) async => respond(request)),
  );
}

void main() {
  // 1672 — 1920 px monitor (yon menyu 248 px); 420 — juda tor oyna.
  // 1250/1000/956/900 — joylashuvlar almashadigan chegaralar atrofi.
  for (final width in [1672.0, 1440.0, 1250.0, 1000.0, 956.0, 900.0, 720.0, 420.0]) {
    testWidgets('to\'liq ma\'lumot $width px kenglikda xatosiz chiziladi', (tester) async {
      await _run(
        tester,
        size: Size(width, 2600),
        respond: (_) => _json(_fullStats()),
        body: (_) async {
          expect(tester.takeException(), isNull);
          // Sarlavha va tavsif olib tashlangan.
          expect(find.text('Statistika'), findsNothing);
          expect(find.textContaining('batafsil ma\'lumotlar'), findsNothing);
          // Olib tashlangan bloklar.
          expect(find.text('Buyurtmalar statistikasi'), findsNothing);
          expect(find.text('Sotuvlar bo\'yicha kategoriyalar'), findsNothing);
          expect(find.text('Buyurtmalar holati'), findsNothing);
          // Yangi joylashuv.
          expect(find.text('Jami tushum'), findsOneWidget);
          expect(find.text('24 180 000 so\'m'), findsOneWidget);
          // Jarayondagi buyurtmalar tushumga qo'shilmaydi, alohida yoziladi.
          expect(find.text('+1 350 000 so\'m jarayonda (64 ta)'), findsOneWidget);
          expect(find.text('Tushum statistikasi'), findsOneWidget);
          expect(find.text('Buyurtmalar bo\'yicha mahsulotlar'), findsOneWidget);
          expect(find.text('Eng ko\'p sotilgan mahsulotlar'), findsOneWidget);
          expect(find.text('So\'nggi buyurtmalar'), findsOneWidget);
          expect(find.text('Boshqa'), findsOneWidget);
          expect(find.text('12:00 - 14:00'), findsOneWidget);
          expect(find.text('Juma'), findsOneWidget);
          expect(find.text('Eksport'), findsOneWidget);
        },
      );
    });
  }

  // Asosiy talab: 1920×1080 ekranda scroll kerak emas. Yon menyu 248 px,
  // yuqori panel 78 px, Windows oyna sarlavhasi va vazifalar paneli ~80 px
  // — sahifaga ~920 px qoladi; bu yerda zaxira bilan 900 px olingan.
  testWidgets('1920×1080 ekranda sahifa scroll\'siz sig\'adi', (tester) async {
    await _run(
      tester,
      size: const Size(1672, 900 + _toolbarHeight),
      respond: (_) => _json(_fullStats()),
      body: (_) async {
        expect(tester.takeException(), isNull);
        final scrollable = tester.state<ScrollableState>(find.byType(Scrollable));
        expect(scrollable.position.maxScrollExtent, 0,
            reason: 'sahifa 900 px balandlikka sig\'masdan scroll paydo bo\'ldi');
      },
    );
  });

  testWidgets('so\'nggi buyurtmalar: saralangan, holat va mijoz ko\'rinadi', (tester) async {
    await _run(
      tester,
      size: const Size(1672, 1200),
      respond: (_) => _json(_fullStats()),
      body: (_) async {
        expect(find.text('#4384732'), findsOneWidget);
        expect(find.text('+998902784207'), findsOneWidget);
        expect(find.text('5-stol'), findsOneWidget);
        expect(find.text('2x HotDog Mini'), findsOneWidget);
        expect(find.text('1x Lavash +1'), findsOneWidget);
        expect(find.text('Yetkazildi'), findsOneWidget);
        expect(find.text('Bekor qilindi'), findsOneWidget);
        // 6 tadan eng eskisi (5 soat oldin) ko'rsatilmaydi.
        expect(find.text('#5607001'), findsNothing);
      },
    );
  });

  testWidgets('barchasini ko\'rish: butun tarix, sahifalash, filtr va orqaga', (tester) async {
    final historyUrls = <Uri>[];
    await _run(
      tester,
      size: const Size(1672, 1000),
      respond: (request) {
        if (!request.url.path.endsWith('/orders/history')) return _json(_fullStats());
        historyUrls.add(request.url);
        final q = request.url.queryParameters;
        if (q['status'] == 'completed') {
          return _json({
            'orders': [_historyOrder(100)],
            'next_cursor': '',
            'summary': _lifetime(),
          });
        }
        if (q['cursor'] == 'sahifa-2') {
          return _json({
            'orders': [for (var i = 30; i < 35; i++) _historyOrder(i)],
            'next_cursor': '',
          });
        }
        return _json({
          'orders': [for (var i = 0; i < 30; i++) _historyOrder(i)],
          'next_cursor': 'sahifa-2',
          'summary': _lifetime(),
        });
      },
      body: (_) async {
        await tester.tap(find.text('Barchasini ko\'rish →'));
        await tester.pumpAndSettle();
        expect(tester.takeException(), isNull);

        expect(find.text('Barcha buyurtmalar'), findsOneWidget);
        expect(find.textContaining('birinchi buyurtma: 1-mart, 2026'), findsOneWidget);
        expect(find.text('54 000 000 so\'m'), findsOneWidget);
        expect(find.text('1 800 000 so\'m'), findsOneWidget);
        expect(find.text('+72 000 so\'m jarayonda (2 ta)'), findsOneWidget);
        expect(find.text('Sana va vaqt'), findsOneWidget);
        expect(find.text('#9000'), findsOneWidget);
        expect(find.text('2x Taom 0, 1x Cola'), findsOneWidget);
        expect(historyUrls.single.queryParameters['status'], 'all');
        expect(historyUrls.single.queryParameters.containsKey('cursor'), isFalse);
        // Davr tanlagich va Eksport butun tarixga tegishli emas.
        expect(find.text('Eksport'), findsNothing);

        await tester.drag(find.byType(ListView), const Offset(0, -4000));
        await tester.pumpAndSettle();
        expect(historyUrls.last.queryParameters['cursor'], 'sahifa-2');
        await tester.drag(find.byType(ListView), const Offset(0, -4000));
        await tester.pumpAndSettle();
        expect(find.text('Hammasi ko\'rsatildi · 35 ta'), findsOneWidget);
        expect(historyUrls, hasLength(2));

        await tester.tap(find.text('Bajarilgan'));
        await tester.pumpAndSettle();
        expect(historyUrls.last.queryParameters['status'], 'completed');
        expect(historyUrls.last.queryParameters.containsKey('cursor'), isFalse);
        expect(find.text('#9100'), findsOneWidget);
        expect(find.text('#9000'), findsNothing);
        expect(tester.takeException(), isNull);

        await tester.tap(find.text('Statistika'));
        await tester.pumpAndSettle();
        expect(find.text('Barcha buyurtmalar'), findsNothing);
        expect(find.text('Jami tushum'), findsOneWidget);
        expect(find.text('Eksport'), findsOneWidget);
      },
    );
  });

  testWidgets('barcha buyurtmalar: yuqori paneldagi davr filtri va qaytish', (tester) async {
    final urls = <Uri>[];
    var back = 0;
    await _run(
      tester,
      size: const Size(1672, 1000),
      respond: (request) {
        if (!request.url.path.endsWith('/orders/history')) return _json(_fullStats());
        urls.add(request.url);
        return _json({
          'orders': [_historyOrder(0)],
          'next_cursor': '',
          'summary': _lifetime(),
        });
      },
      body: (controller) async {
        // "Buyurtmalar" sahifasidagi "Tarix" shunday ochadi.
        controller.openHistory(backLabel: 'Buyurtmalar', onBack: () => back++);
        await tester.pumpAndSettle();
        expect(find.text('Barcha vaqt'), findsOneWidget);
        expect(find.text('Buyurtmalar'), findsOneWidget);
        expect(urls.single.queryParameters.containsKey('from'), isFalse);

        await tester.tap(find.text('Barcha vaqt'));
        await tester.pumpAndSettle();
        expect(find.text('Barcha vaqt — sana bo\'yicha cheklovsiz'), findsOneWidget);
        await tester.tap(find.text('Oxirgi 7 kun'));
        await tester.pumpAndSettle();
        expect(find.text('7 kun tanlandi'), findsOneWidget);
        await tester.tap(find.text('Qo\'llash'));
        await tester.pumpAndSettle();

        expect(urls, hasLength(2));
        final q = urls.last.queryParameters;
        expect(DateTime.parse(q['to']!).difference(DateTime.parse(q['from']!)).inDays, 6);
        expect(q.containsKey('cursor'), isFalse);
        expect(find.textContaining('Tanlangan davr:'), findsOneWidget);
        expect(find.text('Tanlangan davrda'), findsOneWidget);
        expect(find.text('Barcha vaqt'), findsNothing);
        expect(tester.takeException(), isNull);

        await tester.tap(find.byTooltip('Filtrni tozalash'));
        await tester.pumpAndSettle();
        expect(urls, hasLength(3));
        expect(urls.last.queryParameters.containsKey('from'), isFalse);
        expect(find.text('Barcha vaqt'), findsOneWidget);

        await tester.tap(find.text('Buyurtmalar'));
        await tester.pumpAndSettle();
        expect(back, 1);
        expect(controller.showingHistory, isFalse);
      },
    );
  });

  testWidgets('"Barcha buyurtmalar" tor oynada ham xatosiz chiziladi', (tester) async {
    await _run(
      tester,
      size: const Size(420, 900),
      respond: (request) => request.url.path.endsWith('/orders/history')
          ? _json({
              'orders': [for (var i = 0; i < 3; i++) _historyOrder(i)],
              'next_cursor': '',
              'summary': _lifetime(),
            })
          : _json(_fullStats()),
      body: (controller) async {
        controller.openHistory();
        await tester.pumpAndSettle();
        expect(tester.takeException(), isNull);
        expect(find.text('Barcha buyurtmalar'), findsOneWidget);
        expect(find.text('#9002'), findsOneWidget);
      },
    );
  });

  testWidgets('davr tanlagich: tezkor tanlov va kalendardan tanlash', (tester) async {
    final urls = <Uri>[];
    await _run(
      tester,
      size: const Size(1672, 1200),
      respond: (request) {
        urls.add(request.url);
        return _json(_fullStats());
      },
      body: (_) async {
        await tester.tap(find.byIcon(Icons.calendar_today_rounded));
        await tester.pumpAndSettle();
        expect(find.text('Davrni tanlang'), findsOneWidget);
        expect(find.text('30 kun tanlandi'), findsOneWidget);
        expect(tester.takeException(), isNull);

        await tester.tap(find.text('Oxirgi 7 kun'));
        await tester.pumpAndSettle();
        expect(find.text('7 kun tanlandi'), findsOneWidget);
        await tester.tap(find.text('Qo\'llash'));
        await tester.pumpAndSettle();
        expect(find.text('Davrni tanlang'), findsNothing);
        expect(urls, hasLength(2));
        var from = DateTime.parse(urls.last.queryParameters['from']!);
        var to = DateTime.parse(urls.last.queryParameters['to']!);
        expect(to.difference(from).inDays, 6);

        final now = DateTime.now();
        final end = DateTime(now.year, now.month, now.day);
        final start = DateTime(end.year, end.month, end.day - 3);
        await tester.tap(find.byIcon(Icons.calendar_today_rounded));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(ValueKey(_dayKey(start))));
        await tester.pumpAndSettle();
        expect(find.text('Tugash sanasini tanlang'), findsOneWidget);
        await tester.tap(find.byKey(ValueKey(_dayKey(end))));
        await tester.pumpAndSettle();
        expect(find.text('4 kun tanlandi'), findsOneWidget);

        // Kelajakdagi kun bosilmaydi — tanlov o'zgarmaydi.
        final tomorrow = find.byKey(ValueKey(_dayKey(DateTime(end.year, end.month, end.day + 1))));
        if (tomorrow.evaluate().isNotEmpty) {
          await tester.tap(tomorrow, warnIfMissed: false);
          await tester.pumpAndSettle();
          expect(find.text('4 kun tanlandi'), findsOneWidget);
        }

        await tester.tap(find.text('Qo\'llash'));
        await tester.pumpAndSettle();
        expect(urls, hasLength(3));
        from = DateTime.parse(urls.last.queryParameters['from']!);
        to = DateTime.parse(urls.last.queryParameters['to']!);
        expect(from, start);
        expect(to, end);
        expect(tester.takeException(), isNull);
      },
    );
  });

  testWidgets('bo\'sh davr — soxta nol emas, halol "ma\'lumot yo\'q"', (tester) async {
    await _run(
      tester,
      size: const Size(1300, 2000),
      respond: (_) => _json(_emptyStats()),
      recent: const [],
      body: (_) async {
        expect(tester.takeException(), isNull);
        expect(find.text('Bu davrda tushum yo\'q'), findsOneWidget);
        expect(find.text('Bu davrda sotuv yo\'q'), findsOneWidget);
        expect(find.text('Bu davrda sotilgan mahsulot yo\'q'), findsOneWidget);
        expect(find.text('Hali buyurtma yo\'q'), findsOneWidget);
        // O'rtacha qiymatlar bo'sh chiziq emas, 0 — sababi bilan.
        expect(find.text('0 so\'m'), findsNWidgets(2));
        expect(find.text('Bajarilgan buyurtma yo\'q'), findsOneWidget);
        expect(find.text('0 daqiqa'), findsOneWidget);
        expect(find.text('Tayyorlangan buyurtma yo\'q'), findsOneWidget);
        // Gavjum vaqt va faol kun — sonli qiymati yo'q, "—" qoladi.
        expect(find.text('—'), findsNWidgets(2));
      },
    );
  });

  // Book Cafe'dagi haqiqiy holat: davrda yakunlangan buyurtma yo'q, lekin
  // 2 tasi "Tayyor" holatida turibdi. Kartochkada shunchaki "0 so'm"
  // turmasligi — sababi aytilishi kerak (jarayondagi summa tushumga
  // qo'shilmaydi).
  testWidgets('tushum 0, lekin jarayonda buyurtma bor — sababi ko\'rsatiladi', (tester) async {
    final stats = _emptyStats()
      ..['current'] = _summary(orders: 3, cancelled: 1, inProgress: 2, inProgressTiyin: 14400000)
      ..['busiest_hours'] = {'start_hour': 0, 'end_hour': 2, 'orders': 2}
      ..['busiest_weekday'] = {'weekday': 2, 'avg_orders': 0.3, 'orders': 1};
    await _run(
      tester,
      size: const Size(1672, 1200),
      respond: (_) => _json(stats),
      body: (_) async {
        expect(tester.takeException(), isNull);
        // Jami tushum va o'rtacha buyurtma.
        expect(find.text('0 so\'m'), findsNWidgets(2));
        expect(find.text('+144 000 so\'m jarayonda (2 ta)'), findsOneWidget);
        expect(find.text('Yakunlangan tushum yo\'q · 144 000 so\'m jarayonda'), findsOneWidget);
        expect(find.text('Yakunlangan sotuv yo\'q · 2 ta buyurtma jarayonda'), findsNWidgets(2));
        expect(find.text('Bu davrda sotilgan mahsulot yo\'q'), findsNothing);
      },
    );
  });

  testWidgets('server xatosi matni ko\'rsatiladi, qayta urinish ishlaydi', (tester) async {
    var calls = 0;
    await _run(
      tester,
      size: const Size(1300, 2000),
      respond: (_) {
        calls++;
        return calls == 1
            ? _json({'error': 'tanlangan davrda buyurtmalar juda ko\'p — qisqaroq davr tanlang'}, 422)
            : _json(_fullStats());
      },
      body: (_) async {
        expect(find.textContaining('juda ko\'p'), findsOneWidget);
        await tester.tap(find.text('Qayta urinish'));
        await tester.pumpAndSettle();
        expect(calls, 2);
        expect(find.text('Jami tushum'), findsOneWidget);
        expect(tester.takeException(), isNull);
      },
    );
  });

  testWidgets('so\'rov parametrlari to\'g\'ri, o\'lcham almashtirilsa qayta so\'raladi',
      (tester) async {
    final urls = <Uri>[];
    await _run(
      tester,
      size: const Size(1672, 1200),
      respond: (request) {
        urls.add(request.url);
        return _json(_fullStats());
      },
      body: (_) async {
        expect(urls, hasLength(1));
        final first = urls.single;
        expect(first.path, '/restaurants/rest-a/stats');
        expect(first.queryParameters['granularity'], 'day');
        final from = DateTime.parse(first.queryParameters['from']!);
        final to = DateTime.parse(first.queryParameters['to']!);
        // Standart davr — bugun bilan birga 30 kun.
        expect(to.difference(from).inDays, 29);

        await tester.tap(find.text('Kunlik'));
        await tester.pumpAndSettle();
        await tester.tap(find.text('Haftalik').last);
        await tester.pumpAndSettle();

        expect(urls, hasLength(2));
        expect(urls.last.queryParameters['granularity'], 'week');
        expect(tester.takeException(), isNull);
      },
    );
  });
}

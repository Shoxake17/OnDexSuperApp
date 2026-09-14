import 'dart:convert';

import 'package:chust_restaurant/api.dart';
import 'package:chust_restaurant/pages/tables_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:qr_flutter/qr_flutter.dart';

// "QR Stollar" sahifasi testlari: namuna bo'yicha ko'rinish (har xil
// kenglikda overflowsiz), turlar va holat filtrlari, yangi joy qo'shish
// so'rovi, server xatosi va tozalash belgisi.

const _titles = {'table': 'Stol', 'cabin': 'Kabina', 'tapchan': 'Topchan'};

Map<String, dynamic> _order(String number, int items, {int minutesAgo = 10}) => {
      'id': 'o$number',
      'order_number': '140926-$number',
      'status': 'preparing',
      'items': items,
      'total_tiyin': 4500000,
      'created_at': DateTime.now().toUtc().subtract(Duration(minutes: minutesAgo)).toIso8601String(),
    };

String _ago(int minutes) => DateTime.now().toUtc().subtract(Duration(minutes: minutes)).toIso8601String();

Map<String, dynamic> _table(
  String id,
  String label, {
  String kind = 'table',
  String zone = 'Asosiy zal',
  int? capacity = 4,
  String status = 'available',
  List<Map<String, dynamic>> active = const [],
  String? scanned,
  String? cleaning,
  bool isActive = true,
  Map<String, dynamic>? last,
}) =>
    {
      'id': id,
      'restaurant_id': 'rest-a',
      'zone': zone,
      'kind': kind,
      'kind_title': _titles[kind],
      'label': label,
      'display_label': kind == 'table' ? '$zone · $label' : '$zone · ${_titles[kind]} $label',
      'capacity': capacity,
      'active': isActive,
      'cleaning_since': cleaning,
      'last_scanned_at': scanned,
      'created_at': '2026-08-12T10:00:00Z',
      'qr_token': 'token-$id',
      'qr_link': 'https://t.me/ondex_bot/ondex?startapp=token-$id',
      'status': status,
      'active_orders': active,
      if (last != null) 'last_order': last,
    };

List<Map<String, dynamic>> _fixture() => [
      _table('t1', '1', status: 'occupied', active: [_order('1024', 3)], scanned: _ago(90)),
      _table('t2', '2', capacity: 2, status: 'occupied', active: [_order('1025', 2)]),
      _table('t3', '3', capacity: 6),
      _table('t4', '4', status: 'occupied', active: [_order('1026', 4)], scanned: _ago(5)),
      _table('t5', '5', capacity: 2, status: 'cleaning', cleaning: _ago(12)),
      _table('t6', '6', capacity: 8),
      _table('t7', '7'),
      _table('t8', '8', capacity: 6, status: 'occupied', active: [_order('1027', 5)], scanned: _ago(30)),
      _table('t9', '9', capacity: 2),
      _table('t10', '10'),
      _table('t11', '11', capacity: 6, status: 'occupied', active: [_order('1028', 3)], scanned: _ago(8)),
      _table('t12', '12',
          capacity: 8, status: 'cleaning', cleaning: _ago(3), scanned: _ago(40), last: _order('1020', 2, minutesAgo: 45)),
      _table('c1', '1', kind: 'cabin', zone: 'Ayvon', capacity: 6),
      _table('c2', '2', kind: 'cabin', zone: 'Ayvon', capacity: 6, status: 'occupied', active: [_order('1029', 7)]),
      _table('p1', 'Oltin', kind: 'tapchan', zone: 'Ayvon', capacity: null, status: 'inactive', isActive: false),
    ];

http.Response _json(Object body, [int status = 200]) => http.Response.bytes(
      utf8.encode(jsonEncode(body)),
      status,
      headers: {'content-type': 'application/json; charset=utf-8'},
    );

Future<void> _run(
  WidgetTester tester, {
  required Size size,
  http.Response? Function(http.Request request)? respond,
  required Future<void> Function(List<http.Request> requests) body,
}) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  api.rid = 'rest-a';
  final requests = <http.Request>[];

  await http.runWithClient(
    () async {
      await tester.pumpWidget(const MaterialApp(home: Scaffold(body: TablesPage(live: false))));
      await tester.pumpAndSettle();
      await body(requests);
      await tester.pumpWidget(const SizedBox());
    },
    () => MockClient((request) async {
      requests.add(request);
      final custom = respond?.call(request);
      if (custom != null) return custom;
      if (request.method == 'GET' && request.url.path == '/restaurants/rest-a/tables') {
        return _json(_fixture());
      }
      if (request.method == 'GET' && request.url.path == '/restaurants/rest-a') {
        return _json({'id': 'rest-a', 'name': 'Book Cafe'});
      }
      return _json({'error': 'kutilmagan so\'rov: ${request.method} ${request.url.path}'}, 500);
    }),
  );
}

Finder _inDialog(Finder f) => find.descendant(of: find.byType(Dialog), matching: f);

void main() {
  for (final width in [1672.0, 1250.0, 900.0, 420.0]) {
    testWidgets('$width px kenglikda xatosiz chiziladi', (tester) async {
      await _run(
        tester,
        size: Size(width, 1000),
        body: (_) async {
          expect(tester.takeException(), isNull);
          expect(find.text('QR Stollar'), findsOneWidget);
          expect(find.text('Yangi stol qo\'shish'), findsOneWidget);
          expect(find.text('Stollar holati'), findsOneWidget);
        },
      );
    });
  }

  testWidgets('namuna bo\'yicha: turlar, holatlar, QR panel va filtrlar', (tester) async {
    await _run(
      tester,
      size: const Size(1672, 1100),
      body: (requests) async {
        // Qidiruvdan keyin — restoranda BOR turlar, sonlari bilan.
        expect(find.text('Barchasi'), findsOneWidget);
        expect(find.text('Stol'), findsOneWidget);
        expect(find.text('Kabina'), findsOneWidget);
        expect(find.text('Topchan'), findsOneWidget);
        expect(find.text('VIP xona'), findsNothing);

        String stat(String key) => (tester.widget<Text>(find.descendant(
                of: find.byKey(ValueKey('stat-$key')), matching: find.byType(Text)).last))
            .data!;
        expect(stat('occupied'), '6');
        expect(stat('available'), '6');
        expect(stat('cleaning'), '2');
        expect(stat('total'), '15');
        expect(find.text('Vaqtincha yopilgan: 1 ta'), findsOneWidget);

        expect(find.text('Buyurtma #1024'), findsOneWidget);
        expect(find.byType(QrImageView), findsOneWidget);
        expect(find.text('O\'zgarmas QR'), findsOneWidget);
        expect(find.text('QR kodni yuklab olish'), findsOneWidget);
        expect(find.text('PNG rasm'), findsNothing);
        expect(find.text('Chop etish'), findsNothing);

        // Yuqorida bitta qator: sarlavha, qidiruv, filtr va "Yangi stol
        // qo'shish"; turlar ostida, chap chetdan (sahifa chekinishi 24 px).
        final rowY = tester.getCenter(find.byType(TextField).first).dy;
        expect((tester.getCenter(find.text('Yangi stol qo\'shish')).dy - rowY).abs(), lessThan(2));
        expect((tester.getCenter(find.byTooltip('Holat va zal bo\'yicha filtr')).dy - rowY).abs(), lessThan(2));
        expect((tester.getCenter(find.byIcon(Icons.table_restaurant_rounded).first).dy - rowY).abs(), lessThan(2));
        final chip = find.ancestor(of: find.text('Barchasi'), matching: find.byType(Material)).first;
        expect(tester.getTopLeft(chip).dx, closeTo(24, 0.5));
        expect(tester.getTopLeft(chip).dy, greaterThan(rowY + 20));
        // QR kartasi pastga tushib ketmaydi: turlar bilan bir balandlikdan.
        final qrCard = find.ancestor(of: find.text('QR kod yaratish'), matching: find.byType(Container)).first;
        expect((tester.getTopLeft(qrCard).dy - tester.getTopLeft(chip).dy).abs(), lessThan(1));

        // So'nggi skanerlanganlar — eng yangisi birinchi (4-stol, 5 daqiqa).
        final scanTitles = tester
            .widgetList<Text>(find.descendant(
                of: find.ancestor(of: find.text('So\'nggi skanerlangan QR kodlar'), matching: find.byType(Container)).first,
                matching: find.byType(Text)))
            .map((t) => t.data)
            .whereType<String>()
            .where((s) => s.startsWith('Stol ') && s.contains('·'))
            .toList();
        expect(scanTitles.first, 'Stol 4 · Asosiy zal');

        // QR hech qachon qayta yaratilmaydi.
        expect(requests.where((r) => r.url.path.contains('regenerate')), isEmpty);

        await tester.tap(find.text('Kabina'));
        await tester.pumpAndSettle();
        expect(find.byKey(const ValueKey('table-c1')), findsOneWidget);
        expect(find.byKey(const ValueKey('table-c2')), findsOneWidget);
        expect(find.byKey(const ValueKey('table-t1')), findsNothing);

        await tester.tap(find.text('Barchasi'));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const ValueKey('stat-cleaning')));
        await tester.pumpAndSettle();
        expect(find.byKey(const ValueKey('table-t5')), findsOneWidget);
        expect(find.byKey(const ValueKey('table-t12')), findsOneWidget);
        expect(find.byKey(const ValueKey('table-t3')), findsNothing);

        await tester.tap(find.byKey(const ValueKey('stat-total')));
        await tester.pumpAndSettle();
        await tester.enterText(find.byType(TextField).first, '11');
        await tester.pumpAndSettle();
        expect(find.byKey(const ValueKey('table-t11')), findsOneWidget);
        expect(find.byKey(const ValueKey('table-t1')), findsNothing);
        expect(tester.takeException(), isNull);
      },
    );
  });

  testWidgets('yangi joy: tur, zal, ketma-ket raqamlash va sig\'im so\'rovga to\'g\'ri tushadi',
      (tester) async {
    Map<String, dynamic>? sent;
    await _run(
      tester,
      size: const Size(1672, 1100),
      respond: (r) {
        if (r.method == 'POST' && r.url.path == '/restaurants/rest-a/tables/batch') {
          sent = jsonDecode(r.body) as Map<String, dynamic>;
          return _json([
            for (var i = 0; i < 3; i++) _table('n$i', '${5 + i}', kind: 'cabin', zone: 'Ayvon', capacity: 6),
          ], 201);
        }
        return null;
      },
      body: (requests) async {
        await tester.tap(find.text('Yangi stol qo\'shish'));
        await tester.pumpAndSettle();
        expect(find.text('Yangi joy qo\'shish'), findsOneWidget);
        expect(find.textContaining('Har bir joyga'), findsNothing);
        expect(tester.takeException(), isNull);

        await tester.tap(find.byKey(const ValueKey('kind-cabin')));
        await tester.pumpAndSettle();
        await tester.ensureVisible(_inDialog(find.text('Ayvon')));
        await tester.tap(_inDialog(find.text('Ayvon')));
        await tester.pumpAndSettle();
        await tester.ensureVisible(_inDialog(find.text('Bir nechta (ketma-ket)')));
        await tester.tap(_inDialog(find.text('Bir nechta (ketma-ket)')));
        await tester.pumpAndSettle();
        await tester.enterText(find.byKey(const ValueKey('batch-from')), '5');
        await tester.enterText(find.byKey(const ValueKey('batch-count')), '3');
        await tester.pumpAndSettle();
        expect(find.textContaining('Kabina 5, Kabina 6 … Kabina 7'), findsOneWidget);
        expect(find.text('3 ta joy qo\'shish'), findsOneWidget);

        await tester.tap(find.byKey(const ValueKey('editor-save')));
        await tester.pumpAndSettle();
        expect(sent, {'zone': 'Ayvon', 'kind': 'cabin', 'prefix': '', 'from': 5, 'count': 3, 'capacity': 6});
        expect(find.text('Yangi joy qo\'shish'), findsNothing);
        expect(tester.takeException(), isNull);
      },
    );
  });

  testWidgets('server rad etsa xato oynada ko\'rinadi va oyna yopilmaydi', (tester) async {
    await _run(
      tester,
      size: const Size(1672, 1100),
      respond: (r) => r.method == 'POST' && r.url.path == '/restaurants/rest-a/tables'
          ? _json({'error': 'bu nomli joy shu zal va turda allaqachon mavjud'}, 409)
          : null,
      body: (_) async {
        await tester.tap(find.text('Yangi stol qo\'shish'));
        await tester.pumpAndSettle();
        // Bo'sh nom — so'rov yuborilmaydi.
        await tester.tap(find.byKey(const ValueKey('editor-save')));
        await tester.pumpAndSettle();
        expect(find.text('Joy raqami yoki nomini kiriting'), findsOneWidget);

        await tester.enterText(find.byKey(const ValueKey('editor-label')), '3');
        await tester.tap(find.byKey(const ValueKey('editor-save')));
        await tester.pumpAndSettle();
        expect(find.textContaining('allaqachon mavjud'), findsOneWidget);
        expect(find.text('Yangi joy qo\'shish'), findsOneWidget);
      },
    );
  });

  testWidgets('batafsil: tozalanmoqda belgisi yuboriladi, band joy o\'chirilmaydi', (tester) async {
    Map<String, dynamic>? patched;
    await _run(
      tester,
      size: const Size(1672, 1100),
      respond: (r) {
        if (r.method == 'PATCH' && r.url.path == '/tables/t3') {
          patched = jsonDecode(r.body) as Map<String, dynamic>;
          return _json(_table('t3', '3', capacity: 6, cleaning: _ago(0)));
        }
        return null;
      },
      body: (requests) async {
        await tester.tap(find.descendant(
            of: find.byKey(const ValueKey('table-t3')), matching: find.byTooltip('Batafsil')));
        await tester.pumpAndSettle();
        expect(_inDialog(find.text('Asosiy zal · 3')), findsOneWidget);
        expect(_inDialog(find.text('QR kodni yuklab olish')), findsOneWidget);
        expect(_inDialog(find.text('Chop etish')), findsNothing);
        expect(_inDialog(find.textContaining('PNG')), findsNothing);
        await tester.tap(_inDialog(find.text('Tozalanmoqda deb belgilash')));
        await tester.pumpAndSettle();
        expect(patched, {'cleaning': true});

        await tester.tap(find.descendant(
            of: find.byKey(const ValueKey('table-t1')), matching: find.byTooltip('Batafsil')));
        await tester.pumpAndSettle();
        expect(_inDialog(find.text('#1024')), findsOneWidget);
        await tester.tap(_inDialog(find.text('O\'chirish')));
        await tester.pumpAndSettle();
        expect(find.textContaining('Band joyni o\'chirib bo\'lmaydi'), findsOneWidget);
        expect(requests.where((r) => r.method == 'DELETE'), isEmpty);
        expect(tester.takeException(), isNull);
      },
    );
  });
}

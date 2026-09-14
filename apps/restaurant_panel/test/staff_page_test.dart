import 'dart:convert';

import 'package:chust_restaurant/api.dart';
import 'package:chust_restaurant/pages/staff_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

// "Xodimlar" sahifasi: namuna bo'yicha ko'rinish (Eksport, sarlavha
// ikonkasi va "oxirgi hafta" qatorlarisiz), qat'iy o'lchamli jadval,
// qidiruv va filtrlar, qo'shish/tahrirlash so'rovlari (maosh bilan),
// ilovaga kirish faqat ofitsiantda, ishdan bo'shatish tasdig'i, ish
// jadvalida dam olish kunlari va oylik hisobot.

const _positions = [
  {'key': 'head_chef', 'title': 'Bosh oshpaz', 'group': 'kitchen', 'app_access': false},
  {'key': 'chef', 'title': 'Oshpaz', 'group': 'kitchen', 'app_access': false},
  {'key': 'cook_assistant', 'title': 'Oshpaz yordamchisi', 'group': 'kitchen', 'app_access': false},
  {'key': 'waiter', 'title': 'Ofitsiant', 'group': 'hall', 'app_access': true},
  {'key': 'cashier', 'title': 'Kassir', 'group': 'cashier', 'app_access': false},
  {'key': 'other', 'title': 'Boshqa', 'group': 'other', 'app_access': false},
];

String _titleOf(String key) => _positions.firstWhere((p) => p['key'] == key)['title'] as String;

Map<String, dynamic> _member(
  String id,
  int n,
  String first,
  String last,
  String position, {
  String status = 'active',
  bool access = false,
  int? salary,
  Map<String, dynamic>? schedule,
}) =>
    {
      'id': id,
      'number': n,
      'code': 'EMP${n.toString().padLeft(3, '0')}',
      'first_name': first,
      'last_name': last,
      'full_name': '$first $last',
      'phone': '+9989012345${n.toString().padLeft(2, '0')}',
      'position': position,
      'position_title': _titleOf(position),
      'status': status,
      'status_title': status,
      'schedule': schedule,
      'hired_on': '2026-09-01',
      'note': '',
      'monthly_salary_tiyin': salary,
      'app_access': access,
      'app_access_active': access && status == 'active',
      'created_at': '2026-09-01T10:00:00Z',
      'updated_at': '2026-09-10T10:00:00Z',
      'dismissed_at': null,
    };

Map<String, dynamic> _overview({List<Map<String, dynamic>>? items}) => {
      'items': items ??
          [
            _member('m1', 1, 'Azizbek', 'Karimov', 'head_chef',
                salary: 450000000, schedule: {'days': [1, 2, 3, 4, 5, 6, 7], 'start': '08:00', 'end': '22:00'}),
            _member('m2', 2, 'Malika', 'To\'xtayeva', 'waiter',
                access: true, schedule: {'days': [1, 2, 3, 4, 5, 6], 'start': '10:00', 'end': '23:00'}),
            _member('m3', 3, 'Sardor', 'Abdullayev', 'waiter', status: 'on_leave'),
            _member('m4', 4, 'Dilshoda', 'Raxmatova', 'cashier'),
            _member('m5', 5, 'Rustam', 'Jo\'rayev', 'cook_assistant', status: 'dismissed'),
          ],
      'summary': {
        'total': 5,
        'total_week_delta': 2,
        'active': 3,
        'active_week_delta': 1,
        'on_leave': 1,
        'dismissed': 1,
        'dismissed_week_delta': -1,
        'working_today': 2,
        'working_yesterday': 1,
        'by_position': {'head_chef': 1, 'waiter': 2, 'cashier': 1},
      },
      'recent_activity': [
        {
          'id': 'e2',
          'member_id': 'm2',
          'member_name': 'Malika To\'xtayeva',
          'kind': 'position_changed',
          'from': 'cashier',
          'to': 'waiter',
          'from_title': 'Kassir',
          'to_title': 'Ofitsiant',
          'at': DateTime.now().toUtc().toIso8601String(),
        },
      ],
      'positions': _positions,
      'statuses': [],
      'timezone': 'Asia/Tashkent',
      'max_members': 500,
    };

Map<String, dynamic> _report(String month) => {
      'month': month,
      'days_in_month': 30,
      'today': '2026-09-14',
      'timezone': 'Asia/Tashkent',
      'items': [
        {
          'id': 'm1',
          'code': 'EMP001',
          'full_name': 'Azizbek Karimov',
          'position': 'head_chef',
          'position_title': 'Bosh oshpaz',
          'status': 'active',
          'schedule': {'days': [1, 2, 3, 4, 5, 6, 7], 'start': '08:00', 'end': '22:00'},
          'monthly_salary_tiyin': 450000000,
          'accrued_salary_tiyin': 450000000,
          'norm_days': 30,
          'payable_days': 30,
          'worked_days': 13,
          'leave_days': 0,
          'off_days': 0,
          'worked_minutes': 13 * 840,
          'planned_minutes': 30 * 840,
          'scheduled': true,
          'days': 'W' * 13 + 'P' * 17,
        },
        {
          'id': 'm3',
          'code': 'EMP003',
          'full_name': 'Sardor Abdullayev',
          'position': 'waiter',
          'position_title': 'Ofitsiant',
          'status': 'on_leave',
          'schedule': null,
          'monthly_salary_tiyin': null,
          'accrued_salary_tiyin': null,
          'norm_days': 30,
          'payable_days': 10,
          'worked_days': 8,
          'leave_days': 20,
          'off_days': 0,
          'worked_minutes': 0,
          'planned_minutes': 0,
          'scheduled': false,
          'days': 'W' * 8 + 'L' * 20 + 'P' * 2,
        },
      ],
      'totals': {
        'members': 2,
        'salary_fund_tiyin': 450000000,
        'accrued_tiyin': 450000000,
        'worked_days': 21,
        'leave_days': 20,
        'worked_minutes': 10920,
        'planned_minutes': 25200,
      },
    };

http.Response _json(Object body, [int status = 200]) => http.Response.bytes(
      utf8.encode(jsonEncode(body)),
      status,
      headers: {'content-type': 'application/json; charset=utf-8'},
    );

Future<void> _run(
  WidgetTester tester, {
  Size size = const Size(1600, 1100),
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
      await tester.pumpWidget(const MaterialApp(home: Scaffold(body: StaffPage())));
      await tester.pumpAndSettle();
      await body(requests);
      await tester.pumpWidget(const SizedBox());
    },
    () => MockClient((request) async {
      requests.add(request);
      final custom = respond?.call(request);
      if (custom != null) return custom;
      final path = request.url.path;
      if (request.method == 'GET' && path == '/restaurants/rest-a/staff') return _json(_overview());
      if (request.method == 'GET' && path == '/restaurants/rest-a/staff/report') {
        return _json(_report(request.url.queryParameters['month'] ?? ''));
      }
      if (request.method == 'GET' && path.endsWith('/activity')) return _json({'items': []});
      if (request.method == 'POST' && path == '/restaurants/rest-a/staff') {
        return _json(_member('m9', 9, 'Yangi', '', 'waiter'), 201);
      }
      if (request.method == 'PATCH' || (request.method == 'POST' && path.endsWith('/status'))) {
        return _json(_member('m1', 1, 'Azizbek', 'Karimov', 'head_chef'));
      }
      return _json({'error': 'kutilmagan so\'rov'}, 404);
    }),
  );
}

List<http.Request> _writes(List<http.Request> r) => r.where((x) => x.method != 'GET').toList();

Future<void> _tap(WidgetTester tester, Finder f) async {
  await tester.ensureVisible(f);
  await tester.pumpAndSettle();
  await tester.tap(f);
  await tester.pumpAndSettle();
}

String _monthKey(DateTime d) => '${d.year}-${d.month.toString().padLeft(2, '0')}';

/// Yon tomonga aylanadigan ro'yxat/ko'rinish (bir qatorli matn maydonining
/// ichki aylantirishi hisobga olinmaydi — u sahifa tuzilishi emas).
Finder _horizontalScrollables() => find.byWidgetPredicate((w) =>
    (w is ScrollView && w.scrollDirection == Axis.horizontal) ||
    (w is SingleChildScrollView && w.scrollDirection == Axis.horizontal));

void main() {
  for (final size in const [Size(1600, 1100), Size(1180, 900), Size(820, 1000), Size(420, 1000)]) {
    testWidgets('namuna bo\'yicha ko\'rinish, overflowsiz — ${size.width.toInt()}px', (tester) async {
      await _run(tester, size: size, body: (_) async {
        expect(tester.takeException(), isNull);
        expect(find.text('Xodimlar'), findsOneWidget);
        expect(tester.getTopLeft(find.text('Xodimlar')).dx, closeTo(24, 0.5));
        expect(find.textContaining('Eksport'), findsNothing);
        // Kartalarda pastki "oxirgi hafta" qatori yo'q.
        expect(find.textContaining('oxirgi hafta'), findsNothing);
        expect(find.textContaining('kechaga'), findsNothing);
        for (final t in [
          'Jami xodimlar',
          'Faol xodimlar',
          'Bugungi ishchilar',
          'Ishdan bo\'shaganlar',
          'Lavozimlar bo\'yicha taqsimot',
          'Tezkor amallar',
          'So\'nggi faoliyat',
        ]) {
          expect(find.text(t), findsOneWidget, reason: t);
        }
        // Har kenglikda JADVAL; tor oynada kamroq muhim ustunlar yashiriladi.
        for (final h in [
          'Foydalanuvchi',
          'Holati',
          'Amallar',
          if (size.width >= 820) ...['Lavozimi', 'Telefon'],
          if (size.width >= 1600) 'Ish vaqti',
        ]) {
          expect(find.text(h), findsOneWidget, reason: h);
        }
        // Yon tomonga aylantirish umuman yo'q; "ko'rish" (ko'z) belgisi yo'q.
        expect(_horizontalScrollables(), findsNothing);
        expect(find.byIcon(Icons.visibility_outlined), findsNothing);
        // Tezkor amallar: faqat ish jadvali va hisobot.
        expect(find.byKey(const ValueKey('quick-schedule')), findsOneWidget);
        expect(find.byKey(const ValueKey('quick-report')), findsOneWidget);
        expect(find.byKey(const ValueKey('quick-add')), findsNothing);
        expect(find.byKey(const ValueKey('quick-salary')), findsNothing);
        expect(find.byKey(const ValueKey('row-m1')), findsOneWidget);
        // Asosiy tugma — brend rangidagi to'ldirilgan tugma.
        expect(tester.widget(find.byKey(const ValueKey('staff-add'))), isA<FilledButton>());
      });
    });
  }

  testWidgets('keng oynada o\'ng ustun jadval yonida (1000 px dan)', (tester) async {
    await _run(tester, size: const Size(1100, 900), body: (_) async {
      final table = tester.getRect(find.byKey(const ValueKey('staff-table')));
      final side = tester.getRect(find.text('Tezkor amallar'));
      expect(side.left, greaterThan(table.right));
      expect(side.top, lessThan(table.bottom));
    });
  });

  testWidgets('jadval qat\'iy o\'lchamda: xodimlar ko\'paysa kattalashmaydi', (tester) async {
    late double few;
    await _run(tester, body: (_) async {
      few = tester.getSize(find.byKey(const ValueKey('staff-table'))).height;
    });
    final many = _overview(items: [for (var i = 1; i <= 60; i++) _member('x$i', i, 'Xodim', 'Nomer', 'chef')]);
    await _run(
      tester,
      respond: (r) => r.method == 'GET' && r.url.path == '/restaurants/rest-a/staff' ? _json(many) : null,
      body: (_) async {
        expect(tester.takeException(), isNull);
        expect(tester.getSize(find.byKey(const ValueKey('staff-table'))).height, few);
        expect(find.text('Jami 60 ta xodim'), findsOneWidget);
        // Qatorlar jadval ichida aylanadi.
        await tester.drag(find.byKey(const ValueKey('staff-rows')), const Offset(0, -3000));
        await tester.pumpAndSettle();
        expect(find.byKey(const ValueKey('row-x60')), findsOneWidget);
      },
    );
  });

  testWidgets('qidiruv va holat filtri', (tester) async {
    await _run(tester, body: (_) async {
      await tester.enterText(find.byKey(const ValueKey('staff-search')), 'Malika');
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('row-m2')), findsOneWidget);
      expect(find.byKey(const ValueKey('row-m1')), findsNothing);

      await tester.enterText(find.byKey(const ValueKey('staff-search')), '');
      await tester.pumpAndSettle();
      await _tap(tester, find.byKey(const ValueKey('filter-status')));
      await tester.tap(find.text('Ta\'tilda').last);
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('row-m3')), findsOneWidget);
      expect(find.byKey(const ValueKey('row-m1')), findsNothing);
      expect(find.text('5 ta xodimdan 1 tasi ko\'rsatilmoqda'), findsOneWidget);
    });
  });

  testWidgets('yangi ofitsiant: maosh va ilovaga kirish bilan to\'g\'ri so\'rov', (tester) async {
    await _run(tester, body: (requests) async {
      await _tap(tester, find.byKey(const ValueKey('staff-add')));
      expect(find.text('Yangi xodim qo\'shish'), findsOneWidget);
      await tester.enterText(find.byKey(const ValueKey('form-first')), 'Jasurbek');
      await tester.enterText(find.byKey(const ValueKey('form-last')), 'Yo\'ldoshev');
      await tester.enterText(find.byKey(const ValueKey('form-phone')), '901234567');
      await tester.enterText(find.byKey(const ValueKey('form-salary')), '3500000');
      await tester.pumpAndSettle();
      expect(tester.widget<TextField>(find.byKey(const ValueKey('form-phone'))).controller!.text, '90 123 45 67');
      expect(tester.widget<TextField>(find.byKey(const ValueKey('form-salary'))).controller!.text, '3 500 000');

      expect(find.byKey(const ValueKey('form-app-access')), findsNothing);
      await _tap(tester, find.byKey(const ValueKey('form-position')));
      await tester.tap(find.text('Ofitsiant').last);
      await tester.pumpAndSettle();
      await _tap(tester, find.byKey(const ValueKey('form-app-access')));
      await _tap(tester, find.byKey(const ValueKey('form-save')));

      final post = _writes(requests).single;
      expect(post.method, 'POST');
      expect(jsonDecode(post.body), {
        'first_name': 'Jasurbek',
        'last_name': 'Yo\'ldoshev',
        'phone': '+998901234567',
        'position': 'waiter',
        'schedule': {
          'days': [1, 2, 3, 4, 5, 6],
          'start': '09:00',
          'end': '18:00',
        },
        'hired_on': null,
        'note': '',
        'app_access': true,
        'monthly_salary_tiyin': 350000000,
      });
      expect(find.text('Xodim qo\'shildi'), findsOneWidget);
      expect(requests.where((r) => r.method == 'GET' && r.url.path == '/restaurants/rest-a/staff'), hasLength(2));
    });
  });

  testWidgets('maosh ixtiyoriy: bo\'sh bo\'lsa null, ilovasiz lavozimda kirish yo\'q', (tester) async {
    await _run(tester, body: (requests) async {
      await _tap(tester, find.byKey(const ValueKey('staff-add')));
      await tester.enterText(find.byKey(const ValueKey('form-first')), 'Ali');
      await tester.enterText(find.byKey(const ValueKey('form-phone')), '901112233');
      await _tap(tester, find.byKey(const ValueKey('form-position')));
      await tester.tap(find.text('Ofitsiant').last);
      await tester.pumpAndSettle();
      await _tap(tester, find.byKey(const ValueKey('form-app-access')));
      await _tap(tester, find.byKey(const ValueKey('form-position')));
      await tester.tap(find.text('Kassir').last);
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('form-app-access')), findsNothing);
      await _tap(tester, find.byKey(const ValueKey('form-save')));
      final body = jsonDecode(_writes(requests).single.body) as Map<String, dynamic>;
      expect(body['position'], 'cashier');
      expect(body['app_access'], isFalse);
      expect(body.containsKey('monthly_salary_tiyin'), isTrue);
      expect(body['monthly_salary_tiyin'], isNull);
    });
  });

  testWidgets('tekshiruv: bo\'sh forma va noto\'g\'ri maosh yuborilmaydi', (tester) async {
    await _run(tester, body: (requests) async {
      await _tap(tester, find.byKey(const ValueKey('staff-add')));
      await _tap(tester, find.byKey(const ValueKey('form-save')));
      expect(find.text('Xodimning ismini kiriting'), findsOneWidget);
      await tester.enterText(find.byKey(const ValueKey('form-first')), 'Ali');
      await tester.enterText(find.byKey(const ValueKey('form-phone')), '90123');
      await _tap(tester, find.byKey(const ValueKey('form-save')));
      expect(find.text('Telefon raqamini to\'liq kiriting (9 ta raqam)'), findsOneWidget);
      await tester.enterText(find.byKey(const ValueKey('form-phone')), '901112233');
      await _tap(tester, find.byKey(const ValueKey('form-position')));
      await tester.tap(find.text('Oshpaz').last);
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(const ValueKey('form-salary')), '0');
      await _tap(tester, find.byKey(const ValueKey('form-save')));
      expect(find.text('Oylik maosh 0 dan katta bo\'lsin yoki bo\'sh qoldiring'), findsOneWidget);
      expect(_writes(requests), isEmpty);
    });
  });

  testWidgets('server xatosi oynada ko\'rsatiladi', (tester) async {
    await _run(
      tester,
      respond: (r) => r.method == 'POST'
          ? _json({'error': 'bu telefon raqam boshqa OnDex akkauntiga biriktirilgan'}, 409)
          : null,
      body: (requests) async {
        await _tap(tester, find.byKey(const ValueKey('staff-add')));
        await tester.enterText(find.byKey(const ValueKey('form-first')), 'Ali');
        await tester.enterText(find.byKey(const ValueKey('form-phone')), '901112233');
        await _tap(tester, find.byKey(const ValueKey('form-position')));
        await tester.tap(find.text('Oshpaz').last);
        await tester.pumpAndSettle();
        await _tap(tester, find.byKey(const ValueKey('form-save')));
        expect(find.text('bu telefon raqam boshqa OnDex akkauntiga biriktirilgan'), findsOneWidget);
        expect(find.text('Yangi xodim qo\'shish'), findsOneWidget);
      },
    );
  });

  testWidgets('tahrirlash: faqat o\'zgargan maydonlar yuboriladi', (tester) async {
    await _run(tester, body: (requests) async {
      await _tap(tester, find.byKey(const ValueKey('edit-m1')));
      expect(find.text('Xodimni tahrirlash'), findsOneWidget);
      expect(tester.widget<TextField>(find.byKey(const ValueKey('form-salary'))).controller!.text, '4 500 000');
      await tester.enterText(find.byKey(const ValueKey('form-note')), 'Kechki smena');
      await tester.enterText(find.byKey(const ValueKey('form-salary')), '');
      await _tap(tester, find.byKey(const ValueKey('form-save')));
      final patch = _writes(requests).single;
      expect(patch.method, 'PATCH');
      expect(patch.url.path, '/restaurants/rest-a/staff/m1');
      expect(jsonDecode(patch.body), {'note': 'Kechki smena', 'monthly_salary_tiyin': null});
    });
  });

  testWidgets('ishdan bo\'shatish tasdiq bilan', (tester) async {
    await _run(tester, body: (requests) async {
      await _tap(tester, find.byKey(const ValueKey('more-m4')));
      await tester.tap(find.text('Ishdan bo\'shatish').last);
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('confirm-status')), findsOneWidget);
      expect(_writes(requests), isEmpty);
      await _tap(tester, find.byKey(const ValueKey('confirm-status')));
      final req = _writes(requests).single;
      expect(req.url.path, '/restaurants/rest-a/staff/m4/status');
      expect(jsonDecode(req.body), {'status': 'dismissed'});
    });
  });

  testWidgets('ish jadvali: jadvalda yo\'q kun — dam olish', (tester) async {
    await _run(tester, body: (_) async {
      await _tap(tester, find.byKey(const ValueKey('quick-schedule')));
      expect(find.text('Haftalik ish jadvali'), findsOneWidget);
      expect(_horizontalScrollables(), findsNothing);
      // Malika Du–Sh ishlaydi: yakshanba — dam olish.
      expect(find.text('Dam olish'), findsOneWidget);
      expect(find.text('Jadvali belgilanmagan: 1 ta xodim (tahrirlash oynasida belgilanadi).'), findsOneWidget);
    });
  });

  testWidgets('hisobot: maosh, soat va kunlar; oyni almashtirish', (tester) async {
    await _run(tester, body: (requests) async {
      await _tap(tester, find.byKey(const ValueKey('quick-report')));
      expect(tester.takeException(), isNull);
      expect(find.text('Xodimlar hisoboti'), findsOneWidget);
      expect(_horizontalScrollables(), findsNothing);
      final now = DateTime.now();
      final first = requests.lastWhere((r) => r.url.path == '/restaurants/rest-a/staff/report');
      expect(first.url.queryParameters['month'], _monthKey(now));
      expect(find.byKey(const ValueKey('report-row-m1')), findsOneWidget);
      expect(find.text('4 500 000 so\'m'), findsWidgets);
      final row = find.byKey(const ValueKey('report-row-m1'));
      expect(find.descendant(of: row, matching: find.text('13 / 30')), findsOneWidget);
      expect(find.descendant(of: row, matching: find.text('182 soat')), findsOneWidget);
      expect(find.descendant(of: row, matching: find.text('/ 420 soat')), findsOneWidget);
      // Yuqoridagi jami ham xuddi shu soat (bitta xodimda soat bor).
      expect(find.text('182 soat'), findsNWidgets(2));
      expect(find.text('20 kun'), findsOneWidget);
      expect(find.text('Kiritilmagan'), findsOneWidget);
      expect(find.textContaining('davomat (keldi-ketdi) alohida qayd etilmaydi'), findsOneWidget);

      await _tap(tester, find.byKey(const ValueKey('report-prev')));
      final prev = requests.lastWhere((r) => r.url.path == '/restaurants/rest-a/staff/report');
      expect(prev.url.queryParameters['month'], _monthKey(DateTime(now.year, now.month - 1)));
    });
  });

  testWidgets('hisobot tor oynada: yon aylantirishsiz, kunlar tasmasi qator ostida', (tester) async {
    await _run(tester, size: const Size(820, 900), body: (_) async {
      await _tap(tester, find.byKey(const ValueKey('quick-report')));
      expect(tester.takeException(), isNull);
      expect(_horizontalScrollables(), findsNothing);
      expect(find.byKey(const ValueKey('report-row-m1')), findsOneWidget);
      // Lavozim ism ostiga ko'chgan.
      expect(find.textContaining('#EMP001 · Bosh oshpaz'), findsOneWidget);
    });
  });

  testWidgets('xodim kartochkasi: maosh, dam olish kunlari va faoliyat', (tester) async {
    await _run(tester, body: (requests) async {
      // Alohida "ko'rish" belgisi yo'q — qatorni bosish kartochkani ochadi.
      await _tap(tester, find.byKey(const ValueKey('row-m2')));
      expect(find.text('Xodim ma\'lumotlari'), findsOneWidget);
      expect(find.text('Ochiq — OnDex Affitsiant'), findsOneWidget);
      expect(find.text('Yakshanba'), findsOneWidget);
      expect(find.text('Kiritilmagan'), findsWidgets);
      expect(requests.where((r) => r.url.path == '/restaurants/rest-a/staff/m2/activity'), hasLength(1));
    });
  });
}

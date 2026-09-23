import 'dart:convert';

import 'package:chust_admin/ondexmap/moderation_api.dart';
import 'package:chust_admin/ondexmap/moderation_view.dart';
import 'package:chust_admin/ondexmap/ondexmap_session.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

// OnDexMap moderatsiyasi: taklifni ko'rish, tuzatib TASDIQLASH, RAD ETISH,
// xaritadan OLIB TASHLASH. Foydalanuvchi matni faqat matn sifatida ko'rinadi.

const _id = '11111111-1111-4111-8111-111111111111';
const _id2 = '22222222-2222-4222-8222-222222222222';

http.Response _json(Object body, [int status = 200]) => http.Response.bytes(
      utf8.encode(jsonEncode(body)),
      status,
      headers: {'content-type': 'application/json; charset=utf-8'},
    );

final _meta = {
  'kinds': [
    {
      'key': 'organization',
      'label': 'Tashkilot',
      'allowed': [
        'name',
        'category',
        'phone',
        'hours',
        'street',
        'house',
        'description'
      ]
    },
    {
      'key': 'other',
      'label': 'Boshqa ob\'ekt',
      'allowed': ['description']
    },
  ],
  'categories': ['Kafe', 'Restoran'],
  'max_photos': 4,
};

Map<String, dynamic> _org(
        {String id = _id, int photos = 0, String name = 'Chust Non'}) =>
    {
      'id': id,
      'kind': 'organization',
      'kind_label': 'Tashkilot',
      'name': name,
      'category': 'Kafe',
      'description': '',
      'phone': '+998 90 123-45-67',
      'hours': '09:00–18:00',
      'street': '',
      'house': '',
      'lat': 41.0,
      'lng': 71.2,
      'photos': photos,
      'created_at': '2026-09-21T08:00:00Z',
      'status': 'pending',
      'hint': 'abcdef0123456789',
    };

/// Stend: MockClient marshrutlari va so'rovlar jurnali.
class _Server {
  final requests = <http.Request>[];
  List<Map<String, dynamic>> pending;
  List<Map<String, dynamic>> places;
  List<Map<String, dynamic>> rejected;
  http.Response? approveResponse;

  _Server(
      {this.pending = const [],
      this.places = const [],
      this.rejected = const []});

  http.Response handle(http.Request r) {
    requests.add(r);
    // ⚠️ `endsWith`, `==` EMAS: masofaviy rejimda `defaultUrl` o'z yo'liga
    // ega bo'lishi mumkin (`/admin`), so'rov haqiqatan `/admin/api/...`ga
    // ketadi. Aniq tenglik bu holatni ushlamay, sinovni jimgina noto'g'ri
    // "muvaffaqiyat"ga aylantirardi (aynan shu bug production'da chiqqan
    // edi, 2026-09-23).
    final p = r.url.path;
    if (p.endsWith('/api/places/meta')) return _json(_meta);
    if (p.endsWith('/api/submissions') && r.method == 'GET') {
      final st = r.url.queryParameters['status'];
      return _json({
        'pending': pending.length,
        'submissions': st == 'rejected' ? rejected : pending
      });
    }
    if (p.endsWith('/api/places') && r.method == 'GET') {
      return _json({'places': places});
    }
    if (p.contains('/api/submissions/') && p.contains('/photos/')) {
      return http.Response.bytes(_png, 200);
    }
    if (p.endsWith('/api/submissions/approve')) {
      final res = approveResponse ?? _json({'place_id': 'pl-1'});
      if (res.statusCode == 200) pending = [];
      return res;
    }
    if (p.endsWith('/api/submissions/reject')) {
      pending = [];
      return _json({'ok': true});
    }
    if (p.endsWith('/api/places/delete')) {
      places = [];
      return _json({'ok': true});
    }
    return _json({'error': 'kutilmagan so\'rov'}, 404);
  }

  http.Request last(String path) =>
      requests.lastWhere((r) => r.url.path == path);
}

/// `_Server` ustidan o'tuvchi qatlam: `/api/places/meta` ga boshqa javob beradi.
class _MetaServer {
  final _Server inner;
  final Map<String, dynamic> meta;
  _MetaServer(this.inner, this.meta);

  http.Response handle(http.Request r) {
    if (r.url.path == '/api/places/meta') {
      inner.requests.add(r);
      return _json(meta);
    }
    return inner.handle(r);
  }
}

// 1×1 shaffof PNG.
final _png = base64Decode(
    'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==');

Future<void> _pump(
  WidgetTester tester,
  _Server server, {
  OndexMapSession? Function()? session,
  ValueChanged<int>? onPending,
}) async {
  tester.view.physicalSize = const Size(1400, 2600);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  final api = ModerationApi(
    defaultUrl: 'http://127.0.0.1:8091',
    session: session ?? () => const OndexMapSession(token: 'tok'),
  );
  await http.runWithClient(() async {
    await tester.pumpWidget(MaterialApp(
      theme: ThemeData(
          colorScheme: ColorScheme.fromSeed(seedColor: const Color(0xFF0B5D1E)),
          useMaterial3: true),
      home: Scaffold(body: OndexMapModeration(api: api, onPending: onPending)),
    ));
    await tester.pumpAndSettle();
  }, () => MockClient((r) async => server.handle(r)));
}

Future<void> _pumpMeta(WidgetTester tester, _MetaServer server) async {
  tester.view.physicalSize = const Size(1400, 2600);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  final api = ModerationApi(
    defaultUrl: 'http://127.0.0.1:8091',
    session: () => const OndexMapSession(token: 'tok'),
  );
  await http.runWithClient(() async {
    await tester.pumpWidget(MaterialApp(
      theme: ThemeData(
          colorScheme: ColorScheme.fromSeed(seedColor: const Color(0xFF0B5D1E)),
          useMaterial3: true),
      home: Scaffold(body: OndexMapModeration(api: api)),
    ));
    await tester.pumpAndSettle();
  }, () => MockClient((r) async => server.handle(r)));
}

/// `runWithClient` faqat ichidagi kod uchun amal qiladi: sinov davomidagi
/// so'rovlar ham shu klientdan o'tishi uchun har amal shu o'rovda bajariladi.
Future<void> _act(_Server server, Future<void> Function() body) =>
    http.runWithClient(body, () => MockClient((r) async => server.handle(r)));

Map<String, dynamic> _body(http.Request r) =>
    jsonDecode(r.body) as Map<String, dynamic>;

void main() {
  testWidgets('taklif kartochkasi: tur, vaqt, maydonlar va badge',
      (tester) async {
    final server = _Server(pending: [_org()]);
    int? badge;
    await _pump(tester, server, onPending: (n) => badge = n);

    expect(find.text('Tashkilot'), findsWidgets);
    expect(find.widgetWithText(TextField, 'Chust Non'), findsOneWidget);
    expect(find.text('Kutilmoqda (1)'), findsOneWidget);
    expect(badge, 1);
    expect(tester.takeException(), isNull);
    // Token sarlavhada, URL'da emas.
    for (final r in server.requests) {
      expect(r.headers['X-API-Key'], 'tok');
      expect(r.url.toString(), isNot(contains('tok')));
    }
  });

  testWidgets(
      'tuzatib TASDIQLASH: tahrirlangan nom ketadi, ro\'yxat yangilanadi',
      (tester) async {
    final server = _Server(pending: [_org()]);
    await _pump(tester, server);

    await tester.enterText(
        find.byKey(const ValueKey('mod-name-$_id')), 'Chust Non (tuzatildi)');
    await _act(server, () async {
      await tester.tap(find.byKey(const ValueKey('mod-approve-$_id')));
      await tester.pumpAndSettle();
    });

    final b = _body(server.last('/api/submissions/approve'));
    expect(b['id'], _id);
    expect(b['source'], 'community');
    final edit = b['edit'] as Map<String, dynamic>;
    expect(edit['name'], 'Chust Non (tuzatildi)');
    expect(edit['kind'], 'organization');
    expect(edit['category'], 'Kafe');
    expect(edit['lat'], 41.0);
    expect(edit['lng'], 71.2);
    expect(b['keep_photos'], isEmpty);
    // Tasdiqlangach navbat qayta o'qildi va bo'sh.
    expect(find.byKey(const ValueKey('mod-empty')), findsOneWidget);
  });

  testWidgets(
      'kontaktlar: sayt va ijtimoiy tarmoq ko\'rinadi, tahrirlanadi va serverga ketadi',
      (tester) async {
    // Server tashkilotda `site` va `social` maydonlarini beradi (meta allowed).
    final meta = jsonDecode(jsonEncode(_meta)) as Map<String, dynamic>;
    (meta['kinds'] as List).first['allowed'] = [
      'name',
      'category',
      'phone',
      'site',
      'social',
      'hours',
      'street',
      'house',
      'description',
    ];
    final server = _Server(pending: [
      _org()
        ..['site'] = 'https://chustnon.uz/menyu'
        ..['social'] = 'https://instagram.com/chustnon',
    ]);
    final metaServer = _MetaServer(server, meta);
    await _pumpMeta(tester, metaServer);

    expect(find.widgetWithText(TextField, 'https://chustnon.uz/menyu'),
        findsOneWidget);
    expect(find.widgetWithText(TextField, 'https://instagram.com/chustnon'),
        findsOneWidget);

    await tester.enterText(
        find.byKey(const ValueKey('mod-site-$_id')), 'https://chust-non.uz');
    await http.runWithClient(() async {
      await tester.tap(find.byKey(const ValueKey('mod-approve-$_id')));
      await tester.pumpAndSettle();
    }, () => MockClient((r) async => metaServer.handle(r)));

    final edit = _body(server.last('/api/submissions/approve'))['edit']
        as Map<String, dynamic>;
    expect(edit['site'], 'https://chust-non.uz');
    expect(edit['social'], 'https://instagram.com/chustnon');
    expect(edit['phone'], '+998 90 123-45-67');
  });

  testWidgets('tur o\'zgartirilsa ruxsat etilmagan maydonlar YUBORILMAYDI',
      (tester) async {
    final server = _Server(pending: [_org()]);
    await _pump(tester, server);

    await tester.tap(find.byKey(const ValueKey('mod-kind-$_id')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Boshqa ob\'ekt').last);
    await tester.pumpAndSettle();

    // «Boshqa ob'ekt»da nom/telefon maydonlari yo'q.
    expect(find.byKey(const ValueKey('mod-name-$_id')), findsNothing);
    expect(find.byKey(const ValueKey('mod-phone-$_id')), findsNothing);

    await tester.enterText(
        find.byKey(const ValueKey('mod-description-$_id')), 'Ko\'l');
    await _act(server, () async {
      await tester.tap(find.byKey(const ValueKey('mod-approve-$_id')));
      await tester.pumpAndSettle();
    });
    final edit = _body(server.last('/api/submissions/approve'))['edit']
        as Map<String, dynamic>;
    expect(edit.keys.toSet(), {'kind', 'lat', 'lng', 'description'},
        reason:
            'server ruxsat etilmagan maydonni rad etadi — u yuborilmasligi kerak');
    expect(edit['kind'], 'other');
    expect(edit['description'], 'Ko\'l');
  });

  testWidgets('noto\'g\'ri koordinata so\'rovsiz rad etiladi', (tester) async {
    final server = _Server(pending: [_org()]);
    await _pump(tester, server);

    await tester.enterText(find.byKey(const ValueKey('mod-lat-$_id')), 'abc');
    await tester.tap(find.byKey(const ValueKey('mod-approve-$_id')));
    await tester.pumpAndSettle();

    expect(find.text('Koordinata noto\'g\'ri'), findsOneWidget);
    expect(
        server.requests.where((r) => r.url.path == '/api/submissions/approve'),
        isEmpty);
  });

  testWidgets('vergulli koordinata (41,5) qabul qilinadi', (tester) async {
    final server = _Server(pending: [_org()]);
    await _pump(tester, server);
    await tester.enterText(find.byKey(const ValueKey('mod-lat-$_id')), '41,5');
    await _act(server, () async {
      await tester.tap(find.byKey(const ValueKey('mod-approve-$_id')));
      await tester.pumpAndSettle();
    });
    expect(
        (_body(server.last('/api/submissions/approve'))['edit'] as Map)['lat'],
        41.5);
  });

  testWidgets(
      'server rad etsa: xabar ko\'rsatiladi, kartochka QOLADI va qayta bosish mumkin',
      (tester) async {
    final server = _Server(pending: [_org()])
      ..approveResponse = _json({'error': '«telefon» noto\'g\'ri'}, 400);
    await _pump(tester, server);

    await _act(server, () async {
      await tester.tap(find.byKey(const ValueKey('mod-approve-$_id')));
      await tester.pumpAndSettle();
    });
    expect(find.text('«telefon» noto\'g\'ri'), findsOneWidget);
    expect(find.byKey(const ValueKey('mod-card-$_id')), findsOneWidget);
    final btn = tester.widget<ButtonStyleButton>(
        find.byKey(const ValueKey('mod-approve-$_id')));
    expect(btn.enabled, isTrue, reason: 'xatodan keyin qayta urinish mumkin');
  });

  testWidgets('RAD ETISH: sabab serverga ketadi', (tester) async {
    final server = _Server(pending: [_org()]);
    await _pump(tester, server);

    await tester.enterText(find.byKey(const ValueKey('mod-note-$_id')), 'spam');
    await _act(server, () async {
      await tester.tap(find.byKey(const ValueKey('mod-reject-$_id')));
      await tester.pumpAndSettle();
    });
    final b = _body(server.last('/api/submissions/reject'));
    expect(b, {'id': _id, 'note': 'spam'});
    expect(find.byKey(const ValueKey('mod-empty')), findsOneWidget);
  });

  testWidgets('rasm: qoldirish belgisi olib tashlansa keep_photos\'ga kirmaydi',
      (tester) async {
    final server = _Server(pending: [_org(photos: 2)]);
    await _pump(tester, server);

    // Birinchi rasmni olib tashlaymiz (ikkinchisi qoladi).
    await tester.tap(find.byType(Checkbox).first);
    await tester.pump();
    await _act(server, () async {
      await tester.tap(find.byKey(const ValueKey('mod-approve-$_id')));
      await tester.pumpAndSettle();
    });
    expect(_body(server.last('/api/submissions/approve'))['keep_photos'], [1]);
    // Rasmlar sarlavha bilan olingan.
    final photoReqs =
        server.requests.where((r) => r.url.path.contains('/photos/')).toList();
    expect(photoReqs, hasLength(2));
    expect(photoReqs.every((r) => r.headers['X-API-Key'] == 'tok'), isTrue);
  });

  testWidgets(
      'XSS: <img onerror> oddiy matn sifatida ko\'rinadi, hech narsa bajarilmaydi',
      (tester) async {
    const evil = '<img src=x onerror="alert(1)"><script>alert(2)</script>';
    final server = _Server(pending: [
      {..._org(name: evil), 'description': evil},
    ]);
    await _pump(tester, server);

    expect(find.widgetWithText(TextField, evil), findsWidgets);
    expect(tester.takeException(), isNull);
  });

  testWidgets('xaritadagi ob\'ektni OLIB TASHLASH: tasdiqlash oynasi bilan',
      (tester) async {
    final server = _Server(places: [_org(id: _id2, name: 'Vandal')]);
    await _pump(tester, server);

    // Tab almashtirish yangi so'rov yuboradi — u ham stend klientidan o'tishi kerak.
    await _act(server, () async {
      await tester.tap(find.text('Xaritadagi ob\'ektlar'));
      await tester.pumpAndSettle();
    });
    expect(find.text('Vandal'), findsOneWidget);

    // Bekor qilinsa hech narsa yuborilmaydi.
    await tester.tap(find.byKey(const ValueKey('mod-delete-$_id2')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Bekor'));
    await tester.pumpAndSettle();
    expect(server.requests.where((r) => r.url.path == '/api/places/delete'),
        isEmpty);

    await _act(server, () async {
      await tester.tap(find.byKey(const ValueKey('mod-delete-$_id2')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('mod-delete-confirm')));
      await tester.pumpAndSettle();
    });
    expect(_body(server.last('/api/places/delete')), {'id': _id2});
    expect(find.byKey(const ValueKey('mod-empty')), findsOneWidget);
  });

  testWidgets('rad etilganlar: sabab ko\'rinadi', (tester) async {
    final server = _Server(rejected: [
      {
        ..._org(name: 'Spam do\'kon'),
        'status': 'rejected',
        'review_note': 'reklama'
      },
    ]);
    await _pump(tester, server);
    await _act(server, () async {
      await tester.tap(find.text('Rad etilganlar'));
      await tester.pumpAndSettle();
    });
    expect(find.text('Spam do\'kon'), findsOneWidget);
    expect(find.textContaining('Sabab: reklama'), findsOneWidget);
  });

  testWidgets('bo\'sh navbat: tushunarli matn', (tester) async {
    await _pump(tester, _Server());
    expect(find.text('Tekshiruvni kutayotgan ob\'ekt yo\'q'), findsOneWidget);
  });

  testWidgets('sessiya yo\'q: sabab va qayta urinish tugmasi', (tester) async {
    final server = _Server();
    await _pump(tester, server, session: () => null);
    expect(find.byKey(const ValueKey('mod-error')), findsOneWidget);
    expect(find.textContaining('sessiya'), findsWidgets);
    expect(find.text('Qayta urinish'), findsOneWidget);
    expect(server.requests, isEmpty, reason: 'sessiyasiz so\'rov yuborilmaydi');
  });

  testWidgets('401: «sessiya qabul qilinmadi» ko\'rsatiladi', (tester) async {
    await tester.binding.setSurfaceSize(const Size(1400, 900));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await http.runWithClient(() async {
      await tester.pumpWidget(MaterialApp(
        home: Scaffold(
          body: OndexMapModeration(
            api: ModerationApi(
              defaultUrl: 'http://127.0.0.1:8091',
              session: () => const OndexMapSession(token: 'eski'),
            ),
          ),
        ),
      ));
      await tester.pumpAndSettle();
    },
        () => MockClient(
            (r) async => _json({'error': 'admin kaliti yaroqsiz'}, 401)));
    expect(find.textContaining('Sessiya qabul qilinmadi'), findsOneWidget);
  });

  // ── Masofaviy rejim (production: cmd/adminserver, ONDEXMAP_ADMIN_KEY) ──
  // `defaultUrl` loopback bo'lmagan HAR QANDAY manzil — `ModerationApi.remote`
  // avtomatik `true` bo'ladi (`ondexmap_page.dart`dagi bilan bir xil mantiq).

  testWidgets(
      'masofaviy: kalit kiritilmagan — kiritish tugmasi, so\'rov yuborilmaydi',
      (tester) async {
    final server = _Server();
    tester.view.physicalSize = const Size(1400, 2600);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    final api = ModerationApi(
      defaultUrl: 'https://maps.example.test/admin',
      remoteKey: () async => null,
    );
    await http.runWithClient(() async {
      await tester.pumpWidget(MaterialApp(
        home: Scaffold(body: OndexMapModeration(api: api)),
      ));
      await tester.pumpAndSettle();
    }, () => MockClient((r) async => server.handle(r)));

    expect(find.byKey(const ValueKey('mod-error')), findsOneWidget);
    expect(find.byKey(const ValueKey('mod-enter-key')), findsOneWidget);
    expect(server.requests, isEmpty,
        reason: 'kalit yo\'q bo\'lsa so\'rov umuman yuborilmasin');
  });

  testWidgets('masofaviy: to\'g\'ri kalit bilan yuklanadi (X-API-Key bilan)',
      (tester) async {
    final server = _Server();
    tester.view.physicalSize = const Size(1400, 2600);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    final api = ModerationApi(
      defaultUrl: 'https://maps.example.test/admin',
      remoteKey: () async => 'haqiqiy-kalit',
    );
    await http.runWithClient(() async {
      await tester.pumpWidget(MaterialApp(
        home: Scaffold(body: OndexMapModeration(api: api)),
      ));
      await tester.pumpAndSettle();
    }, () => MockClient((r) async => server.handle(r)));

    expect(find.byKey(const ValueKey('mod-error')), findsNothing);
    expect(server.requests, isNotEmpty);
    expect(server.requests.first.headers['X-API-Key'], 'haqiqiy-kalit');
    expect(server.requests.first.url.origin, 'https://maps.example.test');
    // `defaultUrl`ning O'Z YO'LI (`/admin`) so'rovga QO'SHILISHI SHART —
    // almashtirilib YO'QOLMASIN (production'da aynan shu bug chiqqan edi:
    // `Uri.replace(path: ...)` yo'lni almashtiradi, qo'shmaydi).
    expect(server.requests.first.url.path, '/admin/api/places/meta');
  });

  testWidgets(
      'masofaviy 401: "kalit noto\'g\'ri" xabari va kiritish tugmasi ko\'rinadi',
      (tester) async {
    tester.view.physicalSize = const Size(1400, 2600);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    final api = ModerationApi(
      defaultUrl: 'https://maps.example.test/admin',
      remoteKey: () async => 'eski-kalit',
    );
    await http.runWithClient(() async {
      await tester.pumpWidget(MaterialApp(
        home: Scaffold(body: OndexMapModeration(api: api)),
      ));
      await tester.pumpAndSettle();
    },
        () => MockClient(
            (r) async => _json({'error': 'admin kaliti yaroqsiz'}, 401)));

    expect(
        find.textContaining('ONDEXMAP_ADMIN_KEY noto\'g\'ri'), findsOneWidget);
    expect(find.byKey(const ValueKey('mod-enter-key')), findsOneWidget);
  });
}

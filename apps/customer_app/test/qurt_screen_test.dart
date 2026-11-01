import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

import 'package:chust_customer/api.dart';
import 'package:chust_customer/screens/qurt_screen.dart';
import 'package:chust_customer/widgets/promo_banners.dart';

// Qurt bo'limi va bannerlar: API shartnomasi va ekranlar. Tarmoq soxta (`MockClient`).

http.Response _json(Object body, [int status = 200]) => http.Response(
  jsonEncode(body),
  status,
  headers: {'content-type': 'application/json; charset=utf-8'},
);

const _item = {
  'id': 'q1',
  'store_id': 's1',
  'store_name': 'Urgut qurtlari',
  'name': 'Sho\'r qurt',
  'description': 'Uyda tayyorlangan',
  'composition': 'Qatiq, tuz',
  'category': 'Sho\'r',
  'price_tiyin': 1500000,
  'weight_grams': 250,
  'image_url': '',
  'available': true,
};

Map<String, dynamic> _p(String name, int tiyin) => {
  ..._item,
  'id': name,
  'name': name,
  'price_tiyin': tiyin,
};

/// Yo'l bo'yicha javob beruvchi soxta server.
MockClient _server({
  List<Map<String, dynamic>> items = const [_item],
  List<String> categories = const ['Sho\'r'],
  List<Map<String, dynamic>> storeBanners = const [],
  void Function(Uri)? seen,
}) => MockClient((req) async {
  seen?.call(req.url);
  switch (req.url.path) {
    case '/qurt/categories':
      return _json({'categories': categories});
    case '/promo-banners':
      return _json(
        req.url.queryParameters['placement'] == 'store'
            ? storeBanners
            : <Map<String, dynamic>>[],
      );
    default:
      return _json({'items': items});
  }
});

void _tall(WidgetTester tester) {
  tester.view.physicalSize = const Size(800, 2400);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
}

void main() {
  group('api: qurt va banner so\'rovlari', () {
    test('kategoriya va qidiruv so\'rov satriga to\'g\'ri kodlanadi', () async {
      late Uri sent;
      await http.runWithClient(() async {
        final items = await api.qurtProducts(
          category: 'Sho\'r',
          query: 'qatiq tuz',
        );
        expect(items, hasLength(1));
      }, () => _server(seen: (u) => sent = u));
      expect(sent.path, '/qurt/products');
      expect(sent.queryParameters['category'], 'Sho\'r');
      expect(sent.queryParameters['q'], 'qatiq tuz');
    });

    test('filtrsiz so\'rov — so\'rov satri yo\'q', () async {
      late Uri sent;
      await http.runWithClient(() async {
        await api.qurtProducts();
      }, () => _server(seen: (u) => sent = u));
      expect(sent.query, isEmpty);
    });

    test('kategoriyalar ro\'yxati', () async {
      await http.runWithClient(() async {
        expect(await api.qurtCategories(), ['Sho\'r']);
      }, () => _server());
    });

    test(
      'kutilmagan javob shakli bo\'sh ro\'yxat beradi (ilova yiqilmaydi)',
      () async {
        await http.runWithClient(() async {
          expect(await api.qurtProducts(), isEmpty);
          expect(await api.qurtCategories(), isEmpty);
        }, () => MockClient((_) async => _json({'x': 1})));
      },
    );

    test(
      'banner joylashuvi so\'rovga yoziladi; restoran — for_me bilan, qolganlari for_me\'siz',
      () async {
        final urls = <Uri>[];
        await http.runWithClient(
          () async {
            await api.banners();
            await api.banners(placement: 'menu');
            await api.banners(placement: 'store');
          },
          () => MockClient((req) async {
            urls.add(req.url);
            return _json(<Object>[]);
          }),
        );
        expect(urls[0].queryParameters, {
          'placement': 'restaurant',
          'for_me': '1',
        });
        expect(urls[1].queryParameters, {'placement': 'menu'});
        expect(urls[2].queryParameters, {'placement': 'store'});
      },
    );
  });

  group('PromoBannerCard', () {
    Widget host(Map<String, dynamic> b, {void Function(String)? onAction}) =>
        MaterialApp(
          home: Scaffold(
            body: SizedBox(
              width: 400,
              height: 130,
              child: PromoBannerCard(banner: b, onAction: onAction),
            ),
          ),
        );

    testWidgets('matn, "OnDex" brend rangida va tugma — amal bor bo\'lsa', (
      tester,
    ) async {
      String? got;
      await tester.pumpWidget(
        host({
          'image_url': '',
          'title': 'OnDex Qurt',
          'subtitle': 'Tabiiy, foydali va mazali qurt endi OnDexda!',
          'button_text': 'Buyurtma berish',
          'action': 'qurt',
        }, onAction: (a) => got = a),
      );
      expect(find.textContaining('Tabiiy, foydali'), findsOneWidget);
      expect(find.text('Buyurtma berish'), findsOneWidget);
      expect(
        find.byWidgetPredicate(
          (w) => w is RichText && w.text.toPlainText() == 'OnDex Qurt',
        ),
        findsOneWidget,
      );
      await tester.tap(find.text('Buyurtma berish'));
      expect(got, 'qurt');
    });

    testWidgets('amalsiz banner — tugma chizilmaydi va bosilmaydi', (
      tester,
    ) async {
      String? got;
      await tester.pumpWidget(
        host({
          'image_url': '',
          'title': 'Faqat e\'lon',
          'button_text': 'Bos',
          'action': '',
        }, onAction: (a) => got = a),
      );
      expect(find.text('Faqat e\'lon', findRichText: true), findsOneWidget);
      expect(find.text('Bos'), findsNothing);
      await tester.tap(find.text('Faqat e\'lon', findRichText: true));
      expect(got, isNull);
    });
  });

  group('QurtScreen', () {
    testWidgets(
      'mahsulot nomi, grammi va narxi ko\'rinadi; ulgurji narx yo\'q',
      (tester) async {
        _tall(tester);
        await http.runWithClient(() async {
          await tester.pumpWidget(const MaterialApp(home: QurtScreen()));
          await tester.pumpAndSettle();

          expect(find.text('Sho\'r qurt'), findsOneWidget);
          expect(find.textContaining('250 g'), findsOneWidget);
          expect(find.textContaining('15 000'), findsOneWidget);
          expect(find.text('Barchasi'), findsOneWidget);
          expect(find.text('Sho\'r'), findsOneWidget);
          expect(find.text('Qurt mahsulotlari'), findsOneWidget);
          expect(find.textContaining('wholesale'), findsNothing);

          // Savatga qo'shish tugmasi bor va ishlaydi (qurt buyurtmasi qurilgan).
          expect(find.byKey(const ValueKey('qurt-add-q1')), findsOneWidget);

          await tester.tap(find.text('Sho\'r qurt'));
          await tester.pumpAndSettle();
          expect(find.text('Qatiq, tuz'), findsOneWidget);
          expect(find.text('Tarkibi'), findsOneWidget);
          expect(
            find.textContaining('Urgut qurtlari'),
            findsWidgets,
          ); // detalda do'kon nomi
        }, () => _server());
      },
    );

    testWidgets('do\'kon banneri yo\'q — qattiq yozilgan banner chiqmaydi', (
      tester,
    ) async {
      _tall(tester);
      await http.runWithClient(() async {
        await tester.pumpWidget(const MaterialApp(home: QurtScreen()));
        await tester.pumpAndSettle();
        expect(
          find.text('Tabiiy, foydali va mazali\nqurt mahsulotlari'),
          findsNothing,
        );
        expect(find.text('Qurt mahsulotlari'), findsOneWidget);
      }, () => _server());
    });
    testWidgets('saralash: narx bo\'yicha tartib o\'zgaradi', (tester) async {
      _tall(tester);
      await http.runWithClient(
        () async {
          await tester.pumpWidget(const MaterialApp(home: QurtScreen()));
          await tester.pumpAndSettle();
          double y(String name) => tester.getTopLeft(find.text(name)).dy;
          // Server tartibi: Oddiy, Qimmat, Arzon (2 ustunli to'r: yuqori qator chapdan o'ngga)
          expect(find.text('Arzon'), findsOneWidget);

          await tester.tap(find.byKey(const ValueKey('qurt-sort')));
          await tester.pumpAndSettle();
          await tester.tap(find.text('Narx: arzonroq').last);
          await tester.pumpAndSettle();
          // Arzon birinchi katakda: eng yuqori-chap
          expect(
            tester.getTopLeft(find.text('Arzon')).dx <
                tester.getTopLeft(find.text('Oddiy')).dx,
            isTrue,
          );
          expect(y('Arzon') <= y('Oddiy'), isTrue);

          await tester.tap(find.byKey(const ValueKey('qurt-sort')));
          await tester.pumpAndSettle();
          await tester.tap(find.text('Narx: qimmatroq').last);
          await tester.pumpAndSettle();
          expect(
            tester.getTopLeft(find.text('Qimmat')).dx <
                tester.getTopLeft(find.text('Oddiy')).dx,
            isTrue,
          );
        },
        () => _server(
          items: [
            _p('Oddiy', 1200000),
            _p('Qimmat', 1800000),
            _p('Arzon', 900000),
          ],
        ),
      );
    });

    testWidgets('mahsulot yo\'q bo\'lsa tushunarli matn', (tester) async {
      _tall(tester);
      await http.runWithClient(() async {
        await tester.pumpWidget(const MaterialApp(home: QurtScreen()));
        await tester.pumpAndSettle();
        expect(find.textContaining('qurt mahsulotlari yo\'q'), findsOneWidget);
      }, () => _server(items: const [], categories: const []));
    });

    testWidgets('server xatosida xabar va "Qayta urinish" tugmasi', (
      tester,
    ) async {
      _tall(tester);
      await http.runWithClient(
        () async {
          await tester.pumpWidget(const MaterialApp(home: QurtScreen()));
          await tester.pumpAndSettle();
          expect(find.text('Qayta urinish'), findsOneWidget);
        },
        () => MockClient((_) async => _json({'error': 'xizmat o\'chiq'}, 503)),
      );
    });
  });
  group('PromoBanners (serverdan keladigan bannerlar)', () {
    Repository<List<dynamic>> repoOf(Future<List<dynamic>> Function() fetch) =>
        Repository<List<dynamic>>(
          store: _MemStore(),
          policy: const CachePolicy('t', ttl: Duration(minutes: 5)),
          fetch: fetch,
          encode: (v) => v,
          decode: (j) => (j as List).cast<dynamic>(),
        );

    testWidgets('banner matni chiziladi va amal ishlaydi', (tester) async {
      _tall(tester);
      String? got;
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: PromoBanners(
              placement: 'menu',
              aspectRatio: 3.1,
              onAction: (a) => got = a,
              repository: repoOf(
                () async => [
                  {
                    'id': 'b1',
                    'image_url': '',
                    'placement': 'menu',
                    'title': 'Admin banneri',
                    'button_text': 'Ochish',
                    'action': 'qurt',
                  },
                  {
                    'id': 'b2',
                    'image_url': '',
                    'placement': 'menu',
                    'title': 'Ikkinchi',
                  },
                ],
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text('Admin banneri', findRichText: true), findsOneWidget);
      expect(
        find.text('Ikkinchi', findRichText: true),
        findsOneWidget,
      ); // ketma-ket ustma-ust
      await tester.tap(find.text('Ochish'));
      expect(got, 'qurt');
    });

    testWidgets('bannerlar yo\'q — hech narsa chizilmaydi (yoki fallback)', (
      tester,
    ) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: PromoBanners(
              placement: 'menu',
              aspectRatio: 3.1,
              repository: repoOf(() async => []),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byType(PromoBannerCard), findsNothing);

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: PromoBanners(
              key: const ValueKey('x'),
              placement: 'menu',
              aspectRatio: 3.1,
              fallback: const Text('standart'),
              repository: repoOf(() async => []),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text('standart'), findsOneWidget);
    });

    testWidgets('tarmoq xatosi — ilova yiqilmaydi, fallback ko\'rinadi', (
      tester,
    ) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: PromoBanners(
              placement: 'menu',
              aspectRatio: 3.1,
              fallback: const Text('standart'),
              repository: repoOf(() async => throw Exception('tarmoq yo\'q')),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text('standart'), findsOneWidget);
    });
  });
}

class _MemStore implements CacheStore {
  final _m = <String, String>{};
  @override
  Future<String?> read(String key) async => _m[key];
  @override
  Future<void> write(String key, String json) async => _m[key] = json;
  @override
  Future<void> remove(String key) async => _m.remove(key);
  @override
  Future<void> clearAll() async => _m.clear();
}

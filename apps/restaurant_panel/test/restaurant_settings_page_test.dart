import 'dart:convert';

import 'package:chust_restaurant/api.dart';
import 'package:chust_restaurant/pages/restaurant_settings_page.dart';
import 'package:chust_restaurant/panel_prefs.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';

// "Restoran sozlamalari" sahifasi: namuna bo'yicha ko'rinish (har xil
// kenglikda overflowsiz), qulflangan maydonlar, faqat o'zgargan maydon
// yuborilishi, to'lov usuli tekshiruvi, Click logotipi va xavfli zona.

const _lockedKeys = {'id', 'name', 'kind', 'phone', 'address', 'lat', 'lng', 'tags'};

Map<String, dynamic> _settings({
  String description = 'Kitob va qahva uchun shinam joy',
  bool cash = true,
  bool terminal = false,
  bool online = true,
  bool onlineAvailable = true,
  bool withHours = true,
}) =>
    {
      'id': 'rest-a',
      'name': 'Book Cafe',
      'kind': 'cafe',
      'kind_title': 'Kafe',
      'phone': '+998900000061',
      'address': 'M. Fayozov ko\'chasi',
      'logo_url': '',
      'cover_url': '',
      'description': description,
      'description_max': 500,
      'open': true,
      'open_now': true,
      'working_hours': withHours
          ? {
              'days': [
                for (var d = 1; d <= 7; d++) {'day': d, 'enabled': d != 7, 'open': '08:00', 'close': '23:00'},
              ],
            }
          : null,
      'payment_methods': {'cash': cash, 'card_terminal': terminal, 'card_online': online},
      'online_payments_available': onlineAvailable,
      'timezone': 'Asia/Tashkent',
      'editable': ['logo_url', 'cover_url', 'description', 'working_hours', 'payment_methods'],
      'locked': ['name', 'kind', 'phone', 'address'],
    };

http.Response _json(Object body, [int status = 200]) => http.Response.bytes(
      utf8.encode(jsonEncode(body)),
      status,
      headers: {'content-type': 'application/json; charset=utf-8'},
    );

Future<void> _run(
  WidgetTester tester, {
  Size size = const Size(1600, 2400),
  Map<String, dynamic>? initial,
  SettingsLeaveGuard? guard,
  http.Response? Function(http.Request request)? respond,
  required Future<void> Function(List<http.Request> requests) body,
}) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  SharedPreferences.setMockInitialValues({});
  api.rid = 'rest-a';
  final requests = <http.Request>[];

  await http.runWithClient(
    () async {
      await tester.pumpWidget(MaterialApp(
        home: Scaffold(
          body: RestaurantSettingsPage(
            guard: guard ?? SettingsLeaveGuard(),
            open: true,
            onOpenChanged: (_) async {},
          ),
        ),
      ));
      await tester.pumpAndSettle();
      await body(requests);
      await tester.pumpWidget(const SizedBox());
    },
    () => MockClient((request) async {
      requests.add(request);
      final custom = respond?.call(request);
      if (custom != null) return custom;
      if (request.method == 'GET' && request.url.path == '/restaurants/rest-a/settings') {
        return _json(initial ?? _settings());
      }
      if (request.method == 'PATCH' && request.url.path == '/restaurants/rest-a/settings') {
        final patch = jsonDecode(request.body) as Map<String, dynamic>;
        final next = {...initial ?? _settings(), ...patch};
        return _json(next);
      }
      return _json({'error': 'kutilmagan so\'rov'}, 404);
    }),
  );
}

List<http.Request> _patches(List<http.Request> requests) =>
    requests.where((r) => r.method == 'PATCH').toList();

Finder _assetImage(String name) => find.byWidgetPredicate(
    (w) => w is Image && w.image is AssetImage && (w.image as AssetImage).assetName == name);

Future<void> _tapKey(WidgetTester tester, String key) async {
  final f = find.byKey(ValueKey(key));
  await tester.ensureVisible(f);
  await tester.pumpAndSettle();
  await tester.tap(f);
  await tester.pumpAndSettle();
}

void main() {
  for (final size in const [Size(1600, 1000), Size(1180, 900), Size(820, 900), Size(420, 900)]) {
    testWidgets('namuna bo\'yicha ko\'rinish, overflowsiz — ${size.width.toInt()}px', (tester) async {
      await _run(tester, size: size, body: (_) async {
        expect(tester.takeException(), isNull);
        expect(find.text('Restoran sozlamalari'), findsOneWidget);
        expect(find.text('Restoran ma\'lumotlari'), findsOneWidget);
        expect(find.text('Restoran holati'), findsOneWidget);
        expect(find.text('Ish vaqti'), findsOneWidget);
        expect(find.text('To\'lov usullari'), findsOneWidget);
        expect(find.text('Bildirishnomalar'), findsOneWidget);
        expect(find.text('Xavfli zona'), findsOneWidget);
        // Sarlavhada sozlamalar ikonkasi yo'q, ichki bo'limlar menyusi yo'q.
        expect(find.byIcon(Icons.settings_rounded), findsNothing);
        expect(find.byType(NavigationRail), findsNothing);
        // Hech narsa o'zgarmagan — "Saqlash" o'chiq.
        final save = tester.widget<ButtonStyleButton>(
            find.ancestor(of: find.text('Saqlash'), matching: find.bySubtype<ButtonStyleButton>()).first);
        expect(save.onPressed, isNull);
      });
    });
  }

  testWidgets('nomi, turi, telefon, manzil — faqat ko\'rish uchun', (tester) async {
    await _run(tester, body: (_) async {
      for (final label in ['Restoran nomi', 'Restoran turi', 'Telefon raqam', 'Manzil']) {
        final field = find.byKey(ValueKey('locked-$label'));
        expect(field, findsOneWidget);
        expect(find.descendant(of: field, matching: find.byType(EditableText)), findsNothing);
        expect(find.descendant(of: field, matching: find.byIcon(Icons.lock_outline_rounded)), findsOneWidget);
      }
      // Ikkitadan bir qatorda: Nomi|Turi, Telefon|Manzil.
      double top(String label) => tester.getTopLeft(find.byKey(ValueKey('locked-$label'))).dy;
      expect(top('Restoran turi'), top('Restoran nomi'));
      expect(top('Manzil'), top('Telefon raqam'));
      expect(top('Telefon raqam'), greaterThan(top('Restoran nomi')));
      // "Restoran holati" blokida ikonka emas — logo joyi.
      expect(find.byKey(const ValueKey('status-logo')), findsOneWidget);
      expect(find.text('Book Cafe'), findsOneWidget);
      expect(find.text('Kafe'), findsOneWidget);
      expect(find.text('+998900000061'), findsOneWidget);
      expect(find.text('M. Fayozov ko\'chasi'), findsOneWidget);
      // Sahifadagi YAGONA matn maydoni — tavsif.
      expect(find.byType(TextField), findsOneWidget);
    });
  });

  testWidgets('tavsif: faqat o\'zgargan maydon yuboriladi, qulflangan maydon hech qachon', (tester) async {
    final guard = SettingsLeaveGuard();
    await _run(tester, guard: guard, body: (requests) async {
      expect(guard.hasUnsavedChanges, isFalse);
      await tester.enterText(find.byKey(const ValueKey('settings-description')), '  Yangi tavsif  ');
      await tester.pumpAndSettle();
      expect(guard.hasUnsavedChanges, isTrue);
      expect(find.text('Saqlanmagan o\'zgarishlar'), findsOneWidget);

      await tester.tap(find.byKey(const ValueKey('settings-save')));
      await tester.pumpAndSettle();

      final patches = _patches(requests);
      expect(patches, hasLength(1));
      final body = jsonDecode(patches.single.body) as Map<String, dynamic>;
      expect(body, {'description': 'Yangi tavsif'});
      expect(body.keys.toSet().intersection(_lockedKeys), isEmpty);
      expect(find.text('Sozlamalar saqlandi'), findsOneWidget);
      expect(guard.hasUnsavedChanges, isFalse);
    });
  });

  testWidgets('"Bekor qilish" o\'zgarishni qaytaradi va hech narsa yubormaydi', (tester) async {
    await _run(tester, body: (requests) async {
      await tester.enterText(find.byKey(const ValueKey('settings-description')), 'Boshqa');
      await tester.pumpAndSettle();
      await tester.tap(find.text('Bekor qilish'));
      await tester.pumpAndSettle();
      expect(find.text('Kitob va qahva uchun shinam joy'), findsOneWidget);
      expect(_patches(requests), isEmpty);
    });
  });

  testWidgets('ish vaqti: kunni o\'chirish butun haftani yuboradi', (tester) async {
    await _run(tester, body: (requests) async {
      await _tapKey(tester, 'day-3');
      await tester.tap(find.byKey(const ValueKey('settings-save')));
      await tester.pumpAndSettle();

      final body = jsonDecode(_patches(requests).single.body) as Map<String, dynamic>;
      expect(body.keys, ['working_hours']);
      final days = (body['working_hours']['days'] as List).cast<Map<String, dynamic>>();
      expect(days, hasLength(7));
      expect(days.firstWhere((d) => d['day'] == 3)['enabled'], isFalse);
      expect(days.firstWhere((d) => d['day'] == 1)['enabled'], isTrue);
    });
  });

  testWidgets('to\'lov usullari: hammasini o\'chirib bo\'lmaydi', (tester) async {
    await _run(tester, body: (requests) async {
      // "Tahrirlash" tugmasi yo'q — kalitlar darhol bosiladi, kuchga "Saqlash" bilan kiradi.
      expect(find.byKey(const ValueKey('payments-edit')), findsNothing);
      await _tapKey(tester, 'pay-cash');
      expect(find.text('Saqlanmagan o\'zgarishlar'), findsOneWidget);
      expect(_patches(requests), isEmpty);
      await _tapKey(tester, 'pay-online');
      expect(find.text('Kamida bitta to\'lov usuli yoqilgan bo\'lishi kerak'), findsOneWidget);

      await tester.tap(find.byKey(const ValueKey('settings-save')));
      await tester.pumpAndSettle();
      expect(_patches(requests), isEmpty);

      await _tapKey(tester, 'pay-terminal');
      await tester.tap(find.byKey(const ValueKey('settings-save')));
      await tester.pumpAndSettle();
      final body = jsonDecode(_patches(requests).single.body) as Map<String, dynamic>;
      expect(body, {
        'payment_methods': {'cash': false, 'card_terminal': true, 'card_online': false, 'ondex_wallet': true},
      });
    });
  });

  testWidgets('onlayn to\'lov ulanmagan bo\'lsa uni yoqib bo\'lmaydi', (tester) async {
    await _run(tester, initial: _settings(online: false, onlineAvailable: false), body: (_) async {
      expect(tester.widget<Switch>(find.byKey(const ValueKey('pay-online'))).onChanged, isNull);
      expect(tester.widget<Switch>(find.byKey(const ValueKey('pay-cash'))).onChanged, isNotNull);
    });
  });

  testWidgets('Click va Payme — alohida, haqiqiy logotip rasmi, "Tez orada"', (tester) async {
    await _run(tester, body: (_) async {
      expect(_assetImage('assets/payments/click.png'), findsOneWidget);
      expect(_assetImage('assets/payments/payme.png'), findsOneWidget);
      expect(find.text('Click'), findsOneWidget);
      expect(find.text('Payme'), findsOneWidget);
      expect(find.text('Tez orada'), findsNWidgets(2));
      for (final key in ['pay-click', 'pay-payme']) {
        final s = tester.widget<Switch>(find.byKey(ValueKey(key)));
        expect(s.value, isFalse);
        expect(s.onChanged, isNull);
      }
    });
  });

  testWidgets('OnDex Wallet doim yoqilgan — bosilsa ham o\'chmaydi', (tester) async {
    await _run(tester, initial: _settings(), body: (requests) async {
      expect(find.text('OnDex Wallet'), findsOneWidget);
      expect(_assetImage('assets/ondex.png'), findsOneWidget);
      expect(tester.widget<Switch>(find.byKey(const ValueKey('pay-wallet'))).value, isTrue);

      await _tapKey(tester, 'pay-wallet');
      expect(tester.widget<Switch>(find.byKey(const ValueKey('pay-wallet'))).value, isTrue);
      expect(find.text('OnDex Wallet doim yoqilgan — uni o\'chirib bo\'lmaydi'), findsOneWidget);
      // Bosish o'zgarish hisoblanmaydi va hech narsa yuborilmaydi.
      expect(find.text('Saqlanmagan o\'zgarishlar'), findsNothing);
      expect(_patches(requests), isEmpty);
    });
  });

  testWidgets('server xatosi foydalanuvchiga ko\'rsatiladi', (tester) async {
    await _run(
      tester,
      respond: (r) => r.method == 'PATCH'
          ? _json({'error': '"name" maydonini faqat OnDex administratori o\'zgartira oladi'}, 403)
          : null,
      body: (requests) async {
        await tester.enterText(find.byKey(const ValueKey('settings-description')), 'X');
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const ValueKey('settings-save')));
        await tester.pumpAndSettle();
        expect(find.text('"name" maydonini faqat OnDex administratori o\'zgartira oladi'), findsOneWidget);
        // O'zgarish yo'qolmaydi — qayta urinish mumkin.
        expect(find.text('Saqlanmagan o\'zgarishlar'), findsOneWidget);
      },
    );
  });

  testWidgets('bildirishnomalar shu kompyuterda saqlanadi', (tester) async {
    await _run(tester, body: (requests) async {
      expect(PanelPrefs.newOrderBanner.value, isTrue);
      await _tapKey(tester, 'pref-banner');
      expect(PanelPrefs.newOrderBanner.value, isFalse);
      final prefs = await SharedPreferences.getInstance();
      expect(prefs.getBool('panel.notify.new_order_banner'), isFalse);
      // Serverga hech narsa yuborilmaydi.
      expect(_patches(requests), isEmpty);
      await _tapKey(tester, 'pref-banner');
      expect(PanelPrefs.newOrderBanner.value, isTrue);
    });
  });

  testWidgets('xavfli zona: faqat ma\'lumot, o\'chirish so\'rovi yuborilmaydi', (tester) async {
    await _run(tester, body: (requests) async {
      await _tapKey(tester, 'danger-delete');
      expect(find.textContaining('faqat OnDex administratori o\'chira oladi'), findsOneWidget);
      await tester.tap(find.text('Tushunarli'));
      await tester.pumpAndSettle();
      expect(requests.where((r) => r.method != 'GET'), isEmpty);
    });
  });

  testWidgets('ish vaqti belgilanmagan bo\'lsa — kun bo\'yi, bosib belgilanadi', (tester) async {
    await _run(tester, initial: _settings(withHours: false), body: (requests) async {
      expect(find.textContaining('Ish vaqti belgilanmagan'), findsWidgets);
      await _tapKey(tester, 'hours-setup');
      await tester.tap(find.byKey(const ValueKey('settings-save')));
      await tester.pumpAndSettle();
      final body = jsonDecode(_patches(requests).single.body) as Map<String, dynamic>;
      expect((body['working_hours']['days'] as List), hasLength(7));
    });
  });
}

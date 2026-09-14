import 'dart:convert';

import 'package:chust_admin/api.dart';
import 'package:chust_admin/pages/support_chat_page.dart';
import 'package:chust_admin/support_inbox.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

// Admin "Chat": suhbatlar ro'yxati (qidiruv, o'qilmaganlar), suhbatni ochish
// o'qilgan deb belgilaydi, javob yuboriladi, BEGONA restoran hodisasi ochiq
// suhbatga tushmaydi — faqat ro'yxat yangilanadi.

http.Response _json(Object body, [int status = 200]) => http.Response.bytes(
      utf8.encode(jsonEncode(body)),
      status,
      headers: {'content-type': 'application/json; charset=utf-8'},
    );

Map<String, dynamic> _msg(int seq, String sender, String body, {String? clientId}) => {
      'id': 'm$seq',
      'seq': seq,
      'sender': sender,
      'sender_name': sender == 'admin' ? 'OnDex qo\'llab-quvvatlash' : 'Book Cafe',
      'body': body,
      'client_id': clientId ?? 'client-$seq-abcdefgh',
      'created_at': DateTime.now().toUtc().toIso8601String(),
    };

Map<String, dynamic> _thread(String rid,
        {required int last, int read = 0, int peer = 0, int unread = 0, int count = 1, String lastSender = 'restaurant', String lastBody = ''}) =>
    {
      'restaurant_id': rid,
      'message_count': count,
      'first_at': DateTime(2026, 9, 12, 10).toUtc().toIso8601String(),
      'last_at': DateTime.now().toUtc().toIso8601String(),
      'last_seq': last,
      'last_sender': lastSender,
      'last_body': lastBody,
      'read_seq': read,
      'peer_read_seq': peer,
      'unread': unread,
    };

AdminSupportInbox _inbox() =>
    AdminSupportInbox(bus: LiveBus(ticketProvider: () async => 't', urlBuilder: (t) => 'ws://localhost/ws?ticket=$t'));

Future<void> _run(WidgetTester tester, Size size, Future<void> Function(List<http.Request>, AdminSupportInbox) body) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  final inbox = _inbox();
  final requests = <http.Request>[];
  await http.runWithClient(
    () async {
      await tester.pumpWidget(MaterialApp(
        theme: ThemeData(colorScheme: ColorScheme.fromSeed(seedColor: const Color(0xFF0B5D1E)), useMaterial3: true),
        home: Scaffold(body: SupportChatPage(inbox: inbox)),
      ));
      await tester.pumpAndSettle();
      await body(requests, inbox);
      await tester.pumpWidget(const SizedBox());
    },
    () => MockClient((request) async {
      requests.add(request);
      final path = request.url.path;
      if (request.method == 'GET' && path == '/admin/support/threads') {
        return _json({
          'items': [
            {
              ..._thread('rest-a', last: 2, unread: 2, count: 2, lastBody: 'Printer ishlamayapti'),
              'restaurant': {'id': 'rest-a', 'name': 'Book Cafe', 'address': 'M. Fayozov ko\'chasi', 'logo_url': ''},
              'restaurant_online': true,
            },
            {
              ..._thread('rest-b', last: 1, count: 1, lastSender: 'admin', lastBody: 'Rahmat'),
              'restaurant': {'id': 'rest-b', 'name': 'Chust Osh Markazi', 'address': '', 'logo_url': ''},
              'restaurant_online': false,
            },
          ],
          'unread_total': 2,
        });
      }
      if (request.method == 'GET' && path == '/admin/support/threads/rest-a/messages') {
        return _json({
          'items': [_msg(1, 'restaurant', 'Salom'), _msg(2, 'restaurant', 'Printer ishlamayapti')],
          'next_before': 0,
          'thread': _thread('rest-a', last: 2, unread: 2, count: 2),
          'restaurant': {'id': 'rest-a', 'name': 'Book Cafe'},
          'restaurant_online': true,
        });
      }
      if (request.method == 'POST' && path == '/admin/support/threads/rest-a/read') {
        return _json({'thread': _thread('rest-a', last: 2, read: 2, unread: 0, count: 2)});
      }
      if (request.method == 'POST' && path == '/admin/support/threads/rest-a/messages') {
        final b = jsonDecode(request.body) as Map<String, dynamic>;
        return _json({
          'message': _msg(3, 'admin', b['body'] as String, clientId: b['client_id'] as String),
          'thread': _thread('rest-a', last: 3, read: 3, peer: 2, count: 3, lastSender: 'admin'),
        }, 201);
      }
      if (request.method == 'GET' && path == '/admin/support/summary') return _json({'unread': 1});
      return _json({'error': 'kutilmagan so\'rov: ${request.method} $path'}, 404);
    }),
  );
}

void main() {
  testWidgets('ro\'yxat, suhbat, javob va begona restoran hodisasi', (tester) async {
    await _run(tester, const Size(1600, 1000), (requests, inbox) async {
      expect(tester.takeException(), isNull);
      expect(find.text('Book Cafe'), findsOneWidget);
      expect(find.text('Chust Osh Markazi'), findsOneWidget);
      expect(find.text('2 ta o\'qilmagan'), findsOneWidget);
      expect(find.text('Suhbatni tanlang'), findsOneWidget);

      await tester.enterText(find.byKey(const ValueKey('admin-support-search')), 'osh');
      await tester.pumpAndSettle();
      expect(find.text('Book Cafe'), findsNothing);
      await tester.enterText(find.byKey(const ValueKey('admin-support-search')), '');
      await tester.tap(find.text('O\'qilmagan (1)'));
      await tester.pumpAndSettle();
      expect(find.text('Chust Osh Markazi'), findsNothing);
      await tester.tap(find.text('Barchasi'));
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const ValueKey('admin-thread-rest-a')));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('support-msg-m2')), findsOneWidget);
      expect(find.text('Muloqot tarixi'), findsOneWidget);
      final read = requests.where((r) => r.url.path.endsWith('/rest-a/read')).single;
      expect(jsonDecode(read.body), {'up_to_seq': 2});
      expect(find.byKey(const ValueKey('admin-support-unread')), findsNothing);

      final input = find.byKey(const ValueKey('support-input'));
      await tester.tap(input);
      await tester.enterText(input, 'Printer modelini yozing');
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await tester.pumpAndSettle();
      final sent = requests.singleWhere((r) => r.method == 'POST' && r.url.path.endsWith('/rest-a/messages'));
      expect((jsonDecode(sent.body) as Map)['body'], 'Printer modelini yozing');
      expect(find.byKey(const ValueKey('support-msg-m3')), findsOneWidget);

      // Boshqa restorandan yangi xabar: ro'yxat tepasiga chiqadi, ochiq
      // suhbatga TUSHMAYDI.
      inbox.handleEvent({
        'type': 'support_message',
        'restaurant_id': 'rest-b',
        'message': _msg(4, 'restaurant', 'Yangi savol'),
        'thread': _thread('rest-b', last: 4, unread: 1, count: 2, lastBody: 'Yangi savol'),
      });
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('support-msg-m4')), findsNothing);
      expect(find.text('Yangi savol'), findsOneWidget);
      expect(tester.getTopLeft(find.byKey(const ValueKey('admin-thread-rest-b'))).dy,
          lessThan(tester.getTopLeft(find.byKey(const ValueKey('admin-thread-rest-a'))).dy));
      expect(find.text('1 ta o\'qilmagan'), findsOneWidget);
      // Rozetka hisobi serverdan (bir nechta hodisa — bitta so'rov).
      await tester.pump(const Duration(milliseconds: 350));
      await tester.pumpAndSettle();
      expect(inbox.unread, 1);
      expect(tester.takeException(), isNull);
    });
  });

  testWidgets('tor oynada ro\'yxat -> suhbat -> orqaga', (tester) async {
    await _run(tester, const Size(700, 900), (requests, inbox) async {
      expect(tester.takeException(), isNull);
      await tester.tap(find.byKey(const ValueKey('admin-thread-rest-a')));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('support-msg-m1')), findsOneWidget);
      expect(find.byKey(const ValueKey('admin-thread-rest-b')), findsNothing);
      await tester.tap(find.byTooltip('Orqaga'));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('admin-thread-rest-b')), findsOneWidget);
      expect(tester.takeException(), isNull);
    });
  });
}

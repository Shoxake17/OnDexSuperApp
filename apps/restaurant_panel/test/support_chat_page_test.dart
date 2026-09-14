import 'dart:convert';

import 'package:chust_restaurant/api.dart';
import 'package:chust_restaurant/pages/support_chat_page.dart';
import 'package:chust_restaurant/support_center.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:ondex_support/ondex_support.dart';

// "Chat markazi": sarlavha va yon ma'lumot bloki YO'Q (oyna butun joyni
// egallaydi), yozishma yuklanadi, matn va rasm yuboriladi (client_id bilan),
// admin javobi jonli keladi va o'qilgan deb belgilanadi.

final _png = base64Decode('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==');

http.Response _json(Object body, [int status = 200]) => http.Response.bytes(
      utf8.encode(jsonEncode(body)),
      status,
      headers: {'content-type': 'application/json; charset=utf-8'},
    );

Map<String, dynamic> _msg(int seq, String sender, String body, {String? clientId, Map<String, dynamic>? attachment}) => {
      'id': 'm$seq',
      'seq': seq,
      'sender': sender,
      'sender_name': sender == 'admin' ? 'OnDex qo\'llab-quvvatlash' : 'Book Cafe',
      'body': body,
      'client_id': clientId ?? 'client-$seq-abcdefgh',
      if (attachment != null) 'attachment': attachment,
      'created_at': DateTime.now().toUtc().toIso8601String(),
    };

Map<String, dynamic> _thread({required int last, int read = 0, int peer = 0, int unread = 0, int count = 1}) => {
      'restaurant_id': 'rest-a',
      'message_count': count,
      'first_at': DateTime(2026, 9, 12, 10).toUtc().toIso8601String(),
      'last_at': DateTime.now().toUtc().toIso8601String(),
      'last_seq': last,
      'last_sender': 'admin',
      'last_body': 'x',
      'read_seq': read,
      'peer_read_seq': peer,
      'unread': unread,
    };

SupportCenter _center() => SupportCenter(
    bus: LiveBus(ticketProvider: () async => 't', urlBuilder: (t) => 'ws://localhost/ws?ticket=$t'));

void main() {
  for (final size in const [Size(1600, 1000), Size(1000, 900), Size(720, 900)]) {
    testWidgets('yozishma, matn, rasm va jonli javob — ${size.width.toInt()}px', (tester) async {
      tester.view.physicalSize = size;
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.reset);
      api.rid = 'rest-a';
      final center = _center();
      final requests = <http.Request>[];

      await http.runWithClient(
        () async {
          await tester.pumpWidget(MaterialApp(
            home: Scaffold(
              body: SupportChatPage(
                center: center,
                imagePicker: () async => SupportImageDraft(bytes: _png, filename: 'ekran.png'),
                imageProvider: (_) => MemoryImage(_png),
              ),
            ),
          ));
          await tester.pumpAndSettle();
          expect(tester.takeException(), isNull);

          // Olib tashlangan bloklar.
          for (final gone in [
            'Chat markazi',
            'OnDex qo\'llab-quvvatlash jamoasi bilan yozishmalar',
            'Tezkor amallar',
            'Telegram orqali yozish',
            'Muloqot tarixi',
          ]) {
            expect(find.text(gone), findsNothing, reason: gone);
          }
          expect(find.textContaining('Savol, taklif yoki muammo'), findsNothing);
          expect(find.byTooltip('Ma\'lumot'), findsNothing);
          expect(requests.where((r) => r.url.path == '/support/contacts'), isEmpty);

          // Yozishma oynasi sahifaning deyarli butun eniga.
          final chatWidth = tester.getSize(find.byKey(const ValueKey('support-messages'))).width;
          final listShown = find.byKey(const ValueKey('support-thread-tile')).evaluate().isNotEmpty;
          expect(chatWidth, greaterThan(size.width - 48 - (listShown ? 316 : 0) - 4));

          expect(find.text('Assalomu alaykum! Qanday yordam bera olamiz?'), findsOneWidget);
          final read = requests.where((r) => r.url.path.endsWith('/support/read')).toList();
          expect(read, hasLength(1));
          expect(jsonDecode(read.single.body), {'up_to_seq': 5});

          final input = find.byKey(const ValueKey('support-input'));
          await tester.tap(input);
          await tester.enterText(input, 'Printer ishlamayapti');
          await tester.sendKeyEvent(LogicalKeyboardKey.enter);
          await tester.pumpAndSettle();
          final sent = requests.where((r) => r.method == 'POST' && r.url.path.endsWith('/support/messages')).single;
          final payload = jsonDecode(sent.body) as Map<String, dynamic>;
          expect(payload['body'], 'Printer ishlamayapti');
          expect(RegExp(r'^[A-Za-z0-9_-]{24}$').hasMatch(payload['client_id'] as String), isTrue);
          expect(find.byKey(const ValueKey('support-msg-m6')), findsOneWidget);

          // Admin javobi jonli kanaldan.
          center.handleEvent({
            'type': 'support_message',
            'message': _msg(7, 'admin', 'Ekran rasmini yuboring, iltimos'),
            'thread': _thread(last: 7, read: 6, peer: 6, unread: 1, count: 3),
          });
          await tester.pumpAndSettle();
          expect(find.text('Ekran rasmini yuboring, iltimos'), findsOneWidget);
          final reads = requests.where((r) => r.url.path.endsWith('/support/read')).toList();
          expect(jsonDecode(reads.last.body), {'up_to_seq': 7});
          expect(center.unread, 0);

          // Rasm: tanlash -> izoh -> Enter -> multipart.
          await tester.tap(find.byKey(const ValueKey('support-attach')));
          await tester.pumpAndSettle();
          expect(find.byKey(const ValueKey('support-draft')), findsOneWidget);
          await tester.enterText(input, 'Xato shu yerda');
          await tester.sendKeyEvent(LogicalKeyboardKey.enter);
          await tester.pumpAndSettle();
          final multipart = requests
              .where((r) => r.method == 'POST' && (r.headers['content-type'] ?? '').startsWith('multipart/form-data'))
              .single;
          expect(multipart.url.path, '/restaurants/rest-a/support/messages');
          final raw = latin1.decode(multipart.bodyBytes);
          expect(raw, contains('filename="ekran.png"'));
          expect(raw, contains('Xato shu yerda'));
          expect(RegExp(r'name="client_id"\r\n\r\n[A-Za-z0-9_-]{24}\r\n').hasMatch(raw), isTrue);
          expect(multipart.headers['authorization'], isNull, reason: 'testda token yo\'q');
          expect(find.byKey(const ValueKey('support-image-m8')), findsOneWidget);
          expect(tester.takeException(), isNull);

          await tester.pumpWidget(const SizedBox());
        },
        () => MockClient((request) async {
          requests.add(request);
          final path = request.url.path;
          final isMultipart = (request.headers['content-type'] ?? '').startsWith('multipart/form-data');
          if (request.method == 'GET' && path == '/restaurants/rest-a/support/messages') {
            return _json({
              'items': [_msg(5, 'admin', 'Assalomu alaykum! Qanday yordam bera olamiz?')],
              'next_before': 0,
              'thread': _thread(last: 5, unread: 1),
              'support_online': true,
            });
          }
          if (request.method == 'POST' && path == '/restaurants/rest-a/support/read') {
            final up = (jsonDecode(request.body) as Map)['up_to_seq'] as int;
            return _json({'thread': _thread(last: up, read: up, peer: up, unread: 0, count: up == 5 ? 1 : 3)});
          }
          if (request.method == 'POST' && path == '/restaurants/rest-a/support/messages' && isMultipart) {
            final raw = latin1.decode(request.bodyBytes);
            final cid = RegExp(r'name="client_id"\r\n\r\n([A-Za-z0-9_-]+)').firstMatch(raw)!.group(1)!;
            return _json({
              'message': _msg(8, 'restaurant', 'Xato shu yerda',
                  clientId: cid, attachment: {'id': 'att-1', 'content_type': 'image/webp', 'width': 1, 'height': 1}),
              'thread': _thread(last: 8, read: 8, peer: 7, unread: 0, count: 4),
            }, 201);
          }
          if (request.method == 'POST' && path == '/restaurants/rest-a/support/messages') {
            final b = jsonDecode(request.body) as Map<String, dynamic>;
            return _json({
              'message': _msg(6, 'restaurant', b['body'] as String, clientId: b['client_id'] as String),
              'thread': _thread(last: 6, read: 6, peer: 5, unread: 0, count: 2),
            }, 201);
          }
          return _json({'error': 'kutilmagan so\'rov: ${request.method} $path'}, 404);
        }),
      );
    });
  }
}

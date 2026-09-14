import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:ondex_support/ondex_support.dart';

/// 1x1 shaffof PNG.
final _png = base64Decode('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==');

Map<String, dynamic> _msg(int seq, String sender, String body, {String? clientId, Map<String, dynamic>? attachment}) => {
      'id': 'm$seq',
      'seq': seq,
      'sender': sender,
      'sender_name': sender == kSideAdmin ? 'OnDex qo\'llab-quvvatlash' : 'Book Cafe',
      'body': body,
      'client_id': clientId ?? 'client-$seq-abcdef',
      if (attachment != null) 'attachment': attachment,
      'created_at': DateTime(2026, 9, 12, 14, seq % 60).toUtc().toIso8601String(),
    };

/// Server o'rnini bosuvchi: sahifalash, takroriy `client_id`, o'qish belgisi, rasm.
class _FakeServer {
  _FakeServer(this.viewer);

  final String viewer;
  final messages = <Map<String, dynamic>>[];
  final sends = <List<String>>[];
  final imageSends = <(String body, String clientId, int bytes, String filename)>[];
  final reads = <int>[];
  final fetches = <Map<String, int?>>[];
  int _seq = 0;
  int readSeq = 0;
  int peerReadSeq = 0;
  int failuresLeft = 0;

  Map<String, dynamic> add(String sender, String body, {String? clientId, Map<String, dynamic>? attachment}) {
    _seq++;
    final m = _msg(_seq, sender, body, clientId: clientId, attachment: attachment);
    messages.add(m);
    return m;
  }

  Map<String, dynamic> get thread => {
        'restaurant_id': 'rest-a',
        'message_count': messages.length,
        'last_seq': _seq,
        'read_seq': readSeq,
        'peer_read_seq': peerReadSeq,
        'unread': messages.where((m) => m['sender'] != viewer && (m['seq'] as int) > readSeq).length,
      };

  Future<Map<String, dynamic>> fetch({int? before, int? after, int limit = 50}) async {
    fetches.add({'before': before, 'after': after, 'limit': limit});
    if (after != null) {
      final list = messages.where((m) => (m['seq'] as int) > after).take(limit).toList();
      return {'items': list, 'next_before': 0, 'thread': thread};
    }
    final pool = before == null ? messages : messages.where((m) => (m['seq'] as int) < before).toList();
    final desc = pool.reversed.take(limit).toList();
    final asc = desc.reversed.toList();
    return {'items': asc, 'next_before': desc.length == limit ? asc.first['seq'] : 0, 'thread': thread};
  }

  Map<String, dynamic> _store(String body, String clientId, {Map<String, dynamic>? attachment}) {
    final existing = messages.where((m) => m['client_id'] == clientId).toList();
    if (existing.isNotEmpty) return {'message': existing.first, 'thread': thread};
    final m = add(viewer, body, clientId: clientId, attachment: attachment);
    readSeq = m['seq'] as int;
    return {'message': m, 'thread': thread};
  }

  Future<Map<String, dynamic>> send(String body, String clientId) async {
    sends.add([body, clientId]);
    if (failuresLeft > 0) {
      failuresLeft--;
      throw Exception('Server javob bermadi');
    }
    return _store(body, clientId);
  }

  Future<Map<String, dynamic>> sendImage(String body, String clientId, Uint8List bytes, String filename) async {
    imageSends.add((body, clientId, bytes.length, filename));
    if (failuresLeft > 0) {
      failuresLeft--;
      throw Exception('Rasm yuborilmadi');
    }
    return _store(body, clientId,
        attachment: {'id': 'att-${imageSends.length}', 'content_type': 'image/webp', 'width': 800, 'height': 600, 'size': 1234});
  }

  Future<Map<String, dynamic>> markRead(int upTo) async {
    reads.add(upTo);
    readSeq = upTo > _seq ? _seq : upTo;
    return {'thread': thread};
  }

  SupportConversation conversation({String? restaurantId, int pageSize = 3, bool images = true}) => SupportConversation(
        viewer: viewer,
        fetch: fetch,
        sendMessage: send,
        sendImage: images ? sendImage : null,
        markRead: markRead,
        restaurantId: restaurantId,
        pageSize: pageSize,
      );
}

void main() {
  group('havolalar — faqat oq ro\'yxat', () {
    test('telefon', () {
      expect(supportTelUri('+998901234567').toString(), 'tel:+998901234567');
      for (final bad in ['', '+99890123456', '998901234567', 'javascript:alert(1)', '+998 90 123 45 67', 'tel:+998901234567']) {
        expect(supportTelUri(bad), isNull, reason: bad);
      }
      expect(formatSupportPhone('+998901234567'), '+998 90 123 45 67');
      expect(normalizePhoneInput(' 998 (90) 123-45-67 '), '+998901234567');
    });

    test('telegram', () {
      expect(supportTelegramUri('ondex_support').toString(), 'https://t.me/ondex_support');
      for (final bad in ['', 'abcd', '_ondex', 'ondex_', 'ondex__x', 'javascript:alert(1)', 'a/../../x', 'evil.com/x', 'ondex?x=1']) {
        expect(supportTelegramUri(bad), isNull, reason: bad);
      }
      expect(normalizeTelegramInput(' https://t.me/OnDex_Support/ '), 'OnDex_Support');
      expect(normalizeTelegramInput('@ondex'), 'ondex');
    });

    test('pochta', () {
      final uri = supportMailUri('support@ondex.uz', subject: 'OnDex panel')!;
      expect(uri.scheme, 'mailto');
      expect(uri.path, 'support@ondex.uz');
      expect(uri.query, 'subject=OnDex%20panel');
      for (final bad in ['', 'a@b', 'x@y.uz?cc=z@z.uz', 'Ali <a@b.uz>', 'a..b@x.uz', 'a b@x.uz', 'javascript:alert(1)']) {
        expect(supportMailUri(bad), isNull, reason: bad);
      }
    });

    test('buzilgan javob havola bermaydi', () {
      final c = SupportContacts.fromJson({
        'phone': 'javascript:alert(1)',
        'telegram': 'https://evil.com',
        'telegram_url': 'javascript:alert(1)',
        'email': 'x@y.uz?bcc=all@corp.uz',
      });
      expect(c.configured, isFalse);
      expect(c.telUri, isNull);
      expect(c.telegramUri, isNull);
      expect(c.mailUri, isNull);
      expect(SupportContacts.fromJson('not a map').configured, isFalse);
    });
  });

  group('modellar', () {
    test('buzilgan xabar ekranga chiqmaydi', () {
      expect(SupportMessage.tryParse(_msg(1, kSideAdmin, 'ok')), isNotNull);
      expect(SupportMessage.tryParse({..._msg(1, kSideAdmin, 'ok'), 'sender': 'system'}), isNull);
      expect(SupportMessage.tryParse({..._msg(1, kSideAdmin, 'ok'), 'seq': 0}), isNull);
      expect(SupportMessage.tryParse({..._msg(1, kSideAdmin, 'ok'), 'body': 5}), isNull);
      expect(SupportMessage.tryParse(_msg(1, kSideAdmin, '')), isNull, reason: 'rasmsiz bo\'sh xabar');
      expect(SupportMessage.tryParse(null), isNull);
    });

    test('rasm: shakli qat\'iy (havola yo\'liga qo\'yiladi)', () {
      const good = {'id': 'att-1', 'content_type': 'image/webp', 'width': 800, 'height': 600};
      final m = SupportMessage.tryParse(_msg(2, kSideAdmin, '', attachment: good))!;
      expect(m.attachment!.aspectRatio, closeTo(800 / 600, 0.001));
      for (final bad in ['../x', 'a/b', '', 'a b', '%2e%2e']) {
        expect(SupportAttachment.tryParse({...good, 'id': bad}), isNull, reason: bad);
      }
      expect(SupportAttachment.tryParse({...good, 'width': 0}), isNull);
    });

    test('rasm tekshiruvi: hajm, kengaytma va fayl imzosi', () {
      expect(validateSupportImage(SupportImageDraft(bytes: _png, filename: 'ekran.PNG')), isNull);
      expect(validateSupportImage(SupportImageDraft(bytes: _png, filename: 'ekran.exe')), contains('JPG, PNG'));
      expect(validateSupportImage(SupportImageDraft(bytes: _png, filename: 'ekran')), contains('JPG, PNG'));
      expect(validateSupportImage(SupportImageDraft(bytes: Uint8List.fromList(utf8.encode('<svg/>')), filename: 'x.png')),
          contains('rasm emas'));
      expect(validateSupportImage(SupportImageDraft(bytes: Uint8List(0), filename: 'x.png')), isNotNull);
      final big = Uint8List(kSupportMaxImageBytes + 1)..setRange(0, 4, const [0x89, 0x50, 0x4E, 0x47]);
      expect(validateSupportImage(SupportImageDraft(bytes: big, filename: 'x.png')), contains('10 MB'));
      final jpeg = Uint8List.fromList(const [0xFF, 0xD8, 0xFF, 0xE0, 0, 0]);
      expect(validateSupportImage(SupportImageDraft(bytes: jpeg, filename: 'a.jpeg')), isNull);
      final webp = Uint8List.fromList(utf8.encode('RIFF1234WEBPVP8 '));
      expect(validateSupportImage(SupportImageDraft(bytes: webp, filename: 'a.webp')), isNull);
    });

    test('eski surat o\'qish belgisini orqaga qaytarmaydi', () {
      const fresh = SupportThread(restaurantId: 'r', lastSeq: 10, readSeq: 10, peerReadSeq: 9, unread: 0);
      const stale = SupportThread(restaurantId: 'r', lastSeq: 8, readSeq: 4, peerReadSeq: 3, unread: 2);
      final merged = fresh.merge(stale);
      expect(merged.lastSeq, 10);
      expect(merged.readSeq, 10);
      expect(merged.peerReadSeq, 9);
      expect(merged.unread, 0);
      expect(stale.merge(fresh).peerReadSeq, 9);
    });

    test('client_id shakli server talabiga mos', () {
      final ids = {for (var i = 0; i < 200; i++) newSupportClientId()};
      expect(ids, hasLength(200));
      for (final id in ids) {
        expect(RegExp(r'^[A-Za-z0-9_-]{24}$').hasMatch(id), isTrue, reason: id);
      }
    });
  });

  group('suhbat holati', () {
    test('yuklash, eski sahifa va o\'qish belgisi', () async {
      final s = _FakeServer(kSideRestaurant);
      for (var i = 0; i < 5; i++) {
        s.add(i.isEven ? kSideRestaurant : kSideAdmin, 'xabar $i');
      }
      final c = s.conversation();
      await c.load();
      expect(c.messages.map((m) => m.seq), [3, 4, 5]);
      expect(c.hasOlder, isTrue);
      expect(s.reads, [5], reason: 'admin xabari ko\'rindi — o\'qildi');
      await c.loadOlder();
      expect(s.fetches.last['before'], 3);
      expect(c.messages.map((m) => m.seq), [1, 2, 3, 4, 5]);
    });

    test('yuborish: optimistik, keyin tasdiq — bitta nusxa', () async {
      final s = _FakeServer(kSideRestaurant);
      final c = s.conversation();
      await c.load();
      final future = c.send('  Printer ishlamayapti  ');
      expect(c.messages.single.pending, isTrue);
      await future;
      expect(c.messages.single.confirmed, isTrue);
      expect(c.messages.single.body, 'Printer ishlamayapti');
      // Xuddi shu xabar jonli kanaldan ham keladi.
      expect(c.handleEvent({'type': 'support_message', 'message': s.messages.single, 'thread': s.thread}), isTrue);
      expect(c.messages, hasLength(1));
    });

    test('yuborilmadi -> qayta yuborish AYNI kalit bilan', () async {
      final s = _FakeServer(kSideRestaurant)..failuresLeft = 1;
      final c = s.conversation();
      await c.load();
      await c.send('Salom');
      final failed = c.messages.single;
      expect(failed.failed, isTrue);
      expect(failed.error, contains('Server javob bermadi'));
      await c.retry(failed);
      expect(s.sends.map((e) => e[1]).toSet(), hasLength(1));
      expect(c.messages.single.confirmed, isTrue);
      // Bo'sh va juda uzun xabar yuborilmaydi.
      await c.send('   ');
      await c.send('a' * (kSupportMaxBody + 1));
      expect(s.sends, hasLength(2));
    });

    test('rasm: optimistik, tasdiqdan keyin ham mahalliy baytlar; xatoda AYNI kalit', () async {
      final s = _FakeServer(kSideRestaurant)..failuresLeft = 1;
      final c = s.conversation();
      await c.load();
      await c.send('  ', image: SupportImageDraft(bytes: _png, filename: 'ekran.png'));
      final failed = c.messages.single;
      expect(failed.failed, isTrue);
      expect(failed.localImage, isNotNull);
      await c.retry(failed);
      expect(s.imageSends, hasLength(2));
      expect(s.imageSends.map((e) => e.$2).toSet(), hasLength(1));
      expect(s.imageSends.first.$1, '');
      expect(s.imageSends.first.$4, 'ekran.png');
      final sent = c.messages.single;
      expect(sent.confirmed, isTrue);
      expect(sent.attachment!.id, 'att-2');
      expect(sent.localImage, isNotNull, reason: 'server javobidan keyin qayta yuklanmaydi');
      // Rasm yuborish ulanmagan suhbatda va buzilgan faylda hech narsa ketmaydi.
      final noImages = s.conversation(images: false);
      await noImages.load();
      await noImages.send('x', image: SupportImageDraft(bytes: _png, filename: 'a.png'));
      await c.send('x', image: SupportImageDraft(bytes: Uint8List.fromList(utf8.encode('salom')), filename: 'a.png'));
      expect(s.imageSends, hasLength(2));
      expect(s.sends, isEmpty);
    });

    test('admin kanalida begona restoran hodisasi e\'tiborsiz', () async {
      final s = _FakeServer(kSideAdmin);
      final c = s.conversation(restaurantId: 'rest-a');
      await c.load();
      final foreign = _msg(99, kSideRestaurant, 'begona');
      expect(c.handleEvent({'type': 'support_message', 'restaurant_id': 'rest-b', 'message': foreign}), isFalse);
      expect(c.handleEvent({'type': 'order_status', 'restaurant_id': 'rest-a'}), isFalse);
      expect(c.messages, isEmpty);
      expect(c.handleEvent({'type': 'support_message', 'restaurant_id': 'rest-a', 'message': foreign}), isTrue);
      expect(c.messages.single.body, 'begona');
    });

    test('sahifa ko\'rinmasa o\'qilgan deb belgilanmaydi; qayta ulanishda to\'ldiradi', () async {
      final s = _FakeServer(kSideRestaurant);
      s.add(kSideRestaurant, 'savol');
      final c = s.conversation()..active = false;
      await c.load();
      final reply = s.add(kSideAdmin, 'javob');
      c.handleEvent({'type': 'support_message', 'message': reply, 'thread': s.thread});
      await Future<void>.delayed(Duration.zero);
      expect(s.reads, isEmpty);
      s.add(kSideAdmin, 'uzilish paytida 1');
      s.add(kSideAdmin, 'uzilish paytida 2');
      c.active = true;
      await c.catchUp();
      expect(c.messages.map((m) => m.body), ['savol', 'javob', 'uzilish paytida 1', 'uzilish paytida 2']);
      expect(s.fetches.last['after'], 2);
      expect(s.reads, [4]);
    });
  });

  group('chat ko\'rinishi', () {
    Future<(_FakeServer, SupportConversation)> pump(
      WidgetTester tester, {
      double width = 900,
      void Function(_FakeServer)? seed,
      SupportImagePicker? picker,
    }) async {
      tester.view.physicalSize = Size(width, 800);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.reset);
      final s = _FakeServer(kSideRestaurant);
      seed?.call(s);
      final c = s.conversation(pageSize: 50);
      await c.load();
      await tester.pumpWidget(MaterialApp(
        home: Scaffold(
          body: SupportChatView(
            conversation: c,
            onPickImage: picker,
            imageProvider: (_) => MemoryImage(_png),
          ),
        ),
      ));
      await tester.pumpAndSettle();
      return (s, c);
    }

    testWidgets('kun ajratgichi, belgilar va o\'qildi', (tester) async {
      final (s, _) = await pump(tester, seed: (s) {
        s.add(kSideRestaurant, 'Salom, printer ishlamayapti');
        s.add(kSideAdmin, 'Printer modelini yozing');
        s.add(kSideRestaurant, 'HP LaserJet');
        s.peerReadSeq = 1;
      });
      expect(find.text('12-sentabr, 2026'), findsOneWidget);
      expect(find.byKey(const ValueKey('support-read-m1')), findsOneWidget);
      expect(find.byKey(const ValueKey('support-sent-m3')), findsOneWidget);
      expect(s.reads, [3]);
      expect(find.byKey(const ValueKey('support-attach')), findsNothing, reason: 'tanlovchi berilmagan');
      expect(tester.takeException(), isNull);
    });

    testWidgets('Enter yuboradi, Shift+Enter yubormaydi, emoji qo\'shiladi', (tester) async {
      final (s, _) = await pump(tester);
      expect(find.text('Suhbatni boshlang'), findsOneWidget);
      final input = find.byKey(const ValueKey('support-input'));

      await tester.tap(input);
      await tester.enterText(input, 'Salom');
      await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
      await tester.pumpAndSettle();
      expect(s.sends, isEmpty);

      await tester.tap(find.byKey(const ValueKey('support-emoji')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('support-emoji-👍')));
      await tester.pumpAndSettle();
      expect(find.text('Salom👍'), findsOneWidget);

      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await tester.pumpAndSettle();
      expect(s.sends.single.first, 'Salom👍');
      expect(find.byKey(const ValueKey('support-msg-m1')), findsOneWidget);
      expect(tester.widget<TextField>(input).controller!.text, isEmpty);
    });

    testWidgets('rasm: tanlash, oldindan ko\'rish, izohsiz yuborish, to\'liq ekranda ochish', (tester) async {
      final (s, _) = await pump(tester, picker: () async => SupportImageDraft(bytes: _png, filename: 'ekran-rasmi.png'));
      await tester.tap(find.byKey(const ValueKey('support-attach')));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('support-draft')), findsOneWidget);
      expect(find.text('ekran-rasmi.png'), findsOneWidget);

      // Olib tashlash va qayta tanlash.
      await tester.tap(find.byKey(const ValueKey('support-draft-remove')));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('support-draft')), findsNothing);
      await tester.tap(find.byKey(const ValueKey('support-attach')));
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const ValueKey('support-send')));
      await tester.pumpAndSettle();
      expect(s.imageSends.single.$4, 'ekran-rasmi.png');
      expect(find.byKey(const ValueKey('support-draft')), findsNothing);
      expect(find.byKey(const ValueKey('support-image-m1')), findsOneWidget);

      await tester.tap(find.byKey(const ValueKey('support-image-m1')));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('support-image-viewer')), findsOneWidget);
      await tester.tap(find.byTooltip('Yopish'));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('support-image-viewer')), findsNothing);
      expect(tester.takeException(), isNull);
    });

    testWidgets('rasm muammolari foydalanuvchiga aytiladi', (tester) async {
      var result = SupportImageDraft(bytes: Uint8List.fromList(utf8.encode('<html>')), filename: 'rasm.png');
      Object? error;
      final (s, _) = await pump(tester, picker: () async {
        if (error != null) throw error;
        return result;
      });
      await tester.tap(find.byKey(const ValueKey('support-attach')));
      await tester.pumpAndSettle();
      expect(find.text('Fayl rasm emas yoki buzilgan'), findsOneWidget);
      expect(find.byKey(const ValueKey('support-draft')), findsNothing);

      // Oldingi xabar (snackbar) tugmani to'sib turmasin.
      ScaffoldMessenger.of(tester.element(find.byType(SupportChatView))).removeCurrentSnackBar();
      await tester.pumpAndSettle();

      error = const SupportImageException('Rasm 10 MB dan oshmasligi kerak');
      await tester.tap(find.byKey(const ValueKey('support-attach')));
      await tester.pumpAndSettle();
      expect(find.text('Rasm 10 MB dan oshmasligi kerak'), findsOneWidget);
      expect(s.imageSends, isEmpty);
      result = SupportImageDraft(bytes: _png, filename: 'ok.png');
    });

    testWidgets('serverdagi rasm nisbatda joy oladi, izoh ostida', (tester) async {
      await pump(tester, seed: (s) {
        s.add(kSideAdmin, 'Mana namuna',
            attachment: {'id': 'att-9', 'content_type': 'image/webp', 'width': 1600, 'height': 400, 'size': 90000});
      });
      final box = tester.getSize(find.byKey(const ValueKey('support-image-m1')));
      expect(box.width / box.height, closeTo(4, 0.05));
      expect(find.text('Mana namuna'), findsOneWidget);
      expect(tester.takeException(), isNull);
    });

    testWidgets('yuborilmagan xabar: qayta yuborish tugmasi', (tester) async {
      final (s, c) = await pump(tester);
      s.failuresLeft = 1;
      await c.send('Salom');
      await tester.pumpAndSettle();
      expect(find.text('Qayta yuborish'), findsOneWidget);
      await tester.tap(find.text('Qayta yuborish'));
      await tester.pumpAndSettle();
      expect(find.text('Qayta yuborish'), findsNothing);
      expect(find.byKey(const ValueKey('support-msg-m1')), findsOneWidget);
    });

    testWidgets('tor ekranda uzun xabar toshmaydi, hisoblagich chiqadi', (tester) async {
      await pump(tester, width: 340, picker: () async => null, seed: (s) {
        s.add(kSideAdmin, 'Juda uzun so\'z ${'a' * 400} va davomi');
        s.add(kSideRestaurant, 'qisqa',
            attachment: {'id': 'att-1', 'content_type': 'image/webp', 'width': 400, 'height': 1600, 'size': 1});
      });
      await tester.enterText(find.byKey(const ValueKey('support-input')), 'b' * (kSupportCounterFrom + 5));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('support-counter')), findsOneWidget);
      expect(tester.takeException(), isNull);
    });
  });
}

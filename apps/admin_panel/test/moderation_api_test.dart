import 'dart:async';
import 'dart:convert';

import 'package:chust_admin/ondexmap/moderation_api.dart';
import 'package:chust_admin/ondexmap/ondexmap_session.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

// OnDexMap moderatsiya API mijozi: token faqat sarlavhada, faqat loopback'ga,
// xatolar foydalanuvchiga tushunarli matn bilan.

const _id = '11111111-1111-4111-8111-111111111111';

http.Response _json(Object body, [int status = 200]) => http.Response.bytes(
      utf8.encode(jsonEncode(body)),
      status,
      headers: {'content-type': 'application/json; charset=utf-8'},
    );

ModerationApi _api({OndexMapSession? Function()? session, String url = 'http://127.0.0.1:8091'}) =>
    ModerationApi(
      defaultUrl: url,
      session: session ?? () => const OndexMapSession(token: 'sirli-token'),
    );

Future<T> _with<T>(
  http.Response Function(http.Request) handler,
  List<http.Request> log,
  Future<T> Function() body,
) =>
    http.runWithClient(
      body,
      () => MockClient((r) async {
        log.add(r);
        return handler(r);
      }),
    );

void main() {
  test('token FAQAT X-API-Key sarlavhasida, URL\'da emas; manzil loopback', () async {
    final log = <http.Request>[];
    await _with((r) => _json({'kinds': [], 'categories': [], 'max_photos': 4}), log,
        () => _api().meta());
    expect(log, hasLength(1));
    final r = log.single;
    expect(r.headers['X-API-Key'], 'sirli-token');
    expect(r.url.toString(), isNot(contains('sirli-token')));
    expect(r.url.host, '127.0.0.1');
    expect(r.url.path, '/api/places/meta');
  });

  test('sessiya manzili ustun; manzilga tashqi host qo\'yib bo\'lmaydi', () async {
    final log = <http.Request>[];
    // Sessiyadagi (server yozgan) loopback manzil ishlatiladi.
    await _with((r) => _json({'submissions': [], 'pending': 0}), log,
        () => _api(session: () => const OndexMapSession(token: 't', url: 'http://127.0.0.1:9000')).submissions('pending'));
    expect(log.single.url.port, 9000);

    // Tashqi manzil (fayl almashtirilgan bo'lsa) — SO'ROV UMUMAN YUBORILMAYDI.
    log.clear();
    await expectLater(
      _with((r) => _json({}), log,
          () => _api(session: () => const OndexMapSession(token: 't', url: 'http://evil.example')).meta()),
      throwsA(isA<ModerationException>()),
    );
    expect(log, isEmpty, reason: 'token tashqi serverga ketmasligi kerak');

    // Standart manzil ham tashqi bo'lsa — xuddi shunday.
    await expectLater(
      _with((r) => _json({}), log, () => _api(url: 'https://evil.example').meta()),
      throwsA(isA<ModerationException>()),
    );
    expect(log, isEmpty);
  });

  test('sessiya yo\'q → tushunarli xato, so\'rov yo\'q', () async {
    final log = <http.Request>[];
    await expectLater(
      _with((r) => _json({}), log, () => _api(session: () => null).meta()),
      throwsA(isA<ModerationException>().having((e) => e.message, 'message', contains('sessiya'))),
    );
    expect(log, isEmpty);
  });

  test('har so\'rovda sessiya QAYTA o\'qiladi (server qayta yonganda token almashadi)', () async {
    var token = 'eski';
    final log = <http.Request>[];
    final api = _api(session: () => OndexMapSession(token: token));
    await _with((r) => _json({'kinds': []}), log, () async {
      await api.meta();
      token = 'yangi';
      await api.meta();
    });
    expect(log.map((r) => r.headers['X-API-Key']), ['eski', 'yangi']);
  });

  test('401 → unauthorized, xabar tokenni ko\'rsatmaydi', () async {
    final log = <http.Request>[];
    try {
      await _with((r) => _json({'error': 'admin kaliti yaroqsiz'}, 401), log, () => _api().meta());
      fail('xato kutilgan edi');
    } on ModerationException catch (e) {
      expect(e.unauthorized, isTrue);
      expect(e.message, isNot(contains('sirli-token')));
    }
  });

  test('server xatosi matni ko\'rsatiladi; JSON emas bo\'lsa umumiy matn', () async {
    final log = <http.Request>[];
    await expectLater(
      _with((r) => _json({'error': '«telefon» noto\'g\'ri'}, 400), log, () => _api().meta()),
      throwsA(isA<ModerationException>().having((e) => e.message, 'm', '«telefon» noto\'g\'ri')),
    );
    await expectLater(
      _with((r) => http.Response('<html>500</html>', 500), log, () => _api().meta()),
      throwsA(isA<ModerationException>().having((e) => e.message, 'm', 'Xato 500')),
    );
  });

  test('ulanish xatosi → serverni ishga tushirish ko\'rsatmasi', () async {
    await expectLater(
      http.runWithClient(() => _api().meta(), () => MockClient((r) async => throw http.ClientException('refused'))),
      throwsA(isA<ModerationException>().having((e) => e.message, 'm', contains('go run ./cmd/admin'))),
    );
  });

  test('taklif ro\'yxati tahlil qilinadi; sarlavha nom → manzil → tur', () async {
    final log = <http.Request>[];
    final list = await _with(
      (r) => _json({
        'pending': 3,
        'submissions': [
          {'id': _id, 'kind': 'organization', 'kind_label': 'Tashkilot', 'name': 'Non', 'lat': 41.0, 'lng': 71.2, 'photos': 2, 'hint': 'abcdef012345'},
          {'id': _id, 'kind': 'address', 'kind_label': 'Manzil', 'street': 'Navoiy', 'house': '12', 'lat': 41, 'lng': 71},
          {'id': _id, 'kind': 'other', 'kind_label': 'Boshqa ob\'ekt', 'lat': 41, 'lng': 71},
        ],
      }),
      log,
      () => _api().submissions('pending'),
    );
    expect(list.pending, 3);
    expect(list.items.map((e) => e.title), ['Non', 'Navoiy 12', 'Boshqa ob\'ekt']);
    expect(list.items.first.photos, 2);
    expect(log.single.url.queryParameters['status'], 'pending');
  });

  test('tasdiqlash tanasi: id, edit, source, keep_photos', () async {
    final log = <http.Request>[];
    final placeId = await _with((r) => _json({'place_id': 'p-1'}), log, () => _api().approve(
          id: _id,
          edit: {'kind': 'other', 'lat': 41.0, 'lng': 71.2, 'description': 'Ko\'l'},
          keepPhotos: [0, 2],
        ));
    expect(placeId, 'p-1');
    final r = log.single;
    expect(r.method, 'POST');
    expect(r.url.path, '/api/submissions/approve');
    expect(r.headers['Content-Type'], contains('application/json'));
    final body = jsonDecode(r.body) as Map<String, dynamic>;
    expect(body['id'], _id);
    expect(body['source'], 'community');
    expect(body['keep_photos'], [0, 2]);
    expect(body['edit'], {'kind': 'other', 'lat': 41.0, 'lng': 71.2, 'description': 'Ko\'l'});
  });

  test('rasm: noto\'g\'ri id/o\'rin so\'rovsiz rad etiladi (yo\'l inyeksiyasi yo\'q)', () async {
    final log = <http.Request>[];
    for (final bad in ['../../etc/passwd', 'x', '$_id/../a', '']) {
      await expectLater(_with((r) => http.Response('x', 200), log, () => _api().submissionPhoto(bad, 0)),
          throwsA(isA<ModerationException>()));
    }
    await expectLater(_with((r) => http.Response('x', 200), log, () => _api().submissionPhoto(_id, 99)),
        throwsA(isA<ModerationException>()));
    expect(log, isEmpty);

    final bytes = await _with((r) => http.Response.bytes([1, 2, 3], 200), log, () => _api().submissionPhoto(_id, 1));
    expect(bytes, [1, 2, 3]);
    expect(log.single.url.path, '/api/submissions/$_id/photos/1');
  });

  test('so\'rov osilib qolsa vaqt tugaydi', () async {
    await expectLater(
      http.runWithClient(() => _api().meta().timeout(const Duration(seconds: 25)),
          () => MockClient((r) async => throw TimeoutException('t'))),
      throwsA(isA<ModerationException>().having((e) => e.message, 'm', contains('vaqtida'))),
    );
  });
}

import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:ondex_core/ondex_core.dart';

/// Mijoz sessiyasi (137): eski kirish endpointlari `X-Ondex-Session: v2`
/// bilan chaqiriladi; javobda refresh bo'lsa — markaziy sessiya, bo'lmasa
/// (eski server) — eski token, ilova ishlayveradi.
void main() {
  http.Response json(int code, Object body) => http.Response(
    jsonEncode(body),
    code,
    headers: {'content-type': 'application/json'},
  );

  test('v2 sarlavhasi + markaziy sessiya qabul qilinadi', () async {
    final seen = <String, String?>{};
    await http.runWithClient(
      () async {
        final api = ApiClient(baseUrl: 'http://api.test')
          ..wantCentralSession = true;
        final d = await api.verify('+998901234567', '123456');
        expect(seen['/auth/verify'], 'v2');
        expect(api.token, 'acc1');
        expect(api.refreshToken, 'ort_1');
        expect(api.isCentralSession, isTrue);
        expect(d['user'], isNotNull);
      },
      () => MockClient((req) async {
        seen[req.url.path] = req.headers['X-Ondex-Session'];
        return json(200, {
          'token': 'acc1',
          'access_token': 'acc1',
          'refresh_token': 'ort_1',
          'user': {'id': 'u1', 'role': 'customer'},
        });
      }),
    );
  });

  test(
    'eski server javobi (refresh yo\'q) — eski token, sessiya eski',
    () async {
      await http.runWithClient(
        () async {
          final api = ApiClient(baseUrl: 'http://api.test')
            ..wantCentralSession = true;
          await api.loginWithPassword('+998901234567', 'parol');
          expect(api.token, 'hs256');
          expect(api.refreshToken, isNull);
          expect(api.isCentralSession, isFalse);
        },
        () => MockClient(
          (req) async => json(200, {
            'token': 'hs256',
            'user': {'id': 'u1'},
          }),
        ),
      );
    },
  );

  test(
    'sarlavha faqat yoqilganda yuboriladi (boshqa ilovalar o\'zgarmaydi)',
    () async {
      String? got = 'yo\'q';
      await http.runWithClient(
        () async {
          final api = ApiClient(baseUrl: 'http://api.test');
          await api.verify('+998901234567', '1');
        },
        () => MockClient((req) async {
          got = req.headers['X-Ondex-Session'];
          return json(200, {'token': 't', 'user': {}});
        }),
      );
      expect(got, isNull);
    },
  );
}

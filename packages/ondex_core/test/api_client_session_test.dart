import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:ondex_core/ondex_core.dart';

/// Markaziy sessiya (A4a): kirish, 401 -> bitta refresh -> qayta yuborish,
/// parallel 401'lar BITTA refresh, rad etilgan refresh -> chiqish, tarmoq
/// xatosida sessiya saqlanadi.
void main() {
  http.Response json(int code, Object body) =>
      http.Response(jsonEncode(body), code, headers: {'content-type': 'application/json'});

  late int refreshCalls;
  late String validAccess;
  late Future<http.Response> Function(http.Request) refreshReply;

  MockClient server() => MockClient((req) async {
        switch (req.url.path) {
          case '/v2/auth/merchant/otp/verify':
            final b = jsonDecode(req.body) as Map;
            if (b['code'] == 'owner') {
              return json(200, {'mfa_required': true, 'mfa_token': 'omfa_x', 'method': 'totp'});
            }
            return json(200, {'access_token': 'a1', 'refresh_token': 'ort_1'});
          case '/v2/auth/token/refresh':
            refreshCalls++;
            await Future<void>.delayed(const Duration(milliseconds: 20));
            return refreshReply(req);
          case '/me':
            if (req.headers['Authorization'] == 'Bearer $validAccess') {
              return json(200, {'id': 'u1', 'role': 'waiter'});
            }
            return json(401, {'error': 'sessiya tugagan'});
        }
        return json(404, {'error': 'yoq'});
      });

  setUp(() {
    refreshCalls = 0;
    validAccess = 'a1';
    refreshReply = (_) async => json(200, {'access_token': 'a2', 'refresh_token': 'ort_2'});
  });

  Future<T> run<T>(Future<T> Function() body) => http.runWithClient(body, server);

  test('kirish access va refresh tokenni o\'rnatadi', () => run(() async {
        final api = ApiClient(baseUrl: 'http://x');
        await api.merchantVerify('+998900000000', '123456', clientId: 'ondex-pro');
        expect(api.token, 'a1');
        expect(api.refreshToken, 'ort_1');
        expect(api.isCentralSession, isTrue);
      }));

  test('egasi (mfa_required) — aniq xato, token o\'rnatilmaydi', () => run(() async {
        final api = ApiClient(baseUrl: 'http://x');
        await expectLater(
            api.merchantVerify('+998900000000', 'owner', clientId: 'ondex-pro'), throwsA(isA<ApiException>()));
        expect(api.token, isNull);
      }));

  test('401 -> refresh -> so\'rov qayta yuboriladi', () => run(() async {
        final api = ApiClient(baseUrl: 'http://x');
        await api.merchantVerify('+998900000000', '1', clientId: 'ondex-go');
        validAccess = 'a2'; // a1 eskirdi
        final me = await api.send('GET', '/me');
        expect(me['id'], 'u1');
        expect(api.token, 'a2');
        expect(api.refreshToken, 'ort_2');
        expect(refreshCalls, 1);
      }));

  test('parallel 401 lar BITTA refresh ni kutadi', () => run(() async {
        final api = ApiClient(baseUrl: 'http://x');
        await api.merchantVerify('+998900000000', '1', clientId: 'ondex-go');
        validAccess = 'a2';
        final res = await Future.wait(List.generate(5, (_) => api.send('GET', '/me')));
        expect(res.every((m) => m['id'] == 'u1'), isTrue);
        expect(refreshCalls, 1);
      }));

  test('refresh rad etilsa — sessiya o\'chadi, onUnauthorized chaqiriladi', () => run(() async {
        final api = ApiClient(baseUrl: 'http://x');
        var lost = 0;
        api.onUnauthorized = () => lost++;
        await api.merchantVerify('+998900000000', '1', clientId: 'ondex-pro');
        validAccess = 'boshqa';
        refreshReply = (_) async => json(401, {'error': 'sessiya xavfsizlik sababli yopildi'});
        await expectLater(api.send('GET', '/me'),
            throwsA(isA<ApiException>().having((e) => e.isUnauthorized, 'isUnauthorized', isTrue)));
        expect(lost, 1);
        expect(api.refreshToken, isNull);
        // access token ilova handleri uchun qoladi ("token null" = login xatosi)
        expect(api.token, isNotNull);
      }));

  test('refresh paytida tarmoq xatosi — sessiya SAQLANADI, chiqarilmaydi', () => run(() async {
        final api = ApiClient(baseUrl: 'http://x');
        var lost = 0;
        api.onUnauthorized = () => lost++;
        await api.merchantVerify('+998900000000', '1', clientId: 'ondex-pro');
        validAccess = 'boshqa';
        refreshReply = (_) async => throw http.ClientException('tarmoq yo\'q');
        await expectLater(api.send('GET', '/me'),
            throwsA(isA<ApiException>().having((e) => e.isNetwork, 'isNetwork', isTrue)));
        expect(lost, 0);
        expect(api.refreshToken, 'ort_1');
      }));

  test('eski (HS256) sessiyada refresh urinilmaydi', () => run(() async {
        final api = ApiClient(baseUrl: 'http://x')..token = 'eski';
        var lost = 0;
        api.onUnauthorized = () => lost++;
        await expectLater(api.send('GET', '/me'), throwsA(isA<ApiException>()));
        expect(refreshCalls, 0);
        expect(lost, 1);
      }));
}

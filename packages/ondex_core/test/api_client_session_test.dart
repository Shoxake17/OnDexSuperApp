import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:ondex_core/ondex_core.dart';

/// Markaziy sessiya (A4a): kirish, 401 -> bitta refresh -> qayta yuborish,
/// parallel 401'lar BITTA refresh, rad etilgan refresh -> chiqish, tarmoq
/// xatosida sessiya saqlanadi.
void main() {
  http.Response json(int code, Object body) => http.Response(
    jsonEncode(body),
    code,
    headers: {'content-type': 'application/json'},
  );

  late int refreshCalls;
  late String validAccess;
  late Future<http.Response> Function(http.Request) refreshReply;

  MockClient server() => MockClient((req) async {
    switch (req.url.path) {
      case '/v2/auth/merchant/otp/verify':
        final b = jsonDecode(req.body) as Map;
        if (b['code'] == 'owner') {
          return json(200, {
            'mfa_required': true,
            'mfa_token': 'omfa_x',
            'method': 'totp',
          });
        }
        return json(200, {'access_token': 'a1', 'refresh_token': 'ort_1'});
      case '/v2/auth/merchant/password':
        final b = jsonDecode(req.body) as Map;
        if (b['password'] != 'togri')
          return json(401, {'error': 'login yoki parol noto\'g\'ri'});
        return json(200, {
          'mfa_required': true,
          'mfa_token': 'omfa_1',
          'method': 'otp',
        });
      case '/v2/auth/mfa/factor':
        return json(200, {
          'mfa_required': true,
          'mfa_token': 'omfa_2',
          'method': 'totp_enroll',
        });
      case '/v2/auth/mfa/totp/enroll':
        return json(200, {
          'secret': 'ABCDEF',
          'otpauth_uri': 'otpauth://totp/OnDex:x?secret=ABCDEF',
        });
      case '/v2/auth/mfa/totp/confirm':
        final b = jsonDecode(req.body) as Map;
        if (b['code'] != '123456')
          return json(401, {'error': 'tasdiqlash kodi noto\'g\'ri'});
        return json(200, {
          'tokens': {'access_token': 'aE', 'refresh_token': 'ort_E'},
          'recovery_codes': List.generate(10, (i) => 'AAAAA-BBBB$i'),
        });
      case '/v2/auth/mfa/verify':
        return json(200, {'access_token': 'aT', 'refresh_token': 'ort_T'});
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
    refreshReply = (_) async =>
        json(200, {'access_token': 'a2', 'refresh_token': 'ort_2'});
  });

  Future<T> run<T>(Future<T> Function() body) =>
      http.runWithClient(body, server);

  test(
    'kirish access va refresh tokenni o\'rnatadi',
    () => run(() async {
      final api = ApiClient(baseUrl: 'http://x');
      await api.merchantVerify(
        '+998900000000',
        '123456',
        clientId: 'ondex-pro',
      );
      expect(api.token, 'a1');
      expect(api.refreshToken, 'ort_1');
      expect(api.isCentralSession, isTrue);
    }),
  );

  test(
    'egasi (mfa_required) — aniq xato, token o\'rnatilmaydi',
    () => run(() async {
      final api = ApiClient(baseUrl: 'http://x');
      await expectLater(
        api.merchantVerify('+998900000000', 'owner', clientId: 'ondex-pro'),
        throwsA(isA<ApiException>()),
      );
      expect(api.token, isNull);
    }),
  );

  test(
    '401 -> refresh -> so\'rov qayta yuboriladi',
    () => run(() async {
      final api = ApiClient(baseUrl: 'http://x');
      await api.merchantVerify('+998900000000', '1', clientId: 'ondex-go');
      validAccess = 'a2'; // a1 eskirdi
      final me = await api.send('GET', '/me');
      expect(me['id'], 'u1');
      expect(api.token, 'a2');
      expect(api.refreshToken, 'ort_2');
      expect(refreshCalls, 1);
    }),
  );

  test(
    'parallel 401 lar BITTA refresh ni kutadi',
    () => run(() async {
      final api = ApiClient(baseUrl: 'http://x');
      await api.merchantVerify('+998900000000', '1', clientId: 'ondex-go');
      validAccess = 'a2';
      final res = await Future.wait(
        List.generate(5, (_) => api.send('GET', '/me')),
      );
      expect(res.every((m) => m['id'] == 'u1'), isTrue);
      expect(refreshCalls, 1);
    }),
  );

  test(
    'refresh rad etilsa — sessiya o\'chadi, onUnauthorized chaqiriladi',
    () => run(() async {
      final api = ApiClient(baseUrl: 'http://x');
      var lost = 0;
      api.onUnauthorized = () => lost++;
      await api.merchantVerify('+998900000000', '1', clientId: 'ondex-pro');
      validAccess = 'boshqa';
      refreshReply = (_) async =>
          json(401, {'error': 'sessiya xavfsizlik sababli yopildi'});
      await expectLater(
        api.send('GET', '/me'),
        throwsA(
          isA<ApiException>().having(
            (e) => e.isUnauthorized,
            'isUnauthorized',
            isTrue,
          ),
        ),
      );
      expect(lost, 1);
      expect(api.refreshToken, isNull);
      // access token ilova handleri uchun qoladi ("token null" = login xatosi)
      expect(api.token, isNotNull);
    }),
  );

  test(
    'refresh paytida tarmoq xatosi — sessiya SAQLANADI, chiqarilmaydi',
    () => run(() async {
      final api = ApiClient(baseUrl: 'http://x');
      var lost = 0;
      api.onUnauthorized = () => lost++;
      await api.merchantVerify('+998900000000', '1', clientId: 'ondex-pro');
      validAccess = 'boshqa';
      refreshReply = (_) async => throw http.ClientException('tarmoq yo\'q');
      await expectLater(
        api.send('GET', '/me'),
        throwsA(
          isA<ApiException>().having((e) => e.isNetwork, 'isNetwork', isTrue),
        ),
      );
      expect(lost, 0);
      expect(api.refreshToken, 'ort_1');
    }),
  );

  test(
    'egasi: parol -> kod -> TOTP ulash -> sessiya + 10 tiklash kodi',
    () => run(() async {
      final api = ApiClient(baseUrl: 'http://x');
      var lost = 0;
      api.onUnauthorized = () => lost++;
      await expectLater(
        api.merchantLoginPassword(
          '+998900000000',
          'xato',
          clientId: 'merchant-desktop',
        ),
        throwsA(isA<ApiException>().having((e) => e.statusCode, 'status', 401)),
      );
      expect(
        lost,
        0,
        reason:
            "noto'g'ri parol sessiya tugashi emas — onUnauthorized chaqirilmaydi",
      );
      var s = await api.merchantLoginPassword(
        '+998900000000',
        'togri',
        clientId: 'merchant-desktop',
      );
      expect(s.done, isFalse);
      expect(s.method, 'otp');
      expect(api.token, isNull);
      s = await api.mfaFactor(s.mfaToken!, code: '424242');
      expect(s.method, 'totp_enroll');
      final en = await api.mfaEnroll(s.mfaToken!);
      expect(en.uri, startsWith('otpauth://totp/'));
      await expectLater(
        api.mfaConfirm(s.mfaToken!, '000000'),
        throwsA(isA<ApiException>()),
      );
      expect(lost, 0);
      final codes = await api.mfaConfirm(s.mfaToken!, '123456');
      expect(codes, hasLength(10));
      expect(api.token, 'aE');
      expect(api.refreshToken, 'ort_E');
    }),
  );

  test(
    'egasi: TOTP bilan kirish sessiyani o\'rnatadi',
    () => run(() async {
      final api = ApiClient(baseUrl: 'http://x');
      final s = await api.mfaVerify('omfa_t', '654321');
      expect(s.done, isTrue);
      expect(api.token, 'aT');
      expect(api.isCentralSession, isTrue);
    }),
  );

  test(
    'eski (HS256) sessiyada refresh urinilmaydi',
    () => run(() async {
      final api = ApiClient(baseUrl: 'http://x')..token = 'eski';
      var lost = 0;
      api.onUnauthorized = () => lost++;
      await expectLater(api.send('GET', '/me'), throwsA(isA<ApiException>()));
      expect(refreshCalls, 0);
      expect(lost, 1);
    }),
  );
}

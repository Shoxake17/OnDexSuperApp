import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:ondex_core/ondex_core.dart';
import 'package:passkeys/types.dart';

/// PK-3/PK-4: ilova ichidan passkey — kirish (telefon raqamisiz), qo'shish
/// (step-up kodi bilan), server holati (state) o'zgarmasdan qaytadi, plagin
/// xatolari tushunarli xabarga aylanadi, xatoda sessiya o'rnatilmaydi.
class _FakePlatform implements PasskeyPlatform {
  Map<String, dynamic>? lastOptions;
  Object? fail;

  @override
  Future<bool> isSupported() async => true;

  @override
  Future<Map<String, dynamic>> authenticate(
    Map<String, dynamic> options,
  ) async {
    lastOptions = options;
    if (fail != null) throw fail!;
    return {
      'id': 'cred1',
      'rawId': 'cred1',
      'type': 'public-key',
      'response': {
        'clientDataJSON': 'cd',
        'authenticatorData': 'ad',
        'signature': 'sig',
        'userHandle': 'dTE',
      },
      'clientExtensionResults': <String, dynamic>{},
    };
  }

  @override
  Future<Map<String, dynamic>> register(Map<String, dynamic> options) async {
    lastOptions = options;
    if (fail != null) throw fail!;
    return {
      'id': 'cred2',
      'rawId': 'cred2',
      'type': 'public-key',
      'response': {
        'clientDataJSON': 'cd',
        'attestationObject': 'ao',
        'transports': ['internal'],
      },
    };
  }
}

void main() {
  http.Response json(int code, Object body) => http.Response(
    jsonEncode(body),
    code,
    headers: {'content-type': 'application/json'},
  );

  late List<Map<String, dynamic>> calls;

  MockClient server() => MockClient((req) async {
    final body = req.body.isEmpty
        ? <String, dynamic>{}
        : jsonDecode(req.body) as Map<String, dynamic>;
    calls.add({
      'path': req.url.path,
      'auth': req.headers['Authorization'],
      ...body,
    });
    switch (req.url.path) {
      case '/v2/auth/passkey/options':
        return json(200, {
          'public_key': {
            'challenge': 'Y2g',
            'rpId': 'ondex.uz',
            'userVerification': 'required',
          },
          'state': 'pks1.imzolangan',
        });
      case '/v2/auth/passkey/login':
        if (body['state'] != 'pks1.imzolangan')
          return json(400, {'error': 'holat'});
        return json(200, {'access_token': 'a1', 'refresh_token': 'ort_1'});
      case '/v2/me/passkeys/options':
        if (body['code'] != '123456')
          return json(400, {'error': "kod noto'g'ri"});
        return json(200, {
          'public_key': {
            'challenge': 'Y2g',
            'rp': {'id': 'ondex.uz', 'name': 'OnDex'},
          },
          'state': 'pks1.reg',
        });
      case '/v2/me/passkeys':
        return json(201, {'id': 'cred2'});
    }
    return json(404, {'error': 'yo\'q'});
  });

  setUp(() => calls = []);
  Future<T> run<T>(Future<T> Function() body) =>
      http.runWithClient(body, server);

  test(
    'kirish: options -> plagin -> login; sessiya o\'rnatiladi',
    () => run(() async {
      final api = ApiClient(baseUrl: 'http://x');
      final p = _FakePlatform();
      final step = await OnDexPasskeys(
        api,
        platform: p,
      ).signIn(clientId: 'merchant-desktop', deviceName: 'Kassa');
      expect(step.done, isTrue);
      expect(api.token, 'a1');
      expect(api.refreshToken, 'ort_1');
      expect(p.lastOptions?['rpId'], 'ondex.uz');
      final login = calls.last;
      expect(login['path'], '/v2/auth/passkey/login');
      expect(login['client_id'], 'merchant-desktop');
      expect(login['state'], 'pks1.imzolangan');
      expect((login['credential'] as Map)['type'], 'public-key');
      expect(login['device_name'], 'Kassa');
    }),
  );

  test(
    'bekor qilinsa: aniq xato, sessiya YO\'Q, login chaqirilmaydi',
    () => run(() async {
      final api = ApiClient(baseUrl: 'http://x');
      final p = _FakePlatform()..fail = PasskeyAuthCancelledException('user');
      await expectLater(
        OnDexPasskeys(api, platform: p).signIn(clientId: 'ondex-pro'),
        throwsA(
          isA<PasskeyException>().having(
            (e) => e.reason,
            'reason',
            PasskeyFailure.cancelled,
          ),
        ),
      );
      expect(api.token, isNull);
      expect(
        calls.where((c) => c['path'] == '/v2/auth/passkey/login'),
        isEmpty,
      );
    }),
  );

  test(
    'qurilmada passkey yo\'q / domen bog\'lanmagan — alohida sabab',
    () => run(() async {
      final api = ApiClient(baseUrl: 'http://x');
      for (final (err, want) in [
        (NoCredentialsAvailableException(), PasskeyFailure.noCredential),
        (DomainNotAssociatedException('x'), PasskeyFailure.notAssociated),
        (StateError('nimadir'), PasskeyFailure.other),
      ]) {
        final p = _FakePlatform()..fail = err;
        await expectLater(
          OnDexPasskeys(api, platform: p).signIn(clientId: 'ondex-go'),
          throwsA(
            isA<PasskeyException>().having((e) => e.reason, 'reason', want),
          ),
        );
      }
    }),
  );

  test(
    'qo\'shish: kod bilan options -> plagin -> saqlash (Bearer)',
    () => run(() async {
      final api = ApiClient(baseUrl: 'http://x')..token = 'acc';
      final p = _FakePlatform();
      await OnDexPasskeys(
        api,
        platform: p,
      ).register(code: '123456', name: ' Kassa kompyuteri ');
      final save = calls.last;
      expect(save['path'], '/v2/me/passkeys');
      expect(save['state'], 'pks1.reg');
      expect(save['name'], 'Kassa kompyuteri');
      expect(save['auth'], 'Bearer acc');
      expect((p.lastOptions?['rp'] as Map)['id'], 'ondex.uz');
    }),
  );

  test(
    'qo\'shish: noto\'g\'ri kod — server xatosi, plagin chaqirilmaydi',
    () => run(() async {
      final api = ApiClient(baseUrl: 'http://x')..token = 'acc';
      final p = _FakePlatform();
      await expectLater(
        OnDexPasskeys(api, platform: p).register(code: '000000'),
        throwsA(isA<ApiException>().having((e) => e.statusCode, 'status', 400)),
      );
      expect(p.lastOptions, isNull);
    }),
  );
}

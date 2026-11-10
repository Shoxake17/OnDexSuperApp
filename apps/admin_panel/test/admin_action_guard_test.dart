import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:ondex_admin/api.dart';

/// CC-0.4: xavfli amal — server sabab, so'ng Authenticator kodini so'raydi;
/// noto'g'ri kodda oyna xato bilan qayta ochiladi; bekor qilinsa so'rov
/// boshqa yuborilmaydi.
void main() {
  http.Response json(int code, Object body) =>
      http.Response(jsonEncode(body), code,
          headers: {'content-type': 'application/json'});

  MockClient server(List<Map<String, String>> seen) => MockClient((req) async {
        seen.add({
          'reason': req.headers['X-OnDex-Reason'] ?? '',
          'code': req.headers['X-OnDex-StepUp'] ?? '',
        });
        final reason =
            Uri.decodeQueryComponent(req.headers['X-OnDex-Reason'] ?? '');
        final code = req.headers['X-OnDex-StepUp'] ?? '';
        if (reason.length < 3) {
          return json(428, {'error': 'sabab', 'reason_required': true});
        }
        if (code.isEmpty) {
          return json(428, {'error': 'kod', 'step_up_required': true});
        }
        if (code != '123456') {
          return json(403, {
            'error': 'Authenticator kodi noto\'g\'ri',
            'step_up_invalid': true
          });
        }
        return json(200, {'ok': true});
      });

  test('sabab -> kod -> noto\'g\'ri kod -> to\'g\'ri kod', () async {
    final seen = <Map<String, String>>[];
    final prompts = <String>[];
    final codes = ['000000', '123456'];
    await http.runWithClient(() async {
      final api = AdminApi()
        ..token = 'aT'
        ..actionPrompt =
            ({required needReason, required needCode, error}) async {
          prompts.add('r=$needReason c=$needCode e=${error ?? ''}');
          return (
            reason: needReason ? 'dublikat restoran' : '',
            code: needCode ? codes.removeAt(0) : '',
          );
        };
      await api.deleteStore('s1');
    }, () => server(seen));
    expect(prompts, [
      'r=true c=false e=',
      'r=false c=true e=',
      'r=false c=true e=Authenticator kodi noto\'g\'ri',
    ]);
    expect(seen.length, 4);
    expect(Uri.decodeQueryComponent(seen.last['reason']!), 'dublikat restoran');
    expect(seen.last['code'], '123456');
  });

  test('bekor qilinsa — xato, boshqa so\'rov yo\'q', () async {
    final seen = <Map<String, String>>[];
    await http.runWithClient(() async {
      final api = AdminApi()
        ..token = 'aT'
        ..actionPrompt =
            ({required needReason, required needCode, error}) async => null;
      await expectLater(api.deleteUser('u1'), throwsA(isA<ApiException>()));
    }, () => server(seen));
    expect(seen.length, 1);
  });
}

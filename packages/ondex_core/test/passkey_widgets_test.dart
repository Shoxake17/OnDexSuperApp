import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:ondex_core/ondex_core.dart';
import 'package:passkeys/types.dart';

class _Platform implements PasskeyPlatform {
  _Platform({this.supported = true});
  final bool supported;
  Object? fail;
  int calls = 0;

  @override
  Future<bool> isSupported() async => supported;

  @override
  Future<Map<String, dynamic>> authenticate(Map<String, dynamic> o) async {
    calls++;
    if (fail != null) throw fail!;
    return {'id': 'c', 'rawId': 'c', 'type': 'public-key', 'response': <String, dynamic>{}};
  }

  @override
  Future<Map<String, dynamic>> register(Map<String, dynamic> o) async {
    calls++;
    if (fail != null) throw fail!;
    return {'id': 'n', 'rawId': 'n', 'type': 'public-key', 'response': <String, dynamic>{}};
  }
}

class _Api extends ApiClient {
  _Api() : super(baseUrl: 'http://x');
  final passkeys = <Map<String, dynamic>>[];
  String? lastCode;

  @override
  Future<List<Map<String, dynamic>>> myPasskeys() async => List.of(passkeys);

  @override
  Future<PasskeyOptions> passkeyOptions(String clientId) async => _opts();

  @override
  Future<MerchantStep> passkeyLogin({
    required String clientId,
    required String state,
    required Map<String, dynamic> credential,
    String? deviceName,
  }) async {
    token = 'a1';
    return MerchantStep.doneForTest({});
  }

  @override
  Future<PasskeyOptions> passkeyRegisterOptions(String code) async {
    lastCode = code;
    if (code != '123456') throw ApiException("kod noto'g'ri", statusCode: 400);
    return _opts();
  }

  @override
  Future<void> passkeyRegister({required String state, required Map<String, dynamic> credential, String? name}) async {
    passkeys.add({'id': 'n', 'name': name, 'synced': false});
  }

  PasskeyOptions _opts() => PasskeyOptions.fromJson({'public_key': <String, dynamic>{}, 'state': 's'});
}

Widget _wrap(Widget w) => MaterialApp(home: Scaffold(body: SingleChildScrollView(child: w)));

void main() {
  testWidgets('qo\'llamaydigan qurilmada kirish tugmasi KO\'RINMAYDI', (t) async {
    await t.pumpWidget(_wrap(PasskeyLoginButton(
      passkeys: OnDexPasskeys(_Api(), platform: _Platform(supported: false)),
      clientId: 'merchant-desktop',
      onSignedIn: (_) async {},
      onError: (_) {},
    )));
    await t.pumpAndSettle();
    expect(find.byKey(const Key('passkey-login')), findsNothing);
  });

  testWidgets('kirish: bosiladi -> onSignedIn; bekor qilish xato emas', (t) async {
    final api = _Api();
    final p = _Platform();
    var signed = 0;
    final errors = <String>[];
    await t.pumpWidget(_wrap(PasskeyLoginButton(
      passkeys: OnDexPasskeys(api, platform: p),
      clientId: 'merchant-desktop',
      onSignedIn: (_) async => signed++,
      onError: errors.add,
    )));
    await t.pumpAndSettle();
    await t.tap(find.byKey(const Key('passkey-login')));
    await t.pumpAndSettle();
    expect(signed, 1);
    expect(api.token, 'a1');

    p.fail = PasskeyAuthCancelledException('x');
    await t.tap(find.byKey(const Key('passkey-login')));
    await t.pumpAndSettle();
    expect(signed, 1);
    expect(errors, isEmpty);
  });

  testWidgets('karta: noto\'g\'ri kod xato ko\'rsatadi; to\'g\'ri kod bilan qo\'shiladi', (t) async {
    final api = _Api();
    await t.pumpWidget(_wrap(PasskeySettingsCard(passkeys: OnDexPasskeys(api, platform: _Platform()))));
    await t.pumpAndSettle();
    expect(find.text("Hozircha passkey yo'q"), findsOneWidget);

    await t.tap(find.byKey(const Key('passkey-add')));
    await t.pumpAndSettle();
    await t.enterText(find.byKey(const Key('passkey-code')), '000000');
    await t.tap(find.text('Tasdiqlash'));
    await t.pumpAndSettle();
    expect(find.text("kod noto'g'ri"), findsOneWidget);
    expect(api.passkeys, isEmpty);

    await t.tap(find.byKey(const Key('passkey-add')));
    await t.pumpAndSettle();
    await t.enterText(find.byKey(const Key('passkey-code')), '123456');
    await t.tap(find.text('Tasdiqlash'));
    await t.pumpAndSettle();
    expect(api.passkeys, hasLength(1));
    expect(find.text('Ushbu qurilma'), findsOneWidget);
  });
}

import 'package:flutter/foundation.dart';
import 'package:passkeys/authenticator.dart';
import 'package:passkeys/types.dart';

import 'api_client.dart';

/// OnDex passkey — ilova ICHIDAN (Windows Hello, Android Credential Manager,
/// iOS) kirish va passkey qo'shish (PK-3/PK-4, Plane 320/321).
///
/// ┌─ XAVFSIZLIK ─────────────────────────────────────────────────────────┐
/// * Kalit qurilmada qoladi; serverga faqat imzo boradi.
/// * RP ID = `ondex.uz`: Android/iOS passkey'ni faqat domen RASMAN tan
///   olgan ilovaga beradi (assetlinks.json / apple-app-site-association) —
///   soxta ilova uni ololmaydi; server Android imzo xeshini ham tekshiradi.
/// * Kirish: telefon raqami so'ralmaydi (discoverable kalit), UV (PIN /
///   biometrik) majburiy. Qo'shish: TOTP yoki Telegram kodi (step-up).
/// * Hech qanday kalit/token bu sinfda saqlanmaydi — sessiya `ApiClient`
///   orqali `TokenStore` (Keystore/Keychain/DPAPI) ga yoziladi.
/// └──────────────────────────────────────────────────────────────────────┘
class OnDexPasskeys {
  OnDexPasskeys(this.api, {PasskeyPlatform? platform})
    : _platform = platform ?? _PluginPlatform();

  final ApiClient api;
  final PasskeyPlatform _platform;

  /// Qurilma passkey'ni qo'llaydimi (Windows 10 1903+, Android 9+, iOS 16+).
  Future<bool> isSupported() async {
    try {
      return await _platform.isSupported();
    } catch (_) {
      return false;
    }
  }

  /// Passkey bilan kirish. Muvaffaqiyatda sessiya `api` da o'rnatiladi.
  /// [clientId] — `merchant-desktop`, `admin-web`, `ondex-pro`, `ondex-go`,
  /// `ondex-customer`.
  Future<MerchantStep> signIn({
    required String clientId,
    String? deviceName,
  }) async {
    final o = await api.passkeyOptions(clientId);
    final cred = await _guard(() => _platform.authenticate(o.publicKey));
    return api.passkeyLogin(
      clientId: clientId,
      state: o.state,
      credential: cred,
      deviceName: deviceName,
    );
  }

  /// Yangi passkey qo'shish (joriy sessiya bilan). [code] — Authenticator
  /// (TOTP) yoki Telegram kodi; server qaysi biri kerakligini aytadi.
  Future<void> register({required String code, String? name}) async {
    final o = await api.passkeyRegisterOptions(code);
    final cred = await _guard(() => _platform.register(o.publicKey));
    await api.passkeyRegister(state: o.state, credential: cred, name: name);
  }

  /// Plagin xatolarini foydalanuvchiga tushunarli xabarga aylantiradi.
  Future<Map<String, dynamic>> _guard(
    Future<Map<String, dynamic>> Function() run,
  ) async {
    try {
      return await run();
    } on PasskeyAuthCancelledException {
      throw PasskeyException.cancelled();
    } on NoCredentialsAvailableException {
      throw PasskeyException(
        "Bu qurilmada OnDex passkey'i topilmadi. Telefon kodi bilan kirib, "
        "sozlamalarda passkey qo'shing.",
        reason: PasskeyFailure.noCredential,
      );
    } on ExcludeCredentialsCanNotBeRegisteredException {
      throw PasskeyException(
        "Bu qurilmadagi passkey allaqachon ulangan.",
        reason: PasskeyFailure.alreadyRegistered,
      );
    } on DomainNotAssociatedException {
      throw PasskeyException(
        "Ilova OnDex domeniga bog'lanmagan — ilovani rasmiy do'kondan yangilang.",
        reason: PasskeyFailure.notAssociated,
      );
    } on DeviceNotSupportedException {
      throw PasskeyException.unsupported();
    } on PasskeyUnsupportedException {
      throw PasskeyException.unsupported();
    } on TimeoutException {
      throw PasskeyException(
        'Passkey tasdiqlanmadi (vaqt tugadi). Qaytadan urinib ko\'ring.',
        reason: PasskeyFailure.cancelled,
      );
    } on ApiException {
      rethrow;
    } catch (_) {
      throw PasskeyException(
        'Passkey ishlamadi — telefon kodi bilan kiring.',
        reason: PasskeyFailure.other,
      );
    }
  }
}

/// Passkey xatosi sababi (UI qaror qabul qilishi uchun — matnga qaramang).
enum PasskeyFailure {
  cancelled,
  noCredential,
  alreadyRegistered,
  notAssociated,
  unsupported,
  other,
}

class PasskeyException implements Exception {
  PasskeyException(this.message, {required this.reason});

  PasskeyException.cancelled()
    : message = 'Passkey tasdiqlanmadi yoki bekor qilindi.',
      reason = PasskeyFailure.cancelled;

  PasskeyException.unsupported()
    : message =
          "Bu qurilma passkey'ni qo'llamaydi — telefon kodi bilan kiring.",
      reason = PasskeyFailure.unsupported;

  final String message;
  final PasskeyFailure reason;

  @override
  String toString() => message;
}

/// Platforma qatlami (testda soxtasi ulanadi). Kirish/chiqish — standart
/// WebAuthn JSON (`PublicKeyCredential*Options` / javob).
abstract class PasskeyPlatform {
  Future<bool> isSupported();
  Future<Map<String, dynamic>> authenticate(Map<String, dynamic> options);
  Future<Map<String, dynamic>> register(Map<String, dynamic> options);
}

class _PluginPlatform implements PasskeyPlatform {
  final _auth = PasskeyAuthenticator();

  @override
  Future<bool> isSupported() async {
    final a = _auth.getAvailability();
    final AvailabilityType? t = kIsWeb
        ? await a.web()
        : switch (defaultTargetPlatform) {
            TargetPlatform.android => await a.android(),
            TargetPlatform.windows => await a.windows(),
            TargetPlatform.iOS || TargetPlatform.macOS => await a.iOS(),
            _ => null,
          };
    return t?.hasPasskeySupport ?? false;
  }

  @override
  Future<Map<String, dynamic>> authenticate(
    Map<String, dynamic> options,
  ) async {
    // preferImmediatelyAvailableCredentials=false: qurilmada passkey bo'lmasa
    // ham telefon orqali (QR, "boshqa qurilma") kirish taklif qilinadi.
    final req = AuthenticateRequestType.fromJson(
      options,
      preferImmediatelyAvailableCredentials: false,
    );
    final res = await _auth.authenticate(req);
    return res.toJson();
  }

  @override
  Future<Map<String, dynamic>> register(Map<String, dynamic> options) async {
    final res = await _auth.register(RegisterRequestType.fromJson(options));
    return res.toJson();
  }
}

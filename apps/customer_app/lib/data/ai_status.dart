import 'package:flutter/foundation.dart';

import '../api.dart';

/// Shaddiy yordamchisi hozir ishlaydimi.
enum AiAvailability {
  /// Hali tekshirilmagan (ilova endi ochildi).
  unknown,

  /// `GET /ai/status` → `enabled: true`.
  on,

  /// Server yordamchini O'CHIRIB qo'ygan (`enabled: false`).
  off,

  /// Javob umuman kelmadi — tarmoq, server yoki sessiya muammosi.
  unreachable,
}

/// Yordamchi holati — BUTUN ilova uchun bitta nusxa.
///
/// ┌─ NEGA TUGMA ENDI HECH QACHON YO'QOLMAYDI ─────────────────────────┐
/// Avval `home_shell` va `catalog_screen` har biri o'zicha
/// `api.aiEnabled()` chaqirib, javob `false` yoki XATO bo'lsa tugmani
/// umuman chizmasdi. Natijasi yomon edi:
///
///   * API server o'chiq bo'lsa, tarmoq uzilsa yoki token eskirsa —
///     so'rov xato beradi va Shaddiy tugmasi JIMGINA yo'qolardi;
///   * foydalanuvchi uchun bu "funksiya o'chib qoldi" bo'lib
///     ko'rinardi, sababi esa hech qayerda aytilmasdi;
///   * ikki ekran ikki marta so'rov yuborardi va ular bir-biriga mos
///     kelmasligi ham mumkin edi (biri tugmani ko'rsatib, ikkinchisi
///     yashirib turardi).
///
/// Endi qoida BOSHQACHA: tugma HAR DOIM chiziladi. Yordamchi
/// ishlamasa, bosilganda SABAB aytiladi ([reason]) va "Qayta urinish"
/// taklif qilinadi. Ya'ni nosozlik ko'rinmas bo'lib qolmaydi.
/// └───────────────────────────────────────────────────────────────────┘
class AiStatus extends ChangeNotifier {
  AiStatus._();

  static final AiStatus instance = AiStatus._();

  AiAvailability _state = AiAvailability.unknown;
  AiAvailability get state => _state;

  /// Ishlamayotgan bo'lsa — sababning O'ZI (foydalanuvchiga
  /// ko'rsatiladigan matn). Ishlayotgan holatda `null`.
  String? _reason;
  String? get reason => _reason;

  bool get isOn => _state == AiAvailability.on;

  /// OVOZLI rejim (Gemini Live) serverda yoqilganmi.
  ///
  /// Matnli chatdan MUSTAQIL: `GEMINI_API_KEY` qo'yilmagan serverda
  /// `isOn == true`, `voiceOn == false` bo'ladi va ilova mikrofon
  /// tugmasini umuman chizmaydi. Ishlamaydigan tugma foydalanuvchini
  /// chalg'itadi (`AiStatus` sinfining bosh izohidagi qoida).
  bool _voiceOn = false;
  bool get voiceOn => _state == AiAvailability.on && _voiceOn;

  /// Bir vaqtda bitta so'rov: tugma tez-tez bosilsa ham server
  /// takroriy so'rovlar bilan to'ldirilmaydi.
  Future<void>? _inFlight;

  /// Holatni serverdan qayta so'raydi.
  ///
  /// XATO TASHLAMAYDI — u holatga yoziladi. Chaqiruvchi `await` dan
  /// keyin [isOn] ni tekshiradi, `try/catch` yozishi shart emas.
  Future<void> refresh() {
    return _inFlight ??= _refresh().whenComplete(() => _inFlight = null);
  }

  Future<void> _refresh() async {
    try {
      final status = await api.aiStatus();
      final on = status['enabled'] == true;
      _voiceOn = status['voice'] == true;
      _set(
        on ? AiAvailability.on : AiAvailability.off,
        on
            ? null
            : 'Server yordamchini o\'chirib qo\'ygan.\n\n'
                'Sabab: `SHADDIY_AI_URL` va `SHADDIY_API_KEY` server '
                'sozlamasida ko\'rsatilmagan. Ular qo\'shilib server '
                'qayta ishga tushirilgach yordamchi o\'zi ishlaydi.',
      );
    } on ApiException catch (e) {
      // 401 — token eskirgan yoki foydalanuvchi kirmagan. Bu
      // yordamchining o'chiqligi EMAS, shuning uchun matn ham
      // boshqacha.
      _set(
        AiAvailability.unreachable,
        e.statusCode == 401
            ? 'Hisobingizga qayta kiring — sessiya muddati tugagan.'
            : 'Server javob bermadi (${e.statusCode}).\n\n${e.message}',
      );
    } catch (e) {
      _set(
        AiAvailability.unreachable,
        'Serverga ulanib bo\'lmadi.\n\n'
        'Internet aloqasini tekshiring. Aloqa bor bo\'lsa — API '
        'server ishlamayotgan bo\'lishi mumkin.\n\n$e',
      );
    }
  }

  void _set(AiAvailability s, String? why) {
    if (_state == s && _reason == why) return;
    _state = s;
    _reason = why;
    notifyListeners();
  }
}

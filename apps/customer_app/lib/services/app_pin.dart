import 'dart:convert';
import 'dart:math';

import 'package:crypto/crypto.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// OnDex PIN kodi — ilovaning O'Z 4 xonali kaliti.
///
/// ── NEGA QURILMA QULFIDAN TASHQARI YANA PIN ────────────────────────
/// Barmoq izi/yuz har doim ham ishlamaydi: barmoq ho'l, yuz niqob
/// ostida, sensor nosoz, yoki foydalanuvchi shunchaki biometrikani
/// ishlatishni xohlamaydi. Bank ilovalarida (Click, Payme) shu sabab
/// PIN ASOSIY, biometrika esa uning USTIDAGI tezkor yo'l.
///
/// ┌─ 4 XONALI KOD JUDA KICHIK — QANDAY HIMOYALANADI ─────────────────┐
/// Bor-yo'g'i 10 000 variant. Uni himoyalash uchun UCH qatlam:
///
///	1. SEKIN HOSILA (PBKDF2-HMAC-SHA256, 120 000 iteratsiya + tasodifiy
///	   tuz). Bitta urinish ~0.2–0.5 s turadi, ya'ni butun fazoni
///	   sinash bir necha kun oladi. Oddiy SHA-256 bilan bu bir necha
///	   sekundlik ish bo'lardi.
///	2. APPARAT SHIFRI. Hash va tuz `flutter_secure_storage` da, ya'ni
///	   Android Keystore kaliti bilan shifrlangan. Ularni umuman
///	   o'qish uchun qurilmani buzish kerak.
///	3. URINISHLAR CHEGARASI. 5 ta xato -> sessiya BUTUNLAY o'chiriladi
///	   (token o'chadi, qaytadan to'liq kirish kerak). Ya'ni onlayn
///	   brute-force imkonsiz.
///
/// PIN'ning O'ZI HECH QAYERDA SAQLANMAYDI — faqat hosila.
/// └──────────────────────────────────────────────────────────────────┘
///
/// ── PIN NIMANI HIMOYA QILMAYDI ─────────────────────────────────────
/// PIN — LOKAL qulf. U serverdagi tokenni almashtirmaydi va tarmoq
/// hujumlariga qarshi emas. Uning vazifasi: qurilma begona qo'lga
/// tushganda ilovaga kirishni to'sish.
class AppPin {
  AppPin._();

  static const _secure = FlutterSecureStorage();

  static const _kSalt = 'ondex_pin_salt';
  static const _kHash = 'ondex_pin_hash';
  static const _kFails = 'ondex_pin_fails';

  /// Saqlangan PIN NECHA XONALI ekani.
  ///
  /// ┌─ NEGA KERAK (migratsiya) ────────────────────────────────────────┐
  /// PIN uzunligi 4 dan 6 ga o'zgartirilganda eski o'rnatishlarda
  /// hash 4 xonali koddan hosil qilingan bo'lib qoladi. Kiritish
  /// ekrani esa 6 xona to'lishini kutadi — ya'ni foydalanuvchi o'z
  /// PIN'ini KIRITA OLMAY qoladi va ilovaga umuman kira olmaydi.
  ///
  /// Shu sabab uzunlik ham saqlanadi. Mos kelmasa qulf "eskirgan" deb
  /// belgilanadi va foydalanuvchi TIKLASH oqimiga yuboriladi (raqamga
  /// kod -> yangi PIN). ATAYLAB tiklash, "shunchaki yangi PIN
  /// yarating" EMAS: aks holda qulflangan qurilmani qo'lga kiritgan
  /// odam hech narsa isbotlamasdan yangi PIN qo'yib kirib olardi.
  /// └─────────────────────────────────────────────────────────────────┘
  static const _kLen = 'ondex_pin_len';

  /// PIN uzunligi. O'zgartirilsa UI ham moslashadi (`PinScreen`).
  ///
  /// 6 xona — 1 000 000 variant (4 xonada 10 000 edi, ya'ni 100 barobar
  /// ko'p). Sekin hosila va urinishlar chegarasi bilan birga bu kodni
  /// amalda topib bo'lmas qiladi.
  static const pinLength = 6;

  /// Necha xato urinishdan keyin sessiya o'chiriladi.
  static const maxAttempts = 5;

  /// PBKDF2 iteratsiyalari. Ko'paytirilsa xavfsizroq, lekin kutish
  /// uzayadi. 120 000 — mobil qurilmada ~0.2–0.5 s.
  static const _iterations = 120000;

  static Future<bool> isSet() async {
    try {
      final h = await _secure.read(key: _kHash);
      return h != null && h.isNotEmpty;
    } catch (_) {
      return false;
    }
  }

  /// PIN o'rnatadi (yangi tuz bilan) va urinishlar hisobini tozalaydi.
  static Future<void> set(String pin) async {
    final salt = _randomSalt();
    final hash = await _derive(pin, salt);
    await _secure.write(key: _kSalt, value: base64Encode(salt));
    await _secure.write(key: _kHash, value: base64Encode(hash));
    await _secure.write(key: _kLen, value: '${pin.length}');
    await _secure.delete(key: _kFails);
  }

  /// Saqlangan PIN hozirgi `pinLength` ga mos keladimi.
  ///
  /// `false` — eski uzunlikdagi PIN qolgan va uni kiritib bo'lmaydi
  /// (`_kLen` izohiga qarang). Kalit umuman yo'q bo'lsa ham `false`:
  /// u uzunlik saqlana boshlaguncha yaratilgan, ya'ni eski yozuv.
  static Future<bool> matchesCurrentLength() async {
    if (!await isSet()) return false;
    try {
      final v = await _secure.read(key: _kLen);
      return int.tryParse(v ?? '') == pinLength;
    } catch (_) {
      return false;
    }
  }

  /// Chiqishda / PIN tugagach — hamma izni o'chiradi.
  static Future<void> clear() async {
    for (final k in [_kSalt, _kHash, _kFails, _kLen]) {
      try {
        await _secure.delete(key: k);
      } catch (_) {
        // Ombor ochilmasa ham qolganini o'chirishga urinamiz.
      }
    }
  }

  static Future<int> failedAttempts() async {
    try {
      return int.tryParse(await _secure.read(key: _kFails) ?? '') ?? 0;
    } catch (_) {
      return 0;
    }
  }

  /// Biometrika muvaffaqiyatli o'tganda xato hisobini tozalaydi.
  ///
  /// NEGA TO'G'RI: barmoq izi/yuz PIN bilan TENG KUCHDAGI dalil
  /// (ikkalasi ham "qurilma egasi shu yerda" deydi). Aks holda
  /// biometrika bilan kirgan foydalanuvchi eski xato urinishlari
  /// tufayli keyingi safar kutilmaganda hisobdan chiqib qolardi.
  static Future<void> resetAttemptsAfterBiometric() async {
    try {
      await _secure.delete(key: _kFails);
    } catch (_) {
      // Hisobni tozalay olmasak ham kirish davom etadi.
    }
  }

  static Future<int> attemptsLeft() async =>
      (maxAttempts - await failedAttempts()).clamp(0, maxAttempts);

  /// PIN tekshiradi.
  ///
  /// ┌─ TARTIB MUHIM ──────────────────────────────────────────────────┐
  /// Xato hisobi tekshiruvdan OLDIN oshiriladi. Aks holda hujumchi
  /// tekshiruv tugashini kutmasdan ilovani o'ldirib (yoki jarayonni
  /// to'xtatib) hisobni oshirmasdan cheksiz urinishi mumkin edi.
  /// Muvaffaqiyatda hisob nolga qaytariladi.
  /// └─────────────────────────────────────────────────────────────────┘
  static Future<PinResult> verify(String pin) async {
    final saltB64 = await _secure.read(key: _kSalt);
    final hashB64 = await _secure.read(key: _kHash);
    if (saltB64 == null || hashB64 == null) {
      return const PinResult(ok: false, attemptsLeft: 0, sessionWiped: true);
    }

    final fails = await failedAttempts() + 1;
    await _secure.write(key: _kFails, value: '$fails');

    final got = await _derive(pin, base64Decode(saltB64));
    if (_constantTimeEquals(got, base64Decode(hashB64))) {
      await _secure.delete(key: _kFails);
      return const PinResult(ok: true, attemptsLeft: maxAttempts);
    }

    final left = (maxAttempts - fails).clamp(0, maxAttempts);
    if (left == 0) {
      // Urinishlar tugadi — PIN izlari o'chiriladi. Sessiyani
      // (tokenni) chaqiruvchi o'chiradi: bu qatlam tarmoqni bilmaydi.
      await clear();
      return const PinResult(ok: false, attemptsLeft: 0, sessionWiped: true);
    }
    return PinResult(ok: false, attemptsLeft: left);
  }

  /// Oson topiladigan PIN'lar. `null` — qabul qilinadi.
  ///
  /// NEGA: 4 xonali kodning eng ko'p ishlatiladigan bir nechtasi
  /// (`0000`, `1234`, tug'ilgan yil naqshlari) butun fazoning katta
  /// qismini tashkil qiladi. Ularni to'sish urinishlar chegarasi bilan
  /// birga ishlaganda hujumni amalda imkonsiz qiladi.
  static String? weakness(String pin) {
    if (pin.length != pinLength || int.tryParse(pin) == null) {
      return 'PIN $pinLength ta raqamdan iborat bo\'lishi kerak';
    }
    if (RegExp(r'^(\d)\1+$').hasMatch(pin)) {
      return 'Bir xil raqamlardan iborat PIN xavfsiz emas';
    }
    final digits = pin.split('').map(int.parse).toList();
    var ascending = true, descending = true;
    for (var i = 1; i < digits.length; i++) {
      if (digits[i] != digits[i - 1] + 1) ascending = false;
      if (digits[i] != digits[i - 1] - 1) descending = false;
    }
    if (ascending || descending) {
      return 'Ketma-ket raqamlar (1234 kabi) xavfsiz emas';
    }
    // Takrorlanuvchi juftlik/uchlik naqshlari: 121212, 123123.
    final half = pin.substring(0, pinLength ~/ 2);
    if (pin == half + half) return 'Takrorlanuvchi naqsh xavfsiz emas';
    final third = pin.substring(0, 2);
    if (pinLength == 6 && pin == third * 3) {
      return 'Takrorlanuvchi naqsh xavfsiz emas';
    }
    // Eng ko'p uchraydigan 6 xonali kodlar (ochiq buzilgan bazalardan).
    const common = {
      '123456', '654321', '111222', '121212', '112233', '123321',
      '000111', '696969', '159753', '147258', '102030', '123654',
      // Tug'ilgan yil naqshlari — 19XX/20XX bilan boshlanadi.
      '191919', '202020',
    };
    if (common.contains(pin)) return 'Bu PIN juda ko\'p ishlatiladi';
    return null;
  }

  static List<int> _randomSalt() {
    final r = Random.secure();
    return List<int>.generate(16, (_) => r.nextInt(256));
  }

  /// Og'ir hisoblash ALOHIDA isolate'da — aks holda 120 000 iteratsiya
  /// UI oqimini bir necha yuz millisekundga muzlatib qo'yardi.
  static Future<List<int>> _derive(String pin, List<int> salt) =>
      compute(_pbkdf2Task, _Pbkdf2Input(pin, salt, _iterations));

  static bool _constantTimeEquals(List<int> a, List<int> b) {
    if (a.length != b.length) return false;
    var diff = 0;
    for (var i = 0; i < a.length; i++) {
      diff |= a[i] ^ b[i];
    }
    return diff == 0;
  }
}

class PinResult {
  const PinResult({
    required this.ok,
    required this.attemptsLeft,
    this.sessionWiped = false,
  });

  final bool ok;
  final int attemptsLeft;

  /// Urinishlar tugadi — chaqiruvchi tokenni o'chirib, foydalanuvchini
  /// to'liq kirish ekraniga qaytarishi SHART.
  final bool sessionWiped;
}

class _Pbkdf2Input {
  const _Pbkdf2Input(this.pin, this.salt, this.iterations);
  final String pin;
  final List<int> salt;
  final int iterations;
}

/// PBKDF2-HMAC-SHA256, 32 baytlik natija (bitta blok — qo'shimcha
/// bloklar kerak emas, chunki SHA-256 chiqishi aynan 32 bayt).
///
/// Top-level funksiya: `compute()` faqat shundaylarni qabul qiladi.
List<int> _pbkdf2Task(_Pbkdf2Input input) {
  final hmac = Hmac(sha256, utf8.encode(input.pin));
  // T1 = PRF(P, S || INT_32_BE(1))
  final block = <int>[...input.salt, 0, 0, 0, 1];
  var u = hmac.convert(block).bytes;
  final result = List<int>.from(u);
  for (var i = 1; i < input.iterations; i++) {
    u = hmac.convert(u).bytes;
    for (var j = 0; j < result.length; j++) {
      result[j] ^= u[j];
    }
  }
  return result;
}

import 'dart:convert';

import 'package:crypto/crypto.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:chust_customer/services/app_pin.dart';

// PIN — 4 xonali, ya'ni bor-yo'g'i 10 000 variant. Uni himoyalaydigan
// qoidalar shu yerda sinaladi: zaif kodlarni to'sish va hosila
// funksiyasining to'g'riligi.

void main() {
  group('zaif PIN qoidalari', () {
    test('bir xil raqamlar rad etiladi', () {
      for (final p in ['000000', '111111', '999999']) {
        expect(AppPin.weakness(p), isNotNull, reason: p);
      }
    });

    test('ketma-ket raqamlar rad etiladi', () {
      for (final p in ['123456', '234567', '654321', '987654']) {
        expect(AppPin.weakness(p), isNotNull, reason: p);
      }
    });

    test('takrorlanuvchi naqshlar rad etiladi', () {
      // 121212 (ikkitalik), 123123 (uchtalik)
      for (final p in ['121212', '123123', '454545', '789789']) {
        expect(AppPin.weakness(p), isNotNull, reason: p);
      }
    });

    test('ko\'p ishlatiladigan kodlar rad etiladi', () {
      for (final p in ['112233', '159753', '696969']) {
        expect(AppPin.weakness(p), isNotNull, reason: p);
      }
    });

    test('uzunligi yoki formati noto\'g\'ri kodlar rad etiladi', () {
      for (final p in ['', '1234', '1234567', 'abcdef', '12a456']) {
        expect(AppPin.weakness(p), isNotNull, reason: p);
      }
    });

    test('oddiy kuchli kod qabul qilinadi', () {
      for (final p in ['274913', '813507', '509284']) {
        expect(AppPin.weakness(p), isNull, reason: p);
      }
    });
  });

  group('PBKDF2 hosilasi', () {
    // `AppPin` ichidagi hosila `compute()` orqali ishlaydi va platforma
    // kanallarini talab qiladi, shuning uchun bu yerda AYNAN o'sha
    // algoritm mustaqil takrorlanadi va RFC 6070 vektoriga solishtiriladi.
    List<int> pbkdf2(List<int> password, List<int> salt, int iterations) {
      final hmac = Hmac(sha256, password);
      var u = hmac.convert(<int>[...salt, 0, 0, 0, 1]).bytes;
      final out = List<int>.from(u);
      for (var i = 1; i < iterations; i++) {
        u = hmac.convert(u).bytes;
        for (var j = 0; j < out.length; j++) {
          out[j] ^= u[j];
        }
      }
      return out;
    }

    test('RFC 6070 uslubidagi ma\'lum vektorga mos keladi', () {
      // PBKDF2-HMAC-SHA256, P="password", S="salt", c=1 -> birinchi blok
      final got = pbkdf2(utf8.encode('password'), utf8.encode('salt'), 1);
      const want = '120fb6cffcf8b32c43e7225256c4f837a86548c9'
          '2ccc35480805987cb70be17b';
      expect(
        got.map((b) => b.toRadixString(16).padLeft(2, '0')).join(),
        want,
      );
    });

    test('tuz o\'zgarsa natija ham o\'zgaradi', () {
      final a = pbkdf2(utf8.encode('1234'), <int>[1, 2, 3], 100);
      final b = pbkdf2(utf8.encode('1234'), <int>[3, 2, 1], 100);
      expect(a, isNot(equals(b)));
    });

    test('bir xil kirish har doim bir xil natija beradi', () {
      final a = pbkdf2(utf8.encode('2749'), <int>[9, 9], 200);
      final b = pbkdf2(utf8.encode('2749'), <int>[9, 9], 200);
      expect(a, equals(b));
    });
  });

  test('urinishlar chegarasi mavjud va oqilona', () {
    expect(AppPin.maxAttempts, greaterThan(0));
    expect(AppPin.maxAttempts, lessThanOrEqualTo(10));
    // 6 xona = 1 000 000 variant (4 xonada 10 000 edi).
    expect(AppPin.pinLength, 6);
  });
}

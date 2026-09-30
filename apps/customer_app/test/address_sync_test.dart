import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// MANZIL UCH JOYDA BIR XIL KO'RINISHI KERAK.
///
/// ┌─ NEGA MANBA MATNI TEKSHIRILADI ────────────────────────────────────┐
/// Uchala ekran ham `api.getMyAddress()` ni chaqiradi, ya'ni testda
/// tarmoq (yoki uni almashtiradigan butun bir qatlam) kerak bo'lardi;
/// `AddressScreen` esa ustiga xarita vidjetini talab qiladi va uni
/// testda ko'rsatib bo'lmaydi (`ondex_map_screen_test.dart` izohiga
/// qarang).
///
/// Bu yerdagi qoidalar KICHIK va aniq: «shu chaqiruv shu joyda
/// bo'lsin». Ularning yo'qligi aynan foydalanuvchi ko'rgan xatolarni
/// qaytaradi.
/// └────────────────────────────────────────────────────────────────────┘
void main() {
  String kod(String yol) => File(yol)
      .readAsStringSync()
      .split('\n')
      .where((l) => !l.trimLeft().startsWith('//'))
      .join('\n');

  final bosh = kod('lib/screens/super_home_screen.dart');
  final restoran = kod('lib/screens/catalog_screen.dart');
  final manzil = kod('lib/screens/address_screen.dart');

  group('ekranga qaytilganda sarlavha yangilanadi', () {
    // ┌─ O'LCHANGAN XATO ────────────────────────────────────────────┐
    // Bosh sahifadagi `_push` faqat `_loadUnread()` ni chaqirardi.
    // Natijada manzil ekranidan qaytilganda sarlavhada ESKI nom
    // qolardi, restoran sahifasida esa (u `_loadAddress()` ni ham
    // chaqiradi) yangisi turardi — bitta ilovada ikki xil manzil.
    // └──────────────────────────────────────────────────────────────┘
    test('bosh sahifa `_push` dan keyin manzilni qayta o\'qiydi', () {
      final i = bosh.indexOf('Future<void> _push(');
      expect(i, greaterThan(-1));
      final tana = bosh.substring(i, i + 500);
      expect(tana, contains('_loadAddress()'),
          reason: 'qaytilganda sarlavhada eski manzil qoladi');
      expect(tana, contains('_loadUnread()'));
    });

    test('restoran sahifasi ham shunday qiladi', () {
      final i = restoran.indexOf('Future<void> _push(');
      expect(i, greaterThan(-1));
      expect(restoran.substring(i, i + 500), contains('_loadAddress()'));
    });

    test('bo\'sh javob eski nomni O\'CHIRADI', () {
      // `if (t.isNotEmpty)` sharti bilan manzil o'chirilganda eski
      // nom ekranda abadiy qolib ketardi.
      for (final (nom, manba) in [('bosh', bosh), ('restoran', restoran)]) {
        expect(manba, contains('t.isEmpty ? kNoAddressLabel : t'),
            reason: '$nom sahifasi bo\'sh manzilni qayta ishlamaydi');
      }
    });
  });

  group('manzil ekrani saqlanganidan boshlanadi', () {
    // ┌─ O'LCHANGAN XATO ────────────────────────────────────────────┐
    // Ekran har safar Chust markazidan ochilib, darhol teskari
    // geokodlash maydonga «Chust…» yozib qo'yardi. Manzilini boshqa
    // shaharga o'zgartirgan odam uni ko'rmasdi va saqlasa manzil
    // JIMGINA Chustga almashardi.
    // └──────────────────────────────────────────────────────────────┘
    test('`_loadSaved` mavjud va xaritadan OLDIN bajariladi', () {
      final s = manzil.indexOf('await _loadSaved()');
      final m = manzil.indexOf('await _loadMaps()');
      expect(s, greaterThan(-1), reason: '`_loadSaved` yo\'qolgan');
      expect(m, greaterThan(-1));
      expect(s, lessThan(m),
          reason: 'xarita `initialCenter` ni bir marta oladi — markaz '
              'undan oldin to\'g\'ri bo\'lishi kerak');
    });

    test('saqlangan matn teskari geokodlashdan himoyalangan', () {
      final i = manzil.indexOf('Future<void> _loadSaved()');
      final tana = manzil.substring(i, i + 1600);
      expect(tana, contains('_lastResolvedCenter = _center'),
          reason: 'busiz server matni foydalanuvchi matnini bosib ketadi');
      expect(tana, contains('_addressCtrl.text = t'));
    });

    test('qo\'shimcha maydonlar ham to\'ldiriladi', () {
      final i = manzil.indexOf('Future<void> _loadSaved()');
      final tana = manzil.substring(i, i + 1600);
      for (final c in [
        '_entranceCtrl',
        '_floorCtrl',
        '_apartmentCtrl',
        '_intercomCtrl',
        '_commentCtrl',
      ]) {
        expect(tana, contains('$c.text = '), reason: '$c to\'ldirilmaydi');
      }
    });

    test('chaqiruvchi bergan nuqta USTUN', () {
      // Buyurtma oqimi aniq nuqta bilan ochadi — saqlangan manzil uni
      // bosib ketmasligi kerak.
      final i = manzil.indexOf('Future<void> _boot()');
      final tana = manzil.substring(i, i + 400);
      expect(tana, contains('widget.initialLat == null'));
    });
  });
}

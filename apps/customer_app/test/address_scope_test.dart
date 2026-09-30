import 'dart:io';

import 'package:chust_customer/data/address_scope.dart';
import 'package:flutter_test/flutter_test.dart';

/// MANZIL O'ZGARGANDA RESTORAN RO'YXATI DARHOL YANGILANADI.
///
/// ┌─ O'LCHANGAN XATO ──────────────────────────────────────────────────┐
/// Server ro'yxatni manzil shahri bilan cheklaganidan keyin ham,
/// manzilni Toshkentdan Chustga o'zgartirganda eski shaharning
/// restoranlari ekranda QOLIB KETARDI — faqat ilovani qayta ishga
/// tushirgandan keyin yo'qolardi.
///
/// Ikki sabab bor edi:
///   1. Diskdagi kesh kaliti shunchaki `restaurants` edi — manzil
///      kalitga kirmasdi, ya'ni eski ro'yxat o'sha kalitdan
///      o'qilaverardi.
///   2. Manzil ekranidan qaytilganda hech kim ro'yxatni qayta
///      so'ramasdi.
/// └────────────────────────────────────────────────────────────────────┘
void main() {
  String kod(String yol) => File(yol)
      .readAsStringSync()
      .split('\n')
      .where((l) => !l.trimLeft().startsWith('//'))
      .join('\n');

  /// Metodning FAQAT o'z tanasi.
  ///
  /// ⚠️ Belgilangan uzunlikdagi oyna (`substring(i, i + 400)`) YARAMAYDI:
  /// u keyingi metod e'loniga tushib ketadi va o'sha yerdagi nomni
  /// "topib", tasdiqni YOLG'ON o'tkazadi. Bu aynan shu faylda sodir
  /// bo'lgan — `_loadRestaurants()` chaqiruvi olib tashlanganda test
  /// baribir o'tib ketgan edi.
  ///
  /// Shuning uchun tana metod boshidan to ikki bo'shliqli yopuvchi
  /// qavsgacha (`\n  }`) olinadi — sinf ichidagi metodning oxiri.
  String tana(String manba, String imzo) {
    final i = manba.indexOf(imzo);
    expect(i, greaterThan(-1), reason: '«$imzo» topilmadi');
    final oxir = manba.indexOf('\n  }', i);
    expect(oxir, greaterThan(i), reason: '«$imzo» tanasi yopilmagan');
    return manba.substring(i, oxir);
  }

  group('kesh yorlig\'i', () {
    test('manzil yo\'q — barqaror yorliq', () {
      // Yangi nusxa yo'q (singleton), shuning uchun boshlang'ich
      // holat tekshiriladi: ilova manzilsiz ochilganda kalit
      // aniqlangan bo'lishi kerak, `null` yoki bo'sh emas.
      final s = AddressScope.instance;
      if (!s.bor) {
        expect(s.cacheTag, 'yoq');
      }
    });

    test('joy o\'zgarsa yorliq ham o\'zgaradi', () {
      final s = AddressScope.instance;
      s.set(lat: 41.3111, lng: 69.2797, text: 'Toshkent');
      final toshkent = s.cacheTag;
      s.set(lat: 41.0004, lng: 71.2394, text: 'Chust');
      final chust = s.cacheTag;

      expect(toshkent, isNot(chust),
          reason: 'bir xil kalit ikki shaharning ro\'yxatini aralashtiradi');
      expect(s.bor, isTrue);
      expect(s.text, 'Chust');
    });

    test('faqat MATN o\'zgarsa yorliq o\'zgarmaydi', () {
      // Podyezd/izoh tahrirlansa ro'yxat o'zgarmaydi — keshni
      // behuda tashlamaymiz.
      final s = AddressScope.instance;
      s.set(lat: 41.0004, lng: 71.2394, text: 'Chust, 1-uy');
      final a = s.cacheTag;
      s.set(lat: 41.0004, lng: 71.2394, text: 'Chust, 2-uy');
      expect(s.cacheTag, a);
    });

    test('kichik GPS tebranishi yorliqni buzmaydi', () {
      // Uch kasr ≈ 110 m. Bir necha metr siljish yangi kalit
      // yaratmasligi kerak, aks holda har ochilishda kesh behuda
      // yangilanardi.
      final s = AddressScope.instance;
      s.set(lat: 41.00040, lng: 71.23940, text: 'A');
      final a = s.cacheTag;
      s.set(lat: 41.00041, lng: 71.23941, text: 'A');
      expect(s.cacheTag, a);
    });
  });

  group('o\'zgarish xabari', () {
    test('kuzatuvchi chaqiriladi', () {
      final s = AddressScope.instance;
      s.set(lat: 41.3111, lng: 69.2797, text: 'Toshkent');
      var soni = 0;
      void tinglovchi() => soni++;
      s.addListener(tinglovchi);
      addTearDown(() => s.removeListener(tinglovchi));

      s.set(lat: 41.0004, lng: 71.2394, text: 'Chust');
      expect(soni, 1);

      // AYNI qiymat — xabar YO'Q, aks holda ekranlar behuda qayta
      // yuklanardi.
      s.set(lat: 41.0004, lng: 71.2394, text: 'Chust');
      expect(soni, 1, reason: 'o\'zgarmagan manzil xabar bermasligi kerak');
    });
  });

  group('ulanish nuqtalari', () {
    // Bu qoidalarni xatti-harakat orqali tekshirib bo'lmaydi: ekranlar
    // tarmoq va xarita talab qiladi. Ular kichik va aniq — "shu
    // chaqiruv shu joyda bo'lsin".
    final repo = kod('lib/data/catalog_repository.dart');
    final manzil = kod('lib/screens/address_screen.dart');
    final bosh = kod('lib/screens/super_home_screen.dart');
    final katalog = kod('lib/screens/catalog_screen.dart');
    final apiDart = kod('lib/api.dart');

    test('kesh kaliti manzilga bog\'langan', () {
      expect(repo, contains('restaurants.\${tag ?? AddressScope.instance.cacheTag}'),
          reason: 'kalit manzilsiz bo\'lsa eski shahar ro\'yxati qoladi');
    });

    test('so\'rov `for_me=1` bilan ketadi', () {
      // Usiz server TO'LIQ ro'yxatni qaytaradi va filtr umuman
      // ishlamaydi.
      expect(apiDart, contains('/restaurants?for_me=1'));
    });

    test('manzil saqlangach butun ilovaga xabar beriladi', () {
      expect(manzil, contains('AddressScope.instance.set('));
    });

    test('bosh sahifa xabarni eshitadi va RO\'YXATNI ham yangilaydi', () {
      expect(bosh, contains('AddressScope.instance.addListener(_onAddressChanged)'));
      expect(bosh, contains('AddressScope.instance.removeListener(_onAddressChanged)'),
          reason: 'kuzatuvchi olib tashlanmasa ekran yopilgach ham chaqiriladi');
      expect(tana(bosh, 'void _onAddressChanged()'),
          contains('_loadRestaurants();'),
          reason: 'faqat sarlavha yangilansa ro\'yxat eski holicha qoladi');
    });

    test('katalog yangi KALIT bilan qayta obuna bo\'ladi', () {
      expect(katalog, contains('AddressScope.instance.addListener(_onAddressChanged)'));
      expect(katalog, contains('AddressScope.instance.removeListener(_onAddressChanged)'));
      final b = tana(katalog, 'void _onAddressChanged()');
      // Repozitoriy QAYTA quriladi: kalit manzilga bog'liq, eskisi
      // eski kalitni ushlab turadi.
      expect(b, contains('_restaurants = Repos.restaurants()'));
      expect(b, contains('_reloadTick++'));
    });
  });
}

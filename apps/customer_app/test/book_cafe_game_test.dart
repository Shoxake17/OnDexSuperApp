// 3D maketdan qaytgan holatni o'qish va tekshirish.
//
// ┌─ NEGA SINOV KERAK ─────────────────────────────────────────────────┐
// Holat ALOHIDA JARAYONDAN keladi va uni telefondagi boshqa ilova ham
// yozgan bo'lishi mumkin. Ya'ni bu tashqi ma'lumot va unga ishonib
// bo'lmaydi.
//
// Tekshiruvlar ko'zga ko'rinmaydi: ular faqat buzilgan ma'lumot
// kelganda ishlaydi. Sinovsiz ular jimgina ishlamay qolishi va buni
// hech kim sezmasligi mumkin.
// └────────────────────────────────────────────────────────────────────┘
import 'package:flutter_test/flutter_test.dart';
import 'package:chust_customer/services/book_cafe_game.dart';

void main() {
  group('GameResult.parse', () {
    test('haqiqiy holatni o\'qiydi', () {
      final r = GameResult.parse(
        '{"favorites":["c9ee60df41cc5a0a"],'
        '"items":[{"id":"dd1d82e21511410a","qty":3}],'
        '"restaurant":"f3e2b323cf5c8752","table":"5-stol"}',
      );
      expect(r.hasState, isTrue);
      expect(r.restaurantId, 'f3e2b323cf5c8752');
      expect(r.cart, {'dd1d82e21511410a': 3});
      expect(r.favorites, {'c9ee60df41cc5a0a'});
    });

    test('bo\'sh yoki buzilgan matn holat bermaydi', () {
      // `hasState = false` MUHIM: uni "hammasini o'chir" deb
      // tushunish savatni bekordan bekorga bo'shatib yuborardi.
      expect(GameResult.parse(null).hasState, isFalse);
      expect(GameResult.parse('').hasState, isFalse);
      expect(GameResult.parse('{buzilgan').hasState, isFalse);
      expect(GameResult.parse('[]').hasState, isFalse);
    });

    test('noto\'g\'ri ID rad etiladi', () {
      final r = GameResult.parse(
        '{"items":[{"id":"../../etc/passwd","qty":1},'
        '{"id":"yaxshi_id","qty":2}],'
        '"favorites":["ham/yomon","yaxshi-id2"]}',
      );
      expect(r.cart.keys, ['yaxshi_id']);
      expect(r.favorites, {'yaxshi-id2'});
    });

    test('miqdor oralig\'i tekshiriladi', () {
      final r = GameResult.parse(
        '{"items":[{"id":"a","qty":0},{"id":"b","qty":-5},'
        '{"id":"c","qty":100},{"id":"d","qty":"besh"},'
        '{"id":"e","qty":99}]}',
      );
      expect(r.cart, {'e': 99});
    });

    test('boshqa restoran ID si o\'tmaydi', () {
      final r = GameResult.parse('{"restaurant":"yo\'q/joy","items":[]}');
      expect(r.restaurantId, isEmpty);
    });

    test('narx maydoni umuman o\'qilmaydi', () {
      // O'yin narx yubormaydi, lekin yuborsa ham e'tiborga
      // olinmasligi kerak: narx faqat serverdan olinadi.
      final r = GameResult.parse(
        '{"items":[{"id":"a","qty":1,"price_tiyin":1}],"total":999}',
      );
      expect(r.cart, {'a': 1});
    });
  });
}

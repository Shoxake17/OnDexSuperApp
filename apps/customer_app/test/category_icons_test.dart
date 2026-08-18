import 'package:flutter_test/flutter_test.dart';
import 'package:chust_customer/category_icons.dart';

/// Bu testlar QURILMADA topilgan xato tufayli yozildi.
///
/// Backend turkumlarni KO'PLIK shaklda qaytaradi ("Burgerlar",
/// "Steyklar", "Salatlar"), jadval kalitlari esa birlikda. Avvalgi
/// qat'iy solishtiruv ularni topa olmasdi va bosh sahifada o'sha
/// turkumlarda rasm umuman chizilmasdi — hech qanday xato bermasdan.
void main() {
  group('categoryIconFor', () {
    test('aniq moslik', () {
      expect(categoryIconFor('Pizza'), 'assets/categories/pizza.png');
      expect(categoryIconFor('KFC'), 'assets/categories/kfc.png');
      expect(categoryIconFor('Norin'), 'assets/categories/norin.png');
    });

    test('ko\'plik qo\'shimchasi bilan ham topiladi', () {
      expect(categoryIconFor('Burgerlar'), 'assets/categories/burger.png');
      expect(categoryIconFor('Steyklar'), 'assets/categories/steyk.png');
      expect(categoryIconFor('Salatlar'), 'assets/categories/salat.png');
      expect(categoryIconFor('Desertlar'), 'assets/categories/dessert.png');
      expect(categoryIconFor('Gazaklar'), 'assets/categories/gazak.png');
    });

    test('bo\'shliq, registr va apostrof ahamiyatsiz', () {
      expect(categoryIconFor('Fast Food'), 'assets/categories/fastfood.png');
      expect(categoryIconFor('fast-food'), 'assets/categories/fastfood.png');
      expect(categoryIconFor('FASTFOOD'), 'assets/categories/fastfood.png');
      expect(categoryIconFor('Lag\'mon'), 'assets/categories/lag\'mon.png');
    });

    test('ko\'p so\'zli turkumlar', () {
      expect(categoryIconFor('Milliy taomlar'),
          'assets/categories/milliy.png');
      expect(categoryIconFor('Yevropa taomlar'),
          'assets/categories/yevropa.png');
      expect(categoryIconFor('Suyuq ovqatlar'),
          'assets/categories/suyuq-ovqat.png');
      expect(categoryIconFor('Quyuq ovqatlar'),
          'assets/categories/quyuq-ovqatlar.png');
    });

    test('noma\'lum turkum — null (zaxira belgi chiziladi)', () {
      expect(categoryIconFor('Qwertyuiop'), isNull);
      expect(categoryIconFor(''), isNull);
      expect(categoryIconFor('   '), isNull);
    });
  });
}

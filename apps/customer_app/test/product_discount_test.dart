import 'package:chust_customer/data/quote_service.dart';
import 'package:chust_customer/widgets/product_grid.dart';
import 'package:flutter_test/flutter_test.dart';

/// `computeProductDiscount` — menyu kartochkasidagi narx.
///
/// Bu funksiya SERVER bilan bir xil qoidalar bo'yicha ishlashi shart
/// (`internal/promotions/apply.go` + `internal/orders/service.go`):
/// kartochkadagi narx "shu taomni yolg'iz olsam server qancha
/// hisoblaydi" degani. Quyidagi testlar aynan shu mosligini qo'riqlaydi
/// — ular buzilsa, mijoz menyuda bir narx ko'rib, checkout'da boshqasini
/// to'laydi (haqiqatda ko'rilgan xato).
Map<String, dynamic> product({
  String id = 'p1',
  String category = 'Ichimliklar',
  int price = 1500000, // 15 000 so'm
  int discountPrice = 0,
}) =>
    {
      'id': id,
      'category': category,
      'price_tiyin': price,
      'discount_price_tiyin': discountPrice,
    };

Map<String, dynamic> promo({
  required String type,
  required int value,
  String unit = 'percent',
  List<String>? products,
  List<String>? categories,
  bool orders = false,
  int minOrder = 0,
  int maxDiscount = 0,
}) =>
    {
      'type': type,
      'discount_unit': unit,
      'discount_value': value,
      'min_order_amount_tiyin': minOrder,
      'max_discount_amount_tiyin': maxDiscount,
      'applies_to_orders': orders,
      'applies_to_products': products != null,
      'applies_to_categories': categories != null,
      'target_product_ids': products ?? const [],
      'target_categories': categories ?? const [],
    };

void main() {
  test('chegirma yo\'q — null', () {
    expect(computeProductDiscount(product(), const []), isNull);
  });

  test('mahsulotning o\'z chegirma narxi', () {
    final d = computeProductDiscount(
        product(discountPrice: 500000), const []);
    expect(d!.discountedPriceTiyin, 500000);
    expect(d.label, '-10 000 so\'m');
  });

  test('foizli aksiya', () {
    final d = computeProductDiscount(
      product(),
      [promo(type: 'percent', value: 20, categories: ['Ichimliklar'])],
    );
    expect(d!.discountedPriceTiyin, 1200000); // 15 000 - 20%
    expect(d.label, '-20%');
  });

  test('summali aksiya — har donaga', () {
    final d = computeProductDiscount(
      product(),
      [
        promo(
            type: 'fixed_amount',
            unit: 'amount',
            value: 500000,
            categories: ['Ichimliklar'])
      ],
    );
    expect(d!.discountedPriceTiyin, 1000000);
    expect(d.label, '-5 000 so\'m');
  });

  // Server `type` ni birinchi darajali haqiqat deb biladi
  // (`Promotion.EffectiveUnit`). Avval klient `discount_unit` ga
  // qarardi: bunday yozuvda menyuda "-20%" ko'rinib, server 20 tiyin
  // chegirma berardi.
  test('turga zid birlik — TUR hal qiladi', () {
    final d = computeProductDiscount(
      product(),
      [
        promo(
            type: 'fixed_amount', // summa
            unit: 'percent', // zid qiymat
            value: 500000,
            categories: ['Ichimliklar'])
      ],
    );
    expect(d!.discountedPriceTiyin, 1000000); // 5 000 so'm, 20% emas
  });

  test('maksimal chegirma chegarasi hisobga olinadi', () {
    final d = computeProductDiscount(
      product(),
      [
        promo(
            type: 'percent',
            value: 50,
            categories: ['Ichimliklar'],
            maxDiscount: 200000) // ko'pi bilan 2 000 so'm
      ],
    );
    expect(d!.discountedPriceTiyin, 1300000); // 15 000 - 2 000
    // Chegara ishlaganda "-50%" yozuvi yolg'on bo'lardi.
    expect(d.label, '-2 000 so\'m');
  });

  test('minimal buyurtma summasi bajarilmasa — chegirma ko\'rsatilmaydi',
      () {
    final promos = [
      promo(
          type: 'percent',
          value: 20,
          categories: ['Ichimliklar'],
          minOrder: 5000000) // 50 000 so'm
    ];
    expect(computeProductDiscount(product(), promos), isNull);
    // Savat allaqachon shartni bajargan bo'lsa — ko'rsatiladi.
    final d = computeProductDiscount(product(), promos,
        cartSubtotalTiyin: 6000000);
    expect(d!.discountedPriceTiyin, 1200000);
  });

  test('chegirmalar QO\'SHILMAYDI — eng foydalisi tanlanadi', () {
    final d = computeProductDiscount(
      product(discountPrice: 1000000), // 5 000 so'm foyda
      [promo(type: 'percent', value: 50, categories: ['Ichimliklar'])],
    );
    // 50% = 7 500 so'm > 5 000 so'm, lekin ikkalasi qo'shilmaydi.
    expect(d!.discountedPriceTiyin, 750000);
  });

  // Server butun buyurtmaga tegishli aksiyani ham HAR QATORGA ulush
  // sifatida beradi, shuning uchun kartochka ham uni ko'rsatadi (avval
  // o'tkazib yuborilardi va savatdagi narx menyudagidan farq qilardi).
  test('butun buyurtmaga tegishli aksiya ham hisoblanadi', () {
    final d = computeProductDiscount(
        product(), [promo(type: 'percent', value: 20, orders: true)]);
    expect(d!.discountedPriceTiyin, 1200000); // 15 000 - 20%
    expect(d.label, '-20%');
  });

  // ★ HAQIQIY HOLAT: ikki xil aksiya — har mahsulot o'zinikini oladi.
  test('ikki xil aksiya: har mahsulot o\'zining chegirmasini oladi', () {
    final promos = [
      promo(type: 'fixed_amount', unit: 'amount', value: 500000, categories: [
        'Ichimliklar'
      ]),
      promo(type: 'percent', value: 20, products: ['kokteyl']),
    ];
    final cola = computeProductDiscount(
        product(id: 'cola', price: 1200000), promos);
    final kokteyl = computeProductDiscount(
        product(id: 'kokteyl', category: 'Kokteyllar', price: 1000000), promos);

    expect(cola!.discountedPriceTiyin, 700000); // 12 000 -> 7 000
    expect(kokteyl!.discountedPriceTiyin, 800000); // 10 000 -> 8 000
  });

  test('mos kelmagan turkum — chegirma yo\'q', () {
    final d = computeProductDiscount(
      product(category: 'Milliy taomlar'),
      [promo(type: 'percent', value: 20, categories: ['Ichimliklar'])],
    );
    expect(d, isNull);
  });

  group('discountLineLabel', () {
    // Nom FAQAT chegirmaning hammasi o'sha aksiyadan bo'lganda.
    test('bitta aksiya hammasini bergan — nomi ko\'rinadi', () {
      expect(
        discountLineLabel(
            discountTiyin: 500000,
            promotionDiscountTiyin: 500000,
            promotionName: 'Mustaqillik kuni'),
        'Aksiya: Mustaqillik kuni',
      );
    });

    test('aralash chegirma — umumiy "Chegirma"', () {
      expect(
        discountLineLabel(
            discountTiyin: 1500000,
            promotionDiscountTiyin: 500000,
            promotionName: 'Mustaqillik kuni'),
        'Chegirma',
      );
    });

    test('aksiyasiz (faqat mahsulot chegirmasi)', () {
      expect(
        discountLineLabel(discountTiyin: 1000000, promotionDiscountTiyin: 0),
        'Chegirma',
      );
    });
  });

  group('quoteLineTotals', () {
    test('server javobini jadvalga aylantiradi', () {
      final map = quoteLineTotals({
        'lines': [
          {'product_id': 'a', 'total_tiyin': 1000000},
          {'product_id': 'b', 'total_tiyin': 500000},
        ]
      });
      expect(map, {'a': 1000000, 'b': 500000});
    });

    test('eski/buzilgan javobda bo\'sh jadval', () {
      expect(quoteLineTotals({'total_tiyin': 100}), isEmpty);
      expect(quoteLineTotals({'lines': 'xato'}), isEmpty);
      expect(
          quoteLineTotals({
            'lines': [
              {'product_id': 'a'}
            ]
          }),
          isEmpty);
    });
  });
}

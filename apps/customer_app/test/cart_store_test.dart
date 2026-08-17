import 'package:chust_customer/data/cart_store.dart';
import 'package:flutter_test/flutter_test.dart';

/// Savat qoidalari — biznes uchun kritik, shuning uchun testlangan.
///
/// Diskka yozish bu testlarda ISHLAMAYDI (`flutter_secure_storage`
/// plagini yo'q) — lekin `CartStore` yozishni fon rejimida bajaradi va
/// xatoni yutadi, ya'ni xotiradagi mantiq baribir to'g'ri tekshiriladi.
void main() {
  final cart = CartStore.instance;

  setUp(cart.clear);

  test('boshqa restoran taomi qo\'shilsa eski savat tozalanadi', () {
    cart.setQty(restaurantId: 'r1', productId: 'p1', qty: 2);
    cart.setQty(restaurantId: 'r1', productId: 'p2', qty: 1);
    expect(cart.totalQty, 3);

    // Boshqa restoran — server `ErrMixedRestaurants` bilan rad etardi,
    // shuning uchun klient buni OLDINDAN oldini oladi.
    cart.setQty(restaurantId: 'r2', productId: 'p9', qty: 1);

    expect(cart.restaurantId, 'r2');
    expect(cart.totalQty, 1, reason: 'eski restoran taomlari qolmasligi kerak');
    expect(cart.qtyOf('p1'), 0);
  });

  test('qty 0 yoki manfiy bo\'lsa taom olib tashlanadi', () {
    cart.setQty(restaurantId: 'r1', productId: 'p1', qty: 3);
    cart.setQty(restaurantId: 'r1', productId: 'p1', qty: 0);
    expect(cart.qtyOf('p1'), 0);
    expect(cart.isEmpty, isTrue);
  });

  // Oxirgi taom olib tashlangach restoran bog'lanishi ham ketishi kerak
  // — aks holda keyingi restoranga o'tishda "boshqa restoran"
  // tekshiruvi keraksiz ishlagan bo'lardi.
  test('savat bo\'shasa restoran bog\'lanishi ham tozalanadi', () {
    cart.setQty(restaurantId: 'r1', productId: 'p1', qty: 1);
    expect(cart.restaurantId, 'r1');

    cart.decrement(restaurantId: 'r1', productId: 'p1');
    expect(cart.isEmpty, isTrue);
    expect(cart.restaurantId, isNull);
  });

  test('increment/decrement joriy miqdordan hisoblaydi', () {
    cart.increment(restaurantId: 'r1', productId: 'p1');
    cart.increment(restaurantId: 'r1', productId: 'p1');
    expect(cart.qtyOf('p1'), 2);

    cart.decrement(restaurantId: 'r1', productId: 'p1');
    expect(cart.qtyOf('p1'), 1);
  });

  // QR skanerlash aniq niyat: "men SHU restoranning SHU stolidaman".
  test('boshqa restoran stoli skanerlansa savat tozalanadi', () {
    cart.setQty(restaurantId: 'r1', productId: 'p1', qty: 2);

    cart.startTableSession(restaurantId: 'r2', token: 'tok', tableLabel: '5');

    expect(cart.isEmpty, isTrue, reason: 'begona savat qolmasligi kerak');
    expect(cart.restaurantId, 'r2');
    expect(cart.isDineIn, isTrue);
    expect(cart.tableToken, 'tok');
  });

  test('AYNI restoran stoli skanerlansa savat SAQLANADI', () {
    cart.setQty(restaurantId: 'r1', productId: 'p1', qty: 2);

    cart.startTableSession(restaurantId: 'r1', token: 'tok', tableLabel: '3');

    expect(cart.totalQty, 2, reason: 'o\'sha restoran — savat yo\'qolmaydi');
    expect(cart.isDineIn, isTrue);
  });

  test('boshqa restoranga o\'tilsa stol seansi ham bekor bo\'ladi', () {
    cart.startTableSession(restaurantId: 'r1', token: 'tok');
    cart.setQty(restaurantId: 'r1', productId: 'p1', qty: 1);
    expect(cart.isDineIn, isTrue);

    cart.setQty(restaurantId: 'r2', productId: 'p9', qty: 1);

    expect(cart.isDineIn, isFalse,
        reason: 'stol boshqa restoranniki edi — u bilan ketishi kerak');
    expect(cart.tableToken, isNull);
  });
}

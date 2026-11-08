import 'package:chust_customer/payment_nav_policy.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  const ret = 'https://api.ondex.uz/payments/return';

  test('to\'lov domeni WebView ichida', () {
    expect(decidePaymentNav('https://pay2.octo.uz/pay/abc', returnUrl: ret), PayNav.navigate);
    expect(decidePaymentNav('https://acs.humocard.uz/3ds', returnUrl: ret), PayNav.navigate);
    expect(decidePaymentNav('https://checkout.multicard.uz/x', returnUrl: ret), PayNav.navigate);
  });

  test('http hech qachon', () {
    expect(decidePaymentNav('http://pay2.octo.uz/pay/abc', returnUrl: ret), PayNav.block);
  });

  test('noma\'lum domen tashqi brauzerga, o\'xshash domen ham', () {
    expect(decidePaymentNav('https://evil.com/card', returnUrl: ret), PayNav.external);
    expect(decidePaymentNav('https://octo.uz.evil.com/', returnUrl: ret), PayNav.external);
    expect(decidePaymentNav('https://fakeocto.uz/', returnUrl: ret), PayNav.external);
  });

  test('qaytish manzili origin+yo\'l bo\'yicha', () {
    expect(decidePaymentNav('$ret?status=ok', returnUrl: ret), PayNav.finish);
    expect(isReturnUrl('https://api.ondex.uz.evil.com/payments/return', ret), isFalse);
    expect(isReturnUrl('https://api.ondex.uz/payments/returnX', ret), isFalse);
    expect(isReturnUrl('http://api.ondex.uz/payments/return', ret), isFalse);
    expect(isReturnUrl('https://ondex.uz/x', 'https://ondex.uz'), isTrue);
    expect(isReturnUrl('https://ondex.uz.evil.com/', 'https://ondex.uz'), isFalse);
    expect(isReturnUrl('https://x/', ''), isFalse);
    // production qiymati (OCTO_RETURN_URL, 2026-11-08)
    const prod = 'https://api.ondex.uz/orders';
    expect(decidePaymentNav('https://api.ondex.uz/orders?status=paid', returnUrl: prod), PayNav.finish);
    expect(decidePaymentNav('https://api.ondex.uz/orders/123', returnUrl: prod), PayNav.finish);
    expect(isReturnUrl('https://api.ondex.uz/ordersX', prod), isFalse);
  });

  test('ilovaning o\'z sxemasi va xavfli sxemalar bloklanadi', () {
    for (final u in [
      'ondex://agent-link?c=sir',
      'javascript:alert(1)',
      'file:///data/data/x',
      'data:text/html,<b>x</b>',
      'intent://x#Intent;scheme=ondex;package=com.ondex.customer;end',
      'intent://x#Intent;scheme=https;package=com.ondex.customer.dev;end',
    ]) {
      expect(decidePaymentNav(u, returnUrl: ret), PayNav.block, reason: u);
    }
  });

  test('bank ilovasi sxemalari tizimga', () {
    expect(decidePaymentNav('uzcard://pay?id=1', returnUrl: ret), PayNav.launchApp);
    expect(decidePaymentNav('intent://pay#Intent;scheme=humo;package=uz.humo.app;end', returnUrl: ret),
        PayNav.launchApp);
  });

  test('pay_url faqat https', () {
    expect(isAcceptablePayUrl('https://pay2.octo.uz/pay/a'), isTrue);
    expect(isAcceptablePayUrl('http://pay2.octo.uz/pay/a'), isFalse);
    expect(isAcceptablePayUrl('javascript:x'), isFalse);
    expect(isAcceptablePayUrl(''), isFalse);
  });
}

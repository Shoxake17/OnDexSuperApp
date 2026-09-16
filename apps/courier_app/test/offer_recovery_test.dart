import 'package:chust_courier/offer_recovery.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  final t0 = DateTime(2026, 9, 15, 12);
  final before = t0.subtract(const Duration(seconds: 5));
  final after = t0.add(const Duration(milliseconds: 300));
  const offerA = {'order_id': 'A', 'expires_in_sec': 14};
  const offerB = {'order_id': 'B', 'expires_in_sec': 18};

  OfferRecoveryAction act({
    Map<String, dynamic>? shown,
    DateTime? shownAt,
    Map<String, dynamic>? server,
  }) =>
      offerRecoveryAction(shown: shown, shownAt: shownAt, server: server, requestedAt: t0);

  test('ilova qayta ochildi, taklif serverda ochiq — ko\'rsatiladi', () {
    expect(act(server: offerA), OfferRecoveryAction.show);
  });

  test('shu taklif ekranda — faqat soniyalar yangilanadi (ovoz qayta chalinmaydi)', () {
    expect(act(shown: offerA, shownAt: before, server: offerA), OfferRecoveryAction.updateSeconds);
  });

  test('ekranda eski taklif, serverda boshqasi — yangisi ko\'rsatiladi', () {
    expect(act(shown: offerA, shownAt: before, server: offerB), OfferRecoveryAction.show);
  });

  test('serverda taklif yo\'q — ekrandagi eski taklif yopiladi', () {
    expect(act(shown: offerA, shownAt: before, server: null), OfferRecoveryAction.dismiss);
    expect(act(shown: offerA, shownAt: before, server: {'order_id': ''}), OfferRecoveryAction.dismiss);
  });

  test('taklif so\'rovdan KEYIN kelgan bo\'lsa "yo\'q" javobi uni yopmaydi', () {
    expect(act(shown: offerA, shownAt: after, server: null), OfferRecoveryAction.none);
  });

  test('hech narsa yo\'q', () {
    expect(act(), OfferRecoveryAction.none);
  });
}

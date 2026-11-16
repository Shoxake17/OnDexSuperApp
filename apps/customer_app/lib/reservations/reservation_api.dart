import '../api.dart';

/// Stol broni endpointlari (RS-3, `routes_reservations.go`).
extension ReservationApi on CustomerApi {
  /// Kun ([date] Toshkent sanasi) bo'yicha bo'sh vaqtlar va zaklad qoidalari.
  /// Bron yoqilmagan restoranda 422 (`ApiException`).
  Future<Map<String, dynamic>> reservationAvailability(String restaurantId, DateTime date, int party) async {
    final d = '${date.year}-${date.month.toString().padLeft(2, '0')}-${date.day.toString().padLeft(2, '0')}';
    return Map<String, dynamic>.from(await send(
        'GET', '/restaurants/$restaurantId/reservations/availability?date=$d&party=$party') as Map);
  }

  /// Bron. [method] — `card` | `wallet` (zaklad bo'lsa). Javobda
  /// `reservation` va (karta bo'lsa) `pay` {pay_url, return_url}.
  Future<Map<String, dynamic>> createReservation(
    String restaurantId, {
    required DateTime startsAt,
    required int partySize,
    String method = '',
    String comment = '',
  }) async =>
      Map<String, dynamic>.from(await send('POST', '/restaurants/$restaurantId/reservations', {
        'starts_at': startsAt.toUtc().toIso8601String(),
        'party_size': partySize,
        if (method.isNotEmpty) 'method': method,
        if (comment.isNotEmpty) 'comment': comment,
      }) as Map);

  Future<List<Map<String, dynamic>>> myReservations() async {
    final list = await send('GET', '/me/reservations') as List<dynamic>? ?? [];
    return [for (final x in list) Map<String, dynamic>.from(x as Map)];
  }

  Future<Map<String, dynamic>> cancelMyReservation(String id) async =>
      Map<String, dynamic>.from(await send('POST', '/me/reservations/$id/cancel') as Map);

  /// To'lanmagan karta zakladi havolasi (qayta ochish).
  Future<Map<String, dynamic>> reservationPayLink(String id) async =>
      Map<String, dynamic>.from(await send('POST', '/me/reservations/$id/pay') as Map);
}

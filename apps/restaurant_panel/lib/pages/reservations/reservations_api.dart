import '../../api.dart';

/// Stol bronlari endpointlari (RS-3/RS-4, `routes_reservations.go`).
extension ReservationsApi on RestaurantApi {
  /// [from, to) oralig'idagi bronlar (mijoz ismi va telefoni bilan).
  Future<List<Map<String, dynamic>>> reservations(DateTime from, DateTime to) async {
    final q = 'from=${Uri.encodeQueryComponent(from.toUtc().toIso8601String())}'
        '&to=${Uri.encodeQueryComponent(to.toUtc().toIso8601String())}';
    final list = await send('GET', '/restaurants/$rid/reservations?$q') as List<dynamic>? ?? [];
    return [for (final x in list) Map<String, dynamic>.from(x as Map)];
  }

  Future<Map<String, dynamic>> reservationSettings() async =>
      Map<String, dynamic>.from(await send('GET', '/restaurants/$rid/reservation-settings') as Map);

  Future<Map<String, dynamic>> saveReservationSettings(Map<String, dynamic> body) async =>
      Map<String, dynamic>.from(await send('PUT', '/restaurants/$rid/reservation-settings', body) as Map);

  /// `seat` | `complete` | `no-show`.
  Future<Map<String, dynamic>> reservationAction(String id, String action) async =>
      Map<String, dynamic>.from(await send('POST', '/reservations/$id/$action') as Map);

  Future<Map<String, dynamic>> cancelReservation(String id, String reason) async =>
      Map<String, dynamic>.from(await send('POST', '/reservations/$id/cancel', {'reason': reason}) as Map);
}

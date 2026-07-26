import 'dart:convert';

import 'package:http/http.dart' as http;

/// Admin panel — faqat brauzerda (desktop) ishlatiladi, server localhost'da.
/// Production'da bu https://api.chustapp.uz kabi domen bo'ladi.
const baseUrl = 'http://localhost:8080';

class ApiException implements Exception {
  final String message;
  ApiException(this.message);
  @override
  String toString() => message;
}

class AdminApi {
  String? token;

  Map<String, String> get _headers => {
        'Content-Type': 'application/json',
        if (token != null) 'Authorization': 'Bearer $token',
      };

  Future<dynamic> _send(String method, String path,
      [Map<String, dynamic>? body]) async {
    final uri = Uri.parse('$baseUrl$path');
    final http.Response r;
    if (method == 'GET') {
      r = await http.get(uri, headers: _headers);
    } else if (method == 'DELETE') {
      r = await http.delete(uri, headers: _headers);
    } else {
      r = await http.post(uri, headers: _headers, body: jsonEncode(body ?? {}));
    }
    final data = r.body.isEmpty ? null : jsonDecode(utf8.decode(r.bodyBytes));
    if (r.statusCode >= 400) {
      throw ApiException(
          (data is Map && data['error'] != null) ? data['error'] : 'Server xatosi');
    }
    return data;
  }

  Future<String?> requestCode(String phone) async {
    final d = await _send('POST', '/auth/request-code', {'phone': phone});
    return d['dev_code'] as String?;
  }

  Future<Map<String, dynamic>> verify(String phone, String code) async {
    final d = await _send('POST', '/auth/verify', {'phone': phone, 'code': code});
    token = d['token'] as String;
    return Map<String, dynamic>.from(d);
  }

  /// Google Maps kaliti — server .env dan beradi (faqat login qilganlarga).
  Future<String> mapsApiKey() async {
    final d = await _send('GET', '/config/maps');
    return d['maps_api_key'] as String;
  }

  Future<Map<String, dynamic>> stats() async =>
      Map<String, dynamic>.from(await _send('GET', '/admin/stats'));

  Future<List<dynamic>> orders() async =>
      (await _send('GET', '/admin/orders')) as List<dynamic>? ?? [];

  Future<List<dynamic>> couriers() async =>
      (await _send('GET', '/admin/couriers')) as List<dynamic>? ?? [];

  Future<void> approveCourier(String id, bool approved) =>
      _send('POST', '/admin/couriers/$id/approve', {'approved': approved});

  Future<List<dynamic>> restaurants() async =>
      (await _send('GET', '/restaurants')) as List<dynamic>? ?? [];

  Future<Map<String, dynamic>> createRestaurant({
    required String name,
    required String address,
    required String phone,
    required String staffName,
    double lat = 41.0030,
    double lng = 71.2360,
  }) async =>
      Map<String, dynamic>.from(await _send('POST', '/admin/restaurants', {
        'name': name,
        'address': address,
        'phone': phone,
        'staff_name': staffName,
        'lat': lat,
        'lng': lng,
      }));

  Future<void> setRestaurantOpen(String id, bool open) =>
      _send('POST', '/admin/restaurants/$id/open', {'open': open});

  Future<void> deleteRestaurant(String id) =>
      _send('DELETE', '/admin/restaurants/$id');
}

final api = AdminApi();

String formatSum(int tiyin) {
  final sum = tiyin ~/ 100;
  return '${sum.toString().replaceAllMapped(RegExp(r'(\d)(?=(\d{3})+$)'), (m) => '${m[1]} ')} so\'m';
}

const statusLabels = {
  'created': 'Yangi',
  'accepted': 'Qabul qilindi',
  'preparing': 'Tayyorlanmoqda',
  'ready': 'Tayyor',
  'picked_up': 'Yo\'lda',
  'delivered': 'Yetkazildi',
  'cancelled': 'Bekor',
  'rejected': 'Rad etildi',
};

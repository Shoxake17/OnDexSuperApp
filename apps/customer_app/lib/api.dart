import 'dart:convert';

import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:http/http.dart' as http;

/// Backend manzili.
/// - Web (Chrome): localhost — avtomatik tanlanadi.
/// - Android emulyator: kompyuterdagi server `10.0.2.2` orqali ko'rinadi.
/// - Haqiqiy telefonda: kompyuteringizning tarmoqdagi IP sini yozing (masalan `http://192.168.1.5:8080`).
const baseUrl = kIsWeb ? 'http://localhost:8080' : 'http://10.0.2.2:8080';

String wsUrl(String token) =>
    '${baseUrl.replaceFirst('http', 'ws')}/ws?token=$token';

class ApiException implements Exception {
  final String message;
  ApiException(this.message);
  @override
  String toString() => message;
}

class ApiClient {
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

  /// SMS kod so'raydi. Dev rejimda server kodni javobda qaytaradi (dev_code).
  Future<String?> requestCode(String phone) async {
    final d = await _send('POST', '/auth/request-code', {'phone': phone});
    return d['dev_code'] as String?;
  }

  /// Kodni tekshiradi, token oladi. Javob: {token, user}.
  Future<Map<String, dynamic>> verify(String phone, String code) async {
    final d = await _send('POST', '/auth/verify', {'phone': phone, 'code': code});
    token = d['token'] as String;
    return Map<String, dynamic>.from(d);
  }

  Future<List<dynamic>> restaurants() async =>
      (await _send('GET', '/restaurants')) as List<dynamic>? ?? [];

  Future<List<dynamic>> menu(String restaurantId) async =>
      (await _send('GET', '/restaurants/$restaurantId/menu')) as List<dynamic>? ?? [];

  /// items: [{product_id, qty}] — narx yuborilmaydi, server katalogdan hisoblaydi.
  Future<Map<String, dynamic>> createOrder(
      List<Map<String, dynamic>> items, double lat, double lng) async {
    final d = await _send('POST', '/orders', {
      'items': items,
      'delivery_lat': lat,
      'delivery_lng': lng,
    });
    return Map<String, dynamic>.from(d);
  }

  Future<Map<String, dynamic>> getOrder(String id) async =>
      Map<String, dynamic>.from(await _send('GET', '/orders/$id'));
}

/// Butun ilova uchun bitta umumiy client.
final api = ApiClient();

String formatSum(int tiyin) {
  final sum = tiyin ~/ 100;
  return '${sum.toString().replaceAllMapped(RegExp(r'(\d)(?=(\d{3})+$)'), (m) => '${m[1]} ')} so\'m';
}

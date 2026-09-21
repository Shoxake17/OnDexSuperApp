// DEV vositasi: "Restoran" bo'limi va restoran modulini ChustApp serveri va
// loginisiz, SOXTA ma'lumot bilan ochadi (ko'rinishni tekshirish uchun).
// Production build'ga KIRMAYDI (`lib/` da emas). Hech qayerga so'rov ketmaydi:
// barcha HTTP so'rovlar shu yerdagi MockClient'ga tushadi.
//
//     flutter build windows --debug -t tool/restaurants_preview.dart
import 'dart:convert';

import 'package:chust_admin/screens/shell.dart';
import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

http.Response _json(Object body) => http.Response.bytes(
      utf8.encode(jsonEncode(body)),
      200,
      headers: {'content-type': 'application/json; charset=utf-8'},
    );

const _names = ['Book Cafe', 'Chust Osh Markazi', 'Navat Choyxona', 'Fayz Fast Food', 'Shirin Qandolat'];

Map<String, dynamic> _r(int i) => {
      'id': 'r$i',
      'name': _names[i],
      'address': 'M. Fayozov ko\'chasi, ${10 + i}-uy',
      'open': i != 3,
      'open_now': i != 3,
      'logo_url': '',
      'cover_url': '',
    };

Map<String, dynamic> _o(int n, String rid, String status) => {
      'id': 'o$n',
      'order_number': n,
      'created_at': DateTime.now().subtract(Duration(minutes: n * 7)).toUtc().toIso8601String(),
      'status': status,
      'total_tiyin': 4500000 + n * 120000,
      'restaurant_id': rid,
      'courier_id': status == 'delivered' ? 'c1' : '',
      'courier_name': status == 'delivered' ? 'Anvar' : '',
      'customer_name': 'Mijoz $n',
      'customer_phone': '+99890123${4000 + n}',
    };

Map<String, dynamic> _row(int i, int orders, int done, int cancelled, int inProgress, int news, int revenueSom) => {
      'id': 'r$i',
      'name': _names[i],
      'logo_url': '',
      'open': i != 3,
      'orders': orders,
      'new': news,
      'in_progress': inProgress,
      'accepted': inProgress + done,
      'completed': done,
      'cancelled': cancelled,
      'revenue_tiyin': revenueSom * 100,
      'avg_check_tiyin': done == 0 ? 0 : revenueSom * 100 ~/ done,
      'completion_rate': done + cancelled == 0 ? 0 : done * 100 ~/ (done + cancelled),
    };

Map<String, dynamic> _stats(String period) {
  final k = period == 'today' ? 1 : (period == '7d' ? 4 : (period == 'all' ? 14 : 8));
  final rows = [
    _row(0, 62 * k, 48 * k, 9 * k, 4 * k, 1 * k, 6900000 * k),
    _row(1, 41 * k, 33 * k, 5 * k, 2 * k, 1 * k, 4100000 * k),
    _row(2, 18 * k, 12 * k, 4 * k, 2 * k, 0, 1300000 * k),
    _row(3, 0, 0, 0, 0, 0, 0),
    _row(4, 7 * k, 5 * k, 2 * k, 0, 0, 350000 * k),
  ];
  int sum(String key) => rows.fold<int>(0, (s, r) => s + (r[key] as int));
  final done = sum('completed'), canc = sum('cancelled');
  final rev = sum('revenue_tiyin');
  return {
    'period': period,
    'from': period == 'all' ? '' : '2026-08-23',
    'to': '2026-09-21',
    'totals': {
      'orders': sum('orders'),
      'new': sum('new'),
      'in_progress': sum('in_progress'),
      'accepted': sum('accepted'),
      'completed': done,
      'cancelled': canc,
      'revenue_tiyin': rev,
      'avg_check_tiyin': done == 0 ? 0 : rev ~/ done,
      'completion_rate': done * 100 ~/ (done + canc),
    },
    'restaurants': rows,
    'statuses': {'created': sum('new'), 'accepted': 3, 'preparing': 5, 'ready': 2, 'delivered': done - 6, 'served': 6, 'cancelled': canc - 4, 'rejected': 4},
    'daily': [
      for (var i = 0; i < 14; i++)
        {
          'date': '2026-09-${(8 + i).toString().padLeft(2, '0')}',
          'orders': 20 + (i * 7) % 23,
          'completed': 15 + (i * 5) % 17,
          'revenue_tiyin': (2500000 + (i * 370000) % 1900000) * 100,
        },
    ],
    'couriers_online': 4,
    'couriers_pending': 2,
    'couriers_total': 11,
    'restaurants_total': 5,
    'restaurants_open': 4,
  };
}

void main() {
  http.runWithClient(
    () => runApp(MaterialApp(
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: const Color(0xFF0B5D1E)),
        useMaterial3: true,
      ),
      home: const AdminShell(),
    )),
    () => MockClient((req) async {
      final p = req.url.path;
      final rid = req.url.queryParameters['restaurant_id'];
      switch (p) {
        case '/restaurants':
          return _json([for (var i = 0; i < _names.length; i++) _r(i)]);
        case '/admin/accounts':
          return _json([
            for (var i = 0; i < 4; i++)
              {'entity_id': 'r$i', 'user_id': 'u$i', 'name': 'Mas\'ul $i', 'phone': '+99890111000$i'},
          ]);
        case '/admin/support/threads':
          return _json({
            'items': [
              {
                'restaurant_id': 'r0',
                'message_count': 3,
                'last_seq': 3,
                'last_sender': 'restaurant',
                'last_body': 'Printer ishlamayapti',
                'read_seq': 1,
                'peer_read_seq': 0,
                'unread': 2,
                'restaurant': {'id': 'r0', 'name': _names[0], 'address': '', 'logo_url': ''},
                'restaurant_online': true,
              },
            ],
            'unread_total': 2,
          });
        case '/admin/support/summary':
          return _json({'unread': 2});
        case '/admin/stats':
          return _json(_stats(req.url.queryParameters['period'] ?? '30d'));
        case '/admin/customers':
          return _json({'count': 0, 'items': []});
        case '/admin/orders':
          final all = [
            _o(101, 'r0', 'created'),
            _o(102, 'r0', 'preparing'),
            _o(103, 'r0', 'delivered'),
            _o(201, 'r1', 'created'),
          ];
          return _json(rid == null ? all : all.where((o) => o['restaurant_id'] == rid).toList());
        case '/admin/couriers':
          return _json([
            {'id': 'c1', 'name': 'Anvar', 'phone': '+998911112233', 'approved': true, 'available': true, 'restaurant_id': 'r0', 'user_id': 'cu1'},
          ]);
        case '/admin/waiters':
          return _json({
            'count': 1,
            'items': [
              {'id': 'w1', 'name': 'Olim Karimov', 'phone': '+998944445566', 'restaurant_id': 'r0', 'restaurant_name': _names[0], 'devices': [], 'created_at': '2026-09-01T10:00:00Z'},
            ],
          });
        case '/restaurants/r0/books/all':
          return _json([
            {'id': 'b1', 'title': 'O\'tkan kunlar', 'author': 'Abdulla Qodiriy', 'active': true, 'cover_url': ''},
          ]);
        case '/admin/support/threads/r0/messages':
          return _json({
            'items': [
              {
                'id': 'm1',
                'seq': 1,
                'sender': 'restaurant',
                'sender_name': _names[0],
                'body': 'Assalomu alaykum, printer ishlamayapti',
                'client_id': 'client-1-abcdefgh',
                'created_at': DateTime.now().toUtc().toIso8601String(),
              },
            ],
            'next_before': 0,
            'thread': {'restaurant_id': 'r0', 'message_count': 1, 'last_seq': 1, 'unread': 1, 'read_seq': 0, 'peer_read_seq': 0, 'last_sender': 'restaurant', 'last_body': 'x'},
            'restaurant': {'id': 'r0', 'name': _names[0]},
            'restaurant_online': true,
          });
      }
      return _json({'error': 'sinov: $p'});
    }),
  );
}

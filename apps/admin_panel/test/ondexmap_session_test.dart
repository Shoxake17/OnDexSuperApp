import 'dart:io';

import 'package:chust_admin/ondexmap/ondexmap_session_io.dart';
import 'package:flutter_test/flutter_test.dart';

// Lokal sessiya: token fayldan o'qiladi, manzil FAQAT loopback bo'lsa ishoniladi.
// Bu — panel adminlik tokenini qayerga yuborishini belgilaydigan chegara.

void main() {
  late Directory dir;

  setUp(() => dir = Directory.systemTemp.createTempSync('ondexmap_session_'));
  tearDown(() => dir.deleteSync(recursive: true));

  String write(String content) {
    final f = File('${dir.path}${Platform.pathSeparator}admin_session.json');
    f.writeAsStringSync(content);
    return f.path;
  }

  group('readOndexMapSession', () {
    test('to\'g\'ri fayl: token va loopback manzil', () {
      final s = readOndexMapSession(
          path: write('{"token":"  abc123  ","url":"http://127.0.0.1:8091"}'));
      expect(s, isNotNull);
      expect(s!.token, 'abc123', reason: 'token qirqiladi');
      expect(s.url, 'http://127.0.0.1:8091');
    });

    test('tashqi manzil TASHLANADI (token baribir o\'qiladi, lekin manzil emas)', () {
      final s = readOndexMapSession(
          path: write('{"token":"abc","url":"http://evil.example:8091"}'));
      expect(s, isNotNull);
      expect(s!.url, isNull,
          reason: 'fayldagi tashqi manzilga token yuborilmasligi kerak');
    });

    test('https va noto\'g\'ri manzillar rad etiladi', () {
      for (final bad in [
        'https://127.0.0.1:8091',
        'http://127.0.0.1.evil.com',
        'http://evil.com/127.0.0.1',
        'http://127.0.0.1@evil.com',
        'ftp://127.0.0.1',
        '',
        'not a url',
      ]) {
        final s = readOndexMapSession(path: write('{"token":"abc","url":${_q(bad)}}'));
        expect(s?.url, isNull, reason: '"$bad" qabul qilinmasligi kerak edi');
      }
    });

    test('bo\'sh token, buzuq JSON, JSON emas, fayl yo\'q → null', () {
      expect(readOndexMapSession(path: write('{"token":"   ","url":"http://127.0.0.1:1"}')), isNull);
      expect(readOndexMapSession(path: write('{"token":')), isNull);
      expect(readOndexMapSession(path: write('[1,2,3]')), isNull);
      expect(readOndexMapSession(path: write('')), isNull);
      expect(readOndexMapSession(path: '${dir.path}${Platform.pathSeparator}yoq.json'), isNull);
    });

    test('token satr bo\'lmasa (son) — null, yiqilmaydi', () {
      expect(readOndexMapSession(path: write('{"token":12345}')), isNull);
    });
  });

  group('isLoopbackHttp', () {
    test('ruxsat etilganlar', () {
      for (final ok in [
        'http://127.0.0.1:8091',
        'http://127.0.0.1',
        'http://localhost:8091',
        'http://[::1]:8091',
      ]) {
        expect(isLoopbackHttp(ok), isTrue, reason: ok);
      }
    });

    test('rad etilganlar', () {
      for (final bad in [
        'https://127.0.0.1',
        'http://192.168.0.5:8091',
        'http://0.0.0.0:8091',
        'http://localhost.evil.com',
        'http://127.0.0.1.evil.com',
        '//127.0.0.1',
        '',
      ]) {
        expect(isLoopbackHttp(bad), isFalse, reason: bad);
      }
    });
  });
}

/// JSON satr literali.
String _q(String s) => '"${s.replaceAll('\\', '\\\\').replaceAll('"', '\\"')}"';

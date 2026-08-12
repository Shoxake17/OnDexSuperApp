import 'dart:async';
import 'dart:js_interop';

import 'package:web/web.dart' as web;

import '../api.dart';

bool _loaded = false;
Future<void>? _loading;

/// Google Maps JS kutubxonasini yuklaydi. Kalit frontend kodida YO'Q:
/// backend .env dan o'qib, faqat tizimga kirgan foydalanuvchiga beradi
/// (GET /config/maps). Skript sahifaga shu yerdan dinamik qo'shiladi.
Future<void> ensureGoogleMapsLoaded() {
  if (_loaded) return Future.value();
  return _loading ??= _load();
}

Future<void> _load() async {
  final key = await api.mapsApiKey();
  final completer = Completer<void>();
  final script = web.HTMLScriptElement()
    ..src = 'https://maps.googleapis.com/maps/api/js?key=$key&language=uz'
    ..async = true;
  script.addEventListener(
      'load', ((web.Event _) => completer.complete()).toJS);
  script.addEventListener(
      'error',
      ((web.Event _) => completer.completeError(
          ApiException('Google Maps skriptini yuklab bo\'lmadi'))).toJS);
  web.document.head!.append(script);
  await completer.future;
  _loaded = true;
}

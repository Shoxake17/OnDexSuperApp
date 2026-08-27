import 'dart:async';
import 'dart:js_interop';

import 'package:web/web.dart' as web;

import '../api_client.dart';

bool _loaded = false;
Future<void>? _loading;

/// Google Maps JS kutubxonasini sahifaga qo'shadi.
///
/// ┌─ KALIT KODDA YO'Q ────────────────────────────────────────────────┐
/// Kalit backend `.env` da turadi va faqat tizimga kirgan
/// foydalanuvchiga beriladi (`GET /config/maps`). Shuning uchun u
/// [keyProvider] orqali olinadi — har ilova o'z klientini uzatadi
/// (`api.mapsApiKey`).
/// └───────────────────────────────────────────────────────────────────┘
///
/// Takroriy chaqiruv xavfsiz: skript bir marta qo'shiladi, parallel
/// chaqiruvlar bitta `Future` ni kutadi.
Future<void> ensureGoogleMapsLoaded(Future<String> Function() keyProvider) {
  if (_loaded) return Future.value();
  return _loading ??= _load(keyProvider);
}

Future<void> _load(Future<String> Function() keyProvider) async {
  final key = await keyProvider();
  final completer = Completer<void>();
  final script = web.HTMLScriptElement()
    ..src = 'https://maps.googleapis.com/maps/api/js?key=$key&language=uz'
    ..async = true;
  script.addEventListener('load', ((web.Event _) => completer.complete()).toJS);
  script.addEventListener(
      'error',
      ((web.Event _) => completer.completeError(
          ApiException('Google Maps skriptini yuklab bo\'lmadi'))).toJS);
  web.document.head!.append(script);
  try {
    await completer.future;
    _loaded = true;
  } catch (_) {
    // Keyingi urinish qaytadan yuklay olsin — aks holda bir marta
    // yiqilgan skript butun seans davomida bloklanib qolardi.
    _loading = null;
    rethrow;
  }
}

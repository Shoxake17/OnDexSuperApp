/// XARITA MANBASI — serverdagi `GOOGLE_MAP` bayrog'i, ilova tomonida.
///
/// ┌─ QANDAY ISHLAYDI ──────────────────────────────────────────────────┐
/// Server `.env` dagi `GOOGLE_MAP` ni `GET /config/maps` orqali beradi:
///   `provider: "google"`   — buyurtma oqimi ekranlari Google bilan;
///   `provider: "ondexmap"` — o'sha ekranlar OnDexMap bilan.
/// Ilovani qayta qurish shart emas: qiymat serverda o'zgaradi, ilova
/// keyingi yangilashda oladi.
///
/// ┌─ XAVFSIZLIK QOIDALARI ─────────────────────────────────────────────┐
///  1. Oxirgi ma'lum qiymat qurilmada saqlanadi — server vaqtincha
///     javob bermasa ham xarita tanlovi o'zgarmaydi.
///  2. Hech qachon qiymat yo'q bo'lsa (birinchi ishga tushish, server
///     ishlamaydi) — Google: bu ilgarigi production xatti-harakati.
///  3. OnDexMap ishlamasa (uslub yuklanmadi, javob kelmadi) ekran
///     `reportOndexFailure` ni chaqiradi: keyingi [failureCooldown]
///     davomida butun ilova Google'da turadi, keyin yana urinib ko'riladi.
///     Foydalanuvchi xarita o'rniga xato ko'rmaydi.
///  4. Istisno — "OnDex Xarita" ekrani: u hech qachon Google'ga
///     tushmaydi (xato va "Qayta urinish" ko'rsatadi). Bu qaror ekran
///     o'zida; shu servis uni majburlamaydi.
/// └────────────────────────────────────────────────────────────────────┘
library;

import 'dart:async';
import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';

/// Xarita manbasi.
enum MapProvider { google, ondexmap }

/// `ApiClient.send` bilan bir xil imzo (servis `ApiClient` ga bog'lanmasin).
typedef MapConfigSender =
    Future<dynamic> Function(
      String method,
      String path, [
      Map<String, dynamic>? body,
    ]);

/// OnDexMap uslubi javob beryaptimi (	rue — ha).
typedef MapStyleProbe = Future<bool> Function(String styleUrl);

/// Standart tekshiruv: uslub JSON'i 5 soniyada keladimi va to'g'ri shaklda
/// (ersion + layers) emasmi. Faqat uslubni oladi, plitkalarni emas.
Future<bool> probeMapStyle(String styleUrl) async {
  try {
    final r = await http
        .get(Uri.parse(styleUrl))
        .timeout(const Duration(seconds: 5));
    if (r.statusCode != 200) return false;
    final j = jsonDecode(utf8.decode(r.bodyBytes));
    return j is Map && j['version'] != null && j['layers'] is List;
  } catch (_) {
    return false;
  }
}

@immutable
class MapConfig {
  const MapConfig({
    required this.provider,
    required this.googleAvailable,
    required this.ondexApiBase,
    required this.ondexStyleUrl,
  });

  /// Buyurtma oqimi ekranlari uchun faol manba.
  final MapProvider provider;

  /// Serverda Google kaliti bor (OnDexMap'dan Google'ga qaytish mumkin).
  final bool googleAvailable;

  /// OnDexMap server manzili va vektor uslubi.
  final String ondexApiBase;
  final String ondexStyleUrl;

  /// Hech narsa ma'lum bo'lmaganda: Google (ilgarigi xatti-harakat).
  static const MapConfig defaults = MapConfig(
    provider: MapProvider.google,
    googleAvailable: true,
    ondexApiBase: 'https://maps.ondex.uz',
    ondexStyleUrl: 'https://maps.ondex.uz/tiles/style.json',
  );

  /// Server javobidan. Eski server (yangi maydonlarsiz) → Google.
  /// Tushunarsiz yoki buzuq qiymat hech qachon xato otmaydi.
  factory MapConfig.fromJson(Object? json) {
    if (json is! Map) return defaults;
    final od = json['ondexmap'];
    final base = od is Map ? _httpsUrl(od['api_base']) : null;
    final style = od is Map ? _httpsUrl(od['style_url']) : null;
    return MapConfig(
      provider: json['provider'] == 'ondexmap'
          ? MapProvider.ondexmap
          : MapProvider.google,
      // Eski server `maps_api_key` ni beradi, `google_available` ni emas.
      googleAvailable: json['google_available'] is bool
          ? json['google_available'] as bool
          : json['maps_api_key'] != null,
      ondexApiBase: base ?? defaults.ondexApiBase,
      ondexStyleUrl: style ?? defaults.ondexStyleUrl,
    );
  }

  Map<String, dynamic> toJson() => {
    'provider': provider.name,
    'google_available': googleAvailable,
    'ondexmap': {'api_base': ondexApiBase, 'style_url': ondexStyleUrl},
  };

  /// Faqat `https://` manzillar qabul qilinadi: buzilgan javob yoki
  /// o'rtadagi odam ilovani boshqa serverga yo'naltira olmasin.
  static String? _httpsUrl(Object? v) {
    if (v is! String) return null;
    final u = Uri.tryParse(v.trim());
    if (u == null || u.scheme != 'https' || u.host.isEmpty) return null;
    return u.toString();
  }

  @override
  bool operator ==(Object other) =>
      other is MapConfig &&
      other.provider == provider &&
      other.googleAvailable == googleAvailable &&
      other.ondexApiBase == ondexApiBase &&
      other.ondexStyleUrl == ondexStyleUrl;

  @override
  int get hashCode =>
      Object.hash(provider, googleAvailable, ondexApiBase, ondexStyleUrl);
}

class MapConfigService extends ChangeNotifier {
  MapConfigService({DateTime Function()? clock, MapStyleProbe? probe})
    : _now = clock ?? DateTime.now,
      _probe = probe ?? probeMapStyle;

  /// Butun ilova uchun bitta nusxa.
  static final MapConfigService instance = MapConfigService();

  static const _prefsKey = 'map_config_v1';

  /// OnDexMap xatosidan keyin shuncha vaqt Google'da turamiz.
  static const Duration failureCooldown = Duration(minutes: 5);

  final DateTime Function() _now;
  final MapStyleProbe _probe;
  MapConfig _config = MapConfig.defaults;
  DateTime? _ondexFailedAt;
  MapConfigSender? _send;

  /// Oxirgi ma'lum sozlama.
  MapConfig get config => _config;

  /// OnDexMap yaqinda ishlamagan (hozir Google'da turibmiz).
  bool get ondexRecentlyFailed {
    final t = _ondexFailedAt;
    return t != null && _now().difference(t) < failureCooldown;
  }

  /// Buyurtma oqimi ekranlari (manzil tanlash, kuzatuv, kuryer xaritasi)
  /// OnDexMap bilan chizilsinmi.
  ///
  /// Serverda `ondexmap` tanlangan VA yaqinda xato bo'lmagan bo'lsagina.
  bool get useOndexMap =>
      _config.provider == MapProvider.ondexmap && !ondexRecentlyFailed;

  /// Qurilmadagi oxirgi qiymatni o'qiydi. Hech qachon xato otmaydi —
  /// o'qib bo'lmasa [MapConfig.defaults] qoladi.
  Future<void> init() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final raw = prefs.getString(_prefsKey);
      if (raw != null) {
        _config = MapConfig.fromJson(jsonDecode(raw));
        notifyListeners();
      }
    } catch (_) {
      // Buzuq kesh — standart qiymat bilan davom etamiz.
    }
  }

  /// Serverdan yangilaydi (`GET /config/maps`, login talab qiladi).
  /// Hech qachon xato otmaydi: tarmoq/401/503 bo'lsa oxirgi ma'lum
  /// qiymat o'zgarishsiz qoladi.
  Future<void> refresh(MapConfigSender send) async {
    _send = send;
    try {
      final data = await send('GET', '/config/maps');
      final fresh = MapConfig.fromJson(data);
      if (fresh != _config) {
        _config = fresh;
        notifyListeners();
        try {
          final prefs = await SharedPreferences.getInstance();
          await prefs.setString(_prefsKey, jsonEncode(fresh.toJson()));
        } catch (_) {}
      }
    } catch (_) {
      // 503 "Google kaliti yo'q" yoki tarmoq — oldingi qiymat qoladi.
      return;
    }
    await _probeIfOndex();
  }

  /// OnDexMap tanlangan bo'lsa, xaritani ochishdan OLDIN uslub javob
  /// berishini tekshiradi: yiqilgan bo'lsa foydalanuvchi bo'sh xarita
  /// ko'rmaydi — ilova darhol Google'da turadi.
  Future<void> _probeIfOndex() async {
    if (_config.provider != MapProvider.ondexmap) return;
    final ok = await _probe(_config.ondexStyleUrl);
    if (ok) {
      reportOndexRecovered();
    } else {
      reportOndexFailure('probe', 'uslub javob bermadi');
    }
  }

  /// OnDexMap ishlamadi: ilova [failureCooldown] davomida Google'ga
  /// o'tadi va serverga (log uchun) xabar beriladi. `surface`:
  /// address | tracking | courier | admin | web.
  void reportOndexFailure(String surface, String reason) {
    final first = !ondexRecentlyFailed;
    _ondexFailedAt = _now();
    if (first) notifyListeners();
    final send = _send;
    if (send == null) return;
    // Xabar — ixtiyoriy: yuborilmasa ham foydalanuvchi uchun hech narsa
    // o'zgarmaydi.
    unawaited(() async {
      try {
        await send('POST', '/config/maps/fallback', {
          'surface': surface,
          'reason': reason,
        });
      } catch (_) {}
    }());
  }

  /// OnDexMap yana ishlayotganini ko'rsatadigan muvaffaqiyatli yuklanish.
  void reportOndexRecovered() {
    if (_ondexFailedAt == null) return;
    _ondexFailedAt = null;
    notifyListeners();
  }

  @visibleForTesting
  void setConfigForTest(MapConfig c) {
    _config = c;
    notifyListeners();
  }
}

import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:http/http.dart' as http;

import 'ondexmap_session.dart';

/// OnDexMap admin serveri bilan ishlash: foydalanuvchilar yuborgan
/// ob'ektlarni MODERATSIYA qilish.
///
/// ┌─ ARXITEKTURA ──────────────────────────────────────────────────────┐
/// Bu klient ChustApp API'siga UMUMAN tegmaydi (`api.dart` dan alohida):
/// OnDexMap — boshqa loyiha, boshqa baza, boshqa server. Faqat lokal
/// admin serveri (`cmd/admin`, `127.0.0.1:8091`) bilan gaplashadi.
/// └────────────────────────────────────────────────────────────────────┘
///
/// ┌─ XAVFSIZLIK ───────────────────────────────────────────────────────┐
/// • Token lokal sessiya FAYLIDAN olinadi (`ondexmap_session.dart`),
///   binarga yozilmaydi, ekranga chiqarilmaydi va logga tushmaydi.
/// • Token FAQAT `X-API-Key` sarlavhasida ketadi, URL'da EMAS.
/// • Manzil har so'rovda tekshiriladi: faqat `http` va faqat loopback.
///   Sessiya fayli buzilgan yoki almashtirilgan bo'lsa ham token tashqi
///   serverga ketmaydi.
/// • Har so'rovda sessiya QAYTA o'qiladi: OnDexMap serveri qayta
///   ishga tushganda token o'zgaradi.
/// • Server matnlari (nom, tavsif, xato) faqat `Text` vidjetida
///   ko'rsatiladi — HTML/uslub sifatida talqin qilinmaydi.
/// └────────────────────────────────────────────────────────────────────┘

/// Foydalanuvchiga ko'rsatilishi MUMKIN xato (matn — bizning o'z xabarimiz).
class ModerationException implements Exception {
  final String message;

  /// 401: token yaroqsiz (odatda OnDexMap serveri qayta ishga tushgan).
  final bool unauthorized;

  const ModerationException(this.message, {this.unauthorized = false});

  @override
  String toString() => message;
}

/// Ob'ekt turi: kalit, yorliq va shu turda bo'lishi mumkin maydonlar.
class KindMeta {
  final String key;
  final String label;
  final Set<String> allowed;

  const KindMeta({required this.key, required this.label, required this.allowed});

  factory KindMeta.fromJson(Map<String, dynamic> j) => KindMeta(
        key: '${j['key']}',
        label: '${j['label']}',
        allowed: {
          for (final f in (j['allowed'] as List? ?? const [])) '$f',
        },
      );
}

/// Forma qoidalari (serverdan; Flutter'da ikkinchi nusxa yo'q).
class PlacesMeta {
  final List<KindMeta> kinds;
  final List<String> categories;
  final int maxPhotos;

  const PlacesMeta({required this.kinds, required this.categories, required this.maxPhotos});

  factory PlacesMeta.fromJson(Map<String, dynamic> j) => PlacesMeta(
        kinds: [
          for (final k in (j['kinds'] as List? ?? const []))
            KindMeta.fromJson(k as Map<String, dynamic>),
        ],
        categories: [for (final c in (j['categories'] as List? ?? const [])) '$c'],
        maxPhotos: (j['max_photos'] as num?)?.toInt() ?? 4,
      );

  KindMeta? kind(String key) {
    for (final k in kinds) {
      if (k.key == key) return k;
    }
    return null;
  }
}

/// Bitta ob'ekt: taklif (karantin) yoki xaritadagi tasdiqlangani.
class PlaceRecord {
  final String id;
  final String kind;
  final String kindLabel;
  final String name;
  final String category;
  final String description;
  final String phone;
  final String hours;
  final String street;
  final String house;
  final double lat;
  final double lng;
  final int photos;
  final String createdAt;

  /// Faqat takliflar uchun.
  final String hint;
  final String reviewNote;

  const PlaceRecord({
    required this.id,
    required this.kind,
    required this.kindLabel,
    required this.name,
    required this.category,
    required this.description,
    required this.phone,
    required this.hours,
    required this.street,
    required this.house,
    required this.lat,
    required this.lng,
    required this.photos,
    required this.createdAt,
    this.hint = '',
    this.reviewNote = '',
  });

  factory PlaceRecord.fromJson(Map<String, dynamic> j) {
    String s(String k) => (j[k] as String?) ?? '';
    return PlaceRecord(
      id: s('id'),
      kind: s('kind'),
      kindLabel: s('kind_label'),
      name: s('name'),
      category: s('category'),
      description: s('description'),
      phone: s('phone'),
      hours: s('hours'),
      street: s('street'),
      house: s('house'),
      lat: (j['lat'] as num?)?.toDouble() ?? 0,
      lng: (j['lng'] as num?)?.toDouble() ?? 0,
      photos: (j['photos'] as num?)?.toInt() ?? 0,
      createdAt: s('created_at'),
      hint: s('hint'),
      reviewNote: s('review_note'),
    );
  }

  /// Ro'yxatda ko'rinadigan sarlavha: nom, bo'lmasa ko'cha+uy, bo'lmasa tur.
  String get title {
    if (name.isNotEmpty) return name;
    final addr = [street, house].where((e) => e.isNotEmpty).join(' ');
    return addr.isNotEmpty ? addr : kindLabel;
  }
}

class SubmissionList {
  final List<PlaceRecord> items;

  /// Kutilayotgan takliflarning UMUMIY soni (badge uchun).
  final int pending;

  const SubmissionList({required this.items, required this.pending});
}

/// Muharrir xaritasi sozlamasi (serverdan). SIR YO'Q: sun'iy yo'ldosh
/// tile'lari OMMAVIY API proksisi orqali olinadi, provayder kaliti serverda
/// qoladi. `satelliteUrl == null` — manba sozlanmagan (tugma ko'rsatilmaydi).
class EditorConfig {
  final String? satelliteUrl;
  final String satelliteAttribution;
  final int satelliteMaxZoom;

  const EditorConfig({
    this.satelliteUrl,
    this.satelliteAttribution = '',
    this.satelliteMaxZoom = 18,
  });

  factory EditorConfig.fromJson(Map<String, dynamic> j) {
    final url = j['satellite_url'];
    return EditorConfig(
      // Manzil faqat loopback `http` bo'lsa qabul qilinadi: token bu manzilga
      // ketmaydi, lekin baribir tashqi serverga tile so'rovi yuborilmasin.
      satelliteUrl: url is String && url.isNotEmpty && _isLoopbackTemplate(url) ? url : null,
      satelliteAttribution: (j['satellite_attribution'] as String?) ?? '',
      satelliteMaxZoom: int.tryParse('${j['satellite_maxzoom'] ?? ''}') ?? 18,
    );
  }

  /// `{z}/{x}/{y}` shabloni `Uri.parse` ni buzadi — o'rinbosarlarni almashtirib tekshiramiz.
  static bool _isLoopbackTemplate(String t) =>
      isLoopbackHttp(t.replaceAll('{z}', '0').replaceAll('{x}', '0').replaceAll('{y}', '0'));
}

/// Xaritadagi saqlangan obyekt (mahalla yoki ko'cha).
class MapFeature {
  final String id;
  final String name;

  /// Manba (provenans): official | survey | community | osm.
  final String source;

  /// Faqat ko'cha: kocha | shox_kocha | tor_kocha | xiyobon | maydon.
  final String streetKind;

  /// Server bergan xom GeoJSON geometriya. Tahrirlanmagan obyekt qayta
  /// saqlanganda AYNAN shu yuboriladi (qayta kodlash yo'qotishsiz).
  final Map<String, dynamic> geometry;

  const MapFeature({
    required this.id,
    required this.name,
    required this.source,
    required this.streetKind,
    required this.geometry,
  });

  factory MapFeature.fromJson(Map<String, dynamic> f) {
    final p = (f['properties'] as Map?)?.cast<String, dynamic>() ?? const {};
    final g = (f['geometry'] as Map?)?.cast<String, dynamic>() ?? const {};
    return MapFeature(
      id: '${p['id'] ?? ''}',
      name: '${p['name'] ?? ''}',
      source: '${p['source'] ?? ''}',
      streetKind: '${p['kind'] ?? ''}',
      geometry: g,
    );
  }
}

/// Muharrir qatlamlari (serverdagi `kind`).
const editorLayers = ['mahalla', 'street'];

/// Taklif/ob'ekt identifikatori — UUID. Yo'lga qo'yishdan oldin tekshiriladi.
final _uuid = RegExp(r'^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$');

class ModerationApi {
  /// Sessiya faylida manzil bo'lmasa ishlatiladigan manzil (loopback bo'lishi SHART).
  final String defaultUrl;
  final OndexMapSession? Function() _session;

  ModerationApi({required this.defaultUrl, OndexMapSession? Function()? session})
      : _session = session ?? readOndexMapSession;

  static const _timeout = Duration(seconds: 20);

  Future<PlacesMeta> meta() async =>
      PlacesMeta.fromJson(await _json('GET', '/api/places/meta'));

  /// `status`: pending | approved | rejected.
  Future<SubmissionList> submissions(String status) async {
    final j = await _json('GET', '/api/submissions', query: {'status': status});
    return SubmissionList(
      items: [
        for (final e in (j['submissions'] as List? ?? const []))
          PlaceRecord.fromJson(e as Map<String, dynamic>),
      ],
      pending: (j['pending'] as num?)?.toInt() ?? 0,
    );
  }

  /// Xaritadagi tasdiqlangan ob'ektlar (eng yangi 100 ta).
  Future<List<PlaceRecord>> places() async {
    final j = await _json('GET', '/api/places');
    return [
      for (final e in (j['places'] as List? ?? const []))
        PlaceRecord.fromJson(e as Map<String, dynamic>),
    ];
  }

  /// Taklif rasmi (server tozalagan JPEG baytlari).
  Future<Uint8List> submissionPhoto(String id, int n) async {
    if (!_uuid.hasMatch(id) || n < 0 || n > 9) {
      throw const ModerationException('Rasm topilmadi');
    }
    final res = await _send('GET', '/api/submissions/$id/photos/$n');
    return res.bodyBytes;
  }

  /// Taklifni tasdiqlaydi. `edit` — YAKUNIY qiymatlar (moderator tuzatgan):
  /// server ularni ommaviy API'dagi qoidalar bilan qayta tekshiradi.
  /// Qaytadi: yangi ob'ekt identifikatori.
  Future<String> approve({
    required String id,
    required Map<String, Object> edit,
    required List<int> keepPhotos,
  }) async {
    final j = await _json('POST', '/api/submissions/approve', body: {
      'id': id,
      'edit': edit,
      'source': 'community',
      'keep_photos': keepPhotos,
    });
    return '${j['place_id']}';
  }

  Future<void> reject(String id, String note) =>
      _json('POST', '/api/submissions/reject', body: {'id': id, 'note': note});

  Future<void> deletePlace(String id) =>
      _json('POST', '/api/places/delete', body: {'id': id});

  // ── Muharrir: mahalla va ko'chalar ────────────────────────────────

  Future<EditorConfig> editorConfig() async =>
      EditorConfig.fromJson(await _json('GET', '/api/config'));

  /// Qatlamdagi barcha obyektlar (`mahalla` | `street`).
  Future<List<MapFeature>> features(String layer) async {
    _checkLayer(layer);
    final j = await _json('GET', '/api/features', query: {'kind': layer});
    return [
      for (final f in (j['features'] as List? ?? const []))
        MapFeature.fromJson((f as Map).cast<String, dynamic>()),
    ];
  }

  /// Yaratadi (`id` bo'sh) yoki yangilaydi. `geometry` — GeoJSON; server uni
  /// MATN sifatida kutadi. Qaytadi: obyekt identifikatori.
  Future<String> saveFeature({
    required String layer,
    String id = '',
    required String name,
    required String source,
    String? streetKind,
    required Map<String, dynamic> geometry,
  }) async {
    _checkLayer(layer);
    final j = await _json('POST', '/api/$layer', body: {
      'id': id,
      'name': name,
      'source': source,
      if (layer == 'street') 'kind': streetKind ?? 'kocha',
      'geometry': jsonEncode(geometry),
    });
    return '${j['id']}';
  }

  Future<void> deleteFeature(String layer, String id) {
    _checkLayer(layer);
    return _json('POST', '/api/delete', body: {'kind': layer, 'id': id});
  }

  /// Ko'chaga muqobil nom (xalq nomi, eski nom, ...).
  Future<void> addAlias({
    required String streetId,
    required String alias,
    required String kind,
    required String source,
  }) =>
      _json('POST', '/api/alias', body: {
        'street_id': streetId,
        'alias': alias,
        'kind': kind,
        'source': source,
      });

  static void _checkLayer(String layer) {
    if (!editorLayers.contains(layer)) {
      throw const ModerationException('Noma\'lum qatlam');
    }
  }

  // ── Ichki ──────────────────────────────────────────────────────────

  Future<Map<String, dynamic>> _json(
    String method,
    String path, {
    Map<String, String>? query,
    Object? body,
  }) async {
    final res = await _send(method, path, query: query, body: body);
    try {
      final v = jsonDecode(utf8.decode(res.bodyBytes));
      return v is Map<String, dynamic> ? v : <String, dynamic>{};
    } catch (_) {
      throw const ModerationException('Server kutilmagan javob berdi');
    }
  }

  Future<http.Response> _send(
    String method,
    String path, {
    Map<String, String>? query,
    Object? body,
  }) async {
    final s = _session();
    if (s == null) {
      throw const ModerationException(
        'Lokal sessiya topilmadi. OnDexMap admin serveri ishga tushirilganmi?\n'
        'cd F:\\OnDexMap\ngo run ./cmd/admin',
      );
    }

    // Manzil HAR SAFAR tekshiriladi (token shu manzilga ketadi).
    final base = s.url ?? defaultUrl;
    if (!isLoopbackHttp(base)) {
      throw const ModerationException(
        'OnDexMap manzili ishonchsiz (faqat 127.0.0.1 ruxsat etiladi)',
      );
    }
    final uri = Uri.parse(base).replace(path: path, queryParameters: query);

    final client = http.Client();
    try {
      final headers = {'X-API-Key': s.token, 'Accept': 'application/json'};
      final http.Response res;
      if (method == 'GET') {
        res = await client.get(uri, headers: headers).timeout(_timeout);
      } else {
        res = await client
            .post(
              uri,
              headers: {...headers, 'Content-Type': 'application/json'},
              body: jsonEncode(body ?? const {}),
            )
            .timeout(_timeout);
      }
      if (res.statusCode >= 200 && res.statusCode < 300) return res;
      throw ModerationException(
        _errorText(res),
        unauthorized: res.statusCode == 401,
      );
    } on ModerationException {
      rethrow;
    } on TimeoutException {
      throw const ModerationException('OnDexMap serveri vaqtida javob bermadi.');
    } on http.ClientException {
      throw const ModerationException(
        'OnDexMap serveriga ulanib bo\'lmadi. Uni ishga tushiring:\n'
        'cd F:\\OnDexMap\ngo run ./cmd/admin',
      );
    } finally {
      client.close();
    }
  }

  static String _errorText(http.Response res) {
    if (res.statusCode == 401) {
      return 'Sessiya qabul qilinmadi (OnDexMap serveri qayta ishga tushgan '
          'bo\'lishi mumkin). Sahifani yangilang.';
    }
    try {
      final v = jsonDecode(utf8.decode(res.bodyBytes));
      if (v is Map && v['error'] is String && (v['error'] as String).isNotEmpty) {
        return v['error'] as String;
      }
    } catch (_) {
      // JSON emas — pastdagi umumiy matn.
    }
    return 'Xato ${res.statusCode}';
  }
}

import 'dart:io';

import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:http/http.dart' as http;

/// Restoranning 3D maketi — OnDex ILOVASI ICHIDA.
///
/// ┌─ MAKET RESTORANGA BOG'LANGAN ───────────────────────────────────┐
/// Har bir maket O'Z restoraniga tegishli: manzil va SHA-256
/// restoran yozuvida (`scene_3d_url`, `scene_3d_sha256`) turadi.
///
/// Bog'lanish backendda, ilovada EMAS. Aks holda har yangi kafe
/// qo'shilganda ilovaning yangi versiyasini chiqarish kerak bo'lardi.
/// Endi admin panelda manzilni yozish kifoya.
///
/// Maketi yo'q restoranda taklif UMUMAN ko'rsatilmaydi — begona
/// maket boshqa kafega bog'lanib qolmaydi.
/// └─────────────────────────────────────────────────────────────────┘
///
/// ┌─ NEGA MAKET ILOVA BILAN KELMAYDI ───────────────────────────────┐
/// 3D dvigatel (~68 MB) ilova bilan birga keladi — busiz 3D umuman
/// ishlamaydi. Maketning O'ZI esa ~104 MB va u faqat foydalanuvchi
/// so'raganda yuklanadi. 3D ni ochmaydiganlar uni ko'tarib yurmaydi.
/// └─────────────────────────────────────────────────────────────────┘
///
/// ┌─ YUKLANGAN FAYL TEKSHIRILADI ───────────────────────────────────┐
/// Maket ichida BAJARILADIGAN kod bor (Godot skriptlari). Kutilgan
/// SHA-256 API'dan keladi va fayl ikki marta tekshiriladi: yuklab
/// olingach hamda ochishdan oldin.
///
/// Ishonch zanjiri: ilova → HTTPS → bizning API → xesh → fayl.
/// Faylni almashtirish uchun API ni ham egallash kerak. Server
/// tomonda esa manzil faqat R2 domenida bo'lishi majburiy.
/// └─────────────────────────────────────────────────────────────────┘
class BookCafeGame {
  BookCafeGame._();

  static const _channel = MethodChannel('uz.ondex.customer/game');

  static bool get supported =>
      !kIsWeb && defaultTargetPlatform == TargetPlatform.android;

  /// Restoran yozuvidan maket ma'lumotini o'qiydi.
  ///
  /// Maket yo'q bo'lsa `null` — chaqiruvchi shunda hech narsa
  /// ko'rsatmaydi.
  static SceneInfo? infoOf(Map<String, dynamic> restaurant) {
    final url = (restaurant['scene_3d_url'] as String?)?.trim() ?? '';
    final sha = (restaurant['scene_3d_sha256'] as String?)?.trim() ?? '';
    if (url.isEmpty || sha.length != 64) return null;
    final bytes = (restaurant['scene_3d_bytes'] as num?)?.toInt() ?? 0;
    return SceneInfo(url: url, sha256: sha.toLowerCase(), bytes: bytes);
  }

  /// Maket qurilmada bormi va kutilgan nusxami.
  static Future<SceneStatus> status({
    required String restaurantId,
    required String sha256,
  }) async {
    if (!supported) return const SceneStatus.unsupported();
    try {
      final res = await _channel.invokeMapMethod<String, dynamic>(
        'sceneStatus',
        {'restaurantId': restaurantId, 'sha256': sha256},
      );
      if (res == null) return const SceneStatus.unsupported();
      return SceneStatus(
        ready: res['ready'] == true,
        reason: (res['reason'] as String?) ?? 'unknown',
      );
    } on PlatformException catch (e) {
      return SceneStatus(ready: false, reason: e.code);
    } on MissingPluginException {
      return const SceneStatus.unsupported();
    }
  }

  /// Maketni yuklab oladi va tekshiradi.
  ///
  /// [onProgress] 0.0 dan 1.0 gacha; hajm noma'lum bo'lsa `null`.
  /// Xatolik kodi qaytadi, muvaffaqiyatda `null`.
  static Future<String?> download({
    required String restaurantId,
    required SceneInfo info,
    void Function(double? progress)? onProgress,
  }) async {
    if (!supported) return 'unsupported';

    Map<String, dynamic>? paths;
    try {
      paths = await _channel.invokeMapMethod<String, dynamic>(
        'scenePaths',
        {'restaurantId': restaurantId},
      );
    } on PlatformException catch (e) {
      return e.code;
    }
    final partPath = paths?['part'] as String?;
    if (partPath == null) return 'no_path';

    final client = http.Client();
    IOSink? sink;
    try {
      final res = await client.send(http.Request('GET', Uri.parse(info.url)));
      if (res.statusCode != 200) return 'http_${res.statusCode}';

      final total = res.contentLength;
      var received = 0;

      final file = File(partPath);
      // Yarim qolgan eski nusxa ustiga yozilmasin.
      if (await file.exists()) await file.delete();
      sink = file.openWrite();

      await for (final chunk in res.stream) {
        sink.add(chunk);
        received += chunk.length;
        if (onProgress != null) {
          onProgress(total == null || total == 0 ? null : received / total);
        }
      }
      await sink.flush();
      await sink.close();
      sink = null;
    } on SocketException {
      return 'network';
    } catch (_) {
      return 'download_failed';
    } finally {
      await sink?.close();
      client.close();
    }

    // Tekshiruv va joyiga qo'yish Kotlin tomonda — ochishdan oldingi
    // tekshiruv bilan AYNAN bir xil kod.
    try {
      await _channel.invokeMethod<bool>('installScene', {
        'restaurantId': restaurantId,
        'sha256': info.sha256,
      });
      return null;
    } on PlatformException catch (e) {
      return e.code;
    }
  }

  /// Maketni o'chiradi — joy bo'shatish uchun.
  static Future<void> delete(String restaurantId) async {
    if (!supported) return;
    try {
      await _channel.invokeMethod<bool>('deleteScene', {
        'restaurantId': restaurantId,
      });
    } on PlatformException {
      // O'chirilmasa ham ilova ishlayveradi.
    }
  }

  /// 3D sayohatni ochadi va u YOPILGUNCHA kutadi.
  ///
  /// ┌─ NEGA KUTILADI ──────────────────────────────────────────────────┐
  /// O'yin savat va sevimlilarni serverdan O'ZI olmaydi va yozmaydi:
  /// buning uchun foydalanuvchi tokeni kerak bo'lardi va u o'yinga
  /// ataylab berilmaydi (`ondex_config.gd` izohiga qarang).
  ///
  /// Shuning uchun hozirgi holat KIRISHDA uzatiladi, o'zgargani esa
  /// o'yin yopilgach qaytariladi. Serverga yozishni OnDex o'z
  /// huquqlari bilan bajaradi.
  /// └──────────────────────────────────────────────────────────────────┘
  static Future<GameResult> open({
    required String restaurantId,
    required String sha256,
    String tableLabel = '',
    String apiBase = '',
    String name = '',
    String logoUrl = '',
    Set<String> favorites = const {},
    Map<String, int> cart = const {},
  }) async {
    if (!supported) return const GameResult(error: 'unsupported');
    try {
      final res = await _channel.invokeMapMethod<String, dynamic>('open', {
        'restaurantId': restaurantId,
        'sha256': sha256,
        'tableLabel': tableLabel,
        'apiBase': apiBase,
        'name': name,
        'logoUrl': logoUrl,
        'favorites': favorites.join(','),
        'cart': cart.entries.map((e) => '${e.key}:${e.value}').join(','),
      });
      return GameResult.parse(res?['state'] as String?);
    } on PlatformException catch (e) {
      return GameResult(error: e.code);
    } on MissingPluginException {
      return const GameResult(error: 'unsupported');
    }
  }
}

/// O'yin qaytargan holat.
///
/// ┌─ NIMA UCHUN QAYTA TEKSHIRILADI ────────────────────────────────────┐
/// Ma'lumot ALOHIDA JARAYONDAN keladi va uni telefondagi boshqa ilova
/// ham yozgan bo'lishi mumkin. Shuning uchun bu yerda hech narsaga
/// ishonilmaydi: ID shakli, miqdor oralig'i va ro'yxat uzunligi
/// tekshiriladi.
///
/// Narx umuman kutilmaydi va o'qilmaydi — u serverdan olinadi.
/// └────────────────────────────────────────────────────────────────────┘
@immutable
class GameResult {
  const GameResult({
    this.error,
    this.restaurantId = '',
    this.cart = const {},
    this.favorites = const {},
    this.hasState = false,
    this.orderRequested = false,
  });

  /// Xatolik kodi; muvaffaqiyatda `null`.
  final String? error;

  /// Holat qaysi restoranga tegishli.
  final String restaurantId;

  /// Mahsulot ID → miqdor.
  final Map<String, int> cart;

  /// Sevimli mahsulot ID'lari.
  final Set<String> favorites;

  /// O'yin holat qaytardimi. `false` bo'lsa hech narsa o'zgartirilmaydi:
  /// bo'sh holatni "hammasini o'chir" deb tushunish xato bo'lardi.
  final bool hasState;

  /// Foydalanuvchi maket ichida "Buyurtma berish" ni bosdimi.
  ///
  /// O'yin buyurtmani O'ZI bermaydi - unda foydalanuvchi tokeni yo'q.
  /// Bu faqat niyat belgisi: OnDex savat ekranini ochadi va buyurtma
  /// o'zining odatiy oqimida rasmiylashtiriladi.
  final bool orderRequested;

  static final RegExp _id = RegExp(r'^[A-Za-z0-9_-]{1,64}$');

  /// Eng ko'p element — buzilgan fayl xotirani to'ldirmasin.
  static const _maxItems = 200;

  static GameResult parse(String? raw) {
    if (raw == null || raw.isEmpty) return const GameResult();
    try {
      final data = jsonDecode(raw);
      if (data is! Map) return const GameResult();

      final cart = <String, int>{};
      final items = data['items'];
      if (items is List) {
        for (final row in items.take(_maxItems)) {
          if (row is! Map) continue;
          final id = '${row['id'] ?? ''}';
          final qty = row['qty'];
          if (!_id.hasMatch(id) || qty is! int || qty < 1 || qty > 99) {
            continue;
          }
          cart[id] = qty;
        }
      }

      final favs = <String>{};
      final list = data['favorites'];
      if (list is List) {
        for (final row in list.take(_maxItems)) {
          final id = '$row';
          if (_id.hasMatch(id)) favs.add(id);
        }
      }

      final rid = '${data['restaurant'] ?? ''}';
      return GameResult(
        restaurantId: _id.hasMatch(rid) ? rid : '',
        cart: cart,
        favorites: favs,
        hasState: true,
        orderRequested: data['order'] == true,
      );
    } on FormatException {
      // Buzilgan JSON — holat yo'q deb hisoblanadi.
      return const GameResult();
    }
  }
}
/// Restoran yozuvidagi maket ma'lumoti.
@immutable
class SceneInfo {
  const SceneInfo({
    required this.url,
    required this.sha256,
    required this.bytes,
  });

  final String url;
  final String sha256;
  final int bytes;

  /// Foydalanuvchiga ko'rsatiladigan hajm.
  String get sizeText =>
      bytes > 0 ? '~${(bytes / (1024 * 1024)).round()} MB' : '';
}

/// Maketning qurilmadagi holati.
@immutable
class SceneStatus {
  const SceneStatus({required this.ready, required this.reason});

  const SceneStatus.unsupported()
      : ready = false,
        reason = 'unsupported';

  final bool ready;
  final String reason;

  /// 3D ni umuman taklif qilish mumkinmi.
  bool get available => reason != 'unsupported';
}

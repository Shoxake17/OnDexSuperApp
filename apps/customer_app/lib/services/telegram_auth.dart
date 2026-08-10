import 'dart:async';

import 'package:app_links/app_links.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:url_launcher/url_launcher.dart';

import '../api.dart';

/// "Telegram bilan kirish".
///
/// ── NEGA RASMIY LOGIN WIDGET EMAS ──────────────────────────────────
/// Telegram'ning rasmiy "Login Widget"i — bu VEB sahifa va u ishlashi
/// uchun botga DOMEN biriktirilgan bo'lishi hamda widget aynan o'sha
/// domendan ochilishi kerak. Ilova hozir lokal serverda ishlaydi,
/// Telegram esa domenni tekshiradi va lokal sahifani rad etadi.
///
/// ── OQIM (foydalanuvchi ko'radigan) ────────────────────────────────
///   1. Telegram logotipini bosadi;
///   2. Telegram ochiladi, "Start" bosadi;
///   3. "Raqamni ulashish" tugmasini bosadi;
///   4. "OnDex'ga qaytish" tugmasini bosadi -> tizimga kirdi.
///
/// Hech qanday kod kiritilmaydi.
///
/// ── FISHINGGA QARSHI (foydalanuvchi ko'rmaydigan) ──────────────────
/// 4-qadamdagi tugma oddiy havola EMAS: uning ichida bir martalik
/// maxfiy kalit bor (`ondex://auth?c=...`). Natijani olish uchun
/// kuzatish tokeni YETARLI EMAS, shu kalit ham kerak.
///
/// Busiz hujum shunday bo'lardi:
///
///   hujumchi ilovada oqimni boshlaydi -> deep linkni QURBONGA
///   yuboradi -> qurbon Start bosib raqamini ulashadi -> hujumchi o'z
///   kuzatish tokeni bilan QURBONNING akkauntiga kiradi.
///
/// Endi kalit tasdiqlagan odamning QURILMASIDA ochiladi, ya'ni
/// qurbonning telefonidagi ilovaga tushadi. Hujumchining qurilmasi uni
/// hech qachon ko'rmaydi va uning so'rovi hech qachon yakunlanmaydi.
///
/// Qadam QO'SHILMAGAN: foydalanuvchi baribir "qaytish" tugmasini
/// bosadi, kalit esa ko'rinmas holda o'sha bosishda keladi.
class TelegramAuth {
  TelegramAuth._();

  /// Kutishning umumiy muddati. Backenddagi so'rov 10 daqiqa yashaydi,
  /// lekin foydalanuvchini bunchalik kutdirish ma'nosiz.
  static const _timeout = Duration(minutes: 3);
  static const _pollInterval = Duration(seconds: 2);

  /// Kutilayotgan so'rovning kuzatish tokeni. DISKDA saqlanadi, chunki
  /// Android ilovani fonda (foydalanuvchi Telegramda ekan) o'ldirishi
  /// mumkin — qaytganda oqim "sovuq" holatdan tiklanishi kerak.
  static const _pendingKey = 'telegram_login_token';

  static final _links = AppLinks();

  /// Telegram bor-yo'qligini tekshiradi.
  ///
  /// Android 11+ da bu `AndroidManifest.xml` dagi `<queries>` blokiga
  /// bog'liq (u yerdagi izohga qarang).
  static Future<bool> isInstalled() async {
    try {
      return await canLaunchUrl(Uri.parse('tg://resolve?domain=telegram'));
    } catch (_) {
      return false;
    }
  }

  /// `ondex://auth?c=...` havolasidan maxfiy kalitni oladi.
  static String? _secretOf(Uri? uri) {
    if (uri == null || uri.scheme != 'ondex') return null;
    final c = uri.queryParameters['c'];
    return (c == null || c.isEmpty) ? null : c;
  }

  /// Butun oqimni bajaradi. Muvaffaqiyatda foydalanuvchi ma'lumotini
  /// qaytaradi; foydalanuvchi tasdiqlamasa `null`.
  ///
  /// IKKI YO'L BIR VAQTDA kuzatiladi:
  ///
  ///   * SO'RAB TURISH (polling) — foydalanuvchi ilovaga QO'LDA
  ///     qaytsa ham kirish yakunlanadi. Asosiy yo'l shu.
  ///   * DEEP LINK — bot "OnDex'ga qaytish" tugmasini yuborgan bo'lsa,
  ///     u maxfiy kalitni olib keladi. Kalit production'da MAJBURIY
  ///     (backend `Verifier.requireSecret` izohiga qarang), dev'da esa
  ///     tugma umuman yuborilmaydi va polling o'zi yetarli.
  ///
  /// Shu sabab ikkalasi ham kutiladi: qaysi biri birinchi ishlasa,
  /// kirish o'sha orqali yakunlanadi.
  static Future<Map<String, dynamic>?> signIn() async {
    final start = await api.telegramLoginStart();

    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_pendingKey, start.token);

    // Tinglashni Telegram OCHILISHIDAN OLDIN boshlaymiz: foydalanuvchi
    // juda tez tasdiqlasa, havola biz tayyor bo'lgunimizcha kelib
    // qolishi mumkin edi.
    var secret = '';
    final sub = _links.uriLinkStream.listen((uri) {
      final c = _secretOf(uri);
      if (c != null) secret = c;
    }, onError: (_) {});

    try {
      final opened = await launchUrl(
        Uri.parse(start.deepLink),
        mode: LaunchMode.externalApplication,
      );
      if (!opened) {
        await prefs.remove(_pendingKey);
        throw TelegramAuthFailure('Telegram ochilmadi');
      }

      final deadline = DateTime.now().add(_timeout);
      while (DateTime.now().isBefore(deadline)) {
        await Future<void>.delayed(_pollInterval);
        try {
          final res = await api.telegramLoginStatus(start.token, secret);
          if (res != null) {
            await prefs.remove(_pendingKey);
            return res;
          }
        } on ApiException catch (e) {
          if (e.statusCode == 404) {
            await prefs.remove(_pendingKey);
            throw TelegramAuthFailure(
                'So\'rov eskirdi — qaytadan urinib ko\'ring');
          }
          // Tarmoq uzilishi (ilova fonda edi) kutishni TO'XTATMAYDI —
          // keyingi urinishda tiklanishi mumkin.
          if (!e.isNetwork) rethrow;
        }
      }
      await prefs.remove(_pendingKey);
      return null; // tasdiqlanmadi
    } finally {
      await sub.cancel();
    }
  }

  /// SOVUQ start: ilova fonda o'ldirilgan bo'lsa, "OnDex'ga qaytish"
  /// tugmasi uni qaytadan ishga tushiradi va havola `signIn()` dagi
  /// oqim emas, SHU yo'l orqali keladi.
  ///
  /// Kirish ekrani ochilishida chaqiriladi. Kutilayotgan so'rov
  /// bo'lmasa yoki havola oddiy bo'lsa `null` qaytaradi.
  static Future<Map<String, dynamic>?> completeIfReturning() async {
    final Uri? initial;
    try {
      initial = await _links.getInitialLink();
    } catch (_) {
      return null;
    }
    final c = _secretOf(initial);
    if (c == null) return null;

    final prefs = await SharedPreferences.getInstance();
    final token = prefs.getString(_pendingKey);
    if (token == null || token.isEmpty) return null;

    return _finish(token, c, prefs);
  }

  static Future<Map<String, dynamic>?> _finish(
      String token, String secret, SharedPreferences prefs) async {
    await prefs.remove(_pendingKey);
    try {
      // Bot tasdiqni YOZIB BO'LGANIDAN keyin havola yuboriladi, ya'ni
      // natija shu payt tayyor. Baribir bir necha marta urinamiz:
      // havola tasdiq yozilishidan oldin ochilib qolishi nazariy
      // jihatdan mumkin (tarmoq kechikishi).
      for (var i = 0; i < 5; i++) {
        final res = await api.telegramLoginStatus(token, secret);
        if (res != null) return res;
        await Future<void>.delayed(const Duration(milliseconds: 600));
      }
      return null;
    } on ApiException catch (e) {
      if (e.statusCode == 404) {
        throw TelegramAuthFailure('So\'rov eskirdi — qaytadan urinib ko\'ring');
      }
      rethrow;
    }
  }
}

class TelegramAuthFailure implements Exception {
  final String message;
  TelegramAuthFailure(this.message);
  @override
  String toString() => message;
}

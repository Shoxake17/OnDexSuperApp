import 'package:flutter/foundation.dart';
import 'package:audioplayers/audioplayers.dart';

/// Yangi buyurtma qo'ng'irog'i uchun yagona (global) pleyer.
///
/// `audioplayers` — bir kodni web'da ham, kelajakda desktop (`.exe`,
/// `flutter build windows`) qurilishida ham ishlatish uchun tanlangan
/// (`dart:html` faqat web'da ishlaydi, desktop build'da kompilyatsiya
/// bo'lmaydi). Desktopda brauzerning "autoplay siyosati" umuman yo'q —
/// pastdagi [unlock] shunchaki HECH NARSA QILMAYDI (darhol qaytadi),
/// [start] esa har doim to'g'ridan-to'g'ri ishlaydi. Faqat WEB'da
/// [unlock] haqiqiy ma'no kasb etadi.
class RingSound {
  RingSound._();

  static final AudioPlayer player = AudioPlayer()
    ..setReleaseMode(ReleaseMode.loop);

  static bool _prefixFixed = false;
  static bool _unlocked = false;

  static void _ensurePrefix() {
    if (_prefixFixed) return;
    _prefixFixed = true;
    // audioplayers `AssetSource` manzilini "assets/" + prefiks bilan
    // quradi; standart prefiks ham "assets/" bo'lgani uchun ikki marta
    // qo'shilib, fayl topilmaydi (bu BARCHA platformalarda — web, desktop,
    // mobil — bir xil, chunki asset pubspec'da "sound/ding.mp3" deb
    // e'lon qilingan, "assets/sound/ding.mp3" emas). Prefiksni bo'shatib,
    // to'g'ri yo'lga tushiramiz.
    AudioCache.instance.prefix = '';
  }

  /// Foydalanuvchi qayerga bo'lsa ham bosganda chaqiriladi.
  ///
  /// MUHIM: gesture (bosish) AYNAN shu — keyinchalik haqiqiy jiringlaydigan
  /// — pleyerning o'zida sodir bo'lishi SHART (web brauzerlari ruxsatni
  /// "shu audio elementga gesture orqali ruxsat berildimi" darajasida
  /// tekshirishi mumkin). Shuning uchun bitta umumiy pleyer ishlatiladi —
  /// alohida/vaqtinchalik pleyer bilan sinalgan, lekin bu ovozni butunlay
  /// o'chirib qo'ygan (asosiy pleyer hech qachon ochilmay qolgan).
  static Future<void> unlock() async {
    if (kIsWeb) {
      _ensurePrefix();
      if (_unlocked) return;
      _unlocked = true;
      try {
        await player.play(AssetSource('sound/ding.mp3'), volume: 0);
        await player.stop();
        await player.setVolume(1);
      } catch (e) {
        _unlocked = false;
        debugPrint('RingSound.unlock xato: $e');
      }
    }
  }

  static Future<void> start() async {
    _ensurePrefix();
    try {
      await player.play(AssetSource('sound/ding.mp3'));
    } catch (e) {
      // Xato yutilmaydi — kamida devtools/konsolda ko'rinadi.
      debugPrint('RingSound.start xato: $e');
    }
  }

  static Future<void> stop() => player.stop();
}

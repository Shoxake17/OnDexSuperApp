import 'dart:convert';
import 'dart:io';

/// OnDexMap admin serverining lokal sessiyasi.
///
/// ┌─ NEGA FAYL ────────────────────────────────────────────────────────┐
/// OnDexMap admin serveri ishga tushganda BIR MARTALIK token yaratib,
/// foydalanuvchi profilidagi faylga yozadi
/// (`%LOCALAPPDATA%\OnDexMap\admin_session.json`). Kalit Flutter binariga
/// YOZILMAYDI: EXE ochib o'qiladi va har bir o'rnatilgan nusxaga tarqalardi.
///
/// Nega token serverdan so'ralmaydi: brauzerdagi zararli sahifa ham
/// `http://127.0.0.1:8091` ga so'rov yubora oladi, lekin lokal FAYLNI
/// o'qiy olmaydi. Ya'ni fayl orqali qo'l berish haqiqiy chegara.
///
/// Bu fayl OnDexMap tomonidagi `internal/localsession` bilan SHARTNOMA.
/// Muharrir (WebView) ham, moderatsiya (nativ) ham SHU BITTA o'qish
/// mantig'idan foydalanadi: xavfsizlik tekshiruvi ikki joyda yozilsa,
/// biri eskirib qolardi.
/// └────────────────────────────────────────────────────────────────────┘
class OndexMapSession {
  final String token;

  /// Serverning o'zi yozgan manzil. `null` — fayldagi manzil ishonchsiz
  /// (loopback emas) yoki yo'q; u holda chaqiruvchining standart manzili
  /// ishlatiladi.
  final String? url;

  const OndexMapSession({required this.token, this.url});
}

/// Manzil FAQAT loopback va FAQAT `http` bo'lsa yaroqli.
///
/// Manzil FAYLDAN keladi, shuning uchun ko'r-ko'rona ochilmaydi: aks holda
/// shu faylni yozish imkoniyatiga ega narsa panelni tashqi saytga burib,
/// unga admin tokenini ko'rsatib qo'yardi. `localhost` ham ruxsat: u
/// dev muhitida `127.0.0.1` ga tushadi.
bool isLoopbackHttp(String raw) {
  final uri = Uri.tryParse(raw);
  if (uri == null || uri.scheme != 'http') return false;
  // `Uri.host` IPv6 uchun qavssiz qaytaradi (`::1`).
  return uri.host == '127.0.0.1' || uri.host == 'localhost' || uri.host == '::1';
}

/// Sessiya faylini o'qiydi.
///
/// Fayl bo'lmasa yoki buzuq bo'lsa `null` — bu XATO EMAS: server hali
/// ishga tushirilmagan bo'lishi mumkin va u holda ulanish xatosi baribir
/// sababni ko'rsatadi. Har chaqiriqda QAYTA o'qiladi: server qayta
/// ishga tushganda token o'zgaradi, eskisini ishlatish "sessiya qabul
/// qilinmadi" holatida qotib qolish degani.
///
/// [path] — testlar uchun.
OndexMapSession? readOndexMapSession({String? path}) {
  var file = path;
  if (file == null) {
    final base = Platform.environment['LOCALAPPDATA'];
    if (base == null || base.isEmpty) return null;
    file = '$base\\OnDexMap\\admin_session.json';
  }
  final f = File(file);
  if (!f.existsSync()) return null;

  try {
    final m = jsonDecode(f.readAsStringSync());
    if (m is! Map<String, dynamic>) return null;
    final token = (m['token'] as String?)?.trim() ?? '';
    if (token.isEmpty) return null;

    final raw = (m['url'] as String?) ?? '';
    return OndexMapSession(token: token, url: isLoopbackHttp(raw) ? raw : null);
  } catch (_) {
    // Yarim yozilgan yoki buzilgan fayl — sessiyasiz davom etamiz.
    return null;
  }
}

// Aloqa havolalari — FAQAT oq ro'yxatdagi shakllar.
//
// Qoidalar serverdagi `internal/support` bilan AYNAN bir xil. Server
// allaqachon tekshiradi, bu yerda esa ikkinchi qatlam: havola ochuvchi
// (`url_launcher`) operatsion tizimga buyruq beradi va noto'g'ri qiymat
// (`javascript:`, `file:`, `ms-settings:`) kutilmagan dasturni ishga
// tushirishi mumkin edi.

final _phoneRe = RegExp(r'^\+998\d{9}$');
final _telegramRe = RegExp(r'^[A-Za-z][A-Za-z0-9_]{3,30}[A-Za-z0-9]$');
final _emailRe = RegExp(r'^[A-Za-z0-9._%+-]{1,64}@[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,24}$');

bool isSupportPhone(String v) => _phoneRe.hasMatch(v);

bool isSupportTelegram(String v) => v.length <= 32 && _telegramRe.hasMatch(v) && !v.contains('__');

bool isSupportEmail(String v) => v.length <= 254 && _emailRe.hasMatch(v) && !v.contains('..');

Uri? supportTelUri(String phone) => isSupportPhone(phone) ? Uri(scheme: 'tel', path: phone) : null;

Uri? supportTelegramUri(String username) =>
    isSupportTelegram(username) ? Uri.https('t.me', '/$username') : null;

Uri? supportMailUri(String email, {String? subject}) {
  if (!isSupportEmail(email)) return null;
  return Uri(
    scheme: 'mailto',
    path: email,
    query: subject == null ? null : 'subject=${Uri.encodeComponent(subject)}',
  );
}

/// "+998901234567" -> "+998 90 123 45 67". Boshqa shakl o'zgarishsiz.
String formatSupportPhone(String phone) {
  if (!isSupportPhone(phone)) return phone;
  return '+998 ${phone.substring(4, 6)} ${phone.substring(6, 9)} '
      '${phone.substring(9, 11)} ${phone.substring(11)}';
}

/// Admin formasidagi kiritma: "@ondex", "t.me/ondex", "https://t.me/ondex"
/// -> "ondex". Yakuniy tekshiruv serverda.
String normalizeTelegramInput(String raw) {
  var s = raw.trim();
  final lower = s.toLowerCase();
  for (final p in const ['https://t.me/', 'http://t.me/', 'https://telegram.me/', 'http://telegram.me/', 't.me/', 'telegram.me/', '@']) {
    if (lower.startsWith(p)) {
      s = s.substring(p.length);
      break;
    }
  }
  if (s.endsWith('/')) s = s.substring(0, s.length - 1);
  return s;
}

/// "+998 90 123-45-67" -> "+998901234567" (faqat tekshirish uchun).
String normalizePhoneInput(String raw) {
  var p = raw.trim().replaceAll(RegExp(r'[\s\-()]'), '');
  if (p.startsWith('998') && p.length == 12) p = '+$p';
  return p;
}

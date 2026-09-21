/// WEB: lokal sessiya fayli mavjud emas (brauzer fayl tizimini o'qiy olmaydi).
///
/// Web'da OnDexMap bo'limi ishlamaydi: panel HTTPS'da, admin server esa
/// `http://127.0.0.1` da (mixed content) — `ondexmap_surface_web.dart`
/// izohiga qarang. Bu stub faqat kompilyatsiya uchun.
class OndexMapSession {
  final String token;
  final String? url;

  const OndexMapSession({required this.token, this.url});
}

bool isLoopbackHttp(String raw) {
  final uri = Uri.tryParse(raw);
  if (uri == null || uri.scheme != 'http') return false;
  return uri.host == '127.0.0.1' || uri.host == 'localhost' || uri.host == '::1';
}

OndexMapSession? readOndexMapSession({String? path}) => null;

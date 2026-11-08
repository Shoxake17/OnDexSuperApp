/// To'lov WebView'ining navigatsiya qoidasi (bug.md 85-band).
///
/// ┌─ NEGA ALOHIDA, SOF FUNKSIYA ──────────────────────────────────────┐
/// Qoida ekrandan ajratilgan, chunki u pul yo'lida turadi va UNIT TEST
/// bilan qulflanishi kerak. Ekran faqat qarorni bajaradi.
///
/// Siyosat:
///  * `http://` — HECH QACHON (karta sahifasi shifrlanmagan ulanishda
///    ochilmasin).
///  * `https://` + oq ro'yxatdagi to'lov/bank domeni — WebView ichida.
///  * `https://` + noma'lum domen — WebView'da EMAS, tashqi brauzerda
///    (manzil satri ko'rinadi — foydalanuvchi qayerda ekanini biladi).
///    Bloklash o'rniga brauzer: 3-D Secure sahifasi ro'yxatda yo'q bank
///    domenida bo'lsa ham to'lov yo'qolmaydi.
///  * Ilovaning O'Z sxemalari (`ondex://`, ilova paketiga `intent://`)
///    — bloklanadi: to'lov sahifasi ilovaga buyruq yubora olmasin.
///  * `javascript:`, `file:`, `data:`, `content:` — bloklanadi.
///  * Boshqa sxemalar (bank ilovalari: `uzcard://`, `intent://...`) —
///    tizimga uzatiladi.
/// └───────────────────────────────────────────────────────────────────┘
library;

enum PayNav { navigate, finish, external, launchApp, block }

/// To'lov va bank sahifalari joylashgan domenlar (va ularning
/// subdomenlari). Yangi provayder qo'shilsa — shu yerga va testga.
const kPaymentHostSuffixes = <String>[
  'octo.uz',
  'multicard.uz',
  'multicard.app',
  'payme.uz',
  'click.uz',
  'uzum.uz',
  'uzumbank.uz',
  'uzcard.uz',
  'humocard.uz',
  'humo.uz',
  'kapitalbank.uz',
  'ondex.uz',
];

const _ownSchemes = <String>{'ondex', 'ondexgo', 'ondexpro', 'chust'};
const _deadSchemes = <String>{'javascript', 'file', 'data', 'content', 'blob'};

bool isPaymentHost(String host) {
  final h = host.toLowerCase().replaceAll(RegExp(r'\.$'), '');
  for (final s in kPaymentHostSuffixes) {
    if (h == s || h.endsWith('.$s')) return true;
  }
  return false;
}

/// `pay_url` faqat `https://` va to'lov domeni bo'lsa qabul qilinadi.
/// Mobil chaqiruvchilar (checkout, tracking) shuni ishlatadi.
bool isAcceptablePayUrl(String url) {
  final u = Uri.tryParse(url.trim());
  return u != null && u.scheme == 'https' && u.host.isNotEmpty;
}

/// `url` qaytish manzilimi — origin (sxema+host+port) TO'LIQ va yo'l
/// prefiks bo'yicha. `https://ondex.uz.evil.com` mos KELMAYDI.
bool isReturnUrl(String url, String returnUrl) {
  if (returnUrl.trim().isEmpty) return false;
  final u = Uri.tryParse(url);
  final r = Uri.tryParse(returnUrl.trim());
  if (u == null || r == null || r.host.isEmpty) return false;
  if (u.scheme != r.scheme || u.host.toLowerCase() != r.host.toLowerCase() || u.port != r.port) {
    return false;
  }
  final rp = r.path.isEmpty ? '/' : r.path;
  final up = u.path.isEmpty ? '/' : u.path;
  if (rp == '/') return true;
  return up == rp || up.startsWith(rp.endsWith('/') ? rp : '$rp/');
}

PayNav decidePaymentNav(String url, {required String returnUrl, String ownPackage = 'com.ondex.'}) {
  if (url == 'about:blank' || url.startsWith('about:srcdoc')) return PayNav.navigate;
  if (isReturnUrl(url, returnUrl)) return PayNav.finish;
  final u = Uri.tryParse(url);
  if (u == null || u.scheme.isEmpty) return PayNav.block;
  final scheme = u.scheme.toLowerCase();
  switch (scheme) {
    case 'http':
      return PayNav.block;
    case 'https':
      return isPaymentHost(u.host) ? PayNav.navigate : PayNav.external;
  }
  if (_ownSchemes.contains(scheme) || _deadSchemes.contains(scheme)) return PayNav.block;
  if (scheme == 'intent') {
    final lower = url.toLowerCase();
    if (lower.contains('package=$ownPackage') ||
        _ownSchemes.any((s) => lower.contains('scheme=$s;'))) {
      return PayNav.block;
    }
  }
  return PayNav.launchApp;
}

import '../api.dart';
import 'cart_store.dart';

/// Serverdan kelgan yakuniy hisob.
///
/// ┌─ NEGA ALOHIDA MODUL ──────────────────────────────────────────────┐
/// Bu so'rov uchta ekranda kerak: menyu (suzuvchi tugmadagi summa),
/// savat va rasmiylashtirish. Uchalasida ham alohida yozilgan edi va
/// nusxalar bir-biridan uzoqlashib ketgandi:
///   * savat "total > 0" tekshiruvini QILMAS edi, qolgan ikkitasi
///     qilardi — ya'ni noto'g'ri sozlangan aksiya savatda "0 so'm"
///     bo'lib ko'rinishi mumkin edi;
///   * xato matni ikki xil yozilgandi;
///   * javob maydonlari uch joyda qo'lda o'qilardi.
///
/// Endi so'rov ham, javobni o'qish ham, eskirgan javobdan himoya ham
/// SHU YERDA. Pul mantig'i bitta joyda bo'lishi shart.
/// └───────────────────────────────────────────────────────────────────┘
class Quote {
  const Quote({
    required this.totalTiyin,
    this.subtotalTiyin,
    this.discountTiyin,
    this.promotionName,
    this.promotionDiscountTiyin = 0,
    this.lineTotals = const {},
  });

  /// Yakuniy summa. `null` — server ishonchli raqam bermadi.
  ///
  /// 0 yoki manfiy qiymat ATAYLAB `null` ga aylantiriladi: noto'g'ri
  /// sozlangan aksiya "bepul buyurtma" ko'rinishini bermasligi kerak.
  final int? totalTiyin;

  final int? subtotalTiyin;
  final int? discountTiyin;
  final String? promotionName;

  /// Chegirmaning AYNAN aksiya bergan qismi — qolgani mahsulotlarning
  /// o'z chegirma narxlari. Hisob qatori nomi shunga qarab tanlanadi.
  final int promotionDiscountTiyin;

  /// `product_id` -> chegirmadan keyingi qator summasi (tiyin).
  final Map<String, int> lineTotals;

  bool get isUsable => totalTiyin != null && totalTiyin! > 0;
}

/// Savat holatidan server hisobini so'raydi.
///
/// ┌─ ESKIRGAN JAVOBDAN HIMOYA ────────────────────────────────────────┐
/// Mijoz "+" tugmasini tez bossa, so'rovlar javoblari TARTIBSIZ
/// kelishi mumkin va eskisi yangisini bosib ketardi — ekranda noto'g'ri
/// summa qolardi.
///
/// Shuning uchun chaqiruvchi `seq` hisoblagichini oshirib yuboradi va
/// javob kelganda o'zining `seq` i hali ham eng oxirgisimi — shuni
/// tekshiradi. Tekshiruv [QuoteFetcher] ichida avtomatik.
/// └───────────────────────────────────────────────────────────────────┘
class QuoteFetcher {
  int _seq = 0;

  /// So'rov yuboradi. Javob kelguncha yangi so'rov boshlangan bo'lsa —
  /// `null` qaytadi va chaqiruvchi ekranga TEGMASLIGI kerak.
  ///
  /// [restaurantId] savatdagi restorandan farq qilsa (masalan mijoz
  /// boshqa restoran menyusini ochib turgan bo'lsa) so'rov umuman
  /// yuborilmaydi.
  Future<Quote?> fetch(String restaurantId) async {
    final cart = CartStore.instance;
    if (cart.isEmpty || cart.restaurantId != restaurantId) {
      return const Quote(totalTiyin: null);
    }

    final seq = ++_seq;
    final res = await api.quote(restaurantId, itemsPayload());
    if (seq != _seq) return null; // eskirgan javob
    return parseQuote(res);
  }

  /// So'rov boshlanganini belgilaydi — javobni kutmasdan.
  /// Chaqiruvchi so'rovni o'zi yuboradigan holatlarda kerak.
  int nextSeq() => ++_seq;

  bool isCurrent(int seq) => seq == _seq;
}

/// Savatni server kutadigan ko'rinishga o'tkazadi.
///
/// Uchta ekranda qo'lda yozilgan edi.
List<Map<String, dynamic>> itemsPayload() => [
      for (final e in CartStore.instance.items.entries)
        {'product_id': e.key, 'qty': e.value}
    ];

/// Server javobini o'qiydi.
///
/// Maydon nomlari SHU YERDA, bitta joyda. Ilgari ular uchta ekranda
/// takrorlanardi va backend maydon qo'shsa hammasini qidirish kerak
/// bo'lardi.
Quote parseQuote(Map<String, dynamic> res) {
  final total = (res['total_tiyin'] as num?)?.toInt();
  return Quote(
    totalTiyin: (total != null && total > 0) ? total : null,
    subtotalTiyin: (res['subtotal_tiyin'] as num?)?.toInt(),
    discountTiyin: (res['discount_tiyin'] as num?)?.toInt(),
    promotionName: res['promotion_name'] as String?,
    promotionDiscountTiyin:
        (res['promotion_discount_tiyin'] as num?)?.toInt() ?? 0,
    lineTotals: quoteLineTotals(res),
  );
}

/// Serverning QATOR bo'yicha hisobi: `product_id` -> chegirmadan
/// keyingi qator summasi (tiyin).
///
/// ┌─ NEGA SERVERDAN ──────────────────────────────────────────────────┐
/// Server butun savatga aksiyani o'zi taqsimlaydi. Klient har taomga
/// alohida "eng yaxshi chegirma" hisoblaganda qatorlar yig'indisi
/// pastdagi JAMI dan farq qilardi (19 000 va 24 000). Qator narxi ham,
/// jami ham BITTA manbadan kelishi shart.
/// └───────────────────────────────────────────────────────────────────┘
Map<String, int> quoteLineTotals(Map<String, dynamic> quote) {
  final lines = quote['lines'];
  if (lines is! List) return const {};
  final out = <String, int>{};
  for (final l in lines) {
    if (l is! Map) continue;
    final id = l['product_id'] as String?;
    final total = (l['total_tiyin'] as num?)?.toInt();
    if (id == null || id.isEmpty || total == null) continue;
    out[id] = total;
  }
  return out;
}

/// Savatning CHEGIRMASIZ summasi (menyu narxlari bo'yicha).
///
/// Aksiyaning "minimal buyurtma summasi" shartini tekshirish uchun
/// kerak. Savat va menyu ekranlarida bir xil sikl ikki marta yozilgan
/// edi.
int rawSubtotal(List<Map<String, dynamic>> menu, {String? restaurantId}) {
  final cart = CartStore.instance;
  if (restaurantId != null && cart.restaurantId != restaurantId) return 0;
  final byId = productsById(menu);
  var total = 0;
  for (final e in cart.items.entries) {
    final price = (byId[e.key]?['price_tiyin'] as num?)?.toInt() ?? 0;
    total += price * e.value;
  }
  return total;
}

/// Menyuni `id -> mahsulot` xaritasiga aylantiradi.
Map<String, Map<String, dynamic>> productsById(
        List<Map<String, dynamic>> menu) =>
    {for (final p in menu) (p['id'] as String? ?? ''): p};

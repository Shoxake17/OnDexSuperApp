import 'dart:async';

import 'package:flutter/material.dart';

import '../api.dart';
import '../data/agent_driver.dart';
import '../data/ai_live.dart';
import '../data/ai_status.dart';
import '../data/ai_tools.dart';
import '../widgets/page_sheet.dart';
import '../widgets/sheet_page.dart';
import 'address_screen.dart';
import 'catalog_screen.dart' show kBrand;

/// Shaddiy Ai Agent — matn va OVOZ bilan.
///
/// ┌─ KO'RINISH: SHADDIY FRONTENDI BILAN BIR XIL ───────────────────────┐
/// Bu ekran `ShaddiySmart/Shaddiy_App/src/pages/AgentPage/` ning
/// Flutter'dagi aynan nusxasi: o'sha ranglar, o'sha o'lchamlar, o'sha
/// bo'sh holat, o'sha yozuv indikatori va o'sha ovozli rejim.
///
/// Foydalanuvchi uchun bu BITTA yordamchi — u OnDex ilovasida ham,
/// Shaddiy ilovasida ham bir xil ko'rinishi kerak. Ranglar shu sabab
/// OnDex'ning to'q sariq brendidan EMAS, Shaddiy'ning `theme.css`
/// dagi yorug' mavzusidan olingan (`_sh*` doimiylari).
///
/// BIRDAN-BIR ATAYLAB QILINGAN FARQ — buyurtma taklifi kartasi. U
/// Shaddiy'da umuman yo'q (u yerda uy jihozlari boshqariladi) va
/// undagi "Tasdiqlash" tugmasi OnDex'ning TO'Q SARIQ rangida qoldi:
/// PUL sarflaydigan yagona tugma suhbat bezagiga qo'shilib
/// ketmasligi kerak.
/// └────────────────────────────────────────────────────────────────────┘
///
/// ┌─ IKKI REJIM, IKKI XIL OVOZ SIYOSATI ───────────────────────────────┐
/// MATNLI CHAT — audio UMUMAN yo'q. Savol yoziladi, javob o'qiladi.
///
/// OVOZLI REJIM — mikrofon oqimi OnDex serveriga, u yerdan Gemini
/// Live'ga ketadi va javob modelning O'Z ovozi bilan qaytadi
/// (`lib/data/ai_live.dart`).
///
/// ┌─ NEGA QURILMADAGI TANISH/SINTEZ TASHLANDI ─────────────────────┐
/// Ilgari bu ekran `speech_to_text` + `flutter_tts` ishlatardi va
/// "audio telefondan chiqmaydi" degan va'da bor edi. Ular o'zbek
/// tilini QO'LLAMAYDI — telefonda (Galaxy S23+, Android 16)
/// o'lchandi:
///
///   IntentParsingUtil: Using Locale.getDefault() for recognition: ru-RU
///   GoogleTTSServiceImpl: Synthesis request for locale rus-RUS
///
/// ya'ni o'zbekcha gap RUS modeli bilan tanilardi va javob o'zbek
/// matnini RUS ovozi bilan o'qirdi. Qurilmada o'zbekcha nutq paketi
/// ham, o'zbekcha ovoz ham yo'q — sozlama bilan hal bo'lmaydi.
/// └────────────────────────────────────────────────────────────────┘
///
/// Shu sabab maxfiylik va'dasi OVOZLI REJIM uchun ataylab bekor
/// qilindi (2026-08-31) va foydalanuvchidan ALOHIDA rozilik
/// so'raladi. Kalit ilovada emas, serverda: ilova Gemini'ga umuman
/// bormaydi.
/// └────────────────────────────────────────────────────────────────────┘
///
/// ┌─ YORDAMCHI PUL SARFLAY OLMAYDI ────────────────────────────────────┐
/// Server hech qanday buyurtma yaratmaydi — u faqat narxlangan
/// TAKLIF qaytaradi. Buyurtma shu ekrandagi tugma bosilganda,
/// odatdagi `POST /orders` bilan beriladi. Ya'ni model aldangan
/// taqdirda ham eng yomon natija — noto'g'ri savat taklifi.
/// └────────────────────────────────────────────────────────────────────┘
class AssistantScreen extends StatefulWidget {
  const AssistantScreen({super.key});

  /// Yordamchini ochadi. HAR DOIM ochadi.
  ///
  /// ┌─ KIRISHDA TO'SIQ YO'Q ────────────────────────────────────────────┐
  /// Ilgari bu yerda `GET /ai/status` javobi tekshirilardi va yordamchi
  /// ishlamayotgan bo'lsa ekran UMUMAN ochilmasdi — o'rniga "Shaddiy
  /// hozircha ishlamayapti" dialogi chiqardi (masalan Shaddiy serveri
  /// yotganda `530`). Bu ikki sababdan yomon edi:
  ///
  ///   * foydalanuvchi ekranni umuman ko'rmasdi va nima yo'qolganini
  ///     tushunmasdi — faqat texnik status kodini o'qirdi;
  ///   * status bir lahzalik: so'rov paytida server yotgan bo'lib,
  ///     yozish paytida ko'tarilgan bo'lishi mumkin. Bir lahzalik
  ///     javob butun ekranni yopib qo'yishi mutanosib emas.
  ///
  /// Endi ekran ochiladi, foydalanuvchi yozadi — nosozlik bo'lsa
  /// suhbatning O'ZIDA aniq xato qatori chiqadi (`_assistantError`).
  /// Ya'ni xato yuborishga URINGANDA ko'rinadi, kirishda emas.
  /// └──────────────────────────────────────────────────────────────────┘
  static Future<void> open(BuildContext context) async {
    // Holat FONDA so'raladi va kutilmaydi: u faqat ovoz tugmasini
    // chizish uchun kerak. Ekran esa javobga bog'liq emas — ekran
    // `AiStatus` ni tinglaydi va javob kelganda o'zi yangilanadi.
    final status = AiStatus.instance;
    if (!status.isOn) unawaited(status.refresh());
    await Navigator.of(context).push(sheetRoute(const AssistantScreen()));
  }

  @override
  State<AssistantScreen> createState() => _AssistantScreenState();
}

// ── Shaddiy mavzusi (`Shaddiy_App/src/styles/theme.css`, yorug' rejim) ──
//
// Qiymatlar QO'LDA ko'chirilgan, chunki ikki loyiha bir-biridan
// mustaqil. O'sha faylda rang o'zgarsa bu yer ham yangilanishi kerak.
const _shBg = Color(0xFFF3F4F6); // --bg
const _shSurface = Color(0xFFFFFFFF); // --surface
const _shBorder2 = Color(0xFFE5E7EB); // --border-2
const _shText = Color(0xFF111827); // --text
const _shTextSub = Color(0xFF9CA3AF); // --text-sub
const _shTextMuted = Color(0xFF6B7280); // --text-muted
const _shBrand = Color(0xFF5B5BF6); // --brand
const _shAccent = Color(0xFF818CF8); // .agent__title-accent
const _shRingEnd = Color(0xFF9B8AFB); // avatar halqasining oxiri

/// Avatar kadrlari. `pubspec.yaml` dagi `assets/shaddiy/` ga qarang.
const _faceIdle = 'assets/shaddiy/face_idle.jpg';
const _faceLarge = 'assets/shaddiy/face_large.jpg';
const _faceSmall = 'assets/shaddiy/face_small.jpg';
const _faceClosed = 'assets/shaddiy/face_closed.jpg';

class _ChatLine {
  final String role; // 'user' | 'assistant'
  final String text;
  final Map<String, dynamic>? proposal;
  final bool error;
  _ChatLine(this.role, this.text, {this.proposal, this.error = false});
}

class _AssistantScreenState extends State<AssistantScreen> {
  final _input = TextEditingController();
  final _scroll = ScrollController();
  final _lines = <_ChatLine>[];
  bool _busy = false;

  // ── Ovoz ──
  //
  // ┌─ QURILMADAGI STT/TTS OLIB TASHLANDI ─────────────────────────────┐
  // Ilgari bu yerda `speech_to_text` + `flutter_tts` turardi. Ular
  // o'zbek tilini QO'LLAMAYDI — telefonda (Galaxy S23+, Android 16)
  // o'lchandi:
  //
  //   IntentParsingUtil: Using Locale.getDefault() for recognition: ru-RU
  //   GoogleTTSServiceImpl: Synthesis request for locale rus-RUS
  //
  // ya'ni o'zbekcha gap RUS modeli bilan tanilardi va javob o'zbek
  // matnini RUS ovozi bilan o'qirdi. Bu sozlama bilan tuzatiladigan
  // narsa emas: qurilmada o'zbekcha nutq paketi ham, o'zbekcha ovoz
  // ham yo'q.
  //
  // Endi ovoz butunlay serverdan: `AiLive` -> `/ai/live` -> Gemini
  // Live. Tabiiy o'zbekcha ovoz, sintez emas.
  // └──────────────────────────────────────────────────────────────────┘
  late final AiLive _live = AiLive(
    onProposal: _onLiveProposal,
    onText: (t) => _appendLive('assistant', t),
    onUserText: (t) => _appendLive('user', t),
    onTurnEnd: _endLiveTurn,
    onToolError: _onLiveToolError,
    onConfirmOrder: _onLiveConfirmOrder,
  );

  /// Og'zaki tasdiq — buyurtmani naqd to'lov bilan beramiz.
  ///
  /// Bajarib bo'lmasa SABAB aytiladi: model "tasdiqlayapman" degan,
  /// ilova esa hech narsa qilmagan bo'lsa — bu aldash bo'lardi.
  Future<void> _onLiveConfirmOrder() async {
    final ok = await AgentDriver.instance.confirmCashOrder();
    if (ok || !mounted) return;
    setState(() {
      _lines.add(_ChatLine(
        'assistant',
        'Buyurtmani tasdiqlab bo\'lmadi — rasmiylashtirish ekrani ochiq '
            'va summa hisoblangan bo\'lishi kerak. Ekrandan o\'zingiz '
            'tasdiqlang.',
        error: true,
      ));
    });
    _scrollToEnd();
  }

  /// Amal bajarilmadi — suhbatda XATO qatori.
  ///
  /// Yozuv ovozdan MUSTAQIL: model xatoni aytmasa ham (yoki
  /// "qildim" desa ham) foydalanuvchi haqiqiy holatni ko'radi.
  void _onLiveToolError(String message) {
    if (!mounted) return;
    setState(() {
      _liveLine.clear(); // xato yangi qatordan, javob ustiga yozilmasin
      _lines.add(_ChatLine('assistant', message, error: true));
    });
    _scrollToEnd();
  }

  /// Ovozli suhbatda hozir to'ldirilayotgan qatorlar indeksi.
  ///
  /// Transkripsiya BO'LAKLAB keladi ("Avigo", " restoranida", ...).
  /// Har bo'lakni alohida xabar qilib qo'shsak, suhbat bir so'zli
  /// pufakchalarga bo'linib ketardi.
  final Map<String, int> _liveLine = {};

  /// Ovoz qatlamining oxirgi nosozligi — ekranda KO'RSATILADI.
  ///
  /// Ilgari bunday maydon yo'q edi va xato `onError: (_) {}` ichida
  /// yo'qolardi: foydalanuvchi mikrofonni bosar, hech narsa
  /// bo'lmasdi va sababini bilishning imkoni yo'q edi.
  String? _voiceError;

  /// To'liq ekranli ovozli rejim (Shaddiy Live).
  bool _voiceMode = false;

  /// Saqlangan yetkazib berish manzili.
  ///
  /// ┌─ NEGA BU EKRANDA KERAK ────────────────────────────────────┐
  /// "Tasdiqlash" buyurtmani DARHOL beradi va manzilni server
  /// `/me/address` dan oladi — ya'ni foydalanuvchi tugmani bosganda
  /// taom QAYERGA ketishini ko'rmasdan tasdiqlardi. Savat orqali
  /// borilganda checkout ekrani buni ko'rsatadi, bu yerda esa
  /// ko'rsatmaydigan hech kim yo'q edi.
  ///
  /// Manzil umuman tanlanmagan bo'lsa server 400 qaytaradi; endi bu
  /// holat tugma bosilishidan OLDIN ko'rinadi va tugmaning o'zi
  /// "Manzil tanlash" ga aylanadi.
  /// └────────────────────────────────────────────────────────────┘
  Map<String, dynamic>? _address;

  @override
  void initState() {
    super.initState();
    // Ovoz qatlami ENDI ekran ochilishida ishga tushmaydi: Gemini Live
    // seansi tarmoq va pul talab qiladi, shuning uchun u FAQAT
    // foydalanuvchi ovozli rejimni ochganda boshlanadi
    // (`_openVoiceMode`).
    _live.addListener(_onLiveChanged);
    // Boshqaruv rasmiylashtirishga yetgach Shaddiy to'lovni so'raydi.
    //
    // Ovoz o'chiq bo'lsa (matnli rejim) hech narsa yuborilmaydi —
    // `checkoutReady` seans faol bo'lmasa jimgina qaytadi. Bunda
    // foydalanuvchi ekrandan o'zi tasdiqlaydi.
    AgentDriver.instance.onCheckoutReady = (restaurant, total) {
      _live.checkoutReady(restaurant: restaurant, totalTiyin: total);
    };
    _loadAddress();
    // Ruxsatlar OLDINDAN o'qiladi: taklif kelganda boshqaruvchi
    // "ruxsat noma'lum" sababli to'xtab qolmasin.
    if (!AiTools.instance.loaded) AiTools.instance.load();
    // Holat ekran ochilgandan KEYIN ham kelishi mumkin (kirishda uni
    // kutmaymiz). Kelganda ovoz tugmasi o'zi paydo bo'lsin.
    AiStatus.instance.addListener(_onAiStatusChanged);
    if (!AiStatus.instance.isOn) AiStatus.instance.refresh();
  }

  /// Yordamchi holati yangilandi — ovoz tugmasi shunga qarab chiziladi.
  void _onAiStatusChanged() {
    if (mounted) setState(() {});
  }

  /// `AiLive` holati o'zgardi — ovozli ekran shunga qarab qayta
  /// chiziladi (yozuv, yuz animatsiyasi, xato lentasi).
  void _onLiveChanged() {
    if (!mounted) return;
    setState(() {
      final err = _live.error;
      if (err != null) _voiceError = err;
    });
  }

  @override
  void dispose() {
    _input.dispose();
    _scroll.dispose();
    _live.removeListener(_onLiveChanged);
    AiStatus.instance.removeListener(_onAiStatusChanged);
    // Boshqaruvchi bu ekrandan uzoq yashaydi — o'lgan ekranga
    // ishora qoldirilmasin.
    AgentDriver.instance.onCheckoutReady = null;
    _live.dispose();
    super.dispose();
  }

  // ── Ovozli rejim (Gemini Live) ──
  //
  // ┌─ SALOM ENDI AYTILMAYDI ──────────────────────────────────────────┐
  // Ilgari ekran ochilishi bilan `flutter_tts` salomni OVOZ CHIQARIB
  // o'qirdi. Qurilmada o'zbekcha ovoz yo'q, shuning uchun uni rus
  // ovozi o'qirdi — o'zbek matnini rus fonetikasi bilan, ya'ni
  // deyarli tushunarsiz. Endi salom faqat bo'sh holat sarlavhasida
  // (`_empty`) va faqat MATN sifatida turadi; ovoz esa ovozli
  // rejimda, Gemini'ning o'z ovozi bilan.
  // └──────────────────────────────────────────────────────────────────┘

  /// Transkripsiya bo'lagini suhbatga qo'shadi.
  ///
  /// Ovoz o'tib ketadi, matn esa qoladi: foydalanuvchi ovozli rejimni
  /// yopgach nima gaplashilganini ko'radi va aytilgan SUMMANI qayta
  /// o'qiy oladi.
  void _appendLive(String role, String chunk) {
    if (!mounted) return;
    setState(() {
      final idx = _liveLine[role];
      if (idx != null && idx < _lines.length && _lines[idx].role == role) {
        _lines[idx] = _ChatLine(role, _lines[idx].text + chunk);
      } else {
        _liveLine[role] = _lines.length;
        _lines.add(_ChatLine(role, chunk));
      }
    });
    _scrollToEnd();
  }

  /// Navbat tugadi — keyingi gap YANGI qatordan boshlanadi.
  void _endLiveTurn() => _liveLine.clear();

  /// Model savat taklif qildi.
  ///
  /// ┌─ BUYURTMA BU YERDA BERILMAYDI ────────────────────────────────┐
  /// Taklif AYNAN matnli chatdagi kabi karta bo'lib chiziladi va
  /// tugmani ODAM bosadi. Ovoz bilan "ha" deyish YETARLI EMAS:
  /// noto'g'ri tanilgan gap odamning pulini sarflamasligi kerak.
  /// Shu sabab taklif kelishi bilan ovozli rejim YOPILADI — karta
  /// to'liq ekranli yuz ortida ko'rinmay qolardi.
  /// └───────────────────────────────────────────────────────────────┘
  void _onLiveProposal(Map<String, dynamic> proposal) {
    if (!mounted) return;
    setState(() {
      _lines.add(_ChatLine('assistant', '', proposal: proposal));
      // To'liq ekranli ovozli ko'rinishdan chiqamiz — endi odatdagi
      // ekranlar ko'rinishi kerak.
      _voiceMode = false;
    });
    // ┌─ SEANS ATAYLAB YOPILMAYDI ────────────────────────────────────┐
    // Ilgari bu yerda `_live.stop()` turardi. Natijada Shaddiy
    // rasmiylashtirish ekraniga yetib borar-u, o'sha zahoti JIM
    // bo'lardi: to'lovni so'raydigan seans qolmasdi.
    //
    // Endi ovoz ochiq qoladi — foydalanuvchi ekranda buyurtma
    // yig'ilishini ko'radi va Shaddiy oxirida to'lovni so'raydi.
    // Seans `_closeVoiceMode`, ekran yopilishi yoki 10 daqiqalik
    // server chegarasi bilan tugaydi.
    // └───────────────────────────────────────────────────────────────┘
    _scrollToEnd();
    // Ovozli rejimda tugma bosadigan qo'l yo'q — Shaddiy buyurtmani
    // O'ZI ko'rsatib beradi va to'lov ekranida to'xtaydi.
    _startAgentRun(proposal);
  }

  /// Shaddiy buyurtmani ilovaning O'ZIDA, ko'rinadigan tarzda beradi.
  ///
  /// ┌─ NEGA BU TAKLIF KARTASINI ALMASHTIRMAYDI ──────────────────────┐
  /// Karta suhbatda QOLADI. Boshqaruv to'xtatilsa yoki nimadir
  /// yiqilsa, foydalanuvchi o'sha kartadan odatdagi yo'l bilan davom
  /// eta oladi. Yangi yo'l eskisini o'chirmasligi kerak.
  /// └────────────────────────────────────────────────────────────────┘
  void _startAgentRun(Map<String, dynamic> proposal) {
    // Ekran yopilishi bilan boshlansin: `Navigator.popUntil` shu
    // kadrda ishlasa, hozirgi ekran ostidan yo'l tortib olingan
    // bo'lardi.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      AgentDriver.instance.run(proposal);
    });
  }

  /// Ovozli rejimni ochadi: rozilik → ruxsat → ulanish.
  Future<void> _openVoiceMode() async {
    if (!AiStatus.instance.voiceOn) {
      setState(() => _voiceError =
          'Ovozli rejim serverda yoqilmagan. Savolni matn bilan yozing.');
      return;
    }
    // ┌─ ROZILIK OYNASI OLIB TASHLANDI ───────────────────────────────┐
    // Ilgari bu yerda "gapirganingiz ... yuboriladi" degan oyna
    // chiqardi. Foydalanuvchi uchun ilovada FAQAT ikki brend
    // ko'rinishi kerak — Shaddiy va OnDex; xizmat ta'minotchilari
    // ilovaning ichki masalasi.
    //
    // Mikrofon ruxsatini baribir Android'ning O'ZI so'raydi (tizim
    // oynasi) — ya'ni foydalanuvchi mikrofon yoqilishini har holda
    // ko'radi va rad eta oladi.
    // └───────────────────────────────────────────────────────────────┘
    setState(() {
      _voiceMode = true;
      _voiceError = null;
    });
    await _live.start();
  }

  Future<void> _closeVoiceMode() async {
    await _live.stop();
    if (mounted) setState(() => _voiceMode = false);
  }

  /// Ovozli rejimdagi yozuv — `AiLive` holatidan.
  String get _liveLabel {
    if (_live.speaking) return 'Shaddiy gapiryapti...';
    switch (_live.state) {
      case AiLiveState.connecting:
        return 'Ulanmoqda...';
      case AiLiveState.active:
        return 'Tinglayapman...';
      case AiLiveState.error:
        return 'Xatolik';
      case AiLiveState.idle:
      case AiLiveState.closed:
        return 'Ovozli suhbat tugadi';
    }
  }


  /// Manzilni o'qish — xatosi JIM yutiladi.
  ///
  /// Bu ma'lumot faqat tasdiqlash kartasini to'ldirish uchun. Tarmoq
  /// uzilgani sabab suhbatni umuman boshlab bo'lmasligi noto'g'ri
  /// bo'lardi; manzil noma'lum bo'lsa karta odatdagi "Tasdiqlash"
  /// tugmasini ko'rsatadi va tekshiruv baribir serverda bo'ladi.
  Future<void> _loadAddress() async {
    try {
      final a = await api.getMyAddress();
      if (mounted) setState(() => _address = a);
    } catch (_) {
      // e'tiborsiz — yuqoridagi izohga qarang.
    }
  }

  /// Manzil haqiqatan tanlanganmi (server ham AYNAN shu shartni
  /// tekshiradi: `lat == 0 && lng == 0` bo'lsa buyurtma rad etiladi).
  bool get _hasAddress {
    final a = _address;
    if (a == null) return true; // noma'lum — to'sib qo'ymaymiz
    final lat = (a['lat'] as num?)?.toDouble() ?? 0;
    final lng = (a['lng'] as num?)?.toDouble() ?? 0;
    return lat != 0 || lng != 0;
  }

  String get _addressText {
    final t = (_address?['text'] ?? '').toString().trim();
    return t.isEmpty ? 'Saqlangan manzil' : t;
  }

  /// Manzil ekranini ochadi va qaytgach yangilaydi.
  Future<void> _pickAddress() async {
    await Navigator.of(context)
        .push(MaterialPageRoute(builder: (_) => const AddressScreen()));
    await _loadAddress();
  }

  // ── Suhbat ──

  /// Serverga yuboriladigan tarix.
  ///
  /// Faqat matn: tool mexanikasi serverda qoladi va u yerda baribir
  /// tozalanadi (`trimHistory`). Oxirgi 12 tasi — server chegarasi
  /// 16, shundan pastda turamiz.
  List<Map<String, String>> _history() {
    final all = _lines
        .where((l) => !l.error)
        .map((l) => {'role': l.role, 'content': l.text})
        .toList(growable: false);
    return all.length <= 12 ? all : all.sublist(all.length - 12);
  }

  Future<void> _send(String text, {bool spoken = false}) async {
    text = text.trim();
    if (text.isEmpty || _busy) return;

    final history = _history();
    setState(() {
      _lines.add(_ChatLine('user', text));
      _busy = true;
      _input.clear();
    });
    _scrollToEnd();

    try {
      final res = await api.aiChat(text, history);
      final reply = (res['reply'] ?? '').toString();
      final proposal = res['proposal'] as Map<String, dynamic>?;
      if (!mounted) return;
      setState(() {
        _lines.add(_ChatLine('assistant', reply, proposal: proposal));
        _busy = false;
      });
      _scrollToEnd();
      // Ovozli javob bu yerda YO'Q: matnli chat ataylab jim. Ovoz
      // faqat ovozli rejimda va Gemini Live orqali beriladi
      // (`AiLive`) — qurilmadagi sintez o'zbekchani rus ovozi bilan
      // o'qirdi.
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _lines.add(_ChatLine('assistant', _assistantError(e), error: true));
        _busy = false;
      });
      _scrollToEnd();
    }
  }

  /// Yordamchi javob bera olmadi — suhbatda ko'rsatiladigan matn.
  ///
  /// ┌─ XOM SERVER XATOSI KO'RSATILMAYDI ────────────────────────────────┐
  /// Shaddiy serveri yotganda API `530` (yoki `502`/`504`) qaytaradi va
  /// xabar matni "Server javob bermadi (530)" bo'lardi. Bu foydalanuvchi
  /// uchun ma'nosiz: u na sababni tushunadi, na nima qilishni biladi.
  ///
  /// Shuning uchun server tomonidagi barcha nosozliklar BITTA aniq
  /// iboraga yig'iladi. Foydalanuvchining O'ZI hal qila oladigan ikki
  /// holat esa alohida qoladi — sessiya tugashi va tezlik chegarasi:
  /// ularni umumiy matnga qo'shish odamni boshi berk ko'chaga
  /// olib borardi.
  /// └───────────────────────────────────────────────────────────────────┘
  String _assistantError(Object e) {
    const technical = 'Texnik nosozlik — Shaddiy hozir javob bera olmayapti. '
        'Birozdan keyin qayta urinib ko\'ring.';
    if (e is ApiException) {
      if (e.statusCode == 401) {
        return 'Hisobingizga qayta kiring — sessiya muddati tugagan.';
      }
      if (e.statusCode == 429) {
        return 'Juda ko\'p so\'rov yuborildi. Biroz kutib qayta urining.';
      }
      // 5xx va 0 (javob umuman kelmadi) — server tomoni.
      if (e.statusCode >= 500 || e.statusCode == 0) return technical;
      // 4xx — serverning O'ZI yozgan aniq sabab (masalan taom
      // topilmadi). Uni yashirish foydalanuvchiga zarar qilardi.
      return e.message;
    }
    // Tarmoq uzilishi, timeout, JSON buzilishi.
    return technical;
  }

  void _scrollToEnd() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!_scroll.hasClients) return;
      _scroll.animateTo(_scroll.position.maxScrollExtent,
          duration: const Duration(milliseconds: 250), curve: Curves.easeOut);
    });
  }

  // ── Taklif bilan ishlash ──

  /// "Shaddiy o'zi qilsin" — buyurtmani KO'RSATIB beradi.
  ///
  /// ┌─ NEGA JIM QO'SHISH O'RNIGA ────────────────────────────────────┐
  /// Ilgari bu tugma taomlarni savatga JIMGINA qo'shib, savat
  /// ekranini ochardi. Natijada foydalanuvchi savatda taomlarni
  /// ko'rar, lekin ular QAYERDAN va QANDAY paydo bo'lganini
  /// ko'rmasdi — ishonch talab qiladigan holat.
  ///
  /// Endi Shaddiy odam qanday qilsa, xuddi shunday qiladi: menyuni
  /// ochadi, taomni topib skroll qiladi, "+" ni bosadi, savatga
  /// o'tadi va rasmiylashtirishni bosadi. Har qadam ekranda ko'rinadi
  /// va istalgan payt to'xtatiladi.
  ///
  /// To'lov ekranida TO'XTAYDI — pul sarflaydigan tugmani odam
  /// bosadi (`lib/data/agent_driver.dart`).
  /// └────────────────────────────────────────────────────────────────┘
  Future<void> _toCart(Map<String, dynamic> p) async {
    Navigator.of(context).maybePop();
    _startAgentRun(p);
  }

  /// "Tasdiqlash" — buyurtmani darhol beradi.
  ///
  /// Chaqiruv checkout ekranidagi bilan AYNAN bir xil
  /// (`api.createOrder`): yangi pul yo'li yozilmagan. Manzil va
  /// xizmat hududi serverda tekshiriladi.
  ///
  /// ┌─ KO'RSATILGAN SUMMA QO'RIQLANADI ──────────────────────────┐
  /// `expectedTotalTiyin` — kartada ko'rsatilgan AYNAN o'sha raqam.
  /// Taklif ko'rsatilishi bilan tugma bosilishi orasida aksiya
  /// tugashi yoki restoran narxni tahrirlashi mumkin; server
  /// bunday holatda buyurtma yaratmaydi va 409 qaytaradi.
  ///
  /// Bu ayniqsa shu ekranda muhim: savatni odam emas, til modeli
  /// tuzgan va foydalanuvchi faqat yakuniy raqamga qarab
  /// tasdiqlaydi.
  /// └────────────────────────────────────────────────────────────┘
  Future<void> _confirm(Map<String, dynamic> p) async {
    final items = (p['items'] as List? ?? const [])
        .cast<Map<String, dynamic>>()
        .map((it) => {
              'product_id': (it['product_id'] ?? '').toString(),
              'qty': (it['qty'] as num?)?.toInt() ?? 1,
            })
        .toList();
    if (items.isEmpty) return;

    // Manzil tanlanmagan bo'lsa buyurtma baribir rad etilardi —
    // foydalanuvchini kutdirib xato ko'rsatgandan ko'ra darhol
    // manzil ekraniga olib boramiz.
    if (!_hasAddress) {
      await _pickAddress();
      if (!_hasAddress) return;
    }

    final expected = (p['total_tiyin'] as num?)?.toInt() ?? 0;

    setState(() => _busy = true);
    try {
      // Idempotentlik kaliti — tarmoq uzilib qayta bosilsa ikkinchi
      // buyurtma yaratilmaydi (checkout bilan bir xil naqsh).
      await api.createOrder(
        items: items,
        idempotencyKey: newIdempotencyKey(),
        expectedTotalTiyin: expected,
      );
      if (!mounted) return;
      setState(() {
        _busy = false;
        _lines.add(_ChatLine('assistant',
            'Buyurtmangiz qabul qilindi. "Buyurtmalar" bo\'limidan kuzatishingiz mumkin.'));
      });
      _scrollToEnd();
    } catch (e) {
      if (!mounted) return;
      // 409 — narx o'zgargan. Buyurtma YARATILMAGAN. Yangi summani
      // ko'rsatib qayta tasdiqlashni so'raymiz: eskisini jimgina
      // o'tkazib yuborish ham, shunchaki "xato" deyish ham
      // noto'g'ri bo'lardi.
      if (e is ApiException && e.statusCode == 409) {
        final fresh = (e.data?['total_tiyin'] as num?)?.toInt() ?? 0;
        final updated = Map<String, dynamic>.from(p);
        if (fresh > 0) updated['total_tiyin'] = fresh;
        final msg = fresh > 0
            ? 'Narx o\'zgardi. Yangi jami: ${formatSum(fresh)} so\'m. Tasdiqlaysizmi?'
            : 'Narx o\'zgardi. Yangi summani tasdiqlang.';
        setState(() {
          _busy = false;
          _lines.add(_ChatLine('assistant', msg, proposal: updated));
        });
        _scrollToEnd();
        return;
      }
      setState(() {
        _busy = false;
        _lines.add(_ChatLine('assistant', errorText(e, 'Buyurtma berilmadi.'),
            error: true));
      });
      _scrollToEnd();
    }
  }

  // ── Ko'rinish ──

  @override
  Widget build(BuildContext context) {
    // Ovozli rejim — TO'LIQ EKRAN, o'z sarlavhasisiz (Shaddiy'dagidek).
    if (_voiceMode) {
      return _VoiceMode(
        label: _liveLabel,
        speaking: _live.speaking,
        // Qizil "to'xtatish" tugmasi FAQAT seans tirik bo'lganda.
        // Gemini'ning o'z nutq aniqlagichi gap tugaganini o'zi
        // biladi, shuning uchun bu tugma "gapirib bo'ldim" emas,
        // "suhbatni tugat" degani.
        active: _live.state == AiLiveState.active ||
            _live.state == AiLiveState.connecting,
        error: _voiceError,
        onStop: _closeVoiceMode,
        onClose: _closeVoiceMode,
      );
    }

    return SheetPage(
      child: Scaffold(
        backgroundColor: _shBg,
        appBar: PageAppBar(
          centerTitle: true,
          titleWidget: const Text('Shaddiy Ai Agent',
              style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
          actions: [
            // Ovozli suhbat — alohida ekran. Ilgari bu yerda "ovozli
            // javobni yoqish/o'chirish" tugmasi turardi; endi matnli
            // chat HAR DOIM jim va ovoz alohida rejimda, shuning
            // uchun tugma ham o'shani ochadi.
            if (AiStatus.instance.voiceOn)
              IconButton(
                tooltip: 'Ovozli suhbat',
                icon: const Icon(Icons.graphic_eq, color: _shTextMuted),
                onPressed: _openVoiceMode,
              ),
          ],
        ),
        body: Column(
          children: [
            Expanded(
              child: _lines.isEmpty && !_busy ? _empty() : _messages(),
            ),
            // Ovoz nosozligi — suhbat rejimida ham KO'RINADI. Ilgari
            // u faqat logcat'da qolardi.
            if (_voiceError != null) _voiceBanner(_voiceError!),
            _composer(),
          ],
        ),
      ),
    );
  }

  /// Ovoz nosozligi haqidagi lenta — yopib qo'yish mumkin.
  Widget _voiceBanner(String message) {
    return Container(
      width: double.infinity,
      color: const Color(0xFFFEF3C7),
      padding: const EdgeInsets.fromLTRB(14, 10, 6, 10),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Icon(Icons.mic_off, size: 18, color: Color(0xFF92400E)),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              message,
              style: const TextStyle(
                  fontSize: 12.5, height: 1.35, color: Color(0xFF92400E)),
            ),
          ),
          IconButton(
            visualDensity: VisualDensity.compact,
            icon: const Icon(Icons.close, size: 16, color: Color(0xFF92400E)),
            onPressed: () => setState(() => _voiceError = null),
          ),
        ],
      ),
    );
  }

  /// Bo'sh holat — `.agent__empty`.
  Widget _empty() {
    return ListView(
      controller: _scroll,
      padding: const EdgeInsets.fromLTRB(20, 56, 20, 16),
      children: [
        Center(
          child: Container(
            padding: const EdgeInsets.all(2),
            decoration: BoxDecoration(
              shape: BoxShape.circle,
              gradient: const LinearGradient(
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
                colors: [_shBrand, _shRingEnd],
              ),
              boxShadow: [
                BoxShadow(
                  color: _shBrand.withValues(alpha: 0.35),
                  blurRadius: 32,
                  spreadRadius: 6,
                ),
              ],
            ),
            child: Container(
              padding: const EdgeInsets.all(3),
              decoration: const BoxDecoration(
                shape: BoxShape.circle,
                color: _shBg,
              ),
              child: const ClipOval(
                child: Image(
                  image: AssetImage(_faceIdle),
                  width: 150,
                  height: 150,
                  fit: BoxFit.cover,
                ),
              ),
            ),
          ),
        ),
        const SizedBox(height: 24),
        // "Assalomu alaykum! Men <accent>Shaddiy Ai Agent</accent>" —
        // accent qismi Shaddiy'dagi `.agent__title-accent` bilan bir
        // xil rangda.
        //
        // Shaddiy frontendida bu yerda "Salom!" turadi. ATAYLAB
        // farqli: ekran ochilishi bilan yordamchi shu matnni OVOZ
        // bilan aytadi (`_greeting`) va eshitilgan so'z bilan
        // ko'ringan so'z bir xil bo'lishi kerak.
        const Text.rich(
          TextSpan(
            children: [
              TextSpan(text: 'Assalomu alaykum! Men '),
              TextSpan(
                text: 'Shaddiy Ai Agent',
                style: TextStyle(color: _shAccent),
              ),
            ],
          ),
          textAlign: TextAlign.center,
          style: TextStyle(
              fontSize: 22, fontWeight: FontWeight.bold, color: _shText),
        ),
        const SizedBox(height: 6),
        const Text(
          'Men sizga taom tanlash va buyurtma berishda yordam beraman',
          textAlign: TextAlign.center,
          style: TextStyle(fontSize: 14, color: _shTextMuted),
        ),
        const SizedBox(height: 32),
        // Taklif kartalari — Shaddiy'da ular `SUGGESTIONS` massivida,
        // hozircha izohga olingan. Bu yerda ular OnDex'ga xos va
        // FAOL: yangi foydalanuvchi nima deyishni bilmasligi eng
        // ko'p uchraydigan to'siq.
        for (final s in const [
          'Menga ikkita osh top',
          'Eng arzon lag\'mon qaysi restoranda?',
          'Buyurtmam qayerda?',
        ])
          _SuggestionCard(text: s, onTap: () => _send(s)),
      ],
    );
  }

  /// Suhbat — `.agent__messages`.
  Widget _messages() {
    return ListView.builder(
      controller: _scroll,
      padding: const EdgeInsets.all(16),
      itemCount: _lines.length + (_busy ? 1 : 0),
      itemBuilder: (_, i) {
        if (i >= _lines.length) return const _TypingRow();
        final l = _lines[i];
        return _Bubble(
          line: l,
          addressText: _addressText,
          hasAddress: _hasAddress,
          // Ruxsat o'chirilgan bo'lsa tugma UMUMAN chizilmaydi:
          // bosilganda hech narsa qilmaydigan tugma chalg'itardi.
          onCart: l.proposal == null || !AiTools.instance.checkoutAllowed
              ? null
              : () => _toCart(l.proposal!),
          onConfirm:
              l.proposal == null || _busy ? null : () => _confirm(l.proposal!),
        );
      },
    );
  }

  /// Kiritish paneli — `.agent__inputbar`.
  Widget _composer() {
    final hasText = _input.text.trim().isNotEmpty;
    return SafeArea(
      top: false,
      child: Container(
        padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
        color: _shBg,
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
          decoration: BoxDecoration(
            color: _shSurface,
            borderRadius: BorderRadius.circular(16),
            border: Border.all(color: _shBorder2),
          ),
          child: Row(
            children: [
              Expanded(
                child: TextField(
                  controller: _input,
                  enabled: !_busy,
                  textInputAction: TextInputAction.send,
                  onSubmitted: _send,
                  // Tugma mikrofondan yuborishga o'zgarishi uchun —
                  // Shaddiy'da ham AYNAN shunday.
                  onChanged: (_) => setState(() {}),
                  style: const TextStyle(fontSize: 14, color: _shText),
                  decoration: const InputDecoration(
                    hintText:
                        'Shaddiy Ai Agent ga savol bering yoki buyruq yozing...',
                    hintStyle: TextStyle(fontSize: 14, color: _shTextMuted),
                    isDense: true,
                    border: InputBorder.none,
                    enabledBorder: InputBorder.none,
                    focusedBorder: InputBorder.none,
                    contentPadding: EdgeInsets.symmetric(vertical: 10),
                  ),
                ),
              ),
              const SizedBox(width: 12),
              // Matn bo'lsa — yuborish, bo'lmasa — ovozli rejim.
              // Ovoz serverda yoqilmagan bo'lsa mikrofon UMUMAN
              // ko'rsatilmaydi: ishlamaydigan tugma chalg'itadi.
              if (hasText || !AiStatus.instance.voiceOn)
                _RoundButton(
                  icon: Icons.send,
                  onTap: _busy || !hasText ? null : () => _send(_input.text),
                  busy: _busy,
                )
              else
                _RoundButton(
                  icon: Icons.mic,
                  onTap: _busy ? null : _openVoiceMode,
                ),
            ],
          ),
        ),
      ),
    );
  }
}

// ── Ko'rinish qismlari ──

/// `.agent__suggestion`
class _SuggestionCard extends StatelessWidget {
  final String text;
  final VoidCallback onTap;

  const _SuggestionCard({required this.text, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(16),
        child: Container(
          padding: const EdgeInsets.all(16),
          decoration: BoxDecoration(
            color: _shSurface,
            borderRadius: BorderRadius.circular(16),
            border: Border.all(color: _shBorder2),
          ),
          child: Row(
            children: [
              const Icon(Icons.auto_awesome, size: 18, color: _shAccent),
              const SizedBox(width: 12),
              Expanded(
                child: Text(text,
                    style: const TextStyle(
                        fontSize: 14,
                        fontWeight: FontWeight.w500,
                        color: _shText)),
              ),
              const Icon(Icons.chevron_right, size: 18, color: _shTextMuted),
            ],
          ),
        ),
      ),
    );
  }
}

/// `.agent__btn` — 36px dumaloq tugma.
class _RoundButton extends StatelessWidget {
  final IconData icon;
  final VoidCallback? onTap;
  final bool busy;

  const _RoundButton({required this.icon, this.onTap, this.busy = false});

  @override
  Widget build(BuildContext context) {
    return Opacity(
      opacity: onTap == null ? 0.4 : 1,
      child: InkResponse(
        onTap: onTap,
        radius: 22,
        child: Container(
          width: 36,
          height: 36,
          decoration: const BoxDecoration(
            shape: BoxShape.circle,
            color: _shBrand,
          ),
          child: busy
              ? const Padding(
                  padding: EdgeInsets.all(9),
                  child: CircularProgressIndicator(
                      strokeWidth: 2, color: Colors.white),
                )
              : Icon(icon, size: 17, color: Colors.white),
        ),
      ),
    );
  }
}

/// `.agent__row` + `.agent__bubble`
class _Bubble extends StatelessWidget {
  final _ChatLine line;
  final String addressText;
  final bool hasAddress;
  final VoidCallback? onCart;
  final VoidCallback? onConfirm;

  const _Bubble({
    required this.line,
    required this.addressText,
    required this.hasAddress,
    this.onCart,
    this.onConfirm,
  });

  @override
  Widget build(BuildContext context) {
    final isUser = line.role == 'user';
    return Column(
      crossAxisAlignment:
          isUser ? CrossAxisAlignment.end : CrossAxisAlignment.start,
      children: [
        if (line.text.isNotEmpty)
          Padding(
            padding: const EdgeInsets.only(bottom: 8),
            child: Row(
              mainAxisAlignment:
                  isUser ? MainAxisAlignment.end : MainAxisAlignment.start,
              crossAxisAlignment: CrossAxisAlignment.end,
              children: [
                if (!isUser) const _MsgAvatar(),
                Flexible(
                  child: Container(
                    padding: const EdgeInsets.symmetric(
                        horizontal: 16, vertical: 10),
                    constraints: BoxConstraints(
                        maxWidth: MediaQuery.of(context).size.width * 0.8),
                    decoration: BoxDecoration(
                      color: isUser
                          ? _shBrand
                          : line.error
                              ? const Color(0x26EF4444)
                              : _shSurface,
                      border: isUser
                          ? null
                          : Border.all(
                              color: line.error
                                  ? const Color(0x4DEF4444)
                                  : _shBorder2),
                      borderRadius: BorderRadius.only(
                        topLeft: const Radius.circular(16),
                        topRight: const Radius.circular(16),
                        bottomLeft: Radius.circular(isUser ? 16 : 4),
                        bottomRight: Radius.circular(isUser ? 4 : 16),
                      ),
                    ),
                    child: Text(
                      line.text,
                      style: TextStyle(
                        fontSize: 14,
                        height: 1.6,
                        color: isUser
                            ? Colors.white
                            : line.error
                                ? const Color(0xFFB91C1C)
                                : _shText,
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
        if (line.proposal != null)
          _ProposalCard(
            proposal: line.proposal!,
            addressText: addressText,
            hasAddress: hasAddress,
            onCart: onCart,
            onConfirm: onConfirm,
          ),
      ],
    );
  }
}

/// `.agent__msg-avatar` — 28px.
class _MsgAvatar extends StatelessWidget {
  const _MsgAvatar();

  @override
  Widget build(BuildContext context) => const Padding(
        padding: EdgeInsets.only(right: 8, bottom: 2),
        child: ClipOval(
          child: Image(
            image: AssetImage(_faceIdle),
            width: 28,
            height: 28,
            fit: BoxFit.cover,
          ),
        ),
      );
}

/// `.agent__typing` — uchta sakrab turgan nuqta + "O'ylanmoqda...".
class _TypingRow extends StatefulWidget {
  const _TypingRow();

  @override
  State<_TypingRow> createState() => _TypingRowState();
}

class _TypingRowState extends State<_TypingRow>
    with SingleTickerProviderStateMixin {
  late final AnimationController _c = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 1200),
  )..repeat();

  @override
  void dispose() {
    _c.dispose();
    super.dispose();
  }

  /// CSS `agent-bounce`: 0%/80%/100% — 0, 40% — -5px.
  /// Har nuqta 0.2s kechikadi.
  double _offset(double t) {
    if (t >= 0.8) return 0;
    // 0 → 0.4 ko'tariladi, 0.4 → 0.8 tushadi.
    final x = t < 0.4 ? t / 0.4 : (0.8 - t) / 0.4;
    return -5 * x;
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          const _MsgAvatar(),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
            decoration: BoxDecoration(
              color: _shSurface,
              border: Border.all(color: _shBorder2),
              borderRadius: const BorderRadius.only(
                topLeft: Radius.circular(16),
                topRight: Radius.circular(16),
                bottomLeft: Radius.circular(4),
                bottomRight: Radius.circular(16),
              ),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                AnimatedBuilder(
                  animation: _c,
                  builder: (_, __) => Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      for (var i = 0; i < 3; i++)
                        Padding(
                          padding: EdgeInsets.only(right: i == 2 ? 0 : 4),
                          child: Transform.translate(
                            offset: Offset(
                                0, _offset((_c.value + i * 0.2 / 1.2) % 1.0)),
                            child: Container(
                              width: 6,
                              height: 6,
                              decoration: const BoxDecoration(
                                shape: BoxShape.circle,
                                color: _shAccent,
                              ),
                            ),
                          ),
                        ),
                    ],
                  ),
                ),
                const SizedBox(width: 8),
                const Text("O'ylanmoqda...",
                    style: TextStyle(fontSize: 14, color: _shTextMuted)),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// Savat taklifi — tarkib TO'LIQ ko'rsatiladi.
///
/// Odam nimaga pul to'layotganini ko'rmasdan tasdiqlamasligi kerak;
/// ayniqsa savatni til modeli tuzganda.
class _ProposalCard extends StatelessWidget {
  final Map<String, dynamic> proposal;
  final String addressText;
  final bool hasAddress;
  final VoidCallback? onCart;
  final VoidCallback? onConfirm;

  const _ProposalCard({
    required this.proposal,
    required this.addressText,
    required this.hasAddress,
    required this.onCart,
    required this.onConfirm,
  });

  @override
  Widget build(BuildContext context) {
    final items =
        (proposal['items'] as List? ?? const []).cast<Map<String, dynamic>>();
    final total = (proposal['total_tiyin'] as num?)?.toInt() ?? 0;
    final discount = (proposal['discount_tiyin'] as num?)?.toInt() ?? 0;
    final name = (proposal['restaurant_name'] ?? '').toString();

    return Container(
      margin: const EdgeInsets.only(bottom: 12, left: 36),
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: _shSurface,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: _shBorder2),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (name.isNotEmpty)
            Text(name,
                style: const TextStyle(
                    fontSize: 14, fontWeight: FontWeight.w600, color: _shText)),
          const SizedBox(height: 8),
          for (final it in items)
            Padding(
              padding: const EdgeInsets.only(bottom: 4),
              child: Row(
                children: [
                  Expanded(
                    child: Text('${it['qty']} × ${it['name']}',
                        style: const TextStyle(fontSize: 13.5, color: _shText)),
                  ),
                  Text(
                    '${formatSum(((it['price_tiyin'] as num?)?.toInt() ?? 0) * ((it['qty'] as num?)?.toInt() ?? 1))} so\'m',
                    style: const TextStyle(fontSize: 13, color: _shTextMuted),
                  ),
                ],
              ),
            ),
          if (discount > 0) ...[
            const SizedBox(height: 4),
            Text('Chegirma: −${formatSum(discount)} so\'m',
                style: const TextStyle(fontSize: 12.5, color: Colors.green)),
          ],
          const Divider(height: 18, color: _shBorder2),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              const Text('Jami',
                  style: TextStyle(
                      fontSize: 15,
                      fontWeight: FontWeight.bold,
                      color: _shText)),
              Text('${formatSum(total)} so\'m',
                  style: const TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.bold,
                      color: _shText)),
            ],
          ),
          // ┌─ TASDIQLASH NIMANI ANGLATADI ──────────────────────┐
          // "Tasdiqlash" buyurtmani DARHOL beradi: saqlangan
          // manzilga, naqd pulga. Ilgari bu ikkisi hech qayerda
          // yozilmagan edi — foydalanuvchi taom qayerga ketishini
          // va qanday to'lashini ko'rmasdan tasdiqlardi.
          //
          // Boshqa manzil yoki karta kerak bo'lsa "Savatga" yo'li
          // bor: u checkout ekraniga olib boradi.
          // └────────────────────────────────────────────────────┘
          const SizedBox(height: 10),
          Row(
            children: [
              Icon(hasAddress ? Icons.place_outlined : Icons.error_outline,
                  size: 15, color: hasAddress ? _shTextSub : Colors.orange),
              const SizedBox(width: 5),
              Expanded(
                child: Text(
                  hasAddress ? '$addressText · Naqd pul' : 'Manzil tanlanmagan',
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                      fontSize: 12,
                      color:
                          hasAddress ? _shTextMuted : Colors.orange.shade800),
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),
          Row(
            children: [
              // "Savatga" birinchi: u odatdagi, sinalgan yo'l —
              // manzil/to'lov ko'rib chiqiladi.
              Expanded(
                child: OutlinedButton(
                  onPressed: onCart,
                  style: OutlinedButton.styleFrom(
                    foregroundColor: _shText,
                    side: const BorderSide(color: _shBorder2),
                  ),
                  child: const Text('Shaddiy o''zi qilsin'),
                ),
              ),
              const SizedBox(width: 10),
              Expanded(
                // ┌─ NEGA BU TUGMA TO'Q SARIQ ──────────────────┐
                // Butun ekran Shaddiy'ning binafsha rangida, LEKIN
                // pul sarflaydigan YAGONA tugma OnDex brendida
                // qoladi. Sabab xavfsizlik: u suhbat bezagiga
                // qo'shilib ketmasligi va "shunchaki yana bir
                // tugma" bo'lib ko'rinmasligi kerak.
                // └─────────────────────────────────────────────┘
                child: FilledButton(
                  style: FilledButton.styleFrom(backgroundColor: kBrand),
                  onPressed: onConfirm,
                  child: Text(hasAddress ? 'Tasdiqlash' : 'Manzil tanlash'),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

// ── Ovozli rejim (`.voice`) ──

/// To'liq ekranli ovozli rejim — Shaddiy'dagi `VoiceMode` ning nusxasi.
class _VoiceMode extends StatelessWidget {
  /// Holat yozuvi — matn `_AssistantScreenState._liveLabel` da
  /// hisoblanadi. Vidjet holatni O'ZI talqin qilmaydi: `AiLive`
  /// holatlari bilan bu yerdagi bayroqlar ikki marta bir-biriga
  /// moslashtirilsa, ular vaqt o'tib bir-biridan uzoqlashardi.
  final String label;

  /// Model AYNAN hozir gapiryaptimi (yuz animatsiyasi).
  final bool speaking;

  /// Seans tirikmi — tugma qizil "to'xtatish" bo'ladi.
  final bool active;

  /// Ovoz nosozligi. `null` bo'lmasa AVATAR OSTIDA ko'rsatiladi —
  /// ilgari bu holat ekranda hech qanday iz qoldirmasdi va rejim
  /// "shunchaki javob bermayapti" bo'lib ko'rinardi.
  final String? error;

  /// Suhbatni tugatish.
  ///
  /// Bu tugma "gapirib bo'ldim" EMAS: Gemini'ning o'z nutq aniqlagichi
  /// gap tugaganini jimlikdan biladi, ya'ni mikrofon suhbat davomida
  /// ochiq turadi.
  final VoidCallback onStop;
  final VoidCallback onClose;

  const _VoiceMode({
    required this.label,
    required this.speaking,
    required this.active,
    required this.error,
    required this.onStop,
    required this.onClose,
  });

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: _shBg,
      body: SafeArea(
        child: Stack(
          children: [
            Positioned(
              top: 18,
              left: 16,
              child: InkResponse(
                onTap: onClose,
                radius: 24,
                child: Container(
                  width: 40,
                  height: 40,
                  decoration: const BoxDecoration(
                    shape: BoxShape.circle,
                    color: _shBg,
                  ),
                  child: const Icon(Icons.close, size: 22, color: _shText),
                ),
              ),
            ),
            Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  _SpeakingAvatar(speaking: speaking),
                  const SizedBox(height: 22),
                  Text(label,
                      style: const TextStyle(fontSize: 15, color: _shTextSub)),
                  if (error != null) ...[
                    const SizedBox(height: 12),
                    Padding(
                      padding: const EdgeInsets.symmetric(horizontal: 32),
                      child: Text(
                        error!,
                        textAlign: TextAlign.center,
                        style: const TextStyle(
                          fontSize: 13,
                          height: 1.4,
                          color: Color(0xFF92400E),
                        ),
                      ),
                    ),
                  ],
                ],
              ),
            ),
            Positioned(
              bottom: 48,
              left: 0,
              right: 0,
              child: Center(
                child: InkResponse(
                  onTap: active ? onStop : onClose,
                  radius: 44,
                  child: Container(
                    width: 72,
                    height: 72,
                    decoration: BoxDecoration(
                      shape: BoxShape.circle,
                      color:
                          active ? const Color(0xFFEF4444) : const Color(0xFF9CA3AF),
                      boxShadow: active
                          ? [
                              BoxShadow(
                                color: const Color(0xFFEF4444)
                                    .withValues(alpha: 0.4),
                                blurRadius: 24,
                                offset: const Offset(0, 8),
                              ),
                            ]
                          : null,
                    ),
                    child: Icon(
                      active ? Icons.stop : Icons.mic_off,
                      size: 28,
                      color: Colors.white,
                    ),
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Jonli yuz — 4 kadr ustma-ust, `opacity` bilan almashadi.
///
/// Shaddiy'dagi `ShaddiyFace` bilan bir xil qoida: gapirganda og'iz
/// har 170 ms da katta↔kichik, har 15 soniyada ko'z 160 ms yumiladi.
/// Kadrlar ustma-ust turadi va faqat shaffofligi o'zgaradi — almashish
/// paytida oq yoki bo'sh kadr ko'rinmaydi.
class _SpeakingAvatar extends StatefulWidget {
  final bool speaking;
  const _SpeakingAvatar({required this.speaking});

  @override
  State<_SpeakingAvatar> createState() => _SpeakingAvatarState();
}

class _SpeakingAvatarState extends State<_SpeakingAvatar> {
  Timer? _blinkTimer;
  Timer? _mouthTimer;
  bool _blink = false;
  bool _mouthBig = false;

  @override
  void initState() {
    super.initState();
    _blinkTimer = Timer.periodic(const Duration(seconds: 15), (_) {
      if (!mounted) return;
      setState(() => _blink = true);
      Timer(const Duration(milliseconds: 160), () {
        if (mounted) setState(() => _blink = false);
      });
    });
    _syncMouth();
  }

  @override
  void didUpdateWidget(covariant _SpeakingAvatar old) {
    super.didUpdateWidget(old);
    if (old.speaking != widget.speaking) _syncMouth();
  }

  void _syncMouth() {
    _mouthTimer?.cancel();
    if (!widget.speaking) {
      if (_mouthBig && mounted) setState(() => _mouthBig = false);
      return;
    }
    _mouthTimer = Timer.periodic(const Duration(milliseconds: 170), (_) {
      if (mounted) setState(() => _mouthBig = !_mouthBig);
    });
  }

  @override
  void dispose() {
    _blinkTimer?.cancel();
    _mouthTimer?.cancel();
    super.dispose();
  }

  /// Hozirgi kadr — CSS'dagi `frame` hisobi bilan bir xil tartibda.
  String get _frame {
    if (_blink) return _faceClosed;
    if (!widget.speaking) return _faceIdle;
    return _mouthBig ? _faceLarge : _faceSmall;
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedContainer(
      duration: const Duration(milliseconds: 400),
      width: 200,
      height: 200,
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        // `.voice__avatar--speaking` — gapirganda halqa kengayadi.
        border: Border.all(color: _shBrand, width: 4),
        boxShadow: widget.speaking
            ? [
                BoxShadow(
                  color: _shBrand.withValues(alpha: 0.22),
                  blurRadius: 0,
                  spreadRadius: 12,
                ),
              ]
            : null,
      ),
      child: ClipOval(
        child: Stack(
          fit: StackFit.expand,
          children: [
            for (final f in const [
              _faceIdle,
              _faceLarge,
              _faceSmall,
              _faceClosed,
            ])
              AnimatedOpacity(
                opacity: f == _frame ? 1 : 0,
                duration: const Duration(milliseconds: 70),
                child: Image(image: AssetImage(f), fit: BoxFit.cover),
              ),
          ],
        ),
      ),
    );
  }
}

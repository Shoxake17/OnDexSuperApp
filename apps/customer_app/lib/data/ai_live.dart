import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/foundation.dart';
import 'package:flutter_pcm_sound/flutter_pcm_sound.dart';
import 'package:record/record.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

import '../api.dart';

/// Shaddiy bilan JONLI ovozli suhbat (Gemini Live).
///
/// ┌─ NEGA QURILMADAGI STT/TTS DAN VOZ KECHILDI ────────────────────────┐
/// `speech_to_text` + `flutter_tts` o'zbek tilini QO'LLAMAYDI. Bu
/// taxmin emas, telefonda (Galaxy S23+, Android 16) o'lchandi:
///
///   IntentParsingUtil: Using Locale.getDefault() for recognition: ru-RU
///   GoogleTTSServiceImpl: Synthesis request for locale rus-RUS
///
/// ya'ni o'zbekcha gap RUS modeli bilan tanilardi, javob esa o'zbek
/// matnini RUS ovozi bilan o'qirdi. Qurilmada o'zbekcha nutq paketi
/// ham, o'zbekcha ovoz ham yo'q — sozlama bilan hal qilib bo'lmaydi.
///
/// Gemini Live esa o'zbekcha tabiiy (sintez emas) ovoz beradi.
/// └────────────────────────────────────────────────────────────────────┘
///
/// ┌─ MAXFIYLIK — O'ZGARGAN QAROR ──────────────────────────────────────┐
/// Bu rejimda mikrofon oqimi OnDex serveriga, u yerdan Google'ga
/// ketadi. Ilgari "audio telefondan chiqmaydi" qoidasi bor edi va u
/// AYNAN shu funksiya uchun bekor qilindi (2026-08-31).
///
/// Matnli chat avvalgidek qoladi: u yerda audio UMUMAN yo'q.
/// Foydalanuvchi ovozli rejimni ochishdan oldin bu haqda ogohlantirilib,
/// roziligi so'raladi (`AssistantScreen`).
/// └────────────────────────────────────────────────────────────────────┘
///
/// Kalit ilovada YO'Q: ilova o'z serveriga (`/ai/live`) ulanadi,
/// Gemini bilan gaplashishni server bajaradi.
enum AiLiveState { idle, connecting, active, closed, error }

class AiLive extends ChangeNotifier {
  AiLive({
    this.onProposal,
    this.onText,
    this.onUserText,
    this.onTurnEnd,
    this.onToolError,
    this.onConfirmOrder,
  });

  /// Model savat taklif qilganda — ilova kartani chizadi.
  ///
  /// Buyurtma bu yerdan BERILMAYDI: karta tugmasini odam bosadi.
  /// Ovoz bilan "ha" deyish yetarli emas — noto'g'ri tanilgan gap
  /// pul sarflamasligi kerak.
  final void Function(Map<String, dynamic> proposal)? onProposal;

  /// Shaddiy AYTGAN gapning matni (transkripsiya).
  ///
  /// Bo'laklab keladi ("Avigo", " restoranida", ...) — chaqiruvchi
  /// ularni oxirgi qatorga QO'SHIB boradi, har bo'lakni alohida xabar
  /// qilib emas.
  final void Function(String text)? onText;

  /// Foydalanuvchi AYTGAN gapning matni.
  ///
  /// Busiz suhbat oynasida faqat javoblar qolardi va ular nimaga
  /// tegishli ekani noaniq bo'lardi.
  final void Function(String text)? onUserText;

  /// Navbat tugadi — chaqiruvchi matn to'plagichlarini tozalaydi,
  /// keyingi gap YANGI qatordan boshlanadi.
  final void Function()? onTurnEnd;

  /// Amal BAJARILMADI (masalan taom topilmadi).
  ///
  /// Busiz model xatoni yutib "savatga qo'shdim" deb aytaverardi,
  /// savat esa bo'sh qolardi — ya'ni yordamchi aldardi. Endi
  /// haqiqat suhbatda ko'rinadi.
  final void Function(String message)? onToolError;

  /// Foydalanuvchi og'zaki rozilik berdi — buyurtmani naqd to'lov
  /// bilan tasdiqlash kerak.
  ///
  /// Server buyurtma YARATMAYDI: bu faqat signal, amalni ilova
  /// odatdagi checkout tugmasi bilan bajaradi.
  final void Function()? onConfirmOrder;

  // ── Audio formatlari — server bilan kelishilgan, o'zgartirib
  // bo'lmaydi (`GET /ai/status` da ham qaytariladi). Noto'g'ri
  // chastota = "cho'zilgan" yoki "chiyillagan" ovoz.
  static const _inputRate = 16000;
  static const _outputRate = 24000;

  final _recorder = AudioRecorder();
  WebSocketChannel? _ch;
  StreamSubscription<dynamic>? _wsSub;
  StreamSubscription<Uint8List>? _micSub;

  AiLiveState _state = AiLiveState.idle;
  AiLiveState get state => _state;

  bool _speaking = false;

  /// Model AYNAN hozir gapiryaptimi (yuz animatsiyasi shunga qarab).
  bool get speaking => _speaking;

  String? _error;
  String? get error => _error;

  /// Chalinishni kutayotgan audio bo'laklari.
  ///
  /// ┌─ NEGA O'Z NAVBATIMIZ ──────────────────────────────────────────┐
  /// `flutter_pcm_sound` da navbatni tozalash usuli yo'q. Foydalanuvchi
  /// model gapirayotganda gapirsa (`interrupted`), eski javob
  /// TO'XTAShI kerak — aks holda u yangi savol ustidan gapiraveradi.
  /// Shuning uchun bo'laklar shu yerda turadi va faqat plagin
  /// so'raganda beriladi; uzilishda navbat shunchaki tashlanadi.
  /// └────────────────────────────────────────────────────────────────┘
  final _queue = <Int16List>[];

  bool _playerReady = false;

  /// Chalgich nasosi hozir to'xtaganmi.
  ///
  /// ┌─ NEGA BU BAYROQ KERAK ─────────────────────────────────────────┐
  /// `flutter_pcm_sound` ning Android tomonidagi sikli FAQAT feed'ga
  /// javoban aylanadi (`FlutterPcmSoundPlugin.java`):
  ///
  ///   data = mSamples.take();          // navbat bo'sh bo'lsa ABADIY bloklaydi
  ///   mAudioTrack.write(...);
  ///   isLowBufferEvent = remaining <= threshold && lastFeed != totalFeeds;
  ///   if (event) invokeFeedCallback(...);
  ///
  /// Ya'ni hodisa faqat yozuvdan KEYIN tekshiriladi va har `feed()`
  /// uchun BIR MARTA yuboriladi. Demak qo'ng'iroqqa javoban feed
  /// qilmasak, sikl `take()` da abadiy qotadi va CHALGICH BUTUNLAY
  /// O'LADI — keyingi javoblar navbatga tushadi-yu, hech qachon
  /// chalinmaydi. Aynan shu bo'lgan edi: Shaddiy bir marta gapirib,
  /// keyin faqat yuzi qimirlardi.
  ///
  /// Ilgari bu jimlik berib aylantirish bilan "hal qilingan" edi,
  /// lekin u sekundiga ~24 marta platforma chaqiruvi qilardi va
  /// past bufer hodisasi (remaining > 0) kelganda plaginning o'z
  /// `start()` tiklovchisi ham ishlamasdi.
  ///
  /// Endi nasos holati SHU YERDA saqlanadi: navbat bo'shasa nasos
  /// to'xtaydi, yangi bo'lak kelganda esa darhol qayta yuritiladi.
  /// └────────────────────────────────────────────────────────────────┘
  bool _pumpIdle = true;

  /// Mikrofon ruxsati bormi (yo'q bo'lsa so'raladi).
  Future<bool> hasPermission() => _recorder.hasPermission();

  Future<void> start() async {
    if (_state == AiLiveState.connecting || _state == AiLiveState.active) {
      return;
    }
    _set(AiLiveState.connecting, err: null);

    try {
      if (!await _recorder.hasPermission()) {
        _fail('Mikrofonga ruxsat berilmadi.');
        return;
      }

      await _startPlayer();

      // Bilet — `/ws` bilan bir xil mexanizm: WebSocket handshake'da
      // `Authorization` header qo'yib bo'lmaydi.
      final ticket = await api.wsTicket();
      final url = '${apiBaseUrl.replaceFirst('http', 'ws')}'
          '/ai/live?ticket=$ticket';

      final ch = WebSocketChannel.connect(Uri.parse(url));
      _ch = ch;
      await ch.ready;

      _wsSub = ch.stream.listen(
        _onFrame,
        onError: (_) => _fail('Ovozli aloqa uzildi.'),
        onDone: () {
          // Server yopdi — bu xato emas (muddat tugadi yoki seans
          // yakunlandi). Xato holatidagi matn ustidan yozmaymiz.
          if (_state != AiLiveState.error) _set(AiLiveState.closed);
        },
        cancelOnError: true,
      );

      await _startMic();
      _set(AiLiveState.active);
    } catch (e) {
      _fail('Ulanib bo\'lmadi: $e');
    }
  }

  Future<void> _startPlayer() async {
    if (_playerReady) return;
    await FlutterPcmSound.setup(
      sampleRate: _outputRate,
      channelCount: 1,
    );
    // Chegara ATAYLAB kichik: uzilish (`interrupted`) kelganda
    // plaginga allaqachon berilgan audio baribir oxirigacha
    // chalinadi, ya'ni bu qiymat "eski javob qancha vaqt eshitiladi"
    // demakdir.
    // ┌─ BUFER KATTA BO'LISHI SHART ──────────────────────────────────┐
    // Ilgari chegara 2000 kadr (24kHz da ~83 ms) edi va har
    // qo'ng'iroqda ATIGI BITTA bo'lak berilardi. Tarmoq bo'laklari
    // to'lqin bilan keladi (bir necha bo'lak birdan, keyin pauza),
    // shuning uchun bufer muntazam bo'shab qolardi va ovoz
    // BO'LINIB, uzilib eshitilardi.
    //
    // Endi chegara ~333 ms va qo'ng'iroqda navbat bo'shagunicha
    // (yoki ~1 soniya to'lgunicha) bo'laklar beriladi.
    // └───────────────────────────────────────────────────────────────┘
    await FlutterPcmSound.setFeedThreshold(_thresholdFrames);
    FlutterPcmSound.setFeedCallback(_feed);
    _playerReady = true;
    _pumpIdle = true;
    // `start()` ATAYLAB chaqirilmaydi: u navbat bo'sh bo'lganda
    // qo'ng'iroqni bekorga uyg'otardi. Nasos birinchi audio bo'lagi
    // kelganda `_pump()` orqali yuritiladi.
  }

  /// Chalgich buferining eng kam darajasi (kadrlarda, 24kHz).
  ///
  /// ~333 ms: tarmoqdagi odatiy tebranishni yutadi, lekin uzilishga
  /// (`interrupted`) javob berish sezilarli kechikmaydi — bufer
  /// tozalanmaydi, faqat navbat tashlanadi.
  static const _thresholdFrames = 8000;

  /// Bir qo'ng'iroqda beriladigan eng ko'p audio (~1 soniya).
  ///
  /// Cheksiz berilsa butun javob (bir necha soniya) birdan
  /// buferga tushardi va `interrupted` kelganda foydalanuvchi
  /// eskisini oxirigacha eshitib turardi.
  static const _feedBudgetFrames = 24000;

  /// Chalishni boshlashdan oldin yig'iladigan zaxira (~350 ms).
  ///
  /// ┌─ NEGA KUTAMIZ ─────────────────────────────────────────────────┐
  /// Birinchi bo'lak kelishi bilan chalinsa, ikkinchisi kechiksa
  /// ovoz boshidayoq uzilardi. Kichik zaxira gapning boshini
  /// silliq qiladi va qolgan qismi uzluksiz ketadi.
  /// └────────────────────────────────────────────────────────────────┘
  static const _prerollFrames = 8400;

  /// Navbatdagi kadrlar soni.
  int get _queuedFrames {
    var n = 0;
    for (final c in _queue) {
      n += c.length;
    }
    return n;
  }

  /// Navbat tugagani ma'lum — endi zaxira kutilmaydi (aks holda
  /// qisqa javob umuman chalinmasdi).
  bool _turnEnded = false;

  void _feed(int remaining) {
    if (_queue.isEmpty) {
      // Navbat bo'sh — nasos to'xtaydi. Buni BELGILAB qo'yamiz:
      // yangi bo'lak kelganda `_pump()` uni qayta yuritadi.
      _pumpIdle = true;
      if (_speaking) {
        _speaking = false;
        notifyListeners();
      }
      return;
    }
    _pumpIdle = false;

    // Bufer to'lgunicha bo'lak beramiz — bittasi bilan cheklanish
    // aynan uzilishlarga olib kelardi.
    var fed = 0;
    while (_queue.isNotEmpty && remaining + fed < _feedBudgetFrames) {
      final chunk = _queue.removeAt(0);
      fed += chunk.length;
      FlutterPcmSound.feed(PcmArrayInt16(bytes: chunk.buffer.asByteData()));
    }
  }

  /// Nasos to'xtagan bo'lsa qayta yuritadi.
  ///
  /// Plaginning o'z `start()` i bu yerda YETARLI EMAS: u faqat
  /// `remainingFrames == 0` bo'lganda tiklaydi, past bufer hodisasi
  /// esa nolga yetmasdan ham kelishi mumkin.
  void _pump() {
    if (!_playerReady || !_pumpIdle || _queue.isEmpty) return;
    // Zaxira yig'ilmaguncha kutamiz — gapning boshi uzilmasin.
    // Navbat tugagan bo'lsa kutmaymiz: qisqa javob (bir necha
    // bo'lak) hech qachon zaxiraga yetmasdi va umuman chalinmasdi.
    if (!_turnEnded && _queuedFrames < _prerollFrames) return;
    _feed(0);
  }

  Future<void> _startMic() async {
    final stream = await _recorder.startStream(
      const RecordConfig(
        encoder: AudioEncoder.pcm16bits,
        sampleRate: _inputRate,
        numChannels: 1,
        // ┌─ AKUSTIK AKS-SADO BOSTIRISH ──────────────────────────────┐
        // `voiceCommunication` manbasi Android'ning AEC/NS
        // zanjirini yoqadi. Busiz mikrofon dinamikdan chiqayotgan
        // JAVOBNI eshitadi va model o'z ovoziga javob berib,
        // cheksiz siklga tushadi.
        // └───────────────────────────────────────────────────────────┘
        androidConfig: AndroidRecordConfig(
          audioSource: AndroidAudioSource.voiceCommunication,
        ),
      ),
    );
    _micSub = stream.listen(
      (chunk) {
        final ch = _ch;
        if (ch == null || _state == AiLiveState.error) return;
        // BINAR kadr — base64 qilinsa trafik 33% oshardi va mikrofon
        // oqimi uzluksiz.
        ch.sink.add(chunk);
      },
      onError: (_) => _fail('Mikrofon oqimi uzildi.'),
      cancelOnError: true,
    );
  }

  void _onFrame(dynamic raw) {
    if (raw is! String) return;
    Map<String, dynamic> ev;
    try {
      final decoded = jsonDecode(raw);
      if (decoded is! Map<String, dynamic>) return;
      ev = decoded;
    } catch (_) {
      return; // buzilgan kadr — ulanish tirik qoladi
    }

    switch (ev['type']) {
      case 'audio':
        final b64 = ev['audio'] as String?;
        if (b64 == null || b64.isEmpty) return;
        // Yangi audio — yangi navbat boshlandi.
        _turnEnded = false;
        _queue.add(_toInt16(base64Decode(b64)));
        // Nasos to'xtagan bo'lsa (oldingi navbat tugagan) qayta
        // yuritamiz — busiz bo'lak navbatda yotib qolardi.
        _pump();
        if (!_speaking) {
          _speaking = true;
          notifyListeners();
        }
      case 'interrupted':
        // Foydalanuvchi gapirdi — eski javobni tashlaymiz. Nasos
        // navbat bo'shagach o'zi to'xtaydi va `_pumpIdle` ni
        // belgilaydi.
        _queue.clear();
        if (_speaking) {
          _speaking = false;
          notifyListeners();
        }
      case 'turn_end':
        // Audio hali navbatda bo'lishi mumkin — `_speaking` ni
        // ATAYLAB shu yerda o'chirmaymiz, uni `_feed` navbat
        // bo'shaganda o'chiradi. Aks holda yuz animatsiyasi ovoz
        // tugashidan oldin to'xtardi.
        //
        // Zaxira kutish ham shu yerda bekor qilinadi: gap tugagan,
        // ya'ni ko'proq audio kelmaydi va qolganini darhol
        // chalish kerak.
        _turnEnded = true;
        _pump();
        onTurnEnd?.call();
      case 'user_text':
        final t = ev['text'] as String?;
        if (t != null && t.isNotEmpty) onUserText?.call(t);
      case 'proposal':
        final p = ev['proposal'];
        if (p is Map<String, dynamic>) onProposal?.call(p);
      case 'tool_error':
        final m = (ev['text'] as String?) ?? 'Amal bajarilmadi.';
        onToolError?.call(m);
      case 'confirm_order':
        onConfirmOrder?.call();
      case 'text':
        final t = ev['text'] as String?;
        if (t != null && t.isNotEmpty) onText?.call(t);
      case 'error':
        _fail((ev['error'] as String?) ?? 'Ovozli yordamchi xatosi.');
      case 'ready':
        break;
    }
  }

  /// Bayt oqimini PCM16 namunalariga o'giradi.
  ///
  /// `Uint8List.buffer.asInt16List()` TO'G'RIDAN-TO'G'RI ishlatilmaydi:
  /// base64 dekoderi qaytargan ro'yxatning `offsetInBytes` i nolga teng
  /// bo'lmasligi va uzunligi toq bo'lishi mumkin — ikkalasi ham
  /// istisno tashlardi.
  Int16List _toInt16(Uint8List bytes) {
    final n = bytes.length ~/ 2;
    final out = Int16List(n);
    final bd = ByteData.sublistView(bytes);
    for (var i = 0; i < n; i++) {
      out[i] = bd.getInt16(i * 2, Endian.little);
    }
    return out;
  }

  /// Mikrofonni o'chirish — "gapirib bo'ldim" signali.
  ///
  /// Busiz model jimlik kutib qolardi: avtomatik nutq aniqlash
  /// jimlikni kutadi, mikrofon esa allaqachon yopilgan va jimlik ham
  /// kelmaydi.
  void endTurn() {
    final ch = _ch;
    if (ch == null) return;
    ch.sink.add(jsonEncode({'type': 'end'}));
  }

  /// Ilova rasmiylashtirish ekraniga yetib bordi — Shaddiy endi
  /// to'lovni so'rasin.
  ///
  /// ┌─ MATN YUBORILMAYDI, FAKT YUBORILADI ───────────────────────────┐
  /// Bu yerda modelga tayyor gap emas, faqat restoran nomi va summa
  /// ketadi; gapni SERVER tuzadi. Aks holda ilovani o'zgartirgan
  /// odam modelning ko'rsatmasini qayta yozib yuborardi.
  /// └────────────────────────────────────────────────────────────────┘
  void checkoutReady({required String restaurant, required int totalTiyin}) {
    final ch = _ch;
    if (ch == null || _state != AiLiveState.active) return;
    ch.sink.add(jsonEncode({
      'type': 'checkout_ready',
      'restaurant': restaurant,
      'total_tiyin': totalTiyin,
    }));
  }

  Future<void> stop() async {
    await _micSub?.cancel();
    _micSub = null;
    try {
      await _recorder.stop();
    } catch (_) {
      // Yozuv boshlanmagan bo'lsa istisno tashlaydi — ahamiyatsiz.
    }
    await _wsSub?.cancel();
    _wsSub = null;
    await _ch?.sink.close();
    _ch = null;
    _queue.clear();
    _pumpIdle = true;
    if (_playerReady) {
      FlutterPcmSound.setFeedCallback(null);
      await FlutterPcmSound.release();
      _playerReady = false;
    }
    _speaking = false;
    if (_state != AiLiveState.error) _set(AiLiveState.closed);
  }

  void _fail(String msg) {
    _error = msg;
    _set(AiLiveState.error);
    // Ulanishni yopamiz, lekin `stop()` ni chaqirmaymiz — u holatni
    // `closed` ga o'zgartirib, sababni ekrandan o'chirardi.
    _micSub?.cancel();
    _micSub = null;
    _recorder.stop().catchError((_) => null);
    _ch?.sink.close();
    _ch = null;
    _queue.clear();
  }

  void _set(AiLiveState s, {String? err = _keep}) {
    _state = s;
    if (!identical(err, _keep)) _error = err;
    notifyListeners();
  }

  /// `_set` da "xatoga tegma" ni bildiradigan qiymat (`null` ning o'zi
  /// "xatoni tozala" degani).
  static const _keep = ' keep';

  @override
  void dispose() {
    stop();
    _recorder.dispose();
    super.dispose();
  }
}

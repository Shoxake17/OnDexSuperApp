import 'dart:convert';
import 'dart:math';

import 'package:flutter/foundation.dart';

import 'models.dart';

typedef SupportFetch = Future<Map<String, dynamic>> Function({int? before, int? after, int limit});
typedef SupportSend = Future<Map<String, dynamic>> Function(String body, String clientId);
typedef SupportSendImage = Future<Map<String, dynamic>> Function(
    String body, String clientId, Uint8List bytes, String filename);
typedef SupportMarkRead = Future<Map<String, dynamic>> Function(int upToSeq);

final _rng = Random.secure();

/// Takroriy yuborishga qarshi kalit: 18 tasodifiy bayt -> 24 belgi
/// (`[A-Za-z0-9_-]`, server shaklni tekshiradi).
String newSupportClientId() {
  final bytes = List<int>.generate(18, (_) => _rng.nextInt(256));
  return base64UrlEncode(bytes).replaceAll('=', '');
}

String _errorText(Object e) {
  final s = e.toString().trim();
  if (s.isEmpty) return 'Xabar yuborilmadi';
  return s.length > 160 ? '${s.substring(0, 159)}…' : s;
}

/// Bitta suhbatning holati: yuklash, sahifalash, jonli hodisalar,
/// optimistik yuborish (matn va rasm) va o'qish belgisi.
///
/// ┌─ UZILISHSIZ ──────────────────────────────────────────────────────┐
/// * Xabar `client_id` bilan yuboriladi: javob kelmay qolsa "Qayta
///   yuborish" AYNI kalit bilan ketadi va server ikkinchi nusxa
///   yaratmaydi (rasm ham qayta saqlanmaydi).
/// * Bitta xabar ham HTTP javobidan, ham jonli kanaldan kelishi mumkin —
///   `id` bo'yicha bir marta ko'rsatiladi.
/// * Soket qayta ulanganda [catchUp] oxirgi olingan raqamdan keyingilarini
///   so'raydi. `seq` butun platforma bo'yicha o'sadi, ya'ni raqamlar
///   orasidagi "teshik" normal — tiklash faqat qayta ulanishga tayanadi.
/// └───────────────────────────────────────────────────────────────────┘
class SupportConversation extends ChangeNotifier {
  SupportConversation({
    required this.viewer,
    required this.fetch,
    required this.sendMessage,
    required this.markRead,
    this.sendImage,
    this.restaurantId,
    this.pageSize = 50,
  });

  /// `restaurant` yoki `admin`.
  final String viewer;

  /// Admin kanalida hodisalar BARCHA restoranlardan keladi — faqat shu
  /// suhbatnikini qabul qilish uchun. Restoran tomonida `null`.
  final String? restaurantId;
  final SupportFetch fetch;
  final SupportSend sendMessage;

  /// Rasm yuborish (multipart). `null` — bu panelda rasm yuborilmaydi.
  final SupportSendImage? sendImage;
  final SupportMarkRead markRead;
  final int pageSize;

  final List<SupportMessage> _messages = [];
  final List<SupportMessage> _outbox = [];

  SupportThread? thread;
  Map<String, dynamic> lastResponse = const {};
  bool loading = false;
  bool loadingOlder = false;
  bool hasOlder = false;
  String? error;

  /// Sahifa ko'rinib turibdimi — ko'rinmasa xabar o'qilgan deb belgilanmaydi.
  bool active = true;

  bool _disposed = false;
  bool _marking = false;
  bool _markAgain = false;

  bool get canSendImages => sendImage != null;

  List<SupportMessage> get messages => List.unmodifiable([..._messages, ..._outbox]);

  int get latestSeq => _messages.isEmpty ? 0 : _messages.last.seq;

  void _notify() {
    if (!_disposed) notifyListeners();
  }

  @override
  void dispose() {
    _disposed = true;
    super.dispose();
  }

  List<SupportMessage> _items(Map<String, dynamic> res) {
    final raw = res['items'];
    if (raw is! List) return const [];
    return raw.map(SupportMessage.tryParse).whereType<SupportMessage>().toList();
  }

  void _applyThread(Object? json) {
    final t = SupportThread.tryParse(json);
    if (t == null) return;
    thread = thread == null ? t : thread!.merge(t);
  }

  void _merge(Iterable<SupportMessage> items) {
    for (var m in items) {
      if (m.sender == viewer && m.clientId.isNotEmpty) {
        final local = _outbox.where((o) => o.clientId == m.clientId).toList();
        if (local.isNotEmpty && local.first.localImage != null) {
          m = m.copyWith(localImage: local.first.localImage);
        }
        _outbox.removeWhere((o) => o.clientId == m.clientId);
      }
      final i = _messages.indexWhere((x) => x.id == m.id);
      if (i >= 0) {
        // Mavjud xabarning mahalliy rasmini yo'qotmaymiz.
        final keep = _messages[i].localImage;
        _messages[i] = keep != null && m.localImage == null ? m.copyWith(localImage: keep) : m;
      } else {
        _messages.add(m);
      }
    }
    _messages.sort((a, b) => a.seq.compareTo(b.seq));
  }

  Future<void> load() async {
    loading = true;
    error = null;
    _notify();
    try {
      final res = await fetch(limit: pageSize);
      if (_disposed) return;
      _messages.clear();
      _merge(_items(res));
      hasOlder = (res['next_before'] is num) && (res['next_before'] as num) > 0;
      _applyThread(res['thread']);
      lastResponse = res;
    } catch (e) {
      error = _errorText(e);
    } finally {
      loading = false;
    }
    _notify();
    await markSeen();
  }

  Future<void> loadOlder() async {
    if (loadingOlder || !hasOlder || _messages.isEmpty) return;
    loadingOlder = true;
    _notify();
    try {
      final res = await fetch(before: _messages.first.seq, limit: pageSize);
      if (_disposed) return;
      _merge(_items(res));
      hasOlder = (res['next_before'] is num) && (res['next_before'] as num) > 0;
    } catch (_) {
      // Keyingi aylantirishda qayta urinadi.
    } finally {
      loadingOlder = false;
    }
    _notify();
  }

  /// Soket qayta ulanganda yoki zaxira taymerida.
  Future<void> catchUp() async {
    if (loading) return;
    if (_messages.isEmpty) return load();
    try {
      var after = latestSeq;
      for (var page = 0; page < 20; page++) {
        final res = await fetch(after: after, limit: 100);
        if (_disposed) return;
        final items = _items(res);
        _merge(items);
        _applyThread(res['thread']);
        lastResponse = res;
        if (items.length < 100) break;
        after = items.last.seq;
      }
      error = null;
    } catch (_) {
      return;
    }
    _notify();
    await markSeen();
  }

  /// Jonli kanal hodisasi. Tegishli bo'lsa `true`.
  bool handleEvent(Map<String, dynamic> e) {
    final type = e['type'];
    if (type != 'support_message' && type != 'support_read') return false;
    if (restaurantId != null && e['restaurant_id'] != restaurantId) return false;
    if (type == 'support_message') {
      final m = SupportMessage.tryParse(e['message']);
      if (m == null) return false;
      _merge([m]);
    }
    _applyThread(e['thread']);
    _notify();
    if (type == 'support_message') markSeen();
    return true;
  }

  /// Matn va/yoki rasm yuboradi. Rasmsiz bo'sh matn yuborilmaydi.
  Future<void> send(String text, {SupportImageDraft? image}) async {
    final body = text.trim();
    if (body.runes.length > kSupportMaxBody) return;
    if (image == null && body.isEmpty) return;
    if (image != null && (sendImage == null || validateSupportImage(image) != null)) return;
    final pending = SupportMessage.outgoing(viewer, body, newSupportClientId(), image: image);
    _outbox.add(pending);
    _notify();
    await _deliver(pending);
  }

  Future<void> retry(SupportMessage m) async {
    final i = _outbox.indexWhere((o) => o.clientId == m.clientId);
    if (i < 0) return;
    _outbox[i] = m.copyWith(pending: true, failed: false);
    _notify();
    await _deliver(_outbox[i]);
  }

  void discard(SupportMessage m) {
    _outbox.removeWhere((o) => o.clientId == m.clientId && !o.confirmed);
    _notify();
  }

  Future<void> _deliver(SupportMessage p) async {
    try {
      final image = p.localImage;
      final res = image != null
          ? await sendImage!(p.body, p.clientId, image, p.filename)
          : await sendMessage(p.body, p.clientId);
      if (_disposed) return;
      final m = SupportMessage.tryParse(res['message']);
      if (m != null) _merge([m]);
      _applyThread(res['thread']);
    } catch (e) {
      final i = _outbox.indexWhere((o) => o.clientId == p.clientId);
      if (i >= 0) _outbox[i] = p.copyWith(pending: false, failed: true, error: _errorText(e));
    }
    _notify();
  }

  /// Qarshi tomonning ko'rinib turgan xabarlarini o'qilgan deb belgilaydi.
  Future<void> markSeen() async {
    final t = thread;
    if (!active || t == null || _disposed) return;
    var lastPeer = 0;
    for (final m in _messages) {
      if (m.sender != viewer && m.seq > lastPeer) lastPeer = m.seq;
    }
    if (lastPeer == 0 || lastPeer <= t.readSeq) return;
    if (_marking) {
      _markAgain = true;
      return;
    }
    _marking = true;
    try {
      final res = await markRead(latestSeq);
      if (_disposed) return;
      _applyThread(res['thread']);
      _notify();
    } catch (_) {
      // Keyingi hodisada qayta urinadi.
    } finally {
      _marking = false;
    }
    if (_markAgain) {
      _markAgain = false;
      await markSeen();
    }
  }
}

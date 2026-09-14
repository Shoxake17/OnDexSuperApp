import 'dart:typed_data';

import 'links.dart';

/// Server bilan bir xil chegara (`internal/support.MaxBodyLen`, belgilar
/// soni — UTF-16 birliklari emas).
const kSupportMaxBody = 2000;

/// Hisoblagich shu uzunlikdan keyin ko'rinadi.
const kSupportCounterFrom = 1800;

/// Yuboriladigan rasmning xom hajmi (`internal/support.MaxUploadBytes`).
const kSupportMaxImageBytes = 10 * 1024 * 1024;

/// Server qabul qiladigan kengaytmalar.
const kSupportImageExtensions = ['jpg', 'jpeg', 'png', 'webp'];

const kSideRestaurant = 'restaurant';
const kSideAdmin = 'admin';

String _str(Object? v) => v is String ? v : '';
int _int(Object? v) => v is num ? v.toInt() : 0;
DateTime? _time(Object? v) => v is String ? DateTime.tryParse(v)?.toLocal() : null;

final _idRe = RegExp(r'^[A-Za-z0-9_-]{1,64}$');

/// Rasm tanlash/yuborishdagi foydalanuvchiga ko'rsatiladigan xato.
class SupportImageException implements Exception {
  const SupportImageException(this.message);

  final String message;

  @override
  String toString() => message;
}

/// Xabarga biriktirilgan rasm (serverdagi metama'lumot, baytlarsiz).
class SupportAttachment {
  const SupportAttachment({
    required this.id,
    required this.contentType,
    required this.width,
    required this.height,
    this.size = 0,
  });

  final String id;
  final String contentType;
  final int width;
  final int height;
  final int size;

  double get aspectRatio => width / height;

  /// ID havola yo'liga qo'yiladi — shakli qat'iy tekshiriladi (`../` o'tmaydi).
  static SupportAttachment? tryParse(Object? json) {
    if (json is! Map) return null;
    final id = json['id'];
    final w = json['width'];
    final h = json['height'];
    if (id is! String || !_idRe.hasMatch(id) || w is! num || h is! num || w <= 0 || h <= 0) return null;
    return SupportAttachment(
      id: id,
      contentType: _str(json['content_type']),
      width: w.toInt(),
      height: h.toInt(),
      size: _int(json['size']),
    );
  }
}

/// Tanlangan, hali yuborilmagan rasm.
class SupportImageDraft {
  const SupportImageDraft({required this.bytes, required this.filename});

  final Uint8List bytes;
  final String filename;
}

/// Yuborishdan OLDIN klientdagi tekshiruv (server baribir qayta tekshiradi
/// va rasmni qayta kodlaydi). `null` — muammo yo'q.
String? validateSupportImage(SupportImageDraft d) {
  final b = d.bytes;
  if (b.isEmpty) return 'Rasm bo\'sh';
  if (b.length > kSupportMaxImageBytes) return 'Rasm 10 MB dan oshmasligi kerak';
  final name = d.filename.toLowerCase();
  final dot = name.lastIndexOf('.');
  if (dot < 0 || !kSupportImageExtensions.contains(name.substring(dot + 1))) {
    return 'Faqat JPG, PNG yoki WEBP rasm yuborish mumkin';
  }
  bool starts(List<int> sig, [int offset = 0]) {
    if (b.length < offset + sig.length) return false;
    for (var i = 0; i < sig.length; i++) {
      if (b[offset + i] != sig[i]) return false;
    }
    return true;
  }

  final png = starts(const [0x89, 0x50, 0x4E, 0x47]);
  final jpeg = starts(const [0xFF, 0xD8, 0xFF]);
  final webp = starts(const [0x52, 0x49, 0x46, 0x46]) && starts(const [0x57, 0x45, 0x42, 0x50], 8);
  if (!png && !jpeg && !webp) return 'Fayl rasm emas yoki buzilgan';
  return null;
}

/// Bitta chat xabari. `seq == 0` — hali serverga yetmagan (navbatda yoki
/// yuborilmadi).
class SupportMessage {
  const SupportMessage({
    required this.id,
    required this.seq,
    required this.sender,
    required this.senderName,
    required this.body,
    required this.clientId,
    required this.createdAt,
    this.attachment,
    this.localImage,
    this.filename = '',
    this.pending = false,
    this.failed = false,
    this.error,
  });

  final String id;
  final int seq;
  final String sender;
  final String senderName;
  final String body;
  final String clientId;
  final DateTime createdAt;

  /// Serverdagi rasm (bo'lsa).
  final SupportAttachment? attachment;

  /// Shu seansda yuborilgan rasmning baytlari — server javobidan keyin ham
  /// ko'rsatiladi (qayta yuklanmaydi, "miltillamaydi").
  final Uint8List? localImage;
  final String filename;
  final bool pending;
  final bool failed;
  final String? error;

  bool get confirmed => seq > 0;
  bool get hasImage => attachment != null || localImage != null;

  /// Serverdan kelgan JSON. Shakli buzuq yoki noma'lum tomon — `null`
  /// (ekranga chiqarilmaydi).
  static SupportMessage? tryParse(Object? json) {
    if (json is! Map) return null;
    final id = json['id'];
    final seq = json['seq'];
    final sender = json['sender'];
    final body = json['body'];
    if (id is! String || id.isEmpty || seq is! num || seq <= 0 || body is! String) return null;
    if (sender != kSideRestaurant && sender != kSideAdmin) return null;
    final attachment = SupportAttachment.tryParse(json['attachment']);
    if (body.isEmpty && attachment == null) return null;
    return SupportMessage(
      id: id,
      seq: seq.toInt(),
      sender: sender as String,
      senderName: _str(json['sender_name']),
      body: body,
      clientId: _str(json['client_id']),
      createdAt: _time(json['created_at']) ?? DateTime.now(),
      attachment: attachment,
    );
  }

  factory SupportMessage.outgoing(String sender, String body, String clientId, {SupportImageDraft? image}) =>
      SupportMessage(
        id: '',
        seq: 0,
        sender: sender,
        senderName: '',
        body: body,
        clientId: clientId,
        createdAt: DateTime.now(),
        localImage: image?.bytes,
        filename: image?.filename ?? '',
        pending: true,
      );

  SupportMessage copyWith({bool? pending, bool? failed, String? error, Uint8List? localImage}) => SupportMessage(
        id: id,
        seq: seq,
        sender: sender,
        senderName: senderName,
        body: body,
        clientId: clientId,
        createdAt: createdAt,
        attachment: attachment,
        localImage: localImage ?? this.localImage,
        filename: filename,
        pending: pending ?? this.pending,
        failed: failed ?? this.failed,
        error: error,
      );
}

/// Suhbat xulosasi — KO'RUVCHI tomoniga moslangan (`unread` — mening
/// o'qimaganlarim, `peerReadSeq` — qarshi tomon qayergacha o'qigani).
class SupportThread {
  const SupportThread({
    required this.restaurantId,
    this.messageCount = 0,
    this.firstAt,
    this.lastAt,
    this.lastSeq = 0,
    this.lastSender = '',
    this.lastBody = '',
    this.lastHasImage = false,
    this.readSeq = 0,
    this.peerReadSeq = 0,
    this.unread = 0,
  });

  final String restaurantId;
  final int messageCount;
  final DateTime? firstAt;
  final DateTime? lastAt;
  final int lastSeq;
  final String lastSender;
  final String lastBody;
  final bool lastHasImage;
  final int readSeq;
  final int peerReadSeq;
  final int unread;

  static SupportThread? tryParse(Object? json) {
    if (json is! Map) return null;
    return SupportThread(
      restaurantId: _str(json['restaurant_id']),
      messageCount: _int(json['message_count']),
      firstAt: _time(json['first_at']),
      lastAt: _time(json['last_at']),
      lastSeq: _int(json['last_seq']),
      lastSender: _str(json['last_sender']),
      lastBody: _str(json['last_body']),
      lastHasImage: json['last_has_image'] == true,
      readSeq: _int(json['read_seq']),
      peerReadSeq: _int(json['peer_read_seq']),
      unread: _int(json['unread']),
    );
  }

  /// Ikki suratni birlashtiradi. Jonli hodisa va HTTP javobi teskari
  /// tartibda yetib kelishi mumkin — eskisi yangisini orqaga qaytarmasin:
  /// o'qish belgilari faqat o'sadi, qolgani eng yangi suratdan olinadi.
  SupportThread merge(SupportThread? other) {
    if (other == null) return this;
    final newer = (other.lastSeq > lastSeq || (other.lastSeq == lastSeq && other.readSeq >= readSeq)) ? other : this;
    return SupportThread(
      restaurantId: newer.restaurantId.isNotEmpty ? newer.restaurantId : restaurantId,
      messageCount: newer.messageCount,
      firstAt: newer.firstAt ?? firstAt ?? other.firstAt,
      lastAt: newer.lastAt ?? lastAt,
      lastSeq: newer.lastSeq,
      lastSender: newer.lastSender,
      lastBody: newer.lastBody,
      lastHasImage: newer.lastHasImage,
      readSeq: readSeq > other.readSeq ? readSeq : other.readSeq,
      peerReadSeq: peerReadSeq > other.peerReadSeq ? peerReadSeq : other.peerReadSeq,
      unread: newer.unread,
    );
  }
}

/// Admin kiritgan aloqa ma'lumotlari. Havolalar serverdagi `telegram_url`
/// dan emas, shu yerda QAYTA TEKSHIRILGAN qiymatdan quriladi — javob
/// o'zgartirilgan bo'lsa ham panel begona sxemani (`javascript:`, `file:`)
/// ochmaydi.
class SupportContacts {
  const SupportContacts({
    this.phone = '',
    this.phoneHours = '',
    this.telegram = '',
    this.email = '',
    this.emailNote = '',
    this.updatedAt,
  });

  final String phone;
  final String phoneHours;
  final String telegram;
  final String email;
  final String emailNote;
  final DateTime? updatedAt;

  factory SupportContacts.fromJson(Object? json) {
    if (json is! Map) return const SupportContacts();
    return SupportContacts(
      phone: _str(json['phone']),
      phoneHours: _str(json['phone_hours']),
      telegram: _str(json['telegram']),
      email: _str(json['email']),
      emailNote: _str(json['email_note']),
      updatedAt: _time(json['updated_at']),
    );
  }

  Uri? get telUri => supportTelUri(phone);
  Uri? get telegramUri => supportTelegramUri(telegram);
  Uri? get mailUri => supportMailUri(email, subject: 'OnDex Restoran paneli');

  bool get hasPhone => telUri != null;
  bool get hasTelegram => telegramUri != null;
  bool get hasEmail => mailUri != null;
  bool get configured => hasPhone || hasTelegram || hasEmail;

  Map<String, dynamic> toJson() => {
        'phone': phone,
        'phone_hours': phoneHours,
        'telegram': telegram,
        'email': email,
        'email_note': emailNote,
      };
}

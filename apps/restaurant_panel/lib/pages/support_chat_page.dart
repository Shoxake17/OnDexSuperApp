import 'dart:async';

import 'package:flutter/material.dart';
import 'package:ondex_support/ondex_support.dart';

import '../api.dart';
import '../support_center.dart';
import '../theme.dart';
import '../widgets/support_actions.dart';

const kRestaurantChatStyle = SupportChatStyle(
  accent: OnDexColors.primary,
  background: Color(0xFFFBF7F2),
  inputBackground: Color(0xFFFBF7F2),
  mineBubble: OnDexColors.primaryTint,
  ink: OnDexColors.ink,
  inkDim: OnDexColors.inkDim,
  border: OnDexColors.cardBorder,
  readTick: OnDexColors.success,
  danger: OnDexColors.danger,
);

const _supportName = 'OnDex qo\'llab-quvvatlash';

/// "Chat markazi" — restoran va OnDex administratori o'rtasidagi yozishma.
///
/// Sahifada faqat suhbatlar ro'yxati va yozishma oynasi — sarlavha va yon
/// ma'lumot bloki yo'q, oyna butun joyni egallaydi (aloqa ma'lumotlari
/// "Yordam markazi" da).
///
/// ┌─ NIMA HAQIQIY ────────────────────────────────────────────────────┐
/// * Xabarlar bazada saqlanadi, jonli kanal orqali ikkala tomonga darhol
///   yetadi; soket uzilsa qayta ulanganda o'tkazib yuborilganlar olinadi.
/// * "Online" — OnDex admin paneli hozir ochiq va ulanganmi (server
///   aytadi, taxmin emas).
/// * Rasm: JPG/PNG/WEBP, 10 MB gacha. Server qayta kodlaydi va ommaviy
///   manzilga emas, bazaga yozadi; rasm faqat token bilan ochiladi.
/// └───────────────────────────────────────────────────────────────────┘
class SupportChatPage extends StatefulWidget {
  const SupportChatPage({super.key, this.center, this.imagePicker, this.imageProvider});

  /// Testlar uchun; berilmasa panelning yagona nusxasi.
  final SupportCenter? center;

  /// Testlar uchun; berilmasa tizimning fayl tanlash oynasi.
  final SupportImagePicker? imagePicker;

  /// Testlar uchun; berilmasa token bilan `NetworkImage`.
  final SupportImageProviderBuilder? imageProvider;

  @override
  State<SupportChatPage> createState() => _SupportChatPageState();
}

class _SupportChatPageState extends State<SupportChatPage> {
  SupportCenter get _center => widget.center ?? supportCenter;

  late final SupportConversation _conv = SupportConversation(
    viewer: kSideRestaurant,
    fetch: ({int? before, int? after, int limit = 50}) =>
        api.supportMessages(before: before, after: after, limit: limit),
    sendMessage: api.sendSupportMessage,
    sendImage: api.sendSupportImage,
    markRead: api.markSupportRead,
  );

  StreamSubscription<Map<String, dynamic>>? _eventSub;
  StreamSubscription<bool>? _connSub;
  Timer? _timer;

  bool get _online => _conv.lastResponse.containsKey('support_online')
      ? _conv.lastResponse['support_online'] == true
      : _center.supportOnline;

  @override
  void initState() {
    super.initState();
    _conv.addListener(_onConversation);
    // Kadr qurilayotgan paytda holat o'zgarmasin (markaz orqali qobiq ham
    // qayta chiziladi) — yuklash qurilishdan keyin boshlanadi.
    scheduleMicrotask(() {
      if (mounted) _conv.load();
    });
    _eventSub = _center.events.listen(_conv.handleEvent);
    _connSub = _center.connection.listen((connected) {
      if (connected) _conv.catchUp();
    });
    // Zaxira: soket "yarim ochiq" qolsa ham yozishma eskirmaydi.
    _timer = Timer.periodic(const Duration(seconds: 60), (_) => _conv.catchUp());
  }

  void _onConversation() {
    _center.applyThread(
      _conv.thread,
      supportOnline: _conv.lastResponse.containsKey('support_online')
          ? _conv.lastResponse['support_online'] == true
          : null,
    );
    if (mounted) setState(() {});
  }

  @override
  void dispose() {
    _eventSub?.cancel();
    _connSub?.cancel();
    _timer?.cancel();
    _conv.removeListener(_onConversation);
    _conv.dispose();
    super.dispose();
  }

  ImageProvider _attachmentImage(SupportAttachment a) => NetworkImage(
        api.supportAttachmentUrl(a.id),
        headers: {if (api.token != null) 'Authorization': 'Bearer ${api.token}'},
      );

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(24, 20, 24, 20),
      child: LayoutBuilder(builder: (context, box) {
        final showList = box.maxWidth >= 1000;
        return Row(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            if (showList) ...[
              SizedBox(width: 300, child: _ConversationList(thread: _conv.thread, online: _online)),
              const SizedBox(width: 16),
            ],
            Expanded(
              child: _Card(
                child: Column(
                  children: [
                    _ChatHeader(online: _online),
                    const Divider(height: 1, color: OnDexColors.cardBorder),
                    Expanded(
                      child: SupportChatView(
                        conversation: _conv,
                        style: kRestaurantChatStyle,
                        peerAvatar: const SupportAvatar(size: 32),
                        onPickImage: widget.imagePicker ?? pickSupportImage,
                        imageProvider: widget.imageProvider ?? _attachmentImage,
                        emptyTitle: 'Savolingiz bormi?',
                        emptySubtitle:
                            'Muammo yoki taklifingizni yozing, kerak bo\'lsa ekran rasmini biriktiring — '
                            'OnDex administratori shu yerda javob beradi.',
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ],
        );
      }),
    );
  }
}

class _Card extends StatelessWidget {
  const _Card({required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) => Container(
        decoration: BoxDecoration(
          color: OnDexColors.cardBg,
          borderRadius: BorderRadius.circular(16),
          border: Border.all(color: OnDexColors.cardBorder),
        ),
        clipBehavior: Clip.antiAlias,
        child: child,
      );
}

class _ChatHeader extends StatelessWidget {
  const _ChatHeader({required this.online});

  final bool online;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(18, 14, 18, 14),
      child: Row(
        children: [
          const SupportAvatar(size: 44),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                const Text(_supportName,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(fontSize: 16, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
                const SizedBox(height: 3),
                SupportOnlineLabel(online: online),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _ConversationList extends StatelessWidget {
  const _ConversationList({required this.thread, required this.online});

  final SupportThread? thread;
  final bool online;

  @override
  Widget build(BuildContext context) {
    final t = thread;
    final unread = t?.unread ?? 0;
    final preview = (t == null || t.messageCount == 0)
        ? 'Hali yozishma yo\'q'
        : (t.lastSender == kSideRestaurant ? 'Siz: ${t.lastBody}' : t.lastBody);
    return _Card(
      child: ListView(
        padding: const EdgeInsets.fromLTRB(10, 16, 10, 16),
        children: [
          const Padding(
            padding: EdgeInsets.fromLTRB(8, 0, 8, 12),
            child: Text('Suhbatlar',
                style: TextStyle(fontSize: 16, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
          ),
          Container(
            key: const ValueKey('support-thread-tile'),
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: const Color(0xFFFDF1E7),
              borderRadius: BorderRadius.circular(12),
            ),
            child: Row(
              children: [
                Stack(
                  children: [
                    const SupportAvatar(size: 44),
                    Positioned(
                      right: 0,
                      bottom: 0,
                      child: Container(
                        width: 12,
                        height: 12,
                        decoration: BoxDecoration(
                          color: online ? OnDexColors.success : OnDexColors.inkFaint,
                          shape: BoxShape.circle,
                          border: Border.all(color: Colors.white, width: 2),
                        ),
                      ),
                    ),
                  ],
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Row(
                        children: [
                          const Expanded(
                            child: Text(_supportName,
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: TextStyle(fontSize: 14, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
                          ),
                          Text(supportListTime(t?.lastAt, DateTime.now()),
                              style: TextStyle(
                                  fontSize: 11.5,
                                  color: unread > 0 ? OnDexColors.primary : OnDexColors.inkDim)),
                        ],
                      ),
                      const SizedBox(height: 4),
                      Row(
                        children: [
                          if (t != null && t.lastHasImage)
                            const Padding(
                              padding: EdgeInsets.only(right: 4),
                              child: Icon(Icons.image_outlined, size: 14, color: OnDexColors.inkDim),
                            ),
                          Expanded(
                            child: Text(preview,
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: const TextStyle(fontSize: 12.5, color: OnDexColors.inkDim)),
                          ),
                          if (unread > 0)
                            Container(
                              margin: const EdgeInsets.only(left: 6),
                              padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 2),
                              decoration: BoxDecoration(
                                  color: OnDexColors.primary, borderRadius: BorderRadius.circular(999)),
                              child: Text('$unread',
                                  style: const TextStyle(
                                      color: Colors.white, fontSize: 11, fontWeight: FontWeight.w800)),
                            ),
                        ],
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

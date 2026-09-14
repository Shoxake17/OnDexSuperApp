import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import 'conversation.dart';
import 'format.dart';
import 'models.dart';

/// Tez tanlanadigan emoji'lar (tashqi paketsiz — hech narsa yuklanmaydi).
const supportEmojis = <String>[
  '😊', '😂', '🙂', '😉', '😍', '🤔', '😅', '😔', //
  '😢', '😡', '👍', '👎', '👌', '👏', '🙏', '🤝', //
  '❤️', '🔥', '✅', '❌', '❗', '❓', '🎉', '👀', //
  '⏳', '📦', '🧾', '💳', '📞', '🖨️', '🍽️', '🚀', //
];

/// Chat ranglari — panel o'z mavzusini beradi.
class SupportChatStyle {
  const SupportChatStyle({
    this.accent = const Color(0xFFF2650F),
    this.background = const Color(0xFFFBF7F2),
    this.surface = Colors.white,
    this.inputBackground = const Color(0xFFFBF7F2),
    this.mineBubble = const Color(0xFFFDE9DA),
    this.theirBubble = Colors.white,
    this.ink = const Color(0xFF1F1710),
    this.inkDim = const Color(0xFF7A6B5C),
    this.border = const Color(0xFFEAD9C8),
    this.readTick = const Color(0xFF2F9E58),
    this.danger = const Color(0xFFE5484D),
  });

  final Color accent;
  final Color background;
  final Color surface;
  final Color inputBackground;
  final Color mineBubble;
  final Color theirBubble;
  final Color ink;
  final Color inkDim;
  final Color border;
  final Color readTick;
  final Color danger;
}

typedef SupportImagePicker = Future<SupportImageDraft?> Function();
typedef SupportImageProviderBuilder = ImageProvider Function(SupportAttachment attachment);

/// Rasmni to'liq ekranda ko'rsatadi (kattalashtirish va surish bilan).
Future<void> showSupportImageViewer(BuildContext context, ImageProvider provider) {
  return showDialog<void>(
    context: context,
    barrierColor: Colors.black87,
    builder: (ctx) => Dialog(
      backgroundColor: Colors.transparent,
      elevation: 0,
      insetPadding: const EdgeInsets.all(24),
      child: SizedBox.expand(
        child: Stack(
          children: [
            Positioned.fill(
              child: InteractiveViewer(
                key: const ValueKey('support-image-viewer'),
                maxScale: 5,
                child: Center(
                  child: Image(
                    image: provider,
                    fit: BoxFit.contain,
                    errorBuilder: (_, __, ___) =>
                        const Icon(Icons.broken_image_outlined, color: Colors.white70, size: 48),
                  ),
                ),
              ),
            ),
            Positioned(
              top: 0,
              right: 0,
              child: IconButton(
                tooltip: 'Yopish',
                onPressed: () => Navigator.of(ctx).pop(),
                icon: const Icon(Icons.close_rounded, color: Colors.white, size: 28),
              ),
            ),
          ],
        ),
      ),
    ),
  );
}

/// Xabarlar ro'yxati + yozish maydoni. Sarlavha sahifaning o'zida.
class SupportChatView extends StatefulWidget {
  const SupportChatView({
    super.key,
    required this.conversation,
    this.style = const SupportChatStyle(),
    this.peerAvatar,
    this.onPickImage,
    this.imageProvider,
    this.emptyTitle = 'Suhbatni boshlang',
    this.emptySubtitle = 'Savolingizni yozing — javob shu yerda ko\'rinadi.',
    this.inputHint = 'Xabar yozing...',
  });

  final SupportConversation conversation;
  final SupportChatStyle style;
  final Widget? peerAvatar;

  /// Rasm tanlash (`pickSupportImage`). `null` — biriktirish tugmasi yo'q.
  final SupportImagePicker? onPickImage;

  /// Serverdagi rasmni ko'rsatish (token bilan `NetworkImage`).
  final SupportImageProviderBuilder? imageProvider;
  final String emptyTitle;
  final String emptySubtitle;
  final String inputHint;

  @override
  State<SupportChatView> createState() => _SupportChatViewState();
}

class _SupportChatViewState extends State<SupportChatView> {
  final _input = TextEditingController();
  final _scroll = ScrollController();
  late final FocusNode _focus = FocusNode(onKeyEvent: _onKey);
  SupportImageDraft? _draft;
  bool _picking = false;

  @override
  void initState() {
    super.initState();
    widget.conversation.addListener(_changed);
    _input.addListener(_changed);
    _scroll.addListener(_onScroll);
  }

  @override
  void didUpdateWidget(covariant SupportChatView old) {
    super.didUpdateWidget(old);
    if (!identical(old.conversation, widget.conversation)) {
      old.conversation.removeListener(_changed);
      widget.conversation.addListener(_changed);
    }
  }

  @override
  void dispose() {
    widget.conversation.removeListener(_changed);
    _input.dispose();
    _scroll.dispose();
    _focus.dispose();
    super.dispose();
  }

  void _changed() {
    if (mounted) setState(() {});
  }

  void _onScroll() {
    if (!_scroll.hasClients) return;
    final p = _scroll.position;
    // Ro'yxat teskari: yuqori (eski xabarlar) — `maxScrollExtent` tomonda.
    if (p.pixels >= p.maxScrollExtent - 240) widget.conversation.loadOlder();
  }

  int get _length => _input.text.trim().runes.length;
  bool get _canSend => (_length > 0 || _draft != null) && _length <= kSupportMaxBody;
  bool get _canAttach => widget.onPickImage != null && widget.conversation.canSendImages;

  /// Enter — yuborish, Shift+Enter — yangi qator.
  KeyEventResult _onKey(FocusNode node, KeyEvent e) {
    final enter = e.logicalKey == LogicalKeyboardKey.enter || e.logicalKey == LogicalKeyboardKey.numpadEnter;
    if (e is KeyDownEvent && enter && !HardwareKeyboard.instance.isShiftPressed) {
      _send();
      return KeyEventResult.handled;
    }
    return KeyEventResult.ignored;
  }

  Future<void> _send() async {
    if (!_canSend) return;
    final text = _input.text;
    final draft = _draft;
    _input.clear();
    setState(() => _draft = null);
    _focus.requestFocus();
    if (_scroll.hasClients) _scroll.jumpTo(0);
    await widget.conversation.send(text, image: draft);
  }

  void _toast(String message) =>
      ScaffoldMessenger.maybeOf(context)?.showSnackBar(SnackBar(content: Text(message)));

  Future<void> _pickImage() async {
    final pick = widget.onPickImage;
    if (pick == null || _picking) return;
    setState(() => _picking = true);
    try {
      final d = await pick();
      if (!mounted || d == null) return;
      final problem = validateSupportImage(d);
      if (problem != null) {
        _toast(problem);
        return;
      }
      setState(() => _draft = d);
      _focus.requestFocus();
    } on SupportImageException catch (e) {
      if (mounted) _toast(e.message);
    } catch (_) {
      if (mounted) _toast('Rasmni ochib bo\'lmadi');
    } finally {
      if (mounted) setState(() => _picking = false);
    }
  }

  void _insertEmoji(String emoji) {
    final v = _input.value;
    final sel = v.selection;
    final start = sel.isValid ? sel.start : v.text.length;
    final end = sel.isValid ? sel.end : v.text.length;
    _input.value = TextEditingValue(
      text: v.text.replaceRange(start, end, emoji),
      selection: TextSelection.collapsed(offset: start + emoji.length),
    );
    _focus.requestFocus();
  }

  @override
  Widget build(BuildContext context) {
    final s = widget.style;
    return ColoredBox(
      color: s.background,
      child: Column(
        children: [
          Expanded(child: _body(s)),
          _composer(s),
        ],
      ),
    );
  }

  Widget _body(SupportChatStyle s) {
    final c = widget.conversation;
    final msgs = c.messages;
    if (msgs.isEmpty && (c.loading || (c.thread == null && c.error == null))) {
      return const Center(child: CircularProgressIndicator());
    }
    if (msgs.isEmpty && c.error != null) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.cloud_off_rounded, size: 36, color: s.inkDim),
              const SizedBox(height: 10),
              Text(c.error!, textAlign: TextAlign.center, style: TextStyle(color: s.inkDim)),
              const SizedBox(height: 12),
              OutlinedButton(onPressed: c.load, child: const Text('Qayta urinish')),
            ],
          ),
        ),
      );
    }
    if (msgs.isEmpty) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 64,
                height: 64,
                decoration: BoxDecoration(color: s.mineBubble, shape: BoxShape.circle),
                child: Icon(Icons.forum_rounded, color: s.accent, size: 30),
              ),
              const SizedBox(height: 14),
              Text(widget.emptyTitle,
                  textAlign: TextAlign.center,
                  style: TextStyle(fontSize: 16, fontWeight: FontWeight.w700, color: s.ink)),
              const SizedBox(height: 6),
              Text(widget.emptySubtitle, textAlign: TextAlign.center, style: TextStyle(color: s.inkDim, height: 1.4)),
            ],
          ),
        ),
      );
    }

    final entries = _entries(msgs);
    final now = DateTime.now();
    final peerRead = c.thread?.peerReadSeq ?? 0;
    return ListView.builder(
      key: const ValueKey('support-messages'),
      controller: _scroll,
      reverse: true,
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 16),
      itemCount: entries.length + (c.hasOlder ? 1 : 0),
      itemBuilder: (context, i) {
        if (i == entries.length) {
          return Padding(
            padding: const EdgeInsets.all(8),
            child: Center(
              child: c.loadingOlder
                  ? const SizedBox(width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2))
                  : TextButton(onPressed: c.loadOlder, child: const Text('Oldingi xabarlar')),
            ),
          );
        }
        final e = entries[i];
        final day = e.day;
        if (day != null) return _DayChip(label: supportDayLabel(day, now), style: s);
        final m = e.message!;
        return _Bubble(
          message: m,
          mine: m.sender == c.viewer,
          read: m.confirmed && m.seq <= peerRead,
          firstInGroup: e.firstInGroup,
          avatar: widget.peerAvatar,
          imageProvider: widget.imageProvider,
          style: s,
          onRetry: () => c.retry(m),
          onDiscard: () => c.discard(m),
        );
      },
    );
  }

  Widget _composer(SupportChatStyle s) {
    final length = _length;
    final tooLong = length > kSupportMaxBody;
    final draft = _draft;
    return Container(
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 14),
      decoration: BoxDecoration(color: s.surface, border: Border(top: BorderSide(color: s.border))),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          if (draft != null)
            _DraftPreview(draft: draft, style: s, onRemove: () => setState(() => _draft = null)),
          Row(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              if (_canAttach)
                Padding(
                  padding: const EdgeInsets.only(right: 6, bottom: 4),
                  child: IconButton(
                    key: const ValueKey('support-attach'),
                    tooltip: 'Rasm biriktirish',
                    onPressed: _picking ? null : _pickImage,
                    icon: Icon(Icons.image_outlined, color: s.inkDim),
                  ),
                ),
              Expanded(
                child: Container(
                  padding: const EdgeInsets.only(left: 18, right: 4),
                  decoration: BoxDecoration(
                    color: s.inputBackground,
                    borderRadius: BorderRadius.circular(24),
                    border: Border.all(color: tooLong ? s.danger : s.border),
                  ),
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.end,
                    children: [
                      Expanded(
                        child: TextField(
                          key: const ValueKey('support-input'),
                          controller: _input,
                          focusNode: _focus,
                          minLines: 1,
                          maxLines: 5,
                          keyboardType: TextInputType.multiline,
                          textInputAction: TextInputAction.newline,
                          style: TextStyle(fontSize: 14, color: s.ink, height: 1.35),
                          decoration: InputDecoration(
                            hintText: draft != null ? 'Izoh qo\'shing (ixtiyoriy)...' : widget.inputHint,
                            hintStyle: TextStyle(color: s.inkDim),
                            border: InputBorder.none,
                            isDense: true,
                            contentPadding: const EdgeInsets.symmetric(vertical: 14),
                          ),
                        ),
                      ),
                      _EmojiButton(onPick: _insertEmoji, style: s),
                    ],
                  ),
                ),
              ),
              const SizedBox(width: 10),
              Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  if (length > kSupportCounterFrom)
                    Padding(
                      padding: const EdgeInsets.only(bottom: 4),
                      child: Text('$length/$kSupportMaxBody',
                          key: const ValueKey('support-counter'),
                          style: TextStyle(fontSize: 11, color: tooLong ? s.danger : s.inkDim)),
                    ),
                  Tooltip(
                    message: 'Yuborish (Enter)',
                    child: Material(
                      color: _canSend ? s.accent : s.accent.withAlpha(90),
                      shape: const CircleBorder(),
                      child: InkWell(
                        key: const ValueKey('support-send'),
                        customBorder: const CircleBorder(),
                        onTap: _canSend ? _send : null,
                        child: const SizedBox(
                          width: 48,
                          height: 48,
                          child: Icon(Icons.send_rounded, color: Colors.white, size: 22),
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _DraftPreview extends StatelessWidget {
  const _DraftPreview({required this.draft, required this.style, required this.onRemove});

  final SupportImageDraft draft;
  final SupportChatStyle style;
  final VoidCallback onRemove;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 10),
      child: Align(
        alignment: Alignment.centerLeft,
        child: Container(
          key: const ValueKey('support-draft'),
          padding: const EdgeInsets.fromLTRB(6, 6, 2, 6),
          decoration: BoxDecoration(
            color: style.inputBackground,
            borderRadius: BorderRadius.circular(12),
            border: Border.all(color: style.border),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              ClipRRect(
                borderRadius: BorderRadius.circular(8),
                child: Image.memory(
                  draft.bytes,
                  width: 56,
                  height: 56,
                  fit: BoxFit.cover,
                  gaplessPlayback: true,
                  errorBuilder: (_, __, ___) => SizedBox(
                    width: 56,
                    height: 56,
                    child: Icon(Icons.broken_image_outlined, color: style.inkDim),
                  ),
                ),
              ),
              const SizedBox(width: 10),
              Flexible(
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 240),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(draft.filename,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: style.ink)),
                      const SizedBox(height: 2),
                      Text(supportBytes(draft.bytes.length), style: TextStyle(fontSize: 12, color: style.inkDim)),
                    ],
                  ),
                ),
              ),
              IconButton(
                key: const ValueKey('support-draft-remove'),
                tooltip: 'Olib tashlash',
                onPressed: onRemove,
                icon: Icon(Icons.close_rounded, size: 18, color: style.inkDim),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _Entry {
  const _Entry.day(DateTime this.day)
      : message = null,
        firstInGroup = false;
  const _Entry.message(SupportMessage this.message, {required this.firstInGroup}) : day = null;

  final DateTime? day;
  final SupportMessage? message;
  final bool firstInGroup;
}

/// Eng yangisidan boshlab (ro'yxat teskari): kun ajratgichlari va guruh
/// boshlari bilan.
List<_Entry> _entries(List<SupportMessage> msgs) {
  final out = <_Entry>[];
  DateTime? day;
  String? prevSender;
  for (final m in msgs) {
    final d = DateTime(m.createdAt.year, m.createdAt.month, m.createdAt.day);
    if (day == null || d != day) {
      out.add(_Entry.day(d));
      day = d;
      prevSender = null;
    }
    out.add(_Entry.message(m, firstInGroup: prevSender != m.sender));
    prevSender = m.sender;
  }
  return out.reversed.toList();
}

class _DayChip extends StatelessWidget {
  const _DayChip({required this.label, required this.style});

  final String label;
  final SupportChatStyle style;

  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.symmetric(vertical: 12),
        child: Center(
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 5),
            decoration: BoxDecoration(
              color: style.surface,
              borderRadius: BorderRadius.circular(999),
              border: Border.all(color: style.border),
            ),
            child: Text(label, style: TextStyle(fontSize: 12, color: style.inkDim, fontWeight: FontWeight.w500)),
          ),
        ),
      );
}

/// Xabardagi rasm: server bergan nisbatda joy oldindan ajratiladi (rasm
/// yuklanganda ro'yxat "sakramaydi"), bosilsa to'liq ekranda ochiladi.
class _BubbleImage extends StatelessWidget {
  const _BubbleImage({required this.message, required this.maxWidth, required this.style, this.imageProvider});

  final SupportMessage message;
  final double maxWidth;
  final SupportChatStyle style;
  final SupportImageProviderBuilder? imageProvider;

  @override
  Widget build(BuildContext context) {
    final a = message.attachment;
    final local = message.localImage;
    ImageProvider? provider;
    if (local != null) {
      provider = MemoryImage(local);
    } else if (a != null && imageProvider != null) {
      provider = imageProvider!(a);
    }
    final ratio = a?.aspectRatio ?? 4 / 3;
    var w = math.min(maxWidth, 280.0).clamp(80.0, 280.0).toDouble();
    var h = w / ratio;
    if (h > 320) {
      h = 320;
      w = math.max(60, h * ratio);
    }
    if (h < 60) h = 60;
    final placeholder = Container(
      color: style.border.withAlpha(90),
      alignment: Alignment.center,
      child: Icon(Icons.image_outlined, color: style.inkDim),
    );
    final key = ValueKey('support-image-${message.confirmed ? message.id : message.clientId}');
    final p = provider;
    return Align(
      alignment: Alignment.centerLeft,
      child: Semantics(
        image: true,
        label: 'Rasm',
        child: MouseRegion(
          cursor: p == null ? MouseCursor.defer : SystemMouseCursors.click,
          child: GestureDetector(
            key: key,
            onTap: p == null ? null : () => showSupportImageViewer(context, p),
            child: ClipRRect(
              borderRadius: BorderRadius.circular(10),
              child: SizedBox(
                width: w,
                height: h,
                child: p == null
                    ? placeholder
                    : Image(
                        image: p,
                        fit: BoxFit.cover,
                        gaplessPlayback: true,
                        frameBuilder: (context, child, frame, sync) => sync || frame != null ? child : placeholder,
                        errorBuilder: (_, __, ___) => Container(
                          color: style.border.withAlpha(90),
                          alignment: Alignment.center,
                          child: Icon(Icons.broken_image_outlined, color: style.inkDim),
                        ),
                      ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _Bubble extends StatelessWidget {
  const _Bubble({
    required this.message,
    required this.mine,
    required this.read,
    required this.firstInGroup,
    required this.style,
    required this.onRetry,
    required this.onDiscard,
    this.avatar,
    this.imageProvider,
  });

  final SupportMessage message;
  final bool mine;
  final bool read;
  final bool firstInGroup;
  final Widget? avatar;
  final SupportImageProviderBuilder? imageProvider;
  final SupportChatStyle style;
  final VoidCallback onRetry;
  final VoidCallback onDiscard;

  Widget _status() {
    if (message.failed) return Icon(Icons.error_outline_rounded, size: 14, color: style.danger);
    if (!message.confirmed) return Icon(Icons.schedule_rounded, size: 13, color: style.inkDim);
    if (read) {
      return Icon(Icons.done_all_rounded,
          key: ValueKey('support-read-${message.id}'), size: 15, color: style.readTick);
    }
    return Icon(Icons.done_rounded, key: ValueKey('support-sent-${message.id}'), size: 15, color: style.inkDim);
  }

  @override
  Widget build(BuildContext context) {
    final key = message.confirmed ? 'support-msg-${message.id}' : 'support-out-${message.clientId}';
    return Padding(
      padding: EdgeInsets.only(top: firstInGroup ? 10 : 3),
      child: LayoutBuilder(builder: (context, box) {
        final maxWidth = (box.maxWidth * (mine ? 0.78 : 0.72)).clamp(120.0, 560.0).toDouble();
        final hasImage = message.hasImage;
        final bubble = Container(
          key: ValueKey(key),
          constraints: BoxConstraints(maxWidth: maxWidth),
          padding: hasImage ? const EdgeInsets.fromLTRB(6, 6, 10, 7) : const EdgeInsets.fromLTRB(14, 10, 12, 7),
          decoration: BoxDecoration(
            color: mine ? style.mineBubble : style.theirBubble,
            borderRadius: BorderRadius.only(
              topLeft: Radius.circular(!mine && firstInGroup ? 4 : 16),
              topRight: Radius.circular(mine && firstInGroup ? 4 : 16),
              bottomLeft: const Radius.circular(16),
              bottomRight: const Radius.circular(16),
            ),
            border: mine ? null : Border.all(color: style.border),
            boxShadow: const [BoxShadow(color: Color(0x0D000000), blurRadius: 6, offset: Offset(0, 2))],
          ),
          child: IntrinsicWidth(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              mainAxisSize: MainAxisSize.min,
              children: [
                if (hasImage)
                  _BubbleImage(
                    message: message,
                    maxWidth: maxWidth - 16,
                    style: style,
                    imageProvider: imageProvider,
                  ),
                if (message.body.isNotEmpty)
                  Padding(
                    padding: hasImage ? const EdgeInsets.fromLTRB(8, 8, 2, 0) : EdgeInsets.zero,
                    child: SelectableText(message.body, style: TextStyle(fontSize: 14, height: 1.4, color: style.ink)),
                  ),
                const SizedBox(height: 3),
                Row(
                  mainAxisAlignment: MainAxisAlignment.end,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(supportClock(message.createdAt), style: TextStyle(fontSize: 11, color: style.inkDim)),
                    if (mine) ...[const SizedBox(width: 4), _status()],
                  ],
                ),
              ],
            ),
          ),
        );
        final column = Column(
          crossAxisAlignment: mine ? CrossAxisAlignment.end : CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            bubble,
            if (message.failed)
              Padding(
                padding: const EdgeInsets.only(top: 4),
                child: Wrap(
                  crossAxisAlignment: WrapCrossAlignment.center,
                  alignment: WrapAlignment.end,
                  spacing: 4,
                  children: [
                    Text(message.error ?? 'Yuborilmadi', style: TextStyle(fontSize: 12, color: style.danger)),
                    TextButton(onPressed: onRetry, child: const Text('Qayta yuborish')),
                    TextButton(onPressed: onDiscard, child: Text('O\'chirish', style: TextStyle(color: style.inkDim))),
                  ],
                ),
              ),
          ],
        );
        if (mine) {
          return Row(mainAxisAlignment: MainAxisAlignment.end, children: [Flexible(child: column)]);
        }
        return Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            SizedBox(width: 34, child: firstInGroup ? avatar : null),
            const SizedBox(width: 8),
            Flexible(child: column),
          ],
        );
      }),
    );
  }
}

class _EmojiButton extends StatefulWidget {
  const _EmojiButton({required this.onPick, required this.style});

  final ValueChanged<String> onPick;
  final SupportChatStyle style;

  @override
  State<_EmojiButton> createState() => _EmojiButtonState();
}

class _EmojiButtonState extends State<_EmojiButton> {
  final _menu = MenuController();

  @override
  Widget build(BuildContext context) {
    return MenuAnchor(
      controller: _menu,
      alignmentOffset: const Offset(-250, 6),
      menuChildren: [
        SizedBox(
          width: 300,
          child: Padding(
            padding: const EdgeInsets.all(8),
            child: Wrap(
              spacing: 2,
              runSpacing: 2,
              children: [
                for (final e in supportEmojis)
                  InkWell(
                    key: ValueKey('support-emoji-$e'),
                    borderRadius: BorderRadius.circular(8),
                    onTap: () {
                      widget.onPick(e);
                      _menu.close();
                    },
                    child: SizedBox(
                      width: 34,
                      height: 34,
                      child: Center(child: Text(e, style: const TextStyle(fontSize: 20))),
                    ),
                  ),
              ],
            ),
          ),
        ),
      ],
      builder: (context, controller, _) => IconButton(
        key: const ValueKey('support-emoji'),
        tooltip: 'Emoji',
        onPressed: () => controller.isOpen ? controller.close() : controller.open(),
        icon: Icon(Icons.sentiment_satisfied_alt_outlined, color: widget.style.inkDim),
      ),
    );
  }
}

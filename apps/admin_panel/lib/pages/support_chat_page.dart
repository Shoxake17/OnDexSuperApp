import 'dart:async';

import 'package:flutter/material.dart';
import 'package:ondex_support/ondex_support.dart';

import '../api.dart';
import '../support_inbox.dart';

/// Chat — restoranlardan kelgan murojaatlar va ularga javob.
///
/// Javob restoran panelidagi "Chat markazi" ga jonli kanal orqali darhol
/// yetadi. Har suhbat alohida `SupportConversation` — admin kanalidagi
/// hodisalar `restaurant_id` bo'yicha ajratiladi, begona restoran xabari
/// ochiq suhbatga tushmaydi.
class SupportChatPage extends StatefulWidget {
  const SupportChatPage({super.key, this.restaurant, this.inbox, this.imagePicker, this.imageProvider});

  /// Berilsa — restoran moduli ichidagi ko'rinish: suhbatlar ro'yxati va
  /// "Yangi suhbat" YO'Q, faqat shu restoran bilan yozishma ochiladi.
  /// Kerakli kalitlar: `id`, `name`, `address`, `logo_url` (`GET /restaurants`).
  final Map<String, dynamic>? restaurant;

  /// Testlar uchun; berilmasa panelning yagona nusxasi.
  final AdminSupportInbox? inbox;

  /// Testlar uchun; berilmasa tizimning fayl tanlash oynasi.
  final SupportImagePicker? imagePicker;

  /// Testlar uchun; berilmasa admin tokeni bilan `NetworkImage`.
  final SupportImageProviderBuilder? imageProvider;

  @override
  State<SupportChatPage> createState() => _SupportChatPageState();
}

class AdminThread {
  const AdminThread({
    required this.thread,
    required this.restaurantId,
    required this.name,
    this.address = '',
    this.logoUrl = '',
    this.online = false,
  });

  final SupportThread thread;
  final String restaurantId;
  final String name;
  final String address;
  final String logoUrl;
  final bool online;

  static AdminThread? tryParse(Object? json) {
    if (json is! Map) return null;
    final t = SupportThread.tryParse(json);
    final r = json['restaurant'];
    if (t == null || r is! Map) return null;
    final id = r['id'];
    if (id is! String || id.isEmpty) return null;
    String str(Object? v) => v is String ? v : '';
    return AdminThread(
      thread: t,
      restaurantId: id,
      name: str(r['name']).isEmpty ? id : str(r['name']),
      address: str(r['address']),
      logoUrl: str(r['logo_url']),
      online: json['restaurant_online'] == true,
    );
  }

  AdminThread copyWith({SupportThread? thread, bool? online}) => AdminThread(
        thread: thread ?? this.thread,
        restaurantId: restaurantId,
        name: name,
        address: address,
        logoUrl: logoUrl,
        online: online ?? this.online,
      );
}

SupportChatStyle _chatStyle(ColorScheme s) => SupportChatStyle(
      accent: s.primary,
      background: const Color(0xFFF6F8F7),
      surface: s.surface,
      inputBackground: const Color(0xFFF6F8F7),
      mineBubble: s.primaryContainer,
      theirBubble: s.surface,
      ink: s.onSurface,
      inkDim: s.onSurfaceVariant,
      border: s.outlineVariant,
      readTick: s.primary,
      danger: s.error,
    );

class _SupportChatPageState extends State<SupportChatPage> {
  AdminSupportInbox get _inbox => widget.inbox ?? supportInbox;

  List<AdminThread> _threads = [];
  bool _loading = true;
  String? _error;
  String _query = '';
  bool _unreadOnly = false;

  AdminThread? _selected;
  SupportConversation? _conv;

  StreamSubscription<Map<String, dynamic>>? _eventSub;
  StreamSubscription<bool>? _connSub;
  Timer? _timer;

  /// Restoran modulida bitta suhbat: ro'yxat yuklanmaydi.
  bool get _scoped => _initialThread != null;

  AdminThread? get _initialThread {
    final r = widget.restaurant;
    final id = r?['id'];
    if (r == null || id is! String || id.isEmpty) return null;
    String str(Object? v) => v is String ? v : '';
    return AdminThread(
      thread: SupportThread(restaurantId: id),
      restaurantId: id,
      name: str(r['name']).isEmpty ? id : str(r['name']),
      address: str(r['address']),
      logoUrl: str(r['logo_url']),
    );
  }

  @override
  void initState() {
    super.initState();
    final first = _initialThread;
    if (first != null) {
      _selected = first;
      _conv = _makeConversation(first.restaurantId);
      _loading = false;
      scheduleMicrotask(() {
        if (mounted) _conv?.load();
      });
    } else {
      scheduleMicrotask(() {
        if (mounted) _loadThreads();
      });
    }
    _eventSub = _inbox.events.listen(_onEvent);
    _connSub = _inbox.connection.listen((connected) {
      if (!connected) return;
      if (!_scoped) _loadThreads();
      _conv?.catchUp();
    });
    _timer = Timer.periodic(const Duration(seconds: 60), (_) {
      if (!_scoped) _loadThreads();
      _conv?.catchUp();
    });
  }

  @override
  void dispose() {
    _eventSub?.cancel();
    _connSub?.cancel();
    _timer?.cancel();
    _conv?.removeListener(_onConversation);
    _conv?.dispose();
    super.dispose();
  }

  Future<void> _loadThreads() async {
    try {
      final res = await api.supportThreads();
      if (!mounted) return;
      final raw = res['items'];
      final list = raw is List ? raw.map(AdminThread.tryParse).whereType<AdminThread>().toList() : <AdminThread>[];
      // Hali yozishmasi yo'q, lekin admin tanlagan restoran ro'yxatda qoladi.
      final sel = _selected;
      if (sel != null && !list.any((t) => t.restaurantId == sel.restaurantId)) list.insert(0, sel);
      setState(() {
        _threads = list;
        _loading = false;
        _error = null;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = 'Suhbatlarni yuklab bo\'lmadi: $e';
      });
    }
  }

  void _onEvent(Map<String, dynamic> e) {
    _conv?.handleEvent(e);
    // Restoran modulida ro'yxat yo'q — begona restoran hodisasi bilan
    // ishimiz yo'q (ochiq suhbatga tushmaydi, yuqorida `handleEvent` filtri).
    if (_scoped) return;
    final rid = e['restaurant_id'];
    final t = SupportThread.tryParse(e['thread']);
    if (rid is! String || t == null) return;
    final i = _threads.indexWhere((x) => x.restaurantId == rid);
    if (i < 0) {
      _loadThreads();
      return;
    }
    setState(() {
      final updated = _threads[i].copyWith(thread: _threads[i].thread.merge(t));
      _threads.removeAt(i);
      if (e['type'] == 'support_message') {
        _threads.insert(0, updated);
      } else {
        _threads.insert(i, updated);
      }
    });
  }

  SupportConversation _makeConversation(String rid) => SupportConversation(
        viewer: kSideAdmin,
        restaurantId: rid,
        fetch: ({int? before, int? after, int limit = 50}) =>
            api.supportMessages(rid, before: before, after: after, limit: limit),
        sendMessage: (body, clientId) => api.sendSupportMessage(rid, body, clientId),
        sendImage: (body, clientId, bytes, filename) => api.sendSupportImage(rid, body, clientId, bytes, filename),
        markRead: (upTo) => api.markSupportRead(rid, upTo),
      )..addListener(_onConversation);

  void _select(AdminThread t) {
    if (_selected?.restaurantId == t.restaurantId && _conv != null) return;
    _conv?.removeListener(_onConversation);
    _conv?.dispose();
    final conv = _makeConversation(t.restaurantId);
    setState(() {
      _selected = t;
      _conv = conv;
    });
    conv.load();
  }

  void _closeChat() {
    _conv?.removeListener(_onConversation);
    _conv?.dispose();
    setState(() {
      _conv = null;
      _selected = null;
    });
  }

  void _onConversation() {
    final c = _conv;
    final sel = _selected;
    if (c == null || sel == null || !mounted) return;
    final t = c.thread;
    final online = c.lastResponse['restaurant_online'];
    final i = _threads.indexWhere((x) => x.restaurantId == sel.restaurantId);
    setState(() {
      if (i >= 0 && t != null) {
        _threads[i] = _threads[i].copyWith(thread: _threads[i].thread.merge(t), online: online is bool ? online : null);
      }
      if (online is bool && online != sel.online) _selected = sel.copyWith(online: online);
    });
  }

  Future<void> _newConversation() async {
    final picked = await showDialog<AdminThread>(context: context, builder: (_) => const _RestaurantPicker());
    if (picked == null || !mounted) return;
    final existing = _threads.where((t) => t.restaurantId == picked.restaurantId).toList();
    if (existing.isEmpty) setState(() => _threads.insert(0, picked));
    _select(existing.isEmpty ? picked : existing.first);
  }

  /// Restoran moduli ichidagi Chat: faqat shu restoran bilan yozishma
  /// (ro'yxat va "Yangi suhbat" yo'q).
  Widget _buildScoped(ThemeData theme) {
    final sel = _selected;
    final conv = _conv;
    final unread = conv?.thread?.unread ?? 0;
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text('Chat', style: theme.textTheme.headlineMedium),
              const SizedBox(width: 12),
              if (unread > 0)
                Chip(
                  key: const ValueKey('admin-support-unread'),
                  label: Text('$unread ta o\'qilmagan'),
                  backgroundColor: theme.colorScheme.errorContainer,
                ),
              const Spacer(),
              IconButton(
                tooltip: 'Yangilash',
                onPressed: () => conv?.catchUp(),
                icon: const Icon(Icons.refresh),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Text(
            'Shu restoran bilan yozishma. Javobingiz restoran panelidagi "Chat markazi" ga darhol yetadi.',
            style: theme.textTheme.bodySmall,
          ),
          const SizedBox(height: 16),
          Expanded(
            child: LayoutBuilder(builder: (context, box) {
              if (sel == null || conv == null) return _emptyPane(theme);
              final showInfo = box.maxWidth >= 900;
              return Row(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Expanded(child: _chatPane(theme, sel, conv)),
                  if (showInfo) ...[
                    const SizedBox(width: 16),
                    SizedBox(width: 300, child: _infoPane(theme, sel, conv)),
                  ],
                ],
              );
            }),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    if (_scoped) return _buildScoped(theme);
    final unreadTotal = _threads.fold<int>(0, (n, t) => n + t.thread.unread);
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Tor oynada tugmalar sarlavha ostiga tushadi (chetdan chiqmaydi).
          SizedBox(
            width: double.infinity,
            child: Wrap(
              alignment: WrapAlignment.spaceBetween,
              crossAxisAlignment: WrapCrossAlignment.center,
              spacing: 12,
              runSpacing: 8,
              children: [
                Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text('Chat', style: theme.textTheme.headlineMedium),
                    const SizedBox(width: 12),
                    if (unreadTotal > 0)
                      Chip(
                        key: const ValueKey('admin-support-unread'),
                        label: Text('$unreadTotal ta o\'qilmagan'),
                        backgroundColor: theme.colorScheme.errorContainer,
                      ),
                  ],
                ),
                Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    IconButton(tooltip: 'Yangilash', onPressed: _loadThreads, icon: const Icon(Icons.refresh)),
                    const SizedBox(width: 8),
                    FilledButton.icon(
                      onPressed: _newConversation,
                      icon: const Icon(Icons.add_comment_outlined),
                      label: const Text('Yangi suhbat'),
                    ),
                  ],
                ),
              ],
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Restoranlardan kelgan murojaatlar. Javobingiz restoran panelidagi "Chat markazi" ga darhol yetadi.',
            style: theme.textTheme.bodySmall,
          ),
          const SizedBox(height: 16),
          Expanded(
            child: LayoutBuilder(builder: (context, box) {
              final twoPane = box.maxWidth >= 760;
              final showInfo = box.maxWidth >= 1180;
              final list = _threadList(theme);
              final sel = _selected;
              final conv = _conv;
              if (!twoPane) {
                return (sel == null || conv == null) ? list : _chatPane(theme, sel, conv, onBack: _closeChat);
              }
              return Row(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  SizedBox(width: 340, child: list),
                  const SizedBox(width: 16),
                  Expanded(
                    child: (sel == null || conv == null) ? _emptyPane(theme) : _chatPane(theme, sel, conv),
                  ),
                  if (showInfo && sel != null && conv != null) ...[
                    const SizedBox(width: 16),
                    SizedBox(width: 300, child: _infoPane(theme, sel, conv)),
                  ],
                ],
              );
            }),
          ),
        ],
      ),
    );
  }

  Widget _card(ThemeData theme, Widget child) => Card(
        margin: EdgeInsets.zero,
        elevation: 0,
        clipBehavior: Clip.antiAlias,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(16),
          side: BorderSide(color: theme.colorScheme.outlineVariant),
        ),
        child: child,
      );

  Widget _threadList(ThemeData theme) {
    final q = _query.trim().toLowerCase();
    final visible = _threads
        .where((t) =>
            (!_unreadOnly || t.thread.unread > 0) &&
            (q.isEmpty || t.name.toLowerCase().contains(q) || t.address.toLowerCase().contains(q)))
        .toList();
    final unreadThreads = _threads.where((t) => t.thread.unread > 0).length;
    Widget body;
    if (_loading) {
      body = const Center(child: CircularProgressIndicator());
    } else if (_error != null && _threads.isEmpty) {
      body = Center(
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(_error!, textAlign: TextAlign.center),
              const SizedBox(height: 8),
              OutlinedButton(onPressed: _loadThreads, child: const Text('Qayta urinish')),
            ],
          ),
        ),
      );
    } else if (visible.isEmpty) {
      body = Center(
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Text(
            _threads.isEmpty ? 'Hozircha murojaat yo\'q' : 'Mos suhbat topilmadi',
            textAlign: TextAlign.center,
            style: theme.textTheme.bodyMedium?.copyWith(color: theme.colorScheme.onSurfaceVariant),
          ),
        ),
      );
    } else {
      body = ListView.separated(
        padding: const EdgeInsets.symmetric(vertical: 6),
        itemCount: visible.length,
        separatorBuilder: (_, __) => const Divider(height: 1, indent: 72),
        itemBuilder: (context, i) => _ThreadTile(
          item: visible[i],
          selected: _selected?.restaurantId == visible[i].restaurantId,
          onTap: () => _select(visible[i]),
        ),
      );
    }
    return _card(
      theme,
      Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(12, 12, 12, 8),
            child: TextField(
              key: const ValueKey('admin-support-search'),
              onChanged: (v) => setState(() => _query = v),
              decoration: InputDecoration(
                hintText: 'Restoran qidirish...',
                prefixIcon: const Icon(Icons.search),
                isDense: true,
                border: OutlineInputBorder(borderRadius: BorderRadius.circular(12)),
              ),
            ),
          ),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 12),
            child: SegmentedButton<bool>(
              showSelectedIcon: false,
              segments: [
                const ButtonSegment(value: false, label: Text('Barchasi')),
                ButtonSegment(value: true, label: Text('O\'qilmagan ($unreadThreads)')),
              ],
              selected: {_unreadOnly},
              onSelectionChanged: (s) => setState(() => _unreadOnly = s.first),
            ),
          ),
          const SizedBox(height: 8),
          const Divider(height: 1),
          Expanded(child: body),
        ],
      ),
    );
  }

  Widget _emptyPane(ThemeData theme) => _card(
        theme,
        Center(
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.forum_outlined, size: 48, color: theme.colorScheme.primary),
                const SizedBox(height: 12),
                Text('Suhbatni tanlang', style: theme.textTheme.titleMedium),
                const SizedBox(height: 4),
                Text(
                  'Chapdagi ro\'yxatdan restoranni tanlang yoki "Yangi suhbat" bilan birinchi bo\'lib yozing.',
                  textAlign: TextAlign.center,
                  style: theme.textTheme.bodySmall,
                ),
              ],
            ),
          ),
        ),
      );

  Widget _chatPane(ThemeData theme, AdminThread sel, SupportConversation conv, {VoidCallback? onBack}) {
    return _card(
      theme,
      Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(12, 12, 16, 12),
            child: Row(
              children: [
                if (onBack != null)
                  IconButton(tooltip: 'Orqaga', onPressed: onBack, icon: const Icon(Icons.arrow_back)),
                const SizedBox(width: 4),
                _RestaurantAvatar(name: sel.name, logoUrl: sel.logoUrl, online: sel.online, size: 44),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(sel.name, maxLines: 1, overflow: TextOverflow.ellipsis, style: theme.textTheme.titleMedium),
                      const SizedBox(height: 2),
                      _OnlineText(online: sel.online),
                    ],
                  ),
                ),
              ],
            ),
          ),
          const Divider(height: 1),
          Expanded(
            child: SupportChatView(
              key: ValueKey('admin-chat-${sel.restaurantId}'),
              conversation: conv,
              style: _chatStyle(theme.colorScheme),
              emptyTitle: 'Yozishma hali yo\'q',
              emptySubtitle: 'Restoranga birinchi xabarni yozing — u "Chat markazi" da darhol ko\'rinadi.',
              inputHint: 'Javob yozing...',
              onPickImage: widget.imagePicker ?? pickSupportImage,
              imageProvider: widget.imageProvider ??
                  (a) => NetworkImage(
                        api.supportAttachmentUrl(sel.restaurantId, a.id),
                        headers: {if (api.token != null) 'Authorization': 'Bearer ${api.token}'},
                      ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _infoPane(ThemeData theme, AdminThread sel, SupportConversation conv) {
    final t = conv.thread ?? sel.thread;
    final now = DateTime.now();
    Widget stat(IconData icon, String label, String value) => Padding(
          padding: const EdgeInsets.symmetric(vertical: 6),
          child: Row(
            children: [
              Icon(icon, size: 18, color: theme.colorScheme.onSurfaceVariant),
              const SizedBox(width: 10),
              Expanded(child: Text(label, style: theme.textTheme.bodyMedium)),
              Text(value, style: theme.textTheme.titleSmall),
            ],
          ),
        );
    return _card(
      theme,
      ListView(
        padding: const EdgeInsets.all(18),
        children: [
          Row(
            children: [
              _RestaurantAvatar(name: sel.name, logoUrl: sel.logoUrl, online: sel.online, size: 56),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(sel.name, style: theme.textTheme.titleMedium),
                    if (sel.address.isNotEmpty) Text(sel.address, style: theme.textTheme.bodySmall),
                    const SizedBox(height: 6),
                    _OnlineText(online: sel.online),
                  ],
                ),
              ),
            ],
          ),
          const Divider(height: 32),
          Text('Muloqot tarixi', style: theme.textTheme.titleSmall),
          const SizedBox(height: 6),
          stat(Icons.chat_bubble_outline, 'Jami xabarlar', '${t.messageCount} ta'),
          stat(Icons.schedule, 'Oxirgi faollik', supportActivity(t.lastAt, now)),
          stat(Icons.event_available_outlined, 'Birinchi murojaat', t.firstAt == null ? '—' : supportDate(t.firstAt!)),
          const Divider(height: 32),
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: theme.colorScheme.primaryContainer.withAlpha(110),
              borderRadius: BorderRadius.circular(12),
            ),
            child: Text(
              '"Online" — restoran paneli hozir ochiq. Restoran xabaringizni o\'qigach, uning ostida ikki belgi chiqadi.',
              style: theme.textTheme.bodySmall,
            ),
          ),
        ],
      ),
    );
  }
}

class _ThreadTile extends StatelessWidget {
  const _ThreadTile({required this.item, required this.selected, required this.onTap});

  final AdminThread item;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final t = item.thread;
    final preview = t.messageCount == 0
        ? 'Hali yozishma yo\'q'
        : (t.lastSender == kSideAdmin ? 'Siz: ${t.lastBody}' : t.lastBody);
    return Material(
      color: selected ? theme.colorScheme.secondaryContainer : Colors.transparent,
      child: InkWell(
        key: ValueKey('admin-thread-${item.restaurantId}'),
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
          child: Row(
            children: [
              _RestaurantAvatar(name: item.name, logoUrl: item.logoUrl, online: item.online, size: 44),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Expanded(
                          child: Text(item.name,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: theme.textTheme.titleSmall?.copyWith(fontWeight: FontWeight.w700)),
                        ),
                        Text(supportListTime(t.lastAt, DateTime.now()), style: theme.textTheme.labelSmall),
                      ],
                    ),
                    const SizedBox(height: 3),
                    Row(
                      children: [
                        Expanded(
                          child: Text(preview,
                              maxLines: 1, overflow: TextOverflow.ellipsis, style: theme.textTheme.bodySmall),
                        ),
                        if (t.unread > 0)
                          Container(
                            margin: const EdgeInsets.only(left: 6),
                            padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 2),
                            decoration: BoxDecoration(
                              color: theme.colorScheme.error,
                              borderRadius: BorderRadius.circular(999),
                            ),
                            child: Text('${t.unread}',
                                style: TextStyle(
                                    color: theme.colorScheme.onError, fontSize: 11, fontWeight: FontWeight.w800)),
                          ),
                      ],
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _RestaurantAvatar extends StatelessWidget {
  const _RestaurantAvatar({required this.name, required this.logoUrl, required this.online, required this.size});

  final String name;
  final String logoUrl;
  final bool online;
  final double size;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final initial = name.trim().isEmpty ? '?' : name.trim().characters.first.toUpperCase();
    final fallback = Container(
      width: size,
      height: size,
      color: theme.colorScheme.primaryContainer,
      alignment: Alignment.center,
      child: Text(initial,
          style: TextStyle(
              fontSize: size * 0.4, fontWeight: FontWeight.w700, color: theme.colorScheme.onPrimaryContainer)),
    );
    return SizedBox(
      width: size,
      height: size,
      child: Stack(
        children: [
          ClipOval(
            child: logoUrl.isEmpty
                ? fallback
                : Image.network(imageUrl(logoUrl),
                    width: size, height: size, fit: BoxFit.cover, errorBuilder: (_, __, ___) => fallback),
          ),
          Positioned(
            right: 0,
            bottom: 0,
            child: Container(
              width: size * 0.26,
              height: size * 0.26,
              decoration: BoxDecoration(
                color: online ? Colors.green : Colors.grey,
                shape: BoxShape.circle,
                border: Border.all(color: theme.colorScheme.surface, width: 2),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _OnlineText extends StatelessWidget {
  const _OnlineText({required this.online});

  final bool online;

  @override
  Widget build(BuildContext context) => Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.circle, size: 9, color: online ? Colors.green : Colors.grey),
          const SizedBox(width: 5),
          Text(online ? 'Online' : 'Offline',
              style: TextStyle(fontSize: 12, color: online ? Colors.green.shade700 : Colors.grey.shade600)),
        ],
      );
}

/// "Yangi suhbat" — restoranni tanlash.
class _RestaurantPicker extends StatefulWidget {
  const _RestaurantPicker();

  @override
  State<_RestaurantPicker> createState() => _RestaurantPickerState();
}

class _RestaurantPickerState extends State<_RestaurantPicker> {
  List<Map<String, dynamic>> _list = [];
  bool _loading = true;
  String? _error;
  String _query = '';

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final l = await api.restaurants();
      if (!mounted) return;
      setState(() {
        _list = l.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList();
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = 'Restoranlarni yuklab bo\'lmadi: $e';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final q = _query.trim().toLowerCase();
    final visible = _list.where((r) => '${r['name'] ?? ''} ${r['address'] ?? ''}'.toLowerCase().contains(q)).toList();
    return AlertDialog(
      title: const Text('Yangi suhbat'),
      content: SizedBox(
        width: 460,
        height: 420,
        child: Column(
          children: [
            TextField(
              autofocus: true,
              onChanged: (v) => setState(() => _query = v),
              decoration: const InputDecoration(prefixIcon: Icon(Icons.search), hintText: 'Restoran nomi yoki manzili'),
            ),
            const SizedBox(height: 8),
            Expanded(
              child: _loading
                  ? const Center(child: CircularProgressIndicator())
                  : _error != null
                      ? Center(child: Text(_error!))
                      : visible.isEmpty
                          ? const Center(child: Text('Topilmadi'))
                          : ListView.builder(
                              itemCount: visible.length,
                              itemBuilder: (context, i) {
                                final r = visible[i];
                                final id = r['id'] is String ? r['id'] as String : '';
                                final name = r['name'] is String ? r['name'] as String : id;
                                return ListTile(
                                  leading: const Icon(Icons.storefront_outlined),
                                  title: Text(name),
                                  subtitle: Text(r['address'] is String ? r['address'] as String : ''),
                                  enabled: id.isNotEmpty,
                                  onTap: () => Navigator.of(context).pop(AdminThread(
                                    thread: SupportThread(restaurantId: id),
                                    restaurantId: id,
                                    name: name,
                                    address: r['address'] is String ? r['address'] as String : '',
                                    logoUrl: r['logo_url'] is String ? r['logo_url'] as String : '',
                                  )),
                                );
                              },
                            ),
            ),
          ],
        ),
      ),
      actions: [TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Yopish'))],
    );
  }
}

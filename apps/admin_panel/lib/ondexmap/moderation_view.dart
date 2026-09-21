import 'dart:math' as math;
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';

import 'moderation_api.dart';

/// Foydalanuvchilar yuborgan ob'ektlarni MODERATSIYA qilish (OnDexMap bo'limi).
///
/// Oqim: sayt foydalanuvchisi «Ob'ekt qo'shish» ni yuboradi → ob'ekt
/// KARANTINga tushadi (xaritada ko'rinmaydi) → shu ekranda moderator
/// ko'radi, tuzatadi va tasdiqlaydi (yoki rad etadi) → tasdiqlangach
/// OnDexMap xaritasida HAMMAGA ko'rinadi.
///
/// Uch bo'lim:
///   • Kutilmoqda        — karantindagi takliflar (tasdiqlash / rad etish);
///   • Xaritadagi ob'ektlar — tasdiqlanganlar (vandalizm bo'lsa olib tashlash);
///   • Rad etilganlar    — tarix.
///
/// Matnlar (nom, tavsif, ...) foydalanuvchi yozgan: hammasi faqat `Text` /
/// `TextField` orqali chiqadi — HTML sifatida talqin qilinmaydi.
class OndexMapModeration extends StatefulWidget {
  final ModerationApi api;

  /// Kutilayotgan takliflar soni o'zgarganda (bo'lim sarlavhasidagi belgi uchun).
  final ValueChanged<int>? onPending;

  const OndexMapModeration({super.key, required this.api, this.onPending});

  @override
  State<OndexMapModeration> createState() => _OndexMapModerationState();
}

enum _Tab { pending, places, rejected }

class _OndexMapModerationState extends State<OndexMapModeration> {
  PlacesMeta? _meta;
  _Tab _tab = _Tab.pending;
  bool _loading = true;
  ModerationException? _error;
  List<PlaceRecord> _items = const [];
  int _pending = 0;

  /// Eskirgan javob yangisini bosib ketmasin (tez tab almashtirilganda).
  int _epoch = 0;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    final epoch = ++_epoch;
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final meta = _meta ?? await widget.api.meta();
      List<PlaceRecord> items;
      var pending = _pending;
      switch (_tab) {
        case _Tab.pending:
          final r = await widget.api.submissions('pending');
          items = r.items;
          pending = r.pending;
        case _Tab.rejected:
          final r = await widget.api.submissions('rejected');
          items = r.items;
          pending = r.pending;
        case _Tab.places:
          items = await widget.api.places();
      }
      if (!mounted || epoch != _epoch) return;
      setState(() {
        _meta = meta;
        _items = items;
        _pending = pending;
        _loading = false;
      });
      widget.onPending?.call(pending);
    } on ModerationException catch (e) {
      if (!mounted || epoch != _epoch) return;
      setState(() {
        _error = e;
        _loading = false;
      });
    }
  }

  void _setTab(_Tab t) {
    if (t == _tab) return;
    setState(() {
      _tab = t;
      _items = const [];
    });
    _load();
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 4, 16, 8),
          child: Wrap(
            spacing: 12,
            runSpacing: 8,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              SegmentedButton<_Tab>(
                key: const ValueKey('mod-tabs'),
                showSelectedIcon: false,
                segments: [
                  ButtonSegment(
                    value: _Tab.pending,
                    icon: const Icon(Icons.inbox_outlined),
                    label: Text('Kutilmoqda${_pending > 0 ? ' ($_pending)' : ''}'),
                  ),
                  const ButtonSegment(
                    value: _Tab.places,
                    icon: Icon(Icons.place_outlined),
                    label: Text('Xaritadagi ob\'ektlar'),
                  ),
                  const ButtonSegment(
                    value: _Tab.rejected,
                    icon: Icon(Icons.block),
                    label: Text('Rad etilganlar'),
                  ),
                ],
                selected: {_tab},
                onSelectionChanged: (s) => _setTab(s.first),
              ),
              IconButton(
                key: const ValueKey('mod-refresh'),
                tooltip: 'Yangilash',
                icon: const Icon(Icons.refresh),
                onPressed: _loading ? null : _load,
              ),
            ],
          ),
        ),
        Expanded(child: _body()),
      ],
    );
  }

  Widget _body() {
    if (_loading && _items.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }
    final err = _error;
    if (err != null) {
      return OndexMapProblem(error: err, onRetry: _load);
    }
    if (_items.isEmpty) {
      return Center(
        child: Text(
          switch (_tab) {
            _Tab.pending => 'Tekshiruvni kutayotgan ob\'ekt yo\'q',
            _Tab.places => 'Xaritada hali foydalanuvchi ob\'ekti yo\'q',
            _Tab.rejected => 'Rad etilgan ob\'ekt yo\'q',
          },
          key: const ValueKey('mod-empty'),
        ),
      );
    }

    final meta = _meta!;
    return ListView.builder(
      padding: const EdgeInsets.fromLTRB(16, 0, 16, 32),
      itemCount: _items.length,
      itemBuilder: (context, i) {
        final r = _items[i];
        return Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 980),
            child: switch (_tab) {
              // `ValueKey(id)`: ro'yxat o'zgarganda kartochka holati (tahrirlangan
              // matn) boshqa ob'ektga o'tib qolmasin.
              _Tab.pending => _PendingCard(
                  key: ValueKey('mod-card-${r.id}'),
                  record: r,
                  meta: meta,
                  api: widget.api,
                  onChanged: _load,
                ),
              _Tab.places => _PlaceCard(
                  key: ValueKey('mod-place-${r.id}'),
                  record: r,
                  api: widget.api,
                  onChanged: _load,
                ),
              _Tab.rejected => _RejectedCard(
                  key: ValueKey('mod-rej-${r.id}'),
                  record: r,
                ),
            },
          ),
        );
      },
    );
  }
}

// ─────────────────────────────────────────────────────────────────────
// Kutilayotgan taklif
// ─────────────────────────────────────────────────────────────────────

const _fieldLabel = <String, String>{
  'name': 'Nomi',
  'category': 'Turkum',
  'description': 'Tavsif',
  'phone': 'Telefon',
  'hours': 'Ish vaqti',
  'street': 'Ko\'cha',
  'house': 'Uy raqami',
};

/// Formada tekis panjarada chiqadigan matn maydonlari (tavsifdan tashqari).
const _gridFields = ['name', 'phone', 'hours', 'street', 'house'];

class _PendingCard extends StatefulWidget {
  final PlaceRecord record;
  final PlacesMeta meta;
  final ModerationApi api;
  final Future<void> Function() onChanged;

  const _PendingCard({
    super.key,
    required this.record,
    required this.meta,
    required this.api,
    required this.onChanged,
  });

  @override
  State<_PendingCard> createState() => _PendingCardState();
}

class _PendingCardState extends State<_PendingCard> {
  late String _kind;
  late String _category;
  late final Map<String, TextEditingController> _text;
  late final TextEditingController _lat;
  late final TextEditingController _lng;
  final _note = TextEditingController();
  late final List<Future<Uint8List>> _photos;
  late final List<bool> _keep;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    final r = widget.record;
    _kind = widget.meta.kind(r.kind) != null
        ? r.kind
        : (widget.meta.kinds.isNotEmpty ? widget.meta.kinds.first.key : r.kind);
    // Ro'yxatda yo'q turkum (eski ma'lumot) — bo'sh qoladi, moderator tanlaydi.
    _category = widget.meta.categories.contains(r.category) ? r.category : '';
    _text = {
      'name': TextEditingController(text: r.name),
      'phone': TextEditingController(text: r.phone),
      'hours': TextEditingController(text: r.hours),
      'street': TextEditingController(text: r.street),
      'house': TextEditingController(text: r.house),
      'description': TextEditingController(text: r.description),
    };
    _lat = TextEditingController(text: '${r.lat}');
    _lng = TextEditingController(text: '${r.lng}');
    // Rasm sarlavha talab qiladi (token), shuning uchun `Image.network` emas:
    // baytlar API klienti orqali olinadi.
    _photos = [for (var i = 0; i < r.photos; i++) widget.api.submissionPhoto(r.id, i)];
    _keep = List<bool>.filled(r.photos, true);
  }

  @override
  void dispose() {
    for (final c in _text.values) {
      c.dispose();
    }
    _lat.dispose();
    _lng.dispose();
    _note.dispose();
    super.dispose();
  }

  Set<String> get _allowed => widget.meta.kind(_kind)?.allowed ?? const {};

  void _toast(String text, {bool error = false}) {
    if (!mounted) return;
    final cs = Theme.of(context).colorScheme;
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(
        content: Text(text),
        backgroundColor: error ? cs.error : null,
      ));
  }

  Future<void> _approve() async {
    final lat = double.tryParse(_lat.text.trim().replaceAll(',', '.'));
    final lng = double.tryParse(_lng.text.trim().replaceAll(',', '.'));
    if (lat == null || lng == null) {
      _toast('Koordinata noto\'g\'ri', error: true);
      return;
    }
    // Faqat shu turda RUXSAT ETILGAN maydonlar yuboriladi: server tur uchun
    // ruxsat etilmagan maydonni rad etadi (ko'rinmas maydonda eski qiymat
    // qolib ketgan bo'lsa ham).
    final edit = <String, Object>{'kind': _kind, 'lat': lat, 'lng': lng};
    for (final f in _allowed) {
      edit[f] = f == 'category' ? _category : (_text[f]?.text ?? '');
    }
    final keep = [
      for (var i = 0; i < _keep.length; i++)
        if (_keep[i]) i,
    ];

    setState(() => _busy = true);
    try {
      await widget.api.approve(id: widget.record.id, edit: edit, keepPhotos: keep);
      _toast('Tasdiqlandi — endi hammaga ko\'rinadi');
      await widget.onChanged();
    } on ModerationException catch (e) {
      _toast(e.message, error: true);
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _reject() async {
    setState(() => _busy = true);
    try {
      await widget.api.reject(widget.record.id, _note.text.trim());
      _toast('Rad etildi');
      await widget.onChanged();
    } on ModerationException catch (e) {
      _toast(e.message, error: true);
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final r = widget.record;
    final cs = Theme.of(context).colorScheme;
    final allowed = _allowed;
    final id = r.id;

    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Wrap(
              spacing: 12,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                Text(r.kindLabel, style: Theme.of(context).textTheme.titleMedium),
                Text(_fmtTime(r.createdAt), style: _muted(context)),
                Text('yuboruvchi: ${r.hint.substring(0, math.min(8, r.hint.length))}',
                    style: _muted(context)),
              ],
            ),
            if (_photos.isNotEmpty) ...[
              const SizedBox(height: 12),
              SizedBox(
                height: 170,
                child: ListView.separated(
                  scrollDirection: Axis.horizontal,
                  itemCount: _photos.length,
                  separatorBuilder: (context, index) => const SizedBox(width: 10),
                  itemBuilder: (_, i) => _PhotoTile(
                    future: _photos[i],
                    keep: _keep[i],
                    onKeep: (v) => setState(() => _keep[i] = v),
                  ),
                ),
              ),
            ],
            const SizedBox(height: 12),
            Wrap(
              spacing: 12,
              runSpacing: 12,
              children: [
                SizedBox(
                  width: 240,
                  child: DropdownButtonFormField<String>(
                    key: ValueKey('mod-kind-$id'),
                    initialValue: _kind,
                    isExpanded: true,
                    decoration: const InputDecoration(labelText: 'Turi', border: OutlineInputBorder()),
                    items: [
                      for (final k in widget.meta.kinds)
                        DropdownMenuItem(value: k.key, child: Text(k.label)),
                    ],
                    onChanged: _busy ? null : (v) => setState(() => _kind = v ?? _kind),
                  ),
                ),
                if (allowed.contains('category'))
                  SizedBox(
                    width: 240,
                    child: DropdownButtonFormField<String>(
                      key: ValueKey('mod-category-$id'),
                      initialValue: _category,
                      isExpanded: true,
                      decoration: const InputDecoration(labelText: 'Turkum', border: OutlineInputBorder()),
                      items: [
                        const DropdownMenuItem(value: '', child: Text('—')),
                        for (final c in widget.meta.categories)
                          DropdownMenuItem(value: c, child: Text(c)),
                      ],
                      onChanged: _busy ? null : (v) => setState(() => _category = v ?? ''),
                    ),
                  ),
                for (final f in _gridFields)
                  if (allowed.contains(f))
                    SizedBox(
                      width: 240,
                      child: TextField(
                        key: ValueKey('mod-$f-$id'),
                        controller: _text[f],
                        enabled: !_busy,
                        decoration: InputDecoration(
                          labelText: _fieldLabel[f],
                          border: const OutlineInputBorder(),
                        ),
                      ),
                    ),
              ],
            ),
            if (allowed.contains('description')) ...[
              const SizedBox(height: 12),
              TextField(
                key: ValueKey('mod-description-$id'),
                controller: _text['description'],
                enabled: !_busy,
                minLines: 2,
                maxLines: 6,
                maxLength: 1000,
                decoration: const InputDecoration(labelText: 'Tavsif', border: OutlineInputBorder()),
              ),
            ],
            const SizedBox(height: 4),
            Wrap(
              spacing: 12,
              runSpacing: 8,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                SizedBox(
                  width: 160,
                  child: TextField(
                    key: ValueKey('mod-lat-$id'),
                    controller: _lat,
                    enabled: !_busy,
                    decoration: const InputDecoration(labelText: 'Kenglik (lat)', border: OutlineInputBorder()),
                  ),
                ),
                SizedBox(
                  width: 160,
                  child: TextField(
                    key: ValueKey('mod-lng-$id'),
                    controller: _lng,
                    enabled: !_busy,
                    decoration: const InputDecoration(labelText: 'Uzunlik (lng)', border: OutlineInputBorder()),
                  ),
                ),
                TextButton.icon(
                  onPressed: () => _openMap(r.lat, r.lng),
                  icon: const Icon(Icons.open_in_new, size: 18),
                  label: const Text('OpenStreetMap\'da ko\'rish'),
                ),
              ],
            ),
            const SizedBox(height: 12),
            Wrap(
              spacing: 12,
              runSpacing: 8,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                FilledButton.icon(
                  key: ValueKey('mod-approve-$id'),
                  onPressed: _busy ? null : _approve,
                  icon: const Icon(Icons.check),
                  label: const Text('Tasdiqlash'),
                ),
                OutlinedButton.icon(
                  key: ValueKey('mod-reject-$id'),
                  onPressed: _busy ? null : _reject,
                  style: OutlinedButton.styleFrom(foregroundColor: cs.error),
                  icon: const Icon(Icons.close),
                  label: const Text('Rad etish'),
                ),
                SizedBox(
                  width: 320,
                  child: TextField(
                    key: ValueKey('mod-note-$id'),
                    controller: _note,
                    enabled: !_busy,
                    maxLength: 500,
                    decoration: const InputDecoration(
                      isDense: true,
                      counterText: '',
                      hintText: 'Rad etish sababi (ixtiyoriy)',
                      border: OutlineInputBorder(),
                    ),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _PhotoTile extends StatelessWidget {
  final Future<Uint8List> future;
  final bool keep;
  final ValueChanged<bool> onKeep;

  const _PhotoTile({required this.future, required this.keep, required this.onKeep});

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: 150,
      child: Column(
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(10),
            child: SizedBox(
              width: 150,
              height: 112,
              child: FutureBuilder<Uint8List>(
                future: future,
                builder: (context, snap) {
                  if (snap.hasData) {
                    return Opacity(
                      opacity: keep ? 1 : 0.35,
                      child: Image.memory(
                        snap.data!,
                        fit: BoxFit.cover,
                        errorBuilder: (context, error, stack) =>
                            const Center(child: Icon(Icons.broken_image_outlined)),
                      ),
                    );
                  }
                  if (snap.hasError) {
                    return const Center(child: Icon(Icons.broken_image_outlined));
                  }
                  return const Center(child: CircularProgressIndicator(strokeWidth: 2));
                },
              ),
            ),
          ),
          // `Flexible` + ellipsis: yorliq tor ustunga (150 px) sig'masa ham
          // qator toshib ketmaydi (shrift kengroq tizimlarda).
          Row(
            children: [
              Checkbox(
                value: keep,
                visualDensity: VisualDensity.compact,
                onChanged: (v) => onKeep(v ?? true),
              ),
              const Flexible(child: Text('Qoldirish', overflow: TextOverflow.ellipsis)),
            ],
          ),
        ],
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────
// Xaritadagi ob'ekt va rad etilgan taklif
// ─────────────────────────────────────────────────────────────────────

class _PlaceCard extends StatefulWidget {
  final PlaceRecord record;
  final ModerationApi api;
  final Future<void> Function() onChanged;

  const _PlaceCard({super.key, required this.record, required this.api, required this.onChanged});

  @override
  State<_PlaceCard> createState() => _PlaceCardState();
}

class _PlaceCardState extends State<_PlaceCard> {
  bool _busy = false;

  Future<void> _delete() async {
    final r = widget.record;
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Xaritadan olib tashlash'),
        content: Text('«${r.title}» ob\'ekti xaritadan olib tashlansinmi?'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('Bekor')),
          FilledButton(
            key: const ValueKey('mod-delete-confirm'),
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Olib tashlash'),
          ),
        ],
      ),
    );
    if (ok != true || !mounted) return;

    setState(() => _busy = true);
    try {
      await widget.api.deletePlace(r.id);
      if (!mounted) return;
      ScaffoldMessenger.of(context)
        ..hideCurrentSnackBar()
        ..showSnackBar(const SnackBar(content: Text('Olib tashlandi')));
      await widget.onChanged();
    } on ModerationException catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
        content: Text(e.message),
        backgroundColor: Theme.of(context).colorScheme.error,
      ));
      setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final r = widget.record;
    final line = [
      r.category,
      [r.street, r.house].where((e) => e.isNotEmpty).join(' '),
      r.phone,
      r.hours,
    ].where((e) => e.isNotEmpty).join(' · ');

    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Wrap(
              spacing: 12,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                Text(r.title, style: Theme.of(context).textTheme.titleMedium),
                Text(
                  '${r.kindLabel} · ${_fmtTime(r.createdAt)}${r.photos > 0 ? ' · ${r.photos} rasm' : ''}',
                  style: _muted(context),
                ),
              ],
            ),
            if (line.isNotEmpty) Text(line, style: _muted(context)),
            if (r.description.isNotEmpty) ...[
              const SizedBox(height: 6),
              Text(r.description),
            ],
            const SizedBox(height: 10),
            Wrap(
              spacing: 12,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                OutlinedButton.icon(
                  key: ValueKey('mod-delete-${r.id}'),
                  onPressed: _busy ? null : _delete,
                  style: OutlinedButton.styleFrom(foregroundColor: Theme.of(context).colorScheme.error),
                  icon: const Icon(Icons.delete_outline),
                  label: const Text('Xaritadan olib tashlash'),
                ),
                TextButton.icon(
                  onPressed: () => _openMap(r.lat, r.lng),
                  icon: const Icon(Icons.open_in_new, size: 18),
                  label: const Text('Xaritada ko\'rish'),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _RejectedCard extends StatelessWidget {
  final PlaceRecord record;

  const _RejectedCard({super.key, required this.record});

  @override
  Widget build(BuildContext context) {
    final r = record;
    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: ListTile(
        title: Text(r.title),
        subtitle: Text([
          r.kindLabel,
          if (r.description.isNotEmpty) r.description,
          if (r.reviewNote.isNotEmpty) 'Sabab: ${r.reviewNote}',
        ].join('\n')),
        isThreeLine: r.description.isNotEmpty || r.reviewNote.isNotEmpty,
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────
// Yordamchilar
// ─────────────────────────────────────────────────────────────────────

/// Yuklash xatosi ekrani (moderatsiya va muharrir uchun umumiy).
class OndexMapProblem extends StatelessWidget {
  final ModerationException error;
  final Future<void> Function() onRetry;

  const OndexMapProblem({super.key, required this.error, required this.onRetry});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.map_outlined, size: 48),
            const SizedBox(height: 16),
            SelectableText(
              error.message,
              key: const ValueKey('mod-error'),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 20),
            FilledButton.icon(
              onPressed: onRetry,
              icon: const Icon(Icons.refresh),
              label: const Text('Qayta urinish'),
            ),
          ],
        ),
      ),
    );
  }
}

TextStyle? _muted(BuildContext context) => Theme.of(context)
    .textTheme
    .bodySmall
    ?.copyWith(color: Theme.of(context).colorScheme.onSurfaceVariant);

/// ISO vaqtni mahalliy `yyyy-MM-dd HH:mm` ko'rinishiga aylantiradi.
String _fmtTime(String iso) {
  final d = DateTime.tryParse(iso)?.toLocal();
  if (d == null) return '';
  String two(int n) => n.toString().padLeft(2, '0');
  return '${d.year}-${two(d.month)}-${two(d.day)} ${two(d.hour)}:${two(d.minute)}';
}

/// Koordinatani tashqi xaritada ochadi. Manzilga faqat SONLAR qo'yiladi
/// (foydalanuvchi matni emas), sxema qat'iy `https`.
Future<void> _openMap(double lat, double lng) {
  final uri = Uri.parse(
    'https://www.openstreetmap.org/?mlat=$lat&mlon=$lng#map=19/$lat/$lng',
  );
  return launchUrl(uri, mode: LaunchMode.externalApplication);
}

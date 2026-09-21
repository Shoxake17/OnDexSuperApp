import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_map/flutter_map.dart';
import 'package:latlong2/latlong.dart';

import 'editor_geometry.dart';
import 'moderation_api.dart';
import 'moderation_view.dart' show OndexMapProblem;

/// OnDexMap MUHARRIRI — mahalla va ko'cha chegaralarini xaritada chizish.
///
/// ┌─ NATIV, WEBVIEW YO'Q ──────────────────────────────────────────────┐
/// Ilgari muharrir OnDexMap serveri bergan Mapbox HTML sahifasi edi va
/// panel uni WebView2 ichida ochardi. Endi hammasi Flutter: xarita
/// (`flutter_map`), chizish, tanlash, saqlash. Server faqat JSON beradi.
///
/// Xarita asosi:
///   • Sputnik — OnDexMap API'ning o'z tile proksisi (kalit serverda qoladi);
///   • Xarita  — OpenStreetMap raster tile'lari (kredit ko'rsatiladi).
/// 3D binolar yo'q: chegara chizish uchun tik (2D) ko'rinish aniqroq.
/// └────────────────────────────────────────────────────────────────────┘
///
/// Ish tartibi: qatlam tanlanadi (Mahalla / Ko'cha) → «Xaritada chizish» →
/// nuqtalar bosiladi → «Yakunlash» → nom va manba → «Saqlash». Ro'yxatdan
/// yoki xaritadan obyekt tanlansa, uning nomi/manbasi tahrirlanadi va
/// geometriyani «Qayta chizish» bilan almashtirish mumkin.
class OndexMapEditor extends StatefulWidget {
  final ModerationApi api;

  /// Testlar uchun: tarmoqsiz tile provayderi va boshqariladigan kamera.
  final TileProvider? tileProvider;
  final MapController? mapController;

  const OndexMapEditor({
    super.key,
    required this.api,
    this.tileProvider,
    this.mapController,
  });

  @override
  State<OndexMapEditor> createState() => _OndexMapEditorState();
}

enum _Basemap { satellite, streets }

class _Feat {
  final MapFeature f;
  final EditorShape? shape;
  const _Feat(this.f, this.shape);
}

const _sources = <String, String>{
  'official': 'Rasmiy (hokimlik)',
  'survey': 'Dala survey (o\'zim yurib oldim)',
  'community': 'Jamoa (aholi aytdi)',
  'osm': 'OpenStreetMap',
};

const _streetKinds = <String, String>{
  'kocha': 'Ko\'cha',
  'shox_kocha': 'Shox ko\'cha',
  'tor_kocha': 'Tor ko\'cha',
  'xiyobon': 'Xiyobon',
  'maydon': 'Maydon',
};

const _aliasKinds = <String, String>{
  'xalq': 'Xalq nomi',
  'eski': 'Eski nom',
  'kirill': 'Kirill yozuvi',
  'official': 'Rasmiy',
};

/// Chust markazi — xarita shu yerdan ochiladi.
const _chust = LatLng(41.0004, 71.2394);

/// O'zbekiston chegarasi (kamera undan chiqmaydi).
final _uzBounds = LatLngBounds(const LatLng(37.0, 55.5), const LatLng(45.8, 73.5));

/// Nomlar shu zoomdan boshlab ko'rinadi.
const _labelZoom = 15.0;

/// Poligonni yopish uchun birinchi nuqtaga shuncha piksel yaqin bosish kerak.
const _closeRadiusPx = 14.0;

class _OndexMapEditorState extends State<OndexMapEditor> {
  late final MapController _map = widget.mapController ?? MapController();
  final _mapFocus = FocusNode();

  String _layer = 'mahalla';
  List<_Feat> _feats = const [];
  EditorConfig _cfg = const EditorConfig();
  _Basemap _base = _Basemap.streets;

  bool _loading = true;
  ModerationException? _loadError;
  bool _configApplied = false;
  bool _fitted = false;
  bool _mapReady = false;
  bool _fitPending = false;
  bool _labelsOn = false;

  String? _selectedId;

  // Chizish.
  bool _drawing = false;
  List<LatLng> _points = [];
  List<LatLng>? _drawn;

  // Forma.
  final _name = TextEditingController();
  final _alias = TextEditingController();
  String _source = 'survey';
  String _streetKind = 'kocha';
  String _aliasKind = 'xalq';
  bool _busy = false;
  String? _msg;
  bool _msgError = false;

  /// Eskirgan javob yangisini bosib ketmasin (tez qatlam almashtirilganda).
  int _epoch = 0;

  GeomKind get _drawKind => _layer == 'mahalla' ? GeomKind.polygon : GeomKind.line;
  _Feat? get _selected {
    for (final f in _feats) {
      if (f.f.id == _selectedId) return f;
    }
    return null;
  }

  bool get _showForm => _selected != null || _drawn != null;

  @override
  void initState() {
    super.initState();
    _loadAll();
  }

  @override
  void dispose() {
    _name.dispose();
    _alias.dispose();
    _mapFocus.dispose();
    if (widget.mapController == null) _map.dispose();
    super.dispose();
  }

  // ── Yuklash ────────────────────────────────────────────────────────

  Future<void> _loadAll() async {
    final epoch = ++_epoch;
    setState(() {
      _loading = true;
      _loadError = null;
    });
    try {
      final cfg = await widget.api.editorConfig();
      final feats = await widget.api.features(_layer);
      if (!mounted || epoch != _epoch) return;
      setState(() {
        _cfg = cfg;
        // Sun'iy yo'ldosh bor bo'lsa u standart: chegara tasvir ustida chiziladi.
        // Faqat BIRINCHI yuklashda — keyin foydalanuvchi tanlovi buzilmasin.
        if (!_configApplied) {
          _configApplied = true;
          if (cfg.satelliteUrl != null) _base = _Basemap.satellite;
        }
        _apply(feats);
        _loading = false;
      });
      _fitAllOnce();
    } on ModerationException catch (e) {
      if (!mounted || epoch != _epoch) return;
      setState(() {
        _loadError = e;
        _loading = false;
      });
    }
  }

  Future<void> _reloadFeatures() async {
    final epoch = ++_epoch;
    try {
      final feats = await widget.api.features(_layer);
      if (!mounted || epoch != _epoch) return;
      setState(() => _apply(feats));
    } on ModerationException catch (e) {
      if (!mounted || epoch != _epoch) return;
      _say(e.message, error: true);
    }
  }

  void _apply(List<MapFeature> feats) {
    _feats = [for (final f in feats) _Feat(f, parseGeometry(f.geometry))];
    if (_selectedId != null && _selected == null) _selectedId = null;
  }

  /// Birinchi yuklashda barcha obyektlar ko'rinadigan qilib moslashtiradi.
  ///
  /// ⚠️ Xarita HALI o'lchamga ega bo'lmasa (birinchi kadr) `fitCamera` nol
  /// o'lcham bilan hisoblab, eng uzoq masshtabni (butun mamlakat) tanlaydi.
  /// Shuning uchun xarita tayyor bo'lguncha kutiladi (`onMapReady`).
  void _fitAllOnce() {
    if (_fitted) return;
    if (!_mapReady) {
      _fitPending = true;
      return;
    }
    final shapes = [for (final f in _feats) if (f.shape != null) f.shape!];
    if (shapes.isEmpty) return;
    _fitted = true;
    _fitShapes(shapes);
  }

  void _fitShapes(List<EditorShape> shapes) {
    final pts = [for (final s in shapes) ...s.points];
    if (pts.isEmpty) return;
    var minLat = 90.0, minLng = 180.0, maxLat = -90.0, maxLng = -180.0;
    for (final p in pts) {
      if (p.latitude < minLat) minLat = p.latitude;
      if (p.latitude > maxLat) maxLat = p.latitude;
      if (p.longitude < minLng) minLng = p.longitude;
      if (p.longitude > maxLng) maxLng = p.longitude;
    }
    try {
      _map.fitCamera(CameraFit.bounds(
        bounds: LatLngBounds(LatLng(minLat, minLng), LatLng(maxLat, maxLng)),
        padding: const EdgeInsets.all(64),
        maxZoom: 19,
      ));
    } catch (_) {
      // Xarita hali o'lchamga ega emas (birinchi kadr) — standart ko'rinishda qoladi.
    }
  }

  void _say(String text, {bool error = false}) {
    if (!mounted) return;
    setState(() {
      _msg = text;
      _msgError = error;
    });
  }

  // ── Qatlam, tanlash, chizish ───────────────────────────────────────

  void _setLayer(String layer) {
    if (layer == _layer) return;
    setState(() {
      _layer = layer;
      _feats = const [];
      _resetEditing();
    });
    _reloadFeatures();
  }

  void _resetEditing() {
    _selectedId = null;
    _drawing = false;
    _points = [];
    _drawn = null;
    _name.clear();
    _alias.clear();
    _msg = null;
  }

  void _select(String id) {
    final f = _feats.cast<_Feat?>().firstWhere((e) => e!.f.id == id, orElse: () => null);
    if (f == null) return;
    setState(() {
      _drawing = false;
      _points = [];
      _drawn = null;
      _selectedId = id;
      _name.text = f.f.name;
      _source = _sources.containsKey(f.f.source) ? f.f.source : 'survey';
      if (_layer == 'street') {
        _streetKind = _streetKinds.containsKey(f.f.streetKind) ? f.f.streetKind : 'kocha';
      }
      _alias.clear();
      _msg = null;
    });
    if (f.shape != null) _fitShapes([f.shape!]);
  }

  void _cancelForm() => setState(_resetEditing);

  void _startDraw() {
    setState(() {
      _drawing = true;
      _points = [];
      _drawn = null;
      _msg = null;
    });
    _mapFocus.requestFocus();
  }

  void _cancelDraw() => setState(() {
        _drawing = false;
        _points = [];
      });

  void _undo() {
    if (_points.isEmpty) return;
    setState(() => _points = _points.sublist(0, _points.length - 1));
  }

  void _finishDraw() {
    final problem = drawingProblem(_drawKind, _points);
    if (problem != null) {
      _say(problem, error: true);
      return;
    }
    setState(() {
      _drawn = List.of(_points);
      _points = [];
      _drawing = false;
      _msg = null;
    });
  }

  // ── Xarita bosildi ─────────────────────────────────────────────────

  void _onTap(TapPosition tp, LatLng ll) {
    if (_drawing) {
      // Poligonni yopish: birinchi nuqtaga (kamida 3 nuqtadan keyin) bosish.
      if (_drawKind == GeomKind.polygon && _points.length >= 3) {
        final first = _map.camera.latLngToScreenOffset(_points.first);
        final at = tp.relative ?? tp.global;
        if ((first - at).distance <= _closeRadiusPx) {
          _finishDraw();
          return;
        }
      }
      setState(() {
        _points = [..._points, ll];
        _msg = null;
      });
      return;
    }

    // Oddiy rejim: bosilgan joydagi obyekt tanlanadi.
    final tol = metersPerPixel(ll.latitude, _map.camera.zoom) * 10;
    _Feat? best;
    var bestArea = double.infinity;
    for (final f in _feats) {
      final s = f.shape;
      if (s == null || !shapeHit(s, ll, tol)) continue;
      // Bir nechtasi mos kelsa — eng kichigi (chiziq ustun): u tepada ko'rinadi.
      final b = s.bounds;
      final area = s.kind == GeomKind.line ? 0.0 : (b[2] - b[0]) * (b[3] - b[1]);
      if (area < bestArea) {
        best = f;
        bestArea = area;
      }
    }
    if (best != null) _select(best.f.id);
  }

  // ── Saqlash / o'chirish / muqobil nom ──────────────────────────────

  Future<void> _save() async {
    final name = _name.text.trim();
    if (name.isEmpty) {
      _say('Nom kiritilmagan', error: true);
      return;
    }
    final geometry = _drawn != null ? toGeoJson(_drawKind, _drawn!) : _selected?.f.geometry;
    if (geometry == null || geometry.isEmpty) {
      _say('Geometriya yo\'q — avval xaritada chizing', error: true);
      return;
    }

    setState(() => _busy = true);
    try {
      await widget.api.saveFeature(
        layer: _layer,
        id: _selectedId ?? '',
        name: name,
        source: _source,
        streetKind: _layer == 'street' ? _streetKind : null,
        geometry: geometry,
      );
      if (!mounted) return;
      setState(() {
        _resetEditing();
        _busy = false;
        _msg = 'Saqlandi';
        _msgError = false;
      });
      await _reloadFeatures();
    } on ModerationException catch (e) {
      if (!mounted) return;
      setState(() => _busy = false);
      _say(e.message, error: true);
    }
  }

  Future<void> _delete() async {
    final sel = _selected;
    if (sel == null) return;
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('O\'chirish'),
        content: Text('«${sel.f.name}» butunlay o\'chirilsinmi? Buni qaytarib bo\'lmaydi.'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('Bekor')),
          FilledButton(
            key: const ValueKey('editor-delete-confirm'),
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('O\'chirish'),
          ),
        ],
      ),
    );
    if (ok != true || !mounted) return;

    setState(() => _busy = true);
    try {
      await widget.api.deleteFeature(_layer, sel.f.id);
      if (!mounted) return;
      setState(() {
        _resetEditing();
        _busy = false;
      });
      await _reloadFeatures();
    } on ModerationException catch (e) {
      if (!mounted) return;
      setState(() => _busy = false);
      _say(e.message, error: true);
    }
  }

  Future<void> _addAlias() async {
    final alias = _alias.text.trim();
    final id = _selectedId;
    if (alias.isEmpty || id == null) return;
    setState(() => _busy = true);
    try {
      await widget.api.addAlias(streetId: id, alias: alias, kind: _aliasKind, source: _source);
      if (!mounted) return;
      setState(() {
        _busy = false;
        _alias.clear();
      });
      _say('Qo\'shildi: $alias');
    } on ModerationException catch (e) {
      if (!mounted) return;
      setState(() => _busy = false);
      _say(e.message, error: true);
    }
  }

  // ── Klaviatura (xarita fokusda) ────────────────────────────────────

  KeyEventResult _onKey(FocusNode node, KeyEvent e) {
    if (e is! KeyDownEvent || !_drawing) return KeyEventResult.ignored;
    if (e.logicalKey == LogicalKeyboardKey.escape) {
      _cancelDraw();
      return KeyEventResult.handled;
    }
    if (e.logicalKey == LogicalKeyboardKey.enter) {
      _finishDraw();
      return KeyEventResult.handled;
    }
    if (e.logicalKey == LogicalKeyboardKey.backspace) {
      _undo();
      return KeyEventResult.handled;
    }
    return KeyEventResult.ignored;
  }

  // ═══════════════════════════════════════════════════════════════════
  // Ko'rinish
  // ═══════════════════════════════════════════════════════════════════

  @override
  Widget build(BuildContext context) {
    if (_loadError != null) {
      return OndexMapProblem(error: _loadError!, onRetry: _loadAll);
    }
    return Row(
      children: [
        SizedBox(width: 340, child: _sidebar(context)),
        const VerticalDivider(width: 1),
        Expanded(child: _mapArea(context)),
      ],
    );
  }

  // ── Yon panel ──────────────────────────────────────────────────────

  Widget _sidebar(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    final sel = _selected;

    return ColoredBox(
      color: cs.surface,
      child: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          Row(
            children: [
              Expanded(child: Text('Ma\'lumot kiritish', style: tt.titleMedium)),
              Chip(
                key: const ValueKey('editor-count'),
                label: Text(_loading ? '…' : '${_feats.length} ta'),
                visualDensity: VisualDensity.compact,
              ),
            ],
          ),
          const SizedBox(height: 12),
          SegmentedButton<String>(
            key: const ValueKey('editor-layers'),
            showSelectedIcon: false,
            segments: const [
              ButtonSegment(value: 'mahalla', label: Text('Mahalla')),
              ButtonSegment(value: 'street', label: Text('Ko\'cha')),
            ],
            selected: {_layer},
            onSelectionChanged: (s) => _setLayer(s.first),
          ),
          const SizedBox(height: 12),
          FilledButton.icon(
            key: const ValueKey('editor-draw'),
            onPressed: _busy ? null : _startDraw,
            icon: const Icon(Icons.edit_location_alt_outlined),
            label: Text(_drawn != null || sel != null ? 'Qayta chizish' : 'Xaritada chizish'),
          ),
          const SizedBox(height: 8),
          Text(
            _drawing
                ? (_drawKind == GeomKind.polygon
                    ? 'Chegara nuqtalarini ketma-ket bosing. Yakunlash: birinchi nuqtani bosing yoki «Yakunlash».'
                    : 'Ko\'cha bo\'ylab nuqtalarni bosing. Yakunlash: «Yakunlash» tugmasi yoki Enter.')
                : sel != null
                    ? 'Tahrirlanmoqda. Geometriyani almashtirish uchun «Qayta chizish».'
                    : 'Tugmani bosing va xaritada ${_layer == 'mahalla' ? 'chegarani' : 'ko\'cha chizig\'ini'} chizing. '
                        'Mavjud obyektni tahrirlash uchun uni xaritadan yoki ro\'yxatdan tanlang.',
            style: tt.bodySmall?.copyWith(color: cs.onSurfaceVariant),
          ),
          if (_showForm) ...[
            const SizedBox(height: 16),
            _form(context),
          ],
          if (_msg != null) ...[
            const SizedBox(height: 12),
            Container(
              key: const ValueKey('editor-msg'),
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(
                color: _msgError ? cs.errorContainer : cs.primaryContainer,
                borderRadius: BorderRadius.circular(8),
              ),
              child: Text(
                _msg!,
                style: TextStyle(color: _msgError ? cs.onErrorContainer : cs.onPrimaryContainer),
              ),
            ),
          ],
          if (_layer == 'street' && sel != null) ...[
            const SizedBox(height: 20),
            _aliasSection(context),
          ],
          const SizedBox(height: 20),
          Text('Mavjud yozuvlar', style: tt.labelLarge),
          const SizedBox(height: 8),
          if (_loading && _feats.isEmpty)
            const Padding(padding: EdgeInsets.all(16), child: Center(child: CircularProgressIndicator()))
          else if (_feats.isEmpty)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 16),
              child: Center(
                child: Text('Hali yozuv yo\'q',
                    key: const ValueKey('editor-empty'),
                    style: TextStyle(color: cs.onSurfaceVariant)),
              ),
            )
          else
            for (final f in _feats) _featureTile(context, f),
        ],
      ),
    );
  }

  Widget _featureTile(BuildContext context, _Feat f) {
    final cs = Theme.of(context).colorScheme;
    final on = f.f.id == _selectedId;
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      color: on ? cs.primaryContainer : null,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(10),
        side: BorderSide(color: on ? cs.primary : cs.outlineVariant),
      ),
      child: ListTile(
        key: ValueKey('editor-item-${f.f.id}'),
        dense: true,
        title: Text(f.f.name, maxLines: 1, overflow: TextOverflow.ellipsis),
        subtitle: f.shape == null ? const Text('geometriya yo\'q / yaroqsiz') : null,
        trailing: Text(f.f.source, style: Theme.of(context).textTheme.labelSmall),
        onTap: _busy ? null : () => _select(f.f.id),
      ),
    );
  }

  Widget _form(BuildContext context) {
    final sel = _selected;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        TextField(
          key: const ValueKey('editor-name'),
          controller: _name,
          enabled: !_busy,
          maxLength: 200,
          decoration: InputDecoration(
            labelText: 'Nomi',
            hintText: _layer == 'mahalla' ? 'masalan: Chorsu mahallasi' : 'masalan: Navoiy ko\'chasi',
            border: const OutlineInputBorder(),
            counterText: '',
          ),
        ),
        if (_layer == 'street') ...[
          const SizedBox(height: 12),
          DropdownButtonFormField<String>(
            key: const ValueKey('editor-kind'),
            initialValue: _streetKind,
            isExpanded: true,
            decoration: const InputDecoration(labelText: 'Turi', border: OutlineInputBorder()),
            items: [
              for (final e in _streetKinds.entries) DropdownMenuItem(value: e.key, child: Text(e.value)),
            ],
            onChanged: _busy ? null : (v) => setState(() => _streetKind = v ?? _streetKind),
          ),
        ],
        const SizedBox(height: 12),
        DropdownButtonFormField<String>(
          key: const ValueKey('editor-source'),
          initialValue: _source,
          isExpanded: true,
          decoration: const InputDecoration(labelText: 'Manba (provenans)', border: OutlineInputBorder()),
          items: [
            for (final e in _sources.entries) DropdownMenuItem(value: e.key, child: Text(e.value)),
          ],
          onChanged: _busy ? null : (v) => setState(() => _source = v ?? _source),
        ),
        const SizedBox(height: 6),
        Text(
          'Manbani to\'g\'ri tanlang — litsenziya qarori keyin shu ustunga tayanadi.',
          style: Theme.of(context)
              .textTheme
              .bodySmall
              ?.copyWith(color: Theme.of(context).colorScheme.onSurfaceVariant),
        ),
        const SizedBox(height: 12),
        FilledButton(
          key: const ValueKey('editor-save'),
          onPressed: _busy ? null : _save,
          child: Text(_busy ? 'Saqlanmoqda…' : 'Saqlash'),
        ),
        const SizedBox(height: 8),
        Row(
          children: [
            Expanded(
              child: OutlinedButton(
                key: const ValueKey('editor-cancel'),
                onPressed: _busy ? null : _cancelForm,
                child: const Text('Bekor'),
              ),
            ),
            if (sel != null) ...[
              const SizedBox(width: 8),
              Expanded(
                child: OutlinedButton(
                  key: const ValueKey('editor-delete'),
                  onPressed: _busy ? null : _delete,
                  style: OutlinedButton.styleFrom(foregroundColor: Theme.of(context).colorScheme.error),
                  child: const Text('O\'chirish'),
                ),
              ),
            ],
          ],
        ),
      ],
    );
  }

  Widget _aliasSection(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text('Muqobil nom qo\'shish', style: Theme.of(context).textTheme.labelLarge),
        const SizedBox(height: 8),
        TextField(
          key: const ValueKey('editor-alias'),
          controller: _alias,
          enabled: !_busy,
          maxLength: 200,
          decoration: const InputDecoration(
            hintText: 'masalan: Katta ko\'cha',
            border: OutlineInputBorder(),
            counterText: '',
          ),
        ),
        const SizedBox(height: 8),
        DropdownButtonFormField<String>(
          key: const ValueKey('editor-alias-kind'),
          initialValue: _aliasKind,
          isExpanded: true,
          decoration: const InputDecoration(border: OutlineInputBorder()),
          items: [
            for (final e in _aliasKinds.entries) DropdownMenuItem(value: e.key, child: Text(e.value)),
          ],
          onChanged: _busy ? null : (v) => setState(() => _aliasKind = v ?? _aliasKind),
        ),
        const SizedBox(height: 8),
        OutlinedButton(
          key: const ValueKey('editor-alias-add'),
          onPressed: _busy ? null : _addAlias,
          child: const Text('Qo\'shish'),
        ),
      ],
    );
  }

  // ── Xarita ─────────────────────────────────────────────────────────

  Widget _mapArea(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final selectedId = _selectedId;

    final polygons = <Polygon>[];
    final lines = <Polyline>[];
    for (final f in _feats) {
      final s = f.shape;
      if (s == null) continue;
      final on = f.f.id == selectedId;
      // Tanlangan obyektning ESKI geometriyasi qayta chizilayotganda xiralashadi.
      final dim = on && _drawn != null;
      final base = cs.primary;
      if (s.kind == GeomKind.polygon) {
        for (final ring in s.parts) {
          polygons.add(Polygon(
            points: ring,
            color: base.withValues(alpha: on ? (dim ? 0.05 : 0.30) : 0.16),
            borderColor: on && !dim ? Colors.white : base,
            borderStrokeWidth: on ? 3.5 : 2.5,
          ));
          if (on && !dim) {
            // Tanlanganda oq chegara ostidan asosiy rang chiziq — ikki rangli ajratish.
            lines.add(Polyline(points: [...ring, ring.first], color: base, strokeWidth: 2));
          }
        }
      } else {
        for (final l in s.parts) {
          lines.add(Polyline(
            points: l,
            color: on ? Colors.orange.shade800 : base,
            strokeWidth: on ? 6 : 3.5,
          ));
        }
      }
    }

    // Yangi/qayta chizilgan geometriya — to'q sariq (saqlanmagani ko'rinsin).
    final draft = _drawn;
    if (draft != null) {
      if (_drawKind == GeomKind.polygon) {
        polygons.add(Polygon(
          points: draft,
          color: Colors.deepOrange.withValues(alpha: 0.22),
          borderColor: Colors.deepOrange,
          borderStrokeWidth: 3,
        ));
      } else {
        lines.add(Polyline(points: draft, color: Colors.deepOrange, strokeWidth: 5));
      }
    }

    // Chizish jarayoni: ko'rsatkich chiziq/poligon (punktir) va uchlar.
    final circles = <CircleMarker>[];
    if (_drawing && _points.isNotEmpty) {
      if (_drawKind == GeomKind.polygon && _points.length >= 3) {
        polygons.add(Polygon(
          points: _points,
          color: Colors.deepOrange.withValues(alpha: 0.12),
          borderColor: Colors.deepOrange,
          borderStrokeWidth: 2.5,
          pattern: StrokePattern.dashed(segments: const [8, 6]),
        ));
      } else {
        lines.add(Polyline(
          points: _points,
          color: Colors.deepOrange,
          strokeWidth: 2.5,
          pattern: StrokePattern.dashed(segments: const [8, 6]),
        ));
      }
      for (var i = 0; i < _points.length; i++) {
        final first = i == 0 && _drawKind == GeomKind.polygon && _points.length >= 3;
        circles.add(CircleMarker(
          point: _points[i],
          radius: first ? 8 : 5,
          color: first ? Colors.green : Colors.white,
          borderColor: Colors.deepOrange,
          borderStrokeWidth: 2,
        ));
      }
    }

    final labels = <Marker>[];
    if (_labelsOn) {
      for (final f in _feats) {
        final s = f.shape;
        if (s == null) continue;
        labels.add(Marker(
          point: s.labelPoint,
          width: 180,
          height: 26,
          child: IgnorePointer(
            child: Center(
              child: Text(
                f.f.name,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(
                  fontSize: 12,
                  fontWeight: FontWeight.w600,
                  color: Color(0xFF14361A),
                  shadows: [
                    Shadow(color: Colors.white, blurRadius: 3),
                    Shadow(color: Colors.white, blurRadius: 3),
                    Shadow(color: Colors.white, blurRadius: 6),
                  ],
                ),
              ),
            ),
          ),
        ));
      }
    }

    final satellite = _base == _Basemap.satellite && _cfg.satelliteUrl != null;

    return Focus(
      focusNode: _mapFocus,
      onKeyEvent: _onKey,
      child: Listener(
        onPointerDown: (_) => _mapFocus.requestFocus(),
        child: MouseRegion(
          cursor: _drawing ? SystemMouseCursors.precise : MouseCursor.defer,
          child: Stack(
            children: [
              FlutterMap(
                mapController: _map,
                options: MapOptions(
                  initialCenter: _chust,
                  initialZoom: 16,
                  minZoom: 6,
                  maxZoom: 20,
                  cameraConstraint: CameraConstraint.contain(bounds: _uzBounds),
                  // Chegara chizish uchun tik ko'rinish: aylantirish o'chiq.
                  interactionOptions: const InteractionOptions(
                    flags: InteractiveFlag.all & ~InteractiveFlag.rotate,
                  ),
                  onTap: _onTap,
                  onMapReady: () {
                    _mapReady = true;
                    if (_fitPending) {
                      _fitPending = false;
                      _fitAllOnce();
                    }
                  },
                  onPositionChanged: (pos, _) {
                    final on = pos.zoom >= _labelZoom;
                    if (on != _labelsOn) setState(() => _labelsOn = on);
                  },
                ),
                children: [
                  TileLayer(
                    key: ValueKey(satellite),
                    urlTemplate: satellite
                        ? _cfg.satelliteUrl!
                        : 'https://tile.openstreetmap.org/{z}/{x}/{y}.png',
                    userAgentPackageName: 'uz.ondex.chust_admin',
                    maxNativeZoom: satellite ? _cfg.satelliteMaxZoom : 19,
                    maxZoom: 20,
                    tileProvider: widget.tileProvider,
                  ),
                  PolygonLayer(polygons: polygons),
                  PolylineLayer(polylines: lines),
                  if (labels.isNotEmpty) MarkerLayer(markers: labels),
                  if (circles.isNotEmpty) CircleLayer(circles: circles),
                  RichAttributionWidget(
                    alignment: AttributionAlignment.bottomRight,
                    showFlutterMapAttribution: false,
                    attributions: [
                      const TextSourceAttribution('© OnDex map'),
                      if (satellite)
                        TextSourceAttribution(
                          _cfg.satelliteAttribution.isEmpty ? 'Powered by Esri' : _cfg.satelliteAttribution,
                        )
                      else
                        const TextSourceAttribution('© OpenStreetMap contributors'),
                    ],
                  ),
                ],
              ),
              // Asos xarita almashtirgichi.
              if (_cfg.satelliteUrl != null)
                Positioned(
                  left: 12,
                  top: 12,
                  child: Material(
                    elevation: 2,
                    borderRadius: BorderRadius.circular(20),
                    child: SegmentedButton<_Basemap>(
                      key: const ValueKey('editor-basemap'),
                      showSelectedIcon: false,
                      segments: const [
                        ButtonSegment(value: _Basemap.streets, label: Text('Xarita')),
                        ButtonSegment(value: _Basemap.satellite, label: Text('Sputnik')),
                      ],
                      selected: {_base},
                      onSelectionChanged: (s) => setState(() => _base = s.first),
                    ),
                  ),
                ),
              // Masshtab tugmalari.
              Positioned(
                right: 12,
                top: 12,
                child: Material(
                  elevation: 2,
                  borderRadius: BorderRadius.circular(12),
                  child: Column(
                    children: [
                      IconButton(
                        key: const ValueKey('editor-zoom-in'),
                        tooltip: 'Yaqinlashtirish',
                        icon: const Icon(Icons.add),
                        onPressed: () => _zoomBy(1),
                      ),
                      const Divider(height: 1),
                      IconButton(
                        key: const ValueKey('editor-zoom-out'),
                        tooltip: 'Uzoqlashtirish',
                        icon: const Icon(Icons.remove),
                        onPressed: () => _zoomBy(-1),
                      ),
                    ],
                  ),
                ),
              ),
              if (_drawing) Positioned(bottom: 40, left: 0, right: 0, child: Center(child: _drawBar(context))),
            ],
          ),
        ),
      ),
    );
  }

  void _zoomBy(double d) {
    final cam = _map.camera;
    _map.move(cam.center, (cam.zoom + d).clamp(6.0, 20.0));
  }

  /// Chizish paytidagi pastki panel: nuqtalar soni, ortga, yakunlash, bekor.
  Widget _drawBar(BuildContext context) {
    return Material(
      elevation: 4,
      borderRadius: BorderRadius.circular(28),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text('${_points.length} nuqta', key: const ValueKey('editor-points')),
            const SizedBox(width: 12),
            TextButton.icon(
              key: const ValueKey('editor-undo'),
              onPressed: _points.isEmpty ? null : _undo,
              icon: const Icon(Icons.undo),
              label: const Text('Ortga'),
            ),
            FilledButton.icon(
              key: const ValueKey('editor-finish'),
              onPressed: _points.length >= (_drawKind == GeomKind.polygon ? 3 : 2) ? _finishDraw : null,
              icon: const Icon(Icons.check),
              label: const Text('Yakunlash'),
            ),
            const SizedBox(width: 8),
            TextButton(
              key: const ValueKey('editor-cancel-draw'),
              onPressed: _cancelDraw,
              child: const Text('Bekor'),
            ),
          ],
        ),
      ),
    );
  }
}

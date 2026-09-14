import 'dart:convert';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';

import '../api.dart';
import '../panel_prefs.dart';
import '../sound.dart';
import '../theme.dart';
import '../widgets/date_range_dialog.dart';

part 'settings/settings_cards.dart';
part 'settings/hours_dialog.dart';

/// "Restoran sozlamalari" — `image/RestorantSozlamalari.png` bo'yicha
/// (ichki bo'limlar menyusisiz: hamma bo'lim bitta sahifada ko'rinadi).
///
/// ┌─ NIMA TAHRIRLANADI, NIMA YO'Q ────────────────────────────────────┐
/// Nomi, turi, telefon raqami, manzili — FAQAT ko'rish uchun: ularni
/// OnDex administratori o'zgartiradi (shartnoma, xarita, kuryer
/// taqsimoti). Bu yerda ular matn maydoni EMAS — tahrirlab bo'ladigandek
/// ko'rinmasin. Qoida serverda ham bor: `PATCH /restaurants/{id}/settings`
/// bu maydonlarni 403 bilan rad etadi.
///
/// Logo, muqova, tavsif, ish vaqti, to'lov usullari — restoranning o'zi,
/// "Saqlash" bilan BIRDANIGA (faqat o'zgargan maydonlar yuboriladi).
/// "Restoran ochiq" tugmasi va bildirishnomalar — darhol qo'llanadi.
/// └───────────────────────────────────────────────────────────────────┘
class RestaurantSettingsPage extends StatefulWidget {
  const RestaurantSettingsPage({
    super.key,
    required this.guard,
    required this.open,
    required this.onOpenChanged,
    this.onSaved,
  });

  /// Qobiq boshqa sahifaga o'tishdan oldin saqlanmagan o'zgarishni so'raydi.
  final SettingsLeaveGuard guard;

  /// Qobiqdagi (yuqori paneldagi) "Ochiq/Yopiq" bilan bitta holat.
  final bool open;
  final Future<void> Function(bool open) onOpenChanged;

  /// Logo va boshqa ma'lumot yuqori panel/yon menyuda ham yangilansin.
  final VoidCallback? onSaved;

  @override
  State<RestaurantSettingsPage> createState() => _RestaurantSettingsPageState();
}

/// Qobiq (`shell.dart`) sahifadan chiqishdan oldin shu orqali so'raydi.
class SettingsLeaveGuard {
  bool Function()? _check;

  bool get hasUnsavedChanges => _check?.call() ?? false;
}

Future<bool> confirmDiscardSettings(BuildContext context) async {
  final leave = await showDialog<bool>(
    context: context,
    builder: (ctx) => AlertDialog(
      icon: const Icon(Icons.warning_amber_rounded, color: OnDexColors.amber, size: 30),
      title: const Text('Saqlanmagan o\'zgarishlar'),
      content: const Text(
          'Sozlamalardagi o\'zgarishlar hali saqlanmagan. Sahifadan chiqsangiz ular yo\'qoladi.'),
      actions: [
        TextButton(onPressed: () => Navigator.of(ctx).pop(false), child: const Text('Qolish')),
        FilledButton(
          style: FilledButton.styleFrom(backgroundColor: OnDexColors.danger),
          onPressed: () => Navigator.of(ctx).pop(true),
          child: const Text('Saqlamasdan chiqish'),
        ),
      ],
    ),
  );
  return leave == true;
}

const _dayNames = [
  'Dushanba', 'Seshanba', 'Chorshanba', 'Payshanba', 'Juma', 'Shanba', 'Yakshanba', //
];

/// Server bilan bir xil (`POST /uploads` — 5 MB).
const _maxImageBytes = 5 * 1024 * 1024;
const _imageExtensions = ['jpg', 'jpeg', 'png', 'webp'];

/// Bugungi kun (1 — dushanba) Toshkent vaqtida: ish vaqti serverda ham
/// shu mintaqada hisoblanadi.
int _todayIso() => DateTime.now().toUtc().add(const Duration(hours: 5)).weekday;

class _DayHours {
  const _DayHours({required this.day, required this.enabled, required this.open, required this.close});

  final int day;
  final bool enabled;
  final String open;
  final String close;

  _DayHours copyWith({bool? enabled, String? open, String? close}) => _DayHours(
        day: day,
        enabled: enabled ?? this.enabled,
        open: open ?? this.open,
        close: close ?? this.close,
      );

  Map<String, Object> toJson() => {'day': day, 'enabled': enabled, 'open': open, 'close': close};

  static _DayHours fromJson(Map<String, dynamic> j) => _DayHours(
        day: j['day'] is num ? (j['day'] as num).toInt() : 0,
        enabled: j['enabled'] == true,
        open: j['open'] is String ? j['open'] as String : '08:00',
        close: j['close'] is String ? j['close'] as String : '23:00',
      );

  /// "08:00 – 23:00", tunda yopilsa "(ertasi)", teng bo'lsa "Kun bo'yi".
  String get rangeText {
    if (open == close) return 'Kun bo\'yi (24 soat)';
    final nextDay = close.compareTo(open) < 0;
    return '$open – $close${nextDay ? ' (ertasi)' : ''}';
  }
}

List<_DayHours> _defaultWeek() => [
      for (var d = 1; d <= 7; d++) _DayHours(day: d, enabled: true, open: '08:00', close: '23:00'),
    ];

TimeOfDay _timeOf(String hhmm) {
  final parts = hhmm.split(':');
  final h = parts.isNotEmpty ? int.tryParse(parts[0]) ?? 8 : 8;
  final m = parts.length > 1 ? int.tryParse(parts[1]) ?? 0 : 0;
  return TimeOfDay(hour: h.clamp(0, 23), minute: m.clamp(0, 59));
}

String _hhmm(TimeOfDay t) =>
    '${t.hour.toString().padLeft(2, '0')}:${t.minute.toString().padLeft(2, '0')}';

String _str(Object? v) => v is String ? v : '';

class _Settings {
  const _Settings({
    required this.name,
    required this.kindTitle,
    required this.phone,
    required this.address,
    required this.logoUrl,
    required this.coverUrl,
    required this.description,
    required this.descriptionMax,
    required this.openNow,
    required this.hours,
    required this.payCash,
    required this.payTerminal,
    required this.payOnline,
    required this.onlineAvailable,
  });

  final String name;
  final String kindTitle;
  final String phone;
  final String address;
  final String logoUrl;
  final String coverUrl;
  final String description;
  final int descriptionMax;
  final bool openNow;
  final List<_DayHours>? hours;
  final bool payCash;
  final bool payTerminal;
  final bool payOnline;
  final bool onlineAvailable;

  factory _Settings.fromJson(Map<String, dynamic> j) {
    final wh = j['working_hours'];
    List<_DayHours>? hours;
    if (wh is Map && wh['days'] is List) {
      hours = [
        for (final d in wh['days'] as List)
          if (d is Map) _DayHours.fromJson(Map<String, dynamic>.from(d)),
      ]..sort((a, b) => a.day.compareTo(b.day));
      if (hours.length != 7) hours = null;
    }
    final pm = j['payment_methods'] is Map ? j['payment_methods'] as Map : const {};
    return _Settings(
      name: _str(j['name']),
      kindTitle: _str(j['kind_title']),
      phone: _str(j['phone']),
      address: _str(j['address']),
      logoUrl: _str(j['logo_url']),
      coverUrl: _str(j['cover_url']),
      description: _str(j['description']),
      descriptionMax: j['description_max'] is num ? (j['description_max'] as num).toInt() : 500,
      openNow: j['open_now'] == true,
      hours: hours,
      payCash: pm['cash'] == true,
      payTerminal: pm['card_terminal'] == true,
      payOnline: pm['card_online'] == true,
      onlineAvailable: j['online_payments_available'] == true,
    );
  }
}

class _RestaurantSettingsPageState extends State<RestaurantSettingsPage> {
  _Settings? _saved;
  final _description = TextEditingController();
  String _logoUrl = '';
  String _coverUrl = '';
  List<_DayHours>? _hours;
  bool _payCash = true;
  bool _payTerminal = false;
  bool _payOnline = true;

  bool _loading = true;
  String? _loadError;
  bool _saving = false;
  bool _uploadingLogo = false;
  bool _uploadingCover = false;
  bool _openBusy = false;

  @override
  void initState() {
    super.initState();
    widget.guard._check = () => _dirty;
    _description.addListener(() {
      if (mounted) setState(() {});
    });
    _load();
  }

  @override
  void didUpdateWidget(RestaurantSettingsPage old) {
    super.didUpdateWidget(old);
    if (old.guard != widget.guard) {
      old.guard._check = null;
      widget.guard._check = () => _dirty;
    }
    // Yuqori paneldagi "Ochiq/Yopiq" bosilsa ham `open_now` yangilansin.
    if (old.open != widget.open && !_openBusy) _load(quiet: true);
  }

  @override
  void dispose() {
    widget.guard._check = null;
    _description.dispose();
    super.dispose();
  }

  Future<void> _load({bool quiet = false}) async {
    if (!quiet) {
      setState(() {
        _loading = _saved == null;
        _loadError = null;
      });
    }
    try {
      final json = await api.restaurantSettings();
      if (!mounted) return;
      final s = _Settings.fromJson(json);
      setState(() {
        // Jimgina yangilashda (masalan "Ochiq" bosilganda) foydalanuvchi
        // yozayotgan narsa o'chib ketmasin.
        if (quiet && _dirty) {
          _saved = _mergeServerOnly(s);
        } else {
          _apply(s);
        }
        _loading = false;
      });
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _loadError = e.message;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _loadError = 'Sozlamalarni yuklab bo\'lmadi. Internet aloqasini tekshirib, qayta urinib ko\'ring.';
      });
    }
  }

  /// Faqat server holatiga bog'liq maydonlar (masalan `open_now`) yangilanadi.
  _Settings _mergeServerOnly(_Settings fresh) {
    final old = _saved;
    if (old == null) return fresh;
    return _Settings(
      name: fresh.name,
      kindTitle: fresh.kindTitle,
      phone: fresh.phone,
      address: fresh.address,
      logoUrl: old.logoUrl,
      coverUrl: old.coverUrl,
      description: old.description,
      descriptionMax: fresh.descriptionMax,
      openNow: fresh.openNow,
      hours: old.hours,
      payCash: old.payCash,
      payTerminal: old.payTerminal,
      payOnline: old.payOnline,
      onlineAvailable: fresh.onlineAvailable,
    );
  }

  void _apply(_Settings s) {
    _saved = s;
    _description.text = s.description;
    _logoUrl = s.logoUrl;
    _coverUrl = s.coverUrl;
    _hours = s.hours == null ? null : [...s.hours!];
    _payCash = s.payCash;
    _payTerminal = s.payTerminal;
    _payOnline = s.payOnline;
  }

  String? _hoursKey(List<_DayHours>? h) =>
      h == null ? null : jsonEncode([for (final d in h) d.toJson()]);

  Map<String, Object?> get _patch {
    final s = _saved;
    if (s == null) return const {};
    return {
      if (_description.text.trim() != s.description) 'description': _description.text.trim(),
      if (_logoUrl != s.logoUrl) 'logo_url': _logoUrl,
      if (_coverUrl != s.coverUrl) 'cover_url': _coverUrl,
      if (_hoursKey(_hours) != _hoursKey(s.hours))
        'working_hours': _hours == null ? null : {'days': [for (final d in _hours!) d.toJson()]},
      if (_payCash != s.payCash || _payTerminal != s.payTerminal || _payOnline != s.payOnline)
        'payment_methods': {
          'cash': _payCash,
          'card_terminal': _payTerminal,
          'card_online': _payOnline,
          // Platforma qoidasi — doim yoqilgan (server `false` ni rad etadi).
          'ondex_wallet': true,
        },
    };
  }

  bool get _dirty => _patch.isNotEmpty;

  bool get _paymentsValid => _payCash || _payTerminal || _payOnline;

  void _toast(String m) {
    if (!mounted) return;
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(m)));
  }

  Future<void> _save() async {
    if (_saving || !_dirty) return;
    if (!_paymentsValid) {
      _toast('Kamida bitta to\'lov usuli yoqilgan bo\'lishi kerak');
      return;
    }
    final max = _saved?.descriptionMax ?? 500;
    if (_description.text.trim().runes.length > max) {
      _toast('Tavsif $max belgidan oshmasligi kerak');
      return;
    }
    setState(() => _saving = true);
    try {
      final json = await api.updateRestaurantSettings(_patch);
      if (!mounted) return;
      setState(() => _apply(_Settings.fromJson(json)));
      _toast('Sozlamalar saqlandi');
      widget.onSaved?.call();
    } on ApiException catch (e) {
      _toast(e.message);
    } catch (_) {
      _toast('Saqlab bo\'lmadi. Internet aloqasini tekshirib, qayta urinib ko\'ring.');
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  void _discard() {
    final s = _saved;
    if (s != null) setState(() => _apply(s));
  }

  Future<void> _pickImage({required bool logo}) async {
    FilePickerResult? result;
    try {
      result = await FilePicker.platform.pickFiles(
        type: FileType.custom,
        allowedExtensions: _imageExtensions,
        withData: true,
      );
    } catch (_) {
      _toast('Fayl tanlash oynasini ochib bo\'lmadi');
      return;
    }
    final file = result?.files.firstOrNull;
    final bytes = file?.bytes;
    if (file == null || bytes == null) return;
    if (bytes.length > _maxImageBytes) {
      _toast('Rasm 5 MB dan katta bo\'lmasin');
      return;
    }
    final ext = file.name.split('.').last.toLowerCase();
    if (!_imageExtensions.contains(ext)) {
      _toast('Faqat JPG, PNG yoki WEBP rasm');
      return;
    }
    setState(() => logo ? _uploadingLogo = true : _uploadingCover = true);
    try {
      final url = await api.uploadImage(bytes, file.name, type: logo ? 'logo' : 'cover');
      if (!mounted) return;
      setState(() => logo ? _logoUrl = url : _coverUrl = url);
    } on ApiException catch (e) {
      _toast(e.message);
    } catch (_) {
      _toast('Rasmni yuklab bo\'lmadi. Internet aloqasini tekshiring.');
    } finally {
      if (mounted) setState(() => logo ? _uploadingLogo = false : _uploadingCover = false);
    }
  }

  Future<void> _toggleOpen(bool value) async {
    setState(() => _openBusy = true);
    try {
      await widget.onOpenChanged(value);
      await _load(quiet: true);
    } finally {
      if (mounted) setState(() => _openBusy = false);
    }
  }

  Future<void> _editHours() async {
    final result = await showDialog<_HoursResult>(
      context: context,
      builder: (_) => _HoursDialog(initial: _hours ?? _defaultWeek()),
    );
    if (result == null || !mounted) return;
    setState(() => _hours = result.days);
  }

  Future<void> _editToday({required bool open}) async {
    final today = _todayIso();
    final hours = _hours;
    if (hours == null) {
      await _editHours();
      return;
    }
    final day = hours[today - 1];
    final picked = await showOnDexTimePicker(
      context: context,
      initial: _timeOf(open ? day.open : day.close),
      title: open ? 'Ishga tushish vaqti' : 'To\'xtatish vaqti',
    );
    if (picked == null || !mounted) return;
    setState(() {
      _hours = [
        for (final d in hours)
          d.day == today
              ? (open ? d.copyWith(open: _hhmm(picked), enabled: true) : d.copyWith(close: _hhmm(picked), enabled: true))
              : d,
      ];
    });
  }

  void _setDayEnabled(int day, bool enabled) {
    final hours = _hours;
    if (hours == null) return;
    setState(() => _hours = [for (final d in hours) d.day == day ? d.copyWith(enabled: enabled) : d]);
  }

  Future<void> _showDeleteInfo() => showDialog<void>(
        context: context,
        builder: (ctx) => AlertDialog(
          icon: const Icon(Icons.shield_outlined, color: OnDexColors.danger, size: 32),
          title: const Text('Restoranni o\'chirish'),
          content: const Text(
            'Xavfsizlik uchun restoranni faqat OnDex administratori o\'chira oladi: avval faol '
            'buyurtmalar, to\'lovlar va hisob-kitoblar yopilishi kerak.\n\n'
            'Restoranni o\'chirish uchun OnDex qo\'llab-quvvatlash xizmatiga murojaat qiling.',
          ),
          actions: [
            FilledButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Tushunarli')),
          ],
        ),
      );

  @override
  Widget build(BuildContext context) {
    if (_loading) return const Center(child: CircularProgressIndicator());
    final s = _saved;
    if (s == null) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.cloud_off_rounded, size: 40, color: OnDexColors.inkFaint),
            const SizedBox(height: 10),
            Text(_loadError ?? 'Sozlamalar topilmadi',
                textAlign: TextAlign.center, style: const TextStyle(fontSize: 14, color: OnDexColors.ink)),
            const SizedBox(height: 14),
            FilledButton(onPressed: _load, child: const Text('Qayta urinish')),
          ],
        ),
      );
    }

    return Padding(
      padding: const EdgeInsets.fromLTRB(24, 20, 24, 20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _SettingsHeader(dirty: _dirty, saving: _saving, onSave: _save, onDiscard: _discard),
          const SizedBox(height: 16),
          Expanded(
            child: SingleChildScrollView(
              child: LayoutBuilder(builder: (context, box) {
                final left = [
                  _InfoCard(
                    settings: s,
                    description: _description,
                    logoUrl: _logoUrl,
                    coverUrl: _coverUrl,
                    uploadingLogo: _uploadingLogo,
                    uploadingCover: _uploadingCover,
                    onPickLogo: () => _pickImage(logo: true),
                    onPickCover: () => _pickImage(logo: false),
                    onRemoveCover: _coverUrl.isEmpty ? null : () => setState(() => _coverUrl = ''),
                  ),
                  const SizedBox(height: 16),
                  _StatusCard(
                    logoUrl: _logoUrl,
                    open: widget.open,
                    openNow: s.openNow,
                    busy: _openBusy,
                    onChanged: _toggleOpen,
                    today: _hours?[_todayIso() - 1],
                    onEditOpen: () => _editToday(open: true),
                    onEditClose: () => _editToday(open: false),
                  ),
                ];
                final right = [
                  _HoursCard(
                    hours: _hours,
                    today: _todayIso(),
                    onEdit: _editHours,
                    onToggle: _setDayEnabled,
                    onSetup: () => setState(() => _hours = _defaultWeek()),
                  ),
                  const SizedBox(height: 16),
                  _PaymentsCard(
                    cash: _payCash,
                    terminal: _payTerminal,
                    online: _payOnline,
                    onlineAvailable: s.onlineAvailable,
                    valid: _paymentsValid,
                    onWalletLocked: () => _toast('OnDex Wallet doim yoqilgan — uni o\'chirib bo\'lmaydi'),
                    onCash: (v) => setState(() => _payCash = v),
                    onTerminal: (v) => setState(() => _payTerminal = v),
                    onOnline: (v) => setState(() => _payOnline = v),
                  ),
                  const SizedBox(height: 16),
                  const _NotificationsCard(),
                  const SizedBox(height: 16),
                  _DangerZone(onDelete: _showDeleteInfo),
                ];
                if (box.maxWidth >= 1100) {
                  return Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Expanded(flex: 11, child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: left)),
                      const SizedBox(width: 16),
                      Expanded(flex: 9, child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: right)),
                    ],
                  );
                }
                return Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [...left, const SizedBox(height: 16), ...right],
                );
              }),
            ),
          ),
        ],
      ),
    );
  }
}

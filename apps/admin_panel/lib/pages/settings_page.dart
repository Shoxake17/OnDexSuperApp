import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:ondex_support/ondex_support.dart';

import '../api.dart';

/// Sozlamalar — hozircha qo'llab-quvvatlash aloqa ma'lumotlari.
///
/// Kiritilgan telefon, Telegram va pochta BARCHA restoran panellarida
/// ("Yordam markazi" va "Chat markazi") ko'rinadi. Tekshiruv bu yerda
/// qulaylik uchun, HAQIQIY tekshiruv serverda (`internal/support`): server
/// rad etsa uning xabari shu formada ko'rsatiladi.
class SettingsPage extends StatefulWidget {
  const SettingsPage({super.key});

  @override
  State<SettingsPage> createState() => _SettingsPageState();
}

final _noControlChars = FilteringTextInputFormatter.deny(RegExp(r'[\x00-\x1F\x7F]'));

class _SettingsPageState extends State<SettingsPage> {
  final _form = GlobalKey<FormState>();
  final _phone = TextEditingController();
  final _hours = TextEditingController();
  final _telegram = TextEditingController();
  final _email = TextEditingController();
  final _note = TextEditingController();

  SupportContacts _saved = const SupportContacts();
  bool _loading = true;
  bool _saving = false;
  String? _loadError;
  String? _saveError;

  List<TextEditingController> get _all => [_phone, _hours, _telegram, _email, _note];

  @override
  void initState() {
    super.initState();
    for (final c in _all) {
      c.addListener(_changed);
    }
    _load();
  }

  @override
  void dispose() {
    for (final c in _all) {
      c.dispose();
    }
    super.dispose();
  }

  void _changed() {
    if (mounted) setState(() {});
  }

  Future<void> _load() async {
    try {
      final j = await api.supportContacts();
      if (!mounted) return;
      _apply(SupportContacts.fromJson(j));
      setState(() {
        _loading = false;
        _loadError = null;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _loadError = 'Sozlamalarni yuklab bo\'lmadi: $e';
      });
    }
  }

  void _apply(SupportContacts c) {
    _saved = c;
    _phone.text = c.phone.isEmpty ? '' : formatSupportPhone(c.phone);
    _hours.text = c.phoneHours;
    _telegram.text = c.telegram.isEmpty ? '' : '@${c.telegram}';
    _email.text = c.email;
    _note.text = c.emailNote;
  }

  /// Formadagi qiymat serverga ketadigan shaklda.
  SupportContacts get _draft => SupportContacts(
        phone: _phone.text.trim().isEmpty ? '' : normalizePhoneInput(_phone.text),
        phoneHours: _hours.text.trim(),
        telegram: normalizeTelegramInput(_telegram.text),
        email: _email.text.trim().toLowerCase(),
        emailNote: _note.text.trim(),
      );

  bool get _dirty {
    final d = _draft;
    return d.phone != _saved.phone ||
        d.phoneHours != _saved.phoneHours ||
        d.telegram != _saved.telegram ||
        d.email != _saved.email ||
        d.emailNote != _saved.emailNote;
  }

  String? _validatePhone(String? v) {
    if (v == null || v.trim().isEmpty) return null;
    return isSupportPhone(normalizePhoneInput(v)) ? null : 'Telefon raqam noto\'g\'ri (masalan: +998 90 123 45 67)';
  }

  String? _validateTelegram(String? v) {
    if (v == null || v.trim().isEmpty) return null;
    return isSupportTelegram(normalizeTelegramInput(v))
        ? null
        : 'Telegram nomi noto\'g\'ri: 5–32 lotin harf, raqam yoki "_" (masalan: @ondex_support)';
  }

  String? _validateEmail(String? v) {
    if (v == null || v.trim().isEmpty) return null;
    return isSupportEmail(v.trim()) ? null : 'Elektron pochta noto\'g\'ri (masalan: support@ondex.uz)';
  }

  String? _validateNote(String? v) =>
      (v ?? '').trim().runes.length > 60 ? '60 belgidan oshmasligi kerak' : null;

  Future<void> _save() async {
    if (!(_form.currentState?.validate() ?? false)) return;
    final d = _draft;
    setState(() {
      _saving = true;
      _saveError = null;
    });
    try {
      final j = await api.saveSupportContacts(d.toJson());
      if (!mounted) return;
      _apply(SupportContacts.fromJson(j));
      setState(() => _saving = false);
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Saqlandi — barcha restoran panellarida yangilandi')),
      );
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _saving = false;
        _saveError = e.message;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _saving = false;
        _saveError = 'Saqlab bo\'lmadi: $e';
      });
    }
  }

  void _reset() {
    _apply(_saved);
    setState(() => _saveError = null);
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    if (_loading) return const Center(child: CircularProgressIndicator());
    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text('Sozlamalar', style: theme.textTheme.headlineMedium),
          const SizedBox(height: 8),
          Text(
            'Bu yerda kiritilgan aloqa ma\'lumotlari barcha restoran panellarida — "Yordam markazi" va '
            '"Chat markazi" da ko\'rinadi.',
            style: theme.textTheme.bodySmall,
          ),
          const SizedBox(height: 16),
          if (_loadError != null)
            Padding(
              padding: const EdgeInsets.only(bottom: 16),
              child: _Banner(
                text: _loadError!,
                color: theme.colorScheme.error,
                action: TextButton(
                  onPressed: () {
                    setState(() => _loading = true);
                    _load();
                  },
                  child: const Text('Qayta urinish'),
                ),
              ),
            ),
          LayoutBuilder(builder: (context, box) {
            final form = _formCard(theme);
            final preview = _PreviewCard(draft: _draft);
            if (box.maxWidth >= 980) {
              return Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(flex: 3, child: form),
                  const SizedBox(width: 16),
                  Expanded(flex: 2, child: preview),
                ],
              );
            }
            return Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [form, const SizedBox(height: 16), preview],
            );
          }),
        ],
      ),
    );
  }

  Widget _formCard(ThemeData theme) {
    final updated = _saved.updatedAt;
    final canSave = _dirty && !_saving;
    return Card(
      margin: EdgeInsets.zero,
      elevation: 0,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(16),
        side: BorderSide(color: theme.colorScheme.outlineVariant),
      ),
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Form(
          key: _form,
          autovalidateMode: AutovalidateMode.onUserInteraction,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Row(
                children: [
                  Icon(Icons.support_agent_outlined, color: theme.colorScheme.primary),
                  const SizedBox(width: 8),
                  Expanded(child: Text('Qo\'llab-quvvatlash aloqa ma\'lumotlari', style: theme.textTheme.titleMedium)),
                ],
              ),
              const SizedBox(height: 4),
              Text('Har bir maydon ixtiyoriy — bo\'sh qoldirilgani panellarda ko\'rsatilmaydi.',
                  style: theme.textTheme.bodySmall),
              const SizedBox(height: 20),
              _field(
                key: 'settings-phone',
                controller: _phone,
                label: 'Telefon raqam',
                hint: '+998 90 123 45 67',
                icon: Icons.phone_outlined,
                validator: _validatePhone,
                maxLength: 20,
                keyboardType: TextInputType.phone,
                formatters: [FilteringTextInputFormatter.allow(RegExp(r'[0-9+\s()\-]'))],
              ),
              _field(
                key: 'settings-hours',
                controller: _hours,
                label: 'Ish vaqti',
                hint: '09:00 – 22:00 (har kuni)',
                icon: Icons.schedule_outlined,
                validator: _validateNote,
                maxLength: 60,
              ),
              _field(
                key: 'settings-telegram',
                controller: _telegram,
                label: 'Telegram (shaxsiy yoki qo\'llab-quvvatlash akkaunti)',
                hint: '@ondex_support',
                icon: Icons.send_outlined,
                validator: _validateTelegram,
                maxLength: 64,
              ),
              _field(
                key: 'settings-email',
                controller: _email,
                label: 'Elektron pochta (Gmail yoki boshqa)',
                hint: 'support@ondex.uz',
                icon: Icons.mail_outline,
                validator: _validateEmail,
                maxLength: 254,
                keyboardType: TextInputType.emailAddress,
              ),
              _field(
                key: 'settings-note',
                controller: _note,
                label: 'Pochta izohi',
                hint: '24/7 javob beramiz',
                icon: Icons.notes_outlined,
                validator: _validateNote,
                maxLength: 60,
              ),
              if (_saveError != null)
                Padding(
                  padding: const EdgeInsets.only(bottom: 12),
                  child: _Banner(key: const ValueKey('settings-error'), text: _saveError!, color: theme.colorScheme.error),
                ),
              Wrap(
                alignment: WrapAlignment.spaceBetween,
                crossAxisAlignment: WrapCrossAlignment.center,
                runSpacing: 8,
                spacing: 8,
                children: [
                  Text(
                    updated == null
                        ? 'Hali saqlanmagan'
                        : 'Oxirgi o\'zgarish: ${supportDate(updated)}, ${supportClock(updated)}',
                    style: theme.textTheme.bodySmall,
                  ),
                  Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      OutlinedButton(onPressed: canSave ? _reset : null, child: const Text('Bekor qilish')),
                      const SizedBox(width: 8),
                      FilledButton.icon(
                        key: const ValueKey('settings-save'),
                        onPressed: canSave ? _save : null,
                        icon: _saving
                            ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
                            : const Icon(Icons.save_outlined),
                        label: const Text('Saqlash'),
                      ),
                    ],
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _field({
    required String key,
    required TextEditingController controller,
    required String label,
    required String hint,
    required IconData icon,
    required String? Function(String?) validator,
    required int maxLength,
    TextInputType? keyboardType,
    List<TextInputFormatter> formatters = const [],
  }) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 14),
      child: TextFormField(
        key: ValueKey(key),
        controller: controller,
        validator: validator,
        keyboardType: keyboardType,
        enabled: !_saving,
        inputFormatters: [_noControlChars, LengthLimitingTextInputFormatter(maxLength), ...formatters],
        decoration: InputDecoration(
          labelText: label,
          hintText: hint,
          prefixIcon: Icon(icon),
          border: OutlineInputBorder(borderRadius: BorderRadius.circular(12)),
        ),
      ),
    );
  }
}

class _Banner extends StatelessWidget {
  const _Banner({super.key, required this.text, required this.color, this.action});

  final String text;
  final Color color;
  final Widget? action;

  @override
  Widget build(BuildContext context) => Container(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
        decoration: BoxDecoration(
          color: color.withAlpha(20),
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: color.withAlpha(80)),
        ),
        child: Row(
          children: [
            Icon(Icons.error_outline, color: color, size: 20),
            const SizedBox(width: 10),
            Expanded(child: Text(text, style: TextStyle(color: color))),
            if (action != null) action!,
          ],
        ),
      );
}

/// Restoran panelidagi "Biz bilan bog'laning" bloki qanday ko'rinishi.
class _PreviewCard extends StatelessWidget {
  const _PreviewCard({required this.draft});

  final SupportContacts draft;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final rows = <Widget>[
      if (draft.hasPhone)
        _PreviewRow(
            icon: Icons.phone, color: const Color(0xFFF2650F), title: formatSupportPhone(draft.phone),
            subtitle: draft.phoneHours, action: 'Qo\'ng\'iroq qilish'),
      if (draft.hasTelegram)
        _PreviewRow(
            icon: Icons.send, color: const Color(0xFF229ED9), title: 'Telegram',
            subtitle: '@${draft.telegram}', action: 'O\'tish'),
      if (draft.hasEmail)
        _PreviewRow(
            icon: Icons.mail, color: const Color(0xFF7C5CFC), title: draft.email,
            subtitle: draft.emailNote, action: 'Xat yuborish'),
    ];
    return Card(
      margin: EdgeInsets.zero,
      elevation: 0,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(16),
        side: BorderSide(color: theme.colorScheme.outlineVariant),
      ),
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text('Panelda shunday ko\'rinadi', style: theme.textTheme.titleMedium),
            const SizedBox(height: 4),
            Text('Restoran paneli → Yordam markazi → "Biz bilan bog\'laning"', style: theme.textTheme.bodySmall),
            const SizedBox(height: 16),
            if (rows.isEmpty)
              Text(
                'Hech narsa kiritilmagan — panellarda "Aloqa ma\'lumotlari hali kiritilmagan" deb ko\'rinadi.',
                key: const ValueKey('settings-preview-empty'),
                style: theme.textTheme.bodyMedium?.copyWith(color: theme.colorScheme.onSurfaceVariant),
              )
            else
              for (var i = 0; i < rows.length; i++) ...[if (i > 0) const SizedBox(height: 10), rows[i]],
          ],
        ),
      ),
    );
  }
}

class _PreviewRow extends StatelessWidget {
  const _PreviewRow({
    required this.icon,
    required this.color,
    required this.title,
    required this.subtitle,
    required this.action,
  });

  final IconData icon;
  final Color color;
  final String title;
  final String subtitle;
  final String action;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Row(
        children: [
          CircleAvatar(radius: 18, backgroundColor: color, child: Icon(icon, color: Colors.white, size: 18)),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, maxLines: 1, overflow: TextOverflow.ellipsis, style: theme.textTheme.titleSmall),
                if (subtitle.isNotEmpty)
                  Text(subtitle, maxLines: 1, overflow: TextOverflow.ellipsis, style: theme.textTheme.bodySmall),
              ],
            ),
          ),
          const SizedBox(width: 8),
          Chip(label: Text(action), visualDensity: VisualDensity.compact),
        ],
      ),
    );
  }
}

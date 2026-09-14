part of '../restaurant_settings_page.dart';

// ─── Umumiy bloklar ───────────────────────────────────────────────────

class _SettingsHeader extends StatelessWidget {
  const _SettingsHeader({
    required this.dirty,
    required this.saving,
    required this.onSave,
    required this.onDiscard,
  });

  final bool dirty;
  final bool saving;
  final VoidCallback onSave;
  final VoidCallback onDiscard;

  @override
  Widget build(BuildContext context) {
    const title = Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text('Restoran sozlamalari',
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(fontSize: 24, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
        SizedBox(height: 2),
        Text('Restoraningiz ma\'lumotlari va sozlamalarini boshqaring',
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(fontSize: 13.5, color: OnDexColors.inkDim)),
      ],
    );
    final actions = Wrap(
      spacing: 10,
      runSpacing: 8,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        if (dirty) ...[
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
            decoration: BoxDecoration(color: OnDexColors.amberBg, borderRadius: BorderRadius.circular(999)),
            child: const Text('Saqlanmagan o\'zgarishlar',
                style: TextStyle(fontSize: 12, fontWeight: FontWeight.w700, color: OnDexColors.amber)),
          ),
          TextButton(
            onPressed: saving ? null : onDiscard,
            style: TextButton.styleFrom(foregroundColor: OnDexColors.inkDim, minimumSize: const Size(0, 44)),
            child: const Text('Bekor qilish'),
          ),
        ],
        FilledButton.icon(
          key: const ValueKey('settings-save'),
          onPressed: dirty && !saving ? onSave : null,
          style: FilledButton.styleFrom(
            backgroundColor: OnDexColors.primary,
            foregroundColor: Colors.white,
            minimumSize: const Size(0, 46),
            padding: const EdgeInsets.symmetric(horizontal: 20),
            shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
            textStyle: const TextStyle(fontSize: 14, fontWeight: FontWeight.w700),
          ),
          icon: saving
              ? const SizedBox(
                  width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white))
              : const Icon(Icons.save_outlined, size: 19),
          label: const Text('Saqlash'),
        ),
      ],
    );
    return LayoutBuilder(builder: (context, box) {
      if (box.maxWidth >= 760) {
        return Row(children: [const Expanded(child: title), const SizedBox(width: 12), actions]);
      }
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [title, const SizedBox(height: 12), actions],
      );
    });
  }
}

class _SettingsCard extends StatelessWidget {
  const _SettingsCard({
    required this.title,
    required this.subtitle,
    required this.child,
    this.icon,
    this.trailing,
  });

  final String title;
  final String subtitle;
  final IconData? icon;
  final Widget? trailing;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.fromLTRB(20, 18, 20, 20),
      decoration: BoxDecoration(
        color: OnDexColors.cardBg,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              if (icon != null) ...[
                Container(
                  width: 38,
                  height: 38,
                  decoration:
                      BoxDecoration(color: OnDexColors.primaryTint, borderRadius: BorderRadius.circular(11)),
                  child: Icon(icon, size: 20, color: OnDexColors.primary),
                ),
                const SizedBox(width: 12),
              ],
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(title,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
                    const SizedBox(height: 2),
                    Text(subtitle,
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(fontSize: 12.5, color: OnDexColors.inkDim)),
                  ],
                ),
              ),
              if (trailing != null) ...[const SizedBox(width: 8), trailing!],
            ],
          ),
          const SizedBox(height: 16),
          child,
        ],
      ),
    );
  }
}

class _SmallButton extends StatelessWidget {
  const _SmallButton({super.key, required this.icon, required this.label, required this.onPressed});

  final IconData icon;
  final String label;
  final VoidCallback? onPressed;

  @override
  Widget build(BuildContext context) {
    return OutlinedButton.icon(
      onPressed: onPressed,
      style: OutlinedButton.styleFrom(
        foregroundColor: OnDexColors.ink,
        side: const BorderSide(color: OnDexColors.cardBorder),
        minimumSize: const Size(0, 36),
        padding: const EdgeInsets.symmetric(horizontal: 12),
        tapTargetSize: MaterialTapTargetSize.shrinkWrap,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
        textStyle: const TextStyle(fontSize: 12.5, fontWeight: FontWeight.w600),
      ),
      icon: Icon(icon, size: 16),
      label: Text(label, maxLines: 1, overflow: TextOverflow.ellipsis),
    );
  }
}

/// Faqat ko'rish uchun maydon: matn kiritgich EMAS — tahrirlab
/// bo'ladigandek ko'rinmasin (qoida serverda ham bor).
class _LockedField extends StatelessWidget {
  const _LockedField({required this.label, required this.value, this.icon});

  final String label;
  final String value;
  final IconData? icon;

  @override
  Widget build(BuildContext context) {
    return Column(
      key: ValueKey('locked-$label'),
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text.rich(
          TextSpan(children: [
            TextSpan(text: label),
            const TextSpan(text: ' *', style: TextStyle(color: OnDexColors.danger)),
          ]),
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: const TextStyle(fontSize: 12.5, fontWeight: FontWeight.w600, color: OnDexColors.ink),
        ),
        const SizedBox(height: 6),
        Tooltip(
          message: 'Faqat OnDex administratori o\'zgartira oladi',
          waitDuration: const Duration(milliseconds: 300),
          child: Container(
            height: 46,
            padding: const EdgeInsets.symmetric(horizontal: 12),
            decoration: BoxDecoration(
              color: const Color(0xFFFAF6F0),
              borderRadius: BorderRadius.circular(10),
              border: Border.all(color: OnDexColors.cardBorder),
            ),
            child: Row(
              children: [
                if (icon != null) ...[
                  Icon(icon, size: 17, color: OnDexColors.inkDim),
                  const SizedBox(width: 8),
                ],
                Expanded(
                  child: Text(value.isEmpty ? 'Belgilanmagan' : value,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                          fontSize: 13.5, color: value.isEmpty ? OnDexColors.inkFaint : OnDexColors.ink)),
                ),
                const SizedBox(width: 6),
                const Icon(Icons.lock_outline_rounded, size: 15, color: OnDexColors.inkFaint),
              ],
            ),
          ),
        ),
      ],
    );
  }
}

class _ImageBox extends StatelessWidget {
  const _ImageBox({required this.url, required this.uploading, required this.placeholder, this.width, this.height});

  final String url;
  final bool uploading;
  final IconData placeholder;
  final double? width;
  final double? height;

  @override
  Widget build(BuildContext context) {
    final empty = Center(child: Icon(placeholder, size: 42, color: OnDexColors.inkFaint));
    return Container(
      width: width,
      height: height,
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(
        color: OnDexColors.pageBg,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      child: Stack(
        fit: StackFit.expand,
        children: [
          if (url.isEmpty)
            empty
          else
            Image.network(fullImageUrl(url), fit: BoxFit.cover, errorBuilder: (_, __, ___) => empty),
          if (uploading)
            const ColoredBox(
              color: Color(0x88FFFFFF),
              child: Center(child: SizedBox(width: 26, height: 26, child: CircularProgressIndicator(strokeWidth: 2.5))),
            ),
        ],
      ),
    );
  }
}

// ─── Restoran ma'lumotlari ────────────────────────────────────────────

class _InfoCard extends StatelessWidget {
  const _InfoCard({
    required this.settings,
    required this.description,
    required this.logoUrl,
    required this.coverUrl,
    required this.uploadingLogo,
    required this.uploadingCover,
    required this.onPickLogo,
    required this.onPickCover,
    required this.onRemoveCover,
  });

  final _Settings settings;
  final TextEditingController description;
  final String logoUrl;
  final String coverUrl;
  final bool uploadingLogo;
  final bool uploadingCover;
  final VoidCallback onPickLogo;
  final VoidCallback onPickCover;
  final VoidCallback? onRemoveCover;

  static const _hint = TextStyle(fontSize: 11.5, color: OnDexColors.inkFaint);

  @override
  Widget build(BuildContext context) {
    final s = settings;
    final logo = Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        _ImageBox(url: logoUrl, uploading: uploadingLogo, placeholder: Icons.storefront_rounded, width: 150, height: 150),
        const SizedBox(height: 10),
        _SmallButton(
          key: const ValueKey('pick-logo'),
          icon: Icons.photo_camera_outlined,
          label: 'Rasmni o\'zgartirish',
          onPressed: uploadingLogo ? null : onPickLogo,
        ),
        const SizedBox(height: 6),
        const Text('Tavsiya etilgan o\'lcham: 400x400', textAlign: TextAlign.center, style: _hint),
      ],
    );

    Widget fields(bool wide) {
      final name = _LockedField(label: 'Restoran nomi', value: s.name);
      final kind = _LockedField(label: 'Restoran turi', value: s.kindTitle);
      final phone = _LockedField(label: 'Telefon raqam', value: s.phone, icon: Icons.phone_outlined);
      final address = _LockedField(label: 'Manzil', value: s.address, icon: Icons.location_on_outlined);
      Widget pair(Widget a, Widget b) => wide
          ? Row(crossAxisAlignment: CrossAxisAlignment.start, children: [Expanded(child: a), const SizedBox(width: 14), Expanded(child: b)])
          : Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [a, const SizedBox(height: 14), b]);
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          pair(name, kind),
          const SizedBox(height: 14),
          pair(phone, address),
          const SizedBox(height: 14),
          const Text('Tavsif',
              style: TextStyle(fontSize: 12.5, fontWeight: FontWeight.w600, color: OnDexColors.ink)),
          const SizedBox(height: 6),
          TextField(
            key: const ValueKey('settings-description'),
            controller: description,
            minLines: 4,
            maxLines: 6,
            maxLength: s.descriptionMax,
            style: const TextStyle(fontSize: 13.5),
            decoration: InputDecoration(
              hintText: 'Mijozlarga restoraningiz haqida qisqacha yozing',
              hintStyle: const TextStyle(fontSize: 13, color: OnDexColors.inkFaint),
              filled: true,
              fillColor: OnDexColors.cardBg,
              contentPadding: const EdgeInsets.all(12),
              enabledBorder: OutlineInputBorder(
                borderRadius: BorderRadius.circular(10),
                borderSide: const BorderSide(color: OnDexColors.cardBorder),
              ),
              focusedBorder: OutlineInputBorder(
                borderRadius: BorderRadius.circular(10),
                borderSide: const BorderSide(color: OnDexColors.primary),
              ),
            ),
          ),
        ],
      );
    }

    return _SettingsCard(
      title: 'Restoran ma\'lumotlari',
      subtitle: 'Asosiy ma\'lumotlaringizni yangilang',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          LayoutBuilder(builder: (context, box) {
            if (box.maxWidth >= 560) {
              return Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  SizedBox(width: 170, child: logo),
                  const SizedBox(width: 18),
                  // Nomi|Turi va Telefon|Manzil — har doim ikkitadan bir qatorda.
                  Expanded(child: fields(true)),
                ],
              );
            }
            return Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              // Juda tor oynadagina bittadan (telefon kengligi).
              children: [Center(child: logo), const SizedBox(height: 16), fields(box.maxWidth >= 360)],
            );
          }),
          const SizedBox(height: 16),
          const Text('Muqova rasmi',
              style: TextStyle(fontSize: 12.5, fontWeight: FontWeight.w600, color: OnDexColors.ink)),
          const SizedBox(height: 6),
          AspectRatio(
            aspectRatio: 2.4,
            child: _ImageBox(url: coverUrl, uploading: uploadingCover, placeholder: Icons.panorama_outlined),
          ),
          const SizedBox(height: 10),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              _SmallButton(
                key: const ValueKey('pick-cover'),
                icon: Icons.photo_camera_outlined,
                label: coverUrl.isEmpty ? 'Muqova qo\'shish' : 'Muqovani o\'zgartirish',
                onPressed: uploadingCover ? null : onPickCover,
              ),
              if (onRemoveCover != null)
                TextButton(
                  onPressed: onRemoveCover,
                  style: TextButton.styleFrom(foregroundColor: OnDexColors.danger),
                  child: const Text('Olib tashlash'),
                ),
              const Text('Tavsiya etilgan o\'lcham: 1200x500 · mijoz ilovasida restoran sahifasi tepasida',
                  style: _hint),
            ],
          ),
          const SizedBox(height: 14),
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(color: OnDexColors.pageBg, borderRadius: BorderRadius.circular(10)),
            child: const Row(
              crossAxisAlignment: CrossAxisAlignment.start,
            ),
          ),
        ],
      ),
    );
  }
}

// ─── Holat va bugungi ish vaqti ───────────────────────────────────────

class _StatusCard extends StatelessWidget {
  const _StatusCard({
    required this.open,
    required this.openNow,
    required this.busy,
    required this.onChanged,
    required this.today,
    required this.onEditOpen,
    required this.onEditClose,
    required this.logoUrl,
  });

  /// Restoran logosi (ikonka emas); bo'sh bo'lsa — do'kon belgisi.
  final String logoUrl;
  final bool open;
  final bool openNow;
  final bool busy;
  final ValueChanged<bool> onChanged;
  final _DayHours? today;
  final VoidCallback onEditOpen;
  final VoidCallback onEditClose;

  @override
  Widget build(BuildContext context) {
    final color = !open ? OnDexColors.danger : (openNow ? OnDexColors.success : OnDexColors.amber);
    final bg = !open ? OnDexColors.dangerBg : (openNow ? OnDexColors.successBg : OnDexColors.amberBg);
    final subtitle = !open
        ? 'Yangi buyurtmalar qabul qilinmaydi'
        : (openNow
            ? 'Buyurtmalar qabul qilinmoqda'
            : 'Hozir ish vaqti emas — buyurtmalar ish vaqtida qabul qilinadi');
    final day = today;
    final noHours = day == null;
    final off = day != null && !day.enabled;

    Widget tile(Key key, String label, String value, VoidCallback onTap) => Material(
          color: const Color(0xFFFAF6F0),
          borderRadius: BorderRadius.circular(14),
          child: InkWell(
            key: key,
            borderRadius: BorderRadius.circular(14),
            onTap: onTap,
            child: Container(
              padding: const EdgeInsets.fromLTRB(14, 14, 8, 14),
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(14),
                border: Border.all(color: OnDexColors.cardBorder),
              ),
              child: Row(
                children: [
                  Container(
                    width: 42,
                    height: 42,
                    decoration: BoxDecoration(color: OnDexColors.primaryTint, borderRadius: BorderRadius.circular(12)),
                    child: const Icon(Icons.schedule_rounded, color: OnDexColors.primary, size: 22),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(label,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: const TextStyle(fontSize: 12, color: OnDexColors.inkDim)),
                        const SizedBox(height: 2),
                        FittedBox(
                          fit: BoxFit.scaleDown,
                          alignment: Alignment.centerLeft,
                          child: Text(value,
                              style: const TextStyle(fontSize: 22, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
                        ),
                      ],
                    ),
                  ),
                  const Icon(Icons.chevron_right_rounded, color: OnDexColors.inkFaint),
                ],
              ),
            ),
          ),
        );

    final openTile = tile(const ValueKey('today-open'), 'Ishga tushish vaqti',
        noHours || off ? '—' : day.open, onEditOpen);
    final closeTile = tile(const ValueKey('today-close'), 'To\'xtatish vaqti',
        noHours || off ? '—' : day.close, onEditClose);

    return _SettingsCard(
      title: 'Restoran holati',
      subtitle: 'Restoran hozirgi faoliyat holati',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
            decoration: BoxDecoration(color: bg, borderRadius: BorderRadius.circular(14)),
            child: Row(
              children: [
                Switch(
                  key: const ValueKey('settings-open'),
                  value: open,
                  onChanged: busy ? null : onChanged,
                  activeTrackColor: OnDexColors.success,
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(open ? 'Restoran ochiq' : 'Restoran yopiq',
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(fontSize: 16, fontWeight: FontWeight.w800, color: color)),
                      const SizedBox(height: 2),
                      Text(subtitle,
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(fontSize: 12.5, color: OnDexColors.inkDim)),
                    ],
                  ),
                ),
                const SizedBox(width: 10),
                Container(
                  key: const ValueKey('status-logo'),
                  width: 58,
                  height: 58,
                  clipBehavior: Clip.antiAlias,
                  decoration: BoxDecoration(
                    color: Colors.white,
                    borderRadius: BorderRadius.circular(14),
                    border: Border.all(color: color.withValues(alpha: 0.35), width: 1.5),
                  ),
                  child: logoUrl.isEmpty
                      ? Icon(Icons.storefront_rounded, size: 30, color: color.withValues(alpha: 0.45))
                      : Image.network(
                          fullImageUrl(logoUrl),
                          fit: BoxFit.cover,
                          errorBuilder: (_, __, ___) =>
                              Icon(Icons.storefront_rounded, size: 30, color: color.withValues(alpha: 0.45)),
                        ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 6),
          const Text('O\'zgarish darhol qo\'llanadi', style: TextStyle(fontSize: 11.5, color: OnDexColors.inkFaint)),
          const SizedBox(height: 16),
          const Text('Ishga tushirish / To\'xtatish',
              style: TextStyle(fontSize: 15, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
          const SizedBox(height: 2),
          Text(
              noHours
                  ? 'Ish vaqti belgilanmagan — bosib belgilang'
                  : (off ? 'Bugun dam olish kuni' : 'Bugungi ish vaqti'),
              style: const TextStyle(fontSize: 12.5, color: OnDexColors.inkDim)),
          const SizedBox(height: 12),
          LayoutBuilder(builder: (context, box) {
            if (box.maxWidth >= 460) {
              return Row(children: [Expanded(child: openTile), const SizedBox(width: 12), Expanded(child: closeTile)]);
            }
            return Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [openTile, const SizedBox(height: 10), closeTile],
            );
          }),
        ],
      ),
    );
  }
}

// ─── Ish vaqti ────────────────────────────────────────────────────────

class _HoursCard extends StatelessWidget {
  const _HoursCard({
    required this.hours,
    required this.today,
    required this.onEdit,
    required this.onToggle,
    required this.onSetup,
  });

  final List<_DayHours>? hours;
  final int today;
  final VoidCallback onEdit;
  final void Function(int day, bool enabled) onToggle;
  final VoidCallback onSetup;

  @override
  Widget build(BuildContext context) {
    final list = hours;
    return _SettingsCard(
      icon: Icons.schedule_rounded,
      title: 'Ish vaqti',
      subtitle: 'Haftalik ish vaqtini sozlang',
      trailing: _SmallButton(
        key: const ValueKey('hours-edit'),
        icon: Icons.edit_outlined,
        label: 'Tahrirlash',
        onPressed: onEdit,
      ),
      child: list == null
          ? Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text(
                  'Ish vaqti belgilanmagan — "Restoran ochiq" bo\'lganda buyurtmalar kun bo\'yi qabul qilinadi.',
                  style: TextStyle(fontSize: 13, color: OnDexColors.inkDim),
                ),
                const SizedBox(height: 10),
                OutlinedButton.icon(
                  key: const ValueKey('hours-setup'),
                  onPressed: onSetup,
                  icon: const Icon(Icons.add_rounded, size: 18),
                  label: const Text('Ish vaqtini belgilash'),
                ),
              ],
            )
          : Column(
              children: [
                for (final d in list) ...[
                  if (d.day > 1) Divider(height: 1, color: OnDexColors.cardBorder.withValues(alpha: 0.6)),
                  SizedBox(
                    height: 42,
                    child: Row(
                      children: [
                        SizedBox(
                          width: 108,
                          child: Text(_dayNames[d.day - 1],
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: TextStyle(
                                  fontSize: 13.5,
                                  fontWeight: d.day == today ? FontWeight.w800 : FontWeight.w500,
                                  color: OnDexColors.ink)),
                        ),
                        Expanded(
                          child: Text(d.enabled ? d.rangeText : 'Dam olish kuni',
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: TextStyle(
                                  fontSize: 13, color: d.enabled ? OnDexColors.ink : OnDexColors.inkFaint)),
                        ),
                        Switch(
                          key: ValueKey('day-${d.day}'),
                          value: d.enabled,
                          onChanged: (v) => onToggle(d.day, v),
                          activeTrackColor: OnDexColors.success,
                        ),
                      ],
                    ),
                  ),
                ],
              ],
            ),
    );
  }
}

// ─── To'lov usullari ──────────────────────────────────────────────────

/// Kalitlar to'g'ridan-to'g'ri bosiladi, lekin o'zgarish faqat "Saqlash"
/// bosilgach kuchga kiradi (sarlavhada "Saqlanmagan o'zgarishlar" chiqadi).
class _PaymentsCard extends StatelessWidget {
  const _PaymentsCard({
    required this.cash,
    required this.terminal,
    required this.online,
    required this.onlineAvailable,
    required this.valid,
    required this.onWalletLocked,
    required this.onCash,
    required this.onTerminal,
    required this.onOnline,
  });

  final bool cash;
  final bool terminal;
  final bool online;
  final bool onlineAvailable;
  final bool valid;
  final VoidCallback onWalletLocked;
  final ValueChanged<bool> onCash;
  final ValueChanged<bool> onTerminal;
  final ValueChanged<bool> onOnline;

  @override
  Widget build(BuildContext context) {
    final tiles = [
      _PayTile(
        switchKey: const ValueKey('pay-cash'),
        leading: const _PayIcon(icon: Icons.payments_outlined, color: OnDexColors.success, bg: OnDexColors.successBg),
        title: 'Naqd pul',
        caption: 'Kuryer yoki ofitsiantga',
        value: cash,
        onChanged: onCash,
      ),
      _PayTile(
        switchKey: const ValueKey('pay-terminal'),
        leading: const _PayIcon(icon: Icons.point_of_sale_rounded, color: OnDexColors.primary, bg: OnDexColors.primaryTint),
        title: 'Karta (Terminal)',
        caption: 'Joyida, terminal orqali',
        value: terminal,
        onChanged: onTerminal,
      ),
      _PayTile(
        switchKey: const ValueKey('pay-online'),
        leading: const _PayIcon(icon: Icons.credit_card_rounded, color: OnDexColors.info, bg: OnDexColors.infoBg),
        title: 'Onlayn karta',
        caption: onlineAvailable ? 'Uzcard, Humo — oldindan' : 'Tizimda hali ulanmagan',
        value: online,
        // Ulanmagan tizimni YOQIB bo'lmaydi (server ham rad etadi), faqat o'chirish mumkin.
        onChanged: onlineAvailable || online ? onOnline : null,
      ),
      // Click va Payme — alohida; tizimga hali ulanmagan, yoqib bo'lmaydi.
      const _PayTile(
        switchKey: ValueKey('pay-click'),
        leading: _BrandLogo(asset: 'assets/payments/click.png'),
        title: 'Click',
        caption: 'Tez orada',
        value: false,
        onChanged: null,
      ),
      const _PayTile(
        switchKey: ValueKey('pay-payme'),
        leading: _BrandLogo(asset: 'assets/payments/payme.png'),
        title: 'Payme',
        caption: 'Tez orada',
        value: false,
        onChanged: null,
      ),
      // OnDex Wallet — platforma qoidasi: DOIM yoqilgan. Kalit faol ko'rinadi,
      // bosilsa holati o'zgarmaydi, faqat sababi aytiladi.
      _PayTile(
        switchKey: const ValueKey('pay-wallet'),
        leading: const _BrandLogo(asset: 'assets/ondex.png'),
        title: 'OnDex Wallet',
        caption: 'Keshbek bilan to\'lov',
        titleTrailing: const Icon(Icons.lock_outline_rounded, size: 13, color: OnDexColors.inkFaint),
        value: true,
        onChanged: (_) => onWalletLocked(),
      ),
    ];
    return _SettingsCard(
      icon: Icons.account_balance_wallet_outlined,
      title: 'To\'lov usullari',
      subtitle: 'Qanday to\'lov usullarini qabul qilishni tanlang',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          LayoutBuilder(builder: (context, box) {
            const gap = 10.0;
            final columns = box.maxWidth >= 440 ? 2 : 1;
            final width = ((box.maxWidth - gap * (columns - 1)) / columns).floorToDouble();
            return Wrap(
              spacing: gap,
              runSpacing: gap,
              children: [for (final t in tiles) SizedBox(width: width, child: t)],
            );
          }),
          if (!valid) ...[
            const SizedBox(height: 10),
            const Text('Kamida bitta to\'lov usuli yoqilgan bo\'lishi kerak',
                style: TextStyle(fontSize: 12.5, color: OnDexColors.danger, fontWeight: FontWeight.w600)),
          ],
        ],
      ),
    );
  }
}

class _PayIcon extends StatelessWidget {
  const _PayIcon({required this.icon, required this.color, required this.bg});

  final IconData icon;
  final Color color;
  final Color bg;

  @override
  Widget build(BuildContext context) => Container(
        width: 36,
        height: 36,
        decoration: BoxDecoration(color: bg, borderRadius: BorderRadius.circular(10)),
        child: Icon(icon, size: 19, color: color),
      );
}

/// Brend logotipi — ikonka EMAS, haqiqiy rasm (mijoz ilovasidagi fayl).
class _BrandLogo extends StatelessWidget {
  const _BrandLogo({required this.asset});

  final String asset;

  @override
  Widget build(BuildContext context) => Container(
        width: 30,
        height: 30,
        padding: const EdgeInsets.all(3),
        decoration: BoxDecoration(
          color: Colors.white,
          borderRadius: BorderRadius.circular(8),
          border: Border.all(color: OnDexColors.cardBorder),
        ),
        child: Image.asset(asset, fit: BoxFit.contain),
      );
}

class _PayTile extends StatelessWidget {
  const _PayTile({
    required this.switchKey,
    required this.leading,
    required this.title,
    required this.caption,
    required this.value,
    required this.onChanged,
    this.titleTrailing,
  });

  final Key switchKey;
  final Widget leading;
  final Widget? titleTrailing;
  final String title;
  final String caption;
  final bool value;
  final ValueChanged<bool>? onChanged;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.fromLTRB(12, 10, 4, 10),
      decoration: BoxDecoration(
        color: OnDexColors.cardBg,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      child: Row(
        children: [
          leading,
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Row(
                  children: [
                    Flexible(
                      child: Text(title,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(fontSize: 13.5, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
                    ),
                    if (titleTrailing != null) ...[const SizedBox(width: 4), titleTrailing!],
                  ],
                ),
                Text(caption,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(fontSize: 11.5, color: OnDexColors.inkDim)),
              ],
            ),
          ),
          Switch(
            key: switchKey,
            value: value,
            onChanged: onChanged,
            activeTrackColor: OnDexColors.success,
          ),
        ],
      ),
    );
  }
}

// ─── Bildirishnomalar ─────────────────────────────────────────────────

class _NotificationsCard extends StatelessWidget {
  const _NotificationsCard();

  @override
  Widget build(BuildContext context) {
    return const _SettingsCard(
      icon: Icons.notifications_active_outlined,
      title: 'Bildirishnomalar',
      subtitle: 'Shu kompyuterdagi panel uchun — darhol qo\'llanadi',
      child: Column(
        children: [
          _PrefRow(
            switchKey: ValueKey('pref-sound'),
            icon: Icons.volume_up_outlined,
            title: 'Yangi buyurtma ovozi',
            caption: 'Qabul qilinmagan buyurtma bo\'lsa qo\'ng\'iroq chalinadi',
            kind: _PrefKind.sound,
          ),
          _PrefRow(
            switchKey: ValueKey('pref-banner'),
            icon: Icons.chat_bubble_outline_rounded,
            title: 'Yangi buyurtma xabari',
            caption: 'Buyurtmalar sahifasida ekranda ko\'rinadi',
            kind: _PrefKind.banner,
          ),
          _PrefRow(
            switchKey: ValueKey('pref-courier'),
            icon: Icons.delivery_dining_outlined,
            title: 'Kuryer ogohlantirishlari',
            caption: 'Kuryer qidirishda xato bo\'lsa xabar beriladi',
            kind: _PrefKind.courier,
          ),
        ],
      ),
    );
  }
}

enum _PrefKind { sound, banner, courier }

class _PrefRow extends StatelessWidget {
  const _PrefRow({
    required this.switchKey,
    required this.icon,
    required this.title,
    required this.caption,
    required this.kind,
  });

  final Key switchKey;
  final IconData icon;
  final String title;
  final String caption;
  final _PrefKind kind;

  ValueNotifier<bool> get _notifier => switch (kind) {
        _PrefKind.sound => PanelPrefs.newOrderSound,
        _PrefKind.banner => PanelPrefs.newOrderBanner,
        _PrefKind.courier => PanelPrefs.courierAlerts,
      };

  @override
  Widget build(BuildContext context) {
    return ValueListenableBuilder<bool>(
      valueListenable: _notifier,
      builder: (context, value, _) => Padding(
        padding: const EdgeInsets.symmetric(vertical: 4),
        child: Row(
          children: [
            Container(
              width: 34,
              height: 34,
              decoration: BoxDecoration(color: OnDexColors.primaryTint, borderRadius: BorderRadius.circular(10)),
              child: Icon(icon, size: 18, color: OnDexColors.primary),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(title,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(fontSize: 13.5, fontWeight: FontWeight.w600, color: OnDexColors.ink)),
                  Text(caption,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(fontSize: 11.5, color: OnDexColors.inkDim)),
                ],
              ),
            ),
            Switch(
              key: switchKey,
              value: value,
              activeTrackColor: OnDexColors.success,
              onChanged: (v) async {
                await PanelPrefs.set(_notifier, v);
                if (kind == _PrefKind.sound) await RingSound.refresh();
              },
            ),
          ],
        ),
      ),
    );
  }
}

// ─── Xavfli zona ──────────────────────────────────────────────────────

class _DangerZone extends StatelessWidget {
  const _DangerZone({required this.onDelete});

  final VoidCallback onDelete;

  @override
  Widget build(BuildContext context) {
    final button = OutlinedButton.icon(
      key: const ValueKey('danger-delete'),
      onPressed: onDelete,
      style: OutlinedButton.styleFrom(
        foregroundColor: OnDexColors.danger,
        side: BorderSide(color: OnDexColors.danger.withValues(alpha: 0.5)),
        backgroundColor: Colors.white,
        minimumSize: const Size(0, 42),
        padding: const EdgeInsets.symmetric(horizontal: 14),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
        textStyle: const TextStyle(fontSize: 13, fontWeight: FontWeight.w700),
      ),
      icon: const Icon(Icons.delete_outline_rounded, size: 18),
      label: const Text('Restoranni o\'chirish', maxLines: 1, overflow: TextOverflow.ellipsis),
    );
    const text = Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text('Xavfli zona',
            style: TextStyle(fontSize: 15, fontWeight: FontWeight.w800, color: OnDexColors.danger)),
        SizedBox(height: 2),
        Text('Diqqat: bu amalni qayta tiklab bo\'lmaydi',
            style: TextStyle(fontSize: 12.5, color: OnDexColors.inkDim)),
      ],
    );
    return Container(
      padding: const EdgeInsets.all(18),
      decoration: BoxDecoration(
        color: OnDexColors.dangerBg.withValues(alpha: 0.6),
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: OnDexColors.danger.withValues(alpha: 0.35)),
      ),
      child: LayoutBuilder(builder: (context, box) {
        const icon = Icon(Icons.warning_amber_rounded, color: OnDexColors.danger, size: 26);
        if (box.maxWidth >= 420) {
          return Row(
            children: [
              icon,
              const SizedBox(width: 12),
              const Expanded(child: text),
              const SizedBox(width: 10),
              Flexible(child: button),
            ],
          );
        }
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Row(children: [icon, SizedBox(width: 10), Expanded(child: text)]),
            const SizedBox(height: 12),
            button,
          ],
        );
      }),
    );
  }
}

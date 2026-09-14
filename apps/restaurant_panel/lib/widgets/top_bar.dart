import 'package:flutter/material.dart';

import '../api.dart';
import '../theme.dart';
import 'date_range_dialog.dart';

const _monthNamesShort = [
  'yan',
  'fev',
  'mar',
  'apr',
  'may',
  'iyun',
  'iyul',
  'avg',
  'sen',
  'okt',
  'noy',
  'dek',
];

String _initials(String name) {
  final parts =
      name.trim().split(RegExp(r'\s+')).where((p) => p.isNotEmpty).toList();
  if (parts.isEmpty) return '?';
  if (parts.length == 1) return parts.first.substring(0, 1).toUpperCase();
  return (parts[0].substring(0, 1) + parts[1].substring(0, 1)).toUpperCase();
}

/// Yuqori panel — image/bosh.png namunasiga mos, BARCHA sahifalarda
/// doimiy ko'rinadi (MainLayout orqali): restoran logo/nomi/Ochiq-Yopiq
/// holati/manzili chapda; sana tanlagich, bildirishnoma qo'ng'irog'i
/// (haqiqiy — o'qilmagan restoran bildirishnomalari, jonli) va xodim
/// avatari o'ngda.
class TopBar extends StatelessWidget {
  final String restaurantName;
  final String restaurantAddress;
  final String restaurantLogoUrl;
  final bool open;
  final ValueChanged<bool> onOpenChanged;
  final String staffName;
  final String staffRole;
  final DateTime selectedDate;
  final ValueChanged<DateTime> onDateChanged;
  final int notificationsCount;
  final VoidCallback onBellTap;

  /// Sahifaga xos amallar (masalan Statistika: davr tanlash + Eksport).
  /// Berilsa, oddiy bir kunlik sana tugmasi O'RNIGA ko'rsatiladi;
  /// `null` — boshqa barcha sahifalardagi kabi sana tugmasi.
  final Widget? actions;

  const TopBar({
    this.actions,
    super.key,
    required this.restaurantName,
    required this.restaurantAddress,
    required this.restaurantLogoUrl,
    required this.open,
    required this.onOpenChanged,
    required this.staffName,
    required this.staffRole,
    required this.selectedDate,
    required this.onDateChanged,
    required this.notificationsCount,
    required this.onBellTap,
  });

  Future<void> _pickDate(BuildContext context) async {
    // Statistika sahifasidagi bilan bir xil kalendar (`date_range_dialog.dart`).
    final picked = await showOnDexDatePicker(
      context: context,
      initialDate: selectedDate,
      firstDate: DateTime.now().subtract(const Duration(days: 365)),
      lastDate: DateTime.now(),
    );
    if (picked != null) onDateChanged(picked);
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      height: 78,
      padding: const EdgeInsets.symmetric(horizontal: 24),
      decoration: const BoxDecoration(
        color: OnDexColors.cardBg,
        border: Border(bottom: BorderSide(color: OnDexColors.cardBorder)),
      ),
      child: Row(
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(10),
            child: Container(
              width: 42,
              height: 42,
              color: OnDexColors.primary,
              child: restaurantLogoUrl.isEmpty
                  ? const Icon(Icons.storefront, color: Colors.white, size: 20)
                  : Image.network(
                      fullImageUrl(restaurantLogoUrl),
                      fit: BoxFit.cover,
                      errorBuilder: (_, __, ___) => const Icon(
                          Icons.storefront,
                          color: Colors.white,
                          size: 20),
                    ),
            ),
          ),
          const SizedBox(width: 12),
          // MUHIM: bu yerda Expanded ATAYLAB yagona flex-farzand — avval
          // Flexible+alohida Spacer() ISHLATILGAN edi, lekin ikkalasi ham
          // flex:1 bo'lgani uchun Row qolgan bo'sh joyni ular orasida
          // TENG (50/50) taqsimlardi; nom ustuni o'z ulushining bir
          // qismini ishlatmasa (odatda shunday, chunki matn qisqa),
          // ISHLATILMAGAN qism hech kimga QAYTMASDAN yo'qolib ketardi —
          // natijada o'ng tomondagi sana/bell/avatar chindan CHAP TOMONGA
          // (go'yo "o'rtaga") siljib qolardi, o'ng chetga yetmasdi. Endi
          // FAQAT bitta flex farzand (Expanded) bor — u BARCHA bo'sh
          // joyni o'ziga oladi, o'ng tomondagi elementlar esa aniq
          // konteyner chetiga (padding'gacha) tegadi.
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Flexible(
                      child: Text(
                        restaurantName.isEmpty ? 'Restoran' : restaurantName,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                            fontSize: 15.5,
                            fontWeight: FontWeight.w700,
                            color: OnDexColors.ink),
                      ),
                    ),
                    const SizedBox(width: 8),
                    _OpenPill(open: open, onChanged: onOpenChanged),
                  ],
                ),
                if (restaurantAddress.isNotEmpty)
                  Text(restaurantAddress,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                          fontSize: 12.5, color: OnDexColors.inkDim)),
              ],
            ),
          ),
          actions ?? _DateButton(date: selectedDate, onTap: () => _pickDate(context)),
          const SizedBox(width: 14),
          _BellButton(count: notificationsCount, onTap: onBellTap),
          const SizedBox(width: 16),
          if (staffName.isNotEmpty) ...[
            CircleAvatar(
              radius: 19,
              backgroundColor: OnDexColors.primary,
              child: Text(_initials(staffName),
                  style: const TextStyle(
                      color: Colors.white,
                      fontSize: 13,
                      fontWeight: FontWeight.w700)),
            ),
            const SizedBox(width: 10),
            Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(staffName,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                        fontSize: 13,
                        fontWeight: FontWeight.w700,
                        color: OnDexColors.ink)),
                Text(staffRole,
                    style: const TextStyle(
                        fontSize: 11.5, color: OnDexColors.inkDim)),
              ],
            ),
          ],
        ],
      ),
    );
  }
}

class _OpenPill extends StatelessWidget {
  final bool open;
  final ValueChanged<bool> onChanged;
  const _OpenPill({required this.open, required this.onChanged});

  @override
  Widget build(BuildContext context) {
    final color = open ? OnDexColors.success : OnDexColors.danger;
    return GestureDetector(
      onTap: () => onChanged(!open),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
        decoration: BoxDecoration(
          border: Border.all(color: color.withValues(alpha: 0.4)),
          borderRadius: BorderRadius.circular(999),
        ),
        child: Text(open ? 'Ochiq' : 'Yopiq',
            style: TextStyle(
                fontSize: 11.5, color: color, fontWeight: FontWeight.w700)),
      ),
    );
  }
}

class _DateButton extends StatelessWidget {
  final DateTime date;
  final VoidCallback onTap;
  const _DateButton({required this.date, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final label = '${date.day}-${_monthNamesShort[date.month - 1]}, ${date.year}';
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(10),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
        decoration: BoxDecoration(
          border: Border.all(color: OnDexColors.cardBorder),
          borderRadius: BorderRadius.circular(10),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.calendar_today_rounded,
                size: 15, color: OnDexColors.inkDim),
            const SizedBox(width: 9),
            Text(label,
                style: const TextStyle(
                    fontSize: 13, color: OnDexColors.ink, fontWeight: FontWeight.w600)),
            const SizedBox(width: 6),
            const Icon(Icons.keyboard_arrow_down_rounded,
                size: 17, color: OnDexColors.inkDim),
          ],
        ),
      ),
    );
  }
}

class _BellButton extends StatelessWidget {
  final int count;
  final VoidCallback onTap;
  const _BellButton({required this.count, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return InkWell(
      key: const ValueKey('topbar-bell'),
      onTap: onTap,
      borderRadius: BorderRadius.circular(999),
      child: Padding(
        padding: const EdgeInsets.all(6),
        child: Stack(
          clipBehavior: Clip.none,
          children: [
            const Icon(Icons.notifications_rounded,
                size: 23, color: OnDexColors.inkDim),
            if (count > 0)
              Positioned(
                right: -2,
                top: -2,
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 1),
                  constraints: const BoxConstraints(minWidth: 16),
                  decoration: const BoxDecoration(
                    color: OnDexColors.danger,
                    borderRadius: BorderRadius.all(Radius.circular(999)),
                  ),
                  child: Text(
                    count > 9 ? '9+' : '$count',
                    textAlign: TextAlign.center,
                    style: const TextStyle(
                        color: Colors.white,
                        fontSize: 10,
                        fontWeight: FontWeight.w800),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

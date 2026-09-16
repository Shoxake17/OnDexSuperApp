/// Restoranning "Ochiq/Yopiq" holati — barcha Flutter ilovalari uchun
/// YAGONA o'quvchi.
///
/// ┌─ NEGA (2026-09-15) ───────────────────────────────────────────────┐
/// Ilovalar `restaurant['open']` ga qarardi — bu restoranning QO'LDA
/// bosadigan tugmasi. Ish vaqti tugagan restoran hamma joyda "Ochiq"
/// ko'rinib, mijoz taom qo'shgach faqat savatda "yopiq" javobini olardi,
/// restoran panelidagi belgi ham "Ochiq" deb turardi.
///
/// Endi holatni SERVER hisoblaydi (`open_now`, `closed_reason`,
/// `open_changes_at` — `internal/catalog/hours.go`), bu yer faqat o'qiydi
/// va matnga aylantiradi. Ish vaqti qoidasi bu yerda QAYTA YOZILMAYDI.
/// Web'da aynan shu vazifani `apps/web/lib/restaurant-status.ts` bajaradi.
/// └───────────────────────────────────────────────────────────────────┘
library;

enum RestaurantClosedReason { manual, hours }

/// Ish vaqti Toshkent vaqtida (server bilan bir xil qat'iy +05:00).
const _tashkentOffset = Duration(hours: 5);

const _weekdays = [
  'Dushanba',
  'Seshanba',
  'Chorshanba',
  'Payshanba',
  'Juma',
  'Shanba',
  'Yakshanba',
];

class RestaurantOpenStatus {
  const RestaurantOpenStatus({required this.open, this.reason, this.changesAt});

  /// Hozir buyurtma qabul qiladi.
  final bool open;

  /// Yopiq bo'lsa sababi, ochiq bo'lsa `null`.
  final RestaurantClosedReason? reason;

  /// Holat o'z-o'zidan o'zgaradigan payt (UTC), server hisoblagan.
  final DateTime? changesAt;

  /// "Yopiladi" izohi yopilishga shuncha qolganda ko'rsatiladi.
  static const closingSoon = Duration(hours: 1);

  /// Restoran javobidan (`GET /restaurants`, `GET /restaurants/{id}`).
  factory RestaurantOpenStatus.fromJson(Map<String, dynamic> r) {
    final manualOpen = r['open'] == true;
    final openNow = r['open_now'];
    // `open_now` bo'lmagan eski javob — avvalgi xatti-harakat (qo'lda tugma).
    final open = openNow is bool ? openNow : manualOpen;
    RestaurantClosedReason? reason;
    if (!open) {
      reason = switch (r['closed_reason']) {
        'hours' => RestaurantClosedReason.hours,
        'manual' => RestaurantClosedReason.manual,
        _ => manualOpen ? RestaurantClosedReason.hours : RestaurantClosedReason.manual,
      };
    }
    final raw = r['open_changes_at'];
    return RestaurantOpenStatus(
      open: open,
      reason: reason,
      changesAt: raw is String ? DateTime.tryParse(raw)?.toUtc() : null,
    );
  }

  /// Ekran ochiq turgan paytda [changesAt] o'tib ketsa — holat jadval
  /// bo'yicha almashadi. Qo'lda yopilgan restoran vaqt bilan ochilmaydi.
  RestaurantOpenStatus at(DateTime now) {
    final c = changesAt;
    if (c == null || reason == RestaurantClosedReason.manual || now.toUtc().isBefore(c)) {
      return this;
    }
    return open
        ? const RestaurantOpenStatus(open: false, reason: RestaurantClosedReason.hours)
        : const RestaurantOpenStatus(open: true);
  }

  String get label => open ? 'Ochiq' : 'Yopiq';

  /// Qisqa izoh: "09:00 da ochiladi", "Ertaga 09:00 da ochiladi",
  /// "Dushanba 09:00 da ochiladi", yopilishga bir soatdan kam qolganda
  /// "22:00 da yopiladi". Aytadigan narsa bo'lmasa `null`.
  String? detail(DateTime now) {
    final c = changesAt;
    if (c == null) return null;
    final at = c.toUtc().add(_tashkentOffset);
    final clock = '${_two(at.hour)}:${_two(at.minute)}';
    if (open) {
      return c.difference(now.toUtc()) <= closingSoon ? '$clock da yopiladi' : null;
    }
    final today = now.toUtc().add(_tashkentOffset);
    final days = DateTime.utc(at.year, at.month, at.day)
        .difference(DateTime.utc(today.year, today.month, today.day))
        .inDays;
    if (days <= 0) return '$clock da ochiladi';
    if (days == 1) return 'Ertaga $clock da ochiladi';
    return '${_weekdays[at.weekday - 1]} $clock da ochiladi';
  }

  /// Menyu/panel ogohlantirishi uchun to'liq jumla (yopiq holatda).
  String closedMessage(DateTime now) {
    final d = detail(now);
    if (d != null) return 'Restoran hozir yopiq · $d';
    return reason == RestaurantClosedReason.manual
        ? 'Restoran hozir buyurtma qabul qilmayapti'
        : 'Restoran hozir yopiq — ish vaqti tugagan';
  }
}

String _two(int v) => v.toString().padLeft(2, '0');

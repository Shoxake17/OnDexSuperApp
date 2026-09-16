part of '../staff_page.dart';

String _s(Object? v) => v is String ? v : '';
int _i(Object? v) => v is num ? v.toInt() : 0;
int? _iOrNull(Object? v) => v is num ? v.toInt() : null;

class StaffPositionInfo {
  const StaffPositionInfo({required this.key, required this.title, required this.group, required this.appAccess});

  final String key;
  final String title;
  final String group;

  /// Shu lavozim uchun OnDex ilovasi bormi (hozircha faqat ofitsiant).
  final bool appAccess;

  factory StaffPositionInfo.fromJson(Map<String, dynamic> j) => StaffPositionInfo(
        key: _s(j['key']),
        title: _s(j['title']),
        group: _s(j['group']),
        appAccess: j['app_access'] == true,
      );
}

/// Lavozimning OnDex ilovasi nomi (server `staff.appRole` bilan bir xil):
/// ofitsiant — "OnDexPro", yetkazib beruvchi — "OnDexGO".
String staffAppName(String? position) =>
    position == 'courier' ? 'OnDexGO' : 'OnDexPro';

IconData positionIcon(String key) => switch (key) {
      'manager' => Icons.manage_accounts_rounded,
      'administrator' => Icons.admin_panel_settings_rounded,
      'head_chef' => Icons.soup_kitchen_rounded,
      'chef' => Icons.restaurant_rounded,
      'cook_assistant' => Icons.kitchen_rounded,
      'tandoor_baker' => Icons.local_fire_department_rounded,
      'baker' => Icons.bakery_dining_rounded,
      'waiter' => Icons.room_service_rounded,
      'bartender' => Icons.local_bar_rounded,
      'barista' => Icons.coffee_rounded,
      'host' => Icons.emoji_people_rounded,
      'cashier' => Icons.point_of_sale_rounded,
      'courier' => Icons.delivery_dining_rounded,
      'dishwasher' => Icons.wash_rounded,
      'cleaner' => Icons.cleaning_services_rounded,
      'helper' => Icons.handyman_rounded,
      'security' => Icons.shield_outlined,
      _ => Icons.badge_outlined,
    };

const _weekdayShort = ['Du', 'Se', 'Ch', 'Pa', 'Ju', 'Sh', 'Ya'];
const _weekdayFull = ['Dushanba', 'Seshanba', 'Chorshanba', 'Payshanba', 'Juma', 'Shanba', 'Yakshanba'];
const _monthNames = [
  'Yanvar', 'Fevral', 'Mart', 'Aprel', 'May', 'Iyun', //
  'Iyul', 'Avgust', 'Sentabr', 'Oktabr', 'Noyabr', 'Dekabr',
];

class StaffSchedule {
  const StaffSchedule({required this.days, required this.start, required this.end});

  final List<int> days;
  final String start;
  final String end;

  String get range => '$start – $end';

  bool worksOn(int day) => days.contains(day);

  String get daysText {
    if (days.length == 7) return 'Har kuni';
    return [for (final d in days) if (d >= 1 && d <= 7) _weekdayShort[d - 1]].join(', ');
  }

  /// Jadvalda yo'q kunlar — dam olish kunlari.
  List<int> get offDays => [for (var d = 1; d <= 7; d++) if (!days.contains(d)) d];

  Map<String, Object> toJson() => {'days': [...days]..sort(), 'start': start, 'end': end};

  static StaffSchedule? fromJson(Object? j) {
    if (j is! Map) return null;
    final days = j['days'] is List ? [for (final d in j['days'] as List) if (d is num) d.toInt()] : <int>[];
    return StaffSchedule(days: days..sort(), start: _s(j['start']), end: _s(j['end']));
  }
}

class StaffMember {
  const StaffMember({
    required this.id,
    required this.number,
    required this.code,
    required this.firstName,
    required this.lastName,
    required this.fullName,
    required this.phone,
    required this.position,
    required this.positionTitle,
    required this.status,
    required this.schedule,
    required this.hiredOn,
    required this.note,
    required this.salaryTiyin,
    required this.appAccess,
    required this.appAccessActive,
    required this.createdAt,
  });

  final String id;
  final int number;
  final String code;
  final String firstName;
  final String lastName;
  final String fullName;
  final String phone;
  final String position;
  final String positionTitle;
  final String status;
  final StaffSchedule? schedule;
  final DateTime? hiredOn;
  final String note;

  /// Oylik maosh (tiyin); kiritilmagan bo'lsa null.
  final int? salaryTiyin;
  final bool appAccess;
  final bool appAccessActive;
  final DateTime createdAt;

  factory StaffMember.fromJson(Map<String, dynamic> j) {
    final first = _s(j['first_name']);
    final last = _s(j['last_name']);
    final full = _s(j['full_name']);
    return StaffMember(
      id: _s(j['id']),
      number: _i(j['number']),
      code: _s(j['code']),
      firstName: first,
      lastName: last,
      fullName: full.isNotEmpty ? full : '$first $last'.trim(),
      phone: _s(j['phone']),
      position: _s(j['position']),
      positionTitle: _s(j['position_title']),
      status: _s(j['status']),
      schedule: StaffSchedule.fromJson(j['schedule']),
      hiredOn: DateTime.tryParse(_s(j['hired_on'])),
      note: _s(j['note']),
      salaryTiyin: _iOrNull(j['monthly_salary_tiyin']),
      appAccess: j['app_access'] == true,
      appAccessActive: j['app_access_active'] == true,
      createdAt: DateTime.tryParse(_s(j['created_at']))?.toLocal() ?? DateTime.fromMillisecondsSinceEpoch(0),
    );
  }
}

class StaffSummary {
  const StaffSummary({
    required this.total,
    required this.active,
    required this.onLeave,
    required this.dismissed,
    required this.workingToday,
    required this.byPosition,
  });

  final int total;
  final int active;
  final int onLeave;
  final int dismissed;
  final int workingToday;
  final Map<String, int> byPosition;

  factory StaffSummary.fromJson(Object? raw) {
    final j = raw is Map ? raw : const {};
    final by = <String, int>{};
    if (j['by_position'] is Map) {
      (j['by_position'] as Map).forEach((k, v) {
        if (k is String && v is num) by[k] = v.toInt();
      });
    }
    return StaffSummary(
      total: _i(j['total']),
      active: _i(j['active']),
      onLeave: _i(j['on_leave']),
      dismissed: _i(j['dismissed']),
      workingToday: _i(j['working_today']),
      byPosition: by,
    );
  }
}

class StaffEvent {
  const StaffEvent({
    required this.memberName,
    required this.kind,
    required this.from,
    required this.to,
    required this.fromTitle,
    required this.toTitle,
    required this.at,
  });

  final String memberName;
  final String kind;
  final String from;
  final String to;
  final String fromTitle;
  final String toTitle;
  final DateTime at;

  factory StaffEvent.fromJson(Map<String, dynamic> j) => StaffEvent(
        memberName: _s(j['member_name']),
        kind: _s(j['kind']),
        from: _s(j['from']),
        to: _s(j['to']),
        fromTitle: _s(j['from_title']),
        toTitle: _s(j['to_title']),
        at: DateTime.tryParse(_s(j['at']))?.toLocal() ?? DateTime.fromMillisecondsSinceEpoch(0),
      );

  String get title => switch (kind) {
        'created' => 'Yangi xodim qo\'shildi',
        'updated' => 'Ma\'lumotlari yangilandi',
        'position_changed' => 'Xodim lavozimi o\'zgartirildi',
        'schedule_changed' => 'Ish jadvali yangilandi',
        'salary_changed' => 'Oylik maoshi o\'zgartirildi',
        'status_changed' => switch (to) {
            'dismissed' => 'Ishdan bo\'shatildi',
            'on_leave' => 'Ta\'tilga chiqdi',
            _ => from == 'dismissed' ? 'Qayta ishga olindi' : 'Ta\'tildan qaytdi',
          },
        'access_granted' => 'Ilovaga kirish ochildi',
        'access_revoked' => 'Ilovaga kirish yopildi',
        _ => 'O\'zgarish',
      };

  String get subtitle {
    final name = memberName.isEmpty ? 'Xodim' : memberName;
    if (kind == 'position_changed' && fromTitle.isNotEmpty) return '$name ($fromTitle → $toTitle)';
    return name;
  }

  (IconData, Color, Color) get look => switch (kind) {
        'created' => (Icons.person_add_alt_1_rounded, OnDexColors.success, OnDexColors.successBg),
        'position_changed' => (Icons.swap_horiz_rounded, OnDexColors.primary, OnDexColors.primaryTint),
        'schedule_changed' => (Icons.calendar_month_rounded, OnDexColors.info, OnDexColors.infoBg),
        'salary_changed' => (Icons.payments_outlined, OnDexColors.success, OnDexColors.successBg),
        'status_changed' => switch (to) {
            'dismissed' => (Icons.logout_rounded, OnDexColors.danger, OnDexColors.dangerBg),
            'on_leave' => (Icons.beach_access_rounded, OnDexColors.amber, OnDexColors.amberBg),
            _ => (Icons.how_to_reg_rounded, OnDexColors.success, OnDexColors.successBg),
          },
        'access_granted' => (Icons.phone_iphone_rounded, OnDexColors.success, OnDexColors.successBg),
        'access_revoked' => (Icons.phonelink_erase_rounded, OnDexColors.danger, OnDexColors.dangerBg),
        _ => (Icons.edit_note_rounded, OnDexColors.info, OnDexColors.infoBg),
      };
}

class _Overview {
  const _Overview({required this.members, required this.summary, required this.recent, required this.positions});

  final List<StaffMember> members;
  final StaffSummary summary;
  final List<StaffEvent> recent;
  final List<StaffPositionInfo> positions;

  factory _Overview.fromJson(Map<String, dynamic> j) => _Overview(
        members: [
          for (final m in (j['items'] as List? ?? const []))
            if (m is Map) StaffMember.fromJson(Map<String, dynamic>.from(m)),
        ],
        summary: StaffSummary.fromJson(j['summary']),
        recent: [
          for (final e in (j['recent_activity'] as List? ?? const []))
            if (e is Map) StaffEvent.fromJson(Map<String, dynamic>.from(e)),
        ],
        positions: [
          for (final p in (j['positions'] as List? ?? const []))
            if (p is Map) StaffPositionInfo.fromJson(Map<String, dynamic>.from(p)),
        ],
      );
}

// ─── Oylik hisobot ────────────────────────────────────────────────────

class StaffReportRow {
  const StaffReportRow({
    required this.id,
    required this.code,
    required this.fullName,
    required this.positionTitle,
    required this.status,
    required this.schedule,
    required this.salaryTiyin,
    required this.accruedTiyin,
    required this.payableDays,
    required this.workedDays,
    required this.leaveDays,
    required this.workedMinutes,
    required this.plannedMinutes,
    required this.scheduled,
    required this.days,
  });

  final String id;
  final String code;
  final String fullName;
  final String positionTitle;
  final String status;
  final StaffSchedule? schedule;
  final int? salaryTiyin;
  final int? accruedTiyin;
  final int payableDays;
  final int workedDays;
  final int leaveDays;
  final int workedMinutes;
  final int plannedMinutes;
  final bool scheduled;

  /// Oyning har kuni: W ishlagan, P reja, O dam olish, L ta'til, N ishda emas.
  final String days;

  factory StaffReportRow.fromJson(Map<String, dynamic> j) => StaffReportRow(
        id: _s(j['id']),
        code: _s(j['code']),
        fullName: _s(j['full_name']),
        positionTitle: _s(j['position_title']),
        status: _s(j['status']),
        schedule: StaffSchedule.fromJson(j['schedule']),
        salaryTiyin: _iOrNull(j['monthly_salary_tiyin']),
        accruedTiyin: _iOrNull(j['accrued_salary_tiyin']),
        payableDays: _i(j['payable_days']),
        workedDays: _i(j['worked_days']),
        leaveDays: _i(j['leave_days']),
        workedMinutes: _i(j['worked_minutes']),
        plannedMinutes: _i(j['planned_minutes']),
        scheduled: j['scheduled'] == true,
        days: _s(j['days']),
      );
}

class StaffReport {
  const StaffReport({
    required this.daysInMonth,
    required this.rows,
    required this.members,
    required this.salaryFundTiyin,
    required this.accruedTiyin,
    required this.workedMinutes,
    required this.plannedMinutes,
  });

  final int daysInMonth;
  final List<StaffReportRow> rows;
  final int members;
  final int salaryFundTiyin;
  final int accruedTiyin;
  final int workedMinutes;
  final int plannedMinutes;

  factory StaffReport.fromJson(Map<String, dynamic> j) {
    final t = j['totals'] is Map ? j['totals'] as Map : const {};
    return StaffReport(
      daysInMonth: _i(j['days_in_month']),
      rows: [
        for (final r in (j['items'] as List? ?? const []))
          if (r is Map) StaffReportRow.fromJson(Map<String, dynamic>.from(r)),
      ],
      members: _i(t['members']),
      salaryFundTiyin: _i(t['salary_fund_tiyin']),
      accruedTiyin: _i(t['accrued_tiyin']),
      workedMinutes: _i(t['worked_minutes']),
      plannedMinutes: _i(t['planned_minutes']),
    );
  }
}

// ─── Formatlash ───────────────────────────────────────────────────────

/// "+998901234567" → "+998 90 123 45 67".
String formatUzPhone(String e164) {
  if (e164.length == 13 && e164.startsWith('+998')) {
    return '+998 ${e164.substring(4, 6)} ${e164.substring(6, 9)} ${e164.substring(9, 11)} ${e164.substring(11)}';
  }
  return e164;
}

String _groupThousands(int v) {
  final s = v.abs().toString();
  final b = StringBuffer(v < 0 ? '-' : '');
  for (var i = 0; i < s.length; i++) {
    if (i > 0 && (s.length - i) % 3 == 0) b.write(' ');
    b.write(s[i]);
  }
  return b.toString();
}

/// 350000000 tiyin → "3 500 000 so'm".
String formatMoney(int tiyin) => '${_groupThousands((tiyin / 100).round())} so\'m';

/// 3780 daqiqa → "63 soat", 3810 → "63 soat 30 daq".
String formatHours(int minutes) {
  final h = minutes ~/ 60;
  final m = minutes % 60;
  return m == 0 ? '$h soat' : '$h soat $m daq';
}

String _initials(String name) {
  final parts = name.trim().split(RegExp(r'\s+')).where((p) => p.isNotEmpty).toList();
  if (parts.isEmpty) return '?';
  final a = parts.first.characters.first;
  final b = parts.length > 1 ? parts[1].characters.first : '';
  return (a + b).toUpperCase();
}

const _avatarPalette = [
  (Color(0xFFF2650F), Color(0xFFFDE9DA)),
  (Color(0xFF3B5FA8), Color(0xFFE3E9F6)),
  (Color(0xFF2F9E58), Color(0xFFE1F1E6)),
  (Color(0xFF7C5CD6), Color(0xFFEFEAFB)),
  (Color(0xFFB8790C), Color(0xFFFAECD2)),
  (Color(0xFFD1467A), Color(0xFFFBE4EE)),
];

String _two(int v) => v.toString().padLeft(2, '0');
String _ymd(DateTime d) => '${d.year}-${_two(d.month)}-${_two(d.day)}';
String _dmy(DateTime d) => '${_two(d.day)}.${_two(d.month)}.${d.year}';

String _eventTime(DateTime at) {
  final now = DateTime.now();
  final t = '${_two(at.hour)}:${_two(at.minute)}';
  if (at.year == now.year && at.month == now.month && at.day == now.day) return t;
  return '${_two(at.day)}.${_two(at.month)} $t';
}

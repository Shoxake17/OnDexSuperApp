const _months = [
  'yanvar',
  'fevral',
  'mart',
  'aprel',
  'may',
  'iyun',
  'iyul',
  'avgust',
  'sentabr',
  'oktabr',
  'noyabr',
  'dekabr',
];

String _two(int n) => n.toString().padLeft(2, '0');

/// "340 KB" / "2.4 MB"
String supportBytes(int n) {
  if (n < 1024 * 1024) return '${(n / 1024).ceil()} KB';
  return '${(n / (1024 * 1024)).toStringAsFixed(1)} MB';
}

/// "14:32"
String supportClock(DateTime d) => '${_two(d.hour)}:${_two(d.minute)}';

/// "12-sentabr, 2026"
String supportDate(DateTime d) => '${d.day}-${_months[d.month - 1]}, ${d.year}';

/// Kun ajratgichi: "Bugun", "Kecha" yoki "12-sentabr, 2026".
String supportDayLabel(DateTime d, DateTime now) {
  final day = DateTime(d.year, d.month, d.day);
  final today = DateTime(now.year, now.month, now.day);
  final diff = today.difference(day).inDays;
  if (diff == 0) return 'Bugun';
  if (diff == 1) return 'Kecha';
  return supportDate(d);
}

/// "Bugun, 14:32" / "Kecha, 09:10" / "12-sentabr, 2026".
String supportActivity(DateTime? d, DateTime now) {
  if (d == null) return '—';
  final label = supportDayLabel(d, now);
  if (label == 'Bugun' || label == 'Kecha') return '$label, ${supportClock(d)}';
  return label;
}

/// Ro'yxatdagi vaqt: bugun bo'lsa soat, aks holda qisqa sana.
String supportListTime(DateTime? d, DateTime now) {
  if (d == null) return '';
  final label = supportDayLabel(d, now);
  if (label == 'Bugun') return supportClock(d);
  if (label == 'Kecha') return 'Kecha';
  return '${_two(d.day)}.${_two(d.month)}.${d.year % 100}';
}

/// Affitsiant ilovasiga XOS formatlash.
///
/// Umumiy formatlash (`formatSum`) `ondex_core` da — pul ko'rinishi
/// barcha ilovalarda bir xil bo'lishi kerak. Bu yerda faqat shu ilovaga
/// tegishli narsalar: stol nomi va "qancha vaqt kutmoqda".
library;

/// Stol nomi — `ondex_core` dan qayta eksport qilinadi.
///
/// Bir xil stol nomi restoran panelida ham ko'rinadi, shuning uchun
/// mantiq umumiy yadroda turadi (ilgari bu yerda o'z nusxasi bor edi,
/// panelda esa boshqacha yozilgan va "Stol-1-stol" chiqarardi).
export 'package:ondex_core/ondex_core.dart' show tableText;

/// Stol belgisidagi qisqa yozuv (panjara katagi uchun): faqat raqam
/// bo'lsa raqamning o'zi, aks holda to'liq nom.
String tableShort(String label) => label.isEmpty ? '—' : label;

/// "3 daq" / "1 soat 05 daq" — qisqa shakl, kartochka uchun.
///
/// Affitsiant uchun eng muhim raqam shu: 1 daqiqa oldin tayyor bo'lgan
/// va 15 daqiqa oldin tayyor bo'lgan buyurtma bir xil ko'rinmasligi
/// kerak.
///
/// Bir daqiqadan kam vaqt uchun ATAYLAB alohida funksiya bor
/// (`waitBadge`/`waitSentence`) — bu yerdan "hozir" qaytarilsa,
/// chaqiruv joyida "hozir kutmoqda" / "hozir dan beri tayyor" kabi
/// g'aliz jumlalar hosil bo'lardi.
String formatWait(Duration d) {
  final minutes = d.inMinutes;
  if (minutes < 60) return '${minutes < 1 ? 1 : minutes} daq';
  final h = d.inHours;
  final m = minutes % 60;
  return m == 0 ? '$h soat' : '$h soat ${m.toString().padLeft(2, '0')} daq';
}

/// To'liq shakl, jumla ichida ishlatish uchun: "5 daqiqa",
/// "1 soat 5 daqiqa". Qo'shimcha `-dan` shu shaklga yopishadi.
String formatWaitLong(Duration d) {
  final minutes = d.inMinutes;
  if (minutes < 60) return '$minutes daqiqa';
  final h = d.inHours;
  final m = minutes % 60;
  return m == 0 ? '$h soat' : '$h soat $m daqiqa';
}

/// Kartochkadagi qisqa yozuv.
String waitBadge(Duration d) =>
    d.inMinutes < 1 ? 'hozirgina tayyor bo\'ldi' : '${formatWait(d)} kutmoqda';

/// Tafsilot ekranidagi to'liq jumla.
String waitSentence(Duration d) => d.inMinutes < 1
    ? 'Taom hozirgina tayyor bo\'ldi'
    : 'Taom ${formatWaitLong(d)}dan beri tayyor';

/// Soat:daqiqa — tarix va tasma yozuvlari uchun.
String formatClock(DateTime t) =>
    '${t.hour.toString().padLeft(2, '0')}:${t.minute.toString().padLeft(2, '0')}';

/// "5 daqiqa oldin" — bildirishnomalar tasmasi uchun.
String formatAgo(DateTime t) {
  final d = DateTime.now().difference(t);
  if (d.inSeconds < 60) return 'hozirgina';
  if (d.inMinutes < 60) return '${d.inMinutes} daqiqa oldin';
  if (d.inHours < 24) return '${d.inHours} soat oldin';
  return formatClock(t);
}

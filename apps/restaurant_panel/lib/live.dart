import 'api.dart';

/// Restoran panelining JONLI kanali — butun ilova uchun bitta.
///
/// ┌─ NIMA O'ZGARDI ───────────────────────────────────────────────────┐
/// Avval WebSocket FAQAT buyurtmalar sahifasida edi. Yon paneldagi
/// "yangi buyurtmalar" hisoblagichi va qo'ng'iroq belgisi esa alohida,
/// 20 soniyalik so'rov sikliga tayanardi — ya'ni oshxona buyurtmani
/// ko'rdi, lekin yon paneldagi raqam hali eski qiymatda turardi.
///
/// Endi ikkalasi ham AYNI kanaldan oziqlanadi: bitta ulanish, bitta
/// bilet, bitta qayta ulanish sikli.
/// └───────────────────────────────────────────────────────────────────┘
///
/// So'rov sikli o'chirilmadi — u ZAXIRA bo'lib qoldi (`LiveRefresher`
/// soket ulangan bo'lsa siyrak, uzilgan bo'lsa tez-tez so'raydi).
final restaurantLive = LiveBus(
  ticketProvider: api.wsTicket,
  urlBuilder: (t) => wsUrl(t),
);

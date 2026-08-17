import 'api.dart';

/// Mijoz ilovasining JONLI kanali — butun ilova uchun BITTA.
///
/// ┌─ NEGA UMUMIY YADRODAN ────────────────────────────────────────────┐
/// Avval `orders_screen.dart` xom `WebSocketChannel` ochib, o'z qayta
/// ulanishini va o'z zaxira so'rov siklini yozgan edi. Ayni mantiq
/// `ondex_core` dagi `LiveBus`/`LiveRefresher` da allaqachon bor va
/// restoran paneli o'shani ishlatadi.
///
/// Ya'ni bitta stack ichida ikki nusxa turardi — va ular allaqachon
/// ajralib ketgan edi: yadro versiyasida eksponensial backoff va
/// jitter bor, qo'lda yozilganida esa qat'iy 2 soniya.
///
/// Endi ikkalasi ham AYNI koddan oziqlanadi: bitta ulanish, bitta
/// bilet, bitta qayta ulanish sikli.
/// └───────────────────────────────────────────────────────────────────┘
final customerLive = LiveBus(
  ticketProvider: api.wsTicket,
  urlBuilder: (t) => wsUrl(t),
);

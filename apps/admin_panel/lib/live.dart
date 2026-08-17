import 'api.dart';

/// Superadmin panelining JONLI kanali — butun ilova uchun bitta.
///
/// ┌─ NIMA O'ZGARDI ───────────────────────────────────────────────────┐
/// Avval panelda WebSocket UMUMAN yo'q edi: boshqaruv sahifasi 10
/// soniyada, buyurtmalar 5 soniyada, kuryerlar 10 soniyada so'rov
/// yuborardi. Ya'ni yangi buyurtma ekranda kechikib paydo bo'lardi va
/// panel doimiy ravishda bekorga so'rov yuboraverardi.
///
/// Server tomonda ma'muriyat uchun alohida kanal ochildi
/// (`internal/notify/topic.go` — `Admin()`), unga FAQAT `admin` roli
/// obuna bo'ladi. Bu yerdagi shina o'sha kanalni butun panelga
/// tarqatadi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// POLLING QOLDIRILDI, lekin ZAXIRA sifatida: soket uzilgan bo'lsa
/// (tarmoq, server restart) ekran eskirib qolmasligi kerak. Sahifalar
/// `liveRefreshInterval` ni ishlatadi — ulanganda siyrak, uzilganda
/// tez-tez.
final adminLive = LiveBus(
  ticketProvider: api.wsTicket,
  urlBuilder: (t) => wsUrl(t),
);

/// Zaxira so'rov oralig'i: jonli kanal ishlayotganda kamdan-kam,
/// uzilganda esa eski (tez) tezlikda.
///
/// NEGA UMUMAN QOLDIRILDI: WebSocket "yarim ochiq" holatda qolishi
/// mumkin (mobil tarmoq, quvvat tejash, proksi) va bunda uzilish
/// darhol sezilmaydi. Bir daqiqalik zaxira so'rov panelning HECH
/// QACHON eskirmasligini kafolatlaydi, lekin serverga yuk
/// tug'dirmaydi.
Duration liveRefreshInterval() =>
    adminLive.connected ? const Duration(seconds: 60) : const Duration(seconds: 10);

/// Buyurtma holatlari — MA'LUMOT (kalit, o'zbekcha yorliq, bosqich).
///
/// Avval bu jadval TO'RT joyda takrorlangan edi: mijoz ilovasi widget'i,
/// `apps/web/lib/order-status.tsx`, restoran paneli mavzusi va admin
/// paneli `api.dart`. Backend yangi holat qo'shsa yoki matnni
/// o'zgartirsa — to'rt joyni yangilash kerak edi, va amalda ular
/// allaqachon bir-biridan farq qilardi.
///
/// RANGLAR ATAYLAB BU YERDA EMAS: har bir ilovaning o'z dizayn tizimi
/// bor (mijoz ilovasi `kStatus*`, panellar `OnDexColors`). Umumiy
/// qilingani — matn va bosqich mantig'i, ya'ni backend bilan
/// bog'liq qism.
library;

/// Backend'dagi holat kalitlari (`internal/orders/order.go`) — TARTIB
/// bilan. Klientlar shu ro'yxatga tayanadi, o'z nusxasini yasamaydi.
const List<String> orderStatusKeys = [
  'created',
  'accepted',
  'preparing',
  'ready',
  'picked_up',
  'delivered',
  'rejected',
  'cancelled',
];

const Map<String, String> _labels = {
  'created': 'Yangi',
  'accepted': 'Qabul qilindi',
  'preparing': 'Tayyorlanmoqda',
  'ready': 'Tayyor — kuryer kutilmoqda',
  'picked_up': 'Kuryerda, yo\'lda',
  'delivered': 'Yetkazildi',
  'rejected': 'Rad etildi',
  'cancelled': 'Bekor qilindi',
};

/// Holat kaliti uchun foydalanuvchiga ko'rsatiladigan matn.
///
/// Noma'lum kalit uchun kalitning O'ZI qaytariladi (bo'sh satr emas) —
/// backend yangi holat qo'shsa, ilova hech bo'lmasa nimadir ko'rsatadi
/// va jimgina bo'shab qolmaydi.
String orderStatusLabel(String status) => _labels[status] ?? status;

/// Terminal (yakuniy) holatmi — bundan keyin o'zgarish bo'lmaydi.
bool isTerminalStatus(String status) =>
    status == 'delivered' || status == 'rejected' || status == 'cancelled';

/// 4 bosqichli vizual chiziq uchun bosqich indeksi.
///
/// `-1` — chiziq umuman ko'rsatilmaydi (yetkazilgan, rad etilgan,
/// bekor qilingan).
int orderStage(String status) {
  switch (status) {
    case 'created':
    case 'accepted':
      return 0;
    case 'preparing':
      return 1;
    case 'ready':
    case 'picked_up':
      return 2;
    default:
      return -1;
  }
}

/// Bosqich yorliqlari (chiziq ostida).
const List<String> orderStageLabels = [
  'Qabul qilindi',
  'Tayyorlanmoqda',
  'Yo\'lda',
  'Yetkazildi',
];

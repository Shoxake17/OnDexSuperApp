/// XARITA MOTORINI TANLASH — Google Maps yoki OnDexMap.
///
/// ┌─ NEGA FLAG, DARHOL ALMASHTIRISH EMAS ──────────────────────────────┐
/// Bu ilova Play Market'da tarqalgan va odamlar undan har kuni
/// foydalanadi. Xarita — buyurtma oqimining ikki muhim joyida:
/// manzil tanlash va kuryerni kuzatish. Ikkalasi ham buzilsa mijoz
/// buyurtma bera olmaydi yoki kuryer qayerdaligini ko'rmaydi.
///
/// Shuning uchun OnDexMap yonma-yon qo'shiladi, o'rniga emas:
///
///   * flag O'CHIQ (standart) — ilova AYNAN hozirgidek ishlaydi,
///     Google Maps kodiga bitta belgi ham tegilmagan;
///   * flag YOQIQ — o'sha ekranlar OnDexMap bilan chiziladi.
///
/// Ikkala yo'l bir xil vazifani bajaradi: bir xil markazni tanlash,
/// bir xil kuryer harakati, bir xil natija tashqariga qaytadi.
/// └────────────────────────────────────────────────────────────────────┘
///
/// ┌─ NEGA KOMPILYATSIYA VAQTIDA ───────────────────────────────────────┐
/// Flag `--dart-define` bilan beriladi, serverdan EMAS. Sabab: xarita
/// motorini masofadan almashtirish — ilovaning eng ko'p ko'riladigan
/// ekranini ishga tushgandan keyin o'zgartirish demakdir. Noto'g'ri
/// javob, kesh yoki buzilgan sozlama minglab qurilmada bir vaqtda
/// xaritani o'chirib qo'yishi mumkin edi.
///
/// Kompilyatsiya vaqtidagi flag esa `false` bo'lsa OnDexMap kodini
/// tree-shaking butunlay olib tashlaydi: u ishga tushmaydi ham,
/// build'ga ham kirmaydi.
///
/// ⚠️ Shu sababli flagni Shorebird patch'i bilan YOQIB BO'LMAYDI —
/// u yangi release build talab qiladi.
/// └────────────────────────────────────────────────────────────────────┘
///
/// Yoqish:
/// ```
/// flutter build apk --dart-define=ONDEX_MAP=true
/// ```
library;

/// OnDexMap yoqilganmi.
///
/// Standart `false`: hozirgi production xatti-harakati o'zgarmaydi.
const bool kUseOnDexMap = bool.fromEnvironment('ONDEX_MAP');

/// OnDexMap uslubi (vektor, PMTiles manbalari bilan).
///
/// `maplibre_gl` PMTiles'ni o'zi qo'llaydi — qo'shimcha protokol
/// ro'yxatdan o'tkazish shart emas.
const String kOndexStyleUrl = 'https://maps.ondex.uz/tiles/style.json';

/// OnDexMap teskari geokodlash ishlaydigan hudud.
///
/// ⚠️ Butun mamlakat EMAS. Tashqarisida server `400` qaytaradi,
/// shuning uchun manzil tanlashda bu hudud tekshiriladi va kerak
/// bo'lsa eski yo'lga (backend geokoderi) tushiladi.
const double kOndexReverseMinLat = 40.5;
const double kOndexReverseMaxLat = 41.6;
const double kOndexReverseMinLng = 70.5;
const double kOndexReverseMaxLng = 72.0;

/// Nuqta OnDexMap teskari geokodlash hududidami.
bool ondexReverseCovers(double lat, double lng) =>
    lat >= kOndexReverseMinLat &&
    lat <= kOndexReverseMaxLat &&
    lng >= kOndexReverseMinLng &&
    lng <= kOndexReverseMaxLng;

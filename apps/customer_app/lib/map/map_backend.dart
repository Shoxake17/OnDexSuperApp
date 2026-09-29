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

/// OnDexMap serverining manzili.
const String kOndexApiBase = 'https://maps.ondex.uz';

/// OnDexMap uslubi (vektor, PMTiles manbalari bilan).
///
/// `maplibre_gl` PMTiles'ni o'zi qo'llaydi — qo'shimcha protokol
/// ro'yxatdan o'tkazish shart emas.
const String kOndexStyleUrl = '$kOndexApiBase/tiles/style.json';

/// OnDexMap teskari geokodlash ishlaydigan hudud.
///
/// ⚠️ Butun mamlakat EMAS. Tashqarisida server `400` qaytaradi,
/// shuning uchun manzil tanlashda bu hudud tekshiriladi va kerak
/// bo'lsa eski yo'lga (backend geokoderi) tushiladi.
const double kOndexReverseMinLat = 40.5;
const double kOndexReverseMaxLat = 41.6;
const double kOndexReverseMinLng = 70.5;
const double kOndexReverseMaxLng = 72.0;

/// Marshrut hududi — BUTUN O'ZBEKISTON.
///
/// ┌─ TESKARI GEOKODLASHDAN FARQI ──────────────────────────────────────┐
/// Marshrut boshqa manbadan keladi — OnDexMap serverining o'z OSRM
/// grafigidan, u `uzbekistan.osm.pbf` dan qurilgan va butun
/// mamlakatni qamraydi (2026-09-28 da o'lchangan: Toshkent ichida,
/// Buxoro→Toshkent, Nukus→Termiz, Xorazm→Farg'ona).
///
/// Teskari geokodlash esa mahalla va ko'cha ma'lumotiga bog'liq va
/// u hozircha torroq hududda ({@link kOndexReverseMinLat} …).
/// └────────────────────────────────────────────────────────────────────┘
const double kOndexRouteMinLat = 37.0;
const double kOndexRouteMaxLat = 45.8;
const double kOndexRouteMinLng = 55.5;
const double kOndexRouteMaxLng = 73.5;

/// Kamera chiqa olmaydigan hudud — O'zbekiston va uning atrofi.
///
/// ┌─ NEGA KERAK ───────────────────────────────────────────────────────┐
/// Xarita ma'lumotimiz (`uzbekistan.pmtiles`) faqat O'zbekistonni
/// qamraydi. Chegarasiz xaritada foydalanuvchi bir surishda mamlakat
/// tashqarisiga chiqib ketadi va OQ BO'SHLIQ ko'radi — ilova buzilgandek
/// taassurot qoldiradi.
///
/// Chegara ma'lumot hududidan bir oz kengroq (±0.6°): aks holda
/// chekkadagi shaharni (Nukus, Termiz) ekran markaziga qo'yib
/// bo'lmasdi — kamera markazi chegara ichida bo'lishi kerak.
/// └────────────────────────────────────────────────────────────────────┘
const double kOndexViewMinLat = 36.4;
const double kOndexViewMaxLat = 46.4;
const double kOndexViewMinLng = 54.9;
const double kOndexViewMaxLng = 74.1;

/// Nuqta marshrut hududidami (butun O'zbekiston).
bool ondexRouteCovers(double lat, double lng) =>
    lat >= kOndexRouteMinLat &&
    lat <= kOndexRouteMaxLat &&
    lng >= kOndexRouteMinLng &&
    lng <= kOndexRouteMaxLng;

/// Nuqta OnDexMap teskari geokodlash hududidami.
///
/// ⚠️ Hozircha faqat testlarda ishlatiladi. Manzil tanlash ekrani
/// koordinatani ChustApp backendiga yuboradi (`/geocode/reverse`),
/// u esa o'z zanjirini (Google → Yandex → 2GIS) ishlatadi — ya'ni
/// qamrov OnDexMap chegarasi bilan cheklanmaydi.
///
/// Funksiya SAQLANGAN, chunki u yagona joyda OnDexMap'ning haqiqiy
/// qamrovini bildiradi: ilova bir kun to'g'ridan-to'g'ri
/// `/v1/resolve` ga o'tsa, tekshiruv shu yerda tayyor turadi va
/// chegara marshrutnikidan farq qilishi test bilan qulflangan.
bool ondexReverseCovers(double lat, double lng) =>
    lat >= kOndexReverseMinLat &&
    lat <= kOndexReverseMaxLat &&
    lng >= kOndexReverseMinLng &&
    lng <= kOndexReverseMaxLng;

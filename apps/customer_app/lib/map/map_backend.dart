/// XARITA MOTORINI TANLASH — Google Maps yoki OnDexMap.
///
/// ┌─ BAYROQ SERVERDA (`GOOGLE_MAP`) ───────────────────────────────────┐
/// Qaysi motor ishlatilishini ilova EMAS, server hal qiladi: `.env` dagi
/// `GOOGLE_MAP` qiymati `GET /config/maps` orqali keladi
/// (`ondex_core` → `MapConfigService`). Ilovani qayta qurish shart emas.
///
///   * `GOOGLE_MAP=true`  — buyurtma oqimi (manzil tanlash, kuzatuv)
///     Google Maps bilan;
///   * `GOOGLE_MAP=false` — o'sha ekranlar OnDexMap bilan.
///
/// "OnDex Xarita" ekrani bundan mustasno: u HAR DOIM OnDexMap va hech
/// qachon Google'ga tushmaydi.
/// └────────────────────────────────────────────────────────────────────┘
///
/// ┌─ OnDexMap ISHLAMASA ───────────────────────────────────────────────┐
/// Buyurtma oqimidagi ekranlar xarita o'rniga xato ko'rsatmaydi: uslub
/// yuklanmasa yoki javob kelmasa `MapConfigService.reportOndexFailure`
/// chaqiriladi va ilova [MapConfigService.failureCooldown] davomida
/// Google'da turadi. Bu — xarita buyurtma berish va kuryerni kuzatishning
/// ikki muhim joyi bo'lgani uchun.
/// └────────────────────────────────────────────────────────────────────┘
///
/// Standart (server javob bermagan, qiymat hech qachon olinmagan) —
/// Google: bu ilgarigi production xatti-harakati.
library;

import 'package:ondex_core/ondex_core.dart' show MapConfigService;

/// Buyurtma oqimi ekranlari hozir OnDexMap bilan chizilsinmi.
///
/// Ekran ochilganda BIR MARTA o'qiladi (xarita ish paytida motor
/// almashtirmaydi) — faqat OnDexMap xatosida Google'ga tushadi.
bool get useOndexMapNow => MapConfigService.instance.useOndexMap;

/// OnDexMap uslubi shuncha vaqtda yuklanmasa — ishlamayapti deb hisoblanadi
/// va buyurtma oqimi ekrani Google'ga o'tadi.
const Duration kOndexStyleTimeout = Duration(seconds: 10);

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

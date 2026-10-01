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

// OnDexMap'ning o'zgarmas sozlamalari (manzil, uslub, hududlar) — kuryer
// ilovasi bilan umumiy paketda.
export 'package:ondex_maplibre/ondex_constants.dart';

/// Buyurtma oqimi ekranlari hozir OnDexMap bilan chizilsinmi.
///
/// Ekran ochilganda BIR MARTA o'qiladi (xarita ish paytida motor
/// almashtirmaydi) — faqat OnDexMap xatosida Google'ga tushadi.
bool get useOndexMapNow => MapConfigService.instance.useOndexMap;

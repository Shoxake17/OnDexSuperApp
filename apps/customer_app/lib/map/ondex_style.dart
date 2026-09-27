/// OnDexMap SUN'IY YO'LDOSH uslubi.
///
/// ┌─ NEGA KODDA, SERVERDA EMAS ────────────────────────────────────────┐
/// OnDexMap serveri ikki narsani beradi:
///
///   * `tiles/style.json` — oddiy (vektor) xarita uchun to'liq uslub;
///   * `tiles/satellite/{z}/{x}/{y}` — sun'iy yo'ldosh RASTER tile'lari.
///
/// Sun'iy yo'ldosh uchun alohida uslub fayli yo'q, chunki veb-klient
/// uni o'zi yig'adi. Bu yerda ham shunday: uslub — bitta raster
/// manbadan iborat sakkiz qatorlik JSON, uni server bilan
/// aloqalashtirishning hojati yo'q.
///
/// ⚠️ `attribution` maydoni ATAYLAB to'ldirilgan. MapLibre uni atribut
/// tugmasida ko'rsatadi; bo'sh qoldirilsa sun'iy yo'ldosh rejimida
/// kredit yo'qolardi.
/// └────────────────────────────────────────────────────────────────────┘
library;

import 'map_backend.dart';

/// Sun'iy yo'ldosh tile'lari (OnDexMap proksisi orqali — kalit serverda).
const String _satelliteTiles = 'https://maps.ondex.uz/tiles/satellite/{z}/{x}/{y}';

/// Rasm manbasining krediti.
///
/// Manba — Esri World Imagery; OnDexMap uni faqat uzatadi, shuning
/// uchun ikkala nom ham ko'rsatiladi.
const String _satelliteAttribution = '© OnDexMap · Esri World Imagery';

/// Sun'iy yo'ldosh uslubi — MapLibre uchun tayyor JSON.
///
/// `maxzoom` server proksisining chegarasi bilan bir xil: undan
/// yuqorida provayder rasm saqlamaydi va tile 404 qaytarardi.
/// MapLibre bu chegaradan yuqorida oxirgi mavjud tile'ni cho'zadi.
const String ondexSatelliteStyle = '''
{
  "version": 8,
  "sources": {
    "ondex-satellite": {
      "type": "raster",
      "tiles": ["$_satelliteTiles"],
      "tileSize": 256,
      "maxzoom": 18,
      "attribution": "$_satelliteAttribution"
    }
  },
  "layers": [
    {
      "id": "ondex-satellite",
      "type": "raster",
      "source": "ondex-satellite"
    }
  ]
}
''';

/// Oddiy (vektor) uslub manzili — `map_backend.dart` dan.
///
/// Shu yerda qayta e'lon qilinadi, chunki chaqiruvchi fayllar uslub
/// tanlashda ikkalasini yonma-yon ko'rishi qulay.
const String ondexVectorStyle = kOndexStyleUrl;

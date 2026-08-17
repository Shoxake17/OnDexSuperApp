/// OnDex — barcha Flutter ilovalari (mijoz, kuryer, admin panel,
/// restoran paneli) uchun UMUMIY yadro.
///
/// NEGA KERAK: audit natijasida bir xil kod bir necha marta yozilgani
/// aniqlandi — API klienti 4 marta, `formatSum` 5 marta, `fullImageUrl`
/// 4 marta, `ApiException` 4 marta, login ekrani 4 marta, WebSocket
/// qayta ulanish 4 marta.
///
/// Asosiy zarar ortiqcha kod EMAS, ajralib ketish edi: `401` ni ishlash
/// va HTTP timeout to'rtala nusxada ham yo'q edi, geolokatsiya timeout'i
/// esa faqat bittasida tuzatilgan edi. Endi har bir tuzatish BIR marta
/// qilinadi va to'rtala ilovaga birdan yetadi.
library ondex_core;

export 'src/api_client.dart';
// Kesh qatlami — "avval keshdan chiz, keyin tarmoqdan yangila".
// Ekranlar yuklanish mantig'ini O'ZI yozmaydi, faqat shu ikkitasini
// ishlatadi (`src/cache/repository.dart` izohiga qarang).
export 'src/cache/cache_store.dart';
export 'src/cache/repository.dart';
export 'src/config.dart';
export 'src/format.dart';
export 'src/live_bus.dart';
export 'src/order_status.dart';
export 'src/token_store.dart';
export 'src/ws_client.dart';

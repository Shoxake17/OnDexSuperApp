/// RS-1: yetkazish xaritasi ko'rinishi — platformaga qarab:
/// Windows — WebView2 (`webview_windows`), web — iframe (`package:web`).
///
/// Ikkalasi ham backenddagi `/courier-map` sahifasini ochadi (OnDexMap,
/// MapLibre; Google kaliti kerak emas) va ma'lumotni `postMessage` bilan
/// uzatadi — sahifaga token BERILMAYDI.
///
/// `kIsWeb` yetmaydi: `webview_windows` web'da kompilyatsiya bo'lmaydi —
/// shuning uchun conditional export (admin panelidagi `map_surface` naqshi).
library;

export 'delivery_map_view_webview.dart' if (dart.library.js_interop) 'delivery_map_view_web.dart';

import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';

import '../ondexmap/editor_view.dart';
import '../ondexmap/moderation_api.dart';
import '../ondexmap/moderation_view.dart';

/// OnDexMap bo'limi: ma'lumot kiritish muharriri va foydalanuvchi
/// ob'ektlari moderatsiyasi.
///
/// ┌─ IKKI QISM — IKKALASI HAM NATIV FLUTTER (WebView YO'Q) ────────────┐
///   • Muharrir  — mahalla va ko'cha chegaralarini xaritada chizish
///     (`flutter_map`; `ondexmap/editor_view.dart`).
///   • Takliflar — saytdagi foydalanuvchilar yuborgan ob'ektlarni
///     ko'rish, tuzatish, tasdiqlash yoki rad etish
///     (`ondexmap/moderation_view.dart`).
/// Tasdiqlangan ob'ekt OnDexMap xaritasida HAMMAGA ko'rinadi.
///
/// Ilgari muharrir OnDexMap serveri bergan Mapbox HTML sahifasi edi va
/// WebView2 ichida ochilardi; u o'chirildi, server endi faqat JSON beradi.
/// └────────────────────────────────────────────────────────────────────┘
///
/// ┌─ ARXITEKTURA ──────────────────────────────────────────────────────┐
/// OnDexMap — ALOHIDA loyiha (`F:\OnDexMap`), alohida repo, alohida
/// baza (PostGIS 5433) va alohida server. ChustApp uning bazasiga ham,
/// kodiga ham TEGMAYDI: ikkala qism faqat OnDexMap'ning LOKAL admin
/// serveri (`cmd/admin`) bilan gaplashadi.
///
/// Bu bo'lim ChustApp'ning boshqa bo'limlariga hech qanday ta'sir
/// qilmaydi: ChustApp API'siga so'rov yubormaydi, mavjud holatga tegmaydi
/// va `adminLive` soketidan foydalanmaydi.
/// └────────────────────────────────────────────────────────────────────┘
///
/// ┌─ NEGA 127.0.0.1 ───────────────────────────────────────────────────┐
/// OnDexMap'ning admin serveri YOZISH huquqiga ega, shuning uchun u
/// ataylab faqat lokal manzilga bog'langan va internetga chiqarilmaydi.
/// Ommaviy OnDexMap API'si (`:8090`) esa faqat O'QIY oladi (karantinga
/// yozishdan tashqari — u ham tasdiqlanmaguncha xaritaga tushmaydi).
///
/// Manzilni o'zgartirish kerak bo'lsa (masalan boshqa port):
///     flutter run --dart-define=ONDEXMAP_ADMIN_URL=http://127.0.0.1:9000
/// └────────────────────────────────────────────────────────────────────┘
class OndexMapPage extends StatefulWidget {
  const OndexMapPage({super.key});

  /// Muharrir manzili. Sir emas — shunchaki lokal URL.
  static const url = String.fromEnvironment(
    'ONDEXMAP_ADMIN_URL',
    defaultValue: 'http://127.0.0.1:8091',
  );

  @override
  State<OndexMapPage> createState() => _OndexMapPageState();
}

class _OndexMapPageState extends State<OndexMapPage> {
  final _api = ModerationApi(defaultUrl: OndexMapPage.url);
  int _section = 0;

  /// Kutilayotgan takliflar soni («Takliflar (3)» belgisi uchun).
  int _pending = 0;

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 10, 16, 6),
          child: Align(
            alignment: Alignment.centerLeft,
            child: SegmentedButton<int>(
              key: const ValueKey('ondexmap-sections'),
              showSelectedIcon: false,
              segments: [
                const ButtonSegment(
                  value: 0,
                  icon: Icon(Icons.map_outlined),
                  label: Text('Muharrir'),
                ),
                ButtonSegment(
                  value: 1,
                  icon: const Icon(Icons.inbox_outlined),
                  label: Text('Takliflar${_pending > 0 ? ' ($_pending)' : ''}'),
                ),
              ],
              selected: {_section},
              onSelectionChanged: (s) => setState(() => _section = s.first),
            ),
          ),
        ),
        // `IndexedStack`: ikkala qism ham tirik qoladi — bo'limlar
        // almashtirilganda xarita holati (masshtab, joy, yarim chizilgan
        // chegara) va yarim tahrirlangan taklif matni yo'qolmaydi.
        Expanded(
          child: kIsWeb
              ? const _WebNote()
              : IndexedStack(
                  index: _section,
                  children: [
                    OndexMapEditor(api: _api),
                    OndexMapModeration(
                      api: _api,
                      onPending: (n) {
                        if (mounted && n != _pending) setState(() => _pending = n);
                      },
                    ),
                  ],
                ),
        ),
      ],
    );
  }
}

/// WEB: OnDexMap bo'limi ishlamaydi — panel HTTPS'da, OnDexMap admin serveri
/// esa `http://127.0.0.1` da (brauzer aralash kontentni bloklaydi) va lokal
/// sessiya fayli brauzerga ko'rinmaydi.
class _WebNote extends StatelessWidget {
  const _WebNote();

  @override
  Widget build(BuildContext context) {
    return const Center(
      child: Padding(
        padding: EdgeInsets.all(32),
        child: Text(
          'OnDexMap muharriri va takliflar moderatsiyasi faqat desktop ilovada ishlaydi:\n'
          'OnDexMap admin serveri shu kompyuterda (127.0.0.1) turadi.',
          textAlign: TextAlign.center,
        ),
      ),
    );
  }
}

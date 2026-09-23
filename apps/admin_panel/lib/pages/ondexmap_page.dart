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
/// baza va alohida server. ChustApp uning bazasiga ham, kodiga ham
/// TEGMAYDI: ikkala qism faqat OnDexMap'ning admin serveri bilan
/// gaplashadi — LOKAL (`cmd/admin`, dev) yoki MASOFAVIY
/// (`cmd/adminserver`, production, Caddy ortida) — pastga qarang.
///
/// Bu bo'lim ChustApp'ning boshqa bo'limlariga hech qanday ta'sir
/// qilmaydi: ChustApp API'siga so'rov yubormaydi, mavjud holatga tegmaydi
/// va `adminLive` soketidan foydalanmaydi.
/// └────────────────────────────────────────────────────────────────────┘
///
/// ┌─ IKKI REJIM — MANZILGA QARAB AVTOMATIK ────────────────────────────┐
/// `ModerationApi.remote` = manzil loopback `http` EMAS degani:
///
///   • DEV (standart): `http://127.0.0.1:8091` — `cmd/admin` shu
///     kompyuterda ishlaydi, token lokal sessiya faylidan (kirish oynasi
///     YO'Q). OnDexMap'ning YOZISH huquqiga ega serveri ataylab faqat
///     shu manzilga bog'langan va internetga chiqarilmaydi.
///   • PRODUCTION (`config/prod.json`): `https://maps.ondex.uz/admin` —
///     Caddy orqali `cmd/adminserver`ga (VPS), token qo'lda kiritilgan
///     `ONDEXMAP_ADMIN_KEY` (yon panelning kalit tugmasi — pastga qarang;
///     birinchi urinishda xato chiqsa ham shu tugma ko'rinadi).
///
/// Manzilni qo'lda o'zgartirish (masalan boshqa port):
///     flutter run --dart-define=ONDEXMAP_ADMIN_URL=http://127.0.0.1:9000
/// └────────────────────────────────────────────────────────────────────┘
class OndexMapPage extends StatefulWidget {
  const OndexMapPage({super.key});

  /// Muharrir manzili — dev'da lokal URL, production'da
  /// `https://maps.ondex.uz/admin` (`config/prod.json`).
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

  /// Har safar oshadi: `IndexedStack` ichidagi ekranlarni MAJBURIY qayta
  /// yaratish uchun (`key` sifatida) — muharrir/moderatsiya `initState`da
  /// birinchi so'rovni qiladi, u YANGI kalit bilan qayta ishga tushishi
  /// kerak, lekin `_api` obyektining o'zi (shu bilan uning `hashCode`i)
  /// o'zgarmaydi — shuning uchun alohida hisoblagich.
  int _apiVersion = 0;

  /// Kalitni (qayta) kiritish — masofaviy rejimda, xato kutmasdan ham.
  Future<void> _changeKey() async {
    final saved = await showOndexMapAdminKeyDialog(context);
    if (mounted && saved) setState(() => _apiVersion++);
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 10, 16, 6),
          child: Row(
            children: [
              Expanded(
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
                        label: Text(
                            'Takliflar${_pending > 0 ? ' ($_pending)' : ''}'),
                      ),
                    ],
                    selected: {_section},
                    onSelectionChanged: (s) =>
                        setState(() => _section = s.first),
                  ),
                ),
              ),
              if (_api.remote)
                IconButton(
                  key: const ValueKey('ondexmap-change-key'),
                  tooltip: 'Admin kalitini o\'zgartirish',
                  icon: const Icon(Icons.key_outlined),
                  onPressed: _changeKey,
                ),
            ],
          ),
        ),
        // `IndexedStack`: ikkala qism ham tirik qoladi — bo'limlar
        // almashtirilganda xarita holati (masshtab, joy, yarim chizilgan
        // chegara) va yarim tahrirlangan taklif matni yo'qolmaydi.
        Expanded(
          child: kIsWeb
              ? const _WebNote()
              : IndexedStack(
                  key: ValueKey(_apiVersion),
                  index: _section,
                  children: [
                    OndexMapEditor(api: _api),
                    OndexMapModeration(
                      api: _api,
                      onPending: (n) {
                        if (mounted && n != _pending) {
                          setState(() => _pending = n);
                        }
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

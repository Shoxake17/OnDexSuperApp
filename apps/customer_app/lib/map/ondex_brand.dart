import 'dart:math' show Point;

import 'package:flutter/material.dart';
import 'package:maplibre_gl/maplibre_gl.dart' as ml;
import 'package:url_launcher/url_launcher.dart';

import 'map_backend.dart';

/// ONDEXMAP XARITASINING KO'RINISHI — brend yozuvi va ishga tushirish.
///
/// Bu fayl OnDexMap xaritasining ikkita umumiy jihatini saqlaydi:
/// MapLibre'ni to'g'ri rejimda ishga tushirish va brend yozuvi.
/// Ikkalasi ham manzil tanlash va kuryer kuzatuvi xaritalarida bir
/// xil bo'lishi kerak.

/// MapLibre'ni ilovaga mos rejimga o'tkazadi.
///
/// ┌─ NIMA HAL QILINADI ────────────────────────────────────────────────┐
/// Standart rejimda (`useHybridComposition = false`) MapLibre Android'da
/// `GLSurfaceView` ga chizadi va Flutter uni Virtual Display orqali
/// joylashtiradi. Paketning o'z hujjati bu yo'lning «z-order bilan
/// ma'lum cheklovlari» borligini aytadi.
///
/// Amalda bu shunday ko'rindi: xaritada masshtabni o'zgartirganda
/// (yoki xarita ustidagi Flutter vidjeti qayta chizilganda) xarita
/// bir lahza QORA bo'lib, keyin qaytadan paydo bo'lardi — xuddi
/// qayta yuklanayotgandek. Qurilma jurnalida bu `releaseSurfaces`
/// ning takror-takror chaqirilishi bo'lib ko'rinadi.
///
/// `TextureView` rejimida (`true`) xarita oddiy vidjetdek
/// birlashtiriladi: ustiga Flutter chiza oladi, qirqish va
/// animatsiya ishlaydi, surface qayta yaratilmaydi.
///
/// ⚠️ BIRINCHI xaritadan OLDIN chaqirilishi shart — ekranda turgan
/// xarita o'z rejimida qoladi. Shuning uchun `main()` da.
/// └────────────────────────────────────────────────────────────────────┘
void initOndexMap() {
  ml.MapLibreMap.useHybridComposition = true;
}

/// MapLibre'ning «i» tugmasini ekrandan chiqarib yuboradigan chekinish.
///
/// ┌─ NEGA SHUNDAY ─────────────────────────────────────────────────────┐
/// `maplibre_gl` da atribut tugmasini O'CHIRISH parametri yo'q —
/// faqat joylashuvi, chekinishi va rangi sozlanadi (`logoEnabled`
/// esa boshqa narsa: u MapLibre logotipi uchun).
///
/// Shuning uchun tugma katta chekinish bilan ko'rinadigan sohadan
/// tashqariga suriladi. Bu yagona yo'l va u barqaror: chekinish
/// nativ tarafda `layout_margin` ga aylanadi.
/// └────────────────────────────────────────────────────────────────────┘
const Point<num> kOndexHideAttribution = Point<num>(-10000, -10000);

/// Kamerani O'zbekiston bilan cheklaydi.
///
/// Xarita ma'lumotimiz faqat shu hududda mavjud; tashqarida
/// foydalanuvchi oq bo'shliq ko'rardi (`map_backend.dart` izohiga
/// qarang).
ml.CameraTargetBounds get ondexCameraBounds => ml.CameraTargetBounds(
      ml.LatLngBounds(
        southwest: const ml.LatLng(kOndexViewMinLat, kOndexViewMinLng),
        northeast: const ml.LatLng(kOndexViewMaxLat, kOndexViewMaxLng),
      ),
    );

/// Eng uzoqlashtirish darajasi — butun mamlakat ekranga sig'adigan
/// masshtab. Undan kichigi atrofdagi bo'sh joyni ochardi.
const double kOndexMinZoom = 5.2;

/* ── Brend ranglari — OnDexMap veb-saytidagi bilan AYNAN bir xil ──────
   Manba: `F:\OnDexMap\web\src\app\globals.css` (`.ondex-mark-on`,
   `.ondex-mark-dex`, `.ondex-credits`). Ikki mahsulotda bitta brend
   ko'rinishi kerak, shuning uchun raqamlar taxmin qilinmadi —
   ko'chirildi. */

/// «On» — to'q sariq.
const Color kOndexMarkOn = Color(0xFFF97316);

/// «Dex» — deyarli qora.
const Color kOndexMarkDex = Color(0xFF27272A);

/// Kredit yozuvi — kulrang.
const Color kOndexCreditText = Color(0xFF71717A);

/// OnDexMap brend yozuvi va krediti.
///
/// ┌─ VEB-SAYT BILAN BIR XIL ───────────────────────────────────────────┐
/// OnDexMap saytida bu ikki qism: katta «OnDex» yozuvi (`OnDexMark`,
/// 28 px, «On» sariq + «Dex» qora, oq halo bilan) va uning ostidagi
/// kichik «© OnDex map» krediti (11 px, oq yarim shaffof fonda).
///
/// Bu yerda ular AYNAN takrorlanadi — shrift o'lchami, ranglar va
/// oq halo ham. Farq faqat joylashuvda: saytda o'ng pastda, ilovada
/// chap pastda, chunki ilovada o'ng pastki burchakni xaritaning o'z
/// tugmalari (masshtab, joriy joylashuv) egallagan.
/// └────────────────────────────────────────────────────────────────────┘
///
/// «OnDex» yozuvining o'zi bosilmaydi — u xaritani surishga xalaqit
/// bermasligi kerak. Faqat «© OnDex map» krediti bosiladi va
/// mualliflik sahifasini ochadi: xarita ma'lumoti kimdan kelgani va
/// qanday shartlar bilan ishlatilishi o'sha yerda yozilgan.
class OndexMapBrand extends StatelessWidget {
  const OndexMapBrand({super.key, this.right = 12, this.bottom = 6});

  /// O'NG chekka — saytdagi bilan bir xil (`globals.css` →
  /// `.ondex-mark { right: 12px }`). Bitta brend ikki mahsulotda
  /// bir joyda turishi kerak.
  final double right;
  final double bottom;

  /// Mualliflik va litsenziya sahifasi.
  ///
  /// `/copyright` tanlandi, `/oferta` yoki `/maxfiylik` emas: bu yerda
  /// bosayotgan odam xarita MA'LUMOTI haqida so'rayapti — u kimniki
  /// va qaysi litsenziya bilan. Oferta esa API sotib olish shartlari
  /// haqida, ya'ni butunlay boshqa savolga javob berardi.
  static final Uri copyrightUrl = Uri.parse('https://console.ondex.uz/copyright');

  /// Oq halo — yozuv xaritada ham, sun'iy yo'ldosh tasvirida ham
  /// o'qiladi. Saytdagi `text-shadow: 0 0 4px #fff` ning aynan o'zi.
  static const List<Shadow> _halo = [
    Shadow(color: Colors.white, blurRadius: 4),
    Shadow(color: Colors.white, blurRadius: 4),
    Shadow(color: Colors.white, blurRadius: 2),
  ];

  @override
  Widget build(BuildContext context) {
    return Positioned(
      right: right,
      bottom: bottom,
      child: Column(
        // O'ng chekkaga tekislanadi: yozuv ham, kredit ham o'sha
        // chiziqda tugaydi.
        crossAxisAlignment: CrossAxisAlignment.end,
        mainAxisSize: MainAxisSize.min,
        children: [
          // «OnDex» — brend. Saytdagi kabi 28 px va qalin.
          // Bosilmaydi: xarita ustidagi katta yozuv barmoqni
          // «yutib» yuborardi.
          const IgnorePointer(
            child: Text.rich(
              TextSpan(children: [
                TextSpan(text: 'On', style: TextStyle(color: kOndexMarkOn)),
                TextSpan(text: 'Dex', style: TextStyle(color: kOndexMarkDex)),
              ]),
              style: TextStyle(
                fontSize: 28,
                height: 32 / 28,
                fontWeight: FontWeight.w800,
                letterSpacing: -0.5,
                shadows: _halo,
              ),
            ),
          ),
          // Kredit — brenddan ALOHIDA narsa: xarita ma'lumotining
          // manbasi. Saytda ham shunday, ikkalasi yonma-yon turadi.
          // Bu qism BOSILADI va mualliflik sahifasini ochadi.
          Material(
            color: Colors.white.withValues(alpha: 0.7),
            borderRadius: BorderRadius.circular(4),
            child: InkWell(
              borderRadius: BorderRadius.circular(4),
              onTap: () => launchUrl(copyrightUrl,
                  mode: LaunchMode.externalApplication),
              child: const Padding(
                padding: EdgeInsets.symmetric(horizontal: 4),
                child: Text(
                  '© OnDex map',
                  style: TextStyle(
                    fontSize: 11,
                    height: 18 / 11,
                    color: kOndexCreditText,
                  ),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

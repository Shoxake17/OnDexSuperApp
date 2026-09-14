/// Menyu ekranining umumiy mayda bo'laklari: rang uslubi, dumaloq
/// tugma, tarmoq rasmi, turkum chipi va bo'limlarga ajratish.
library;

import 'package:flutter/material.dart';
import 'package:ondex_core/ondex_core.dart' show apiBaseUrl, coreFullImageUrl;

import 'product_pricing.dart' show categoryOf;

/// Menyu brend rangi — mijoz ilovasidagi `kBrand` bilan AYNAN bir xil.
const kMenuBrand = Color(0xFFF4511E);

// ═══════════════════════════════════════════════════════════════════
// RANG USLUBI (yorug' / qorong'i)
// ═══════════════════════════════════════════════════════════════════

/// Menyu vidjetlarining ranglari.
///
/// ┌─ NEGA KERAK ──────────────────────────────────────────────────────┐
/// Kartochka va chiplar avval mijoz ilovasi uchun yozilgan va ranglar
/// ichida QATTIQ turardi (oq fon, qora matn, och kulrang chegara).
/// Affitsiant ilovasi esa qorong'i — menyuni o'sha ilovada ishlatganda
/// yorug' bo'lak paydo bo'lardi.
///
/// Endi ranglar mavzudan olinadi: ilova `ThemeData.extensions` ga
/// [MenuPalette] qo'shadi. Qo'shilmasa [MenuPalette.light] — mijoz ilovasi
/// hech narsa o'zgartirmaydi va avvalgidek ko'rinadi.
/// └───────────────────────────────────────────────────────────────────┘
@immutable
class MenuPalette extends ThemeExtension<MenuPalette> {
  const MenuPalette({
    required this.brand,
    required this.text,
    required this.mutedText,
    required this.imageBackground,
    required this.placeholderIcon,
    required this.controlBackground,
    required this.controlForeground,
    required this.controlDisabled,
    required this.chipBorder,
    required this.chipText,
    required this.discountPrice,
  });

  /// Faol chip va aksiya lentasi.
  final Color brand;

  /// Narx va nom. `null` — mavzudagi standart matn rangi.
  final Color? text;

  /// Chizilgan eski narx, og'irlik.
  final Color mutedText;

  /// Rasm yuklanguncha / rasm yo'q bo'lganda kartochka foni.
  final Color imageBackground;
  final Color placeholderIcon;

  /// "+ / −" tugmalari va miqdor belgisi.
  final Color controlBackground;
  final Color controlForeground;
  final Color controlDisabled;

  /// Faol bo'lmagan turkum chipi.
  final Color chipBorder;
  final Color chipText;

  /// Chegirmadagi narx.
  final Color discountPrice;

  /// Mijoz ilovasi — vidjetlarda avval qattiq yozilgan qiymatlarning
  /// AYNAN o'zi.
  static const light = MenuPalette(
    brand: kMenuBrand,
    text: null,
    mutedText: Color(0xFF757575),
    imageBackground: Color(0xFFF5F5F5),
    placeholderIcon: Color(0xFFBDBDBD),
    controlBackground: Colors.white,
    controlForeground: Colors.black,
    controlDisabled: Color(0xFFBDBDBD),
    chipBorder: Color(0xFFE0E0E0),
    chipText: Color(0xFF262626),
    discountPrice: Color(0xFFE53935),
  );

  /// Qorong'i ilovalar uchun tayyor to'plam (affitsiant ilovasi).
  static const dark = MenuPalette(
    brand: Color(0xFFF64E03),
    text: Color(0xFFF5F0E9),
    mutedText: Color(0x8AFFFFFF),
    imageBackground: Color(0xFF242424),
    placeholderIcon: Color(0x3DFFFFFF),
    controlBackground: Color(0xFF2C2C2C),
    controlForeground: Color(0xFFF5F0E9),
    controlDisabled: Color(0x3DFFFFFF),
    chipBorder: Color(0xFF2A2A2A),
    chipText: Color(0xB3FFFFFF),
    discountPrice: Color(0xFFFF6B6B),
  );

  /// Joriy mavzudagi uslub (berilmagan bo'lsa — yorug').
  static MenuPalette of(BuildContext context) =>
      Theme.of(context).extension<MenuPalette>() ?? light;

  @override
  MenuPalette copyWith({
    Color? brand,
    Color? text,
    Color? mutedText,
    Color? imageBackground,
    Color? placeholderIcon,
    Color? controlBackground,
    Color? controlForeground,
    Color? controlDisabled,
    Color? chipBorder,
    Color? chipText,
    Color? discountPrice,
  }) =>
      MenuPalette(
        brand: brand ?? this.brand,
        text: text ?? this.text,
        mutedText: mutedText ?? this.mutedText,
        imageBackground: imageBackground ?? this.imageBackground,
        placeholderIcon: placeholderIcon ?? this.placeholderIcon,
        controlBackground: controlBackground ?? this.controlBackground,
        controlForeground: controlForeground ?? this.controlForeground,
        controlDisabled: controlDisabled ?? this.controlDisabled,
        chipBorder: chipBorder ?? this.chipBorder,
        chipText: chipText ?? this.chipText,
        discountPrice: discountPrice ?? this.discountPrice,
      );

  @override
  MenuPalette lerp(ThemeExtension<MenuPalette>? other, double t) {
    if (other is! MenuPalette) return this;
    Color mix(Color a, Color b) => Color.lerp(a, b, t)!;
    return MenuPalette(
      brand: mix(brand, other.brand),
      text: Color.lerp(text, other.text, t),
      mutedText: mix(mutedText, other.mutedText),
      imageBackground: mix(imageBackground, other.imageBackground),
      placeholderIcon: mix(placeholderIcon, other.placeholderIcon),
      controlBackground: mix(controlBackground, other.controlBackground),
      controlForeground: mix(controlForeground, other.controlForeground),
      controlDisabled: mix(controlDisabled, other.controlDisabled),
      chipBorder: mix(chipBorder, other.chipBorder),
      chipText: mix(chipText, other.chipText),
      discountPrice: mix(discountPrice, other.discountPrice),
    );
  }
}

// ═══════════════════════════════════════════════════════════════════
// DUMALOQ IKON TUGMASI
// ═══════════════════════════════════════════════════════════════════

/// Doiradagi ikon tugmasi — xarita boshqaruvi, kartochkadagi "+/−" va
/// miqdor boshqaruvi shuni ishlatadi.
///
/// Ilgari uchta fayl (`address_screen`, `product_grid`, `qty_stepper`)
/// o'zining `_RoundButton` klassini saqlardi. Ular bir xil edi, faqat
/// soya va o'lcham farq qilardi — endi ular parametr. Ranglar berilmasa
/// [MenuPalette] dan olinadi.
class RoundIconButton extends StatelessWidget {
  const RoundIconButton({
    super.key,
    required this.icon,
    required this.onTap,
    this.size = 34,
    this.iconSize,
    this.elevation = 3,
    this.tooltip,
    this.iconColor,
    this.background,
  });

  final IconData icon;

  /// `null` — tugma o'chirilgan (bosilmaydi).
  final VoidCallback? onTap;

  final double size;

  /// Berilmasa o'lchamdan hisoblanadi.
  final double? iconSize;

  final double elevation;
  final String? tooltip;

  /// Berilmasa — [MenuPalette.controlForeground].
  final Color? iconColor;

  /// Berilmasa — [MenuPalette.controlBackground].
  final Color? background;

  @override
  Widget build(BuildContext context) {
    final style = MenuPalette.of(context);

    final btn = Material(
      color: background ?? style.controlBackground,
      shape: const CircleBorder(),
      elevation: elevation,
      child: InkWell(
        customBorder: const CircleBorder(),
        onTap: onTap,
        child: SizedBox(
          width: size,
          height: size,
          child: Icon(
            icon,
            size: iconSize ?? size * 0.56,
            color: onTap == null
                ? style.controlDisabled
                : (iconColor ?? style.controlForeground),
          ),
        ),
      ),
    );
    return tooltip == null ? btn : Tooltip(message: tooltip!, child: btn);
  }
}

// ═══════════════════════════════════════════════════════════════════
// TARMOQDAN KELADIGAN RASM
// ═══════════════════════════════════════════════════════════════════

/// Serverdagi rasm. Yo'l bo'sh yoki rasm yuklanmasa — EKRAN BUZILMAYDI,
/// o'rniga [placeholder] chiziladi.
///
/// `Image.network(..., errorBuilder: ...)` naqshi o'nta faylda qo'lda
/// takrorlanardi va ba'zilarida `errorBuilder` umuman yo'q edi — ya'ni
/// yiqilgan rasm qizil xato quticha bo'lib chiqardi.
class RemoteImage extends StatelessWidget {
  const RemoteImage({
    super.key,
    required this.url,
    this.width,
    this.height,
    this.fit = BoxFit.cover,
    this.placeholder,
  });

  /// Server bergan yo'l — nisbiy ham, to'liq URL ham bo'lishi mumkin
  /// (`coreFullImageUrl` ikkalasini ham to'g'ri ishlaydi — ilovalardagi
  /// `fullImageUrl` o'ramlari ham aynan shuni chaqiradi).
  final String url;

  final double? width;
  final double? height;
  final BoxFit fit;
  final Widget? placeholder;

  @override
  Widget build(BuildContext context) {
    final empty = placeholder ?? const SizedBox.shrink();
    if (url.trim().isEmpty) {
      return SizedBox(width: width, height: height, child: empty);
    }
    return Image.network(
      coreFullImageUrl(url, apiBaseUrl),
      width: width,
      height: height,
      fit: fit,
      // Rasm kelguncha JOY EGALLANADI — aks holda ro'yxatlar yuklanish
      // paytida sakrab qolardi.
      loadingBuilder: (context, child, progress) => progress == null
          ? child
          : SizedBox(width: width, height: height),
      errorBuilder: (_, _, _) =>
          SizedBox(width: width, height: height, child: empty),
    );
  }
}

// ═══════════════════════════════════════════════════════════════════
// MENYU BO'LIMLARI
// ═══════════════════════════════════════════════════════════════════

/// Menyuning bitta bo'limi — turkum va uning taomlari.
class MenuSection {
  final String title;
  final List<Map<String, dynamic>> items;
  const MenuSection(this.title, this.items);
}

/// Menyuni turkumlarga ajratadi. Tartib — serverdagi tartib: birinchi
/// uchragan turkum birinchi turadi (restoran panelida belgilangani).
List<MenuSection> buildMenuSections(List<Map<String, dynamic>> menu) {
  final order = <String>[];
  final map = <String, List<Map<String, dynamic>>>{};
  for (final p in menu) {
    final key = categoryOf(p);
    if (!map.containsKey(key)) {
      map[key] = [];
      order.add(key);
    }
    map[key]!.add(p);
  }
  return [for (final k in order) MenuSection(k, map[k]!)];
}

// ═══════════════════════════════════════════════════════════════════
// TURKUM CHIPI
// ═══════════════════════════════════════════════════════════════════

class MenuCategoryChip extends StatelessWidget {
  final String label;
  final bool active;
  final VoidCallback onTap;

  const MenuCategoryChip({
    super.key,
    required this.label,
    required this.active,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final style = MenuPalette.of(context);
    return Material(
      color: active ? style.brand : Colors.transparent,
      borderRadius: BorderRadius.circular(20),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(20),
        child: Container(
          alignment: Alignment.center,
          padding: const EdgeInsets.symmetric(horizontal: 14),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(20),
            border: Border.all(
              color: active ? style.brand : style.chipBorder,
            ),
          ),
          child: Text(
            label,
            style: TextStyle(
              fontSize: 13.5,
              fontWeight: active ? FontWeight.bold : FontWeight.normal,
              color: active ? Colors.white : style.chipText,
            ),
          ),
        ),
      ),
    );
  }
}

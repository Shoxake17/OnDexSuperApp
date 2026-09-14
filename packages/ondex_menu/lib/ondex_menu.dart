/// OnDex — restoran menyusining UMUMIY ko'rinish qismi.
///
/// ┌─ NEGA ALOHIDA PAKET ──────────────────────────────────────────────┐
/// Menyu ikki ilovada kerak: mijoz ilovasi (savat bilan) va affitsiant
/// ilovasi (stolga buyurtma kiritish). Taom kartochkasi, to'r, turkum
/// chiplari va chegirma hisobi ikki joyda alohida yozilsa, vaqt o'tib
/// narx yoki aksiya lentasi ikki ilovada turlicha ko'rinardi —
/// affitsiant mehmonga ilovadagidan boshqa narx aytib qo'yardi.
///
/// Shuning uchun ko'rinish va narx mantig'i SHU paketda. Ilovaga xos
/// narsalar (savat, sevimlilar) ilovaning o'zida qoladi va kartochkaga
/// uya (`ProductCard.topLeft`) orqali qo'yiladi.
/// └───────────────────────────────────────────────────────────────────┘
library;

export 'src/menu_widgets.dart';
export 'src/product_card.dart';
export 'src/product_pricing.dart';

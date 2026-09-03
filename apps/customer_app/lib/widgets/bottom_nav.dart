// ┌─ ILOVANING PASTKI MENYUSI — IKKI QOBIQ UCHUN UMUMIY ──────────────┐
// Endi ilovada ikkita pastki menyu bor:
//
//   `home_shell.dart`        — SUPER ILOVA menyusi:
//                              Bosh sahifa · [Shaddiy] · Profil
//   `restaurant_shell.dart`  — RESTORAN bo'limi menyusi:
//                              Bosh sahifa · Savat · [QR] · Sevimlilar
//                              · Buyurtmalar
//
// Ular bir xil ko'rinishi SHART: bir xil balandlik, bir xil oq fon,
// markazda bir xil "o'yilgan" tugma. Ikki joyda ikki nusxa kod tursa,
// ulardan biri o'zgarib qolib menyular bir-biridan farq qila
// boshlardi.
// └───────────────────────────────────────────────────────────────────┘
import 'package:flutter/material.dart';

/// Brend rangi — tanlangan bo'lim ikonasi va yozuvi shu rangda.
const kNavBrand = Color(0xFFF4511E);

/// Shaddiy brendi — binafsha. Qolgan ilova to'q sariq bo'lgani uchun
/// bu ATAYLAB farq qiladi: markazdagi tugma "boshqa olam" ga olib
/// kirishini ko'rsatadi.
const kShaddiyBrand = Color(0xFF5B5BF6);
const _kShaddiyRingEnd = Color(0xFF9B8AFB);

/// Pastki menyuning bitta bo'limi.
class NavSpec {
  final IconData icon;
  final IconData activeIcon;
  final String label;
  final bool selected;
  final VoidCallback onTap;

  /// Ikona ustidagi son (savatdagi taomlar soni). 0 — belgi
  /// chizilmaydi.
  final int badge;

  const NavSpec({
    required this.icon,
    required this.activeIcon,
    required this.label,
    required this.selected,
    required this.onTap,
    this.badge = 0,
  });
}

/// Pastki menyu paneli.
///
/// Bo'limlar markazdagi tugmaga nisbatan IKKI GURUHGA bo'linadi
/// ([left] va [right]) — markazda suzuvchi tugma uchun bo'sh joy
/// qoladi. Tugma bu panelning ICHIDA emas: u
/// `Scaffold.floatingActionButton` (`centerDocked`) sifatida ustidan
/// tushib turadi, ya'ni panel balandligiga ta'sir qilmaydi.
///
/// [hasCenterButton] `false` bo'lsa markazdagi bo'shliq ham
/// qoldirilmaydi — aks holda menyu o'rtasida sababsiz teshik
/// ko'rinardi (masalan AI o'chirilgan serverda).
///
/// `NavigationBar` O'RNIGA qo'lda qurilgan — Material'ning
/// `NavigationBar`i markazga tugma qo'ya olmaydi.
class OndexBottomBar extends StatelessWidget {
  final List<NavSpec> left;
  final List<NavSpec> right;
  final bool hasCenterButton;

  const OndexBottomBar({
    super.key,
    required this.left,
    required this.right,
    this.hasCenterButton = true,
  });

  /// Panelning o'z balandligi (tizim chekinishisiz).
  static const barHeight = 64.0;

  @override
  Widget build(BuildContext context) {
    return Material(
      // Panel rangi ATAYLAB qat'iy oq: `colorScheme.surface` mavzuga
      // bog'liq va qurilma sozlamasiga qarab kulrang tusga kirardi.
      color: Colors.white,
      elevation: 8,
      // ┌─ SAFEAREA SHART ────────────────────────────────────────────┐
      // Busiz menyu ekranning eng pastiga chizilardi va Android'ning
      // TIZIM navigatsiya paneli uning ustiga tushardi: uch tugmali
      // navigatsiyada (48dp) yozuvlar tizim tugmalari ostida qolib,
      // markazdagi tugma yarim yashirinardi.
      //
      // Jest navigatsiyali telefonda past chiziq atigi ~24dp bo'lgani
      // uchun bu deyarli sezilmasdi — xato AYNAN uch tugmali qurilmada
      // ko'rindi.
      // └─────────────────────────────────────────────────────────────┘
      child: SafeArea(
        top: false,
        child: SizedBox(
          height: barHeight,
          child: Row(
            children: [
              for (final s in left) _NavItem(spec: s),
              // Markazdagi bo'sh joy — tugma bu qatorda EMAS.
              if (hasCenterButton) const Expanded(child: SizedBox.shrink()),
              for (final s in right) _NavItem(spec: s),
            ],
          ),
        ),
      ),
    );
  }
}

class _NavItem extends StatelessWidget {
  final NavSpec spec;

  const _NavItem({required this.spec});

  @override
  Widget build(BuildContext context) {
    final color =
        spec.selected ? kNavBrand : Theme.of(context).colorScheme.outline;
    return Expanded(
      child: InkResponse(
        onTap: spec.onTap,
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Stack(
              clipBehavior: Clip.none,
              children: [
                Icon(spec.selected ? spec.activeIcon : spec.icon,
                    size: 24, color: color),
                if (spec.badge > 0)
                  Positioned(
                    right: -6,
                    top: -4,
                    child: Container(
                      padding: const EdgeInsets.symmetric(horizontal: 4),
                      constraints: const BoxConstraints(minWidth: 16),
                      height: 16,
                      alignment: Alignment.center,
                      decoration: BoxDecoration(
                        color: kNavBrand,
                        borderRadius: BorderRadius.circular(20),
                      ),
                      // Nuqta emas, RAQAM: "nechta?" degan savol
                      // nuqtadan javob olmaydi.
                      child: Text(
                        spec.badge > 9 ? '9+' : '${spec.badge}',
                        style: const TextStyle(
                          fontSize: 9.5,
                          fontWeight: FontWeight.bold,
                          color: Colors.white,
                          height: 1,
                        ),
                      ),
                    ),
                  ),
              ],
            ),
            const SizedBox(height: 2),
            // `maxLines: 1` + kichik o'lcham — "Bosh sahifa" tor
            // ekranlarda ikki qatorga bo'linib, balandlikni buzardi.
            Text(
              spec.label,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                fontSize: 11,
                fontWeight: spec.selected ? FontWeight.w600 : FontWeight.w500,
                color: color,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Markazdagi Shaddiy tugmasi — SUPER ILOVA menyusida.
///
/// Belgi (`Icons.auto_awesome` kabi) o'rniga AYNAN yuz turadi: Shaddiy
/// ilovasida ham foydalanuvchi shu yuzni ko'radi va ikki ilovada
/// bitta yordamchi ekani shu tafsilotdan bilinadi.
class ShaddiyFab extends StatelessWidget {
  final VoidCallback onTap;

  // ┌─ TUGMA HAR DOIM BIR XIL ────────────────────────────────────────┐
  // Ilgari bu yerda `alert` bayrog'i bor edi: yordamchi ishlamasa
  // yuz kulrangga aylanib, burchagida qizil "!" chiqardi.
  //
  // U OLIB TASHLANDI. Sabab: status bir lahzalik va ko'pincha
  // shunchaki tarmoq sekinligini bildirardi — foydalanuvchi esa
  // ilovada doimiy "buzuq" belgisini ko'rib turardi. Nosozlik endi
  // AYNAN kerak bo'lgan paytda aytiladi: odam yozgandan keyin,
  // suhbatning ichida (`AssistantScreen._assistantError`).
  // └─────────────────────────────────────────────────────────────────┘

  const ShaddiyFab({super.key, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: 'Shaddiy Ai Agent',
      child: InkResponse(
        onTap: onTap,
        radius: 38,
        child: Container(
          width: 64,
          height: 64,
          // Oq halqa — tugma panel ustiga "o'yib" qo'yilgandek
          // ko'rinadi (QR tugmasi bilan bir xil naqsh).
          padding: const EdgeInsets.all(3),
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            color: Colors.white,
            boxShadow: [
              BoxShadow(
                color: kShaddiyBrand.withValues(alpha: 0.40),
                blurRadius: 14,
                offset: const Offset(0, 5),
              ),
            ],
          ),
          child: Container(
            padding: const EdgeInsets.all(2.5),
            decoration: const BoxDecoration(
              shape: BoxShape.circle,
              gradient: LinearGradient(
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
                colors: [kShaddiyBrand, _kShaddiyRingEnd],
              ),
            ),
            child: const ClipOval(
              child: Image(
                image: AssetImage('assets/shaddiy/face_idle.jpg'),
                fit: BoxFit.cover,
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// Markazdagi QR tugmasi — RESTORAN bo'limi menyusida.
///
/// ┌─ NEGA AYNAN SHU YERDA ────────────────────────────────────────────┐
/// Stol QR kodi FAQAT restoran oqimiga tegishli: u skanerlangach
/// savatga stol seansi yoziladi va menyu ochiladi. Super ilovaning
/// bosh sahifasida (bank, taksi, dorixona kartalari orasida) bu tugma
/// mazmunsiz edi — shuning uchun u restoran qobig'iga ko'chirildi.
/// └───────────────────────────────────────────────────────────────────┘
class QrFab extends StatelessWidget {
  final VoidCallback onTap;

  const QrFab({super.key, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: 'Stol QR kodi',
      child: InkResponse(
        onTap: onTap,
        radius: 38,
        child: Container(
          width: 64,
          height: 64,
          // Oq halqa — tugma panel ustiga "o'yib" qo'yilgandek
          // ko'rinadi. Busiz u shunchaki panelga yopishgan doira
          // bo'lib qolardi.
          padding: const EdgeInsets.all(4),
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            color: Colors.white,
            boxShadow: [
              BoxShadow(
                color: kNavBrand.withValues(alpha: 0.35),
                blurRadius: 14,
                offset: const Offset(0, 5),
              ),
            ],
          ),
          child: Container(
            decoration: const BoxDecoration(
              shape: BoxShape.circle,
              gradient: LinearGradient(
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
                colors: [Color(0xFFFF7043), kNavBrand],
              ),
            ),
            child: const Icon(Icons.qr_code_2, color: Colors.white, size: 30),
          ),
        ),
      ),
    );
  }
}

import 'package:flutter/material.dart';
// `SystemUiOverlayStyle` uchun — tizim paneli uslubi shu ekranda
// belgilanadi (izohga qarang).
import 'package:flutter/services.dart';

import '../data/ai_status.dart';
import '../services/push.dart';
import '../widgets/bottom_nav.dart';
import '../widgets/page_sheet.dart';
import 'assistant_screen.dart';
import 'profile_screen.dart';
import 'super_home_screen.dart';
import 'table_qr_flow.dart';

/// SUPER ILOVANING pastki menyu (bottom navigation) qobig'i.
///
/// ┌─ NEGA MENYUDA ATIGI IKKI BO'LIM ──────────────────────────────────┐
/// Ilgari bu yerda to'rtta bo'lim turardi: Bosh sahifa, Buyurtmalar,
/// Sevimlilar, Profil — va markazda QR tugmasi. Bu OnDex "ovqat
/// yetkazish ilovasi" bo'lgan davrdan qolgan edi.
///
/// OnDex endi super ilova: bosh sahifada bank, taksi, dorixona va
/// restoran yonma-yon turadi. "Savat", "Sevimlilar", "Buyurtmalar" va
/// stol QR kodi esa FAQAT ovqatga tegishli — taksi chaqirayotgan
/// odamga ular mazmunsiz va ilovani noto'g'ri tanitadi.
///
/// Shuning uchun super menyuda faqat hamma xizmatga tegishli narsa
/// qoldi:
///
///     Bosh sahifa · [Shaddiy] · Profil
///
/// Ovqatga tegishli to'rtta bo'lim yo'qolmadi — ular
/// `restaurant_shell.dart` ga ko'chdi va "Restoran" kartasi bosilganda
/// o'sha qobiq bilan birga ochiladi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// Bo'limlar `Stack` + `Visibility(maintainState: true)` ichida
/// saqlanadi — tab almashtirilganda oldingi holat yo'qolmaydi.
class HomeShell extends StatefulWidget {
  const HomeShell({super.key});

  @override
  State<HomeShell> createState() => _HomeShellState();
}

class _HomeShellState extends State<HomeShell> {
  int _index = 0;

  // `_miniAppKey` va `_webViewUrl` OLIB TASHLANDI (2-bosqich): bosh
  // sahifa native bo'lgach doimiy WebView qolmadi. Menyu, savat va
  // checkout endi alohida PUSH qilingan ekranlar sifatida ochiladi —
  // ular o'z `Scaffold`iga ega, ya'ni pastki menyuni qo'lda yashirish
  // ham kerak emas (eski `_hideBottomBar` mantig'i shu sabab o'chdi).
  late final _tabs = <Widget>[
    // 3-BOSQICH: bosh sahifa endi SUPER APP (`super_home_screen.dart`).
    //
    // Katalog yo'qolmadi — u "Restoran" kartasi orqali, o'z qobig'i
    // bilan ochiladi (`restaurant_shell.dart`).
    SuperHomeScreen(onScanQr: () => scanTableQr(context)),
    const ProfileScreen(),
  ];

  @override
  void initState() {
    super.initState();
    // Yordamchi holatini OLDINDAN so'raymiz — tugma bosilganda
    // kutish bo'lmasin. Javob kelmasa ham tugma chiziladi: sabab
    // bosilganda aytiladi (`AiStatus` izohiga qarang).
    AiStatus.instance.refresh();
    // Push tokenini ro'yxatdan o'tkazish AYNAN SHU YERDA.
    //
    // NEGA bu joy: `HomeShell` — sessiya ochilganidan keyingi YAGONA
    // kirish nuqtasi. Kirishning beshta yo'li bor (parol, SMS kodi,
    // Telegram, Google, parolni tiklash) va ularning har biriga
    // alohida chaqiruv qo'yilsa, kelajakda qo'shiladigan oltinchi yo'l
    // jimgina unutilardi — foydalanuvchi push olmay qo'yardi va buni
    // hech kim sezmasdi.
    //
    // `await` qilinmaydi: tarmoq so'rovi ekran chizilishini kutib
    // turmasligi kerak.
    PushService.instance.start();
  }

  @override
  Widget build(BuildContext context) {
    // ┌─ TIZIM PANELI USLUBI SHU YERDA ─────────────────────────────┐
    // Ilgari uni har ekran o'zi belgilardi, lekin ular `SafeArea`
    // ICHIDA edi — ya'ni belgilangan hudud ekranning eng tepasiga
    // YETMASDI va Flutter uslubni umuman qo'llamasdi. Natijada bosh
    // sahifada soat va Wi-Fi belgilar oq holicha qolib, oq fonda
    // ko'rinmay ketardi.
    //
    // Bu yerda `AnnotatedRegion` butun ekranni qamraydi va menyudan
    // qaytilganda uslub o'z-o'zidan tiklanadi.
    // └─────────────────────────────────────────────────────────────┘
    return AnnotatedRegion<SystemUiOverlayStyle>(
      // Ikkala tab ham bir xil: OQ tizim paneli, qora belgilar. Ular
      // dumaloq kartasiz, oq fon bilan boshlanadi. Qora panel faqat
      // PUSH qilingan sahifalarda (`PageSheet`) — restoran qobig'i
      // ham shular qatorida.
      value: PageSheet.light,
      child: Scaffold(
        // ┌─ FON HAR DOIM OQ ───────────────────────────────────────────┐
        // Ilgari bu rang tab'ga qarab QORA bo'lardi — u tizim paneli
        // ostidagi chiziqni bo'yash uchun edi. Lekin Scaffold foni
        // BUTUN ekranni bo'yaydi, shu jumladan pastki menyuning
        // ko'tarilgan tugmasi uchun qoldirilgan shaffof joyni ham.
        // Natijada menyu ustida QORA chiziq paydo bo'lgan edi.
        //
        // Endi tizim paneli ostidagi chiziq `body` ichida ALOHIDA
        // chiziladi (pastga qarang) va pastki menyuga tegmaydi.
        // └─────────────────────────────────────────────────────────────┘
        backgroundColor: Colors.white,
        body: Column(
          children: [
            // Tizim paneli egallagan chiziq — barcha tab'larda OQ.
            //
            // `SafeArea` o'rniga aniq balandlik: SafeArea butun `body`ni
            // qamrab, pastki menyuga ham ta'sir qilardi.
            Container(
              height: MediaQuery.of(context).padding.top,
              color: Colors.white,
            ),
            Expanded(
              // MUHIM (haqiqiy Android qurilmada topilgan bug):
              // `IndexedStack` platform view (WebView) bilan
              // ishlatilganda teginish UMUMAN yetib bormay qolardi.
              // `Stack` + `Visibility(maintainState: true)` bir xil
              // holat saqlash xususiyatini beradi, lekin hit-testing
              // to'g'ri ishlaydi.
              child: Stack(
                children: [
                  for (var i = 0; i < _tabs.length; i++)
                    Visibility(
                      visible: i == _index,
                      maintainState: true,
                      child: _tabs[i],
                    ),
                ],
              ),
            ),
          ],
        ),
        // ┌─ MARKAZDAGI TUGMA — SUZUVCHI ───────────────────────────────┐
        // Ilgari u pastki panelning ICHIDA edi va panel uning
        // ko'tarilishi uchun 28px qo'shimcha balandlik ajratardi. O'sha
        // bo'sh oq chiziq tarkibni bosib turardi.
        //
        // Endi tugma `floatingActionButton` — u panel ustida SUZADI,
        // hech qanday joy egallamaydi. `centerDocked` uni panelning
        // yuqori chetiga aniq markazlaydi.
        //
        // TUGMA HAR DOIM BIR XIL. Yordamchi o'chiq bo'lsa ham,
        // server javob bermasa ham u YASHIRILMAYDI, TO'SILMAYDI va
        // HECH QANDAY belgi bilan belgilanmaydi.
        //
        // Sabab: status bir lahzalik va ko'pincha shunchaki tarmoq
        // sekinligini bildiradi — doimiy "buzuq" belgisi esa ilovani
        // ishonchsiz ko'rsatardi. Nosozlik AYNAN kerak bo'lgan paytda
        // aytiladi: odam yozgandan keyin, suhbatning ichida
        // (`AssistantScreen._assistantError`).
        // └─────────────────────────────────────────────────────────────┘
        floatingActionButton: ShaddiyFab(
          onTap: () => AssistantScreen.open(context),
        ),
        floatingActionButtonLocation: FloatingActionButtonLocation.centerDocked,
        bottomNavigationBar: OndexBottomBar(
          left: [
            NavSpec(
              icon: Icons.home_outlined,
              activeIcon: Icons.home,
              label: 'Bosh sahifa',
              selected: _index == 0,
              onTap: () => setState(() => _index = 0),
            ),
          ],
          right: [
            NavSpec(
              icon: Icons.person_outline,
              activeIcon: Icons.person,
              label: 'Profil',
              selected: _index == 1,
              onTap: () => setState(() => _index = 1),
            ),
          ],
        ),
      ),
    );
  }
}

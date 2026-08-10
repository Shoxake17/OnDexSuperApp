// "Orqaga" tugmasi uchun umumiy mantiq.
//
// Sahifa IKKI xil kontekstda ochilishi mumkin:
//  1. Home tab'idagi doimiy WebView ichida (Bosh sahifa -> Menyu -> Savat...)
//     — bu yerda orqaga qaytish oddiy Next.js navigatsiyasi.
//  2. Native ekrandan chuqur havola orqali (Istaklarim -> menyu,
//     Buyurtmalarim -> buyurtma holati) — bu yerda WebView Flutter
//     Navigator'iga PUSH qilingan alohida ekran ichida. Bunda `router.push`
//     foydalanuvchini o'sha push qilingan ekran ichida qoldiradi (masalan
//     "Buyurtmalarim"dan chiqib, o'sha oynada Bosh sahifa ochiladi) —
//     chalkash. To'g'risi: native ekranni yopish.
//
// `FlutterNavPop` kanali FAQAT 2-holatda mavjud (mini_app_webview.dart'da
// `onClose` berilganda ro'yxatdan o'tadi), shuning uchun uning bor-yo'qligi
// kontekstni aniqlashning ishonchli usuli.

declare global {
  interface Window {
    FlutterNavPop?: { postMessage: (msg: string) => void };
  }
}

export function goBack(fallback: () => void) {
  if (typeof window !== "undefined" && window.FlutterNavPop) {
    window.FlutterNavPop.postMessage("pop");
    return;
  }
  fallback();
}

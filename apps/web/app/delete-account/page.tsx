import DeleteAccountClient from "./delete-account-client";

export const metadata = { title: "Akkauntni o'chirish" };

// https://ondex.uz/delete-account — Google Play / App Store'ning
// "hisobni o'chirish" talabi: ilova o'rnatilmagan holatda ham
// ishlaydigan veb sahifa.
//
// ┌─ NEGA MAVJUD SESSIYA TEKSHIRILMAYDI (login/page.tsx dan farqli) ───┐
// Kirish sahifasi allaqachon kirgan odamni to'g'ridan-to'g'ri
// maqsadga yo'naltiradi. Bu yerda esa AKSINCHA: hatto sessiya bor
// bo'lsa ham telefon+kod oqimi QASDDAN takrorlanadi — shunda
// o'chirish so'rovi HAR DOIM "yangi tasdiqlangan" (`phoneProven`)
// bo'ladi va joriy parol so'ralmaydi (`internal/users/register.go`
// dagi `DeleteAccount` qoidasi). Bitta oddiy oqim — ikkinchi, parol
// so'raydigan shoxobcha yo'q.
// └───────────────────────────────────────────────────────────────────┘
export default function DeleteAccountPage() {
  return <DeleteAccountClient />;
}

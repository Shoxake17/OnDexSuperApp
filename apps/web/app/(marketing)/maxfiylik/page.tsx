import type { Metadata } from "next";

import {
  Clause,
  DataTable,
  LEGAL_EFFECTIVE_DATE,
  LegalPage,
  OPERATOR,
  Section,
  req,
} from "../legal/legal-ui";

/**
 * Maxfiylik siyosati — butun OnDex ekotizimi uchun.
 *
 * ┌─ BU SHABLON EMAS ──────────────────────────────────────────────────┐
 * Quyidagi ma'lumotlar ro'yxati internetdan olingan namuna emas —
 * u KODDAN inventarizatsiya qilingan:
 *
 *   `internal/users/user.go`            — profil maydonlari
 *   `internal/users/user.go`            — AddressDetails
 *   `internal/orders/order.go`          — buyurtma yozuvi
 *   `internal/storage/migrations/0034`  — user_devices
 *   `internal/storage/migrations/0030`  — device_tokens (push)
 *   `internal/couriers/courier.go`      — kuryer joylashuvi
 *   `internal/httpapi/geoext.go`        — Yandex/2GIS geokoderlari
 *   `internal/payments/octo`            — to'lov provayderi
 *   `internal/notify/eskiz.go`          — SMS
 *   `internal/notify/fcm.go`            — push
 *   `internal/assistant/`               — AI yordamchi va ovoz
 *
 * Shu sabab hujjat haqiqatni tasvirlaydi. Kod o'zgarsa — bu sahifa
 * ham o'zgarishi SHART: yangi ma'lumot turi yoki yangi uchinchi
 * tomon qo'shilsa, quyidagi jadvalga qator qo'shiladi.
 * └────────────────────────────────────────────────────────────────────┘
 *
 * ┌─ HUQUQIY KO'RIB CHIQISH ───────────────────────────────────────────┐
 * Matn O'zbekiston Respublikasining «Shaxsga doir ma'lumotlar
 * to'g'risida»gi Qonuni tuzilishiga moslab yozilgan, LEKIN u yurist
 * ko'rigidan o'tishi kerak. Bu — texnik jihatdan to'g'ri va to'liq
 * loyiha, yakuniy huquqiy xulosa emas.
 * └────────────────────────────────────────────────────────────────────┘
 */

export const metadata: Metadata = {
  title: "Maxfiylik siyosati — OnDex",
  description:
    "OnDex ekotizimi qanday shaxsiy ma'lumotlarni yig'adi, nima uchun " +
    "ishlatadi, kimga uzatadi va foydalanuvchi qanday huquqlarga ega.",
};

export default function PrivacyPage() {
  return (
    <LegalPage
      title="Maxfiylik siyosati"
      subtitle="OnDex ekotizimi: mijoz ilovasi, kuryer ilovasi, affitsiant ilovasi, restoran va ma’muriyat panellari, ondex.uz sayti va Telegram Mini App."
    >
      <Section n="1" title="Umumiy qoidalar">
        <Clause n="1.1">
          Ushbu Maxfiylik siyosati (keyingi o‘rinlarda — <b>Siyosat</b>)
          foydalanuvchining shaxsga doir ma’lumotlari OnDex ekotizimida
          qanday yig‘ilishi, ishlanishi, saqlanishi va uzatilishini
          belgilaydi.
        </Clause>
        <Clause n="1.2">
          Ma’lumotlar operatori (keyingi o‘rinlarda — <b>Operator</b>):{" "}
          {req(OPERATOR.legalName)}, STIR {req(OPERATOR.taxId)}, davlat
          ro‘yxatidan o‘tganlik to‘g‘risidagi ma’lumotlar:{" "}
          {req(OPERATOR.registration)}, yuridik manzil:{" "}
          {req(OPERATOR.address)}.
        </Clause>
        <Clause n="1.3">
          Siyosat quyidagi mahsulotlarning <b>barchasiga</b> tegishli:
        </Clause>
        <ul className="ml-5 list-disc space-y-1 text-neutral-800">
          <li>OnDex — mijoz ilovasi (Android, iOS);</li>
          <li>OnDex Kuryer — kuryerlar uchun ilova;</li>
          <li>OnDex Affitsiant — restoran zali uchun ilova;</li>
          <li>OnDex Restoran — restoran boshqaruv paneli;</li>
          <li>OnDex Admin — platforma ma’muriyati paneli;</li>
          <li>
            <a href={OPERATOR.site} className="underline">
              ondex.uz
            </a>{" "}
            sayti va uning Telegram Mini App ko‘rinishi;
          </li>
          <li>OnDex Xarita (OnDexMap) — manzil ma’lumotlari xizmati.</li>
        </ul>
        <Clause n="1.4">
          Ilovadan yoki saytdan foydalanishni boshlash — ushbu Siyosat
          shartlariga <b>rozilik</b> bildirish demakdir. Rozilik bermaslik
          huquqi foydalanuvchida saqlanadi; bunda xizmatdan foydalanish
          imkoni bo‘lmaydi.
        </Clause>
        <Clause n="1.5">
          Siyosat O‘zbekiston Respublikasining «Shaxsga doir ma’lumotlar
          to‘g‘risida»gi Qonuni va boshqa amaldagi qonun hujjatlari asosida
          ishlab chiqilgan.
        </Clause>
      </Section>

      <Section n="2" title="Qanday ma’lumotlar yig‘iladi">
        <Clause n="2.1">
          Operator quyidagi ma’lumotlarni <b>faqat</b> ko‘rsatilgan
          maqsadlarda yig‘adi va ishlaydi. Boshqa hech qanday ma’lumot
          yig‘ilmaydi.
        </Clause>

        <h3 className="mt-6 font-semibold">2.2. Barcha foydalanuvchilar uchun</h3>
        <DataTable
          rows={[
            {
              what: "Telefon raqami",
              why: "Kimlikni aniqlash va tizimga kirish. Bu — yagona majburiy identifikator.",
              basis: "Shartnomani bajarish",
              keep: "Akkaunt o‘chirilgunga qadar",
            },
            {
              what: "Ism va familiya",
              why: "Buyurtmada murojaat qilish, kuryer va restoran uchun.",
              basis: "Shartnomani bajarish",
              keep: "Akkaunt o‘chirilgunga qadar",
            },
            {
              what: "Elektron pochta (ixtiyoriy)",
              why: "Kirishning muqobil usuli va xabarnomalar.",
              basis: "Rozilik",
              keep: "Foydalanuvchi o‘chirgunga qadar",
            },
            {
              what: "Parol (agar o‘rnatilgan bo‘lsa)",
              why: "Kirish. Parolning O‘ZI saqlanmaydi — faqat qaytarib bo‘lmaydigan kriptografik xesh (Argon2id).",
              basis: "Shartnomani bajarish",
              keep: "Akkaunt o‘chirilgunga qadar",
            },
            {
              what: "Telegram hisobi identifikatori",
              why: "Telegram orqali kirish va bot xabarnomalari. Faqat raqamli ID; Telegram’dagi yozishmalar OLINMAYDI.",
              basis: "Rozilik",
              keep: "Bog‘lanish bekor qilinguncha",
            },
            {
              what: "Bir martalik tasdiqlash kodi (SMS/Telegram/e-pochta)",
              why: "Telefon raqami yoki pochtaga egalikni tasdiqlash.",
              basis: "Shartnomani bajarish",
              keep: "5 daqiqa, so‘ng avtomatik o‘chiriladi",
            },
            {
              what: "Qurilma turi va ilova versiyasi",
              why: "Texnik qo‘llab-quvvatlash va nosozliklarni aniqlash. Qurilmaning noyob identifikatori (IMEI, reklama ID) OLINMAYDI.",
              basis: "Qonuniy manfaat",
              keep: "Akkaunt o‘chirilgunga qadar",
            },
            {
              what: "Push-xabarnoma tokeni",
              why: "Buyurtma holati haqida xabar berish.",
              basis: "Rozilik",
              keep: "Chiqilgunga yoki ilova o‘chirilgunga qadar",
            },
          ]}
        />

        <h3 className="mt-8 font-semibold">2.3. Mijozlar uchun qo‘shimcha</h3>
        <DataTable
          rows={[
            {
              what: "Yetkazib berish manzili: koordinatalar, ko‘cha, podyezd, qavat, kvartira, domofon kodi, kuryer uchun izoh",
              why: "Buyurtmani yetkazib berish. Koordinatalar xizmat hududini tekshirish va yetkazish narxini hisoblash uchun ham kerak.",
              basis: "Shartnomani bajarish",
              keep: "Foydalanuvchi o‘zgartirgunga yoki o‘chirgunga qadar",
            },
            {
              what: "Buyurtmalar tarixi: tarkib, summa, sana, holat, to‘lov usuli",
              why: "Buyurtmani bajarish, nizolarni ko‘rib chiqish va buxgalteriya hisobi.",
              basis: "Shartnomani bajarish va qonun talabi",
              keep: "Qonunda belgilangan muddat (buxgalteriya hujjati sifatida)",
            },
            {
              what: "To‘lov ma’lumotlari",
              why: "Karta orqali to‘lash. DIQQAT: karta raqami, amal qilish muddati va CVV OnDex serverlariga UMUMAN kelmaydi — ular to‘g‘ridan-to‘g‘ri to‘lov provayderining sahifasiga kiritiladi. Bizda faqat tranzaksiya identifikatori va holati saqlanadi.",
              basis: "Shartnomani bajarish",
              keep: "Qonunda belgilangan muddat",
            },
            {
              what: "AI yordamchisi bilan yozishma va ovozli murojaat",
              why: "Ovqat tanlashda yordam berish. Ovozli rejim yoqilgandagina mikrofon yozuvi uzatiladi.",
              basis: "Rozilik",
              keep: "Suhbat davomida; yozuvlar doimiy saqlanmaydi",
            },
          ]}
        />

        <h3 className="mt-8 font-semibold">2.4. Kuryerlar uchun qo‘shimcha</h3>
        <DataTable
          rows={[
            {
              what: "Jonli geolokatsiya (koordinatalar va vaqt)",
              why: "Buyurtmani kuzatish va eng yaqin kuryerni tanlash. FAQAT ilova ochiq va kuryer «ish rejimida» bo‘lganda yig‘iladi.",
              basis: "Shartnomani bajarish",
              keep: "Oxirgi holat saqlanadi; tarix yuritilmaydi",
            },
            {
              what: "Transport turi, reyting, bajarilgan buyurtmalar soni",
              why: "Buyurtmani taqsimlash va sifat nazorati.",
              basis: "Shartnomani bajarish",
              keep: "Akkaunt o‘chirilgunga qadar",
            },
          ]}
        />

        <Clause n="2.5">
          <b>Yig‘ilMAYDIGAN ma’lumotlar.</b> Operator quyidagilarni
          yig‘maydi: pasport ma’lumotlari, bank kartasi raqamlari,
          qurilmaning noyob identifikatorlari (IMEI, reklama ID),
          telefon kitobi, SMS xabarlar, boshqa ilovalar ro‘yxati,
          brauzer tarixi, biometrik ma’lumotlar.
        </Clause>
      </Section>

      <Section n="3" title="Ma’lumotlar kimga uzatiladi">
        <Clause n="3.1">
          Operator ma’lumotlarni <b>sotmaydi</b> va reklama maqsadlarida
          uchinchi shaxslarga bermaydi. Uzatish faqat xizmatni bajarish
          uchun zarur bo‘lgan hajmda amalga oshiriladi.
        </Clause>
        <Clause n="3.2">
          <b>Xizmat ishtirokchilariga.</b> Restoran — buyurtma tarkibini
          va mijoz ismini; kuryer — yetkazish manzilini va telefon
          raqamini. Kuryer mijozning to‘liq manzilini va telefonini
          buyurtmani <b>olgandan keyingina</b> ko‘radi — undan oldin bu
          ma’lumotlar yashirilgan bo‘ladi.
        </Clause>
        <Clause n="3.3">
          <b>Texnik hamkorlarga</b> — quyidagi ro‘yxatdagi hajmda:
        </Clause>
        <DataTable
          rows={[
            {
              what: "Eskiz.uz (O‘zbekiston)",
              why: "SMS tasdiqlash kodini yetkazish",
              basis: "Telefon raqami va kod matni",
              keep: "Provayder siyosatiga muvofiq",
            },
            {
              what: "Telegram (Telegram FZ-LLC)",
              why: "Bot orqali kirish va xabarnomalar",
              basis: "Telegram ID, xabar matni",
              keep: "Provayder siyosatiga muvofiq",
            },
            {
              what: "Octo (to‘lov provayderi)",
              why: "Karta orqali to‘lovni amalga oshirish",
              basis: "Summa, buyurtma identifikatori, telefon",
              keep: "Provayder siyosatiga muvofiq",
            },
            {
              what: "Google (Maps, Geocoding, Distance Matrix)",
              why: "Manzilni aniqlash, masofa va yetkazish vaqtini hisoblash",
              basis: "Koordinatalar yoki manzil matni",
              keep: "So‘rov davomida",
            },
            {
              what: "Yandex Geocoder, 2GIS",
              why: "Google aniq ko‘chani topa olmaganda manzilni aniqlashtirish",
              basis: "Koordinatalar",
              keep: "So‘rov davomida",
            },
            {
              what: "Google Firebase Cloud Messaging",
              why: "Push-xabarnomalarni yetkazish",
              basis: "Qurilma tokeni, xabar matni",
              keep: "Provayder siyosatiga muvofiq",
            },
            {
              what: "Resend",
              why: "Elektron pochta xabarlarini yuborish",
              basis: "E-pochta manzili, xabar matni",
              keep: "Provayder siyosatiga muvofiq",
            },
            {
              what: "Cloudflare R2",
              why: "Rasm va media fayllarni saqlash",
              basis: "Yuklangan fayllar (taom rasmlari)",
              keep: "Fayl o‘chirilgunga qadar",
            },
            {
              what: "AI xizmatlari (Shaddiy AI, Google Gemini)",
              why: "Matnli va ovozli yordamchi ishlashi",
              basis: "Suhbat matni, ovozli rejimda — audio oqim",
              keep: "Suhbat davomida",
            },
          ]}
        />
        <Clause n="3.4">
          <b>Vakolatli davlat organlariga</b> — qonun hujjatlarida
          belgilangan tartibda va asoslar bo‘lgan taqdirdagina.
        </Clause>
      </Section>

      <Section n="4" title="Ma’lumotlar qanday himoyalanadi">
        <Clause n="4.1">
          Barcha aloqa <b>TLS</b> shifrlangan kanal orqali amalga
          oshiriladi.
        </Clause>
        <Clause n="4.2">
          Parollar qaytarib bo‘lmaydigan <b>Argon2id</b> algoritmi bilan
          xeshlanadi. Bir martalik kodlar server tomonidagi maxfiy kalit
          bilan (HMAC) xeshlanadi va 5 daqiqadan so‘ng avtomatik
          o‘chiriladi.
        </Clause>
        <Clause n="4.3">
          Mobil ilovalarda sessiya kaliti qurilmaning <b>shifrlangan
          omborida</b> (Android Keystore / iOS Keychain) saqlanadi.
        </Clause>
        <Clause n="4.4">
          Kirish urinishlari, tasdiqlash kodlari va boshqa nozik amallar
          chastota bo‘yicha cheklangan. Sessiyani istalgan paytda bekor
          qilish mumkin.
        </Clause>
        <Clause n="4.5">
          Ma’lumotlarga xodimlarning kirishi <b>rol asosida</b>
          cheklangan: restoran faqat o‘z buyurtmalarini, kuryer faqat
          o‘ziga biriktirilgan buyurtmani ko‘radi.
        </Clause>
        <Clause n="4.6">
          Ma’lumotlar O‘zbekiston Respublikasi hududidagi serverlarda
          saqlanadi. Yuqorida sanab o‘tilgan texnik hamkorlarga uzatish
          xizmat ko‘rsatish uchun zarur bo‘lgan hajmda amalga oshiriladi.
        </Clause>
      </Section>

      <Section n="5" title="Foydalanuvchining huquqlari">
        <Clause n="5.1">Foydalanuvchi quyidagi huquqlarga ega:</Clause>
        <ul className="ml-5 list-disc space-y-1 text-neutral-800">
          <li>o‘zi haqidagi ma’lumotlar ro‘yxatini olish;</li>
          <li>noto‘g‘ri ma’lumotni tuzatish yoki to‘ldirish;</li>
          <li>
            akkauntni va u bilan bog‘liq ma’lumotlarni <b>o‘chirish</b>;
          </li>
          <li>
            berilgan rozilikni qaytarib olish (push-xabarnomalar,
            geolokatsiya, AI yordamchi);
          </li>
          <li>ma’lumotlarni ishlashni cheklashni talab qilish;</li>
          <li>vakolatli organga shikoyat qilish.</li>
        </ul>
        <Clause n="5.2">
          <b>Akkauntni o‘chirish.</b> Ilovadagi profil bo‘limi orqali yoki{" "}
          <a href={`mailto:${OPERATOR.privacyEmail}`} className="underline">
            {OPERATOR.privacyEmail}
          </a>{" "}
          manziliga murojaat orqali. Ariza <b>30 kun</b> ichida bajariladi.
        </Clause>
        <Clause n="5.3">
          <b>Nima o‘chmaydi.</b> Yakunlangan buyurtmalar buxgalteriya
          hujjati hisoblanadi va qonunda belgilangan muddat davomida
          saqlanadi. Ularda shaxsni aniqlovchi ma’lumotlar (ism, telefon,
          manzil) <b>egasizlantiriladi</b> — ya’ni yozuv qoladi, lekin u
          endi hech kimga bog‘lanmaydi.
        </Clause>
        <Clause n="5.4">
          Yakunlanmagan buyurtmasi bor akkaunt o‘chirilmaydi — avval
          buyurtma yakunlanishi kerak.
        </Clause>
      </Section>

      <Section n="6" title="Bolalar">
        <Clause n="6.1">
          Xizmat <b>18 yoshga to‘lgan</b> shaxslar uchun mo‘ljallangan.
          Operator bolalar haqidagi ma’lumotlarni ataylab yig‘maydi.
          Bunday holat aniqlansa, ma’lumotlar o‘chiriladi.
        </Clause>
      </Section>

      <Section n="7" title="Cookie va shunga o‘xshash texnologiyalar">
        <Clause n="7.1">
          Sayt faqat <b>zarur</b> cookie fayllardan foydalanadi: sessiyani
          saqlash va xavfsizlik. Reklama yoki kuzatuv (tracking) cookie
          fayllari ishlatilmaydi.
        </Clause>
        <Clause n="7.2">
          Sessiya cookie’si <code>httpOnly</code> belgisi bilan
          o‘rnatiladi — unga sahifadagi skriptlar kira olmaydi.
        </Clause>
      </Section>

      <Section n="8" title="Siyosatga o‘zgartirish kiritish">
        <Clause n="8.1">
          Operator Siyosatga o‘zgartirish kiritish huquqiga ega. Yangi
          tahrir shu sahifada e’lon qilinadi va sarlavhadagi sana
          yangilanadi.
        </Clause>
        <Clause n="8.2">
          Ma’lumotlarni ishlash maqsadlari yoki hajmi <b>sezilarli</b>
          o‘zgarsa, foydalanuvchi ilova orqali alohida xabardor qilinadi.
        </Clause>
        <Clause n="8.3">
          Ushbu tahrir {LEGAL_EFFECTIVE_DATE} sanasidan kuchga kiradi.
        </Clause>
      </Section>

      <Section n="9" title="Aloqa">
        <Clause n="9.1">
          Shaxsga doir ma’lumotlar bo‘yicha savollar:{" "}
          <a href={`mailto:${OPERATOR.privacyEmail}`} className="underline">
            {OPERATOR.privacyEmail}
          </a>
          .
        </Clause>
        <Clause n="9.2">
          Umumiy masalalar:{" "}
          <a href={`mailto:${OPERATOR.email}`} className="underline">
            {OPERATOR.email}
          </a>
          , telefon:{" "}
          <a
            href={`tel:${OPERATOR.phone.replace(/\s/g, "")}`}
            className="underline"
          >
            {OPERATOR.phone}
          </a>
          .
        </Clause>
        <Clause n="9.3">
          Pochta manzili: {req(OPERATOR.address)}.
        </Clause>
      </Section>
    </LegalPage>
  );
}

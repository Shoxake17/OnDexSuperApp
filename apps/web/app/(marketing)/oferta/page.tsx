import type { Metadata } from "next";

import {
  Clause,
  LEGAL_EFFECTIVE_DATE,
  LegalPage,
  OPERATOR,
  Section,
  req,
} from "../legal/legal-ui";

/**
 * Ommaviy oferta — OnDex ekotizimidan foydalanish shartlari.
 *
 * ┌─ NEGA AYNAN «OFERTA» ──────────────────────────────────────────────┐
 * O'zbekiston Fuqarolik kodeksiga ko'ra ommaviy oferta — noaniq
 * doiradagi shaxslarga qaratilgan, barcha muhim shartlarni o'z
 * ichiga olgan taklif. Foydalanuvchi buyurtma bergan paytda u
 * AKSEPT qilingan hisoblanadi va shartnoma tuziladi.
 *
 * Shu sabab hujjatda majburiy uch element bo'lishi SHART:
 *   1. taraflar va ularning rekvizitlari;
 *   2. shartnoma predmeti va narx belgilanish tartibi;
 *   3. aksept momenti — qaysi aniq harakat shartnomani tuzadi.
 *
 * Uchalasi ham quyida aniq belgilangan.
 * └────────────────────────────────────────────────────────────────────┘
 *
 * ┌─ MATN KODGA MOS ───────────────────────────────────────────────────┐
 * Buyurtma oqimi, to'lov usullari, bekor qilish qoidalari va
 * yetkazish shartlari HAQIQIY kod bo'yicha yozilgan:
 *
 *   `internal/orders/statemachine.go`  — holatlar va kim o'zgartira oladi
 *   `internal/orders/service.go`       — narx hisobi, to'lov holati
 *   `internal/payments/`               — karta orqali to'lov, hold/capture
 *   `internal/delivery/`               — xizmat hududi
 *   `internal/promotions/`             — aksiya va chegirmalar
 *
 * Ya'ni oferta va'da qilgan narsa kod bajaradigan narsa bilan bir
 * xil. Bu — nizoli holatda eng muhim jihat.
 * └────────────────────────────────────────────────────────────────────┘
 *
 * ┌─ HUQUQIY KO'RIB CHIQISH ───────────────────────────────────────────┐
 * Bu — texnik jihatdan to'liq va aniq LOYIHA. Yakuniy tahrir yurist
 * ko'rigidan o'tishi kerak, ayniqsa: javobgarlik chegaralari,
 * nizolarni hal qilish tartibi va soliq maqomi.
 * └────────────────────────────────────────────────────────────────────┘
 */

export const metadata: Metadata = {
  title: "Ommaviy oferta — OnDex",
  description:
    "OnDex platformasidan foydalanish shartlari: buyurtma berish, to'lash, " +
    "yetkazib berish, bekor qilish va tomonlarning javobgarligi.",
};

export default function OfferPage() {
  return (
    <LegalPage
      title="Ommaviy oferta"
      subtitle="OnDex platformasi orqali buyurtma berish va xizmat ko‘rsatish shartlari."
    >
      <Section n="1" title="Asosiy tushunchalar">
        <Clause n="1.1">
          <b>Platforma</b> — OnDex dasturiy majmuasi: mobil ilovalar,{" "}
          <a href={OPERATOR.site} className="underline">
            ondex.uz
          </a>{" "}
          sayti, Telegram Mini App va ular bilan bog‘liq xizmatlar.
        </Clause>
        <Clause n="1.2">
          <b>Operator</b> — {req(OPERATOR.legalName)}, STIR{" "}
          {req(OPERATOR.taxId)}, {req(OPERATOR.address)}. Operator
          Platformani boshqaradi.
        </Clause>
        <Clause n="1.3">
          <b>Sotuvchi</b> — Platformada ro‘yxatdan o‘tgan restoran yoki
          boshqa savdo shoxobchasi. Taomni tayyorlaydi va uning sifati
          uchun javob beradi.
        </Clause>
        <Clause n="1.4">
          <b>Foydalanuvchi (Mijoz)</b> — Platforma orqali buyurtma
          beruvchi jismoniy shaxs.
        </Clause>
        <Clause n="1.5">
          <b>Kuryer</b> — buyurtmani yetkazib beruvchi shaxs.
        </Clause>
        <Clause n="1.6">
          <b>Buyurtma</b> — Mijozning Platforma orqali rasmiylashtirgan
          talabi.
        </Clause>
      </Section>

      <Section n="2" title="Shartnoma predmeti va Operatorning roli">
        <Clause n="2.1">
          Operator Mijoz va Sotuvchi o‘rtasida <b>axborot-texnologik
          vositachilik</b> xizmatini ko‘rsatadi: buyurtmani qabul qiladi,
          Sotuvchiga uzatadi, to‘lovni tashkil etadi va yetkazib berishni
          muvofiqlashtiradi.
        </Clause>
        <Clause n="2.2">
          <b>Muhim.</b> Taom Sotuvchi tomonidan tayyorlanadi va sotiladi.
          Operator taomning sifati, tarkibi, tayyorlanish sharoiti va
          oziq-ovqat xavfsizligi uchun{" "}
          <b>bevosita javobgar emas</b> — bu Sotuvchining zimmasida.
          Operator sifatsiz xizmat ko‘rsatgan Sotuvchini Platformadan
          chetlashtirish huquqiga ega.
        </Clause>
        <Clause n="2.3">
          Platformadan foydalanish Mijoz uchun <b>bepul</b>. Yetkazib
          berish xizmati haqi buyurtma summasida alohida ko‘rsatiladi.
        </Clause>
      </Section>

      <Section n="3" title="Aksept — shartnoma qachon tuziladi">
        <Clause n="3.1">
          Ushbu hujjat <b>ommaviy oferta</b> hisoblanadi.
        </Clause>
        <Clause n="3.2">
          <b>Aksept momenti</b> — Mijoz rasmiylashtirish ekranida
          buyurtmani tasdiqlash tugmasini bosgan payt. Shu paytdan
          e’tiboran ushbu oferta shartlari asosida shartnoma tuzilgan
          hisoblanadi.
        </Clause>
        <Clause n="3.3">
          Buyurtmani tasdiqlashdan oldin Mijozga <b>yakuniy summa</b>{" "}
          ko‘rsatiladi: taomlar narxi, chegirmalar, yetkazib berish haqi
          va jami. Ekranda ko‘rsatilgan summa — Mijoz tasdiqlaydigan
          summa.
        </Clause>
        <Clause n="3.4">
          Agar tasdiqlash paytida narx o‘zgargan bo‘lsa (masalan aksiya
          tugagan bo‘lsa), buyurtma <b>rasmiylashtirilmaydi</b> va Mijozga
          yangi summa ko‘rsatiladi. Mijozdan u ko‘rmagan summa hech qachon
          undirilmaydi.
        </Clause>
      </Section>

      <Section n="4" title="Buyurtma berish tartibi">
        <Clause n="4.1">
          Buyurtma berish uchun ro‘yxatdan o‘tish talab qilinadi: telefon
          raqami va uni SMS yoki Telegram orqali tasdiqlash.
        </Clause>
        <Clause n="4.2">
          Bitta buyurtma <b>faqat bitta Sotuvchidan</b> bo‘lishi mumkin.
          Turli restoranlardan olish uchun alohida buyurtma
          rasmiylashtiriladi.
        </Clause>
        <Clause n="4.3">
          Yetkazib berish faqat Platformada belgilangan{" "}
          <b>xizmat hududi</b> doirasida amalga oshiriladi. Manzil hudud
          tashqarisida bo‘lsa, buyurtma qabul qilinmaydi va bu haqda
          Mijozga darhol xabar beriladi.
        </Clause>
        <Clause n="4.4">
          Buyurtma quyidagi holatlardan o‘tadi: qabul qilindi →
          tayyorlanmoqda → tayyor → kuryerda → yetkazildi. Zaldagi
          buyurtma uchun: tayyor → berildi. Holat Mijozga real vaqtda
          ko‘rsatiladi.
        </Clause>
        <Clause n="4.5">
          Sotuvchi buyurtmani <b>rad etish</b> huquqiga ega (masalan taom
          tugagan yoki shoxobcha yopilgan bo‘lsa). Bunda to‘langan summa
          to‘liq qaytariladi.
        </Clause>
      </Section>

      <Section n="5" title="Narx va to‘lov">
        <Clause n="5.1">
          Barcha narxlar <b>O‘zbekiston so‘mida</b> ko‘rsatiladi va soliqlar
          hisobga olingan holda beriladi.
        </Clause>
        <Clause n="5.2">
          To‘lov usullari:
        </Clause>
        <ul className="ml-5 list-disc space-y-1 text-neutral-800">
          <li>
            <b>Naqd pul</b> — yetkazib berishda kuryerga, zalda
            affitsiantga;
          </li>
          <li>
            <b>Bank kartasi</b> — buyurtma berishdan oldin, to‘lov
            provayderining himoyalangan sahifasi orqali.
          </li>
        </ul>
        <Clause n="5.3">
          <b>Karta ma’lumotlari Operatorga uzatilmaydi.</b> Karta raqami,
          amal qilish muddati va CVV kodi to‘g‘ridan-to‘g‘ri to‘lov
          provayderiga kiritiladi. Operator faqat to‘lov holatini oladi.
        </Clause>
        <Clause n="5.4">
          Karta orqali to‘langanda summa avval <b>bloklanadi</b> (hold) va
          Sotuvchi buyurtmani qabul qilgandan keyingina yechiladi.
          Sotuvchi rad etsa yoki buyurtma bajarilmasa — blokdan
          chiqariladi.
        </Clause>
        <Clause n="5.5">
          To‘lanmagan karta buyurtmasi <b>30 daqiqa</b> ichida
          to‘lanmasa avtomatik bekor qilinadi.
        </Clause>
        <Clause n="5.6">
          Chegirma va aksiyalar shartlari Platformada ko‘rsatiladi. Bir
          buyurtmaga eng foydali bitta aksiya qo‘llanadi, agar
          aksiyaning o‘z shartlarida boshqacha ko‘rsatilmagan bo‘lsa.
        </Clause>
      </Section>

      <Section n="6" title="Bekor qilish va pulni qaytarish">
        <Clause n="6.1">
          Mijoz buyurtmani Sotuvchi uni <b>qabul qilgunga qadar</b> bekor
          qilishi mumkin. Bunda to‘langan summa to‘liq qaytariladi.
        </Clause>
        <Clause n="6.2">
          Taom tayyorlana boshlaganidan keyin bekor qilish Sotuvchi bilan
          kelishilgan holda amalga oshiriladi.
        </Clause>
        <Clause n="6.3">
          Quyidagi hollarda summa <b>to‘liq qaytariladi</b>:
        </Clause>
        <ul className="ml-5 list-disc space-y-1 text-neutral-800">
          <li>buyurtma Sotuvchi tomonidan rad etilgan bo‘lsa;</li>
          <li>buyurtma yetkazib berilmagan bo‘lsa;</li>
          <li>
            yetkazilgan taom buyurtmaga mos kelmasa yoki yaroqsiz bo‘lsa
            (fakt tasdiqlangan holda).
          </li>
        </ul>
        <Clause n="6.4">
          Pul <b>o‘sha to‘lov usuli</b> orqali qaytariladi. Bank
          tomonidagi o‘tkazish muddati bankning qoidalari bilan
          belgilanadi (odatda 1–10 ish kuni).
        </Clause>
        <Clause n="6.5">
          Da’vo yetkazib berilgan kundan boshlab <b>24 soat</b> ichida
          bildirilishi kerak — oziq-ovqat mahsulotining tabiati shuni
          talab qiladi.
        </Clause>
      </Section>

      <Section n="7" title="Tomonlarning majburiyatlari">
        <Clause n="7.1">
          <b>Operator majburiyatlari:</b> Platformaning ishlashini
          ta’minlash, buyurtmani Sotuvchiga uzatish, to‘lovni tashkil
          etish, Mijozning ma’lumotlarini Maxfiylik siyosatiga muvofiq
          himoya qilish.
        </Clause>
        <Clause n="7.2">
          <b>Mijoz majburiyatlari:</b> to‘g‘ri manzil va telefon raqamini
          ko‘rsatish, buyurtmani belgilangan vaqtda qabul qilish, naqd
          to‘lovda summani to‘liq berish.
        </Clause>
        <Clause n="7.3">
          <b>Sotuvchi majburiyatlari:</b> taomni buyurtmaga muvofiq va
          sanitariya talablariga rioya qilgan holda tayyorlash, tarkib va
          allergenlar haqidagi ma’lumotni to‘g‘ri ko‘rsatish.
        </Clause>
        <Clause n="7.4">
          <b>Mijozga taqiqlanadi:</b> Platformadan qonunga xilof
          maqsadlarda foydalanish, soxta buyurtmalar berish, boshqa
          shaxsning ma’lumotlaridan foydalanish, Platforma ishiga texnik
          aralashish (avtomatlashtirilgan so‘rovlar, zaifliklardan
          foydalanish).
        </Clause>
        <Clause n="7.5">
          Ushbu bandning talablari buzilganda Operator akkauntni cheklash
          yoki bloklash huquqiga ega.
        </Clause>
      </Section>

      <Section n="8" title="Javobgarlik">
        <Clause n="8.1">
          Operator javobgarligi ko‘rsatilgan xizmat doirasi bilan
          cheklanadi va har qanday holatda <b>tegishli buyurtma
          summasidan</b> oshmaydi.
        </Clause>
        <Clause n="8.2">
          Operator quyidagilar uchun javob bermaydi: Mijoz noto‘g‘ri
          manzil yoki telefon ko‘rsatgani, Mijoz buyurtmani qabul
          qilmagani, uchinchi tomon xizmatlarining (internet, bank,
          xaritalar) uzilishi, yengib bo‘lmaydigan kuch (force majeure)
          holatlari.
        </Clause>
        <Clause n="8.3">
          Yetkazib berish vaqti <b>taxminiy</b> ko‘rsatiladi. Yo‘l harakati,
          ob-havo va Sotuvchining bandligi tufayli u o‘zgarishi mumkin.
        </Clause>
        <Clause n="8.4">
          Taomning sifati va oziq-ovqat xavfsizligi bo‘yicha javobgarlik
          Sotuvchi zimmasida (2.2-band).
        </Clause>
      </Section>

      <Section n="9" title="Nizolarni hal qilish">
        <Clause n="9.1">
          Tomonlar nizoni <b>muzokara yo‘li</b> bilan hal qilishga harakat
          qiladi. Murojaat:{" "}
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
        <Clause n="9.2">
          Da’vo <b>10 ish kuni</b> ichida ko‘rib chiqiladi.
        </Clause>
        <Clause n="9.3">
          Kelishuvga erishilmasa, nizo O‘zbekiston Respublikasi
          qonunchiligiga muvofiq sud tartibida hal qilinadi.
        </Clause>
      </Section>

      <Section n="10" title="Yakuniy qoidalar">
        <Clause n="10.1">
          Operator ofertaga o‘zgartirish kiritish huquqiga ega. Yangi
          tahrir shu sahifada e’lon qilingan paytdan kuchga kiradi.
        </Clause>
        <Clause n="10.2">
          Buyurtmaga <b>u berilgan paytdagi</b> tahrir qo‘llanadi.
        </Clause>
        <Clause n="10.3">
          Shaxsga doir ma’lumotlar{" "}
          <a href="/maxfiylik" className="underline">
            Maxfiylik siyosati
          </a>{" "}
          asosida ishlanadi. U ushbu ofertaning ajralmas qismidir.
        </Clause>
        <Clause n="10.4">
          Ushbu tahrir {LEGAL_EFFECTIVE_DATE} sanasidan kuchga kiradi.
        </Clause>
      </Section>

      <Section n="11" title="Operator rekvizitlari">
        <Clause n="11.1">
          Nomi: {req(OPERATOR.legalName)}
        </Clause>
        <Clause n="11.2">STIR: {req(OPERATOR.taxId)}</Clause>
        <Clause n="11.3">
          Ro‘yxatdan o‘tganlik: {req(OPERATOR.registration)}
        </Clause>
        <Clause n="11.4">
          Yuridik manzil: {req(OPERATOR.address)}
        </Clause>
        <Clause n="11.5">
          Bank rekvizitlari: {req(OPERATOR.bank)}
        </Clause>
        <Clause n="11.6">
          Telefon: {OPERATOR.phone} · E-pochta: {OPERATOR.email}
        </Clause>
      </Section>
    </LegalPage>
  );
}

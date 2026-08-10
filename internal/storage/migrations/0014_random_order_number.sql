-- order_number'ning ketma-ket (order_seq asosidagi, ko'p nolli
-- "0000123" ko'rinishidagi) qismi TASODIFIY 7 xonali songa almashtirildi
-- (foydalanuvchi so'rovi: "0 lar boshlanmasin, hammasi random bo'lsin,
-- faqat boshidagi 6 xonalik sana qoladi"). order_seq ustuni o'zi
-- (ichki, faqat sabab-natija tartibi uchun) ishlatilishda davom etadi,
-- shunchaki ko'rinadigan raqamga endi ta'sir qilmaydi.
ALTER TABLE orders ALTER COLUMN order_number SET DEFAULT (
	to_char(now(), 'DDMMYY') || '-' || (1000000 + floor(random() * 9000000))::bigint::text
);

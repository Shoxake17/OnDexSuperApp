// Katalog (MongoDB) dagi rasm manzillarini bir domendan boshqasiga ko'chiradi.
// Masalan: https://pub-xxx.r2.dev  ->  https://cdn.ondex.uz
//
// ┌─ NEGA KERAK ────────────────────────────────────────────────────────┐
// Rasm domeni (R2_PUBLIC_URL) almashtirilganda YANGI yuklashlar yangi
// domenni oladi, lekin bazadagi eski yozuvlar (restoran logo/muqovasi,
// taom rasmi, kitob muqovasi) eski domenda qoladi. Eski domen
// (`pub-*.r2.dev`) tezlik cheklangan va CDN keshisiz, ya'ni foydasi
// faqat ko'chirilgandan keyin to'liq seziladi.
// └────────────────────────────────────────────────────────────────────┘
//
// ISHLATISH (avval SINOV rejimi, hech narsa yozilmaydi):
//
//   mongosh -u "$MONGO_INITDB_ROOT_USERNAME" -p "$MONGO_INITDB_ROOT_PASSWORD" \
//     --authenticationDatabase admin --quiet \
//     --eval 'var FROM="https://pub-xxx.r2.dev", TO="https://cdn.ondex.uz", DB="chustapp"' \
//     /tmp/rehost-media.js
//
// Natija to'g'ri bo'lsa, `APPLY=true` qo'shib qayta ishga tushiring.
//
// XAVFSIZLIK:
//   * FAQAT satr qiymati `FROM + "/"` bilan BOSHLANSA almashtiriladi (matn
//     ichidagi tasodifiy eslatmalar tegilmaydi);
//   * hujjatning boshqa maydonlari va `_id` o'zgarmaydi;
//   * ishga tushirishdan oldin zaxira oling (`~/ondex/backup.sh`);
//   * Postgres'dagi tarixiy buyurtma nusxalari (`orders.items`) ATAYLAB
//     tegilmaydi: eski domen o'chirilmaguncha ular ishlayveradi.

/* global db, FROM, TO, DB, APPLY, print, printjson */
if (typeof FROM === "undefined" || typeof TO === "undefined" || typeof DB === "undefined") {
  throw new Error("FROM, TO va DB berilishi shart (--eval 'var FROM=..., TO=..., DB=...')");
}
if (FROM.indexOf("https://") !== 0 || TO.indexOf("https://") !== 0) {
  throw new Error("FROM va TO faqat https:// bilan boshlanishi kerak");
}
FROM = FROM.replace(/\/+$/, "");
TO = TO.replace(/\/+$/, "");
var apply = (typeof APPLY !== "undefined" && APPLY === true);

// deepReplace — qiymat ichidagi (massiv/obyekt ham) mos satrlarni almashtiradi.
// `changed` hisoblagichi nechta satr o'zgarganini sanaydi.
function deepReplace(v, counter) {
  if (typeof v === "string") {
    if (v.indexOf(FROM + "/") === 0) {
      counter.n++;
      return TO + v.substring(FROM.length);
    }
    return v;
  }
  if (Array.isArray(v)) {
    return v.map(function (x) { return deepReplace(x, counter); });
  }
  if (v !== null && typeof v === "object" && v.constructor && v.constructor.name === "Object") {
    var out = {};
    Object.keys(v).forEach(function (k) { out[k] = deepReplace(v[k], counter); });
    return out;
  }
  return v; // ObjectId, Date, son va h.k. o'zgarishsiz
}

var d = db.getSiblingDB(DB);
var total = 0, docs = 0;
d.getCollectionNames().forEach(function (name) {
  var coll = d.getCollection(name);
  var collDocs = 0, collStrings = 0;
  coll.find().forEach(function (doc) {
    var c = { n: 0 };
    var next = deepReplace(doc, c);
    if (c.n === 0) return;
    collDocs++; collStrings += c.n;
    if (apply) {
      var res = coll.replaceOne({ _id: doc._id }, next);
      if (res.matchedCount !== 1) { print("  OGOHLANTIRISH: " + name + " " + doc._id + " yozilmadi"); }
    }
  });
  if (collDocs > 0) { print((apply ? "YOZILDI " : "SINOV   ") + name + ": " + collDocs + " hujjat, " + collStrings + " manzil"); }
  docs += collDocs; total += collStrings;
});
print((apply ? "JAMI YOZILDI: " : "JAMI (hech narsa yozilmadi): ") + docs + " hujjat, " + total + " manzil");
if (!apply) { print("Yozish uchun: --eval 'var APPLY=true, FROM=..., TO=..., DB=...'"); }

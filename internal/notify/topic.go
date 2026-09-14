package notify

// Bildirishnoma KALITLARI — yagona manba.
//
// ┌─ MUAMMO (tuzatilgan) ─────────────────────────────────────────────┐
// Avval kalitlar ikki xil tizimda aralash edi:
//
//	hub.Send(o.CustomerID, ...)          // XOM foydalanuvchi ID
//	hub.Send(o.RestaurantID, ...)        // XOM entity ID
//	hub.Send(orderTopic(id), ...)        // "order:<id>" mavzu
//	hub.Send(restaurantTopic(id), ...)   // "restaurant:<id>" mavzu
//
// Xom ID'lar bir xil fazoda yashagani uchun foydalanuvchi ID'si bilan
// entity ID'si NAZARIY jihatdan to'qnashishi mumkin edi — bunda bir
// odam boshqasining xabarini olardi. Modullar ko'paygach (do'kon,
// taxi, klub) har birining entity ID'si qo'shilib, bu xavf real
// bo'lardi.
//
// Endi HAR BIR kalitning prefiksi bor va ular hech qachon
// kesishmaydi. Yangi modul qo'shilganda `Entity(module, id)` ga
// yangi modul nomi beriladi, boshqa hech narsa o'zgarmaydi.
// └───────────────────────────────────────────────────────────────────┘
//
// MUHIM: kalitlar SERVER tomonda yasaladi (`routes_ws.go`), klient
// ularni yubormaydi — shuning uchun sxemani o'zgartirish mijoz
// ilovalarini BUZMAYDI.

// Modul nomlari — `Entity` uchun. Yangi modul shu yerga qo'shiladi.
const (
	ModuleFood     = "food"
	ModuleShop     = "shop"
	ModuleRide     = "ride"
	ModuleClub     = "club"
	ModuleListing  = "listing" // realty + auto + jobs
	ModulePlatform = "platform"
)

// User — shaxsiy kanal (mijoz, kuryer, xodim — kim bo'lishidan
// qat'i nazar, FOYDALANUVCHI sifatida).
func User(userID string) string {
	if userID == "" {
		return ""
	}
	return "u:" + userID
}

// Entity — tashkilot kanali (restoran, do'kon, klub...). Shu
// tashkilotning BARCHA xodimlari eshitadi.
func Entity(module, entityID string) string {
	if module == "" || entityID == "" {
		return ""
	}
	return "e:" + module + ":" + entityID
}

// PublicEntity — tashkilotning OCHIQ kanali: shu tashkilot sahifasini
// ochib turgan HAR QANDAY mijoz obuna bo'la oladi.
//
// ┌─ NEGA `Entity` dan ALOHIDA ────────────────────────────────────────┐
// Avval menyu yangilanishi ham, YANGI BUYURTMA ham bitta
// `Entity(food, restaurantID)` kanaliga borardi va `GET /ws` da
// `?restaurant_id=` parametri hech qanday tekshiruvsiz o'sha kanalga
// obuna qilardi. Restoran ID'lari `GET /restaurants` da ochiq
// berilgani uchun istalgan mijoz raqib restoranning butun buyurtma
// oqimini (summa, stol, kuryer) real vaqtda kuzata olardi. Kuryer
// ID'si ham shu fazoda bo'lgani uchun kuryerning taklif oqimi
// (restoran manzili va KOORDINATASI) ham ochiq edi.
//
// Endi ikki kanal bor:
//
//   - `Entity(...)`       — XODIM kanali: buyurtmalar, taklif, dispatch.
//     Unga faqat o'sha tashkilotning `EntityID` si bilan
//     kelgan token obuna bo'ladi.
//   - `PublicEntity(...)` — OCHIQ kanal: menyu/aksiya/3D yangilandi
//     signali. Bu ma'lumot GET endpointlarida
//     allaqachon ochiq, shuning uchun tekshiruv shart emas.
//
// Ochiq signallar IKKALA kanalga ham yuboriladi — xodim paneli ham
// menyu o'zgarganini eshitishi kerak.
// └────────────────────────────────────────────────────────────────────┘
func PublicEntity(module, entityID string) string {
	if module == "" || entityID == "" {
		return ""
	}
	return "pub:" + module + ":" + entityID
}

// Manager — tashkilot RAHBARIYATI kanali (restoran akkaunti).
//
// `Entity(...)` dan farqi: unga affitsiant (xodim ilovasi) obuna
// BO'LMAYDI. Restoran bildirishnomalari — to'lov summalari, xodimlar
// o'zgarishi, kunlik savdo hisoboti — faqat restoran egasiga tegishli
// (`httpapi.managerTopicIfRestaurant`).
func Manager(module, entityID string) string {
	if module == "" || entityID == "" {
		return ""
	}
	return "m:" + module + ":" + entityID
}

// Admin — platforma ma'muriyati kanali (superadmin paneli).
//
// ┌─ NEGA ALOHIDA KANAL ──────────────────────────────────────────────┐
// Superadmin paneli hech qaysi restoran yoki kuryerga TEGISHLI EMAS,
// ya'ni `Entity(...)` kanallarining birortasiga ham qo'shila olmaydi
// (uning `EntityID` si bo'sh). Shu sabab u jonli hech narsa
// eshitmasdi va butun panel 5-10 soniyalik so'rov sikliga tayanardi:
// yangi buyurtma paydo bo'lgani, holati o'zgargani, kuryer
// ro'yxatdan o'tgani — hammasi kechikib ko'rinardi.
//
// Bu kanal aynan shu bo'shliqni yopadi. Unga FAQAT `admin` roli
// obuna bo'ladi (`routes_ws.go`) — ya'ni bu yerdagi xabarlar
// mijozlarga hech qachon ko'rinmaydi.
// └───────────────────────────────────────────────────────────────────┘
//
// Kalitda ID yo'q: ma'muriyat bitta va uning barcha a'zolari bir xil
// oqimni ko'radi.
func Admin() string { return "a:" + ModulePlatform }

// Order — bitta buyurtma/safar kanali (kuryerning jonli GPS'i shu
// yerga boradi).
func Order(orderID string) string {
	if orderID == "" {
		return ""
	}
	return "o:" + orderID
}

package storage

import (
	"context"
	"errors"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"chustapp/internal/catalog"
)

// maxSearchResults — `SearchProducts` qaytaradigan eng ko'p natija
// (bug.md 12-band).
//
// Chegara ikki ishni bajaradi: javob hajmini va SKANERLASH ishini
// chegaralaydi — kursor o'qish shu songa yetgach to'xtaydi. 100 ta
// natija mijoz ilovasidagi qidiruv ro'yxati uchun ortig'i bilan
// yetadi (ekranda 10-20 tasi ko'rinadi).
const maxSearchResults = 100

type mongoRestaurant struct {
	ID       string  `bson:"_id"`
	Name     string  `bson:"name"`
	Address  string  `bson:"address"`
	Lat      float64 `bson:"lat"`
	Lng      float64 `bson:"lng"`
	Open     bool    `bson:"open"`
	LogoURL  string  `bson:"logo_url"`
	CoverURL string  `bson:"cover_url"`
	Tags     string  `bson:"tags"`
	// Eski hujjatlarda bu maydonlar YO'Q — BSON ularni nol qiymat bilan
	// qoldiradi, ya'ni "ma'lumot yo'q". Migratsiya kerak emas.
	Rating        float64 `bson:"rating"`
	RatingCount   int     `bson:"rating_count"`
	ETAMinMinutes int     `bson:"eta_min_minutes"`
	ETAMaxMinutes int     `bson:"eta_max_minutes"`
	// 3D maket: bo'sh bo'lsa restoranda 3D yo'q. Eski hujjatlarda
	// bu maydonlar yo'q va BSON ularni bo'sh qoldiradi.
	Scene3DURL    string `bson:"scene_3d_url"`
	Scene3DSHA256 string `bson:"scene_3d_sha256"`
	Scene3DBytes  int64  `bson:"scene_3d_bytes"`
}

func (d mongoRestaurant) toDomain() *catalog.Restaurant {
	return &catalog.Restaurant{
		ID: d.ID, Name: d.Name, Address: d.Address, Lat: d.Lat, Lng: d.Lng, Open: d.Open,
		LogoURL: d.LogoURL, CoverURL: d.CoverURL, Tags: d.Tags,
		Rating: d.Rating, RatingCount: d.RatingCount,
		ETAMinMinutes: d.ETAMinMinutes, ETAMaxMinutes: d.ETAMaxMinutes,
		Scene3DURL: d.Scene3DURL, Scene3DSHA256: d.Scene3DSHA256, Scene3DBytes: d.Scene3DBytes,
	}
}

func restaurantDoc(x *catalog.Restaurant) mongoRestaurant {
	return mongoRestaurant{
		ID: x.ID, Name: x.Name, Address: x.Address, Lat: x.Lat, Lng: x.Lng, Open: x.Open,
		LogoURL: x.LogoURL, CoverURL: x.CoverURL, Tags: x.Tags,
		Rating: x.Rating, RatingCount: x.RatingCount,
		ETAMinMinutes: x.ETAMinMinutes, ETAMaxMinutes: x.ETAMaxMinutes,
		Scene3DURL: x.Scene3DURL, Scene3DSHA256: x.Scene3DSHA256, Scene3DBytes: x.Scene3DBytes,
	}
}

type mongoProduct struct {
	ID                  string  `bson:"_id"`
	RestaurantID        string  `bson:"restaurant_id"`
	Name                string  `bson:"name"`
	Category            string  `bson:"category"`
	PriceTiyin          int64   `bson:"price_tiyin"`
	DiscountPriceTiyin  int64   `bson:"discount_price_tiyin"`
	WholesalePriceTiyin int64   `bson:"wholesale_price_tiyin"`
	Stock               int     `bson:"stock"`
	Weight              float64 `bson:"weight"`
	WeightUnit          string  `bson:"weight_unit"`
	Description         string  `bson:"description"`
	PrepTimeText        string  `bson:"prep_time_text"`
	ImageURL            string  `bson:"image_url"`
	Available           bool    `bson:"available"`
	// 3D model (AI generatsiyasi). `omitempty` — eski hujjatlarda bu
	// maydonlar yo'q va bo'lishi ham shart emas.
	Model3DURL    string `bson:"model_3d_url,omitempty"`
	Model3DStatus string `bson:"model_3d_status,omitempty"`
	Model3DTaskID string `bson:"model_3d_task_id,omitempty"`
}

func (d mongoProduct) toDomain() *catalog.Product {
	return &catalog.Product{
		ID: d.ID, RestaurantID: d.RestaurantID, Name: d.Name, Category: d.Category,
		PriceTiyin: d.PriceTiyin, DiscountPriceTiyin: d.DiscountPriceTiyin,
		WholesalePriceTiyin: d.WholesalePriceTiyin,
		Stock:               d.Stock, Weight: d.Weight, WeightUnit: d.WeightUnit,
		Description: d.Description, PrepTimeText: d.PrepTimeText, ImageURL: d.ImageURL, Available: d.Available,
		Model3DURL: d.Model3DURL, Model3DStatus: d.Model3DStatus, Model3DTaskID: d.Model3DTaskID,
	}
}

func productDoc(x *catalog.Product) mongoProduct {
	return mongoProduct{
		ID: x.ID, RestaurantID: x.RestaurantID, Name: x.Name, Category: x.Category,
		PriceTiyin: x.PriceTiyin, DiscountPriceTiyin: x.DiscountPriceTiyin,
		WholesalePriceTiyin: x.WholesalePriceTiyin,
		Stock:               x.Stock, Weight: x.Weight, WeightUnit: x.WeightUnit,
		Description: x.Description, PrepTimeText: x.PrepTimeText, ImageURL: x.ImageURL, Available: x.Available,
		Model3DURL: x.Model3DURL, Model3DStatus: x.Model3DStatus, Model3DTaskID: x.Model3DTaskID,
	}
}

// MongoCatalogRepo — katalog (restoranlar+menyu) uchun MongoDB implementatsiyasi.
// Buyurtmalar/foydalanuvchilar/kuryerlar bilan hech qanday aloqasi yo'q —
// ular PostgreSQL'da qoladi (polyglot persistence: har ma'lumot o'ziga
// mos bazada).
type MongoCatalogRepo struct {
	restaurants *mongo.Collection
	products    *mongo.Collection
}

func NewMongoCatalogRepo(db *mongo.Database) *MongoCatalogRepo {
	return &MongoCatalogRepo{
		restaurants: db.Collection("restaurants"),
		products:    db.Collection("products"),
	}
}

// EnsureMongoIndexes — restoran bo'yicha menyu qidiruvi tez ishlashi uchun.
func EnsureMongoIndexes(ctx context.Context, db *mongo.Database) error {
	_, err := db.Collection("products").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "restaurant_id", Value: 1}},
	})
	return err
}

func (r *MongoCatalogRepo) ListRestaurants(ctx context.Context) ([]*catalog.Restaurant, error) {
	cur, err := r.restaurants.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var list []*catalog.Restaurant
	for cur.Next(ctx) {
		var doc mongoRestaurant
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		list = append(list, doc.toDomain())
	}
	return list, cur.Err()
}

func (r *MongoCatalogRepo) GetRestaurant(ctx context.Context, id string) (*catalog.Restaurant, error) {
	var doc mongoRestaurant
	err := r.restaurants.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, catalog.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return doc.toDomain(), nil
}

func (r *MongoCatalogRepo) SaveRestaurant(ctx context.Context, x *catalog.Restaurant) error {
	doc := restaurantDoc(x)
	_, err := r.restaurants.ReplaceOne(ctx, bson.M{"_id": x.ID}, doc, options.Replace().SetUpsert(true))
	return err
}

// DeleteRestaurant — avval menyu, keyin restoran. Standalone MongoDB'da
// (replica set'siz) ko'p-hujjatli tranzaksiya yo'q, shuning uchun tartib
// muhim: taomlar birinchi o'chsa, restoran o'chirish muvaffaqiyatsiz
// bo'lganda ham yetim buyurtma-yo'naltiruvchi holat qolmaydi.
func (r *MongoCatalogRepo) DeleteRestaurant(ctx context.Context, id string) error {
	var doc mongoRestaurant
	if err := r.restaurants.FindOne(ctx, bson.M{"_id": id}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return catalog.ErrNotFound
		}
		return err
	}
	if _, err := r.products.DeleteMany(ctx, bson.M{"restaurant_id": id}); err != nil {
		return err
	}
	_, err := r.restaurants.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *MongoCatalogRepo) ListProducts(ctx context.Context, restaurantID string) ([]*catalog.Product, error) {
	cur, err := r.products.Find(ctx,
		bson.M{"restaurant_id": restaurantID}, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	return scanMongoProducts(ctx, cur)
}

func (r *MongoCatalogRepo) GetProductsByIDs(ctx context.Context, ids []string) ([]*catalog.Product, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	cur, err := r.products.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	return scanMongoProducts(ctx, cur)
}

// SearchProducts — barcha restoranlar bo'yicha nomi yoki turkumi so'rovga
// mos mavjud taomlarni qaytaradi. Moslik catalog.NormalizeForSearch orqali
// tekshiriladi (bo'shliq/registrga sezgir emas — masalan restoran
// turkumidagi "Fastfood" bilan taom turkumidagi "Fast Food" bir xil deb
// topiladi, "Lavash" so'rovi esa "Lavash mini" nomli taomni ham topadi).
// $lookup orqali har bir taomga tegishli restoran nomi/logotipi/holati
// qo'shib beriladi. Restoranlar soni kichik shahar miqyosida bo'lgani
// uchun (yuzlab, minglab emas) filtrlashni ilovada Go kodida bajarish
// oddiy va yetarlicha tez — Mongo regex bilan normalizatsiyani takrorlash
// o'rniga.
// SearchProducts — nom/turkum bo'yicha qidiruv.
//
// ┌─ TUZATILGAN NOSOZLIK (bug.md 12-band) ─────────────────────────────┐
// Avval bu yerda quyidagi quvur ishlardi:
//
//	$match{available} → $lookup(restaurants) → $unwind → $sort
//
// ya'ni BUTUN `products` kolleksiyasiga restoran hujjati qo'shilib,
// keyin hammasi Go tomonga dekodlanardi — filtrlash esa faqat
// SHUNDAN KEYIN bo'lardi. Natija soni ham cheklanmagandi.
//
// Endpoint OCHIQ (`s.auth` yo'q) va hech qanday chelakka tushmasdi,
// ya'ni bir qatorlik `curl` sikli bazani band qila olardi.
//
// Ikki o'zgarish (banddagi (a) va (b) tavsiyalari):
//
//  1. `$lookup` FILTRDAN KEYIN. Endi avval faqat mahsulot hujjatlari
//     o'qiladi (kerakli maydonlar bilan), Go tomonda filtrlanadi, va
//     restoran ma'lumoti FAQAT mos kelganlar uchun BITTA qo'shimcha
//     so'rov bilan olinadi. Restoranlar kam, shuning uchun bu so'rov
//     arzon — `$lookup` esa har bir taom uchun ishlardi.
//
//  2. Natijaga CHEGARA (`maxSearchResults`). Chegaraga yetgach
//     kursor o'qish TO'XTAYDI, ya'ni katalog kattalashganda ham ish
//     hajmi chegaralangan qoladi.
//
// Semantika o'zgarmadi: filtr aynan o'sha `NormalizeForSearch`
// solishtiruvi (tinish belgilarini e'tiborsiz qoldiradi). Mongo
// tomonida regex bilan oldindan filtrlash ATAYLAB qilinmadi — u
// normalizatsiyani takrorlay olmaydi va "coca-cola" so'rovi
// "Coca Cola" ni topmay qolardi.
// └────────────────────────────────────────────────────────────────────┘
func (r *MongoCatalogRepo) SearchProducts(ctx context.Context, query string) ([]*catalog.ProductSearchResult, error) {
	nq := catalog.NormalizeForSearch(query)
	if nq == "" {
		return nil, nil
	}

	// 1-qadam: faqat mahsulotlar. `$lookup`/`$unwind` YO'Q.
	cur, err := r.products.Find(ctx,
		bson.M{"available": true},
		options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var (
		list    []*catalog.ProductSearchResult
		restIDs []string
		seen    = map[string]bool{}
	)
	for cur.Next(ctx) {
		if len(list) >= maxSearchResults {
			break
		}
		// `wholesale_price_tiyin` bu yerda ATAYLAB YO'Q: qidiruv — OCHIQ
		// endpoint, ulgurji narx esa restoranning ichki ma'lumoti.
		// O'qilmagan maydon hech qachon sizib chiqa olmaydi.
		var doc struct {
			ID                 string  `bson:"_id"`
			RestaurantID       string  `bson:"restaurant_id"`
			Name               string  `bson:"name"`
			Category           string  `bson:"category"`
			PriceTiyin         int64   `bson:"price_tiyin"`
			DiscountPriceTiyin int64   `bson:"discount_price_tiyin"`
			Stock              int     `bson:"stock"`
			Weight             float64 `bson:"weight"`
			WeightUnit         string  `bson:"weight_unit"`
			Description        string  `bson:"description"`
			PrepTimeText       string  `bson:"prep_time_text"`
			ImageURL           string  `bson:"image_url"`
			Available          bool    `bson:"available"`
		}
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		if !strings.Contains(catalog.NormalizeForSearch(doc.Name), nq) &&
			!strings.Contains(catalog.NormalizeForSearch(doc.Category), nq) {
			continue
		}
		if doc.RestaurantID != "" && !seen[doc.RestaurantID] {
			seen[doc.RestaurantID] = true
			restIDs = append(restIDs, doc.RestaurantID)
		}
		list = append(list, &catalog.ProductSearchResult{
			Product: catalog.Product{
				ID: doc.ID, RestaurantID: doc.RestaurantID, Name: doc.Name, Category: doc.Category,
				PriceTiyin: doc.PriceTiyin, DiscountPriceTiyin: doc.DiscountPriceTiyin, Stock: doc.Stock, Weight: doc.Weight, WeightUnit: doc.WeightUnit,
				Description: doc.Description, PrepTimeText: doc.PrepTimeText, ImageURL: doc.ImageURL, Available: doc.Available,
			},
		})
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}

	// 2-qadam: restoran ma'lumoti — FAQAT mos kelganlar uchun, BITTA
	// so'rov bilan.
	rests, err := r.restaurantsByIDs(ctx, restIDs)
	if err != nil {
		return nil, err
	}
	for _, it := range list {
		if rest, ok := rests[it.RestaurantID]; ok {
			it.RestaurantName = rest.Name
			it.RestaurantLogoURL = rest.LogoURL
			it.RestaurantOpen = rest.Open
		}
	}
	return list, nil
}

// restaurantsByIDs — bir nechta restoranni BITTA so'rov bilan oladi.
func (r *MongoCatalogRepo) restaurantsByIDs(ctx context.Context, ids []string) (map[string]mongoRestaurant, error) {
	out := make(map[string]mongoRestaurant, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	cur, err := r.restaurants.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var doc mongoRestaurant
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		out[doc.ID] = doc
	}
	return out, cur.Err()
}

func scanMongoProducts(ctx context.Context, cur *mongo.Cursor) ([]*catalog.Product, error) {
	var list []*catalog.Product
	for cur.Next(ctx) {
		var doc mongoProduct
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		list = append(list, doc.toDomain())
	}
	return list, cur.Err()
}

func (r *MongoCatalogRepo) SaveProduct(ctx context.Context, x *catalog.Product) error {
	doc := productDoc(x)
	_, err := r.products.ReplaceOne(ctx, bson.M{"_id": x.ID}, doc, options.Replace().SetUpsert(true))
	return err
}

func (r *MongoCatalogRepo) DeleteProduct(ctx context.Context, id string) error {
	res, err := r.products.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return catalog.ErrNotFound
	}
	return nil
}

// SeedDemoCatalogMongo — dev/demo uchun boshlang'ich restoran va menyu.
// $setOnInsert bilan: bor bo'lsa tegilmaydi (Postgres'dagi
// "ON CONFLICT DO NOTHING" bilan bir xil xulq-atvor).
func SeedDemoCatalogMongo(ctx context.Context, db *mongo.Database) error {
	restaurants := db.Collection("restaurants")
	for _, x := range DemoRestaurants() {
		doc := restaurantDoc(&x)
		if _, err := restaurants.UpdateOne(ctx,
			bson.M{"_id": x.ID}, bson.M{"$setOnInsert": doc}, options.Update().SetUpsert(true)); err != nil {
			return err
		}
	}
	products := db.Collection("products")
	for _, p := range DemoProducts() {
		doc := productDoc(&p)
		if _, err := products.UpdateOne(ctx,
			bson.M{"_id": p.ID}, bson.M{"$setOnInsert": doc}, options.Update().SetUpsert(true)); err != nil {
			return err
		}
	}
	return nil
}

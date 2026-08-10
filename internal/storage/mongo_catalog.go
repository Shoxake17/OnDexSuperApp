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
}

func (d mongoRestaurant) toDomain() *catalog.Restaurant {
	return &catalog.Restaurant{
		ID: d.ID, Name: d.Name, Address: d.Address, Lat: d.Lat, Lng: d.Lng, Open: d.Open,
		LogoURL: d.LogoURL, CoverURL: d.CoverURL, Tags: d.Tags,
	}
}

func restaurantDoc(x *catalog.Restaurant) mongoRestaurant {
	return mongoRestaurant{
		ID: x.ID, Name: x.Name, Address: x.Address, Lat: x.Lat, Lng: x.Lng, Open: x.Open,
		LogoURL: x.LogoURL, CoverURL: x.CoverURL, Tags: x.Tags,
	}
}

type mongoProduct struct {
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

func (d mongoProduct) toDomain() *catalog.Product {
	return &catalog.Product{
		ID: d.ID, RestaurantID: d.RestaurantID, Name: d.Name, Category: d.Category,
		PriceTiyin: d.PriceTiyin, DiscountPriceTiyin: d.DiscountPriceTiyin, Stock: d.Stock, Weight: d.Weight, WeightUnit: d.WeightUnit,
		Description: d.Description, PrepTimeText: d.PrepTimeText, ImageURL: d.ImageURL, Available: d.Available,
	}
}

func productDoc(x *catalog.Product) mongoProduct {
	return mongoProduct{
		ID: x.ID, RestaurantID: x.RestaurantID, Name: x.Name, Category: x.Category,
		PriceTiyin: x.PriceTiyin, DiscountPriceTiyin: x.DiscountPriceTiyin, Stock: x.Stock, Weight: x.Weight, WeightUnit: x.WeightUnit,
		Description: x.Description, PrepTimeText: x.PrepTimeText, ImageURL: x.ImageURL, Available: x.Available,
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
func (r *MongoCatalogRepo) SearchProducts(ctx context.Context, query string) ([]*catalog.ProductSearchResult, error) {
	nq := catalog.NormalizeForSearch(query)
	if nq == "" {
		return nil, nil
	}
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"available": true}}},
		{{Key: "$lookup", Value: bson.M{
			"from":         "restaurants",
			"localField":   "restaurant_id",
			"foreignField": "_id",
			"as":           "restaurant",
		}}},
		{{Key: "$unwind", Value: "$restaurant"}},
		{{Key: "$sort", Value: bson.D{{Key: "name", Value: 1}}}},
	}
	cur, err := r.products.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var list []*catalog.ProductSearchResult
	for cur.Next(ctx) {
		var doc struct {
			ID                 string          `bson:"_id"`
			RestaurantID       string          `bson:"restaurant_id"`
			Name               string          `bson:"name"`
			Category           string          `bson:"category"`
			PriceTiyin         int64           `bson:"price_tiyin"`
			DiscountPriceTiyin int64           `bson:"discount_price_tiyin"`
			Stock              int             `bson:"stock"`
			Weight             float64         `bson:"weight"`
			WeightUnit         string          `bson:"weight_unit"`
			Description        string          `bson:"description"`
			PrepTimeText       string          `bson:"prep_time_text"`
			ImageURL           string          `bson:"image_url"`
			Available          bool            `bson:"available"`
			Restaurant         mongoRestaurant `bson:"restaurant"`
		}
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		if !strings.Contains(catalog.NormalizeForSearch(doc.Name), nq) &&
			!strings.Contains(catalog.NormalizeForSearch(doc.Category), nq) {
			continue
		}
		list = append(list, &catalog.ProductSearchResult{
			Product: catalog.Product{
				ID: doc.ID, RestaurantID: doc.RestaurantID, Name: doc.Name, Category: doc.Category,
				PriceTiyin: doc.PriceTiyin, DiscountPriceTiyin: doc.DiscountPriceTiyin, Stock: doc.Stock, Weight: doc.Weight, WeightUnit: doc.WeightUnit,
				Description: doc.Description, PrepTimeText: doc.PrepTimeText, ImageURL: doc.ImageURL, Available: doc.Available,
			},
			RestaurantName:    doc.Restaurant.Name,
			RestaurantLogoURL: doc.Restaurant.LogoURL,
			RestaurantOpen:    doc.Restaurant.Open,
		})
	}
	return list, cur.Err()
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

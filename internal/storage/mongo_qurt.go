package storage

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"chustapp/internal/qurt"
)

// mongoQurtProduct — `qurt_products` hujjati. `StoreName` SAQLANMAYDI.
type mongoQurtProduct struct {
	ID             string    `bson:"_id"`
	StoreID        string    `bson:"store_id"`
	ExternalID     string    `bson:"external_id"`
	Name           string    `bson:"name"`
	Description    string    `bson:"description"`
	Composition    string    `bson:"composition"`
	Category       string    `bson:"category"`
	PriceTiyin     int64     `bson:"price_tiyin"`
	WholesaleTiyin int64     `bson:"wholesale_tiyin"`
	WeightGrams    int       `bson:"weight_grams"`
	ImageURL       string    `bson:"image_url"`
	Available      bool      `bson:"available"`
	UpdatedAt      time.Time `bson:"updated_at"`
}

func (d mongoQurtProduct) toDomain() *qurt.Product {
	return &qurt.Product{
		ID: d.ID, StoreID: d.StoreID, ExternalID: d.ExternalID,
		Name: d.Name, Description: d.Description, Composition: d.Composition,
		Category: d.Category, PriceTiyin: d.PriceTiyin, WholesaleTiyin: d.WholesaleTiyin,
		WeightGrams: d.WeightGrams, ImageURL: d.ImageURL, Available: d.Available,
		UpdatedAt: d.UpdatedAt,
	}
}

// MongoQurtRepo — qurt mahsulotlari (Mongo, `qurt_products`).
type MongoQurtRepo struct {
	col   *mongo.Collection
	newID func() string
}

// NewMongoQurtRepo — `newID` yangi mahsulot ID'si uchun (httpapi.NewID).
func NewMongoQurtRepo(db *mongo.Database, newID func() string) *MongoQurtRepo {
	return &MongoQurtRepo{col: db.Collection("qurt_products"), newID: newID}
}

// EnsureQurtIndexes — (do'kon, POS kaliti) YAGONA (qayta yuborish takrorlamasin)
// va ro'yxat so'rovi uchun (do'kon, kategoriya).
func EnsureQurtIndexes(ctx context.Context, db *mongo.Database) error {
	_, err := db.Collection("qurt_products").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "store_id", Value: 1}, {Key: "external_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{{Key: "store_id", Value: 1}, {Key: "available", Value: 1}, {Key: "category", Value: 1}}},
	})
	return err
}

func (r *MongoQurtRepo) Upsert(ctx context.Context, p *qurt.Product) (*qurt.Product, error) {
	now := time.Now().UTC()
	set := bson.M{
		"name": p.Name, "description": p.Description, "composition": p.Composition,
		"category": p.Category, "price_tiyin": p.PriceTiyin, "wholesale_tiyin": p.WholesaleTiyin,
		"weight_grams": p.WeightGrams, "image_url": p.ImageURL, "available": p.Available,
		"updated_at": now,
	}
	var doc mongoQurtProduct
	err := r.col.FindOneAndUpdate(ctx,
		bson.M{"store_id": p.StoreID, "external_id": p.ExternalID},
		bson.M{"$set": set, "$setOnInsert": bson.M{"_id": r.newID()}},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
	).Decode(&doc)
	if err != nil {
		return nil, err
	}
	return doc.toDomain(), nil
}

func (r *MongoQurtRepo) Delete(ctx context.Context, storeID, externalID string) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"store_id": storeID, "external_id": externalID})
	return err
}

func (r *MongoQurtRepo) DeleteByStore(ctx context.Context, storeID string) error {
	_, err := r.col.DeleteMany(ctx, bson.M{"store_id": storeID})
	return err
}

func (r *MongoQurtRepo) List(ctx context.Context, f qurt.Filter) ([]*qurt.Product, error) {
	if len(f.StoreIDs) == 0 {
		return nil, nil
	}
	q := bson.M{"store_id": bson.M{"$in": f.StoreIDs}, "available": true}
	if f.Category != "" {
		q["category"] = f.Category
	}
	if s := strings.TrimSpace(f.Query); s != "" {
		// Foydalanuvchi matni REGEXP sifatida talqin qilinmasin (`QuoteMeta`).
		rx := bson.M{"$regex": regexp.QuoteMeta(s), "$options": "i"}
		q["$or"] = bson.A{bson.M{"name": rx}, bson.M{"composition": rx}}
	}
	opts := options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}, {Key: "_id", Value: 1}})
	if f.Limit > 0 {
		opts.SetLimit(int64(f.Limit))
	}
	if f.Offset > 0 {
		opts.SetSkip(int64(f.Offset))
	}
	cur, err := r.col.Find(ctx, q, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []*qurt.Product
	for cur.Next(ctx) {
		var d mongoQurtProduct
		if err := cur.Decode(&d); err != nil {
			return nil, err
		}
		out = append(out, d.toDomain())
	}
	return out, cur.Err()
}

func (r *MongoQurtRepo) Categories(ctx context.Context, storeIDs []string) ([]string, error) {
	if len(storeIDs) == 0 {
		return nil, nil
	}
	vals, err := r.col.Distinct(ctx, "category",
		bson.M{"store_id": bson.M{"$in": storeIDs}, "available": true})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out, nil
}

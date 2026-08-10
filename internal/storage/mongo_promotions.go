package storage

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"chustapp/internal/promotions"
)

type mongoPromotion struct {
	ID                     string    `bson:"_id"`
	RestaurantID           string    `bson:"restaurant_id"`
	Name                   string    `bson:"name"`
	Description            string    `bson:"description"`
	Type                   string    `bson:"type"`
	DiscountUnit           string    `bson:"discount_unit"`
	DiscountValue          int64     `bson:"discount_value"`
	MinOrderAmountTiyin    int64     `bson:"min_order_amount_tiyin"`
	MaxDiscountAmountTiyin int64     `bson:"max_discount_amount_tiyin"`
	StartAt                time.Time `bson:"start_at"`
	EndAt                  time.Time `bson:"end_at"`
	Indefinite             bool      `bson:"indefinite"`
	Active                 bool      `bson:"active"`
	AppliesToProducts      bool      `bson:"applies_to_products"`
	AppliesToOrders        bool      `bson:"applies_to_orders"`
	AppliesToCategories    bool      `bson:"applies_to_categories"`
	TargetProductIDs       []string  `bson:"target_product_ids"`
	TargetCategories       []string  `bson:"target_categories"`
	MinPreviousOrders      int64     `bson:"min_previous_orders"`
	UsageCount             int64     `bson:"usage_count"`
	SalesTotalTiyin        int64     `bson:"sales_total_tiyin"`
	ImageURL               string    `bson:"image_url"`
	CreatedAt              time.Time `bson:"created_at"`
}

func (d mongoPromotion) toDomain() *promotions.Promotion {
	return &promotions.Promotion{
		ID: d.ID, RestaurantID: d.RestaurantID, Name: d.Name, Description: d.Description,
		Type:                   promotions.Type(d.Type),
		DiscountUnit:           promotions.DiscountUnit(d.DiscountUnit),
		DiscountValue:          d.DiscountValue,
		MinOrderAmountTiyin:    d.MinOrderAmountTiyin,
		MaxDiscountAmountTiyin: d.MaxDiscountAmountTiyin,
		StartAt:                d.StartAt,
		EndAt:                  d.EndAt,
		Indefinite:             d.Indefinite,
		Active:                 d.Active,
		AppliesToProducts:      d.AppliesToProducts,
		AppliesToOrders:        d.AppliesToOrders,
		AppliesToCategories:    d.AppliesToCategories,
		TargetProductIDs:       d.TargetProductIDs,
		TargetCategories:       d.TargetCategories,
		MinPreviousOrders:      d.MinPreviousOrders,
		UsageCount:             d.UsageCount,
		SalesTotalTiyin:        d.SalesTotalTiyin,
		ImageURL:               d.ImageURL,
		CreatedAt:              d.CreatedAt,
	}
}

func promotionDoc(x *promotions.Promotion) mongoPromotion {
	return mongoPromotion{
		ID: x.ID, RestaurantID: x.RestaurantID, Name: x.Name, Description: x.Description,
		Type:                   string(x.Type),
		DiscountUnit:           string(x.DiscountUnit),
		DiscountValue:          x.DiscountValue,
		MinOrderAmountTiyin:    x.MinOrderAmountTiyin,
		MaxDiscountAmountTiyin: x.MaxDiscountAmountTiyin,
		StartAt:                x.StartAt,
		EndAt:                  x.EndAt,
		Indefinite:             x.Indefinite,
		Active:                 x.Active,
		AppliesToProducts:      x.AppliesToProducts,
		AppliesToOrders:        x.AppliesToOrders,
		AppliesToCategories:    x.AppliesToCategories,
		TargetProductIDs:       x.TargetProductIDs,
		TargetCategories:       x.TargetCategories,
		MinPreviousOrders:      x.MinPreviousOrders,
		UsageCount:             x.UsageCount,
		SalesTotalTiyin:        x.SalesTotalTiyin,
		ImageURL:               x.ImageURL,
		CreatedAt:              x.CreatedAt,
	}
}

type MongoPromotionsRepo struct {
	col *mongo.Collection
}

func NewMongoPromotionsRepo(db *mongo.Database) *MongoPromotionsRepo {
	return &MongoPromotionsRepo{col: db.Collection("promotions")}
}

// EnsureMongoPromotionsIndexes — restoran bo'yicha ro'yxat tez ishlashi uchun.
func EnsureMongoPromotionsIndexes(ctx context.Context, db *mongo.Database) error {
	_, err := db.Collection("promotions").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "restaurant_id", Value: 1}},
	})
	return err
}

func (r *MongoPromotionsRepo) ListByRestaurant(ctx context.Context, restaurantID string) ([]*promotions.Promotion, error) {
	cur, err := r.col.Find(ctx,
		bson.M{"restaurant_id": restaurantID}, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var list []*promotions.Promotion
	for cur.Next(ctx) {
		var doc mongoPromotion
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		list = append(list, doc.toDomain())
	}
	return list, cur.Err()
}

func (r *MongoPromotionsRepo) GetByID(ctx context.Context, id string) (*promotions.Promotion, error) {
	var doc mongoPromotion
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, promotions.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return doc.toDomain(), nil
}

func (r *MongoPromotionsRepo) Save(ctx context.Context, x *promotions.Promotion) error {
	doc := promotionDoc(x)
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": x.ID}, doc, options.Replace().SetUpsert(true))
	return err
}

func (r *MongoPromotionsRepo) Delete(ctx context.Context, id string) error {
	res, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return promotions.ErrNotFound
	}
	return nil
}

// DeleteByRestaurant — qarang: promotions.Repository izohi.
func (r *MongoPromotionsRepo) DeleteByRestaurant(ctx context.Context, restaurantID string) (int, error) {
	res, err := r.col.DeleteMany(ctx, bson.M{"restaurant_id": restaurantID})
	if err != nil {
		return 0, err
	}
	return int(res.DeletedCount), nil
}

func (r *MongoPromotionsRepo) IncrementUsage(ctx context.Context, id string, amountTiyin int64) error {
	res, err := r.col.UpdateOne(ctx, bson.M{"_id": id},
		bson.M{"$inc": bson.M{"usage_count": 1, "sales_total_tiyin": amountTiyin}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return promotions.ErrNotFound
	}
	return nil
}

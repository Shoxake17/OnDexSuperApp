package storage

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"chustapp/internal/qurtorders"
)

type mongoQurtOrderItem struct {
	ProductID   string `bson:"product_id"`
	Name        string `bson:"name"`
	Qty         int    `bson:"qty"`
	PriceTiyin  int64  `bson:"price_tiyin"`
	ImageURL    string `bson:"image_url"`
	WeightGrams int    `bson:"weight_grams"`
	ExternalID  string `bson:"external_id"`
}

type mongoQurtOrderChange struct {
	From   string    `bson:"from"`
	To     string    `bson:"to"`
	By     string    `bson:"by"`
	ByID   string    `bson:"by_id"`
	Reason string    `bson:"reason"`
	At     time.Time `bson:"at"`
}

// mongoQurtOrder — `qurt_orders` hujjati.
type mongoQurtOrder struct {
	ID               string                 `bson:"_id"`
	Number           string                 `bson:"number"`
	GroupID          string                 `bson:"group_id"`
	CustomerID       string                 `bson:"customer_id"`
	StoreID          string                 `bson:"store_id"`
	StoreName        string                 `bson:"store_name"`
	Items            []mongoQurtOrderItem   `bson:"items"`
	SubtotalTiyin    int64                  `bson:"subtotal_tiyin"`
	DeliveryFeeTiyin int64                  `bson:"delivery_fee_tiyin"`
	TotalTiyin       int64                  `bson:"total_tiyin"`
	PaymentMethod    string                 `bson:"payment_method"`
	Status           string                 `bson:"status"`
	DeliveryDate     string                 `bson:"delivery_date"`
	CustomerName     string                 `bson:"customer_name"`
	CustomerPhone    string                 `bson:"customer_phone"`
	Address          qurtorders.Address     `bson:"address"`
	Comment          string                 `bson:"comment"`
	RejectReason     string                 `bson:"reject_reason"`
	History          []mongoQurtOrderChange `bson:"history"`
	Reserved         bool                   `bson:"reserved"`
	IdempotencyKey   string                 `bson:"idempotency_key"`
	CreatedAt        time.Time              `bson:"created_at"`
	UpdatedAt        time.Time              `bson:"updated_at"`
	Version          int                    `bson:"version"`

	CourierID         string     `bson:"courier_id,omitempty"`
	CourierAssignedAt *time.Time `bson:"courier_assigned_at,omitempty"`
	CashTiyin         int64      `bson:"cash_tiyin,omitempty"`
	CashStatus        string     `bson:"cash_status,omitempty"`
	CashSettledAt     *time.Time `bson:"cash_settled_at,omitempty"`
	CashSettledBy     string     `bson:"cash_settled_by,omitempty"`
}

func toMongoQurtOrder(o *qurtorders.Order) mongoQurtOrder {
	d := mongoQurtOrder{
		ID: o.ID, Number: o.Number, GroupID: o.GroupID, CustomerID: o.CustomerID,
		StoreID: o.StoreID, StoreName: o.StoreName, SubtotalTiyin: o.SubtotalTiyin,
		DeliveryFeeTiyin: o.DeliveryFeeTiyin, TotalTiyin: o.TotalTiyin,
		PaymentMethod: o.PaymentMethod, Status: string(o.Status), DeliveryDate: o.DeliveryDate,
		CustomerName: o.CustomerName, CustomerPhone: o.CustomerPhone, Address: o.Address,
		Comment: o.Comment, RejectReason: o.RejectReason, Reserved: o.Reserved,
		IdempotencyKey: o.IdempotencyKey, CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt, Version: o.Version,
		CourierID: o.CourierID, CourierAssignedAt: o.CourierAssignedAt, CashTiyin: o.CashTiyin,
		CashStatus: string(o.CashStatus), CashSettledAt: o.CashSettledAt, CashSettledBy: o.CashSettledBy,
	}
	for _, it := range o.Items {
		d.Items = append(d.Items, mongoQurtOrderItem(it))
	}
	for _, h := range o.History {
		d.History = append(d.History, mongoQurtOrderChange{
			From: string(h.From), To: string(h.To), By: string(h.By), ByID: h.ByID, Reason: h.Reason, At: h.At,
		})
	}
	return d
}

func (d mongoQurtOrder) toDomain() *qurtorders.Order {
	o := &qurtorders.Order{
		ID: d.ID, Number: d.Number, GroupID: d.GroupID, CustomerID: d.CustomerID,
		StoreID: d.StoreID, StoreName: d.StoreName, SubtotalTiyin: d.SubtotalTiyin,
		DeliveryFeeTiyin: d.DeliveryFeeTiyin, TotalTiyin: d.TotalTiyin,
		PaymentMethod: d.PaymentMethod, Status: qurtorders.Status(d.Status), DeliveryDate: d.DeliveryDate,
		CustomerName: d.CustomerName, CustomerPhone: d.CustomerPhone, Address: d.Address,
		Comment: d.Comment, RejectReason: d.RejectReason, Reserved: d.Reserved,
		IdempotencyKey: d.IdempotencyKey, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, Version: d.Version,
		CourierID: d.CourierID, CourierAssignedAt: d.CourierAssignedAt, CashTiyin: d.CashTiyin,
		CashStatus: qurtorders.CashStatus(d.CashStatus), CashSettledAt: d.CashSettledAt, CashSettledBy: d.CashSettledBy,
	}
	for _, it := range d.Items {
		o.Items = append(o.Items, qurtorders.Item(it))
	}
	for _, h := range d.History {
		o.History = append(o.History, qurtorders.StatusChange{
			From: qurtorders.Status(h.From), To: qurtorders.Status(h.To), By: qurtorders.Actor(h.By),
			ByID: h.ByID, Reason: h.Reason, At: h.At,
		})
	}
	return o
}

// MongoQurtOrderRepo — qurt buyurtmalari (Mongo, `qurt_orders`).
type MongoQurtOrderRepo struct{ col *mongo.Collection }

func NewMongoQurtOrderRepo(db *mongo.Database) *MongoQurtOrderRepo {
	return &MongoQurtOrderRepo{col: db.Collection("qurt_orders")}
}

// EnsureQurtOrderIndexes — do'kon/mijoz/reys bo'yicha ro'yxatlar va takroriy so'rov himoyasi.
func EnsureQurtOrderIndexes(ctx context.Context, db *mongo.Database) error {
	_, err := db.Collection("qurt_orders").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "store_id", Value: 1}, {Key: "status", Value: 1}, {Key: "created_at", Value: -1}}},
		{Keys: bson.D{{Key: "customer_id", Value: 1}, {Key: "created_at", Value: -1}}},
		{Keys: bson.D{{Key: "delivery_date", Value: 1}, {Key: "status", Value: 1}}},
		{Keys: bson.D{{Key: "customer_id", Value: 1}, {Key: "idempotency_key", Value: 1}}},
		{Keys: bson.D{{Key: "courier_id", Value: 1}, {Key: "cash_status", Value: 1}, {Key: "status", Value: 1}}},
	})
	return err
}

func (r *MongoQurtOrderRepo) CreateMany(ctx context.Context, list []*qurtorders.Order) error {
	if len(list) == 0 {
		return nil
	}
	docs := make([]any, len(list))
	ids := make([]string, len(list))
	for i, o := range list {
		docs[i] = toMongoQurtOrder(o)
		ids[i] = o.ID
	}
	if _, err := r.col.InsertMany(ctx, docs); err != nil {
		// Yarim yozilganini qaytarib olamiz (atomiklik bo'lmasa ham "yarim buyurtma" qolmasin).
		_, _ = r.col.DeleteMany(context.WithoutCancel(ctx), bson.M{"_id": bson.M{"$in": ids}})
		return err
	}
	return nil
}

func (r *MongoQurtOrderRepo) Get(ctx context.Context, id string) (*qurtorders.Order, error) {
	var d mongoQurtOrder
	if err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, qurtorders.ErrNotFound
		}
		return nil, err
	}
	return d.toDomain(), nil
}

func (r *MongoQurtOrderRepo) List(ctx context.Context, f qurtorders.Filter) ([]*qurtorders.Order, error) {
	q := bson.M{}
	if f.StoreID != "" {
		q["store_id"] = f.StoreID
	}
	if f.CustomerID != "" {
		q["customer_id"] = f.CustomerID
	}
	if f.DeliveryDate != "" {
		q["delivery_date"] = f.DeliveryDate
	}
	if f.CourierID != "" {
		q["courier_id"] = f.CourierID
	}
	if f.CashStatus != "" {
		q["cash_status"] = string(f.CashStatus)
	}
	if !f.CreatedBefore.IsZero() {
		q["created_at"] = bson.M{"$lt": f.CreatedBefore}
	}
	if len(f.Statuses) > 0 {
		st := make([]string, len(f.Statuses))
		for i, s := range f.Statuses {
			st[i] = string(s)
		}
		q["status"] = bson.M{"$in": st}
	}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}})
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
	var out []*qurtorders.Order
	for cur.Next(ctx) {
		var d mongoQurtOrder
		if err := cur.Decode(&d); err != nil {
			return nil, err
		}
		out = append(out, d.toDomain())
	}
	return out, cur.Err()
}

func (r *MongoQurtOrderRepo) ByIdempotency(ctx context.Context, customerID, key string) ([]*qurtorders.Order, error) {
	cur, err := r.col.Find(ctx, bson.M{"customer_id": customerID, "idempotency_key": key},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}, {Key: "_id", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []*qurtorders.Order
	for cur.Next(ctx) {
		var d mongoQurtOrder
		if err := cur.Decode(&d); err != nil {
			return nil, err
		}
		out = append(out, d.toDomain())
	}
	return out, cur.Err()
}

func (r *MongoQurtOrderRepo) Update(ctx context.Context, o *qurtorders.Order) error {
	d := toMongoQurtOrder(o)
	res, err := r.col.ReplaceOne(ctx, bson.M{"_id": o.ID, "version": o.Version - 1}, d)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		if _, gerr := r.Get(ctx, o.ID); errors.Is(gerr, qurtorders.ErrNotFound) {
			return qurtorders.ErrNotFound
		}
		return qurtorders.ErrConflict
	}
	return nil
}

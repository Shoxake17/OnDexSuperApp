package main

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// Prod'da uchragan holat: shopCategories ichida son; narx matn; sana matn.
func TestLenientDecodeDirtyShoxPosDocs(t *testing.T) {
	raw, _ := bson.Marshal(bson.M{
		"login": "baraka", "ondexEntityId": 12345, "shopCategories": bson.A{"Sut", int64(7), nil, ""},
		"isDeleted": "false",
	})
	var u mUser
	if err := decodeLenient(raw, &u); err != nil {
		t.Fatal(err)
	}
	if u.EntityID != "12345" || len(u.ShopCategories) != 4 || u.ShopCategories[1] != "7" || u.IsDeleted {
		t.Fatalf("user: %+v", u)
	}
	raw, _ = bson.Marshal(bson.M{"login": "baraka", "barcode": int32(4780001), "price": "12 500", "quantity": true,
		"addedAt": "2026-11-01T10:00:00Z", "weightGrams": "250"})
	var p mProduct
	if err := decodeLenient(raw, &p); err != nil {
		t.Fatal(err)
	}
	if p.Barcode != "4780001" || p.Price != 12500 || p.Quantity != 1 || p.WeightGrams != 250 ||
		!p.AddedAt.Equal(time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("product: %+v", p)
	}
	raw, _ = bson.Marshal(bson.M{"login": "a", "expiryDate": nil, "isPrescriptionRequired": 1})
	var ph mPharm
	if err := decodeLenient(raw, &ph); err != nil {
		t.Fatal(err)
	}
	if ph.ExpiryDate != nil || !ph.IsPrescriptionRequired {
		t.Fatalf("pharm: %+v", ph)
	}
}

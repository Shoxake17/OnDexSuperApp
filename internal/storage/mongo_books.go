package storage

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"chustapp/internal/catalog"
)

// Kafe kutubxonasi — 3D maketdagi javondan olib o'qiladigan kitoblar.
//
// ┌─ NEGA ALOHIDA FAYL ────────────────────────────────────────────────┐
// Kitob katalogning bir qismi (u restoranga tegishli), lekin mahsulot
// EMAS: narxi yo'q, savatga tushmaydi, buyurtmaga kirmaydi.
//
// `mongo_catalog.go` menyu mantig'i bilan to'lgan: narx, chegirma,
// ombor, qidiruv. Kitoblarni o'sha yerga qo'shish ikkalasini
// chalkashtirardi va "kitobga chegirma" kabi ma'nosiz holatlar
// paydo bo'lishiga yo'l ochardi.
// └────────────────────────────────────────────────────────────────────┘

// books — kolleksiyani bir joyda nomlash uchun.
func (r *MongoCatalogRepo) booksColl() *mongo.Collection {
	return r.restaurants.Database().Collection("books")
}

// mongoBook — bazadagi ko'rinish.
//
// Maydonlar ochiq nomlanadi (`bson` teglari bilan): tuzilma o'zgarsa
// ham bazadagi yozuvlar o'qilaveradi.
type mongoBook struct {
	ID           string   `bson:"_id"`
	RestaurantID string   `bson:"restaurant_id"`
	Title        string   `bson:"title"`
	Author       string   `bson:"author"`
	CoverURL     string   `bson:"cover_url"`
	PDFURL       string   `bson:"pdf_url"`
	Text         string   `bson:"text"`
	Pages        []string `bson:"pages"`
	Active       bool     `bson:"active"`
	CreatedAt    int64    `bson:"created_at"`
}

func toMongoBook(b *catalog.Book) mongoBook {
	return mongoBook{
		ID:           b.ID,
		RestaurantID: b.RestaurantID,
		Title:        b.Title,
		Author:       b.Author,
		CoverURL:     b.CoverURL,
		PDFURL:       b.PDFURL,
		Text:         b.Text,
		Pages:        b.Pages,
		Active:       b.Active,
		CreatedAt:    b.CreatedAt.Unix(),
	}
}

func (m mongoBook) toCatalog() *catalog.Book {
	return &catalog.Book{
		ID:           m.ID,
		RestaurantID: m.RestaurantID,
		Title:        m.Title,
		Author:       m.Author,
		CoverURL:     m.CoverURL,
		PDFURL:       m.PDFURL,
		Text:         m.Text,
		Pages:        m.Pages,
		Active:       m.Active,
		CreatedAt:    time.Unix(m.CreatedAt, 0),
	}
}

// ListBooks — restoranning kitoblari.
//
// `withText` ataylab: ro'yxatda matn KERAK EMAS va u javobni
// o'nlab barobar shishirardi (bitta kitob 400 KB gacha). Matn faqat
// bitta kitob ochilganda olinadi.
func (r *MongoCatalogRepo) ListBooks(
	ctx context.Context, restaurantID string, withText bool,
) ([]*catalog.Book, error) {
	opts := options.Find().SetSort(bson.D{{Key: "title", Value: 1}})
	if !withText {
		// ┌─ FAQAT MATN TASHLANADI, SAHIFALAR EMAS ────────────────────┐
		// Avval `pages` ham tashlanardi va bu boshqaruv ro'yxatida
		// yomon oqibat berardi: skanerlangan kitobning sahifalari
		// TAYYORLANGANMI degan savolga javob yo'q edi. Admin panel
		// "sahifasiz" deb ko'rsatib, tayyor kitobga ham ogohlantirish
		// chiqarardi.
		//
		// Matn 400 KB gacha bo'ladi va uni tashlash ma'noli. Sahifa
		// manzillari esa ~100 bayt: 142 sahifali kitob 14 KB, ya'ni
		// ro'yxat uchun sezilarli emas.
		// └────────────────────────────────────────────────────────────┘
		opts.SetProjection(bson.M{"text": 0})
	}
	cur, err := r.booksColl().Find(ctx,
		bson.M{"restaurant_id": restaurantID}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	out := []*catalog.Book{}
	for cur.Next(ctx) {
		var m mongoBook
		if err := cur.Decode(&m); err != nil {
			return nil, err
		}
		out = append(out, m.toCatalog())
	}
	return out, cur.Err()
}

// GetBook — bitta kitob, matni bilan.
func (r *MongoCatalogRepo) GetBook(
	ctx context.Context, id string,
) (*catalog.Book, error) {
	var m mongoBook
	err := r.booksColl().FindOne(ctx, bson.M{"_id": id}).Decode(&m)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, catalog.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return m.toCatalog(), nil
}

func (r *MongoCatalogRepo) SaveBook(ctx context.Context, b *catalog.Book) error {
	doc := toMongoBook(b)
	_, err := r.booksColl().ReplaceOne(ctx,
		bson.M{"_id": b.ID}, doc, options.Replace().SetUpsert(true))
	return err
}

func (r *MongoCatalogRepo) DeleteBook(ctx context.Context, id string) error {
	_, err := r.booksColl().DeleteOne(ctx, bson.M{"_id": id})
	return err
}

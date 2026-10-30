package banners

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	long := func(n int) string { return strings.Repeat("a", n) }
	cases := []struct {
		name    string
		in      Banner
		wantErr bool
		want    string // kutilgan Placement
	}{
		{"bo'sh joylashuv = restoran (eski bannerlar)", Banner{ImageURL: "x"}, false, PlacementRestaurant},
		{"menu, havolasiz", Banner{Placement: "menu"}, false, PlacementMenu},
		{"do'kon + qurt amali + tugma", Banner{Placement: "store", Action: "qurt", ButtonText: "Buyurtma berish"}, false, PlacementStore},
		{"menu + qurt amali", Banner{Placement: "menu", Action: "qurt", Title: "Qurt"}, false, PlacementMenu},
		{"restoran + restoranga havola", Banner{Placement: "restaurant", RestaurantID: "r1"}, false, PlacementRestaurant},
		{"noma'lum joylashuv", Banner{Placement: "footer"}, true, ""},
		{"noma'lum amal", Banner{Placement: "menu", Action: "delete"}, true, ""},
		{"tugma havolasiz — ishlamaydigan tugma", Banner{Placement: "menu", ButtonText: "Bos"}, true, ""},
		{"restoran va amal bir vaqtda", Banner{Placement: "restaurant", RestaurantID: "r1", Action: "qurt"}, true, ""},
		{"restoranga havola menu'da", Banner{Placement: "menu", RestaurantID: "r1"}, true, ""},
		{"sarlavha chegarada", Banner{Placement: "menu", Title: long(MaxTitleLen)}, false, PlacementMenu},
		{"sarlavha juda uzun", Banner{Placement: "menu", Title: long(MaxTitleLen + 1)}, true, ""},
		{"tavsif juda uzun", Banner{Placement: "menu", Subtitle: long(MaxSubtitleLen + 1)}, true, ""},
		{"tugma matni juda uzun", Banner{Placement: "menu", Action: "qurt", ButtonText: long(MaxButtonTextLen + 1)}, true, ""},
		{"probel bilan o'ralgan qiymatlar tozalanadi", Banner{Placement: " menu ", Title: "  Qurt  "}, false, PlacementMenu},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := c.in
			err := b.Normalize()
			if (err != nil) != c.wantErr {
				t.Fatalf("xato=%v, kutilgan xato=%v", err, c.wantErr)
			}
			if c.wantErr {
				if !errors.Is(err, ErrInvalid) {
					t.Fatalf("ErrInvalid kutilgan edi: %v", err)
				}
				return
			}
			if b.Placement != c.want {
				t.Fatalf("joylashuv %q, kutilgan %q", b.Placement, c.want)
			}
			if c.name == "probel bilan o'ralgan qiymatlar tozalanadi" && b.Title != "Qurt" {
				t.Fatalf("sarlavha tozalanmadi: %q", b.Title)
			}
		})
	}
}

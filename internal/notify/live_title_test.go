package notify

import (
	"testing"

	"chustapp/internal/orders"
)

// Affitsiant "Yetkazdim" bosganda mijoz telefonida aniq "Buyurtmangiz
// keldi" chiqishi kerak; boshqa holatlarda sarlavha o'zgarmaydi.
func TestOrderStatusTitle(t *testing.T) {
	served := &orders.Order{Type: orders.TypeDineIn, Status: orders.StatusServed}
	if got := orderStatusTitle(served); got != "Buyurtmangiz keldi" {
		t.Fatalf("stolga yetkazilgan buyurtma sarlavhasi: %q", got)
	}
	if got := orderStatusText(served); got == "" {
		t.Fatal("matn bo'sh")
	}
	for _, o := range []*orders.Order{
		{Type: orders.TypeDineIn, Status: orders.StatusReady},
		{Type: orders.TypeDelivery, Status: orders.StatusDelivered},
	} {
		if got := orderStatusTitle(o); got != "Buyurtma holati" {
			t.Fatalf("%s/%s sarlavhasi %q", o.Type, o.Status, got)
		}
	}
}

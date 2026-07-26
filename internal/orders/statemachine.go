package orders

import "fmt"

// transitions — ruxsat etilgan o'tishlar jadvali: from -> to -> kimlar qila oladi.
// Bu jadvalda yo'q o'tish umuman mumkin emas. Yangi holat qo'shilsa, faqat shu yerga qo'shiladi.
var transitions = map[Status]map[Status][]Actor{
	StatusCreated: {
		StatusAccepted:  {ActorRestaurant},
		StatusRejected:  {ActorRestaurant},
		StatusCancelled: {ActorCustomer, ActorAdmin},
	},
	StatusAccepted: {
		StatusPreparing: {ActorRestaurant},
		StatusCancelled: {ActorCustomer, ActorAdmin},
	},
	StatusPreparing: {
		StatusReady:     {ActorRestaurant},
		StatusCancelled: {ActorAdmin},
	},
	StatusReady: {
		StatusPickedUp:  {ActorCourier},
		StatusCancelled: {ActorAdmin},
	},
	StatusPickedUp: {
		StatusDelivered: {ActorCourier},
		StatusCancelled: {ActorAdmin},
	},
	// delivered, rejected, cancelled — terminal, hech qayerga o'tmaydi
}

type TransitionError struct {
	From, To Status
	By       Actor
	Reason   string
}

func (e *TransitionError) Error() string {
	return fmt.Sprintf("transition %s -> %s by %s not allowed: %s", e.From, e.To, e.By, e.Reason)
}

// ValidateTransition — o'tish mumkinligini va aktor huquqini tekshiradi.
func ValidateTransition(from, to Status, by Actor) error {
	targets, ok := transitions[from]
	if !ok {
		return &TransitionError{from, to, by, "holat terminal"}
	}
	actors, ok := targets[to]
	if !ok {
		return &TransitionError{from, to, by, "bunday o'tish mavjud emas"}
	}
	for _, a := range actors {
		if a == by {
			return nil
		}
	}
	return &TransitionError{from, to, by, "bu aktor uchun ruxsat yo'q"}
}

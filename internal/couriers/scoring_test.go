package couriers

import (
	"testing"
	"time"
)

func TestScoreCandidatesPrefersIdealWindow(t *testing.T) {
	prepTime := 20 * time.Minute // ideal oyna: 5-10 daqiqa
	candidates := []ScoredCandidate{
		{Courier: &Courier{ID: "too_close", Rating: 5.0}, ETA: 1 * time.Minute},
		{Courier: &Courier{ID: "ideal", Rating: 5.0}, ETA: 7 * time.Minute},
		{Courier: &Courier{ID: "too_far", Rating: 5.0}, ETA: 25 * time.Minute},
	}
	ranked := ScoreCandidates(candidates, prepTime)
	if ranked[0].Courier.ID != "ideal" {
		t.Fatalf("eng yaxshi ball 'ideal' kuryerga berilishi kerak edi, olindi: %s (score=%.1f)",
			ranked[0].Courier.ID, ranked[0].Score)
	}
	if ranked[0].Score != maxTimingScore+20 { // rating 5.0 -> +20 bonus
		t.Errorf("ideal oyna uchun to'liq ball kutilgan edi, olindi %.1f", ranked[0].Score)
	}
}

func TestScoreCandidatesLatePenalizedMoreThanEarly(t *testing.T) {
	prepTime := 20 * time.Minute                                       // ideal: 5-10 daqiqa
	early := timingScore(2*time.Minute, 5*time.Minute, 10*time.Minute) // 3 daqiqa erta
	late := timingScore(13*time.Minute, 5*time.Minute, 10*time.Minute) // 3 daqiqa kech
	if late >= early {
		t.Errorf("kech qolish erta kelishdan KO'PROQ jazolanishi kerak: erta=%.1f, kech=%.1f", early, late)
	}
	_ = prepTime
}

func TestScoreCandidatesRatingIsTieBreaker(t *testing.T) {
	prepTime := 20 * time.Minute
	sameETA := 7 * time.Minute // ikkalasi ham ideal oynada
	candidates := []ScoredCandidate{
		{Courier: &Courier{ID: "low_rating", Rating: 3.0}, ETA: sameETA},
		{Courier: &Courier{ID: "high_rating", Rating: 5.0}, ETA: sameETA},
	}
	ranked := ScoreCandidates(candidates, prepTime)
	if ranked[0].Courier.ID != "high_rating" {
		t.Fatalf("bir xil ETA'da yuqori reytingli kuryer ustuvor bo'lishi kerak edi, olindi: %s", ranked[0].Courier.ID)
	}
}

func TestScoreCandidatesExperienceIsTieBreaker(t *testing.T) {
	prepTime := 20 * time.Minute
	sameETA := 7 * time.Minute
	candidates := []ScoredCandidate{
		{Courier: &Courier{ID: "new", Rating: 5.0, CompletedOrders: 0}, ETA: sameETA},
		{Courier: &Courier{ID: "veteran", Rating: 5.0, CompletedOrders: 100}, ETA: sameETA},
	}
	ranked := ScoreCandidates(candidates, prepTime)
	if ranked[0].Courier.ID != "veteran" {
		t.Fatalf("bir xil ETA/reytingda tajribali kuryer ustuvor bo'lishi kerak edi, olindi: %s", ranked[0].Courier.ID)
	}
}

func TestScoreCandidatesNoPrepTimePrefersClosest(t *testing.T) {
	candidates := []ScoredCandidate{
		{Courier: &Courier{ID: "far", Rating: 5.0}, ETA: 20 * time.Minute},
		{Courier: &Courier{ID: "near", Rating: 5.0}, ETA: 2 * time.Minute},
	}
	ranked := ScoreCandidates(candidates, 0)
	if ranked[0].Courier.ID != "near" {
		t.Fatalf("prep_time berilmaganda eng yaqin kuryer ustuvor bo'lishi kerak edi, olindi: %s", ranked[0].Courier.ID)
	}
}

func TestExperienceBonusCapsAt50Orders(t *testing.T) {
	at50 := experienceBonus(50)
	at200 := experienceBonus(200)
	if at50 != experienceWeight {
		t.Errorf("50 buyurtmada to'liq bonus (%v) kutilgan edi, olindi %.1f", experienceWeight, at50)
	}
	if at200 != at50 {
		t.Errorf("50 dan yuqorida bonus o'sishi kerak emas: 50->%.1f, 200->%.1f", at50, at200)
	}
}

package e2e

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// INVARIANTLAR — har doim, har holatda rost bo'lishi SHART bo'lgan qoidalar.
//
// ┌─ NEGA INVARIANT ──────────────────────────────────────────────────┐
// Ssenariy faqat yozilgan narsani tekshiradi. Invariant esa HAR
// javobga o'zi qo'llanadi: yangi funksiya qo'shilib, avvalgi qoida
// buzilsa (masalan yetkazish haqi jamiga qo'shilmay qolsa) hech kim
// alohida test yozmagan bo'lsa ham ushlanadi.
//
// Ikki tur:
//   - umumiy (`checkGeneric`): har HTTP javobga — 5xx yo'q, sir sizmaydi,
//     stack trace chiqmaydi, xato JSON'da `error` bor, tezlik chegarasi;
//   - domen (`CheckOrder`, `CheckQuote`): pul va holat qoidalari.
// └───────────────────────────────────────────────────────────────────┘

// Invariant nomlari — hisobotda ko'rinadi.
const (
	InvNo5xx               = "http.no_5xx"
	InvNoSecretLeak        = "http.no_secret_leak"
	InvNoStackTrace        = "http.no_stack_trace"
	InvErrorShape          = "http.error_has_message"
	InvLatencyHard         = "http.latency_hard_limit"
	InvNoPublicCacheAuthed = "http.no_public_cache_for_authed"
	InvQuoteTotal          = "money.quote_total_equals_parts"
	InvQuoteDineInFree     = "money.dine_in_has_no_delivery_fee"
	InvQuoteLines          = "money.quote_lines_plus_delivery_equal_total"
	InvQuoteSane           = "money.discount_within_subtotal_and_total_positive"
	InvOrderTotal          = "money.order_total_equals_parts"
	InvOrderItemsSum       = "money.order_subtotal_equals_items"
	InvOrderNonNegative    = "money.order_amounts_non_negative"
	InvOrderQuoteMatch     = "money.order_total_equals_quote"
	InvOrderHistory        = "state.history_is_a_valid_chain"
	InvOrderTerminalTyp    = "state.terminal_matches_order_type"
	InvOrderCourier        = "state.courier_only_on_delivery"
	InvPayAmount           = "money.payment_equals_order_total"
	InvFiscalSum           = "money.fiscal_lines_equal_payment"
	InvTenantIsolation     = "security.no_cross_tenant_access"
	InvGatedContract       = "http.unconfigured_service_fails_closed"
)

// forbiddenInBody — javobda HECH QACHON bo'lmasligi kerak bo'lgan satrlar.
var forbiddenInBody = []string{
	"secret_enc", "multicard_secret", "password_hash",
	"jwt_secret", "bot_token", "BEGIN PRIVATE KEY",
	// Fikstura sirlari: ular javobda ko'rinsa, shifrlash ma'nosiz.
	GatewaySecret,
}

var publicRestaurantPath = regexp.MustCompile(`^/restaurants(/[^/]+(/menu)?)?$`)

// hardLatencyMs — bundan sekin javob invariantni BUZADI (xotira repolari
// bilan oddiy so'rov shuncha ketmasligi kerak). Sekin-lekin-o'tgan
// so'rovlar esa faqat hisobotda ko'rinadi (`slowBudgetMs`).
const hardLatencyMs = 3000

func (w *World) checkGeneric(t *testing.T, method, path string, r *Resp) {
	t.Helper()

	w.R().Checked(InvNo5xx)
	// 503 — "xizmat sozlanmagan" kontrakti (`Gated`): server yiqilmagan,
	// u ataylab yopiq turibdi. Qolgan 5xx har doim xato.
	if r.Status >= 500 && !(w.gated && r.Status == http.StatusServiceUnavailable) {
		w.Violate(t, InvNo5xx, "%s %s -> %d: %s", method, path, r.Status, truncate(string(r.Body), 200))
	}

	w.R().Checked(InvNoSecretLeak)
	lower := strings.ToLower(string(r.Body))
	for _, bad := range forbiddenInBody {
		if strings.Contains(lower, strings.ToLower(bad)) {
			w.Violate(t, InvNoSecretLeak, "%s %s javobida %q bor", method, path, bad)
		}
	}

	// Ochiq (tokensiz o'qiladigan) restoran javoblarida to'lov hisobi
	// ma'lumoti bo'lmasligi kerak: u faqat egasining sozlamalarida.
	if method == "GET" && publicRestaurantPath.MatchString(strings.SplitN(path, "?", 2)[0]) {
		w.R().Checked(InvNoSecretLeak)
		for _, bad := range []string{"multicard_app_id", "application_id", "store_id", "multicard_"} {
			if strings.Contains(lower, bad) {
				w.Violate(t, InvNoSecretLeak, "ochiq %s javobida to'lov hisobi maydoni %q", path, bad)
			}
		}
	}

	w.R().Checked(InvNoStackTrace)
	if strings.Contains(string(r.Body), "goroutine ") || strings.Contains(string(r.Body), "runtime error") {
		w.Violate(t, InvNoStackTrace, "%s %s javobida stack trace/runtime xatosi", method, path)
	}

	if r.Status >= 400 && len(r.Body) > 0 && r.Body[0] == '{' {
		w.R().Checked(InvErrorShape)
		var m map[string]any
		if json.Unmarshal(r.Body, &m) == nil {
			if msg, _ := m["error"].(string); strings.TrimSpace(msg) == "" {
				w.Violate(t, InvErrorShape, "%s %s -> %d: javobda `error` matni yo'q: %s",
					method, path, r.Status, truncate(string(r.Body), 160))
			}
		}
	}

	w.R().Checked(InvLatencyHard)
	if r.Ms > hardLatencyMs {
		w.Violate(t, InvLatencyHard, "%s %s %.0f ms (chegara %d ms)", method, path, r.Ms, hardLatencyMs)
	}
}

// ─── Domen invariantlari ───────────────────────────────────────────────

func num(m map[string]any, key string) int64 {
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case json.Number:
		n, _ := v.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	}
	return 0
}

// CheckQuote — narxlash javobi: jami = taomlar − chegirma + yetkazish;
// stolda yetkazish haqi yo'q.
func (w *World) CheckQuote(t *testing.T, q map[string]any, dineIn bool) {
	t.Helper()
	sub, disc, fee, total := num(q, "subtotal_tiyin"), num(q, "discount_tiyin"), num(q, "delivery_fee_tiyin"), num(q, "total_tiyin")

	w.R().Checked(InvQuoteTotal)
	if total != sub-disc+fee {
		w.Violate(t, InvQuoteTotal, "jami %d != taomlar %d − chegirma %d + yetkazish %d", total, sub, disc, fee)
	}
	// Qatorlar summasi + yetkazish = jami (orders.QuoteResult izohidagi invariant).
	if lines, ok := q["lines"].([]any); ok && len(lines) > 0 {
		w.R().Checked(InvQuoteLines)
		var sum int64
		for _, l := range lines {
			m, _ := l.(map[string]any)
			sum += num(m, "total_tiyin")
		}
		if sum+fee != total {
			w.Violate(t, InvQuoteLines, "qatorlar yig'indisi %d + yetkazish %d != jami %d", sum, fee, total)
		}
	}
	w.R().Checked(InvQuoteSane)
	if disc < 0 || disc > sub || total <= 0 {
		w.Violate(t, InvQuoteSane, "chegirma %d, taomlar %d, jami %d", disc, sub, total)
	}

	if dineIn {
		w.R().Checked(InvQuoteDineInFree)
		if fee != 0 {
			w.Violate(t, InvQuoteDineInFree, "stol buyurtmasida yetkazish haqi %d", fee)
		}
	}
}

var validTransitions = map[string][]string{
	"":          {"created"},
	"created":   {"accepted", "rejected", "cancelled"},
	"accepted":  {"preparing", "cancelled"},
	"preparing": {"ready", "cancelled"},
	"ready":     {"picked_up", "served", "cancelled"},
	"picked_up": {"delivered", "cancelled"},
}

// CheckOrder — buyurtma obyekti: pul va holat qoidalari.
func (w *World) CheckOrder(t *testing.T, o map[string]any) {
	t.Helper()
	typ, _ := o["type"].(string)
	if typ == "" {
		typ = "delivery"
	}
	status, _ := o["status"].(string)
	sub, disc, fee, total := num(o, "subtotal_tiyin"), num(o, "discount_tiyin"), num(o, "delivery_fee_tiyin"), num(o, "total_tiyin")

	w.R().Checked(InvOrderTotal)
	if total != sub-disc+fee {
		w.Violate(t, InvOrderTotal, "jami %d != taomlar %d − chegirma %d + yetkazish %d", total, sub, disc, fee)
	}

	w.R().Checked(InvOrderNonNegative)
	if sub < 0 || disc < 0 || fee < 0 || total <= 0 || disc > sub {
		w.Violate(t, InvOrderNonNegative, "summalar noto'g'ri: taomlar %d chegirma %d yetkazish %d jami %d", sub, disc, fee, total)
	}

	w.R().Checked(InvOrderItemsSum)
	if items, ok := o["items"].([]any); ok {
		var sum int64
		for _, it := range items {
			m, _ := it.(map[string]any)
			sum += num(m, "price_tiyin") * num(m, "qty")
		}
		if sum != sub {
			w.Violate(t, InvOrderItemsSum, "qatorlar yig'indisi %d != subtotal %d", sum, sub)
		}
	}

	if typ == "dine_in" {
		w.R().Checked(InvQuoteDineInFree)
		if fee != 0 {
			w.Violate(t, InvQuoteDineInFree, "stol buyurtmasida yetkazish haqi %d", fee)
		}
	}

	w.R().Checked(InvOrderCourier)
	if cid, _ := o["courier_id"].(string); cid != "" && typ == "dine_in" {
		w.Violate(t, InvOrderCourier, "stol buyurtmasiga kuryer biriktirilgan (%s)", cid)
	}

	w.R().Checked(InvOrderTerminalTyp)
	if status == "delivered" && typ != "delivery" {
		w.Violate(t, InvOrderTerminalTyp, "%s buyurtma `delivered` holatida", typ)
	}
	if status == "served" && typ != "dine_in" {
		w.Violate(t, InvOrderTerminalTyp, "%s buyurtma `served` holatida", typ)
	}

	w.R().Checked(InvOrderHistory)
	hist, _ := o["history"].([]any)
	prev := ""
	for i, h := range hist {
		m, _ := h.(map[string]any)
		from, _ := m["from"].(string)
		to, _ := m["to"].(string)
		if i > 0 && from != prev {
			w.Violate(t, InvOrderHistory, "tarix uzilgan: %d-qadam `%s`dan, oldingisi `%s`da tugagan", i, from, prev)
		}
		okTrans := false
		for _, a := range validTransitions[from] {
			if a == to {
				okTrans = true
			}
		}
		if !okTrans {
			w.Violate(t, InvOrderHistory, "mumkin bo'lmagan o'tish %q -> %q", from, to)
		}
		prev = to
	}
	if len(hist) > 0 && prev != status {
		w.Violate(t, InvOrderHistory, "tarix `%s`da tugaydi, buyurtma holati `%s`", prev, status)
	}
}

// CheckItemsTotal — qisqartirilgan buyurtma ko'rinishi (ofitsiant): jami
// qatorlar yig'indisiga teng bo'lishi shart (stolda yetkazish haqi yo'q).
func (w *World) CheckItemsTotal(t *testing.T, o map[string]any) {
	t.Helper()
	w.R().Checked(InvOrderItemsSum)
	var sum int64
	items, _ := o["items"].([]any)
	for _, it := range items {
		m, _ := it.(map[string]any)
		sum += num(m, "price_tiyin") * num(m, "qty")
	}
	if total := num(o, "total_tiyin"); total != sum {
		w.Violate(t, InvOrderItemsSum, "jami %d != qatorlar yig'indisi %d", total, sum)
	}
}

// CheckOrderMatchesQuote — mijoz ekranda ko'rgan jami = yaratilgan buyurtma jami.
func (w *World) CheckOrderMatchesQuote(t *testing.T, quote, order map[string]any) {
	t.Helper()
	w.R().Checked(InvOrderQuoteMatch)
	if q, o := num(quote, "total_tiyin"), num(order, "total_tiyin"); q != o {
		w.Violate(t, InvOrderQuoteMatch, "quote jami %d, buyurtma jami %d", q, o)
	}
	if q, o := num(quote, "delivery_fee_tiyin"), num(order, "delivery_fee_tiyin"); q != o {
		w.Violate(t, InvOrderQuoteMatch, "quote yetkazish %d, buyurtma yetkazish %d", q, o)
	}
}

// CheckGatewayPayment — shlyuzga ketgan summa buyurtma jamiga teng va
// fiskal chek yig'indisi to'lov summasiga teng.
func (w *World) CheckGatewayPayment(t *testing.T, body map[string]any, orderTotal int64) {
	t.Helper()
	w.R().Checked(InvPayAmount)
	amount, _ := body["amount"].(float64)
	if int64(amount) != orderTotal {
		w.Violate(t, InvPayAmount, "shlyuzga %d ketdi, buyurtma jami %d", int64(amount), orderTotal)
	}
	if items, ok := body["ofd"].([]any); ok && len(items) > 0 {
		w.R().Checked(InvFiscalSum)
		var sum float64
		for _, it := range items {
			m, _ := it.(map[string]any)
			tot, _ := m["total"].(float64)
			sum += tot
		}
		if int64(sum) != int64(amount) {
			w.Violate(t, InvFiscalSum, "fiskal chek yig'indisi %d != to'lov %d", int64(sum), int64(amount))
		}
	}
}

// ExpectDenied — begona rol/tenant ruxsat OLMASLIGI kerak (401/403/404).
func (w *World) ExpectDenied(t *testing.T, a *Actor, method, path string, body any) {
	t.Helper()
	w.R().Checked(InvTenantIsolation)
	r := w.Do(t, a, method, path, body)
	switch r.Status {
	case 401, 403, 404:
	default:
		w.Violate(t, InvTenantIsolation, "%s %s (%s) -> %d, rad etilishi kerak edi", method, path, actorName(a), r.Status)
	}
}

func actorName(a *Actor) string {
	if a == nil {
		return "anonim"
	}
	return a.Name
}

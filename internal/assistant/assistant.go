// Package assistant — OnDex ilovasining ICHIDAGI AI yordamchisi.
//
// ┌─ NEGA BU `agentapi` DAN BOSHQA ────────────────────────────────────┐
// `internal/agentapi` — TASHQI agent (boshqa ilova) foydalanuvchi
// nomidan ish qilishi uchun: u yerda grant, ruxsatlar va pul
// chegarasi kerak, chunki so'rov begona serverdan keladi.
//
// Bu paket esa BOSHQA holat: foydalanuvchi OnDex ilovasining o'zida
// o'tirib, o'z sessiyasi bilan gaplashadi. Shuning uchun bu yerda:
//
//   - grant YO'Q — kim so'rayotgani JWT dan ma'lum;
//   - pul chegarasi YO'Q — chunki yordamchi PUL SARFLAY OLMAYDI:
//     u faqat TAKLIF qaytaradi, buyurtmani odam tugma bosib beradi;
//   - tashqi tomonga foydalanuvchi ma'lumoti berilmaydi.
//
// Shaddiy bu yerda faqat TIL MODELI: u "keyingi qadam" ni aytadi,
// tool'larni esa OnDex O'ZI, foydalanuvchining o'z huquqlari bilan
// bajaradi. Shaddiy buzilgan taqdirda ham u hech kimning akkauntiga
// tegolmaydi — eng ko'pi noto'g'ri taom taklif qiladi.
// └────────────────────────────────────────────────────────────────────┘
package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"chustapp/internal/catalog"
	"chustapp/internal/orders"
)

var (
	ErrNotConfigured = errors.New("AI yordamchi sozlanmagan")
	ErrTooLong       = errors.New("xabar juda uzun")
)

const (
	// maxUserMessage — bitta xabar uzunligi.
	//
	// Har belgi modelga pul turadi va uzun matn (masalan
	// nusxalangan kitob) javob sifatini ham pasaytiradi.
	maxUserMessage = 1000

	// maxHistory — mijozdan qabul qilinadigan suhbat tarixi
	// (juftliklar emas, XABARLAR soni).
	//
	// Tarix MIJOZDA saqlanadi va har so'rovda yuboriladi — server
	// sessiya ushlab turmaydi. Bu ataylab: sessiyasiz xizmatni
	// qayta ishga tushirish, kengaytirish va nosozligini topish
	// osonroq. Xavf yo'q, chunki tarix FAQAT o'sha foydalanuvchining
	// o'z suhbatiga ta'sir qiladi va barcha tool'lar baribir uning
	// o'z huquqlari doirasida bajariladi.
	maxHistory = 16

	// maxSteps — bitta savolda nechta tool bosqichiga ruxsat.
	//
	// "Osh qidir -> menyuni ko'r -> taklif qil" uchun 4 yetarli.
	// Chegarasiz qoldirilsa siklga tushgan model cheksiz so'rov
	// yuborardi (LLM'larda odatiy nosozlik).
	maxSteps = 5
)

// ── Bog'liqliklar (portlar) ──

// LLM — "keyingi qadamni ayt". Shaddiy shu interfeysni bajaradi.
type LLM interface {
	Complete(ctx context.Context, userRef string, messages []Message, tools []Tool) (*Reply, error)
}

// Pricer — savatni katalog bo'yicha tekshiradi va narxlaydi.
type Pricer interface {
	PriceOrder(ctx context.Context, reqs []catalog.ItemRequest) (string, []orders.Item, error)
}

// Quoter — aksiya/chegirmalarni hisobga olgan YAKUNIY summa.
//
// Checkout ekrani ko'rsatadigan AYNAN o'sha hisob ishlatiladi —
// aks holda yordamchi bir raqamni aytib, savatda boshqasi chiqardi.
type Quoter interface {
	Quote(ctx context.Context, restaurantID string, items []orders.Item, customerID string) (*orders.QuoteResult, error)
}

// OrderReader — foydalanuvchining o'z buyurtmalari.
type OrderReader interface {
	ListByCustomer(ctx context.Context, customerID string, limit int) ([]*orders.Order, error)
	GetByID(ctx context.Context, id string) (*orders.Order, error)
}

// Canceller — buyurtmani bekor qilish (holat mashinasi orqali).
type Canceller interface {
	ChangeStatus(ctx context.Context, orderID string, to orders.Status, by orders.Actor) (*orders.Order, error)
}

// ── Xabar turlari ──

// Message — suhbatning bitta qatori (OpenAI-mos shakl).
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content,omitempty"`
	// ToolCalls / ToolCallID — ichki sikl uchun. Mijozdan
	// KELMAYDI va mijozga QAYTARILMAYDI: ilova faqat matnni
	// ko'rsatadi, tool mexanikasi serverda qoladi.
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Tool — model chaqira oladigan amal ta'rifi.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

// Reply — LLM javobi.
type Reply struct {
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls"`
}

// ── Natija ──

// ProposedItem — taklif qilingan savatning bitta qatori.
type ProposedItem struct {
	ProductID  string `json:"product_id"`
	Name       string `json:"name"`
	Qty        int    `json:"qty"`
	PriceTiyin int64  `json:"price_tiyin"`
}

// Proposal — yordamchi tayyorlagan savat.
//
// ┌─ BU BUYURTMA EMAS ─────────────────────────────────────────────────┐
// Server bu yerda HECH NARSA yaratmaydi. Ilova buni karta qilib
// chizadi va foydalanuvchi ikki yo'ldan birini tanlaydi:
//
//	"Tasdiqlash"   -> ilova odatdagi `POST /orders` ni chaqiradi;
//	"Savatga"      -> taomlar mavjud savatga qo'shiladi, checkout ochiladi.
//
// Ikkala yo'l ham ALLAQACHON SINALGAN kod. Ya'ni til modeli
// tizimga yangi pul oqimi qo'shmaydi — u faqat savatni to'ldiradi.
// └────────────────────────────────────────────────────────────────────┘
type Proposal struct {
	RestaurantID   string         `json:"restaurant_id"`
	RestaurantName string         `json:"restaurant_name,omitempty"`
	Items          []ProposedItem `json:"items"`
	SubtotalTiyin  int64          `json:"subtotal_tiyin"`
	DiscountTiyin  int64          `json:"discount_tiyin,omitempty"`
	TotalTiyin     int64          `json:"total_tiyin"`
}

// Result — bitta savolning yakuniy natijasi.
type Result struct {
	Reply string `json:"reply"`
	// Proposal — ilova karta qilib ko'rsatadigan savat taklifi.
	Proposal *Proposal `json:"proposal,omitempty"`
}

// ── Xizmat ──

type Service struct {
	llm     LLM
	catalog catalog.Repository
	pricer  Pricer
	quoter  Quoter
	orders  OrderReader
	cancel  Canceller
}

func NewService(llm LLM, cat catalog.Repository, pricer Pricer, quoter Quoter,
	ord OrderReader, cancel Canceller) *Service {
	return &Service{llm: llm, catalog: cat, pricer: pricer,
		quoter: quoter, orders: ord, cancel: cancel}
}

// Chat — foydalanuvchining bitta savoli.
//
// `userID` FAQAT sessiyadan (JWT) keladi — model unga ta'sir qila
// olmaydi va tool'lar boshqa hech kimning ma'lumotiga tegmaydi.
//
// `disabledTools` — foydalanuvchi O'CHIRGAN amallar. Ular modelga
// e'lon QILINMAYDI va chaqirilsa ham bajarilmaydi (`allowedTools`,
// `runTool`). Ro'yxat serverdagi profildan olinadi, mijozdan emas.
func (s *Service) Chat(ctx context.Context, userID, message string,
	history []Message, disabledTools []string) (*Result, error) {

	message = strings.TrimSpace(message)
	if message == "" {
		return nil, errors.New("xabar bo'sh")
	}
	if len([]rune(message)) > maxUserMessage {
		return nil, ErrTooLong
	}

	msgs := make([]Message, 0, len(history)+3)
	msgs = append(msgs, Message{
		Role:    "system",
		Content: systemPrompt + checkoutNote(disabledSet(disabledTools)),
	})
	msgs = append(msgs, trimHistory(history)...)
	msgs = append(msgs, Message{Role: "user", Content: message})

	var proposal *Proposal
	disabled := disabledSet(disabledTools)
	tools := allowedTools(disabled)

	for step := 0; step < maxSteps; step++ {
		reply, err := s.llm.Complete(ctx, userID, msgs, tools)
		if err != nil {
			return nil, err
		}
		if len(reply.ToolCalls) == 0 {
			return &Result{Reply: plainText(reply.Content), Proposal: proposal}, nil
		}

		// Modelning tool-call xabari tarixga qo'shiladi — busiz
		// keyingi so'rovda "javob nimaga tegishli" ekani noma'lum
		// bo'lardi.
		msgs = append(msgs, Message{
			Role: "assistant", Content: reply.Content, ToolCalls: reply.ToolCalls,
		})

		for _, tc := range reply.ToolCalls {
			var args map[string]any
			// Buzilgan JSON — odatiy hol (model uni o'zi yozadi).
			// Xato emas: bo'sh argument bilan davom etamiz va tool
			// o'zi tushunarli xato qaytaradi.
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				args = map[string]any{}
			}
			out, prop := s.runTool(ctx, userID, tc.Function.Name, args, disabled)
			if prop != nil {
				// Oxirgi taklif ustun turadi: model savatni
				// tuzatib qayta taklif qilishi mumkin.
				proposal = prop
			}
			raw, err := json.Marshal(out)
			if err != nil {
				raw = []byte(`{"ok":false,"error":"natijani o'qib bo'lmadi"}`)
			}
			slog.Info("assistant tool", "user", userID, "tool", tc.Function.Name)
			msgs = append(msgs, Message{
				Role: "tool", ToolCallID: tc.ID, Content: string(raw),
			})
		}
	}

	// Sikl tugadi — model qaror qabul qila olmadi.
	return &Result{
		Reply:    "Kechirasiz, so'rovingizni tushunolmadim. Soddaroq qilib ayting.",
		Proposal: proposal,
	}, nil
}

// plainText — javobdan markdown belgilarini olib tashlaydi.
//
// ┌─ NEGA PROMPTGA ISHONMAYMIZ ────────────────────────────────────────┐
// `systemPrompt` da "markdown ishlatma" deb yozilgan, LEKIN model buni
// muntazam e'tiborsiz qoldiradi (sinovda: "Jami: **49 000 so'm**",
// "• Osh (2 dona)"). Bu shunchaki chiroyi masalasi emas — javob
// `flutter_tts` bilan OVOZ CHIQARIB o'qiladi va yulduzchalar
// "yulduzcha yulduzcha qirq to'qqiz ming" bo'lib eshitiladi.
//
// Shu paketning boshqa joylaridagi qoida bilan bir xil: prompt —
// sifat uchun, kafolat esa KODDA.
//
// Ataylab KAM ish qilinadi: faqat ajratish belgilari olinadi, matn
// qayta tartiblanmaydi. Havola/jadval kabi murakkab shakllar bu yerda
// uchramaydi (yordamchi qisqa og'zaki javob beradi).
// └────────────────────────────────────────────────────────────────────┘
func plainText(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))

	// Qator boshidagi sarlavha (`##`) va ro'yxat belgilari (`-`, `*`,
	// `•`) olib tashlanadi; qator ichidagi `*`/`_`/`` ` `` esa
	// hamma joyda.
	for i, line := range strings.Split(s, "\n") {
		if i > 0 {
			b.WriteByte('\n')
		}
		trimmed := strings.TrimLeft(line, " \t")
		indent := line[:len(line)-len(trimmed)]

		trimmed = strings.TrimLeft(trimmed, "#")
		for _, bullet := range []string{"• ", "- ", "* ", "+ "} {
			if strings.HasPrefix(trimmed, bullet) {
				trimmed = trimmed[len(bullet):]
				break
			}
		}
		b.WriteString(indent)
		// `_` ATAYLAB tegilmaydi: modellar kursiv uchun deyarli har
		// doim `*` ishlatadi, `_` esa so'z ichida uchraydi
		// (`propose_order`, `product_id`) — uni o'chirish foydadan
		// ko'ra ko'proq zarar berardi.
		b.WriteString(strings.Map(func(r rune) rune {
			switch r {
			case '*', '`', '#':
				return -1
			}
			return r
		}, trimmed))
	}
	return strings.TrimSpace(b.String())
}

// trimHistory — mijozdan kelgan tarixni xavfsiz holga keltiradi.
//
// ┌─ NEGA TOZALASH SHART ──────────────────────────────────────────────┐
// Tarix MIJOZDAN keladi, ya'ni unga istalgan narsa yozilishi mumkin.
// Ikki xavf bor va ikkalasi ham shu yerda yopiladi:
//
//  1. `role: "system"` — mijoz o'zining ko'rsatmasini qo'shib,
//     yordamchining qoidalarini almashtira olardi. Faqat `user` va
//     `assistant` qabul qilinadi.
//  2. `tool_calls` — soxta "tool natijasi" yozib, modelga
//     yolg'on ma'lumot berish. Ular butunlay tashlanadi.
//
// Uchinchi xavf — hajm: uzun tarix to'g'ridan-to'g'ri pul, shuning
// uchun oxirgi `maxHistory` ta xabar qoladi.
// └────────────────────────────────────────────────────────────────────┘
func trimHistory(history []Message) []Message {
	out := make([]Message, 0, maxHistory)
	for _, m := range history {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		if len([]rune(content)) > maxUserMessage {
			content = string([]rune(content)[:maxUserMessage])
		}
		out = append(out, Message{Role: m.Role, Content: content})
	}
	if len(out) > maxHistory {
		out = out[len(out)-maxHistory:]
	}
	return out
}

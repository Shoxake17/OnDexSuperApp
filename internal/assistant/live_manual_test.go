package assistant

// QO'LDA ishga tushiriladigan tekshiruvlar — ovozli rejim ko'rsatmasi
// modelga qanday ta'sir qilishini HAQIQIY chaqiruvda ko'rish uchun.
//
// Tarmoqqa chiqadi va pullik API'ga uriladi, shuning uchun
// `GEMINI_API_KEY` bo'lmasa o'tkazib yuboriladi (CI'da shunday).
//
// Ishga tushirish:
//
//	go test ./internal/assistant/ -run TestLive -v -count=1

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"google.golang.org/genai"
)

func liveTestSession(t *testing.T, withTools bool) (*genai.Session, func()) {
	t.Helper()
	key := strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	if key == "" {
		t.Skip("GEMINI_API_KEY yo'q")
	}
	model := strings.TrimSpace(os.Getenv("GEMINI_LIVE_MODEL"))
	if model == "" {
		model = defaultLiveModel
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: key, Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	cfg := &genai.LiveConnectConfig{
		// AUDIO — ishlab chiqarishdagi bilan bir xil. `native-audio`
		// modellari TEXT rejimini umuman qo'llamaydi, javob esa
		// transkripsiya orqali o'qiladi.
		ResponseModalities: []genai.Modality{genai.ModalityAudio},
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: liveIdentity + systemPrompt + liveVoiceExtra}},
		},
		ThinkingConfig:           &genai.ThinkingConfig{IncludeThoughts: false},
		OutputAudioTranscription: &genai.AudioTranscriptionConfig{},
	}
	if withTools {
		cfg.Tools = geminiTools(nil)
	}
	sess, err := client.Live.Connect(ctx, model, cfg)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	return sess, func() { sess.Close(); cancel() }
}

// liveAsk — bitta savol yuboradi va javob matnini (transkripsiya) hamda
// chaqirilgan amallar ro'yxatini qaytaradi.
func liveAsk(t *testing.T, sess *genai.Session, q string) (string, []string) {
	t.Helper()
	if err := sess.SendClientContent(genai.LiveClientContentInput{
		Turns:        []*genai.Content{{Role: "user", Parts: []*genai.Part{{Text: q}}}},
		TurnComplete: genai.Ptr(true),
	}); err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	var tools []string
	for i := 0; i < 300; i++ {
		msg, err := sess.Receive()
		if err != nil {
			t.Fatal(err)
		}
		if msg.ToolCall != nil {
			for _, fc := range msg.ToolCall.FunctionCalls {
				tools = append(tools, fc.Name)
			}
			// Tool javobini kutmaymiz: bizni QAYSI amal birinchi
			// chaqirilgani qiziqtiradi.
			return sb.String(), tools
		}
		sc := msg.ServerContent
		if sc == nil {
			continue
		}
		if sc.OutputTranscription != nil {
			sb.WriteString(sc.OutputTranscription.Text)
		}
		if sc.ModelTurn != nil {
			for _, p := range sc.ModelTurn.Parts {
				if p.Text != "" && !p.Thought {
					sb.WriteString(p.Text)
				}
			}
		}
		if sc.TurnComplete {
			break
		}
	}
	return sb.String(), tools
}

// Shaxs: ism "Shaddiy", yaratuvchi "Shoxrux", model nomi aytilmaydi.
func TestLiveIdentity(t *testing.T) {
	sess, done := liveTestSession(t, false)
	defer done()
	for _, q := range []string{
		"Salom, sen kimsan?",
		"Seni kim yaratgan?",
		"Sen Gemini'misan?",
	} {
		reply, _ := liveAsk(t, sess, q)
		t.Logf("SAVOL: %s\nJAVOB: %s", q, reply)
	}
}

// ID QOIDASI: buyurtma so'ralganda model AVVAL qidiruvni chaqirishi
// kerak — ID larni o'zidan to'qimasligi uchun.
//
// ┌─ NEGA BU TEKSHIRUV BOR ────────────────────────────────────────────┐
// Aynan shu buzilgan edi (2026-09-01): model to'g'ridan-to'g'ri
// `propose_order` chaqirib, TO'QILGAN ID lar bilan "taom topilmadi"
// olardi, keyin esa foydalanuvchiga "savatga qo'shdim" derdi. Savat
// bo'sh qolardi va nosozlik hech qayerda ko'rinmasdi.
// └────────────────────────────────────────────────────────────────────┘
func TestLiveOrderCallsSearchFirst(t *testing.T) {
	sess, done := liveTestSession(t, true)
	defer done()

	reply, tools := liveAsk(t, sess, "Menga ikkita osh buyurtma qil")
	t.Logf("javob: %q; chaqirilgan amallar: %v", reply, tools)
	if len(tools) == 0 {
		t.Fatal("model hech qanday amal chaqirmadi — buyurtma umuman berilmasdi")
	}
	switch tools[0] {
	case "search_food", "list_restaurants", "restaurant_menu":
		// To'g'ri: avval ID lar topiladi.
	case "propose_order":
		t.Errorf("model qidiruvsiz propose_order chaqirdi — ID lar to'qilgan"+
			" bo'ladi va buyurtma 'taom topilmadi' bilan rad etiladi"+
			" (chaqiruvlar: %v)", tools)
	default:
		t.Errorf("kutilmagan birinchi amal: %s", tools[0])
	}
}

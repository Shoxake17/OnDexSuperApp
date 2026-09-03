package assistant

// live.go — ovozli rejim: Gemini Live orqali jonli audio suhbat.
//
// ┌─ NEGA BU ALOHIDA YO'L ─────────────────────────────────────────────┐
// Matnli chat (`Chat`) Shaddiy'ga boradi va u FAQAT til modeli.
// Ovozli rejim esa boshqacha: mikrofon oqimi real vaqtda modelga
// ketadi va model AUDIO qaytaradi (sintez qilingan emas, o'zining
// tabiiy ovozi). Buni qurilmadagi `speech_to_text` + `flutter_tts`
// bilan qilib bo'lmaydi — ularda o'zbekcha model ham, o'zbekcha ovoz
// ham yo'q (telefonda sinaldi: tanish `ru-RU` ga tushardi, ovoz esa
// o'zbek matnini rus ovozi bilan o'qirdi).
//
// MUHIM QAROR (2026-08-31): shu sabab "audio telefondan chiqmaydi"
// qoidasi ovozli rejim uchun BEKOR QILINDI. Mikrofon oqimi OnDex
// serveriga, u yerdan Google'ga boradi. Matnli chat esa avvalgidek
// audiosiz qoladi. Foydalanuvchidan ovozli rejim ochilganda alohida
// rozilik so'raladi.
// └────────────────────────────────────────────────────────────────────┘
//
// ┌─ XAVFSIZLIK — INVARIANTLAR O'ZGARMADI ─────────────────────────────┐
// Kalit FAQAT serverda: ilova Gemini'ga umuman bormaydi, u OnDex
// API'siga ulanadi.
//
// Tool'lar AYNAN matnli chatdagi ro'yxat (`toolDefs`) va AYNAN
// o'sha `runTool` bilan bajariladi. Ya'ni:
//   - `userID` JWT dan keladi, model unga ta'sir qila olmaydi;
//   - `propose_order` HECH NARSA yaratmaydi — narxlangan taklif
//     qaytaradi, tugmani ODAM bosadi (ovozdagi "ha" YETARLI EMAS);
//   - narx har doim katalogdan.
// Ovozli rejim yangi imkoniyat qo'shmaydi, faqat yangi INTERFEYS.
// └────────────────────────────────────────────────────────────────────┘

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"google.golang.org/genai"
)

// Audio formatlari — Gemini Live qat'iy belgilaydi, tanlash yo'q.
const (
	// LiveInputMIME — mijozdan keladigan mikrofon oqimi.
	LiveInputMIME = "audio/pcm;rate=16000"
	// LiveOutputRate — modeldan keladigan audio (ilova shu chastotada
	// chalishi kerak, aks holda ovoz "cho'zilgan" eshitiladi).
	LiveOutputRate = 24000
)

// maxAudioChunk — mijozdan bir marta qabul qilinadigan eng katta
// bo'lak (xom baytlarda, ~1 soniyalik 16kHz PCM16 dan sezilarli
// katta). Chegara yo'q bo'lsa bitta ulanish xotirani yeyishi mumkin.
const maxAudioChunk = 64 << 10

// LiveEvent — seansdan ilovaga ketadigan hodisa.
//
// Bitta oqimda audio ham, holat ham yuboriladi: ikki kanal bo'lsa
// ularning tartibi kafolatlanmasdi va "gapirib bo'ldi" signali audio
// tugashidan oldin kelib qolardi.
type LiveEvent struct {
	Type string `json:"type"`

	// Audio — base64 PCM16 @24kHz (`Type == "audio"`).
	Audio string `json:"audio,omitempty"`

	// Text — modelning matnli transkripti, bo'lsa (`Type == "text"`).
	Text string `json:"text,omitempty"`

	// Proposal — `propose_order` natijasi (`Type == "proposal"`).
	// Ilova buni karta qilib chizadi; buyurtma shu kartadagi tugma
	// bilan, odatdagi `POST /orders` orqali beriladi.
	Proposal *Proposal `json:"proposal,omitempty"`

	// Error — `Type == "error"` bo'lganda foydalanuvchiga
	// ko'rsatiladigan sabab.
	Error string `json:"error,omitempty"`
}

// Hodisa turlari.
const (
	LiveEventReady       = "ready"
	LiveEventAudio       = "audio"
	LiveEventText        = "text"
	// LiveEventUserText — foydalanuvchi AYTGAN gapning matni.
	// Suhbat oynasida u o'z savolini ham ko'rishi kerak, aks holda
	// javob nimaga tegishli ekani noaniq bo'lardi.
	LiveEventUserText = "user_text"
	LiveEventProposal    = "proposal"
	// LiveEventToolError — amal bajarilmadi (masalan taom topilmadi).
	//
	// Ilova buni suhbatda XATO qatori qilib ko'rsatadi. Busiz
	// foydalanuvchi modelning "qildim" degan gapiga ishonardi, ilovada
	// esa hech narsa o'zgarmagan bo'lardi.
	LiveEventToolError = "tool_error"
	// LiveEventConfirmOrder — foydalanuvchi og'zaki rozilik berdi.
	//
	// Server BUYURTMA YARATMAYDI: ilova buni olib, checkout
	// ekranidagi AYNAN o'sha tugmani bosadi (naqd to'lov).
	LiveEventConfirmOrder = "confirm_order"
	LiveEventInterrupted = "interrupted"
	LiveEventTurnEnd     = "turn_end"
	LiveEventError       = "error"
)

// LiveConfig — ovozli rejim sozlamasi.
type LiveConfig struct {
	APIKey string
	Model  string
}

// defaultLiveModel — jonli audio uchun model.
//
// ┌─ NOMNI TAXMIN QILMANG ─────────────────────────────────────────────┐
// Live modellari tez eskiradi. `gemini-2.0-flash-live-001` (Shaddiy
// hamon shuni yozib qo'ygan) 2026-08-31 da ARTIQ YO'Q:
//
//   models/gemini-2.0-flash-live-001 is not found for API version
//   v1beta, or is not supported for bidiGenerateContent
//
// Amaldagi ro'yxatni SO'RANG, eslab qolmang:
//
//   GET https://generativelanguage.googleapis.com/v1beta/models
//   -> supportedGenerationMethods ichida "bidiGenerateContent"
//
// `native-audio` ataylab tanlandi: uning ovozi eng tabiiy (sintez
// emas), ya'ni funksiyaning butun maqsadi shu.
// └────────────────────────────────────────────────────────────────────┘
const defaultLiveModel = "gemini-2.5-flash-native-audio-latest"

// LiveConfigFromEnv — sozlama bor bo'lsa `ok = true`.
//
// Kalit yo'q bo'lsa ovozli rejim butunlay o'chadi: ilova tugmani
// ko'rsatmaydi (`GET /ai/status` dagi `voice` maydoni). Yarim
// ishlaydigan holat bo'lmaydi.
func LiveConfigFromEnv() (LiveConfig, bool) {
	key := strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	if key == "" {
		return LiveConfig{}, false
	}
	model := strings.TrimSpace(os.Getenv("GEMINI_LIVE_MODEL"))
	if model == "" {
		model = defaultLiveModel
	}
	return LiveConfig{APIKey: key, Model: model}, true
}

// liveIdentity — yordamchining SHAXSI.
//
// ┌─ NEGA BU FAQAT OVOZLI YO'LDA ──────────────────────────────────────┐
// Matnli chatda so'rov Shaddiy serveriga boradi va u O'Z tizim
// ko'rsatmasini (ismi, "meni Shoxrux yaratgan") oldiga qo'shadi.
// Shuning uchun `systemPrompt` da faqat OnDex'ga xos VAZIFA turadi —
// shaxs bir joyda, provayderda.
//
// Ovozli rejimda esa Shaddiy zanjirda YO'Q: OnDex Gemini bilan
// to'g'ridan-to'g'ri gaplashadi. Natijada shaxsni hech kim
// qo'shmasdi va modelga faqat "Sen OnDex ilovasidagi yordamchisan"
// yetib borardi — u o'zini "OnDex yordamchisi" deb tanishtirardi,
// "kim yaratgan?" degan savolga esa Google deb javob berardi.
//
// Matn Shaddiy'ning `agent-chat.service.ts` dagi ko'rsatmasi bilan
// mos: ikki rejimda ikki xil shaxs bo'lsa, foydalanuvchi buni
// darhol sezardi.
// └────────────────────────────────────────────────────────────────────┘
const liveIdentity = `Sening isming — Shaddiy. Sen Shaddiy Ai Agent'san.

KIMSAN:
- "Sen kimsan?" deb so'ralsa AYNAN shunday javob ber: "Men Shaddiy. Sizga taom
  tanlash va buyurtma berishda yordam beraman." Boshqa hech narsa qo'shma.
- O'zingni ta'riflaganda OnDex nomini ISHLATMA: "OnDex yordamchisi", "OnDex
  ilovasining yordamchisiman" kabi iboralar NOTO'G'RI. OnDex — sen ishlayotgan
  ilova, sening isming ham, unvoning ham emas. Sening isming faqat Shaddiy.
- Seni Shoxrux yaratgan. "Seni kim yaratgan/yasagan/dasturlagan?" yoki shunga
  o'xshash savolga ALBATTA "Meni Shoxrux yaratgan" deb javob ber. HECH QACHON
  Google, Gemini, OpenAI yoki boshqa kompaniya deb aytma.
- Qaysi model yoki texnologiyada ishlashing haqida so'ralsa, bu haqda
  gapirma va mavzuni foydalanuvchining ishiga qaytar.

`

// liveVoiceExtra — ovozga xos qo'shimcha qoidalar.
//
// Vazifaning o'zi ATAYLAB matnli chatdagi bilan bir xil
// (`systemPrompt`): ikki rejim ikki xil qoida bilan ishlasa,
// foydalanuvchi ovozda bir javob, matnda boshqa javob olardi va
// sabab hech qayerda ko'rinmasdi.
const liveVoiceExtra = `

OVOZLI SUHBAT QOIDALARI:
- Sen HOZIR ovoz bilan gaplashyapsan. Javoblar qisqa bo'lsin — bir-ikki gap.
- Raqamlarni so'z bilan ayt ("qirq besh ming so'm"), belgi va qisqartma ishlatma.
- Ro'yxatni sanab o'tirma; eng mosini ayt va "boshqasini ham aytaymi?" deb so'ra.
- propose_order dan keyin JAMI SUMMANI ayt va foydalanuvchi ekrandagi
  tugmani bosishi kerakligini tushuntir. Sen buyurtma BERA OLMAYSAN va
  foydalanuvchi ovoz bilan "ha" desa ham bermaysan.`

// geminiTools — `toolDefs` ni Gemini formatiga o'giradi.
//
// Ro'yxat BITTA manbadan olinadi: matnli chat va ovoz har xil
// tool'larga ega bo'lsa, "buyurtma ber" matnda ishlab ovozda
// ishlamasdi va model buni xato deb aytmasdi — u shunchaki "qila
// olmayman" derdi.
func geminiTools(disabled map[string]bool) []*genai.Tool {
	defs := allowedTools(disabled)
	decls := make([]*genai.FunctionDeclaration, 0, len(defs))
	for _, t := range defs {
		params, _ := t.Function.Parameters.(map[string]any)
		decls = append(decls, &genai.FunctionDeclaration{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  toGeminiSchema(params),
		})
	}
	return []*genai.Tool{{FunctionDeclarations: decls}}
}

// toGeminiSchema — JSON Schema (map) → `*genai.Schema`.
//
// ┌─ NEGA QO'LDA, `json.Unmarshal` BILAN EMAS ─────────────────────────┐
// `genai.Schema.Type` — enum va u KATTA harf kutadi ("OBJECT"),
// bizning ta'riflarimizda esa odatdagi JSON Schema yozuvi ("object").
// To'g'ridan-to'g'ri unmarshal qilinsa tur "object" bo'lib qolardi va
// Gemini ta'rifni rad etardi — tool'lar jimgina yo'qolardi.
// └────────────────────────────────────────────────────────────────────┘
func toGeminiSchema(m map[string]any) *genai.Schema {
	if len(m) == 0 {
		return nil
	}
	s := &genai.Schema{}
	if v, ok := m["type"].(string); ok {
		s.Type = genai.Type(strings.ToUpper(v))
	}
	if v, ok := m["description"].(string); ok {
		s.Description = v
	}
	if props, ok := m["properties"].(map[string]any); ok && len(props) > 0 {
		s.Properties = make(map[string]*genai.Schema, len(props))
		for name, raw := range props {
			if sub, ok := raw.(map[string]any); ok {
				s.Properties[name] = toGeminiSchema(sub)
			}
		}
	}
	if items, ok := m["items"].(map[string]any); ok {
		s.Items = toGeminiSchema(items)
	}
	s.Required = toStringSlice(m["required"])
	s.Enum = toStringSlice(m["enum"])
	return s
}

// toStringSlice — ta'riflarda `[]string` ham, `[]any` ham uchraydi
// (birinchisi qo'lda yozilgan, ikkinchisi JSON'dan o'qilgan).
func toStringSlice(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// LiveSession — bitta foydalanuvchining ochiq ovozli seansi.
type LiveSession struct {
	svc    *Service
	userID string
	sess   *genai.Session

	// disabled — foydalanuvchi o'chirgan amallar (`runTool` ikkinchi
	// qavat tekshiruvi uchun).
	disabled map[string]bool

	events chan LiveEvent

	// sendMu — `genai.Session` bir vaqtda bitta yozuvchini kutadi.
	// Audio mijoz goroutine'idan, tool javoblari esa qabul
	// siklidan yuboriladi — ya'ni ikki yozuvchi bor.
	sendMu sync.Mutex

	closeOnce sync.Once
}

// StartLive — Gemini Live seansini ochadi.
//
// `userID` FAQAT sessiyadan (JWT) keladi — `Chat` dagi bilan bir xil
// qoida.
//
// `disabledTools` — foydalanuvchi o'chirgan amallar; ular modelga
// e'lon qilinmaydi va chaqirilsa ham bajarilmaydi.
func (s *Service) StartLive(ctx context.Context, cfg LiveConfig,
	userID string, disabledTools []string) (*LiveSession, error) {

	if cfg.APIKey == "" {
		return nil, errors.New("GEMINI_API_KEY sozlanmagan")
	}
	if userID == "" {
		return nil, errors.New("userID bo'sh")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, err
	}

	sess, err := client.Live.Connect(ctx, cfg.Model, &genai.LiveConnectConfig{
		// Faqat AUDIO: matn ham so'ralsa model ikkalasini ham
		// generatsiya qiladi va javob sezilarli sekinlashadi.
		ResponseModalities: []genai.Modality{genai.ModalityAudio},
		SystemInstruction: &genai.Content{
			// Shaxs BIRINCHI: `systemPrompt` "Sen OnDex ilovasidagi
			// yordamchisan" deb boshlanadi va undan oldin ism
			// aytilmasa model o'zini shu ibora bilan tanishtiradi.
			Parts: []*genai.Part{{
				Text: liveIdentity + systemPrompt +
					checkoutNote(disabledSet(disabledTools)) + liveVoiceExtra,
			}},
		},
		Tools: geminiTools(disabledSet(disabledTools)),
		// ┌─ "O'YLASH" MATNI SO'RALMAYDI ─────────────────────────────┐
		// `native-audio` modellari fikrlash jarayonini ham qaytaradi
		// va u AYNAN javob matni bilan bir kanaldan keladi. Sinovda
		// suhbat oynasiga shunday tushardi:
		//
		//   "**Crafting the Uzbek Response** I'm currently
		//    formulating the Uzbek response..."
		//
		// ya'ni foydalanuvchi o'zbekcha javob o'rniga modelning
		// inglizcha ichki mulohazasini o'qirdi.
		//
		// Bu YETARLI EMAS — quyida `p.Thought` ham tekshiriladi.
		// Sozlama modelga qarab e'tiborsiz qoldirilishi mumkin,
		// filtr esa har doim ishlaydi.
		// └───────────────────────────────────────────────────────────┘
		ThinkingConfig: &genai.ThinkingConfig{IncludeThoughts: false},
		// ┌─ TRANSKRIPSIYA — SUHBAT MATNI UCHUN ──────────────────────┐
		// `native-audio` modellari FAQAT audio qaytaradi: `p.Text`
		// hech qachon to'lmaydi. Busiz ovozli suhbat izsiz o'tardi —
		// foydalanuvchi rejimni yopgach nima gaplashilgani, ayniqsa
		// aytilgan SUMMA, hech qayerda qolmasdi.
		//
		// Ikkalasi ham so'raladi: kirish (foydalanuvchi gapi) va
		// chiqish (Shaddiy javobi) — suhbat ikki tomonlama yoziladi.
		// └───────────────────────────────────────────────────────────┘
		InputAudioTranscription:  &genai.AudioTranscriptionConfig{},
		OutputAudioTranscription: &genai.AudioTranscriptionConfig{},
		// ┌─ TIL ATAYLAB KO'RSATILMAGAN ──────────────────────────────┐
		// `SpeechConfig.LanguageCode` ga "uz-UZ" berilsa Gemini uni
		// qo'llamasa ULANISH umuman ochilmaydi. Shaddiy'da bu maydon
		// ham bo'sh va model tizim ko'rsatmasidagi o'zbek tilini
		// o'zi tutib oladi — ya'ni sinovdan o'tgan yo'l shu.
		// └───────────────────────────────────────────────────────────┘
	})
	if err != nil {
		return nil, err
	}

	l := &LiveSession{
		svc:      s,
		userID:   userID,
		sess:     sess,
		disabled: disabledSet(disabledTools),
		// Bufer: audio bo'laklari tez-tez keladi, mijoz esa ularni
		// tarmoqqa yozadi. Bufer to'lsa seans yopiladi — eski
		// audioni to'plab turishning ma'nosi yo'q, u eskirgan gap.
		events: make(chan LiveEvent, 64),
	}
	go l.receive(ctx)
	return l, nil
}

// Events — seansdan keladigan hodisalar. Seans yopilganda kanal ham
// yopiladi.
func (l *LiveSession) Events() <-chan LiveEvent { return l.events }

// SendAudio — mijozdan kelgan mikrofon bo'lagi (xom PCM16 @16kHz).
func (l *LiveSession) SendAudio(pcm []byte) error {
	if len(pcm) == 0 {
		return nil
	}
	if len(pcm) > maxAudioChunk {
		return errors.New("audio bo'lagi juda katta")
	}
	l.sendMu.Lock()
	defer l.sendMu.Unlock()
	return l.sess.SendRealtimeInput(genai.LiveRealtimeInput{
		Media: &genai.Blob{Data: pcm, MIMEType: LiveInputMIME},
	})
}

// AudioStreamEnd — mikrofon o'chirildi (foydalanuvchi tugmani bosdi).
//
// Busiz model gap tugaganini bilmay kutib turardi: avtomatik nutq
// aniqlash jimlikni kutadi, mikrofon esa allaqachon yopilgan va
// jimlik ham kelmaydi.
func (l *LiveSession) AudioStreamEnd() error {
	l.sendMu.Lock()
	defer l.sendMu.Unlock()
	return l.sess.SendRealtimeInput(genai.LiveRealtimeInput{AudioStreamEnd: true})
}

// NotifyCheckoutReady — ilova rasmiylashtirish ekraniga yetib bordi.
//
// ┌─ MATNNI SERVER YOZADI, MIJOZ EMAS ─────────────────────────────────┐
// Mijozga ixtiyoriy matn yuborishga ruxsat berilsa, u modelning
// ko'rsatmasini qayta yozib yuborishi mumkin edi (prompt injection).
// Shuning uchun kanal orqali faqat FAKTLAR keladi — restoran nomi va
// summa — gapni esa shu yerda server tuzadi.
//
// Nom foydalanuvchi kiritadigan qiymat emas (katalogdan), lekin baribir
// uzunligi cheklanadi: modelga cheksiz matn yuborilmasin.
// └────────────────────────────────────────────────────────────────────┘
func (l *LiveSession) NotifyCheckoutReady(restaurant string, totalTiyin int64) error {
	if l.disabled[CapCheckout] {
		// Ruxsat yo'q — bunday holat umuman bo'lmasligi kerak
		// (ilova ham tekshiradi), lekin ikkinchi qavat zarar qilmaydi.
		return nil
	}
	restaurant = trimRunes(strings.TrimSpace(restaurant), 80)
	if restaurant == "" {
		restaurant = "restoran"
	}
	if totalTiyin < 0 {
		totalTiyin = 0
	}

	text := fmt.Sprintf(
		"[TIZIM XABARI — foydalanuvchi aytmadi] Rasmiylashtirish ekrani "+
			"ochildi. Restoran: %s. Jami: %d so'm. HOZIR foydalanuvchidan "+
			"so'ra: restoran nomini va summani ayt, keyin \"naqd to'lov "+
			"bilan buyurtma beraymi?\" deb so'ra. Javobini kut.",
		restaurant, totalTiyin/100)

	l.sendMu.Lock()
	defer l.sendMu.Unlock()
	return l.sess.SendClientContent(genai.LiveClientContentInput{
		Turns: []*genai.Content{{
			Role:  "user",
			Parts: []*genai.Part{{Text: text}},
		}},
		TurnComplete: genai.Ptr(true),
	})
}

// Close — seansni yopadi. Ikki marta chaqirilsa xavfsiz.
func (l *LiveSession) Close() error {
	var err error
	l.closeOnce.Do(func() { err = l.sess.Close() })
	return err
}

// emit — hodisani navbatga qo'yadi. Navbat to'lsa `false` qaytaradi
// va chaqiruvchi seansni yopadi (sekin mijoz butun seansni ushlab
// turmasin).
func (l *LiveSession) emit(e LiveEvent) bool {
	select {
	case l.events <- e:
		return true
	default:
		return false
	}
}

// receive — Gemini'dan keladigan xabarlar sikli.
func (l *LiveSession) receive(ctx context.Context) {
	defer close(l.events)
	defer l.Close()

	l.emit(LiveEvent{Type: LiveEventReady})

	for {
		msg, err := l.sess.Receive()
		if err != nil {
			// Kontekst bekor qilingan bo'lsa bu odatdagi yopilish,
			// nosozlik emas — foydalanuvchiga xato ko'rsatilmaydi.
			if ctx.Err() == nil {
				slog.Warn("assistant live: qabul uzildi",
					"user", l.userID, "err", err)
				l.emit(LiveEvent{
					Type:  LiveEventError,
					Error: "Ovozli aloqa uzildi. Qayta urinib ko'ring.",
				})
			}
			return
		}
		if msg == nil {
			continue
		}

		// Tool chaqiruvlari — bajarib, natijani qaytaramiz.
		if msg.ToolCall != nil && len(msg.ToolCall.FunctionCalls) > 0 {
			if !l.handleToolCalls(ctx, msg.ToolCall.FunctionCalls) {
				return
			}
		}

		sc := msg.ServerContent
		if sc == nil {
			continue
		}
		if sc.ModelTurn != nil {
			for _, p := range sc.ModelTurn.Parts {
				if p == nil {
					continue
				}
				if p.InlineData != nil && len(p.InlineData.Data) > 0 {
					if !l.emit(LiveEvent{
						Type:  LiveEventAudio,
						Audio: base64.StdEncoding.EncodeToString(p.InlineData.Data),
					}) {
						slog.Warn("assistant live: mijoz sekin, seans yopildi",
							"user", l.userID)
						return
					}
				}
				// Fikrlash bo'laklari suhbatga TUSHMAYDI — yuqoridagi
				// `ThinkingConfig` izohiga qarang.
				if p.Text != "" && !p.Thought {
					l.emit(LiveEvent{Type: LiveEventText, Text: plainText(p.Text)})
				}
			}
		}
		// Transkripsiya — suhbat oynasiga yoziladigan matn.
		//
		// Bo'laklab keladi ("Avigo", " restoranida", ...), shuning
		// uchun ilova ularni oxirgi qatorga QO'SHIB boradi, har
		// bo'lakni alohida xabar qilib emas.
		if sc.OutputTranscription != nil && sc.OutputTranscription.Text != "" {
			l.emit(LiveEvent{
				Type: LiveEventText,
				Text: sc.OutputTranscription.Text,
			})
		}
		if sc.InputTranscription != nil && sc.InputTranscription.Text != "" {
			l.emit(LiveEvent{
				Type: LiveEventUserText,
				Text: sc.InputTranscription.Text,
			})
		}
		if sc.Interrupted {
			// Foydalanuvchi model gapirayotganda gapirdi: ilova
			// navbatdagi audioni TASHLASHI kerak, aks holda eski
			// javob yangi savol ustidan gapiraveradi.
			l.emit(LiveEvent{Type: LiveEventInterrupted})
		}
		if sc.TurnComplete {
			l.emit(LiveEvent{Type: LiveEventTurnEnd})
		}
	}
}

// handleToolCalls — tool'larni bajaradi va natijani Gemini'ga
// qaytaradi. `false` — seansni to'xtatish kerak.
func (l *LiveSession) handleToolCalls(ctx context.Context,
	calls []*genai.FunctionCall) bool {

	responses := make([]*genai.FunctionResponse, 0, len(calls))
	for _, fc := range calls {
		if fc == nil {
			continue
		}
		out, prop := l.svc.runTool(ctx, l.userID, fc.Name, fc.Args, l.disabled)

		// ┌─ XATO ENDI KO'RINADI ─────────────────────────────────────┐
		// Ilgari natija faqat modelga qaytarilardi va u xatoni
		// e'tiborsiz qoldirib "savatga qo'shdim" deb aytaverardi —
		// savat esa bo'sh qolardi. Ya'ni yordamchi ALDARDI, sabab
		// esa hech qayerda ko'rinmasdi.
		//
		// Endi nosozlik ikki joyga chiqadi: serverda logga (nima
		// bo'lganini ko'rish uchun) va ilovaga hodisa sifatida
		// (foydalanuvchi haqiqatni ko'rishi uchun).
		// └───────────────────────────────────────────────────────────┘
		toolErr := ""
		if m, ok := out.(map[string]any); ok {
			if okFlag, has := m["ok"].(bool); has && !okFlag {
				toolErr, _ = m["error"].(string)
				if toolErr == "" {
					toolErr = "amal bajarilmadi"
				}
			}
		}
		if toolErr != "" {
			slog.Warn("assistant live tool XATO", "user", l.userID,
				"tool", fc.Name, "xato", toolErr, "args", fc.Args)
			if !l.emit(LiveEvent{
				Type:  LiveEventToolError,
				Text:  toolErr,
				Error: fc.Name,
			}) {
				return false
			}
		} else {
			slog.Info("assistant live tool", "user", l.userID, "tool", fc.Name)
			// Og'zaki tasdiq — ilova checkout tugmasini bosadi.
			if fc.Name == "confirm_order" {
				if !l.emit(LiveEvent{Type: LiveEventConfirmOrder}) {
					return false
				}
			}
		}

		if prop != nil {
			// Taklif ilovaga AYNAN matnli chatdagi shaklda ketadi —
			// bitta karta widget'i ikkala rejimga xizmat qiladi.
			if !l.emit(LiveEvent{Type: LiveEventProposal, Proposal: prop}) {
				return false
			}
		}

		// `runTool` har doim map qaytaradi; Gemini `response` ni
		// obyekt sifatida kutadi.
		resp, ok := out.(map[string]any)
		if !ok {
			resp = map[string]any{"result": out}
		}
		responses = append(responses, &genai.FunctionResponse{
			ID:       fc.ID,
			Name:     fc.Name,
			Response: resp,
		})
	}
	if len(responses) == 0 {
		return true
	}

	l.sendMu.Lock()
	err := l.sess.SendToolResponse(genai.LiveToolResponseInput{
		FunctionResponses: responses,
	})
	l.sendMu.Unlock()
	if err != nil {
		slog.Warn("assistant live: tool javobini yuborib bo'lmadi",
			"user", l.userID, "err", err)
		return false
	}
	return true
}

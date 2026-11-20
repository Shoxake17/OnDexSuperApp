package assistant

import (
	"context"
	"strings"
)

// userMsgKey — joriy so'rovdagi foydalanuvchi xabari (ctx qiymati).
type userMsgKey struct{}

func withUserMessage(ctx context.Context, msg string) context.Context {
	return context.WithValue(ctx, userMsgKey{}, msg)
}

func userMessage(ctx context.Context) string {
	s, _ := ctx.Value(userMsgKey{}).(string)
	return s
}

// cancelWords — "bekor qilish" niyatini bildiruvchi so'zlar (lotin,
// kirill o'zbekcha/ruscha va inglizcha). Ataylab KENG: halol foydalanuvchi
// rad etilmasin; maqsad — foydalanuvchi umuman so'ramagan holatda (begona
// matn ta'sirida) bekor qilishni to'sish.
var cancelWords = []string{
	"bekor", "otmen", "отмен", "бекор", "cancel", "kerak emas", "kerakmas",
	"керак эмас", "не нужн", "qaytaring", "voz kech", "воз кеч",
}

// cancelIntent — xabarda bekor qilish so'rovi bormi.
func cancelIntent(msg string) bool {
	m := strings.ToLower(strings.NewReplacer("ʻ", "'", "ʼ", "'", "‘", "'", "’", "'", "`", "'").Replace(msg))
	for _, w := range cancelWords {
		if strings.Contains(m, w) {
			return true
		}
	}
	return false
}

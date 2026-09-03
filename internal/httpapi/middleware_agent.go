// Tashqi AI agentlar uchun kirish nazorati.
//
// ┌─ NEGA `Server.auth` DAN ALOHIDA ───────────────────────────────────┐
// `auth` — foydalanuvchi JWT'si: bitta sir, bitta shaxs. Agent
// so'rovi esa IKKI mustaqil sirdan iborat (sherik kaliti + grant) va
// ularning bog'liqligi ham tekshiriladi. Bu mantiqni `auth` ichiga
// tiqish uni ikkala holat uchun ham tushunarsiz qilardi.
//
// MUHIM: bu ikki yo'l HECH QACHON kesishmaydi. Agent tokeni bilan
// oddiy endpointga kirib bo'lmaydi va foydalanuvchi JWT'si bilan
// agent endpointiga. Shu sababli agent uchun berilgan ruxsatlar
// oddiy API'ning butun yuzasiga tarqalmaydi.
// └────────────────────────────────────────────────────────────────────┘
package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"chustapp/internal/agentapi"
	"chustapp/internal/ratelimit"
)

// ---- Tezlik cheklovlari ----
//
// Ikki o'lchov bo'yicha, chunki ular turli hujumlarni to'xtatadi:
//
//	agentPartnerLimiter — sherikning BUTUN integratsiyasi bo'yicha.
//	  Buzilgan yoki xato yozilgan sherik serveri butun API'ni
//	  band qilib qo'ymasin.
//	agentGrantLimiter — bitta foydalanuvchi bo'yicha. Sherik halol
//	  bo'lsa ham, uning ichidagi til modeli siklga tushib bitta
//	  odamning nomidan yuzlab so'rov yuborishi mumkin (LLM'larda bu
//	  odatiy nosozlik).
var (
	agentPartnerLimiter = ratelimit.New(20, 200)
	agentGrantLimiter   = ratelimit.New(2, 30)

	// agentWriteLimiter — buyurtma yaratish/tasdiqlash. Qattiqroq:
	// har bir muvaffaqiyatli amal HAQIQIY PUL va restoranda ish.
	// Grant bo'yicha soatiga ~12 (portlash 5) — halol foydalanuvchi
	// bunga yetib bormaydi, sikldagi model esa darhol to'xtaydi.
	agentWriteLimiter = ratelimit.New(0.0033, 5)

	// agentLinkLimiter — ulanish kodini TERISH urinishlari (IP
	// bo'yicha). Kod 8 belgi (32^8) va 10 daqiqa yashaydi; bu
	// chegara bilan taxmin qilish amalda imkonsiz bo'ladi.
	agentLinkLimiter = ratelimit.New(0.1, 10)
)

type agentCtxKey int

const (
	partnerKey agentCtxKey = iota
	grantKey
)

func partnerFrom(r *http.Request) *agentapi.Partner {
	p, _ := r.Context().Value(partnerKey).(*agentapi.Partner)
	return p
}

func grantFrom(r *http.Request) *agentapi.Grant {
	g, _ := r.Context().Value(grantKey).(*agentapi.Grant)
	return g
}

// agentHeaderGrant — grant tokeni keladigan sarlavha.
//
// `Authorization` sherik kaliti bilan band, shuning uchun grant
// alohida sarlavhada. Ikkalasini bitta sarlavhaga birlashtirish
// (masalan "key:grant") mumkin edi, lekin unda loglarda yoki xato
// xabarida bittasi ko'rinib qolsa ikkinchisi ham oshkor bo'lardi.
const agentHeaderGrant = "X-OnDex-Grant"

// agentStatus — domen xatosini HTTP statusiga aylantiradi.
func agentStatus(err error) int {
	switch {
	case errors.Is(err, agentapi.ErrPartnerKey),
		errors.Is(err, agentapi.ErrGrantInvalid),
		errors.Is(err, agentapi.ErrGrantRevoked),
		errors.Is(err, agentapi.ErrGrantExpired):
		return http.StatusUnauthorized
	case errors.Is(err, agentapi.ErrPartnerOff),
		errors.Is(err, agentapi.ErrScopeDenied),
		errors.Is(err, agentapi.ErrTestPartner):
		return http.StatusForbidden
	case errors.Is(err, agentapi.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, agentapi.ErrAwaitingUser):
		// 202 — "qabul qilindi, lekin hali bajarilmadi". Agent buni
		// ko'rib foydalanuvchiga "ilovangizda tasdiqlang" deydi.
		return http.StatusAccepted
	case errors.Is(err, agentapi.ErrLinkPending):
		return http.StatusAccepted
	case errors.Is(err, agentapi.ErrLinkExpired),
		errors.Is(err, agentapi.ErrDraftExpired):
		return http.StatusGone
	case errors.Is(err, agentapi.ErrLinkDecided),
		errors.Is(err, agentapi.ErrDraftDecided),
		errors.Is(err, agentapi.ErrTotalMismatch),
		errors.Is(err, agentapi.ErrOverDaily):
		return http.StatusConflict
	}
	return http.StatusBadRequest
}

// agentPartner — FAQAT sherik kalitini talab qiladigan endpointlar
// (ulanish oqimi, `ping`). Foydalanuvchiga tegishli hech narsa
// ochilmaydi.
func (s *Server) agentPartner(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.AgentSvc == nil {
			httpError(w, http.StatusServiceUnavailable,
				errors.New("agent integratsiyasi sozlanmagan"))
			return
		}
		// ┌─ ★ BRAUZERDAN KELGAN SO'ROV RAD ETILADI ───────────────┐
		// Sherik kaliti FAQAT server tomonida turishi kerak. Uni
		// brauzerdagi JS'ga qo'yish — kalitni butun internetga
		// e'lon qilish bilan barobar.
		//
		// Brauzer cross-origin so'rovda `Origin` ni HAR DOIM
		// yuboradi va uni JS o'zgartira olmaydi. Ya'ni bu tekshiruv
		// integratsiyachining eng qimmat xatosini birinchi
		// so'rovdayoq, jimgina emas — ANIQ xato bilan ushlaydi.
		//
		// (Server-server so'rovda `Origin` bo'lmaydi, shuning uchun
		// halol integratsiyaga bu umuman sezilmaydi.)
		// └────────────────────────────────────────────────────────┘
		if r.Header.Get("Origin") != "" {
			httpError(w, http.StatusForbidden, errors.New(
				"agent API brauzerdan chaqirilmaydi — API kalit faqat serveringizda turishi kerak"))
			return
		}
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			httpError(w, http.StatusUnauthorized,
				errors.New("Authorization: Bearer <API kalit> talab qilinadi"))
			return
		}
		p, err := s.AgentSvc.AuthenticatePartner(r.Context(),
			strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			httpError(w, agentStatus(err), err)
			return
		}
		if ok, wait := agentPartnerLimiter.AllowWithWait(p.ID); !ok {
			tooManyRequests(w, wait)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), partnerKey, p)))
	}
}

// agentAuth — sherik kaliti VA foydalanuvchi granti.
//
// `scope` bo'sh bo'lmasa, grantda o'sha ruxsat borligi ham
// tekshiriladi — ya'ni har bir endpoint o'z ruxsatini o'zi e'lon
// qiladi va uni unutib qoldirish mumkin emas.
func (s *Server) agentAuth(scope string, next http.HandlerFunc) http.HandlerFunc {
	return s.agentPartner(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get(agentHeaderGrant)
		if strings.TrimSpace(token) == "" {
			httpError(w, http.StatusUnauthorized,
				errors.New(agentHeaderGrant+" talab qilinadi — foydalanuvchi hali ulanmagan"))
			return
		}
		p := partnerFrom(r)
		g, err := s.AgentSvc.AuthenticateGrant(r.Context(), p, token)
		if err != nil {
			httpError(w, agentStatus(err), err)
			return
		}
		if scope != "" {
			if err := s.AgentSvc.RequireScope(g, scope); err != nil {
				// Rad etilgan urinish auditga TUSHADI: foydalanuvchi
				// integratsiya nima so'ramoqchi bo'lganini ko'rishi
				// kerak (masalan buyurtma berishga ruxsat bermagan
				// bo'lsa-yu, agent baribir urinayotgan bo'lsa).
				s.AgentSvc.Audit(r.Context(), p, g, "scope.denied", scope, false, clientIP(r))
				httpError(w, agentStatus(err), err)
				return
			}
		}
		if ok, wait := agentGrantLimiter.AllowWithWait(g.ID); !ok {
			tooManyRequests(w, wait)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), grantKey, g)))
	})
}

// agentWrite — pul sarflaydigan amallar uchun qo'shimcha chegara.
func agentWrite(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		g := grantFrom(r)
		if g != nil {
			if ok, wait := agentWriteLimiter.AllowWithWait(g.ID); !ok {
				tooManyRequests(w, wait)
				return
			}
		}
		next(w, r)
	}
}

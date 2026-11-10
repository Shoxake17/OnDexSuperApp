package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"chustapp/internal/identity"
	"chustapp/internal/identity/passkey"
	"chustapp/internal/ratelimit"
	"chustapp/internal/users"
)

// Merchant realmiga kirish (A3b, docs/architecture/AUTH-PHASE-A.md 5, 7-bo'limlar).
//
// ┌─ OQIM ────────────────────────────────────────────────────────────────┐
//   Kod yetkazish: POST /auth/telegram/start yoki /auth/request-code (mavjud)
//
//   POST /v2/auth/merchant/password   {phone, password, client_id, device_name}
//   POST /v2/auth/merchant/otp/verify {phone, code, client_id, device_name}
//        -> {access_token, refresh_token, ...}        (xodim)
//        -> {mfa_required, mfa_token, method, ...}    (egasi)
//
//   method = "otp" | "password" -> POST /v2/auth/mfa/factor        {mfa_token, code | password}
//   method = "totp_enroll"      -> POST /v2/auth/mfa/totp/enroll   {mfa_token} -> {secret, otpauth_uri}
//                                  POST /v2/auth/mfa/totp/confirm  {mfa_token, code}
//                                       -> sessiya + recovery_codes (BIR MARTA ko'rsatiladi)
//   method = "totp"             -> POST /v2/auth/mfa/verify        {mfa_token, code}
//                                  (code — TOTP yoki tiklash kodi)
// └───────────────────────────────────────────────────────────────────────┘
//
// Javoblar keshlanmaydi (`no-store`). Akkaunt mavjudligi hech qaysi
// javobdan bilinmaydi: mijoz, xodim parol bilan, yo'q akkaunt — bir xil 401.

// merchantAuthReady — merchant kirishi ulanganmi (identity + authz + mfa + shifr kaliti).
func (s *Server) merchantAuthReady(w http.ResponseWriter) bool {
	if s.MerchantAuth == nil {
		httpError(w, http.StatusServiceUnavailable, errors.New("biznes kabinetiga yangi kirish hali yoqilmagan"))
		return false
	}
	return true
}

// respondMerchantAuthError — identity xatolari avval (ular users.IsUserFacing
// ro'yxatida yo'q va aks holda 500 bo'lib ketardi), qolgani `respondAuthError`.
func respondMerchantAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, identity.ErrMFAInvalid), errors.Is(err, identity.ErrMFATokenInvalid):
		httpError(w, http.StatusUnauthorized, err)
	case errors.Is(err, identity.ErrMFATooMany):
		tooManyRequests(w, mfaRetryAfter)
	case errors.Is(err, identity.ErrAppNotAllowed), errors.Is(err, identity.ErrRealmMismatch):
		httpError(w, http.StatusForbidden, err)
	case errors.Is(err, identity.ErrUnknownClient):
		httpError(w, http.StatusBadRequest, err)
	default:
		respondAuthError(w, err, http.StatusUnauthorized)
	}
}

func writeOutcome(w http.ResponseWriter, out *identity.Outcome) {
	noStore(w)
	if out.Tokens != nil {
		writeJSON(w, http.StatusOK, out.Tokens)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"mfa_required": true, "mfa_token": out.MFA.Token, "method": out.MFA.Method, "expires_at": out.MFA.ExpiresAt,
		"passkey": out.MFA.Passkey,
	})
}

// respondPasskeyError — passkey xatolari (qolgani respondMerchantAuthError).
func respondPasskeyError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, identity.ErrPasskeyDisabled):
		httpError(w, http.StatusServiceUnavailable, err)
	case errors.Is(err, identity.ErrPasskeyState), errors.Is(err, identity.ErrPasskeyNone),
		errors.Is(err, identity.ErrPasskeyNeedsTOTP), errors.Is(err, passkey.ErrTooMany), errors.Is(err, passkey.ErrExists):
		httpError(w, http.StatusBadRequest, err)
	case errors.Is(err, passkey.ErrNotFound):
		httpError(w, http.StatusNotFound, err)
	default:
		respondMerchantAuthError(w, err)
	}
}

func merchantMeta(r *http.Request, device string) identity.Meta {
	return identity.Meta{IP: clientIP(r), UserAgent: r.UserAgent(), DeviceName: device}
}

// mfaIPLimiter — ikkinchi bosqich endpointlari (akkaunt bo'yicha blok
// `identity.MerchantAuth` ichida). Kod kiritish limiteridan alohida.
var mfaIPLimiter = ratelimit.New(0.2, 30)

// passkeyIPLimiter — passkey endpointlari (alohida oila: TOTP kiritish
// limitini sarflamaydi). Akkaunt bo'yicha xato hisobi — TOTP bilan umumiy.
var passkeyIPLimiter = ratelimit.New(0.2, 30)

// maxSecretLen — mfa_token uchun yuqori chegara.
const maxSecretLen = 1024

// mfaRetryAfter — akkaunt bo'yicha MFA bloki (`identity.MerchantAuth`: 15
// daqiqada 5 ta xato -> 15 daqiqa).
const mfaRetryAfter = 15 * time.Minute

func (s *Server) registerMerchantAuthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v2/auth/merchant/password", func(w http.ResponseWriter, r *http.Request) {
		if !s.merchantAuthReady(w) || rateLimited(w, loginIPLimiter, clientIP(r)) {
			return
		}
		var req struct {
			Phone      string `json:"phone"`
			Password   string `json:"password"`
			ClientID   string `json:"client_id"`
			DeviceName string `json:"device_name"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		if len(req.Phone) > users.MaxEmailLength || len(req.Password) > users.MaxPasswordLength {
			httpError(w, http.StatusUnauthorized, users.ErrInvalidCredentials)
			return
		}
		if rateLimited(w, loginAccountLimiter, loginKey(req.Phone)) {
			return
		}
		out, err := s.MerchantAuth.LoginPassword(r.Context(), req.Phone, req.Password, req.ClientID,
			merchantMeta(r, req.DeviceName))
		if err != nil {
			respondMerchantAuthError(w, err)
			return
		}
		writeOutcome(w, out)
	})

	mux.HandleFunc("POST /v2/auth/merchant/otp/verify", func(w http.ResponseWriter, r *http.Request) {
		if !s.merchantAuthReady(w) || rateLimited(w, verifyIPLimiter, clientIP(r)) {
			return
		}
		var req struct {
			Phone      string `json:"phone"`
			Code       string `json:"code"`
			ClientID   string `json:"client_id"`
			DeviceName string `json:"device_name"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		out, err := s.MerchantAuth.LoginCode(r.Context(), req.Phone, req.Code, req.ClientID, merchantMeta(r, req.DeviceName))
		if err != nil {
			respondMerchantAuthError(w, err)
			return
		}
		writeOutcome(w, out)
	})

	// Admin (platform realmi, A6): kod -> parol -> TOTP (har kirishda).
	// Keyingi bosqichlar umumiy /v2/auth/mfa/* orqali (mfa_token ichida
	// klient = admin-web, realm akkauntdan).
	mux.HandleFunc("POST /v2/auth/admin/otp/verify", func(w http.ResponseWriter, r *http.Request) {
		if !s.merchantAuthReady(w) || rateLimited(w, verifyIPLimiter, clientIP(r)) {
			return
		}
		if !s.adminIPAllowed(r) {
			httpError(w, http.StatusForbidden, errAdminIPDenied)
			return
		}
		var req struct {
			Phone      string `json:"phone"`
			Code       string `json:"code"`
			DeviceName string `json:"device_name"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		out, err := s.MerchantAuth.AdminLoginCode(r.Context(), req.Phone, req.Code, merchantMeta(r, req.DeviceName))
		if err != nil {
			respondMerchantAuthError(w, err)
			return
		}
		writeOutcome(w, out)
	})

	// Ikkinchi bosqich endpointlari: IP bo'yicha cheklov + akkaunt bo'yicha
	// cheklov (`MerchantAuth` ichida, mfa_token egasi bo'yicha).
	type mfaReq struct {
		MFAToken   string `json:"mfa_token"`
		Code       string `json:"code"`
		Password   string `json:"password"`
		DeviceName string `json:"device_name"`
	}
	decodeMFA := func(w http.ResponseWriter, r *http.Request) (*mfaReq, bool) {
		if !s.merchantAuthReady(w) || rateLimited(w, mfaIPLimiter, clientIP(r)) {
			return nil, false
		}
		var req mfaReq
		if !decodeJSON(w, r, &req) {
			return nil, false
		}
		if len(req.MFAToken) > maxSecretLen || len(req.Code) > 64 || len(req.Password) > users.MaxPasswordLength {
			httpError(w, http.StatusUnauthorized, identity.ErrMFATokenInvalid)
			return nil, false
		}
		return &req, true
	}

	mux.HandleFunc("POST /v2/auth/mfa/factor", func(w http.ResponseWriter, r *http.Request) {
		req, ok := decodeMFA(w, r)
		if !ok {
			return
		}
		secret := req.Code
		if req.Password != "" {
			secret = req.Password
		}
		out, err := s.MerchantAuth.AddFactor(r.Context(), req.MFAToken, secret, merchantMeta(r, req.DeviceName))
		if err != nil {
			respondMerchantAuthError(w, err)
			return
		}
		writeOutcome(w, out)
	})

	// password_set — parolsiz egasi kod bilan kirgach parolni O'RNATADI
	// (keyin TOTP ulash). Faqat amr=otp va parol hali yo'q akkauntda.
	mux.HandleFunc("POST /v2/auth/mfa/password", func(w http.ResponseWriter, r *http.Request) {
		if !s.merchantAuthReady(w) || rateLimited(w, mfaIPLimiter, clientIP(r)) {
			return
		}
		var req struct {
			MFAToken        string `json:"mfa_token"`
			Password        string `json:"password"`
			PasswordConfirm string `json:"password_confirm"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		if len(req.MFAToken) > maxSecretLen || len(req.Password) > users.MaxPasswordLength ||
			len(req.PasswordConfirm) > users.MaxPasswordLength {
			httpError(w, http.StatusUnauthorized, identity.ErrMFATokenInvalid)
			return
		}
		out, err := s.MerchantAuth.SetInitialPassword(r.Context(), req.MFAToken, req.Password, req.PasswordConfirm,
			merchantMeta(r, ""))
		if err != nil {
			if users.IsUserFacing(err) {
				httpError(w, http.StatusBadRequest, err) // zaif parol, mos kelmaydi
				return
			}
			respondMerchantAuthError(w, err)
			return
		}
		writeOutcome(w, out)
	})

	mux.HandleFunc("POST /v2/auth/mfa/verify", func(w http.ResponseWriter, r *http.Request) {
		req, ok := decodeMFA(w, r)
		if !ok {
			return
		}
		tok, err := s.MerchantAuth.VerifyMFA(r.Context(), req.MFAToken, req.Code, merchantMeta(r, req.DeviceName))
		if err != nil {
			respondMerchantAuthError(w, err)
			return
		}
		noStore(w)
		writeJSON(w, http.StatusOK, tok)
	})

	// Passkey (AUD-4) — method=totp bosqichida muqobil: avval so'rov
	// parametrlari (navigator.credentials.get), keyin imzolangan javob.
	// Faqat kirish sahifasi manzilida (origin) ishlaydi — fishing domenida
	// yaratilgan imzo rad etiladi.
	mux.HandleFunc("POST /v2/auth/mfa/passkey/options", func(w http.ResponseWriter, r *http.Request) {
		if !s.merchantAuthReady(w) || rateLimited(w, passkeyIPLimiter, clientIP(r)) {
			return
		}
		var req struct {
			MFAToken string `json:"mfa_token"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		if len(req.MFAToken) > maxSecretLen {
			httpError(w, http.StatusUnauthorized, identity.ErrMFATokenInvalid)
			return
		}
		opts, state, err := s.MerchantAuth.PasskeyLoginOptions(r.Context(), req.MFAToken)
		if err != nil {
			respondPasskeyError(w, err)
			return
		}
		noStore(w)
		writeJSON(w, http.StatusOK, map[string]any{"public_key": opts.Response, "state": state})
	})

	mux.HandleFunc("POST /v2/auth/mfa/passkey", func(w http.ResponseWriter, r *http.Request) {
		if !s.merchantAuthReady(w) || rateLimited(w, passkeyIPLimiter, clientIP(r)) {
			return
		}
		var req struct {
			MFAToken   string          `json:"mfa_token"`
			State      string          `json:"state"`
			Credential json.RawMessage `json:"credential"`
			DeviceName string          `json:"device_name"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		if len(req.MFAToken) > maxSecretLen || len(req.State) > 16<<10 || len(req.Credential) == 0 {
			httpError(w, http.StatusUnauthorized, identity.ErrMFATokenInvalid)
			return
		}
		tok, err := s.MerchantAuth.VerifyPasskey(r.Context(), req.MFAToken, req.State, req.Credential,
			merchantMeta(r, req.DeviceName))
		if err != nil {
			respondPasskeyError(w, err)
			return
		}
		noStore(w)
		writeJSON(w, http.StatusOK, tok)
	})

	mux.HandleFunc("POST /v2/auth/mfa/totp/enroll", func(w http.ResponseWriter, r *http.Request) {
		req, ok := decodeMFA(w, r)
		if !ok {
			return
		}
		en, err := s.MerchantAuth.EnrollStart(r.Context(), req.MFAToken)
		if err != nil {
			respondMerchantAuthError(w, err)
			return
		}
		noStore(w)
		// qr_png — QR rasm serverda chiziladi (data: URI): kalit uchinchi
		// tomon QR xizmatiga ketmaydi; web kirish sahifasi shuni ko'rsatadi.
		writeJSON(w, http.StatusOK, map[string]string{"secret": en.Secret, "otpauth_uri": en.URI, "qr_png": qrDataURI(en.URI)})
	})

	mux.HandleFunc("POST /v2/auth/mfa/totp/confirm", func(w http.ResponseWriter, r *http.Request) {
		req, ok := decodeMFA(w, r)
		if !ok {
			return
		}
		tok, codes, err := s.MerchantAuth.EnrollConfirm(r.Context(), req.MFAToken, req.Code, merchantMeta(r, req.DeviceName))
		if err != nil {
			respondMerchantAuthError(w, err)
			return
		}
		noStore(w)
		writeJSON(w, http.StatusOK, map[string]any{"tokens": tok, "recovery_codes": codes})
	})
}

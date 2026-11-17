package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"chustapp/internal/images"
	"chustapp/internal/modules/flow"
	"chustapp/internal/ratelimit"
	"chustapp/internal/users"
)

// OnDexFlow marshrutlari (Plane ONDEXECOSY-472).
//
//	Qurilma (Shorebird updater, ochiq; `shorebird.yaml`:
//	base_url: https://api.ondex.uz/flow):
//	  POST /flow/api/v1/patches/check
//	  POST /flow/api/v1/patches/events
//
//	Nashr (`cmd/ondexflow`; sessiyasiz — RSA imzo bilan, faqat QORALAMA):
//	  POST /flow/publish/releases   multipart: manifest, signature, libapp:<arch>, entries
//	  POST /flow/publish/patches    multipart: manifest, signature, patch:<arch>
//
//	Boshqaruv (FAQAT admin panel: admin roli; o'zgartirish = sabab +
//	Authenticator kodi + audit):
//	  GET  /admin/flow/apps
//	  POST /admin/flow/apps
//	  GET  /admin/flow/apps/{id}/releases
//	  GET  /admin/flow/releases/{id}/patches
//	  POST /admin/flow/patches/{id}/state   {status, rollout_percent, rev}

const flowPublishMaxBytes = 80 << 20

var (
	// Qurilma so'rovlari: IP bo'yicha keng (NAT ortida ko'p telefon bo'lishi
	// mumkin), lekin cheksiz emas.
	flowCheckIPLimiter = ratelimit.New(5, 120).Shared("flowCheckIPLimiter")
	flowEventIPLimiter = ratelimit.New(5, 120).Shared("flowEventIPLimiter")
	// Nashr: imzo tekshiruvi qimmat (RSA) — IP bo'yicha qattiq.
	flowPublishIPLimiter = ratelimit.New(0.1, 10).Shared("flowPublishIPLimiter")
)

// flowBlobs — modul porti -> mavjud media ombori (prod: R2, cdn.ondex.uz).
type flowBlobs struct{ store images.Store }

func (b flowBlobs) Put(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	if r2, ok := b.store.(*images.R2Store); ok {
		// Fayl nomi xeshga bog'liq emas, lekin bir marta yoziladi va
		// o'zgarmaydi (yangi yamoq = yangi raqam = yangi kalit).
		return r2.UploadWithOptions(ctx, key, bytes.NewReader(data), int64(len(data)), images.UploadOptions{
			ContentType: contentType, CacheControl: "public, max-age=31536000, immutable"})
	}
	return b.store.Upload(ctx, key, bytes.NewReader(data), int64(len(data)), contentType)
}

// FlowBlobs — main.go ulanishi uchun.
func FlowBlobs(store images.Store) flow.Blobs { return flowBlobs{store: store} }

func flowFail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, flow.ErrNotFound):
		httpError(w, http.StatusNotFound, err)
	case errors.Is(err, flow.ErrConflict), errors.Is(err, flow.ErrVersionLock), errors.Is(err, flow.ErrTransition):
		httpError(w, http.StatusConflict, err)
	case errors.Is(err, flow.ErrBadSignature), errors.Is(err, flow.ErrStale):
		httpError(w, http.StatusUnauthorized, err)
	case errors.Is(err, flow.ErrInvalid):
		httpError(w, http.StatusBadRequest, err)
	default:
		slog.Error("flow", "err", err)
		httpError(w, http.StatusInternalServerError, errors.New("ichki xato"))
	}
}

func (s *Server) flowReady(w http.ResponseWriter) bool {
	if s.FlowSvc == nil {
		httpError(w, http.StatusServiceUnavailable, errors.New("OnDexFlow sozlanmagan"))
		return false
	}
	return true
}

func (s *Server) registerFlowRoutes(mux *http.ServeMux) {
	adminOnly := []users.Role{users.RoleAdmin}

	// ── Qurilma ─────────────────────────────────────────────────────────
	mux.HandleFunc("POST /flow/api/v1/patches/check", func(w http.ResponseWriter, r *http.Request) {
		if rateLimited(w, flowCheckIPLimiter, clientIP(r)) || !s.flowReady(w) {
			return
		}
		var req flow.CheckRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
			httpError(w, http.StatusBadRequest, errors.New("noto'g'ri so'rov"))
			return
		}
		resp, err := s.FlowSvc.Check(r.Context(), req)
		if err != nil {
			flowFail(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("POST /flow/api/v1/patches/events", func(w http.ResponseWriter, r *http.Request) {
		if rateLimited(w, flowEventIPLimiter, clientIP(r)) || !s.flowReady(w) {
			return
		}
		var req struct {
			Event flow.Event `json:"event"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
			httpError(w, http.StatusBadRequest, errors.New("noto'g'ri so'rov"))
			return
		}
		if err := s.FlowSvc.Report(r.Context(), req.Event); err != nil {
			flowFail(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{})
	})

	// ── Nashr (imzolangan) ──────────────────────────────────────────────
	publish := func(kind string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if rateLimited(w, flowPublishIPLimiter, clientIP(r)) || !s.flowReady(w) {
				return
			}
			raw, sig, files, ok := readFlowMultipart(w, r)
			if !ok {
				return
			}
			var (
				out any
				err error
			)
			if kind == "release" {
				out, err = s.FlowSvc.PublishRelease(r.Context(), raw, sig, files)
			} else {
				out, err = s.FlowSvc.PublishPatch(r.Context(), raw, sig, files)
			}
			if err != nil {
				slog.Warn("flow nashr rad etildi", "kind", kind, "ip", clientIP(r), "err", err)
				flowFail(w, err)
				return
			}
			slog.Info("flow nashr", "kind", kind, "ip", clientIP(r))
			writeJSON(w, http.StatusCreated, out)
		}
	}
	mux.HandleFunc("POST /flow/publish/releases", publish("release"))
	mux.HandleFunc("POST /flow/publish/patches", publish("patch"))

	// ── Admin panel ─────────────────────────────────────────────────────
	mux.HandleFunc("GET /admin/flow/apps", s.auth(adminOnly, func(w http.ResponseWriter, r *http.Request) {
		if !s.flowReady(w) {
			return
		}
		apps, err := s.FlowSvc.ListApps(r.Context())
		if err != nil {
			flowFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"apps": apps})
	}))

	mux.HandleFunc("POST /admin/flow/apps", s.auth(adminOnly, s.adminAction("flow.app.create", true,
		func(w http.ResponseWriter, r *http.Request) {
			if !s.flowReady(w) {
				return
			}
			var in struct {
				ID        string `json:"id"`
				Name      string `json:"name"`
				Platform  string `json:"platform"`
				PublicKey string `json:"public_key"`
			}
			if !decodeJSON(w, r, &in) {
				return
			}
			app, err := s.FlowSvc.CreateApp(r.Context(), in.ID, in.Name, in.Platform, in.PublicKey, claimsFrom(r).Subject)
			if err != nil {
				flowFail(w, err)
				return
			}
			writeJSON(w, http.StatusCreated, app)
		})))

	mux.HandleFunc("GET /admin/flow/apps/{id}/releases", s.auth(adminOnly, func(w http.ResponseWriter, r *http.Request) {
		if !s.flowReady(w) {
			return
		}
		list, err := s.FlowSvc.ListReleases(r.Context(), r.PathValue("id"))
		if err != nil {
			flowFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"releases": list})
	}))

	mux.HandleFunc("GET /admin/flow/releases/{id}/patches", s.auth(adminOnly, func(w http.ResponseWriter, r *http.Request) {
		if !s.flowReady(w) {
			return
		}
		list, err := s.FlowSvc.ListPatches(r.Context(), r.PathValue("id"))
		if err != nil {
			flowFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"patches": list})
	}))

	mux.HandleFunc("POST /admin/flow/patches/{id}/state", s.auth(adminOnly, s.adminAction("flow.patch.state", true,
		func(w http.ResponseWriter, r *http.Request) {
			if !s.flowReady(w) {
				return
			}
			var in struct {
				Status  string `json:"status"`
				Rollout int    `json:"rollout_percent"`
				Rev     int    `json:"rev"`
			}
			if !decodeJSON(w, r, &in) {
				return
			}
			p, err := s.FlowSvc.SetPatchState(r.Context(), r.PathValue("id"), flow.PatchStatus(in.Status),
				in.Rollout, in.Rev, claimsFrom(r).Subject)
			if err != nil {
				flowFail(w, err)
				return
			}
			slog.Info("flow yamoq holati", "patch", p.ID, "number", p.Number, "status", p.Status,
				"rollout", p.Rollout, "by", claimsFrom(r).Subject)
			writeJSON(w, http.StatusOK, p)
		})))
}

// readFlowMultipart — qat'iy chegara bilan o'qiydi; faqat ma'lum maydonlar.
func readFlowMultipart(w http.ResponseWriter, r *http.Request) (raw []byte, sig string, files flow.Files, ok bool) {
	r.Body = http.MaxBytesReader(w, r.Body, flowPublishMaxBytes)
	mr, err := r.MultipartReader()
	if err != nil {
		httpError(w, http.StatusBadRequest, errors.New("multipart kutilgan"))
		return nil, "", nil, false
	}
	files = flow.Files{}
	for parts := 0; ; parts++ {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil || parts >= 10 {
			httpError(w, http.StatusBadRequest, errors.New("multipart o'qilmadi"))
			return nil, "", nil, false
		}
		name := part.FormName()
		b, err := io.ReadAll(part)
		_ = part.Close()
		if err != nil {
			httpError(w, http.StatusRequestEntityTooLarge, errors.New("fayl juda katta"))
			return nil, "", nil, false
		}
		switch {
		case name == "manifest":
			raw = b
		case name == "signature":
			sig = strings.TrimSpace(string(b))
		case name == "entries", strings.HasPrefix(name, "libapp:"), strings.HasPrefix(name, "patch:"):
			if _, dup := files[name]; dup {
				httpError(w, http.StatusBadRequest, errors.New("takroriy maydon"))
				return nil, "", nil, false
			}
			files[name] = b
		default:
			httpError(w, http.StatusBadRequest, errors.New("noma'lum maydon"))
			return nil, "", nil, false
		}
	}
	if len(raw) == 0 || sig == "" {
		httpError(w, http.StatusBadRequest, errors.New("manifest va signature shart"))
		return nil, "", nil, false
	}
	return raw, sig, files, true
}

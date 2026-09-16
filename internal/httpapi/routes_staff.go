package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"chustapp/internal/staff"
	"chustapp/internal/users"
)

// ┌─ "XODIMLAR" BO'LIMI ───────────────────────────────────────────────────┐
// Restoran o'z xodimlarini o'zi boshqaradi (foydalanuvchi qarori
// 2026-08-12). Chegara:
//   - restoran FAQAT o'z restoranining xodimlarini ko'radi/o'zgartiradi
//     (`EntityID` tokendan, so'rov tanasidan emas);
//   - javobda bog'langan akkaunt ID'si CHIQMAYDI;
//   - yozuv o'chirilmaydi — "ishdan bo'shagan" holati (tarix saqlanadi);
//   - ofitsiantning ilovaga kirishi yozuvga ergashadi (`staff.Service`).
// Affitsiant, kuryer, mijoz bu endpointlarga umuman kira olmaydi.
// └────────────────────────────────────────────────────────────────────────┘

const staffBodyLimit = 32 << 10

type staffMemberView struct {
	ID            string          `json:"id"`
	Number        int             `json:"number"`
	Code          string          `json:"code"`
	FirstName     string          `json:"first_name"`
	LastName      string          `json:"last_name"`
	FullName      string          `json:"full_name"`
	Phone         string          `json:"phone"`
	Position      string          `json:"position"`
	PositionTitle string          `json:"position_title"`
	Status        string          `json:"status"`
	StatusTitle   string          `json:"status_title"`
	Schedule      *staff.Schedule `json:"schedule"`
	HiredOn       *string         `json:"hired_on"`
	Note          string          `json:"note"`
	// MonthlySalaryTiyin — faqat restoran egasi/admin ko'radigan javobda.
	MonthlySalaryTiyin *int64     `json:"monthly_salary_tiyin"`
	AppAccess          bool       `json:"app_access"`
	AppAccessActive    bool       `json:"app_access_active"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	DismissedAt        *time.Time `json:"dismissed_at"`
}

func staffView(m *staff.Member) staffMemberView {
	v := staffMemberView{
		ID: m.ID, Number: m.Number, Code: m.Code(), FirstName: m.FirstName, LastName: m.LastName,
		FullName: m.FullName(), Phone: m.Phone, Position: string(m.Position),
		PositionTitle: m.Position.Title(), Status: string(m.Status), StatusTitle: m.Status.Title(),
		Schedule: m.Schedule, Note: m.Note, MonthlySalaryTiyin: m.SalaryTiyin, AppAccess: m.AppAccess,
		AppAccessActive: m.AccessEffective() && m.UserID != "",
		CreatedAt:       m.CreatedAt, UpdatedAt: m.UpdatedAt, DismissedAt: m.DismissedAt,
	}
	if m.HiredOn != nil {
		d := m.HiredOn.Format("2006-01-02")
		v.HiredOn = &d
	}
	return v
}

type staffEventView struct {
	ID         string    `json:"id"`
	MemberID   string    `json:"member_id"`
	MemberName string    `json:"member_name"`
	MemberCode string    `json:"member_code"`
	Kind       string    `json:"kind"`
	From       string    `json:"from"`
	To         string    `json:"to"`
	FromTitle  string    `json:"from_title"`
	ToTitle    string    `json:"to_title"`
	At         time.Time `json:"at"`
}

func staffEventViews(events []staff.Event, members map[string]*staff.Member) []staffEventView {
	out := make([]staffEventView, 0, len(events))
	for _, ev := range events {
		v := staffEventView{ID: ev.ID, MemberID: ev.MemberID, Kind: string(ev.Kind),
			From: ev.From, To: ev.To, At: ev.At}
		if m := members[ev.MemberID]; m != nil {
			v.MemberName, v.MemberCode = m.FullName(), m.Code()
		}
		switch ev.Kind {
		case staff.EventPositionChanged, staff.EventCreated:
			v.FromTitle, v.ToTitle = titleOrEmpty(staff.Position(ev.From).Title(), ev.From), staff.Position(ev.To).Title()
		case staff.EventStatusChanged:
			v.FromTitle, v.ToTitle = staff.Status(ev.From).Title(), staff.Status(ev.To).Title()
		}
		out = append(out, v)
	}
	return out
}

func titleOrEmpty(title, raw string) string {
	if raw == "" {
		return ""
	}
	return title
}

func staffHTTPError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, staff.ErrNotFound):
		httpError(w, http.StatusNotFound, err)
	case errors.Is(err, staff.ErrPhoneTaken), errors.Is(err, staff.ErrAccountConflict),
		errors.Is(err, staff.ErrLimitReached), errors.Is(err, staff.ErrCourierBusy):
		httpError(w, http.StatusConflict, err)
	case errors.Is(err, staff.ErrAccountsDisabled):
		httpError(w, http.StatusServiceUnavailable, err)
	case staff.IsValidation(err):
		httpError(w, http.StatusBadRequest, err)
	default:
		httpError(w, http.StatusInternalServerError, err)
	}
}

func (s *Server) registerStaffRoutes(mux *http.ServeMux) {
	roles := []users.Role{users.RoleRestaurant, users.RoleAdmin}

	// own — egalik (HAMMA narsadan oldin), xizmat, keyin restoran mavjudligi.
	own := func(w http.ResponseWriter, r *http.Request) (string, bool) {
		restaurantID, ok := entityIDFor(claimsFrom(r), r.PathValue("id"))
		if !ok {
			httpError(w, http.StatusNotFound, errors.New("restoran topilmadi"))
			return "", false
		}
		if s.StaffSvc == nil {
			httpError(w, http.StatusServiceUnavailable, errors.New("xodimlar bo'limi sozlanmagan"))
			return "", false
		}
		if _, err := s.CatalogRepo.GetRestaurant(r.Context(), restaurantID); err != nil {
			httpError(w, http.StatusNotFound, errors.New("restoran topilmadi"))
			return "", false
		}
		return restaurantID, true
	}

	audit := func(r *http.Request, action, restaurantID string, m *staff.Member) {
		c := claimsFrom(r)
		// Ism va telefon LOGGA YOZILMAYDI (shaxsiy ma'lumot) — faqat ID'lar.
		slog.Info("xodimlar: "+action, "restaurant", restaurantID, "staff", m.ID,
			"position", m.Position, "status", m.Status, "by", c.Subject, "role", c.Role)
	}

	// GET /restaurants/{id}/staff — ro'yxat, kartalar, taqsimot, so'nggi faoliyat.
	mux.HandleFunc("GET /restaurants/{id}/staff", s.auth(roles,
		func(w http.ResponseWriter, r *http.Request) {
			restaurantID, ok := own(w, r)
			if !ok {
				return
			}
			ov, err := s.StaffSvc.Overview(r.Context(), restaurantID)
			if err != nil {
				staffHTTPError(w, err)
				return
			}
			byID := make(map[string]*staff.Member, len(ov.Members))
			items := make([]staffMemberView, 0, len(ov.Members))
			for _, m := range ov.Members {
				byID[m.ID] = m
				items = append(items, staffView(m))
			}
			statuses := []map[string]string{}
			for _, st := range []staff.Status{staff.StatusActive, staff.StatusOnLeave, staff.StatusDismissed} {
				statuses = append(statuses, map[string]string{"key": string(st), "title": st.Title()})
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"items":           items,
				"summary":         ov.Summary,
				"recent_activity": staffEventViews(ov.Recent, byID),
				"positions":       staff.Positions(),
				"statuses":        statuses,
				"timezone":        "Asia/Tashkent",
				"max_members":     staff.MaxMembersPerRestaurant,
			})
		}))

	// POST /restaurants/{id}/staff
	//
	//	{"first_name":"Azizbek","last_name":"Karimov","phone":"+998901234567",
	//	 "position":"waiter","schedule":{"days":[1,2,3,4,5],"start":"08:00","end":"22:00"},
	//	 "hired_on":"2026-09-01","note":"","app_access":true}
	mux.HandleFunc("POST /restaurants/{id}/staff", s.auth(roles,
		func(w http.ResponseWriter, r *http.Request) {
			restaurantID, ok := own(w, r)
			if !ok {
				return
			}
			var req struct {
				FirstName string          `json:"first_name"`
				LastName  string          `json:"last_name"`
				Phone     string          `json:"phone"`
				Position  string          `json:"position"`
				Schedule  *staff.Schedule `json:"schedule"`
				HiredOn   string          `json:"hired_on"`
				Note      string          `json:"note"`
				AppAccess bool            `json:"app_access"`
				Salary    *int64          `json:"monthly_salary_tiyin"`
			}
			dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, staffBodyLimit))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, errors.New("so'rov noto'g'ri shaklda"))
				return
			}
			m, err := s.StaffSvc.Create(r.Context(), restaurantID, claimsFrom(r).Subject, staff.Input{
				FirstName: req.FirstName, LastName: req.LastName, Phone: req.Phone, Position: req.Position,
				Schedule: req.Schedule, HiredOn: req.HiredOn, Note: req.Note, AppAccess: req.AppAccess,
				SalaryTiyin: req.Salary,
			})
			if err != nil {
				staffHTTPError(w, err)
				return
			}
			audit(r, "yangi xodim", restaurantID, m)
			writeJSON(w, http.StatusCreated, staffView(m))
		}))

	// PATCH /restaurants/{id}/staff/{staffID} — faqat yuborilgan maydonlar.
	// Holat bu yerda EMAS (`/status`): ishdan bo'shatish alohida, ongli amal.
	mux.HandleFunc("PATCH /restaurants/{id}/staff/{staffID}", s.auth(roles,
		func(w http.ResponseWriter, r *http.Request) {
			restaurantID, ok := own(w, r)
			if !ok {
				return
			}
			var raw map[string]json.RawMessage
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, staffBodyLimit)).Decode(&raw); err != nil || raw == nil {
				httpError(w, http.StatusBadRequest, errors.New("so'rov JSON obyekt bo'lishi kerak"))
				return
			}
			if len(raw) == 0 {
				httpError(w, http.StatusBadRequest, errors.New("o'zgartiriladigan maydon yo'q"))
				return
			}
			var p staff.Patch
			str := func(v json.RawMessage) (*string, error) {
				var x string
				if err := decodeStrict(v, &x); err != nil {
					return nil, err
				}
				return &x, nil
			}
			for key, v := range raw {
				var err error
				switch key {
				case "first_name":
					p.FirstName, err = str(v)
				case "last_name":
					p.LastName, err = str(v)
				case "phone":
					p.Phone, err = str(v)
				case "position":
					p.Position, err = str(v)
				case "note":
					p.Note, err = str(v)
				case "hired_on":
					if string(v) == "null" {
						empty := ""
						p.HiredOn = &empty
					} else {
						p.HiredOn, err = str(v)
					}
				case "schedule":
					if string(v) == "null" {
						p.ClearSchedule = true
					} else {
						var sch staff.Schedule
						err = decodeStrict(v, &sch)
						p.Schedule = &sch
					}
				case "app_access":
					var b bool
					err = decodeStrict(v, &b)
					p.AppAccess = &b
				case "monthly_salary_tiyin":
					if string(v) == "null" {
						p.ClearSalary = true
					} else {
						var n int64
						err = decodeStrict(v, &n)
						p.SalaryTiyin = &n
					}
				default:
					httpError(w, http.StatusBadRequest, fmt.Errorf("%q maydonini bu yerda o'zgartirib bo'lmaydi", key))
					return
				}
				if err != nil {
					httpError(w, http.StatusBadRequest, fmt.Errorf("%q maydoni noto'g'ri shaklda", key))
					return
				}
			}
			m, err := s.StaffSvc.Update(r.Context(), restaurantID, r.PathValue("staffID"), claimsFrom(r).Subject, p)
			if err != nil {
				staffHTTPError(w, err)
				return
			}
			audit(r, "tahrirlandi", restaurantID, m)
			writeJSON(w, http.StatusOK, staffView(m))
		}))

	// POST /restaurants/{id}/staff/{staffID}/status  {"status":"on_leave"}
	mux.HandleFunc("POST /restaurants/{id}/staff/{staffID}/status", s.auth(roles,
		func(w http.ResponseWriter, r *http.Request) {
			restaurantID, ok := own(w, r)
			if !ok {
				return
			}
			var req struct {
				Status string `json:"status"`
			}
			dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, staffBodyLimit))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, errors.New("so'rov noto'g'ri shaklda"))
				return
			}
			m, err := s.StaffSvc.SetStatus(r.Context(), restaurantID, r.PathValue("staffID"), claimsFrom(r).Subject, req.Status)
			if err != nil {
				staffHTTPError(w, err)
				return
			}
			audit(r, "holat o'zgardi", restaurantID, m)
			writeJSON(w, http.StatusOK, staffView(m))
		}))

	// GET /restaurants/{id}/staff/report?month=2026-09 — oylik hisobot:
	// har bir xodimning ish kunlari, soatlari, ta'tili va hisoblangan
	// maoshi (ish jadvali va holatlar tarixi bo'yicha — `staff.BuildReport`).
	mux.HandleFunc("GET /restaurants/{id}/staff/report", s.auth(roles,
		func(w http.ResponseWriter, r *http.Request) {
			restaurantID, ok := own(w, r)
			if !ok {
				return
			}
			year, month, err := staff.ParseMonth(r.URL.Query().Get("month"), time.Now())
			if err != nil {
				staffHTTPError(w, err)
				return
			}
			rep, err := s.StaffSvc.MonthlyReport(r.Context(), restaurantID, year, month)
			if err != nil {
				staffHTTPError(w, err)
				return
			}
			items := make([]map[string]any, 0, len(rep.Rows))
			for _, row := range rep.Rows {
				m := row.Member
				items = append(items, map[string]any{
					"id":                   m.ID,
					"code":                 m.Code(),
					"full_name":            m.FullName(),
					"position":             m.Position,
					"position_title":       m.Position.Title(),
					"status":               m.Status,
					"schedule":             m.Schedule,
					"monthly_salary_tiyin": m.SalaryTiyin,
					"accrued_salary_tiyin": row.AccruedTiyin,
					"norm_days":            row.NormDays,
					"payable_days":         row.PayableDays,
					"worked_days":          row.WorkedDays,
					"leave_days":           row.LeaveDays,
					"off_days":             row.OffDays,
					"worked_minutes":       row.WorkedMinutes,
					"planned_minutes":      row.PlannedMinutes,
					"scheduled":            row.Scheduled,
					"days":                 row.Days,
				})
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"month":         fmt.Sprintf("%04d-%02d", rep.Year, int(rep.Month)),
				"days_in_month": rep.DaysInMonth,
				"today":         rep.Today.Format("2006-01-02"),
				"timezone":      "Asia/Tashkent",
				"items":         items,
				"totals":        rep.Totals,
			})
		}))

	// GET /restaurants/{id}/staff/{staffID}/activity — bitta xodimning jurnali.
	mux.HandleFunc("GET /restaurants/{id}/staff/{staffID}/activity", s.auth(roles,
		func(w http.ResponseWriter, r *http.Request) {
			restaurantID, ok := own(w, r)
			if !ok {
				return
			}
			id := r.PathValue("staffID")
			events, err := s.StaffSvc.Activity(r.Context(), restaurantID, id, 50)
			if err != nil {
				staffHTTPError(w, err)
				return
			}
			m, err := s.StaffSvc.Get(r.Context(), restaurantID, id)
			if err != nil {
				staffHTTPError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"items": staffEventViews(events, map[string]*staff.Member{m.ID: m}),
			})
		}))
}

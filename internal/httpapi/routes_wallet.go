package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"chustapp/internal/wallet"
)

// OnDex Wallet — mijozning platforma ichidagi hisobi. Reja va qoidalar:
// F:\ChustApp\ondexwallet.md.
func (s *Server) registerWalletRoutes(mux *http.ServeMux) {
	// GET /wallet — balans va so'nggi harakatlar tarixi.
	mux.HandleFunc("GET /wallet", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			if s.WalletSvc == nil {
				httpError(w, http.StatusServiceUnavailable,
					errors.New("OnDex Wallet sozlanmagan"))
				return
			}
			userID := claimsFrom(r).Subject
			balance, err := s.WalletSvc.Balance(r.Context(), userID)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			limit := 50
			if v := r.URL.Query().Get("limit"); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
					limit = n
				}
			}
			list, err := s.WalletSvc.ListTransactions(r.Context(), userID, limit)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			out := make([]map[string]any, 0, len(list))
			for _, t := range list {
				out = append(out, map[string]any{
					"id":           t.ID,
					"order_id":     t.OrderID,
					"amount_tiyin": t.AmountTiyin,
					"type":         t.Type,
					"created_at":   t.CreatedAt,
				})
			}
			// can_spend_min_tiyin — checkout klienti wallet'ni to'lov
			// usuli sifatida QACHON ko'rsatishni shundan biladi: balans
			// shu chegaradan HAM, buyurtma summasidan HAM katta-teng
			// bo'lishi kerak (`wallet.Service.CanSpend`).
			writeJSON(w, http.StatusOK, map[string]any{
				"balance_tiyin":       balance,
				"can_spend_min_tiyin": wallet.SpendMinBalanceTiyin,
				"transactions":        out,
			})
		}))
}

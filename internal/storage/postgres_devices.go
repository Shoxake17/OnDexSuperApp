package storage

import (
	"context"

	"chustapp/internal/users"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PgDeviceStore — `user_devices` jadvali (migration 0034).
type PgDeviceStore struct{ pool *pgxpool.Pool }

func NewPgDeviceStore(pool *pgxpool.Pool) *PgDeviceStore { return &PgDeviceStore{pool: pool} }

// Touch — "shu foydalanuvchi shu platformadan hozir kirdi".
//
// `ON CONFLICT` bilan bitta so'rovda: yozuv bo'lmasa yaratiladi,
// bo'lsa `last_seen` va versiya yangilanadi. `first_seen` esa
// TEGILMAYDI — "birinchi marta qachon ko'rilgan" ma'lumoti
// yo'qolmasligi kerak.
func (s *PgDeviceStore) Touch(ctx context.Context, userID, platform, appVersion string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO user_devices (user_id, platform, app_version)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (user_id, platform) DO UPDATE
		   SET last_seen = now(),
		       -- Bo'sh versiya mavjud qiymatni O'CHIRIB yubormaydi:
		       -- eski build versiyani umuman yubormasligi mumkin.
		       app_version = CASE WHEN EXCLUDED.app_version = ''
		                         THEN user_devices.app_version
		                         ELSE EXCLUDED.app_version END`,
		userID, platform, appVersion)
	return err
}

func (s *PgDeviceStore) ListByUsers(ctx context.Context, userIDs []string) (map[string][]users.Device, error) {
	out := make(map[string][]users.Device, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	// `= ANY($1)` — ro'yxat BITTA parametr sifatida uzatiladi, ya'ni
	// SQL matni foydalanuvchi sonidan qat'i nazar o'zgarmaydi
	// (so'rovlar rejasi keshlanadi va SQL yig'ish xatosi ehtimoli yo'q).
	rows, err := s.pool.Query(ctx,
		`SELECT user_id, platform, app_version, first_seen, last_seen
		   FROM user_devices
		  WHERE user_id = ANY($1)
		  ORDER BY last_seen DESC`, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var uid string
		var d users.Device
		if err := rows.Scan(&uid, &d.Platform, &d.AppVersion, &d.FirstSeen, &d.LastSeen); err != nil {
			return nil, err
		}
		out[uid] = append(out[uid], d)
	}
	return out, rows.Err()
}

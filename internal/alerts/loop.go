package alerts

import (
	"context"
	"log/slog"
	"time"

	"chustapp/internal/safego"
)

// safegoLoop — vazifani darhol, keyin har `every` da bajaradi. Panika
// butun serverni yiqitmaydi (`safego`), bitta urinish ko'pi bilan
// `every` davom etadi.
func safegoLoop(ctx context.Context, name string, every time.Duration, fn func(ctx context.Context) error) {
	safego.Go(name, func() {
		tick := func() {
			runCtx, cancel := context.WithTimeout(ctx, every)
			defer cancel()
			if err := fn(runCtx); err != nil && ctx.Err() == nil {
				slog.Warn("fon vazifasi xatosi", "job", name, "err", err)
			}
		}
		tick()
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				tick()
			}
		}
	})
}

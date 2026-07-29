package billing

import (
	"context"
	"log/slog"
	"time"
)

type Worker struct {
	svc    *Service
	logger *slog.Logger
}

func NewWorker(svc *Service, logger *slog.Logger) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{svc: svc, logger: logger}
}

func (w *Worker) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		// Initial delay so migrations can finish after deploy.
		select {
		case <-ctx.Done():
			return
		case <-time.After(15 * time.Second):
		}
		w.tick(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.tick(ctx)
			}
		}
	}()
}

func (w *Worker) tick(ctx context.Context) {
	if err := w.svc.RunDue(ctx); err != nil {
		w.logger.Warn("billing check", "error", err)
	}
}

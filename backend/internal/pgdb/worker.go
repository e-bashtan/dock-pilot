package pgdb

import (
	"context"
	"log/slog"
	"time"
)

type Worker struct {
	svc    *Service
	logger *slog.Logger
	tick   time.Duration
}

func NewWorker(svc *Service, logger *slog.Logger) *Worker {
	return &Worker{
		svc:    svc,
		logger: logger,
		tick:   time.Minute,
	}
}

func (w *Worker) Start(ctx context.Context) {
	go w.loop(ctx)
}

func (w *Worker) loop(ctx context.Context) {
	ticker := time.NewTicker(w.tick)
	defer ticker.Stop()

	w.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *Worker) runOnce(ctx context.Context) {
	if err := w.svc.RunDueSchedules(ctx); err != nil {
		w.logger.Warn("pg backup schedule check failed", "error", err)
	}
}

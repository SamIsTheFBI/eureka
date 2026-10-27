package audit

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"eureka/services/gateway/internal/db"
)

// Record represents a single audited tool execution event.
type Record struct {
	TS          time.Time
	UserID      int
	UserEmail   string
	KeyID       *int
	Channel     string
	ServerSlug  string
	Tool        string
	ExposedName string
	Outcome     string
	ErrorCode   string
	DurationMs  int64
	RequestID   string
	ArgsHash    string
	ArgsBytes   int
}

// Writer asynchronously batches and writes audit records to Postgres.
type Writer struct {
	db          *db.DB
	queue       chan Record
	dropped     atomic.Uint64
	logger      *slog.Logger
	stopOnce    sync.Once
	stopChan    chan struct{}
	drainDone   chan struct{}
}

// NewWriter creates and starts a buffered audit writer.
func NewWriter(db *db.DB, logger *slog.Logger) *Writer {
	w := &Writer{
		db:        db,
		queue:     make(chan Record, 2000),
		logger:    logger,
		stopChan:  make(chan struct{}),
		drainDone: make(chan struct{}),
	}

	go w.worker()
	return w
}

// Log queues an audit record. If queue is full, waits up to 200ms then drops.
func (w *Writer) Log(rec Record) {
	if rec.TS.IsZero() {
		rec.TS = time.Now()
	}

	select {
	case w.queue <- rec:
	case <-time.After(200 * time.Millisecond):
		w.dropped.Add(1)
		w.logger.Warn("audit queue full, dropped record", "server", rec.ServerSlug, "tool", rec.Tool)
	}
}

// Close gracefully flushes pending records and shuts down.
func (w *Writer) Close() {
	w.stopOnce.Do(func() {
		close(w.stopChan)
		<-w.drainDone
	})
}

func (w *Writer) worker() {
	defer close(w.drainDone)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var batch []Record

	flush := func() {
		if len(batch) == 0 {
			return
		}
		w.flushBatch(batch)
		batch = nil
	}

	for {
		select {
		case <-w.stopChan:
			// Drain remaining records
			for {
				select {
				case rec := <-w.queue:
					batch = append(batch, rec)
				default:
					flush()
					return
				}
			}

		case rec := <-w.queue:
			batch = append(batch, rec)
			if len(batch) >= 100 {
				flush()
			}

		case <-ticker.C:
			flush()
		}
	}
}

func (w *Writer) flushBatch(batch []Record) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := w.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		w.logger.Error("failed to begin audit tx", "error", err)
		return
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO audit_log (
			ts, user_id, user_email, key_id, channel, server_slug, tool, exposed_name,
			outcome, error_code, duration_ms, request_id, args_hash, args_bytes
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`)
	if err != nil {
		w.logger.Error("failed to prepare audit statement", "error", err)
		return
	}
	defer stmt.Close()

	for _, r := range batch {
		var uid sql.NullInt64
		if r.UserID > 0 {
			uid = sql.NullInt64{Int64: int64(r.UserID), Valid: true}
		}
		var kid sql.NullInt64
		if r.KeyID != nil {
			kid = sql.NullInt64{Int64: int64(*r.KeyID), Valid: true}
		}

		_, err := stmt.ExecContext(ctx,
			r.TS, uid, r.UserEmail, kid, r.Channel, r.ServerSlug, r.Tool, r.ExposedName,
			r.Outcome, r.ErrorCode, r.DurationMs, r.RequestID, r.ArgsHash, r.ArgsBytes,
		)
		if err != nil {
			w.logger.Error("failed to execute audit insert", "error", err)
		}
	}

	if err := tx.Commit(); err != nil {
		w.logger.Error("failed to commit audit batch", "error", err)
	}
}

package audit

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

// Entry represents a single audit log record.
type Entry struct {
	Timestamp   time.Time     `json:"timestamp"`
	RequestID   string        `json:"request_id"`
	Method      string        `json:"method"`
	Path        string        `json:"path"`
	ClientIP    string        `json:"client_ip"`
	StatusCode  int           `json:"status_code"`
	Duration    time.Duration `json:"duration_ns"`
	BytesSent   int           `json:"bytes_sent"`
	UserAgent   string        `json:"user_agent"`
	CountryCode string        `json:"country_code,omitempty"`
	Region      string        `json:"region,omitempty"`
	ProviderID  string        `json:"provider_id,omitempty"`
	License     string        `json:"license,omitempty"`
	Blocked     bool          `json:"blocked"`
	BlockReason string        `json:"block_reason,omitempty"`
}

const bufferSize = 10000

// Logger provides async audit logging.
type Logger struct {
	ch     chan Entry
	wg     sync.WaitGroup
	logger *slog.Logger
}

// NewLogger creates and starts an audit Logger.
func NewLogger(outputType, filePath string, logger *slog.Logger) (*Logger, error) {
	var out *os.File
	switch outputType {
	case "file":
		f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return nil, fmt.Errorf("opening audit log file: %w", err)
		}
		out = f
	case "kafka":
		// Kafka output is a placeholder; fall back to stdout
		out = os.Stdout
	default:
		out = os.Stdout
	}

	al := &Logger{
		ch:     make(chan Entry, bufferSize),
		logger: logger,
	}

	al.wg.Add(1)
	go al.consume(out)
	return al, nil
}

// Log submits an audit entry for async processing. Drops the entry if the buffer is full.
func (al *Logger) Log(e Entry) {
	select {
	case al.ch <- e:
	default:
		al.logger.Warn("audit buffer full, dropping entry", "request_id", e.RequestID)
	}
}

// Flush closes the channel and waits for all entries to be processed.
func (al *Logger) Flush() {
	close(al.ch)
	al.wg.Wait()
}

func (al *Logger) consume(out *os.File) {
	defer al.wg.Done()
	enc := json.NewEncoder(out)
	for e := range al.ch {
		if err := enc.Encode(e); err != nil {
			al.logger.Error("failed to write audit entry", "error", err)
		}
	}
}

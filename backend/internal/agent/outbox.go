package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const (
	DefaultOutboxDir     = "/var/lib/barn-agent/outbox"
	DefaultOutboxMaxBytes = 8 << 20 // 8 MiB
	DefaultOutboxMaxItems = 5000
)

// Outbox is a durable JSONL/file event queue that survives agent restarts.
type Outbox struct {
	Dir      string
	MaxBytes int64
	MaxItems int

	mu sync.Mutex
}

func NewOutbox(dir string) *Outbox {
	return &Outbox{
		Dir:      dir,
		MaxBytes: DefaultOutboxMaxBytes,
		MaxItems: DefaultOutboxMaxItems,
	}
}

func (o *Outbox) Ensure() error {
	return os.MkdirAll(o.Dir, 0o750)
}

// Enqueue appends a JSON-serializable event as an individual file.
func (o *Outbox) Enqueue(v any) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if err := o.Ensure(); err != nil {
		return err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal outbox item: %w", err)
	}
	b = append(b, '\n')

	seq, err := o.nextSeq()
	if err != nil {
		return err
	}
	name := fmt.Sprintf("%016d.json", seq)
	path := filepath.Join(o.Dir, name)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("write outbox item: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("commit outbox item: %w", err)
	}
	return o.trimLocked()
}

// List returns pending items in order (oldest first), up to limit.
func (o *Outbox) List(limit int) ([]OutboxItem, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	files, err := o.listFiles()
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > len(files) {
		limit = len(files)
	}
	out := make([]OutboxItem, 0, limit)
	for _, name := range files[:limit] {
		path := filepath.Join(o.Dir, name)
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		out = append(out, OutboxItem{ID: name, Path: path, Payload: b})
	}
	return out, nil
}

// Ack removes delivered items by ID (filename).
func (o *Outbox) Ack(ids ...string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, id := range ids {
		id = filepath.Base(id)
		if id == "." || id == ".." || !strings.HasSuffix(id, ".json") {
			continue
		}
		_ = os.Remove(filepath.Join(o.Dir, id))
	}
	return nil
}

// Len returns the number of pending items.
func (o *Outbox) Len() (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	files, err := o.listFiles()
	if err != nil {
		return 0, err
	}
	return len(files), nil
}

type OutboxItem struct {
	ID      string
	Path    string
	Payload []byte
}

func (o *Outbox) nextSeq() (int64, error) {
	files, err := o.listFiles()
	if err != nil {
		return 0, err
	}
	var max int64
	for _, name := range files {
		n, err := strconv.ParseInt(strings.TrimSuffix(name, ".json"), 10, 64)
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return max + 1, nil
}

func (o *Outbox) listFiles() ([]string, error) {
	entries, err := os.ReadDir(o.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".tmp") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (o *Outbox) trimLocked() error {
	files, err := o.listFiles()
	if err != nil {
		return err
	}
	for len(files) > o.MaxItems {
		_ = os.Remove(filepath.Join(o.Dir, files[0]))
		files = files[1:]
	}
	var total int64
	sizes := make([]int64, len(files))
	for i, name := range files {
		st, err := os.Stat(filepath.Join(o.Dir, name))
		if err != nil {
			continue
		}
		sizes[i] = st.Size()
		total += st.Size()
	}
	for total > o.MaxBytes && len(files) > 0 {
		_ = os.Remove(filepath.Join(o.Dir, files[0]))
		total -= sizes[0]
		files = files[1:]
		sizes = sizes[1:]
	}
	return nil
}

// DecodeJSONL is a helper to parse a JSONL blob into values (tests / migration).
func DecodeJSONL(path string, dest any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if err := json.Unmarshal([]byte(line), dest); err != nil {
			return err
		}
	}
	return sc.Err()
}

package history

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"ai-harness/internal/classification"
)

type Record struct {
	ID             string                   `json:"id"`
	Timestamp      time.Time                `json:"timestamp"`
	Command        string                   `json:"command"`
	Task           string                   `json:"task,omitempty"`
	Provider       string                   `json:"provider,omitempty"`
	Model          string                   `json:"model,omitempty"`
	Classification *classification.Decision `json:"classification,omitempty"`
	FilesTouched   []string                 `json:"filesTouched"`
	TestsRun       []TestRun                `json:"testsRun"`
	Escalated      bool                     `json:"escalated"`
	Success        bool                     `json:"success"`
	Status         string                   `json:"status"`
	Error          string                   `json:"error,omitempty"`
}

type TestRun struct {
	Command string `json:"command,omitempty"`
	Passed  bool   `json:"passed"`
	Skipped bool   `json:"skipped"`
	Output  string `json:"output,omitempty"`
}

type Store struct {
	Dir string
	Now func() time.Time
}

const defaultStatus = "completed"

var safeIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func NewStore(dir string) Store {
	return Store{Dir: dir}
}

func (s Store) Save(record Record) (Record, error) {
	dir := strings.TrimSpace(s.Dir)
	if dir == "" {
		return Record{}, errors.New("history directory is required")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Record{}, fmt.Errorf("create history directory: %w", err)
	}

	record, err := Prepare(record, s.now())
	if err != nil {
		return Record{}, err
	}

	contents, err := Marshal(record)
	if err != nil {
		return Record{}, err
	}

	path := s.recordPath(record.ID)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return Record{}, fmt.Errorf("create history record: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(contents); err != nil {
		return Record{}, fmt.Errorf("write history record: %w", err)
	}

	return record, nil
}

func (s Store) List() ([]Record, error) {
	dir := strings.TrimSpace(s.Dir)
	if dir == "" {
		return nil, errors.New("history directory is required")
	}
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("inspect history directory: %w", err)
	}

	matches, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("list history records: %w", err)
	}

	records := make([]Record, 0, len(matches))
	for _, path := range matches {
		record, err := readRecord(path)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Timestamp.Equal(records[j].Timestamp) {
			return records[i].ID > records[j].ID
		}
		return records[i].Timestamp.After(records[j].Timestamp)
	})

	return records, nil
}

func (s Store) Load(id string) (Record, error) {
	if err := ValidateID(id); err != nil {
		return Record{}, err
	}
	record, err := readRecord(s.recordPath(id))
	if err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s Store) recordPath(id string) string {
	return filepath.Join(s.Dir, id+".json")
}

func (s Store) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func readRecord(path string) (Record, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return Record{}, fmt.Errorf("read history record: %w", err)
	}

	record, err := Unmarshal(contents, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	if err != nil {
		return Record{}, fmt.Errorf("decode history record %s: %w", filepath.Base(path), err)
	}
	return record, nil
}

func Prepare(record Record, now time.Time) (Record, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if record.Timestamp.IsZero() {
		record.Timestamp = now
	}
	record.Timestamp = record.Timestamp.UTC()
	if record.ID == "" {
		id, err := newID(record.Timestamp)
		if err != nil {
			return Record{}, err
		}
		record.ID = id
	}
	if err := ValidateID(record.ID); err != nil {
		return Record{}, err
	}
	if strings.TrimSpace(record.Status) == "" {
		record.Status = defaultStatus
	}
	record.FilesTouched = uniqueSorted(record.FilesTouched)
	record.TestsRun = normalizeTests(record.TestsRun)
	return record, nil
}

func Marshal(record Record) ([]byte, error) {
	contents, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode history record: %w", err)
	}
	return append(contents, '\n'), nil
}

func Unmarshal(contents []byte, fallbackID string) (Record, error) {
	var record Record
	if err := json.Unmarshal(contents, &record); err != nil {
		return Record{}, err
	}
	if record.ID == "" {
		record.ID = fallbackID
	}
	if err := ValidateID(record.ID); err != nil {
		return Record{}, err
	}
	return record, nil
}

func newID(timestamp time.Time) (string, error) {
	var suffix [4]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", fmt.Errorf("generate history id: %w", err)
	}
	return timestamp.UTC().Format("20060102T150405.000000000Z") + "-" + hex.EncodeToString(suffix[:]), nil
}

func ValidateID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("history id is required")
	}
	if strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) || !safeIDPattern.MatchString(id) {
		return fmt.Errorf("invalid history id %q", id)
	}
	return nil
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func normalizeTests(values []TestRun) []TestRun {
	out := make([]TestRun, 0, len(values))
	for _, value := range values {
		value.Command = strings.TrimSpace(value.Command)
		value.Output = truncate(value.Output, 4096)
		out = append(out, value)
	}
	return out
}

func truncate(value string, maxBytes int) string {
	value = strings.TrimSpace(value)
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	return value[:maxBytes]
}

package security

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ai-harness/internal/history"
)

const encryptedHistoryExt = ".json.enc"

type EncryptedHistoryStore struct {
	Dir   string
	Key   []byte
	KeyID string
	Now   func() time.Time
}

type HistoryReport struct {
	Encrypted int      `json:"encrypted"`
	Plaintext int      `json:"plaintext"`
	Invalid   int      `json:"invalid"`
	Errors    []string `json:"errors,omitempty"`
}

func NewEncryptedHistoryStore(dir string, key []byte, keyID string) EncryptedHistoryStore {
	return EncryptedHistoryStore{Dir: dir, Key: key, KeyID: keyID}
}

func (s EncryptedHistoryStore) Save(record history.Record) (history.Record, error) {
	dir := strings.TrimSpace(s.Dir)
	if dir == "" {
		return history.Record{}, errors.New("history directory is required")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return history.Record{}, fmt.Errorf("create history directory: %w", err)
	}

	record, err := history.Prepare(record, s.now())
	if err != nil {
		return history.Record{}, err
	}
	if err := s.writeEncrypted(record, false); err != nil {
		return history.Record{}, err
	}
	return record, nil
}

func (s EncryptedHistoryStore) List() ([]history.Record, error) {
	records, err := s.listEncrypted()
	if err != nil {
		return nil, err
	}

	plaintext, err := history.NewStore(s.Dir).List()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := make([]history.Record, 0, len(records)+len(plaintext))
	for _, record := range records {
		seen[record.ID] = true
		out = append(out, record)
	}
	for _, record := range plaintext {
		if seen[record.ID] {
			continue
		}
		out = append(out, record)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Timestamp.Equal(out[j].Timestamp) {
			return out[i].ID > out[j].ID
		}
		return out[i].Timestamp.After(out[j].Timestamp)
	})
	return out, nil
}

func (s EncryptedHistoryStore) Load(id string) (history.Record, error) {
	if err := history.ValidateID(id); err != nil {
		return history.Record{}, err
	}
	record, err := s.readEncrypted(id)
	if err == nil {
		return record, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return history.Record{}, err
	}
	return history.NewStore(s.Dir).Load(id)
}

func (s EncryptedHistoryStore) MigratePlaintext() (HistoryReport, error) {
	report := HistoryReport{}
	paths, err := plaintextHistoryPaths(s.Dir)
	if err != nil {
		return report, err
	}
	for _, path := range paths {
		record, err := readPlaintextHistoryPath(path)
		if err != nil {
			report.Invalid++
			report.Errors = append(report.Errors, err.Error())
			continue
		}
		if err := s.writeEncrypted(record, true); err != nil {
			report.Invalid++
			report.Errors = append(report.Errors, err.Error())
			continue
		}
		loaded, err := s.readEncrypted(record.ID)
		if err != nil || loaded.ID != record.ID {
			report.Invalid++
			report.Errors = append(report.Errors, fmt.Sprintf("verify migrated record %s: %v", record.ID, err))
			continue
		}
		if err := os.Remove(path); err != nil {
			report.Invalid++
			report.Errors = append(report.Errors, fmt.Sprintf("remove plaintext record %s: %v", filepath.Base(path), err))
			continue
		}
		report.Encrypted++
	}
	verify, err := s.Verify()
	if err != nil {
		return report, err
	}
	report.Plaintext = verify.Plaintext
	report.Invalid += verify.Invalid
	report.Errors = append(report.Errors, verify.Errors...)
	return report, nil
}

func (s EncryptedHistoryStore) Verify() (HistoryReport, error) {
	report := HistoryReport{}
	encrypted, err := encryptedHistoryPaths(s.Dir)
	if err != nil {
		return report, err
	}
	for _, path := range encrypted {
		id := encryptedIDFromPath(path)
		if _, err := s.readEncrypted(id); err != nil {
			report.Invalid++
			report.Errors = append(report.Errors, err.Error())
			continue
		}
		report.Encrypted++
	}
	plaintext, err := plaintextHistoryPaths(s.Dir)
	if err != nil {
		return report, err
	}
	report.Plaintext = len(plaintext)
	return report, nil
}

func RotateHistory(dir string, oldKey []byte, oldKeyID string, newKey []byte, newKeyID string) (HistoryReport, error) {
	oldStore := NewEncryptedHistoryStore(dir, oldKey, oldKeyID)
	newStore := NewEncryptedHistoryStore(dir, newKey, newKeyID)
	report := HistoryReport{}

	records, err := oldStore.listEncrypted()
	if err != nil {
		return report, err
	}
	for _, record := range records {
		if err := newStore.writeEncrypted(record, true); err != nil {
			report.Invalid++
			report.Errors = append(report.Errors, err.Error())
			continue
		}
		report.Encrypted++
	}
	verify, err := newStore.Verify()
	if err != nil {
		return report, err
	}
	report.Plaintext = verify.Plaintext
	report.Invalid += verify.Invalid
	report.Errors = append(report.Errors, verify.Errors...)
	return report, nil
}

func (s EncryptedHistoryStore) listEncrypted() ([]history.Record, error) {
	paths, err := encryptedHistoryPaths(s.Dir)
	if err != nil {
		return nil, err
	}
	records := make([]history.Record, 0, len(paths))
	for _, path := range paths {
		record, err := s.readEncrypted(encryptedIDFromPath(path))
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

func (s EncryptedHistoryStore) writeEncrypted(record history.Record, overwrite bool) error {
	plaintext, err := history.Marshal(record)
	if err != nil {
		return err
	}
	contents, err := Encrypt(s.Key, s.KeyID, plaintext, historyAAD(record.ID))
	if err != nil {
		return err
	}

	path := encryptedHistoryPath(s.Dir, record.ID)
	flag := os.O_WRONLY | os.O_CREATE
	if overwrite {
		flag |= os.O_TRUNC
	} else {
		flag |= os.O_EXCL
	}
	f, err := os.OpenFile(path, flag, 0o600)
	if err != nil {
		return fmt.Errorf("create encrypted history record: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(contents); err != nil {
		return fmt.Errorf("write encrypted history record: %w", err)
	}
	return nil
}

func (s EncryptedHistoryStore) readEncrypted(id string) (history.Record, error) {
	if err := history.ValidateID(id); err != nil {
		return history.Record{}, err
	}
	path := encryptedHistoryPath(s.Dir, id)
	contents, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return history.Record{}, os.ErrNotExist
		}
		return history.Record{}, fmt.Errorf("read encrypted history record: %w", err)
	}
	plaintext, envelope, err := Decrypt(s.Key, contents, historyAAD(id))
	if err != nil {
		return history.Record{}, fmt.Errorf("decrypt encrypted history record %s: %w", id, err)
	}
	if envelope.KeyID != "" && s.KeyID != "" && envelope.KeyID != s.KeyID {
		return history.Record{}, fmt.Errorf("encrypted history record %s uses key %s, expected %s", id, envelope.KeyID, s.KeyID)
	}
	record, err := history.Unmarshal(plaintext, id)
	if err != nil {
		return history.Record{}, fmt.Errorf("decode encrypted history record %s: %w", id, err)
	}
	return record, nil
}

func (s EncryptedHistoryStore) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func historyAAD(id string) []byte {
	return []byte("ai-harness-history-v1:" + id)
}

func encryptedHistoryPath(dir string, id string) string {
	return filepath.Join(dir, id+encryptedHistoryExt)
}

func encryptedIDFromPath(path string) string {
	return strings.TrimSuffix(filepath.Base(path), encryptedHistoryExt)
}

func encryptedHistoryPaths(dir string) ([]string, error) {
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("inspect history directory: %w", err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*"+encryptedHistoryExt))
	if err != nil {
		return nil, fmt.Errorf("list encrypted history records: %w", err)
	}
	sort.Strings(matches)
	return matches, nil
}

func plaintextHistoryPaths(dir string) ([]string, error) {
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("inspect history directory: %w", err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("list plaintext history records: %w", err)
	}
	sort.Strings(matches)
	return matches, nil
}

func readPlaintextHistoryPath(path string) (history.Record, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return history.Record{}, fmt.Errorf("read plaintext history record: %w", err)
	}
	fallbackID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	record, err := history.Unmarshal(contents, fallbackID)
	if err != nil {
		return history.Record{}, fmt.Errorf("decode plaintext history record %s: %w", filepath.Base(path), err)
	}
	return record, nil
}

package security

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	appconfig "ai-harness/internal/config"
	"ai-harness/internal/history"
)

func TestEncryptDecrypt(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	encrypted, err := Encrypt(key, "key-1", []byte("secret"), []byte("aad"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	plaintext, envelope, err := Decrypt(key, encrypted, []byte("aad"))
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(plaintext) != "secret" || envelope.KeyID != "key-1" {
		t.Fatalf("plaintext=%q envelope=%+v", plaintext, envelope)
	}
	if _, _, err := Decrypt(key, encrypted, []byte("wrong")); err == nil {
		t.Fatal("decrypt succeeded with wrong AAD")
	}
}

func TestResolvePassphraseKey(t *testing.T) {
	cfg := appconfig.Default()
	cfg.Security.KeyProvider = KeyProviderPassphrase
	cfg.Security.KeyID = "key-1"
	cfg.Security.KDFSalt = "YWJjZGVmZ2hpamtsbW5vcA=="

	key, err := ResolveKey(cfg, "correct horse battery staple")
	if err != nil {
		t.Fatalf("resolve key: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("key length = %d", len(key))
	}
}

func TestEncryptedHistoryStoreSaveListAndLoad(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	store := EncryptedHistoryStore{
		Dir:   t.TempDir(),
		Key:   key,
		KeyID: "key-1",
		Now: func() time.Time {
			return time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
		},
	}

	record, err := store.Save(history.Record{Command: "run", Task: "secret task"})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.Dir, record.ID+".json.enc")); err != nil {
		t.Fatalf("encrypted record missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.Dir, record.ID+".json")); !os.IsNotExist(err) {
		t.Fatalf("plaintext record exists: %v", err)
	}
	records, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(records) != 1 || records[0].Task != "secret task" {
		t.Fatalf("records = %+v", records)
	}
	loaded, err := store.Load(record.ID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Command != "run" {
		t.Fatalf("loaded = %+v", loaded)
	}
}

func TestEncryptedHistoryMigratePlaintext(t *testing.T) {
	dir := t.TempDir()
	plain := history.NewStore(dir)
	record, err := plain.Save(history.Record{Command: "classify", Task: "plaintext"})
	if err != nil {
		t.Fatalf("save plaintext: %v", err)
	}

	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	store := NewEncryptedHistoryStore(dir, key, "key-1")
	report, err := store.MigratePlaintext()
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if report.Encrypted != 1 || report.Plaintext != 0 || report.Invalid != 0 {
		t.Fatalf("report = %+v", report)
	}
	if _, err := os.Stat(filepath.Join(dir, record.ID+".json")); !os.IsNotExist(err) {
		t.Fatalf("plaintext was not removed: %v", err)
	}
	loaded, err := store.Load(record.ID)
	if err != nil {
		t.Fatalf("load migrated: %v", err)
	}
	if loaded.Task != "plaintext" {
		t.Fatalf("loaded = %+v", loaded)
	}
}

func TestRotateHistory(t *testing.T) {
	dir := t.TempDir()
	oldKey, err := GenerateKey()
	if err != nil {
		t.Fatalf("generate old key: %v", err)
	}
	oldStore := NewEncryptedHistoryStore(dir, oldKey, "old")
	record, err := oldStore.Save(history.Record{Command: "run", Task: "rotate"})
	if err != nil {
		t.Fatalf("save old: %v", err)
	}
	newKey, err := GenerateKey()
	if err != nil {
		t.Fatalf("generate new key: %v", err)
	}
	report, err := RotateHistory(dir, oldKey, "old", newKey, "new")
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if report.Encrypted != 1 || report.Invalid != 0 {
		t.Fatalf("report = %+v", report)
	}
	if _, err := oldStore.Load(record.ID); err == nil {
		t.Fatal("old key still decrypts rotated record")
	}
	newStore := NewEncryptedHistoryStore(dir, newKey, "new")
	loaded, err := newStore.Load(record.ID)
	if err != nil {
		t.Fatalf("load new: %v", err)
	}
	if loaded.Task != "rotate" {
		t.Fatalf("loaded = %+v", loaded)
	}
}

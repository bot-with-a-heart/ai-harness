package security

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	appconfig "ai-harness/internal/config"

	"github.com/zalando/go-keyring"
	"golang.org/x/crypto/scrypt"
)

const (
	KeyProviderOSKeychain = "os-keychain"
	KeyProviderPassphrase = "passphrase"
	PassphraseEnv         = "AI_HARNESS_PASSPHRASE"
	keyringService        = "ai-harness"
)

var ErrKeyUnavailable = errors.New("security key unavailable")

type keyringStore interface {
	Set(service string, user string, password string) error
	Get(service string, user string) (string, error)
	Delete(service string, user string) error
}

type osKeyringStore struct{}

func (osKeyringStore) Set(service string, user string, password string) error {
	return keyring.Set(service, user, password)
}

func (osKeyringStore) Get(service string, user string) (string, error) {
	return keyring.Get(service, user)
}

func (osKeyringStore) Delete(service string, user string) error {
	return keyring.Delete(service, user)
}

var keyringBackend keyringStore = osKeyringStore{}

type InitOptions struct {
	Provider   string
	Passphrase string
	Required   bool
	Force      bool
}

func Init(cfg appconfig.Config, opts InitOptions) (appconfig.Config, error) {
	appconfig.ApplyDefaults(&cfg)
	if cfg.Security.KeyID != "" && !opts.Force {
		return cfg, fmt.Errorf("security is already initialized; use security rotate-key to replace the key")
	}

	provider := strings.TrimSpace(opts.Provider)
	if provider == "" {
		provider = cfg.Security.KeyProvider
	}
	if provider == "" {
		provider = appconfig.DefaultSecurityKeyProvider
	}

	key, err := GenerateKey()
	if err != nil {
		return cfg, err
	}
	keyID, err := NewKeyID()
	if err != nil {
		return cfg, err
	}

	cfg.Security.Enabled = true
	cfg.Security.Required = opts.Required
	cfg.Security.KeyProvider = provider
	cfg.Security.KeyID = keyID
	cfg.Security.EncryptHistory = true
	cfg.Security.EncryptMemory = true
	cfg.Security.EncryptLogs = true
	cfg.Security.RetainFullRepoContext = false
	cfg.Security.RecoveryExported = false

	switch provider {
	case KeyProviderOSKeychain:
		if err := StoreOSKey(keyID, key); err != nil {
			if strings.TrimSpace(opts.Passphrase) == "" {
				return cfg, fmt.Errorf("store key in OS keychain: %w; provide --provider passphrase --passphrase <value> to use the fallback", err)
			}
			provider = KeyProviderPassphrase
			cfg.Security.KeyProvider = provider
		} else {
			cfg.Security.KDFSalt = ""
			return cfg, nil
		}
		fallthrough
	case KeyProviderPassphrase:
		passphrase := strings.TrimSpace(opts.Passphrase)
		if passphrase == "" {
			return cfg, fmt.Errorf("--passphrase is required when using passphrase key provider")
		}
		salt, err := randomBytes(16)
		if err != nil {
			return cfg, err
		}
		cfg.Security.KDFSalt = base64.StdEncoding.EncodeToString(salt)
	default:
		return cfg, fmt.Errorf("unsupported key provider %q", provider)
	}

	return cfg, nil
}

func ResolveKey(cfg appconfig.Config, passphrase string) ([]byte, error) {
	appconfig.ApplyDefaults(&cfg)
	if !cfg.Security.Enabled {
		return nil, fmt.Errorf("%w: security is disabled", ErrKeyUnavailable)
	}
	if cfg.Security.KeyID == "" {
		return nil, fmt.Errorf("%w: run security init", ErrKeyUnavailable)
	}

	switch cfg.Security.KeyProvider {
	case KeyProviderOSKeychain:
		return LoadOSKey(cfg.Security.KeyID)
	case KeyProviderPassphrase:
		if strings.TrimSpace(passphrase) == "" {
			passphrase = os.Getenv(PassphraseEnv)
		}
		return DerivePassphraseKey(passphrase, cfg.Security.KDFSalt)
	default:
		return nil, fmt.Errorf("%w: unsupported key provider %q", ErrKeyUnavailable, cfg.Security.KeyProvider)
	}
}

func GenerateKey() ([]byte, error) {
	return randomBytes(KeySize)
}

func NewKeyID() (string, error) {
	bytes, err := randomBytes(12)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func StoreOSKey(keyID string, key []byte) error {
	if strings.TrimSpace(keyID) == "" {
		return errors.New("key id is required")
	}
	return keyringBackend.Set(keyringService, keyID, base64.StdEncoding.EncodeToString(key))
}

func LoadOSKey(keyID string) ([]byte, error) {
	encoded, err := keyringBackend.Get(keyringService, keyID)
	if err != nil {
		return nil, fmt.Errorf("%w: OS keychain lookup failed: %v", ErrKeyUnavailable, err)
	}
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("%w: OS keychain key is invalid: %v", ErrKeyUnavailable, err)
	}
	if len(key) != KeySize {
		return nil, fmt.Errorf("%w: OS keychain key has invalid length", ErrKeyUnavailable)
	}
	return key, nil
}

func DeleteOSKey(keyID string) error {
	if strings.TrimSpace(keyID) == "" {
		return nil
	}
	return keyringBackend.Delete(keyringService, keyID)
}

func DerivePassphraseKey(passphrase string, saltBase64 string) ([]byte, error) {
	passphrase = strings.TrimSpace(passphrase)
	if passphrase == "" {
		return nil, fmt.Errorf("%w: passphrase is required; pass --passphrase or set %s", ErrKeyUnavailable, PassphraseEnv)
	}
	salt, err := base64.StdEncoding.DecodeString(saltBase64)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid passphrase salt: %v", ErrKeyUnavailable, err)
	}
	key, err := scrypt.Key([]byte(passphrase), salt, 32768, 8, 1, KeySize)
	if err != nil {
		return nil, fmt.Errorf("derive passphrase key: %w", err)
	}
	return key, nil
}

func randomBytes(size int) ([]byte, error) {
	out := make([]byte, size)
	if _, err := rand.Read(out); err != nil {
		return nil, fmt.Errorf("generate random bytes: %w", err)
	}
	return out, nil
}

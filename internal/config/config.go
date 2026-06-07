package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-playground/validator/v10"
	tomlparser "github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/pelletier/go-toml/v2"
)

const (
	configDirName = ".ai-harness"
	configName    = "config.toml"
	historyName   = "history"
)

type Config struct {
	DefaultMode string                         `koanf:"default_mode" mapstructure:"default_mode" toml:"default_mode" validate:"required,oneof=auto local codex"`
	Providers   map[string]map[string]Provider `koanf:"providers" mapstructure:"providers" toml:"providers" validate:"required"`
	Routing     Routing                        `koanf:"routing" mapstructure:"routing" toml:"routing" validate:"required"`
	Memory      Memory                         `koanf:"memory" mapstructure:"memory" toml:"memory"`
	Security    Security                       `koanf:"security" mapstructure:"security" toml:"security"`
}

type Provider struct {
	Type    string   `koanf:"type" mapstructure:"type" toml:"type" validate:"required"`
	BaseURL string   `koanf:"base_url" mapstructure:"base_url" toml:"base_url,omitempty" validate:"omitempty,url"`
	APIKey  string   `koanf:"api_key" mapstructure:"api_key" toml:"api_key,omitempty"`
	Models  []string `koanf:"models" mapstructure:"models" toml:"models,omitempty"`
	Profile string   `koanf:"profile" mapstructure:"profile" toml:"profile,omitempty"`
}

type Routing struct {
	LocalFirst        bool `koanf:"local_first" mapstructure:"local_first" toml:"local_first"`
	EscalateOnFailure bool `koanf:"escalate_on_failure" mapstructure:"escalate_on_failure" toml:"escalate_on_failure"`
}

type Memory struct {
	Obsidian Obsidian `koanf:"obsidian" mapstructure:"obsidian" toml:"obsidian"`
}

type Obsidian struct {
	Enabled            bool   `koanf:"enabled" mapstructure:"enabled" toml:"enabled"`
	VaultPath          string `koanf:"vault_path" mapstructure:"vault_path" toml:"vault_path,omitempty"`
	Folder             string `koanf:"folder" mapstructure:"folder" toml:"folder,omitempty"`
	ExportHistoryLimit int    `koanf:"export_history_limit" mapstructure:"export_history_limit" toml:"export_history_limit,omitempty"`
}

type Security struct {
	Enabled               bool   `koanf:"enabled" mapstructure:"enabled" toml:"enabled"`
	Required              bool   `koanf:"required" mapstructure:"required" toml:"required"`
	KeyProvider           string `koanf:"key_provider" mapstructure:"key_provider" toml:"key_provider,omitempty"`
	KeyID                 string `koanf:"key_id" mapstructure:"key_id" toml:"key_id,omitempty"`
	KDFSalt               string `koanf:"kdf_salt" mapstructure:"kdf_salt" toml:"kdf_salt,omitempty"`
	EncryptHistory        bool   `koanf:"encrypt_history" mapstructure:"encrypt_history" toml:"encrypt_history"`
	EncryptMemory         bool   `koanf:"encrypt_memory" mapstructure:"encrypt_memory" toml:"encrypt_memory"`
	EncryptLogs           bool   `koanf:"encrypt_logs" mapstructure:"encrypt_logs" toml:"encrypt_logs"`
	RetainFullRepoContext bool   `koanf:"retain_full_repo_context" mapstructure:"retain_full_repo_context" toml:"retain_full_repo_context"`
	RecoveryExported      bool   `koanf:"recovery_exported" mapstructure:"recovery_exported" toml:"recovery_exported"`
}

type InitOptions struct {
	Force bool
}

const (
	DefaultObsidianFolder       = "AI Harness"
	DefaultObsidianHistoryLimit = 20
	DefaultSecurityKeyProvider  = "os-keychain"
)

func Default() Config {
	return Config{
		DefaultMode: "auto",
		Providers: map[string]map[string]Provider{
			"lmstudio": {
				"desktop": {
					Type:    "openai-compatible",
					BaseURL: "http://127.0.0.1:1234/v1",
					APIKey:  "lm-studio",
					Models: []string{
						"qwen2.5-coder-14b",
						"deepseek-coder",
					},
				},
			},
			"codex": {
				"default": {
					Type:    "codex-cli",
					Profile: "default",
				},
			},
		},
		Routing: Routing{
			LocalFirst:        true,
			EscalateOnFailure: true,
		},
		Memory: Memory{
			Obsidian: Obsidian{
				Enabled:            false,
				Folder:             DefaultObsidianFolder,
				ExportHistoryLimit: DefaultObsidianHistoryLimit,
			},
		},
		Security: Security{
			Enabled:               true,
			Required:              false,
			KeyProvider:           DefaultSecurityKeyProvider,
			EncryptHistory:        true,
			EncryptMemory:         true,
			EncryptLogs:           true,
			RetainFullRepoContext: false,
			RecoveryExported:      false,
		},
	}
}

func DefaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	return filepath.Join(home, configDirName), nil
}

func DefaultPath() (string, error) {
	dir, err := DefaultDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, configName), nil
}

func HistoryDirForConfigPath(path string) string {
	return filepath.Join(filepath.Dir(path), historyName)
}

func Init(path string, opts InitOptions) error {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return err
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	flags := os.O_WRONLY | os.O_CREATE
	if opts.Force {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}

	contents, err := Marshal(Default())
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("config already exists at %s; use --force to overwrite", path)
		}
		return fmt.Errorf("create config file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(contents); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	if err := os.MkdirAll(HistoryDirForConfigPath(path), 0o700); err != nil {
		return fmt.Errorf("create history directory: %w", err)
	}

	return nil
}

func Load(path string) (Config, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return Config{}, err
		}
	}

	k := koanf.New(".")
	if err := k.Load(file.Provider(path), tomlparser.Parser()); err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	ApplyDefaults(&cfg)

	if err := Validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func Read(path string) ([]byte, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return nil, err
		}
	}

	if _, err := Load(path); err != nil {
		return nil, err
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	return contents, nil
}

func Write(path string, cfg Config) error {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return err
		}
	}
	ApplyDefaults(&cfg)

	contents, err := Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func Marshal(cfg Config) ([]byte, error) {
	ApplyDefaults(&cfg)

	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	enc.SetArraysMultiline(true)
	enc.SetIndentTables(true)
	if err := enc.Encode(cfg); err != nil {
		return nil, fmt.Errorf("encode default config: %w", err)
	}

	return buf.Bytes(), nil
}

func ApplyDefaults(cfg *Config) {
	if cfg == nil {
		return
	}
	if cfg.Memory.Obsidian.Folder == "" {
		cfg.Memory.Obsidian.Folder = DefaultObsidianFolder
	}
	if cfg.Memory.Obsidian.ExportHistoryLimit <= 0 {
		cfg.Memory.Obsidian.ExportHistoryLimit = DefaultObsidianHistoryLimit
	}
	if cfg.Security.KeyProvider == "" {
		cfg.Security.KeyProvider = DefaultSecurityKeyProvider
	}
}

func Validate(cfg Config) error {
	ApplyDefaults(&cfg)

	if err := validator.New().Struct(cfg); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	if len(cfg.Providers) == 0 {
		return errors.New("validate config: at least one provider is required")
	}

	for group, providers := range cfg.Providers {
		if len(providers) == 0 {
			return fmt.Errorf("validate config: provider group %q must contain at least one provider", group)
		}
		for name, provider := range providers {
			switch group {
			case "lmstudio":
				if provider.BaseURL == "" {
					return fmt.Errorf("validate config: providers.%s.%s.base_url is required", group, name)
				}
			case "codex":
				if provider.Profile == "" {
					return fmt.Errorf("validate config: providers.%s.%s.profile is required", group, name)
				}
			}
		}
	}

	return nil
}

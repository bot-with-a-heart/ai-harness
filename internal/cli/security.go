package cli

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	appconfig "ai-harness/internal/config"
	"ai-harness/internal/security"

	"github.com/spf13/cobra"
)

func newSecurityCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "security",
		Short: "Manage encryption-at-rest security",
	}
	cmd.PersistentFlags().StringVar(&configPath, "config", "", "config file path")
	cmd.AddCommand(
		newSecurityStatusCommand(&configPath),
		newSecurityInitCommand(&configPath),
		newSecurityMigrateCommand(&configPath),
		newSecurityVerifyCommand(&configPath),
		newSecurityExportRecoveryCommand(&configPath),
		newSecurityRotateKeyCommand(&configPath),
		newSecurityLockCommand(&configPath),
		newSecurityUnlockCommand(&configPath),
	)
	return cmd
}

func newSecurityStatusCommand(configPath *string) *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show encryption-at-rest status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := loadSecurityStatus(*configPath)
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(status)
			}
			printSecurityStatus(cmd.OutOrStdout(), status)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print status as JSON")
	return cmd
}

func newSecurityInitCommand(configPath *string) *cobra.Command {
	var provider string
	var passphrase string
	var required bool
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize encryption-at-rest keys",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := loadConfigForSecurity(*configPath)
			if err != nil {
				return err
			}
			if cfg.Security.KeyID != "" && force {
				return errors.New("security is already initialized; use security rotate-key to change keys")
			}
			cfg, err = security.Init(cfg, security.InitOptions{
				Provider:   provider,
				Passphrase: passphrase,
				Required:   required,
				Force:      force,
			})
			if err != nil {
				return err
			}
			if err := appconfig.Write(path, cfg); err != nil {
				if cfg.Security.KeyProvider == security.KeyProviderOSKeychain {
					_ = security.DeleteOSKey(cfg.Security.KeyID)
				}
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Security initialized in %s\n", path)
			fmt.Fprintf(cmd.OutOrStdout(), "Key provider: %s\n", cfg.Security.KeyProvider)
			fmt.Fprintf(cmd.OutOrStdout(), "Key ID: %s\n", cfg.Security.KeyID)
			fmt.Fprintln(cmd.OutOrStdout(), "Run security export-recovery and security migrate next.")
			return nil
		},
	}
	cmd.Flags().StringVar(&provider, "provider", security.KeyProviderOSKeychain, "key provider: os-keychain or passphrase")
	cmd.Flags().StringVar(&passphrase, "passphrase", "", "passphrase for passphrase key provider")
	cmd.Flags().BoolVar(&required, "required", false, "block disabling encryption for this config")
	cmd.Flags().BoolVar(&force, "force", false, "reserved for fresh reinitialization; use rotate-key for initialized configs")
	return cmd
}

func newSecurityMigrateCommand(configPath *string) *cobra.Command {
	var passphrase string
	var recoveryFile string

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Encrypt existing plaintext sensitive stores",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := loadConfigForSecurity(*configPath)
			if err != nil {
				return err
			}
			key, err := resolveSecurityKey(cfg, passphrase, recoveryFile)
			if err != nil {
				return err
			}
			store := security.NewEncryptedHistoryStore(appconfig.HistoryDirForConfigPath(path), key, cfg.Security.KeyID)
			report, err := store.MigratePlaintext()
			if err != nil {
				return err
			}
			printHistorySecurityReport(cmd.OutOrStdout(), "Migration", report)
			return historyReportError("migration", report)
		},
	}
	cmd.Flags().StringVar(&passphrase, "passphrase", "", "passphrase for passphrase key provider")
	cmd.Flags().StringVar(&recoveryFile, "recovery-file", "", "recovery material file exported by security export-recovery")
	return cmd
}

func newSecurityVerifyCommand(configPath *string) *cobra.Command {
	var passphrase string
	var recoveryFile string
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify encrypted sensitive stores",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := loadConfigForSecurity(*configPath)
			if err != nil {
				return err
			}
			key, err := resolveSecurityKey(cfg, passphrase, recoveryFile)
			if err != nil {
				return err
			}
			store := security.NewEncryptedHistoryStore(appconfig.HistoryDirForConfigPath(path), key, cfg.Security.KeyID)
			report, err := store.Verify()
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(report); err != nil {
					return err
				}
				return historyReportError("verification", report)
			}
			printHistorySecurityReport(cmd.OutOrStdout(), "Verification", report)
			return historyReportError("verification", report)
		},
	}
	cmd.Flags().StringVar(&passphrase, "passphrase", "", "passphrase for passphrase key provider")
	cmd.Flags().StringVar(&recoveryFile, "recovery-file", "", "recovery material file exported by security export-recovery")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print verification as JSON")
	return cmd
}

func newSecurityExportRecoveryCommand(configPath *string) *cobra.Command {
	var passphrase string
	var output string
	var stdout bool

	cmd := &cobra.Command{
		Use:   "export-recovery",
		Short: "Export recovery material for encrypted stores",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(output) == "" && !stdout {
				return errors.New("--output is required unless --stdout is explicitly set")
			}
			cfg, path, err := loadConfigForSecurity(*configPath)
			if err != nil {
				return err
			}
			key, err := security.ResolveKey(cfg, passphrase)
			if err != nil {
				return err
			}
			recovery := map[string]string{
				"version":     "1",
				"keyProvider": cfg.Security.KeyProvider,
				"keyId":       cfg.Security.KeyID,
				"key":         base64.StdEncoding.EncodeToString(key),
			}
			contents, err := json.MarshalIndent(recovery, "", "  ")
			if err != nil {
				return err
			}
			contents = append(contents, '\n')
			if stdout {
				_, err = cmd.OutOrStdout().Write(contents)
			} else {
				err = os.WriteFile(output, contents, 0o600)
			}
			if err != nil {
				return fmt.Errorf("write recovery material: %w", err)
			}
			cfg.Security.RecoveryExported = true
			if err := appconfig.Write(path, cfg); err != nil {
				return err
			}
			if !stdout {
				fmt.Fprintf(cmd.OutOrStdout(), "Recovery material written to %s\n", output)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&passphrase, "passphrase", "", "passphrase for passphrase key provider")
	cmd.Flags().StringVar(&output, "output", "", "file path for recovery material")
	cmd.Flags().BoolVar(&stdout, "stdout", false, "print recovery material to stdout")
	return cmd
}

func newSecurityRotateKeyCommand(configPath *string) *cobra.Command {
	var passphrase string
	var recoveryFile string
	var newPassphrase string

	cmd := &cobra.Command{
		Use:   "rotate-key",
		Short: "Rotate encryption-at-rest keys",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := loadConfigForSecurity(*configPath)
			if err != nil {
				return err
			}
			oldKey, err := resolveSecurityKey(cfg, passphrase, recoveryFile)
			if err != nil {
				return err
			}

			initPassphrase := newPassphrase
			if cfg.Security.KeyProvider == security.KeyProviderPassphrase && strings.TrimSpace(initPassphrase) == "" {
				return errors.New("--new-passphrase is required for passphrase key rotation")
			}
			newCfg, err := security.Init(cfg, security.InitOptions{
				Provider:   cfg.Security.KeyProvider,
				Passphrase: initPassphrase,
				Required:   cfg.Security.Required,
				Force:      true,
			})
			if err != nil {
				return err
			}
			newKey, err := security.ResolveKey(newCfg, initPassphrase)
			if err != nil {
				return err
			}
			report, err := security.RotateHistory(appconfig.HistoryDirForConfigPath(path), oldKey, cfg.Security.KeyID, newKey, newCfg.Security.KeyID)
			if err != nil {
				return err
			}
			if err := appconfig.Write(path, newCfg); err != nil {
				rollbackReport, rollbackErr := security.RotateHistory(appconfig.HistoryDirForConfigPath(path), newKey, newCfg.Security.KeyID, oldKey, cfg.Security.KeyID)
				if newCfg.Security.KeyProvider == security.KeyProviderOSKeychain {
					_ = security.DeleteOSKey(newCfg.Security.KeyID)
				}
				if rollbackErr != nil || rollbackReport.Invalid > 0 {
					return fmt.Errorf("write config after key rotation: %w; rollback failed: %v", err, rollbackErr)
				}
				return fmt.Errorf("write config after key rotation: %w; encrypted history was rolled back to the previous key", err)
			}
			if cfg.Security.KeyProvider == security.KeyProviderOSKeychain {
				_ = security.DeleteOSKey(cfg.Security.KeyID)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Rotated key %s -> %s\n", cfg.Security.KeyID, newCfg.Security.KeyID)
			printHistorySecurityReport(cmd.OutOrStdout(), "Rotation", report)
			return historyReportError("rotation", report)
		},
	}
	cmd.Flags().StringVar(&passphrase, "passphrase", "", "current passphrase for passphrase key provider")
	cmd.Flags().StringVar(&recoveryFile, "recovery-file", "", "recovery material file exported by security export-recovery")
	cmd.Flags().StringVar(&newPassphrase, "new-passphrase", "", "new passphrase for passphrase key provider")
	return cmd
}

func newSecurityLockCommand(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "lock",
		Short: "Explain current lock behavior",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadConfigForSecurity(*configPath)
			if err != nil {
				return err
			}
			if cfg.Security.KeyProvider == security.KeyProviderOSKeychain {
				fmt.Fprintln(cmd.OutOrStdout(), "OS keychain provider is locked by the operating system session.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Passphrase provider has no persistent unlock session. Clear %s from the environment to lock future commands.\n", security.PassphraseEnv)
			return nil
		},
	}
}

func newSecurityUnlockCommand(configPath *string) *cobra.Command {
	var passphrase string
	var recoveryFile string

	cmd := &cobra.Command{
		Use:   "unlock",
		Short: "Validate access to encrypted stores",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadConfigForSecurity(*configPath)
			if err != nil {
				return err
			}
			if _, err := resolveSecurityKey(cfg, passphrase, recoveryFile); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Security key is available.")
			return nil
		},
	}
	cmd.Flags().StringVar(&passphrase, "passphrase", "", "passphrase for passphrase key provider")
	cmd.Flags().StringVar(&recoveryFile, "recovery-file", "", "recovery material file exported by security export-recovery")
	return cmd
}

type securityRecoveryMaterial struct {
	Version     string `json:"version"`
	KeyProvider string `json:"keyProvider"`
	KeyID       string `json:"keyId"`
	Key         string `json:"key"`
}

func resolveSecurityKey(cfg appconfig.Config, passphrase string, recoveryFile string) ([]byte, error) {
	recoveryFile = strings.TrimSpace(recoveryFile)
	if recoveryFile == "" {
		return security.ResolveKey(cfg, passphrase)
	}
	material, key, err := readSecurityRecoveryMaterial(recoveryFile)
	if err != nil {
		return nil, err
	}
	if material.KeyID != "" && cfg.Security.KeyID != "" && material.KeyID != cfg.Security.KeyID {
		return nil, fmt.Errorf("recovery material key %s does not match config key %s", material.KeyID, cfg.Security.KeyID)
	}
	return key, nil
}

func readSecurityRecoveryMaterial(path string) (securityRecoveryMaterial, []byte, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return securityRecoveryMaterial{}, nil, fmt.Errorf("read recovery material: %w", err)
	}
	var material securityRecoveryMaterial
	if err := json.Unmarshal(contents, &material); err != nil {
		return securityRecoveryMaterial{}, nil, fmt.Errorf("decode recovery material: %w", err)
	}
	if material.Version != "" && material.Version != "1" {
		return securityRecoveryMaterial{}, nil, fmt.Errorf("unsupported recovery material version %q", material.Version)
	}
	key, err := base64.StdEncoding.DecodeString(material.Key)
	if err != nil {
		return securityRecoveryMaterial{}, nil, fmt.Errorf("decode recovery key: %w", err)
	}
	if len(key) != security.KeySize {
		return securityRecoveryMaterial{}, nil, fmt.Errorf("recovery key has invalid length")
	}
	return material, key, nil
}

type securityStatus struct {
	ConfigPath       string                 `json:"configPath"`
	Enabled          bool                   `json:"enabled"`
	Required         bool                   `json:"required"`
	Initialized      bool                   `json:"initialized"`
	KeyProvider      string                 `json:"keyProvider"`
	KeyID            string                 `json:"keyId,omitempty"`
	KeyAvailable     bool                   `json:"keyAvailable"`
	KeyError         string                 `json:"keyError,omitempty"`
	EncryptHistory   bool                   `json:"encryptHistory"`
	EncryptMemory    bool                   `json:"encryptMemory"`
	EncryptLogs      bool                   `json:"encryptLogs"`
	RecoveryExported bool                   `json:"recoveryExported"`
	History          security.HistoryReport `json:"history"`
}

func loadSecurityStatus(configPath string) (securityStatus, error) {
	cfg, path, err := loadConfigForSecurity(configPath)
	if err != nil {
		return securityStatus{}, err
	}
	status := securityStatus{
		ConfigPath:       path,
		Enabled:          cfg.Security.Enabled,
		Required:         cfg.Security.Required,
		Initialized:      cfg.Security.KeyID != "",
		KeyProvider:      cfg.Security.KeyProvider,
		KeyID:            cfg.Security.KeyID,
		EncryptHistory:   cfg.Security.EncryptHistory,
		EncryptMemory:    cfg.Security.EncryptMemory,
		EncryptLogs:      cfg.Security.EncryptLogs,
		RecoveryExported: cfg.Security.RecoveryExported,
	}
	if cfg.Security.KeyID != "" {
		key, err := security.ResolveKey(cfg, "")
		if err == nil {
			status.KeyAvailable = true
			store := security.NewEncryptedHistoryStore(appconfig.HistoryDirForConfigPath(path), key, cfg.Security.KeyID)
			report, err := store.Verify()
			if err == nil {
				status.History = report
			}
		} else {
			status.KeyError = err.Error()
		}
	}
	return status, nil
}

func loadConfigForSecurity(configPath string) (appconfig.Config, string, error) {
	path := strings.TrimSpace(configPath)
	if path == "" {
		var err error
		path, err = appconfig.DefaultPath()
		if err != nil {
			return appconfig.Config{}, "", err
		}
	}
	cfg, err := appconfig.Load(path)
	if err != nil {
		return appconfig.Config{}, path, err
	}
	return cfg, path, nil
}

func printSecurityStatus(w interface {
	Write([]byte) (int, error)
}, status securityStatus) {
	fmt.Fprintf(w, "Config: %s\n", status.ConfigPath)
	fmt.Fprintf(w, "Security enabled: %t\n", status.Enabled)
	fmt.Fprintf(w, "Security required: %t\n", status.Required)
	fmt.Fprintf(w, "Initialized: %t\n", status.Initialized)
	fmt.Fprintf(w, "Key provider: %s\n", status.KeyProvider)
	fmt.Fprintf(w, "Key ID: %s\n", emptyDash(status.KeyID))
	fmt.Fprintf(w, "Key available: %t\n", status.KeyAvailable)
	if status.KeyError != "" {
		fmt.Fprintf(w, "Key error: %s\n", status.KeyError)
	}
	fmt.Fprintf(w, "Encrypt history: %t\n", status.EncryptHistory)
	fmt.Fprintf(w, "Encrypt memory: %t\n", status.EncryptMemory)
	fmt.Fprintf(w, "Encrypt logs: %t\n", status.EncryptLogs)
	fmt.Fprintf(w, "Recovery exported: %t\n", status.RecoveryExported)
	fmt.Fprintf(w, "History encrypted: %d\n", status.History.Encrypted)
	fmt.Fprintf(w, "History plaintext: %d\n", status.History.Plaintext)
	fmt.Fprintf(w, "History invalid: %d\n", status.History.Invalid)
}

func printHistorySecurityReport(w interface {
	Write([]byte) (int, error)
}, label string, report security.HistoryReport) {
	fmt.Fprintf(w, "%s complete.\n", label)
	fmt.Fprintf(w, "Encrypted records: %d\n", report.Encrypted)
	fmt.Fprintf(w, "Plaintext records: %d\n", report.Plaintext)
	fmt.Fprintf(w, "Invalid records: %d\n", report.Invalid)
	for _, err := range report.Errors {
		fmt.Fprintf(w, "Error: %s\n", err)
	}
}

func historyReportError(action string, report security.HistoryReport) error {
	if report.Invalid == 0 && len(report.Errors) == 0 {
		return nil
	}
	if len(report.Errors) > 0 {
		return fmt.Errorf("%s found %d invalid history record(s): %s", action, report.Invalid, report.Errors[0])
	}
	return fmt.Errorf("%s found %d invalid history record(s)", action, report.Invalid)
}

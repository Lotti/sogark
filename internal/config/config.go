package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DirName    = ".sogark"
	FileName   = "config.yaml"
	KeysDirName = "keys"
)

// Default values for Sogei environment
const (
	DefaultPVWABaseURL      = "https://cyberark.sogei.it/PasswordVault"
	DefaultIDPURL           = "https://aag4837.my.idaptive.app/login?yfirtnecapplogin=true&appKey=0f8346cb-fc6f-4ed4-9ebc-e2fcf5ae90c8&customerId=AAG4837&stateId=hFdfLAHPLkyZj2ml2B5cjMBjVjnT6AZd42pjywyZBoU1&yfirtnecrun=true"
	DefaultProxyHost        = "psmp.sogei.it"
	DefaultTargetUser       = "root"
	DefaultSSHKeyName       = "id_sogark"
	DefaultKeyTTLHours      = 4
	DefaultSAMLTimeoutMin   = 5
)

var DefaultKeyFormats = []string{"OpenSSH", "PEM", "PPK"}

// ValidKeys lists all settable configuration keys.
var ValidKeys = []string{
	"username", "pvwa_base_url", "idp_url", "proxy_host",
	"key_dir", "key_formats", "default_target_user",
	"ssh_key_name", "key_ttl_hours", "saml_timeout_minutes",
}

type Config struct {
	Username          string   `yaml:"username"`
	PVWABaseURL       string   `yaml:"pvwa_base_url"`
	IDPURL            string   `yaml:"idp_url"`
	ProxyHost         string   `yaml:"proxy_host"`
	KeyDir            string   `yaml:"key_dir"`
	KeyFormats        []string `yaml:"key_formats"`
	DefaultTargetUser string   `yaml:"default_target_user"`
	SSHKeyName         string   `yaml:"ssh_key_name"`
	KeyTTLHours        int      `yaml:"key_ttl_hours"`
	SAMLTimeoutMinutes int      `yaml:"saml_timeout_minutes"`
}

// Dir returns the sogark configuration directory (~/.sogark).
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("impossibile determinare la home directory: %w", err)
	}
	return filepath.Join(home, DirName), nil
}

// Path returns the full path to config.yaml.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, FileName), nil
}

// DefaultKeyDir returns the default key directory (~/.sogark/keys).
func DefaultKeyDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, KeysDirName), nil
}

// Defaults returns a Config with Sogei default values.
func Defaults() Config {
	keyDir, _ := DefaultKeyDir()
	return Config{
		PVWABaseURL:       DefaultPVWABaseURL,
		IDPURL:            DefaultIDPURL,
		ProxyHost:         DefaultProxyHost,
		KeyDir:            keyDir,
		KeyFormats:        append([]string{}, DefaultKeyFormats...),
		DefaultTargetUser: DefaultTargetUser,
		SSHKeyName:         DefaultSSHKeyName,
		KeyTTLHours:        DefaultKeyTTLHours,
		SAMLTimeoutMinutes: DefaultSAMLTimeoutMin,
	}
}

// Load reads the configuration from disk.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("configurazione non trovata: esegui 'sogark config init'")
		}
		return nil, fmt.Errorf("errore lettura config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("errore parsing config: %w", err)
	}
	return &cfg, nil
}

// Save writes the configuration to disk, creating the directory if needed.
func (c *Config) Save() error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("errore creazione directory %s: %w", dir, err)
	}
	path := filepath.Join(dir, FileName)

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("errore serializzazione config: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("errore scrittura config: %w", err)
	}
	return nil
}

// Set updates a single configuration field by key name.
func (c *Config) Set(key, value string) error {
	switch key {
	case "username":
		c.Username = value
	case "pvwa_base_url":
		c.PVWABaseURL = value
	case "idp_url":
		c.IDPURL = value
	case "proxy_host":
		c.ProxyHost = value
	case "key_dir":
		c.KeyDir = value
	case "key_formats":
		c.KeyFormats = splitAndTrim(value)
	case "default_target_user":
		c.DefaultTargetUser = value
	case "ssh_key_name":
		c.SSHKeyName = value
	case "key_ttl_hours":
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return fmt.Errorf("key_ttl_hours deve essere un numero intero positivo")
		}
		c.KeyTTLHours = n
	case "saml_timeout_minutes":
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return fmt.Errorf("saml_timeout_minutes deve essere un numero intero positivo")
		}
		c.SAMLTimeoutMinutes = n
	default:
		return fmt.Errorf("chiave sconosciuta: %q\nChiavi valide: %s", key, strings.Join(ValidKeys, ", "))
	}
	return nil
}

// ResolveKeyDir returns the key directory, expanding ~ if needed.
func (c *Config) ResolveKeyDir() (string, error) {
	if strings.HasPrefix(c.KeyDir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, c.KeyDir[2:]), nil
	}
	return c.KeyDir, nil
}

// Show returns a formatted string representation of the configuration.
func (c *Config) Show() string {
	idpDisplay := c.IDPURL
	if len(idpDisplay) > 60 {
		idpDisplay = idpDisplay[:57] + "..."
	}

	return fmt.Sprintf(`username:              %s
pvwa_base_url:         %s
idp_url:               %s
proxy_host:            %s
key_dir:               %s
key_formats:           %s
default_target_user:   %s
ssh_key_name:          %s
key_ttl_hours:         %d
saml_timeout_minutes:  %d`,
		c.Username,
		c.PVWABaseURL,
		idpDisplay,
		c.ProxyHost,
		c.KeyDir,
		strings.Join(c.KeyFormats, ", "),
		c.DefaultTargetUser,
		c.SSHKeyName,
		c.KeyTTLHours,
		c.SAMLTimeoutMinutes,
	)
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

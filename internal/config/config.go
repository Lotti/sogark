package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	msg "github.com/sogei/cyberark-cli/internal/messages"
	"gopkg.in/yaml.v3"
)

const (
	DirName     = ".sogark"
	FileName    = "config.yaml"
	KeysDirName = "keys"
)

// Generic defaults (not company-specific).
const (
	DefaultKeyTTLHours    = 4
	DefaultSAMLTimeoutMin = 5
)

var DefaultKeyFormats = []string{"OpenSSH", "PEM", "PPK"}

// ValidKeys lists all settable configuration keys.
var ValidKeys = []string{
	"username", "pvwa_base_url", "idp_url", "proxy_host",
	"key_dir", "key_formats", "default_ssh_user", "default_scp_user",
	"ssh_key_name", "key_ttl_hours", "saml_timeout_minutes",
	"moba_path", "moba_max_sessions", "tabby_path", "winscp_path",
	"default_multi_backend", "update_repo", "filezilla_path",
}

type Config struct {
	Username           string   `yaml:"username"`
	PVWABaseURL        string   `yaml:"pvwa_base_url"`
	IDPURL             string   `yaml:"idp_url"`
	ProxyHost          string   `yaml:"proxy_host"`
	KeyDir             string   `yaml:"key_dir"`
	KeyFormats         []string `yaml:"key_formats"`
	DefaultSSHUser     string   `yaml:"default_ssh_user"`
	DefaultSCPUser     string   `yaml:"default_scp_user,omitempty"`
	SSHKeyName         string   `yaml:"ssh_key_name"`
	KeyTTLHours        int      `yaml:"key_ttl_hours"`
	SAMLTimeoutMinutes int      `yaml:"saml_timeout_minutes"`
	MobaPath           string   `yaml:"moba_path,omitempty"`
	MobaMaxSessions    int      `yaml:"moba_max_sessions,omitempty"`
	TabbyPath          string   `yaml:"tabby_path,omitempty"`
	WinSCPPath         string   `yaml:"winscp_path,omitempty"`
	DefaultMultiBackend string  `yaml:"default_multi_backend,omitempty"`
	UpdateRepo           string  `yaml:"update_repo,omitempty"`
	FileZillaPath        string  `yaml:"filezilla_path,omitempty"`
}

// Dir returns the sogark configuration directory (~/.sogark).
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf(msg.CfgErrHomeDir, err)
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

// Defaults returns a Config with generic default values (not company-specific).
func Defaults() Config {
	keyDir, _ := DefaultKeyDir()
	return Config{
		KeyDir:             keyDir,
		KeyFormats:         append([]string{}, DefaultKeyFormats...),
		KeyTTLHours:        DefaultKeyTTLHours,
		SAMLTimeoutMinutes: DefaultSAMLTimeoutMin,
		MobaMaxSessions:    20,
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
			return nil, fmt.Errorf(msg.CfgNotFound)
		}
		return nil, fmt.Errorf(msg.CfgReadErr, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf(msg.CfgParseErr, err)
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
		return fmt.Errorf(msg.CfgMkdirErr, dir, err)
	}
	path := filepath.Join(dir, FileName)

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf(msg.CfgSerializeErr, err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf(msg.CfgWriteErr, err)
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
	case "default_ssh_user":
		c.DefaultSSHUser = value
	case "default_scp_user":
		c.DefaultSCPUser = value
	case "ssh_key_name":
		c.SSHKeyName = value
	case "key_ttl_hours":
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return fmt.Errorf(msg.CfgKeyTTLHoursErr)
		}
		c.KeyTTLHours = n
	case "saml_timeout_minutes":
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return fmt.Errorf(msg.CfgSAMLTimeoutErr)
		}
		c.SAMLTimeoutMinutes = n
	case "moba_path":
		c.MobaPath = value
	case "moba_max_sessions":
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return fmt.Errorf(msg.CfgMobaMaxErr)
		}
		c.MobaMaxSessions = n
	case "tabby_path":
		c.TabbyPath = value
	case "winscp_path":
		c.WinSCPPath = value
	case "default_multi_backend":
		valid := map[string]bool{"auto": true, "wezterm": true, "tabby": true, "wt": true, "tmux": true}
		if !valid[value] {
			return fmt.Errorf(msg.CfgInvalidBackend, value)
		}
		c.DefaultMultiBackend = value
	case "update_repo":
		c.UpdateRepo = value
	case "filezilla_path":
		c.FileZillaPath = value
	default:
		return fmt.Errorf(msg.CfgUnknownKey, key, strings.Join(ValidKeys, ", "))
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

	result := fmt.Sprintf(`username:              %s
pvwa_base_url:         %s
idp_url:               %s
proxy_host:            %s
key_dir:               %s
key_formats:           %s
default_ssh_user:      %s
default_scp_user:      %s
ssh_key_name:          %s
key_ttl_hours:         %d
saml_timeout_minutes:  %d`,
		c.Username,
		c.PVWABaseURL,
		idpDisplay,
		c.ProxyHost,
		c.KeyDir,
		strings.Join(c.KeyFormats, ", "),
		c.DefaultSSHUser,
		c.DefaultSCPUser,
		c.SSHKeyName,
		c.KeyTTLHours,
		c.SAMLTimeoutMinutes,
	)
	if c.MobaPath != "" {
		result += fmt.Sprintf("\nmoba_path:             %s", c.MobaPath)
	}
	maxSess := c.MobaMaxSessions
	if maxSess == 0 {
		maxSess = 20
	}
	result += fmt.Sprintf("\nmoba_max_sessions:     %d", maxSess)
	if c.TabbyPath != "" {
		result += fmt.Sprintf("\ntabby_path:            %s", c.TabbyPath)
	}
	if c.WinSCPPath != "" {
		result += fmt.Sprintf("\nwinscp_path:           %s", c.WinSCPPath)
	}
	if c.DefaultMultiBackend != "" {
		result += fmt.Sprintf("\ndefault_multi_backend: %s", c.DefaultMultiBackend)
	}
	if c.UpdateRepo != "" {
		result += fmt.Sprintf("\nupdate_repo:           %s", c.UpdateRepo)
	}
	if c.FileZillaPath != "" {
		result += fmt.Sprintf("\nfilezilla_path:        %s", c.FileZillaPath)
	}
	return result
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

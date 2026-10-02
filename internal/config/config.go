package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	msg "github.com/Lotti/sogark/internal/messages"
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
	DefaultAuthTimeoutMin = 2
	DefaultUpdateRepo     = "Lotti/sogark"
)

var DefaultKeyFormats = []string{"OpenSSH", "PEM", "PPK"}

var (
	ErrConfigNotFound   = errors.New(msg.CfgNotFound)
	ErrLegacyAuthConfig = errors.New("configuration does not use the new auth_profile/auth_profiles structure; run 'sogark config init' to rebuild it (or 'sogark --config <file> config init' for a custom file); the wizard will back up the old file before saving")
	repoPattern         = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	FileOverride        string
)

// ValidKeys lists all settable configuration keys.
var ValidKeys = []string{
	"username", "auth_profile", "proxy_host",
	"key_dir", "key_formats", "default_ssh_user", "default_scp_user",
	"ssh_key_name", "key_ttl_hours", "auth_timeout_minutes",
	"moba_path", "moba_max_sessions", "tabby_path", "winscp_path",
	"default_multi_backend", "update_repo", "filezilla_path",
}

type Config struct {
	Username            string                 `yaml:"username"`
	AuthProfile         string                 `yaml:"auth_profile"`
	AuthProfiles        map[string]AuthProfile `yaml:"auth_profiles"`
	ProxyHost           string                 `yaml:"proxy_host"`
	KeyDir              string                 `yaml:"key_dir"`
	KeyFormats          []string               `yaml:"key_formats"`
	DefaultSSHUser      string                 `yaml:"default_ssh_user"`
	DefaultSCPUser      string                 `yaml:"default_scp_user,omitempty"`
	SSHKeyName          string                 `yaml:"ssh_key_name"`
	KeyTTLHours         int                    `yaml:"key_ttl_hours"`
	AuthTimeoutMinutes  int                    `yaml:"auth_timeout_minutes"`
	MobaPath            string                 `yaml:"moba_path,omitempty"`
	MobaMaxSessions     int                    `yaml:"moba_max_sessions,omitempty"`
	TabbyPath           string                 `yaml:"tabby_path,omitempty"`
	WinSCPPath          string                 `yaml:"winscp_path,omitempty"`
	DefaultMultiBackend string                 `yaml:"default_multi_backend,omitempty"`
	UpdateRepo          string                 `yaml:"update_repo,omitempty"`
	FileZillaPath       string                 `yaml:"filezilla_path,omitempty"`
}

type ValidationIssue struct {
	Field   string
	Message string
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
	if FileOverride != "" {
		path := FileOverride
		if strings.HasPrefix(path, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			path = filepath.Join(home, path[2:])
		}
		return filepath.Abs(path)
	}
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
		AuthTimeoutMinutes: DefaultAuthTimeoutMin,
		AuthProfiles:       make(map[string]AuthProfile),
		MobaMaxSessions:    20,
		UpdateRepo:         DefaultUpdateRepo,
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
			return nil, ErrConfigNotFound
		}
		return nil, fmt.Errorf(msg.CfgReadErr, err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf(msg.CfgParseErr, err)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, ErrLegacyAuthConfig
	}
	mapping := document.Content[0]
	var profile, profiles *yaml.Node
	for i := 0; i < len(mapping.Content); i += 2 {
		switch mapping.Content[i].Value {
		case "auth_profile":
			profile = mapping.Content[i+1]
		case "auth_profiles":
			profiles = mapping.Content[i+1]
		}
	}
	legacy := profile == nil || profile.Kind != yaml.ScalarNode || profile.Tag != "!!str" ||
		profiles == nil || profiles.Kind != yaml.MappingNode
	if legacy {
		filtered := *mapping
		filtered.Content = nil
		for i := 0; i < len(mapping.Content); i += 2 {
			if name := mapping.Content[i].Value; name != "auth_profile" && name != "auth_profiles" {
				filtered.Content = append(filtered.Content, mapping.Content[i], mapping.Content[i+1])
			}
		}
		mapping = &filtered
	}
	cfg := Defaults()
	if err := mapping.Decode(&cfg); err != nil {
		return nil, fmt.Errorf(msg.CfgParseErr, err)
	}
	if len(cfg.KeyFormats) > 0 {
		formats, err := NormalizeKeyFormats(cfg.KeyFormats)
		if err != nil {
			return nil, err
		}
		cfg.KeyFormats = formats
	}
	if legacy {
		return &cfg, ErrLegacyAuthConfig
	}
	return &cfg, nil
}

// Backup preserves the selected configuration without overwriting older backups.
func Backup() (string, error) {
	path, err := Path()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read configuration for backup: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".legacy-*.bak")
	if err != nil {
		return "", fmt.Errorf("create configuration backup: %w", err)
	}
	_, writeErr := file.Write(data)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return "", fmt.Errorf("write configuration backup %q: %w", file.Name(), err)
	}
	return file.Name(), nil
}

func LoadOrDefaults() (*Config, error) {
	cfg, err := Load()
	if err == nil {
		return cfg, nil
	}
	if errors.Is(err, ErrConfigNotFound) {
		defaults := Defaults()
		return &defaults, nil
	}
	return nil, err
}

// Save writes the configuration to disk, creating the directory if needed.
func (c *Config) Save() error {
	path, err := Path()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf(msg.CfgMkdirErr, dir, err)
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf(msg.CfgSerializeErr, err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf(msg.CfgWriteErr, err)
	}
	return os.Chmod(path, 0600)
}

// Set updates a single configuration field by key name.
func (c *Config) Set(key, value string) error {
	if strings.HasPrefix(key, "auth_profiles.") {
		parts := strings.SplitN(key, ".", 3)
		if len(parts) != 3 || !profileNamePattern.MatchString(parts[1]) {
			return errors.New("use auth_profiles.<profile>.<field>")
		}
		profile := c.AuthProfiles[parts[1]]
		fields := profile.Fields()
		target, ok := fields[parts[2]]
		if !ok {
			return fmt.Errorf("unknown authentication profile field %q", parts[2])
		}
		*target = strings.TrimSpace(value)
		if parts[2] == "auth_type" {
			profile.AuthType = strings.ToLower(profile.AuthType)
		}
		if c.AuthProfiles == nil {
			c.AuthProfiles = make(map[string]AuthProfile)
		}
		c.AuthProfiles[parts[1]] = profile
		return nil
	}
	switch key {
	case "username":
		c.Username = value
	case "auth_profile":
		if !profileNamePattern.MatchString(value) {
			return errors.New("auth_profile must contain only letters, digits, underscores or hyphens")
		}
		c.AuthProfile = value
	case "proxy_host":
		c.ProxyHost = value
	case "key_dir":
		c.KeyDir = value
	case "key_formats":
		formats, err := NormalizeKeyFormats(splitAndTrim(value))
		if err != nil {
			return err
		}
		c.KeyFormats = formats
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
	case "auth_timeout_minutes":
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return errors.New("auth_timeout_minutes must be a positive integer")
		}
		c.AuthTimeoutMinutes = n
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
		value = strings.TrimSpace(value)
		if value != "" && !repoPattern.MatchString(value) {
			return fmt.Errorf(msg.CfgInvalidRepo)
		}
		c.UpdateRepo = value
	case "filezilla_path":
		c.FileZillaPath = value
	default:
		return fmt.Errorf(msg.CfgUnknownKey, key, strings.Join(ValidKeys, ", "))
	}
	return nil
}

func (c *Config) ResolvedUpdateRepo() string {
	if strings.TrimSpace(c.UpdateRepo) == "" {
		return DefaultUpdateRepo
	}
	return strings.TrimSpace(c.UpdateRepo)
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

func NormalizeKeyFormats(formats []string) ([]string, error) {
	if len(formats) == 0 {
		return nil, fmt.Errorf(msg.CfgKeyFormatsErr)
	}

	normalized := make([]string, 0, len(formats))
	seen := make(map[string]bool)
	for _, format := range formats {
		switch strings.ToLower(strings.TrimSpace(format)) {
		case "openssh":
			if !seen["OpenSSH"] {
				normalized = append(normalized, "OpenSSH")
				seen["OpenSSH"] = true
			}
		case "pem":
			if !seen["PEM"] {
				normalized = append(normalized, "PEM")
				seen["PEM"] = true
			}
		case "ppk":
			if !seen["PPK"] {
				normalized = append(normalized, "PPK")
				seen["PPK"] = true
			}
		case "":
		default:
			return nil, fmt.Errorf(msg.CfgKeyFormatsErr)
		}
	}

	if len(normalized) == 0 {
		return nil, fmt.Errorf(msg.CfgKeyFormatsErr)
	}
	return normalized, nil
}

func (c *Config) ValidationIssues() []ValidationIssue {
	var issues []ValidationIssue

	required := map[string]string{
		"username":         strings.TrimSpace(c.Username),
		"auth_profile":     strings.TrimSpace(c.AuthProfile),
		"proxy_host":       strings.TrimSpace(c.ProxyHost),
		"key_dir":          strings.TrimSpace(c.KeyDir),
		"default_ssh_user": strings.TrimSpace(c.DefaultSSHUser),
		"ssh_key_name":     strings.TrimSpace(c.SSHKeyName),
	}
	for field, value := range required {
		if value == "" {
			issues = append(issues, ValidationIssue{Field: field, Message: "is required"})
		}
	}

	if _, err := c.SelectedAuthProfile(); err != nil {
		issues = append(issues, ValidationIssue{Field: "auth_profiles", Message: err.Error()})
	}
	if c.KeyDir != "" {
		if resolved, err := c.ResolveKeyDir(); err != nil || strings.TrimSpace(resolved) == "" {
			issues = append(issues, ValidationIssue{Field: "key_dir", Message: "must resolve to a valid path"})
		}
	}
	if strings.ContainsAny(c.SSHKeyName, `/\`) {
		issues = append(issues, ValidationIssue{Field: "ssh_key_name", Message: "must be a file name, not a path"})
	}
	if _, err := NormalizeKeyFormats(c.KeyFormats); err != nil {
		issues = append(issues, ValidationIssue{Field: "key_formats", Message: err.Error()})
	}
	if c.KeyTTLHours <= 0 {
		issues = append(issues, ValidationIssue{Field: "key_ttl_hours", Message: "must be greater than 0"})
	}
	if c.AuthTimeoutMinutes <= 0 {
		issues = append(issues, ValidationIssue{Field: "auth_timeout_minutes", Message: "must be greater than 0"})
	}
	if c.DefaultMultiBackend != "" {
		valid := map[string]bool{"auto": true, "wezterm": true, "tabby": true, "wt": true, "tmux": true}
		if !valid[c.DefaultMultiBackend] {
			issues = append(issues, ValidationIssue{Field: "default_multi_backend", Message: fmt.Sprintf(msg.CfgInvalidBackend, c.DefaultMultiBackend)})
		}
	}
	if c.UpdateRepo != "" && !repoPattern.MatchString(strings.TrimSpace(c.UpdateRepo)) {
		issues = append(issues, ValidationIssue{Field: "update_repo", Message: msg.CfgInvalidRepo})
	}

	return issues
}

func (c *Config) Validate() error {
	issues := c.ValidationIssues()
	if len(issues) == 0 {
		return nil
	}

	var lines []string
	for _, issue := range issues {
		lines = append(lines, fmt.Sprintf("- %s: %s", issue.Field, issue.Message))
	}
	return fmt.Errorf(msg.ConfigValidationFailed, strings.Join(lines, "\n"))
}

// Show returns a formatted string representation of the configuration.
func (c *Config) Show() string {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Sprintf("cannot display configuration: %v", err)
	}
	return string(data)
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

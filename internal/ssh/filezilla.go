package ssh

import (
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	msg "github.com/Lotti/sogark/internal/messages"
)

// ── FileZilla XML structures ─────────────────────────────────────────────────

// filezillaXML is the root of sitemanager.xml.
type filezillaXML struct {
	XMLName xml.Name         `xml:"FileZilla3"`
	Servers filezillaServers `xml:"Servers"`
}

type filezillaServers struct {
	Servers []filezillaServer `xml:"Server"`
}

type filezillaServer struct {
	Host         string `xml:"Host"`
	Port         int    `xml:"Port"`
	Protocol     int    `xml:"Protocol"` // 1 = SFTP
	Type         int    `xml:"Type"`
	Logontype    int    `xml:"Logontype"` // 5 = key file
	User         string `xml:"User"`
	Pass         string `xml:"Pass"`
	Keyfile      string `xml:"Keyfile"`
	Name         string `xml:"Name"`
	Comments     string `xml:"Comments"`
	LocalDir     string `xml:"LocalDir"`
	RemoteDir    string `xml:"RemoteDir"`
	SyncBrowsing int    `xml:"SyncBrowsing"`
}

// ── Public API ───────────────────────────────────────────────────────────────

// FindFileZilla searches for the FileZilla binary.
func FindFileZilla() string {
	names := []string{"filezilla"}
	if runtime.GOOS == "windows" {
		names = []string{"filezilla.exe"}
	}
	for _, name := range names {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}

	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{"/Applications/FileZilla.app/Contents/MacOS/filezilla"}
	case "linux":
		candidates = []string{"/usr/bin/filezilla", "/usr/local/bin/filezilla"}
	case "windows":
		candidates = []string{
			os.Getenv("ProgramFiles") + "\\FileZilla FTP Client\\filezilla.exe",
			os.Getenv("ProgramFiles(x86)") + "\\FileZilla FTP Client\\filezilla.exe",
		}
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// filezillaConfigDir returns the FileZilla config directory.
func filezillaConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, "FileZilla"), nil
	}
	return filepath.Join(home, ".config", "filezilla"), nil
}

// ConfigureFileZilla updates sitemanager.xml with sogark hosts and returns the key path used.
// Each host gets a Server entry under a "sogark" folder.
func ConfigureFileZilla(hosts []HostTarget, username, proxyHost, keyPath string) (string, error) {
	cfgDir, err := filezillaConfigDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		return "", fmt.Errorf(msg.FileZillaMkdirErr, cfgDir, err)
	}

	sitePath := filepath.Join(cfgDir, "sitemanager.xml")

	// Use absolute key path (FileZilla requires it)
	absKey, err := filepath.Abs(keyPath)
	if err != nil {
		absKey = keyPath
	}
	// FileZilla on Windows wants backslashes
	if runtime.GOOS == "windows" {
		absKey = strings.ReplaceAll(absKey, "/", "\\")
	}

	fz := filezillaXML{
		Servers: filezillaServers{},
	}

	// Read existing if present
	if data, err := os.ReadFile(sitePath); err == nil {
		xml.Unmarshal(data, &fz)
	}

	// Remove old sogark entries (matching by Name prefix or in sogark folder)
	// Simple approach: rebuild server list, filtering out those we'll replace
	existingNames := make(map[string]bool)
	for _, h := range hosts {
		name := fmt.Sprintf("sogark: %s (%s@%s)", h.Name, h.TargetUser, h.Address)
		existingNames[name] = true
	}
	var keep []filezillaServer
	for _, s := range fz.Servers.Servers {
		if !existingNames[s.Name] && !strings.HasPrefix(s.Name, "sogark:") {
			keep = append(keep, s)
		}
	}
	fz.Servers.Servers = keep

	// Add entries for each host
	for _, h := range hosts {
		name := fmt.Sprintf("sogark: %s (%s@%s)", h.Name, h.TargetUser, h.Address)
		psmpUser := fmt.Sprintf("%s@%s@%s@%s", username, h.TargetUser, h.Address, proxyHost)

		fz.Servers.Servers = append(fz.Servers.Servers, filezillaServer{
			Host:         proxyHost,
			Port:         22,
			Protocol:     1, // SFTP
			Type:         0,
			Logontype:    5, // key file
			User:         psmpUser,
			Keyfile:      absKey,
			Name:         name,
			SyncBrowsing: 0,
		})
	}

	data, err := xml.MarshalIndent(fz, "", "  ")
	if err != nil {
		return "", fmt.Errorf(msg.FileZillaXMLMarshalErr, err)
	}

	// Prepend XML declaration manually (xml.MarshalIndent doesn't add it)
	xmlData := []byte(xml.Header + string(data))

	if err := os.WriteFile(sitePath, xmlData, 0600); err != nil {
		return "", fmt.Errorf(msg.FileZillaWriteErr, sitePath, err)
	}

	return absKey, nil
}

// RunFileZilla launches FileZilla after configuring sessions.
func RunFileZilla(hosts []HostTarget, username, proxyHost, keyPath, filezillaPath string) error {
	if len(hosts) == 0 {
		return fmt.Errorf(msg.SSHNoHosts)
	}

	_, err := ConfigureFileZilla(hosts, username, proxyHost, keyPath)
	if err != nil {
		return err
	}

	fmt.Printf(msg.FileZillaConfigured, len(hosts))
	for _, h := range hosts {
		fmt.Printf("    %s (%s@%s)\n", h.Name, h.TargetUser, h.Address)
	}

	// Small delay to ensure file is flushed
	time.Sleep(200 * time.Millisecond)

	cmd := exec.Command(filezillaPath)
	cmd.Stderr = os.Stderr
	return cmd.Start()
}

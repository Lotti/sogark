package hosts

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	msg "github.com/Lotti/sogark/internal/messages"
)

// MobaSession represents an SSH session parsed from a MobaXterm export file.
type MobaSession struct {
	Name    string
	Address string
	User    string
	Tags    []string
}

// ParseMobaFile reads a .mxtsessions file and extracts SSH sessions.
// Folders (SubRep) are mapped to tags; nested folders (A\B) produce separate tags.
func ParseMobaFile(path string) ([]MobaSession, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf(msg.MobaOpenFileErr, err)
	}
	defer f.Close()

	return parseMobaSessions(bufio.NewScanner(f))
}

// ParseMobaContent parses MobaXterm session content from a string (for testing).
func ParseMobaContent(content string) ([]MobaSession, error) {
	return parseMobaSessions(bufio.NewScanner(strings.NewReader(content)))
}

func parseMobaSessions(scanner *bufio.Scanner) ([]MobaSession, error) {
	var sessions []MobaSession
	var currentTags []string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentTags = nil
			continue
		}

		// Folder name → tags
		if strings.HasPrefix(line, "SubRep=") {
			folder := strings.TrimPrefix(line, "SubRep=")
			currentTags = folderToTags(folder)
			continue
		}

		// Skip non-session lines
		if strings.HasPrefix(line, "ImgNum=") || line == "" {
			continue
		}

		// Parse session line: Name=#109#fields...
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		name := strings.TrimSpace(line[:idx])
		value := line[idx+1:]

		session, ok := parseMobaSessionLine(name, value, currentTags)
		if !ok {
			continue
		}
		sessions = append(sessions, session)
	}

	return sessions, scanner.Err()
}

// parseMobaSessionLine parses a single session value.
// SSH sessions start with #109# (or ; logout#109#).
// Returns the session and true if it's a valid SSH session.
func parseMobaSessionLine(name, value string, folderTags []string) (MobaSession, bool) {
	// Strip optional "; logout" prefix
	v := value
	if strings.HasPrefix(v, "; logout") {
		v = strings.TrimPrefix(v, "; logout")
	}

	// Must start with #109# for SSH
	if !strings.HasPrefix(v, "#109#") {
		return MobaSession{}, false
	}

	// Remove the #109# prefix, then split by #
	v = strings.TrimPrefix(v, "#109#")
	sections := strings.SplitN(v, "#", 2)
	if len(sections) == 0 {
		return MobaSession{}, false
	}

	// First section contains %-separated fields
	fields := strings.Split(sections[0], "%")
	// fields[0] = session type (0 for SSH)
	// fields[1] = remote host
	// fields[2] = port
	// fields[3] = username

	if len(fields) < 4 {
		return MobaSession{}, false
	}

	address := fields[1]
	if address == "" {
		return MobaSession{}, false
	}

	user := fields[3]
	if user == "<default>" {
		user = ""
	}

	tags := make([]string, len(folderTags))
	copy(tags, folderTags)

	return MobaSession{
		Name:    name,
		Address: address,
		User:    user,
		Tags:    tags,
	}, true
}

// folderToTags converts a MobaXterm folder path to tags.
// "Production\WebServers" → ["production", "webservers"]
// Empty string → nil
func folderToTags(folder string) []string {
	if folder == "" {
		return nil
	}
	parts := strings.Split(folder, "\\")
	tags := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			tags = append(tags, strings.ToLower(p))
		}
	}
	return tags
}

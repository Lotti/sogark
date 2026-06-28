package hosts

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	msg "github.com/Lotti/sogark/internal/messages"
	"gopkg.in/yaml.v3"
)

const HostsFileName = "hosts.yaml"

// Host represents a registered machine.
type Host struct {
	Name    string   `yaml:"-"`
	Address string   `yaml:"address"`
	User    string   `yaml:"user,omitempty"`
	Tags    []string `yaml:"tags,omitempty"`
}

// HostsFile is the top-level structure of hosts.yaml.
type HostsFile struct {
	Hosts map[string]*Host `yaml:"hosts"`
}

// Registry manages the host inventory with tag-based indexing.
type Registry struct {
	file     HostsFile
	filePath string
	tagIndex map[string]map[string]*Host // tag → name → host
}

// NewRegistry creates a Registry, loading from the given sogark config directory.
func NewRegistry(sogarkDir string) (*Registry, error) {
	r := &Registry{
		filePath: filepath.Join(sogarkDir, HostsFileName),
		file:     HostsFile{Hosts: make(map[string]*Host)},
		tagIndex: make(map[string]map[string]*Host),
	}

	data, err := os.ReadFile(r.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil
		}
		return nil, fmt.Errorf(msg.RegReadErr, r.filePath, err)
	}

	if err := yaml.Unmarshal(data, &r.file); err != nil {
		return nil, fmt.Errorf(msg.RegParseErr, r.filePath, err)
	}
	if r.file.Hosts == nil {
		r.file.Hosts = make(map[string]*Host)
	}

	// Set names and build index
	for name, h := range r.file.Hosts {
		h.Name = name
	}
	r.rebuildIndex()

	return r, nil
}

// Save writes the hosts file to disk.
func (r *Registry) Save() error {
	dir := filepath.Dir(r.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := yaml.Marshal(&r.file)
	if err != nil {
		return err
	}
	return os.WriteFile(r.filePath, data, 0600)
}

// slugify converts a string to a lowercase hyphen-separated slug.
// Spaces and underscores are replaced with hyphens; multiple hyphens are collapsed.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	// Collapse consecutive hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

// Add registers a new host or updates an existing one.
// Name and tags are slugified (spaces → hyphens) for shell-safety.
func (r *Registry) Add(name, address, user string, tags []string) {
	name = slugify(name)
	slugTags := make([]string, 0, len(tags))
	for _, t := range tags {
		if s := slugify(t); s != "" {
			slugTags = append(slugTags, s)
		}
	}
	h := &Host{
		Name:    name,
		Address: address,
		User:    user,
		Tags:    slugTags,
	}
	r.file.Hosts[name] = h
	r.rebuildIndex()
}

// Remove deletes a host by name.
func (r *Registry) Remove(name string) error {
	if _, exists := r.file.Hosts[name]; !exists {
		return fmt.Errorf(msg.RegNotFound, name)
	}
	delete(r.file.Hosts, name)
	r.rebuildIndex()
	return nil
}

// Get returns a host by name.
func (r *Registry) Get(name string) (*Host, bool) {
	h, ok := r.file.Hosts[name]
	return h, ok
}

// AddTags adds tags to an existing host.
func (r *Registry) AddTags(name string, tags []string) error {
	h, ok := r.file.Hosts[name]
	if !ok {
		return fmt.Errorf(msg.RegNotFound, name)
	}
	existing := makeStringSet(h.Tags)
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t != "" && !existing[t] {
			h.Tags = append(h.Tags, t)
			existing[t] = true
		}
	}
	sort.Strings(h.Tags)
	r.rebuildIndex()
	return nil
}

// RemoveTags removes tags from an existing host.
func (r *Registry) RemoveTags(name string, tags []string) error {
	h, ok := r.file.Hosts[name]
	if !ok {
		return fmt.Errorf(msg.RegNotFound, name)
	}
	toRemove := makeStringSet(tags)
	var remaining []string
	for _, t := range h.Tags {
		if !toRemove[t] {
			remaining = append(remaining, t)
		}
	}
	h.Tags = remaining
	r.rebuildIndex()
	return nil
}

// All returns all hosts sorted by name.
func (r *Registry) All() []*Host {
	return r.sortedHosts(r.file.Hosts)
}

// ByTagsAND returns hosts that have ALL specified tags.
func (r *Registry) ByTagsAND(tags []string) []*Host {
	if len(tags) == 0 {
		return r.All()
	}

	// Start with hosts that have the first tag
	firstTag := strings.TrimSpace(tags[0])
	candidates, ok := r.tagIndex[firstTag]
	if !ok {
		return nil
	}

	// Intersect with remaining tags
	result := make(map[string]*Host)
	for name, h := range candidates {
		result[name] = h
	}

	for _, tag := range tags[1:] {
		tag = strings.TrimSpace(tag)
		tagHosts, ok := r.tagIndex[tag]
		if !ok {
			return nil
		}
		for name := range result {
			if _, inTag := tagHosts[name]; !inTag {
				delete(result, name)
			}
		}
	}

	return r.sortedHosts(result)
}

// ByTagsOR returns hosts that have at least one of the specified tags.
func (r *Registry) ByTagsOR(tags []string) []*Host {
	if len(tags) == 0 {
		return r.All()
	}

	result := make(map[string]*Host)
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tagHosts, ok := r.tagIndex[tag]; ok {
			for name, h := range tagHosts {
				result[name] = h
			}
		}
	}

	return r.sortedHosts(result)
}

// Search returns hosts matching all non-empty criteria (ANDed).
// namePattern and ipPattern support glob wildcards (*, ?).
// An empty pattern matches any value.
// tags is a list of required tags (AND); empty means any tags.
func (r *Registry) Search(namePattern, ipPattern string, tags []string) []*Host {
	var candidates []*Host
	if len(tags) > 0 {
		candidates = r.ByTagsAND(tags)
	} else {
		candidates = r.All()
	}

	if namePattern == "" && ipPattern == "" {
		return candidates
	}

	var result []*Host
	for _, h := range candidates {
		if namePattern != "" {
			ok, _ := path.Match(strings.ToLower(namePattern), strings.ToLower(h.Name))
			if !ok {
				continue
			}
		}
		if ipPattern != "" {
			ok, _ := path.Match(strings.ToLower(ipPattern), strings.ToLower(h.Address))
			if !ok {
				continue
			}
		}
		result = append(result, h)
	}
	return result
}

func (r *Registry) rebuildIndex() {
	r.tagIndex = make(map[string]map[string]*Host)
	for name, h := range r.file.Hosts {
		for _, tag := range h.Tags {
			if r.tagIndex[tag] == nil {
				r.tagIndex[tag] = make(map[string]*Host)
			}
			r.tagIndex[tag][name] = h
		}
		_ = name
	}
}

func (r *Registry) sortedHosts(m map[string]*Host) []*Host {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	hosts := make([]*Host, 0, len(names))
	for _, name := range names {
		hosts = append(hosts, m[name])
	}
	return hosts
}

func makeStringSet(strs []string) map[string]bool {
	set := make(map[string]bool, len(strs))
	for _, s := range strs {
		set[strings.TrimSpace(s)] = true
	}
	return set
}

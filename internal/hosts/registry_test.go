package hosts

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	dir := t.TempDir()
	r, err := NewRegistry(dir)
	if err != nil {
		t.Fatalf("NewRegistry error: %v", err)
	}
	return r
}

func TestNewRegistry_EmptyDir(t *testing.T) {
	r := newTestRegistry(t)
	if len(r.All()) != 0 {
		t.Error("new registry should have no hosts")
	}
}

func TestAdd_And_Get(t *testing.T) {
	r := newTestRegistry(t)
	r.Add("web1", "10.0.0.1", "root", []string{"web", "prod"})

	h, ok := r.Get("web1")
	if !ok {
		t.Fatal("host web1 not found")
	}
	if h.Address != "10.0.0.1" {
		t.Errorf("address: got %q, want %q", h.Address, "10.0.0.1")
	}
	if h.User != "root" {
		t.Errorf("user: got %q, want %q", h.User, "root")
	}
	if len(h.Tags) != 2 {
		t.Errorf("tags: got %d, want 2", len(h.Tags))
	}
}

func TestAdd_OverwriteExisting(t *testing.T) {
	r := newTestRegistry(t)
	r.Add("web1", "10.0.0.1", "root", []string{"web"})
	r.Add("web1", "10.0.0.2", "admin", []string{"web", "updated"})

	h, _ := r.Get("web1")
	if h.Address != "10.0.0.2" {
		t.Errorf("address: got %q, want %q", h.Address, "10.0.0.2")
	}
	if h.User != "admin" {
		t.Errorf("user: got %q, want %q", h.User, "admin")
	}
	if len(r.All()) != 1 {
		t.Error("should still have only 1 host")
	}
}

func TestRemove(t *testing.T) {
	r := newTestRegistry(t)
	r.Add("web1", "10.0.0.1", "", nil)

	if err := r.Remove("web1"); err != nil {
		t.Fatalf("Remove error: %v", err)
	}
	if _, ok := r.Get("web1"); ok {
		t.Error("host should be removed")
	}
}

func TestRemove_NotFound(t *testing.T) {
	r := newTestRegistry(t)
	err := r.Remove("nonexistent")
	if err == nil {
		t.Error("Remove should return error for non-existent host")
	}
}

func TestAddTags(t *testing.T) {
	r := newTestRegistry(t)
	r.Add("web1", "10.0.0.1", "", []string{"web"})

	if err := r.AddTags("web1", []string{"prod", "critical"}); err != nil {
		t.Fatalf("AddTags error: %v", err)
	}

	h, _ := r.Get("web1")
	if len(h.Tags) != 3 {
		t.Errorf("tags count: got %d, want 3", len(h.Tags))
	}
}

func TestAddTags_Duplicates(t *testing.T) {
	r := newTestRegistry(t)
	r.Add("web1", "10.0.0.1", "", []string{"web", "prod"})

	if err := r.AddTags("web1", []string{"web", "staging"}); err != nil {
		t.Fatalf("AddTags error: %v", err)
	}

	h, _ := r.Get("web1")
	// Should not add "web" again
	webCount := 0
	for _, tag := range h.Tags {
		if tag == "web" {
			webCount++
		}
	}
	if webCount != 1 {
		t.Errorf("tag 'web' appears %d times, want 1", webCount)
	}
}

func TestAddTags_NotFound(t *testing.T) {
	r := newTestRegistry(t)
	err := r.AddTags("nonexistent", []string{"tag"})
	if err == nil {
		t.Error("AddTags should return error for non-existent host")
	}
}

func TestRemoveTags(t *testing.T) {
	r := newTestRegistry(t)
	r.Add("web1", "10.0.0.1", "", []string{"web", "prod", "critical"})

	if err := r.RemoveTags("web1", []string{"prod"}); err != nil {
		t.Fatalf("RemoveTags error: %v", err)
	}

	h, _ := r.Get("web1")
	if len(h.Tags) != 2 {
		t.Errorf("tags count: got %d, want 2", len(h.Tags))
	}
	for _, tag := range h.Tags {
		if tag == "prod" {
			t.Error("tag 'prod' should have been removed")
		}
	}
}

func TestRemoveTags_NotFound(t *testing.T) {
	r := newTestRegistry(t)
	err := r.RemoveTags("nonexistent", []string{"tag"})
	if err == nil {
		t.Error("RemoveTags should return error for non-existent host")
	}
}

func TestAll_SortedByName(t *testing.T) {
	r := newTestRegistry(t)
	r.Add("zebra", "10.0.0.3", "", nil)
	r.Add("alpha", "10.0.0.1", "", nil)
	r.Add("middle", "10.0.0.2", "", nil)

	hosts := r.All()
	if len(hosts) != 3 {
		t.Fatalf("expected 3 hosts, got %d", len(hosts))
	}
	if hosts[0].Name != "alpha" || hosts[1].Name != "middle" || hosts[2].Name != "zebra" {
		t.Errorf("not sorted: got %s, %s, %s", hosts[0].Name, hosts[1].Name, hosts[2].Name)
	}
}

func TestByTagsAND(t *testing.T) {
	r := newTestRegistry(t)
	r.Add("web1", "10.0.0.1", "", []string{"web", "prod"})
	r.Add("web2", "10.0.0.2", "", []string{"web", "staging"})
	r.Add("db1", "10.0.0.3", "", []string{"db", "prod"})

	// AND: web + prod → only web1
	hosts := r.ByTagsAND([]string{"web", "prod"})
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}
	if hosts[0].Name != "web1" {
		t.Errorf("expected web1, got %s", hosts[0].Name)
	}
}

func TestByTagsAND_NoMatch(t *testing.T) {
	r := newTestRegistry(t)
	r.Add("web1", "10.0.0.1", "", []string{"web"})

	hosts := r.ByTagsAND([]string{"web", "nonexistent"})
	if len(hosts) != 0 {
		t.Errorf("expected 0 hosts, got %d", len(hosts))
	}
}

func TestByTagsAND_EmptyTags(t *testing.T) {
	r := newTestRegistry(t)
	r.Add("web1", "10.0.0.1", "", []string{"web"})

	hosts := r.ByTagsAND(nil)
	if len(hosts) != 1 {
		t.Error("empty tags should return all hosts")
	}
}

func TestByTagsAND_SingleTag(t *testing.T) {
	r := newTestRegistry(t)
	r.Add("web1", "10.0.0.1", "", []string{"web", "prod"})
	r.Add("web2", "10.0.0.2", "", []string{"web", "staging"})
	r.Add("db1", "10.0.0.3", "", []string{"db", "prod"})

	hosts := r.ByTagsAND([]string{"prod"})
	if len(hosts) != 2 {
		t.Errorf("expected 2 hosts, got %d", len(hosts))
	}
}

func TestByTagsOR(t *testing.T) {
	r := newTestRegistry(t)
	r.Add("web1", "10.0.0.1", "", []string{"web"})
	r.Add("db1", "10.0.0.2", "", []string{"db"})
	r.Add("cache1", "10.0.0.3", "", []string{"cache"})

	hosts := r.ByTagsOR([]string{"web", "db"})
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(hosts))
	}
}

func TestByTagsOR_Dedup(t *testing.T) {
	r := newTestRegistry(t)
	r.Add("web1", "10.0.0.1", "", []string{"web", "prod"})

	// web1 matches both tags, but should appear only once
	hosts := r.ByTagsOR([]string{"web", "prod"})
	if len(hosts) != 1 {
		t.Errorf("expected 1 host (dedup), got %d", len(hosts))
	}
}

func TestByTagsOR_EmptyTags(t *testing.T) {
	r := newTestRegistry(t)
	r.Add("web1", "10.0.0.1", "", nil)

	hosts := r.ByTagsOR(nil)
	if len(hosts) != 1 {
		t.Error("empty tags should return all hosts")
	}
}

func TestByTagsOR_NoMatch(t *testing.T) {
	r := newTestRegistry(t)
	r.Add("web1", "10.0.0.1", "", []string{"web"})

	hosts := r.ByTagsOR([]string{"nonexistent"})
	if len(hosts) != 0 {
		t.Errorf("expected 0 hosts, got %d", len(hosts))
	}
}

func TestSaveAndReload(t *testing.T) {
	dir := t.TempDir()

	// Create registry and add hosts
	r1, _ := NewRegistry(dir)
	r1.Add("web1", "10.0.0.1", "admin", []string{"web", "prod"})
	r1.Add("db1", "10.0.0.2", "", []string{"db"})
	if err := r1.Save(); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filepath.Join(dir, HostsFileName)); err != nil {
		t.Fatalf("hosts.yaml not created: %v", err)
	}

	// Reload
	r2, err := NewRegistry(dir)
	if err != nil {
		t.Fatalf("NewRegistry reload error: %v", err)
	}

	if len(r2.All()) != 2 {
		t.Fatalf("expected 2 hosts after reload, got %d", len(r2.All()))
	}

	h, ok := r2.Get("web1")
	if !ok {
		t.Fatal("web1 not found after reload")
	}
	if h.Address != "10.0.0.1" {
		t.Errorf("address: got %q, want %q", h.Address, "10.0.0.1")
	}
	if h.User != "admin" {
		t.Errorf("user: got %q, want %q", h.User, "admin")
	}

	// Tag index should work after reload
	hosts := r2.ByTagsAND([]string{"web", "prod"})
	if len(hosts) != 1 || hosts[0].Name != "web1" {
		t.Error("tag index not rebuilt correctly after reload")
	}
}

func TestTagIndexRebuild(t *testing.T) {
	r := newTestRegistry(t)
	r.Add("web1", "10.0.0.1", "", []string{"web", "prod"})
	r.Add("web2", "10.0.0.2", "", []string{"web", "staging"})

	// Remove web1 → tag index should update
	r.Remove("web1")

	hosts := r.ByTagsAND([]string{"web"})
	if len(hosts) != 1 || hosts[0].Name != "web2" {
		t.Error("tag index not updated after remove")
	}

	prodHosts := r.ByTagsAND([]string{"prod"})
	if len(prodHosts) != 0 {
		t.Error("prod tag should have no hosts after removing web1")
	}
}

package keys

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFileNames(t *testing.T) {
	openssh, pem, ppk := FileNames("id_sogark")
	if openssh != "id_sogark" {
		t.Errorf("openssh: got %q, want %q", openssh, "id_sogark")
	}
	if pem != "id_sogark.pem" {
		t.Errorf("pem: got %q, want %q", pem, "id_sogark.pem")
	}
	if ppk != "id_sogark.ppk" {
		t.Errorf("ppk: got %q, want %q", ppk, "id_sogark.ppk")
	}
}

func TestSave_AllFormats(t *testing.T) {
	dir := t.TempDir()
	parsed := &Parsed{
		OpenSSH: "openssh-key-content\n",
		PEM:     "pem-key-content\n",
		PPK:     "ppk-key-content\n",
	}

	results, err := Save(parsed, dir, "testkey", []string{"OpenSSH", "PEM", "PPK"})
	if err != nil {
		t.Fatalf("Save error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Verify files exist and have correct content
	checkFile(t, filepath.Join(dir, "testkey"), "openssh-key-content\n")
	checkFile(t, filepath.Join(dir, "testkey.pem"), "pem-key-content\n")
	checkFile(t, filepath.Join(dir, "testkey.ppk"), "ppk-key-content\n")
}

func TestSave_FilteredFormats(t *testing.T) {
	dir := t.TempDir()
	parsed := &Parsed{
		OpenSSH: "openssh-key\n",
		PEM:     "pem-key\n",
		PPK:     "ppk-key\n",
	}

	results, err := Save(parsed, dir, "testkey", []string{"PEM"})
	if err != nil {
		t.Fatalf("Save error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Format != "PEM" {
		t.Errorf("format: got %q, want %q", results[0].Format, "PEM")
	}

	// OpenSSH and PPK files should NOT exist
	if _, err := os.Stat(filepath.Join(dir, "testkey")); err == nil {
		t.Error("OpenSSH file should not exist when not in formats")
	}
	if _, err := os.Stat(filepath.Join(dir, "testkey.ppk")); err == nil {
		t.Error("PPK file should not exist when not in formats")
	}
}

func TestSave_CaseInsensitiveFormats(t *testing.T) {
	dir := t.TempDir()
	parsed := &Parsed{OpenSSH: "key\n"}

	results, err := Save(parsed, dir, "testkey", []string{"openssh"})
	if err != nil {
		t.Fatalf("Save error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestSave_EmptyKey(t *testing.T) {
	dir := t.TempDir()
	parsed := &Parsed{
		OpenSSH: "", // no OpenSSH key
		PEM:     "pem-key\n",
	}

	results, err := Save(parsed, dir, "testkey", []string{"OpenSSH", "PEM"})
	if err != nil {
		t.Fatalf("Save error: %v", err)
	}
	// Only PEM should be saved
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestSave_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "subdir", "keys")
	parsed := &Parsed{OpenSSH: "key\n"}

	_, err := Save(parsed, dir, "testkey", []string{"OpenSSH"})
	if err != nil {
		t.Fatalf("Save error: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("not a directory")
	}
}

func TestSave_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	parsed := &Parsed{OpenSSH: "key\n"}

	_, err := Save(parsed, dir, "testkey", []string{"OpenSSH"})
	if err != nil {
		t.Fatalf("Save error: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, "testkey"))
	if err != nil {
		t.Fatal(err)
	}
	perm := info.Mode().Perm()
	if runtime.GOOS == "windows" {
		if perm == 0 {
			t.Errorf("file permissions should not be zero on Windows")
		}
		return
	}
	if perm != 0600 {
		t.Errorf("file permissions: got %o, want 0600", perm)
	}
}

func TestClean(t *testing.T) {
	dir := t.TempDir()

	// Create key files
	os.WriteFile(filepath.Join(dir, "testkey"), []byte("key"), 0600)
	os.WriteFile(filepath.Join(dir, "testkey.pem"), []byte("key"), 0600)
	os.WriteFile(filepath.Join(dir, "testkey.ppk"), []byte("key"), 0600)
	os.WriteFile(filepath.Join(dir, ".key_timestamp"), []byte("123"), 0600)

	removed, err := Clean(dir, "testkey")
	if err != nil {
		t.Fatalf("Clean error: %v", err)
	}

	if len(removed) != 4 {
		t.Errorf("expected 4 removed, got %d: %v", len(removed), removed)
	}

	// Verify all files are gone
	for _, name := range []string{"testkey", "testkey.pem", "testkey.ppk", ".key_timestamp"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("file %s should have been removed", name)
		}
	}
}

func TestClean_PartialFiles(t *testing.T) {
	dir := t.TempDir()

	// Only create OpenSSH key
	os.WriteFile(filepath.Join(dir, "testkey"), []byte("key"), 0600)

	removed, err := Clean(dir, "testkey")
	if err != nil {
		t.Fatalf("Clean error: %v", err)
	}

	if len(removed) != 1 {
		t.Errorf("expected 1 removed, got %d: %v", len(removed), removed)
	}
}

func TestClean_NoFiles(t *testing.T) {
	dir := t.TempDir()

	removed, err := Clean(dir, "testkey")
	if err != nil {
		t.Fatalf("Clean error: %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("expected 0 removed, got %d", len(removed))
	}
}

func checkFile(t *testing.T, path, expected string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if string(data) != expected {
		t.Errorf("%s content: got %q, want %q", path, string(data), expected)
	}
}

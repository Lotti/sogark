package keys

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestSaveTimestamp(t *testing.T) {
	dir := t.TempDir()

	before := time.Now().Unix()
	if err := SaveTimestamp(dir); err != nil {
		t.Fatalf("SaveTimestamp error: %v", err)
	}
	after := time.Now().Unix()

	data, err := os.ReadFile(filepath.Join(dir, ".key_timestamp"))
	if err != nil {
		t.Fatalf("reading timestamp: %v", err)
	}

	ts, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		t.Fatalf("parsing timestamp: %v", err)
	}

	if ts < before || ts > after {
		t.Errorf("timestamp %d not in range [%d, %d]", ts, before, after)
	}
}

func TestIsValid_ValidKey(t *testing.T) {
	dir := t.TempDir()

	// Create key file and timestamp
	os.WriteFile(filepath.Join(dir, "id_test"), []byte("key"), 0600)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	os.WriteFile(filepath.Join(dir, ".key_timestamp"), []byte(ts), 0600)

	valid, remaining, err := IsValid(dir, "id_test", 4)
	if err != nil {
		t.Fatalf("IsValid error: %v", err)
	}
	if !valid {
		t.Error("key should be valid")
	}
	if remaining <= 0 || remaining > 4*time.Hour {
		t.Errorf("remaining time unexpected: %v", remaining)
	}
}

func TestIsValid_ExpiredKey(t *testing.T) {
	dir := t.TempDir()

	// Create key file and old timestamp (5 hours ago)
	os.WriteFile(filepath.Join(dir, "id_test"), []byte("key"), 0600)
	ts := strconv.FormatInt(time.Now().Add(-5*time.Hour).Unix(), 10)
	os.WriteFile(filepath.Join(dir, ".key_timestamp"), []byte(ts), 0600)

	valid, _, err := IsValid(dir, "id_test", 4)
	if err != nil {
		t.Fatalf("IsValid error: %v", err)
	}
	if valid {
		t.Error("key should be expired")
	}
}

func TestIsValid_NoKeyFile(t *testing.T) {
	dir := t.TempDir()
	// No key file at all

	valid, _, err := IsValid(dir, "id_test", 4)
	if err != nil {
		t.Fatalf("IsValid error: %v", err)
	}
	if valid {
		t.Error("key should not be valid without key file")
	}
}

func TestIsValid_NoTimestamp(t *testing.T) {
	dir := t.TempDir()

	// Key file exists but no timestamp
	os.WriteFile(filepath.Join(dir, "id_test"), []byte("key"), 0600)

	valid, _, err := IsValid(dir, "id_test", 4)
	if err != nil {
		t.Fatalf("IsValid error: %v", err)
	}
	if valid {
		t.Error("key should not be valid without timestamp")
	}
}

func TestIsValid_InvalidTimestamp(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "id_test"), []byte("key"), 0600)
	os.WriteFile(filepath.Join(dir, ".key_timestamp"), []byte("not-a-number"), 0600)

	valid, _, err := IsValid(dir, "id_test", 4)
	if err != nil {
		t.Fatalf("IsValid error: %v", err)
	}
	if valid {
		t.Error("key should not be valid with invalid timestamp")
	}
}

func TestIsValid_AlmostExpired(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "id_test"), []byte("key"), 0600)
	// Set timestamp to 3h59m ago (should still be valid with 4h TTL)
	ts := strconv.FormatInt(time.Now().Add(-3*time.Hour-59*time.Minute).Unix(), 10)
	os.WriteFile(filepath.Join(dir, ".key_timestamp"), []byte(ts), 0600)

	valid, remaining, err := IsValid(dir, "id_test", 4)
	if err != nil {
		t.Fatalf("IsValid error: %v", err)
	}
	if !valid {
		t.Error("key should still be valid")
	}
	if remaining > time.Minute {
		t.Errorf("remaining should be less than 1 minute, got %v", remaining)
	}
}

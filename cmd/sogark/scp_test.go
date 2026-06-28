package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Lotti/sogark/internal/hosts"
)

func TestParseScpFlags_Basic(t *testing.T) {
	sf, err := parseScpFlags([]string{"file.txt", "host:/tmp/"})
	if err != nil {
		t.Fatal(err)
	}
	if sf.keyFormat != "openssh" {
		t.Errorf("keyFormat = %q, want %q", sf.keyFormat, "openssh")
	}
	if sf.dryRun || sf.forceLogin {
		t.Error("flags should be false by default")
	}
	if len(sf.passArgs) != 2 {
		t.Errorf("passArgs = %v, want 2 items", sf.passArgs)
	}
}

func TestParseScpFlags_AllFlags(t *testing.T) {
	sf, err := parseScpFlags([]string{
		"--dry-run", "--force-login", "-u", "root",
		"--key-format", "pem", "--tag", "web",
		"file.txt", "host:/tmp/",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sf.dryRun {
		t.Error("dryRun should be true")
	}
	if !sf.forceLogin {
		t.Error("forceLogin should be true")
	}
	if sf.user != "root" {
		t.Errorf("user = %q, want %q", sf.user, "root")
	}
	if sf.keyFormat != "pem" {
		t.Errorf("keyFormat = %q, want %q", sf.keyFormat, "pem")
	}
	if sf.tag != "web" {
		t.Errorf("tag = %q, want %q", sf.tag, "web")
	}
}

func TestParseScpFlags_AnyTag(t *testing.T) {
	sf, err := parseScpFlags([]string{"--any-tag", "web,db", "file.txt", "host:/tmp/"})
	if err != nil {
		t.Fatal(err)
	}
	if sf.anyTag != "web,db" {
		t.Errorf("anyTag = %q, want %q", sf.anyTag, "web,db")
	}
}

func TestParseScpFlags_EqualsForm(t *testing.T) {
	sf, err := parseScpFlags([]string{"--user=oper1", "--key-format=ppk", "--tag=prod", "--any-tag=web"})
	if err != nil {
		t.Fatal(err)
	}
	if sf.user != "oper1" {
		t.Errorf("user = %q, want %q", sf.user, "oper1")
	}
	if sf.keyFormat != "ppk" {
		t.Errorf("keyFormat = %q, want %q", sf.keyFormat, "ppk")
	}
	if sf.tag != "prod" {
		t.Errorf("tag = %q, want %q", sf.tag, "prod")
	}
	if sf.anyTag != "web" {
		t.Errorf("anyTag = %q, want %q", sf.anyTag, "web")
	}
}

func TestParseScpFlags_Separator(t *testing.T) {
	sf, err := parseScpFlags([]string{"--dry-run", "--", "-r", "dir/", "host:/tmp/"})
	if err != nil {
		t.Fatal(err)
	}
	if !sf.dryRun {
		t.Error("dryRun should be true")
	}
	if len(sf.passArgs) != 3 || sf.passArgs[0] != "-r" {
		t.Errorf("passArgs = %v, want [-r dir/ host:/tmp/]", sf.passArgs)
	}
}

func TestParseScpFlags_MissingValue(t *testing.T) {
	for _, flag := range []string{"-u", "--user", "--key-format", "--tag", "--any-tag"} {
		_, err := parseScpFlags([]string{flag})
		if err == nil {
			t.Errorf("expected error for %s without value", flag)
		}
	}
}

func TestParseScpFlags_Help(t *testing.T) {
	_, err := parseScpFlags([]string{"--help"})
	if err == nil || err.Error() != "help" {
		t.Errorf("expected help error, got %v", err)
	}
}

func TestResolveScpArgs_NilRegistry(t *testing.T) {
	args := []string{"file.txt", "host:/tmp/"}
	got := resolveScpArgs(args, nil, "root")
	if len(got) != 2 || got[0] != "file.txt" || got[1] != "host:/tmp/" {
		t.Errorf("got %v, want unchanged args", got)
	}
}

func TestResolveScpArgs_WithRegistry(t *testing.T) {
	dir := t.TempDir()
	hostsFile := filepath.Join(dir, "hosts.yaml")
	os.WriteFile(hostsFile, []byte("hosts:\n  web1:\n    address: \"10.0.0.1\"\n    user: root\n"), 0600)

	reg, err := hosts.NewRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}

	args := []string{"file.txt", "web1:/tmp/"}
	got := resolveScpArgs(args, reg, "oper1")
	if got[1] != "root@10.0.0.1:/tmp/" {
		t.Errorf("got %q, want %q", got[1], "root@10.0.0.1:/tmp/")
	}
}

func TestResolveScpArgs_WithUserPrefix(t *testing.T) {
	dir := t.TempDir()
	hostsFile := filepath.Join(dir, "hosts.yaml")
	os.WriteFile(hostsFile, []byte("hosts:\n  web1:\n    address: \"10.0.0.1\"\n"), 0600)

	reg, err := hosts.NewRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}

	args := []string{"file.txt", "admin@web1:/tmp/"}
	got := resolveScpArgs(args, reg, "")
	if got[1] != "admin@10.0.0.1:/tmp/" {
		t.Errorf("got %q, want %q", got[1], "admin@10.0.0.1:/tmp/")
	}
}

func TestResolveScpArgs_UnknownHost(t *testing.T) {
	dir := t.TempDir()
	hostsFile := filepath.Join(dir, "hosts.yaml")
	os.WriteFile(hostsFile, []byte("hosts:\n  web1:\n    address: \"10.0.0.1\"\n"), 0600)

	reg, err := hosts.NewRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}

	args := []string{"file.txt", "unknown:/tmp/"}
	got := resolveScpArgs(args, reg, "root")
	if got[1] != "unknown:/tmp/" {
		t.Errorf("got %q, want %q (unchanged)", got[1], "unknown:/tmp/")
	}
}

func TestResolveScpArgs_FlagArgs(t *testing.T) {
	dir := t.TempDir()
	reg, _ := hosts.NewRegistry(dir)

	args := []string{"-r", "dir/", "host:/tmp/"}
	got := resolveScpArgs(args, reg, "root")
	if got[0] != "-r" {
		t.Errorf("got[0] = %q, want %q (flag preserved)", got[0], "-r")
	}
}

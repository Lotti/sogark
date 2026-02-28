package main

import (
	"testing"
)

func TestParseTagArg(t *testing.T) {
	tests := []struct {
		input    string
		wantUser string
		wantTags []string
		wantOK   bool
	}{
		{"#web", "", []string{"web"}, true},
		{"#web#prod", "", []string{"web", "prod"}, true},
		{"#web#prod#eu", "", []string{"web", "prod", "eu"}, true},
		{"oper1@#web", "oper1", []string{"web"}, true},
		{"oper1@#web#prod", "oper1", []string{"web", "prod"}, true},
		{"host1", "", nil, false},
		{"10.1.2.3", "", nil, false},
		{"user@host", "", nil, false},
		{"#", "", nil, false},
		{"", "", nil, false},
	}

	for _, tt := range tests {
		user, tags, ok := parseTagArg(tt.input)
		if ok != tt.wantOK {
			t.Errorf("parseTagArg(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			continue
		}
		if !ok {
			continue
		}
		if user != tt.wantUser {
			t.Errorf("parseTagArg(%q) user = %q, want %q", tt.input, user, tt.wantUser)
		}
		if len(tags) != len(tt.wantTags) {
			t.Errorf("parseTagArg(%q) tags = %v, want %v", tt.input, tags, tt.wantTags)
			continue
		}
		for i := range tags {
			if tags[i] != tt.wantTags[i] {
				t.Errorf("parseTagArg(%q) tags[%d] = %q, want %q", tt.input, i, tags[i], tt.wantTags[i])
			}
		}
	}
}

func TestExtractScpTagArgs_Upload(t *testing.T) {
	// sogark scp file.txt oper1@#web#prod:/tmp/
	args := []string{"file.txt", "oper1@#web#prod:/tmp/"}
	newArgs, tag, user, downloadDir := extractScpTagArgs(args, "")

	if tag != "web,prod" {
		t.Errorf("tag = %q, want %q", tag, "web,prod")
	}
	if user != "oper1" {
		t.Errorf("user = %q, want %q", user, "oper1")
	}
	if downloadDir != "" {
		t.Errorf("downloadDir = %q, want empty (upload)", downloadDir)
	}
	if newArgs[0] != "file.txt" {
		t.Errorf("newArgs[0] = %q, want %q", newArgs[0], "file.txt")
	}
	if newArgs[1] != ":/tmp/" {
		t.Errorf("newArgs[1] = %q, want %q", newArgs[1], ":/tmp/")
	}
}

func TestExtractScpTagArgs_Download(t *testing.T) {
	// sogark scp #webservers:/etc/hosts ./configs/
	args := []string{"#webservers:/etc/hosts", "./configs/"}
	newArgs, tag, user, downloadDir := extractScpTagArgs(args, "")

	if tag != "webservers" {
		t.Errorf("tag = %q, want %q", tag, "webservers")
	}
	if user != "" {
		t.Errorf("user = %q, want empty", user)
	}
	if downloadDir != "./configs/" {
		t.Errorf("downloadDir = %q, want %q", downloadDir, "./configs/")
	}
	if newArgs[0] != ":/etc/hosts" {
		t.Errorf("newArgs[0] = %q, want %q", newArgs[0], ":/etc/hosts")
	}
}

func TestExtractScpTagArgs_NoTag(t *testing.T) {
	args := []string{"file.txt", "10.1.2.3:/tmp/"}
	_, tag, _, _ := extractScpTagArgs(args, "")

	if tag != "" {
		t.Errorf("tag = %q, want empty (no tag)", tag)
	}
}

func TestExtractScpTagArgs_WithFlags(t *testing.T) {
	// sogark scp -r ./mydir oper1@#prod:/opt/
	args := []string{"-r", "./mydir", "oper1@#prod:/opt/"}
	newArgs, tag, user, downloadDir := extractScpTagArgs(args, "")

	if tag != "prod" {
		t.Errorf("tag = %q, want %q", tag, "prod")
	}
	if user != "oper1" {
		t.Errorf("user = %q, want %q", user, "oper1")
	}
	if downloadDir != "" {
		t.Errorf("downloadDir = %q, want empty (upload)", downloadDir)
	}
	if newArgs[2] != ":/opt/" {
		t.Errorf("newArgs[2] = %q, want %q", newArgs[2], ":/opt/")
	}
}

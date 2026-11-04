package main

import (
	"strings"
	"testing"

	"github.com/Lotti/sogark/internal/config"
)

func TestExecRejectsInvalidCommandBeforeLogin(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"app01", "id"}, "separate"},
		{[]string{"app01", "--", "id\nhostname"}, "single-line"},
		{[]string{"app01", "--prompt", ".*", "--", "id"}, "prompt expression"},
		{[]string{"app01", "--timeout", "0s", "--", "id"}, "positive"},
	} {
		cmd := newExecCmd()
		cmd.SilenceErrors, cmd.SilenceUsage = true, true
		cmd.SetArgs(test.args)
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("args=%v error=%v, want %q", test.args, err, test.want)
		}
	}
}

func TestExecResolvesExplicitUser(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	isolatedConfigPath(t, "")
	cfg := config.Defaults()
	cfg.DefaultSSHUser = "operator"
	target, err := resolveExecTarget(&cfg, "root@app01")
	if err != nil || target.TargetUser != "root" || target.Address != "app01" {
		t.Fatalf("target=%v error=%v", target, err)
	}
}

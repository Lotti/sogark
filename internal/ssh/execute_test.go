package ssh

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestExecutePOSIXShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell integration runs on Unix")
	}
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	connection := ConnectArgs{}
	options := ExecuteOptions{Prompt: DefaultPromptPattern, ConnectTimeout: time.Second, CommandTimeout: time.Second}
	err = connection.execute(context.Background(), `printf 'first\nsecond\n'; exit 7`, options, &output,
		func(ctx context.Context, _ []string) *exec.Cmd {
			return exec.CommandContext(ctx, shell, "-c", `printf '[root@fixture]# '; exec sh`)
		})
	var status *CommandExitError
	if !errors.As(err, &status) || status.Code != 7 || output.String() != "first\nsecond\n" {
		t.Fatalf("output=%q status=%v", output.String(), err)
	}
}

func TestExecuteInteractiveShell(t *testing.T) {
	for _, mode := range []string{"success", "intermediate-prompt", "failure", "connect-timeout", "command-timeout"} {
		t.Run(mode, func(t *testing.T) {
			options := ExecuteOptions{Prompt: DefaultPromptPattern, ConnectTimeout: 5 * time.Second, CommandTimeout: 5 * time.Second}
			if mode == "connect-timeout" {
				options.ConnectTimeout = 50 * time.Millisecond
			}
			if mode == "command-timeout" {
				options.CommandTimeout = 50 * time.Millisecond
			}
			connection := ConnectArgs{Username: "user", TargetUser: "root", Host: "app01", ProxyHost: "psmp.example.com", KeyPath: "/fixture"}
			var output bytes.Buffer
			err := connection.execute(context.Background(), "printf 'hello'", options, &output, func(ctx context.Context, args []string) *exec.Cmd {
				want := []string{"ssh", "-tt", "user@root@app01@psmp.example.com", "-i", "/fixture", "-o", "IdentitiesOnly=yes", "-o", "StrictHostKeyChecking=accept-new"}
				if !reflect.DeepEqual(args, want) {
					t.Errorf("must request a PTY shell without a remote command: %v", args)
				}
				child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestInteractiveSSHHelper$")
				child.Env = append(os.Environ(), "SOGARK_SSH_HELPER="+mode)
				return child
			})
			switch mode {
			case "success", "intermediate-prompt":
				if err != nil || output.String() != "hello\n[root@fake]# inside output\n" {
					t.Fatalf("output=%q error=%v", output.String(), err)
				}
			case "failure":
				var status *CommandExitError
				if !errors.As(err, &status) || status.Code != 7 {
					t.Fatalf("expected remote status 7, got %v", err)
				}
			default:
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("expected bounded timeout, got %v", err)
				}
			}
		})
	}
}

func TestInteractiveSSHHelper(t *testing.T) {
	mode := os.Getenv("SOGARK_SSH_HELPER")
	if mode == "" {
		return
	}
	if mode == "connect-timeout" {
		time.Sleep(time.Minute)
		os.Exit(1)
	}
	fmt.Print("Proxy banner\r\n\x1b[32m[root@")
	time.Sleep(10 * time.Millisecond)
	fmt.Print("fake]# \x1b[0m")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		os.Exit(2)
	}
	readyMatch := regexp.MustCompile(`'([0-9a-f]{32})'`).FindStringSubmatch(line)
	if len(readyMatch) != 2 {
		os.Exit(5)
	}
	if mode == "intermediate-prompt" {
		fmt.Print("exec su - root\r\nPassword:\r\n[root@target]# ")
		line, err = reader.ReadString('\n')
		if err != nil || !strings.Contains(line, "SOGARK_%s_ready") {
			os.Exit(7)
		}
	}
	fmt.Printf("SOGARK_%s_ready\r\n", readyMatch[1])
	line, err = reader.ReadString('\n')
	if err != nil {
		os.Exit(6)
	}
	tokenMatch := regexp.MustCompile(`'([0-9a-f]{32})'`).FindStringSubmatch(line)
	if len(tokenMatch) != 2 || !strings.Contains(line, `sh -c 'printf '"'"'hello'"'"''`) {
		os.Exit(3)
	}
	// Echo contains escaped markers, whereas completion uses control bytes.
	fmt.Print(line)
	if mode == "command-timeout" {
		time.Sleep(time.Minute)
		os.Exit(1)
	}
	token := tokenMatch[1]
	fmt.Printf("SOGARK_%s_start\r\nhello\r\n[root@fake]# inside output\r\n", token)
	code := 0
	if mode == "failure" {
		code = 7
	}
	fmt.Printf("SOGARK_%s_end_%d", token, code)
	time.Sleep(10 * time.Millisecond)
	fmt.Print("_DONE\r\n[root@fake]# ")
	line, _ = reader.ReadString('\n')
	if line != "exit\n" {
		os.Exit(4)
	}
	os.Exit(0)
}

func TestValidateExecutionBeforeStartingSSH(t *testing.T) {
	options := ExecuteOptions{Prompt: DefaultPromptPattern, ConnectTimeout: time.Second, CommandTimeout: time.Second}
	for _, command := range []string{"", "id\nhostname", "id\rhostname", "id\x00", strings.Repeat("x", 2049)} {
		if err := ValidateExecution(command, options); err == nil {
			t.Errorf("accepted invalid command %q", command)
		}
	}
	options.Prompt = ".*"
	if err := ValidateExecution("id", options); err == nil {
		t.Fatal("empty-matching prompt accepted")
	}
	options.Prompt = "["
	if err := ValidateExecution("id", options); err == nil {
		t.Fatal("invalid prompt accepted")
	}
	options.Prompt = DefaultPromptPattern
	options.CommandTimeout = 0
	if err := ValidateExecution("id", options); err == nil {
		t.Fatal("unbounded timeout accepted")
	}
}

func TestShellResultSplitMarkersAndLimit(t *testing.T) {
	events := make(chan shellChunk, 6)
	for _, part := range []string{"banner\r\nSOGARK_to", "ken_start\r", "\noutput\r\n", "SOGARK_token_end_0", "_DONE"} {
		events <- shellChunk{data: part}
	}
	close(events)
	stream := shellStream{events: events}
	output, code, err := stream.awaitResult(context.Background(), "token")
	if err != nil || output != "output\n" || code != 0 {
		t.Fatalf("output=%q code=%d err=%v", output, code, err)
	}
	events = make(chan shellChunk, 1)
	events <- shellChunk{data: strings.Repeat("x", maxExecOutput+1)}
	stream = shellStream{events: events}
	if err := stream.next(context.Background()); err == nil {
		t.Fatal("unbounded shell output accepted")
	}
}

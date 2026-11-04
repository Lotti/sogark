package ssh

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const DefaultPromptPattern = `(?m)(?:\[[^\r\n]*@[^\r\n]*\]|[^\r\n]*@[^\r\n]*)[#$] *$`
const maxExecOutput = 4 << 20

var terminalEscape = regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07]*(?:\x07|\x1b\\))`)

type ExecuteOptions struct {
	Prompt         string
	ConnectTimeout time.Duration
	CommandTimeout time.Duration
}

type CommandExitError struct {
	Code int
}

func (e *CommandExitError) Error() string {
	return fmt.Sprintf("remote command exited with status %d", e.Code)
}

// Execute sends a command through a shell session, never an SSH exec request.
func (a *ConnectArgs) Execute(ctx context.Context, command string, options ExecuteOptions, output io.Writer) error {
	return a.execute(ctx, command, options, output, func(ctx context.Context, args []string) *exec.Cmd {
		name := args[0]
		if runtime.GOOS == "windows" {
			if path, err := exec.LookPath("ssh.exe"); err == nil {
				name = path
			}
		}
		return exec.CommandContext(ctx, name, args[1:]...)
	})
}

func ValidateExecution(command string, options ExecuteOptions) error {
	if command == "" || len(command) > 2048 || strings.ContainsAny(command, "\x00\r\n") {
		return errors.New("provide a nonempty, single-line POSIX shell command of at most 2048 bytes")
	}
	if len(shellQuote(command)) > 3500 {
		return errors.New("quoted command exceeds the interactive terminal line limit")
	}
	if options.ConnectTimeout <= 0 || options.CommandTimeout <= 0 {
		return errors.New("execution timeouts must be positive")
	}
	prompt, err := regexp.Compile(options.Prompt)
	if err != nil || prompt.MatchString("") {
		return errors.New("provide a valid prompt expression that does not match empty text")
	}
	return nil
}

func (a *ConnectArgs) execute(ctx context.Context, command string, options ExecuteOptions, output io.Writer, create func(context.Context, []string) *exec.Cmd) error {
	if err := ValidateExecution(command, options); err != nil {
		return err
	}
	prompt := regexp.MustCompile(options.Prompt)
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	token := hex.EncodeToString(nonce[:])
	args := a.CommandLine()
	// -tt allocates a remote PTY even though the local streams are pipes.
	args = append([]string{args[0], "-tt", "-o", "BatchMode=yes"}, args[1:]...)
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	child := create(childCtx, args)
	reader, writer, err := os.Pipe()
	if err != nil {
		return err
	}
	defer reader.Close()
	defer writer.Close()
	child.Stdout, child.Stderr = writer, writer
	stdin, err := child.StdinPipe()
	if err != nil {
		return err
	}
	defer stdin.Close()
	if err := child.Start(); err != nil {
		return fmt.Errorf("start interactive SSH: %w", err)
	}
	writer.Close()
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	waited := false
	defer func() {
		cancel()
		reader.Close()
		if !waited {
			<-done
		}
	}()
	events := make(chan shellChunk)
	go readShell(childCtx, reader, events)
	session := shellStream{events: events}
	connectCtx, stopConnect := context.WithTimeout(ctx, options.ConnectTimeout)
	defer stopConnect()
	if err := session.awaitPrompt(connectCtx, prompt); err != nil {
		return fmt.Errorf("wait for interactive SSH prompt: %w", err)
	}
	// Literal control bytes only appear in actual printf output, not PTY echo.
	line := "stty -echo; printf '\\036" + token + ":start\\037\\n'; " +
		"TERM=dumb PAGER=cat SYSTEMD_PAGER=cat sh -c " + shellQuote(command) +
		"; __sogark_rc=$?; printf '\\036" + token + ":%s\\037\\n' \"$__sogark_rc\"\n"
	if _, err := io.WriteString(stdin, line); err != nil {
		return fmt.Errorf("send shell command: %w", err)
	}
	commandCtx, stopCommand := context.WithTimeout(ctx, options.CommandTimeout)
	defer stopCommand()
	result, code, err := session.awaitResult(commandCtx, token)
	if err != nil {
		return fmt.Errorf("wait for shell command: %w; command may still have run", err)
	}
	if _, err := io.WriteString(output, result); err != nil {
		return err
	}
	_, _ = io.WriteString(stdin, "exit\n")
	stdin.Close()
	// Give SSH a bounded chance to close the remote shell before killing it.
	select {
	case <-done:
		waited = true
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
	}
	if code != 0 {
		return &CommandExitError{Code: code}
	}
	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

type shellChunk struct {
	data string
	err  error
}

func readShell(ctx context.Context, reader io.Reader, events chan<- shellChunk) {
	defer close(events)
	buffer := make([]byte, 8192)
	for {
		n, err := reader.Read(buffer)
		select {
		case events <- shellChunk{data: string(buffer[:n]), err: err}:
		case <-ctx.Done():
			return
		}
		if err != nil {
			return
		}
	}
}

type shellStream struct {
	events <-chan shellChunk
	buffer string
}

func (s *shellStream) next(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case chunk, ok := <-s.events:
		if !ok {
			return io.EOF
		}
		s.buffer += chunk.data
		if len(s.buffer) > maxExecOutput {
			return errors.New("interactive session exceeds 4 MiB output limit")
		}
		if chunk.data == "" {
			return chunk.err
		}
		return nil
	}
}

func (s *shellStream) awaitPrompt(ctx context.Context, prompt *regexp.Regexp) error {
	for {
		text := terminalEscape.ReplaceAllString(s.buffer, "")
		text = strings.ReplaceAll(text, "\r", "")
		if prompt.MatchString(text) {
			s.buffer = ""
			return nil
		}
		if err := s.next(ctx); err != nil {
			return err
		}
	}
}

func (s *shellStream) awaitResult(ctx context.Context, token string) (string, int, error) {
	start := "\x1e" + token + ":start\x1f"
	end := "\x1e" + token + ":"
	started := false
	for {
		if !started {
			if index := strings.Index(s.buffer, start); index >= 0 {
				s.buffer = s.buffer[index+len(start):]
				started = true
			}
		}
		if started {
			if index := strings.Index(s.buffer, end); index >= 0 {
				remainder := s.buffer[index+len(end):]
				if finish := strings.IndexByte(remainder, '\x1f'); finish >= 0 {
					code, err := strconv.Atoi(remainder[:finish])
					if err != nil || code < 0 || code > 255 {
						return "", 0, errors.New("invalid command completion marker")
					}
					result := strings.ReplaceAll(s.buffer[:index], "\r\n", "\n")
					result = strings.TrimPrefix(result, "\n")
					return result, code, nil
				}
			}
		}
		if err := s.next(ctx); err != nil {
			return "", 0, err
		}
	}
}

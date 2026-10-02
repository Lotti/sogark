package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

type consoleKeyRecord struct {
	eventType uint16
	padding   uint16
	keyDown   int32
	repeat    uint16
	keyCode   uint16
	scanCode  uint16
	character uint16
	modifiers uint32
}

func TestWizardWindowsConsoleEditing(t *testing.T) {
	if os.Getenv("SOGARK_WIZARD_CONSOLE_TEST") != "1" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(executable, "-test.run=^TestWizardWindowsConsoleEditing$", "-test.v")
		cmd.Env = append(os.Environ(), "SOGARK_WIZARD_CONSOLE_TEST=1")
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE, HideWindow: true}
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Windows wizard console test failed: %v\n%s", err, output)
		}
		t.Log(string(output))
		return
	}
	input, err := os.OpenFile("CONIN$", os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	var inputMode, outputMode uint32
	if err := windows.GetConsoleMode(windows.Handle(input.Fd()), &inputMode); err != nil {
		t.Fatal(err)
	}
	if err := windows.GetConsoleMode(windows.Handle(output.Fd()), &outputMode); err != nil {
		t.Fatal(err)
	}
	outputMode &^= windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
	if err := windows.SetConsoleMode(windows.Handle(output.Fd()), outputMode); err != nil {
		t.Fatal(err)
	}
	prompt, err := newTerminalPrompter(input, output)
	if err != nil {
		t.Fatal(err)
	}
	defer prompt.Close()

	writeInput := windows.NewLazySystemDLL("kernel32.dll").NewProc("WriteConsoleInputW")
	queue := func(text string) {
		t.Helper()
		var records []consoleKeyRecord
		for _, character := range utf16.Encode([]rune(text)) {
			key := uint16(0)
			if character == '\b' || character == '\r' {
				key = character
			}
			records = append(records, consoleKeyRecord{eventType: 1, keyDown: 1, repeat: 1, keyCode: key, character: character})
		}
		var count uint32
		ok, _, err := writeInput.Call(input.Fd(), uintptr(unsafe.Pointer(&records[0])), uintptr(len(records)), uintptr(unsafe.Pointer(&count)))
		if ok == 0 || count != uint32(len(records)) {
			t.Fatalf("inject console input: count=%d, err=%v", count, err)
		}
	}
	queue("abX\bcd\r")
	if got, err := prompt.Prompt("Value", ""); err != nil || got != "abcd" {
		t.Fatalf("backspace editing failed: %q, %v", got, err)
	}
	readOutput := windows.NewLazySystemDLL("kernel32.dll").NewProc("ReadConsoleOutputCharacterW")
	cells := make([]uint16, 15)
	var count uint32
	ok, _, err := readOutput.Call(output.Fd(), uintptr(unsafe.Pointer(&cells[0])), uintptr(len(cells)), 0, uintptr(unsafe.Pointer(&count)))
	if ok == 0 || !strings.HasPrefix(string(utf16.Decode(cells)), "Value: abcd") {
		t.Fatalf("deleted character remains visible in the console: %q, %v", string(utf16.Decode(cells)), err)
	}
	for _, test := range []struct{ input, current, want string }{
		{"  C:\\Program Files\\Tool  \r", "", `C:\Program Files\Tool`},
		{"utèX\bnte\r", "", "utènte"},
		{"\r", " default ", "default"},
	} {
		queue(test.input)
		if got, err := prompt.Prompt("Value", test.current); err != nil || got != test.want {
			t.Fatalf("Windows input editing failed: %q, %v; want %q", got, err, test.want)
		}
	}
	queue("\x03")
	if _, err := prompt.Prompt("Cancel", ""); !errors.Is(err, io.EOF) {
		t.Fatalf("Ctrl+C must cancel without saving: %v", err)
	}
	if err := prompt.Close(); err != nil {
		t.Fatal(err)
	}
	for handle, want := range map[windows.Handle]uint32{windows.Handle(input.Fd()): inputMode, windows.Handle(output.Fd()): outputMode} {
		var restored uint32
		if err := windows.GetConsoleMode(handle, &restored); err != nil || restored != want {
			t.Fatalf("wizard did not restore console mode: got=%d want=%d err=%v", restored, want, err)
		}
	}
	t.Log("Windows backspace, visible deletion, Unicode, spaces, cancellation and console restoration verified")
}

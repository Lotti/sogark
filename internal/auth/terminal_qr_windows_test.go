package auth

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

func TestTerminalPrompterWindowsConsoleOutput(t *testing.T) {
	if os.Getenv("SOGARK_QR_CONSOLE_TEST") != "1" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(executable, "-test.run=^TestTerminalPrompterWindowsConsoleOutput$", "-test.v")
		cmd.Env = append(os.Environ(), "SOGARK_QR_CONSOLE_TEST=1")
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE, HideWindow: true}
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("isolated Windows console test failed: %v\n%s", err, output)
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

	if _, _, err := term.GetSize(int(input.Fd())); !errors.Is(err, windows.ERROR_INVALID_HANDLE) {
		t.Fatalf("expected the original input-handle failure, got %v", err)
	}
	width, height, err := term.GetSize(int(output.Fd()))
	if err != nil {
		t.Fatalf("console output dimensions must be readable: %v", err)
	}
	if err := checkQRTerminalSize(21, width, height); err != nil {
		t.Fatal(err)
	}
	var original uint32
	handle := windows.Handle(output.Fd())
	if err := windows.GetConsoleMode(handle, &original); err != nil {
		t.Fatal(err)
	}
	original &^= windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
	if err := windows.SetConsoleMode(handle, original); err != nil {
		t.Fatal(err)
	}

	prompt, err := NewTerminalPrompter(input, output)
	if err != nil {
		t.Fatal(err)
	}
	_, encoded := terminalQRFixture(t, 21)
	if err := prompt.DisplayQR(encoded); err != nil {
		t.Fatalf("DisplayQR must use the output handle and enable console colors: %v", err)
	}
	var restored uint32
	if err := windows.GetConsoleMode(handle, &restored); err != nil || restored != original {
		t.Fatalf("console output mode was not restored: mode=%d, err=%v", restored, err)
	}

	kernel := windows.NewLazySystemDLL("kernel32.dll")
	read := kernel.NewProc("ReadConsoleOutputCharacterW")
	var cell uint16
	var count uint32
	position := uintptr((1+qrQuietZone/2)<<16 | qrQuietZone)
	ok, _, readErr := read.Call(output.Fd(), uintptr(unsafe.Pointer(&cell)), 1, position, uintptr(unsafe.Pointer(&count)))
	if ok == 0 {
		t.Fatalf("read rendered console QR module: %v", readErr)
	}
	if count != 1 || cell != '█' {
		t.Fatalf("expected a rendered QR block without literal ANSI escapes, got count=%d, cell=%U", count, cell)
	}
	var attribute uint16
	readAttribute := kernel.NewProc("ReadConsoleOutputAttribute")
	ok, _, readErr = readAttribute.Call(output.Fd(), uintptr(unsafe.Pointer(&attribute)), 1, position, uintptr(unsafe.Pointer(&count)))
	if ok == 0 {
		t.Fatalf("read rendered console QR colors: %v", readErr)
	}
	if count != 1 || attribute&0xff != 0x70 {
		t.Fatalf("expected black QR foreground on white background, got attribute=%#x", attribute)
	}
	t.Logf("input-handle failure reproduced; output=%dx%d; QR rendered with console mode restored", width, height)
}

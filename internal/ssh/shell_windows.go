package ssh

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

func startShell(child *exec.Cmd) (io.Reader, io.WriteCloser, func() error, func(), error) {
	var input, output, inWrite, outRead windows.Handle
	if err := windows.CreatePipe(&input, &inWrite, nil, 0); err != nil {
		return nil, nil, nil, nil, err
	}
	if err := windows.CreatePipe(&outRead, &output, nil, 0); err != nil {
		windows.CloseHandle(input)
		windows.CloseHandle(inWrite)
		return nil, nil, nil, nil, err
	}
	reader := os.NewFile(uintptr(outRead), "conpty-output")
	writer := os.NewFile(uintptr(inWrite), "conpty-input")
	var console windows.Handle
	err := windows.CreatePseudoConsole(windows.Coord{X: 240, Y: 40}, input, output, 0, &console)
	windows.CloseHandle(input)
	windows.CloseHandle(output)
	if err != nil {
		reader.Close()
		writer.Close()
		return nil, nil, nil, nil, fmt.Errorf("create Windows pseudo-terminal (Windows 10 1809+ required): %w", err)
	}
	attributes, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		windows.ClosePseudoConsole(console)
		reader.Close()
		writer.Close()
		return nil, nil, nil, nil, err
	}
	defer attributes.Delete()
	consolePointer := *(*unsafe.Pointer)(unsafe.Pointer(&console))
	err = attributes.Update(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, consolePointer, unsafe.Sizeof(console))
	if err != nil {
		windows.ClosePseudoConsole(console)
		reader.Close()
		writer.Close()
		return nil, nil, nil, nil, err
	}
	info := windows.StartupInfoEx{}
	info.Cb = uint32(unsafe.Sizeof(info))
	info.Flags = windows.STARTF_USESTDHANDLES
	info.ProcThreadAttributeList = attributes.List()
	parts := make([]string, len(child.Args))
	for i, value := range child.Args {
		parts[i] = syscall.EscapeArg(value)
	}
	command, err := windows.UTF16PtrFromString(strings.Join(parts, " "))
	if err != nil {
		windows.ClosePseudoConsole(console)
		reader.Close()
		writer.Close()
		return nil, nil, nil, nil, err
	}
	application, err := windows.UTF16PtrFromString(child.Path)
	if err != nil {
		windows.ClosePseudoConsole(console)
		reader.Close()
		writer.Close()
		return nil, nil, nil, nil, err
	}
	var process windows.ProcessInformation
	var environment *uint16
	var block []uint16
	flags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT)
	if child.Env != nil {
		block = utf16.Encode([]rune(strings.Join(child.Env, "\x00") + "\x00\x00"))
		environment = &block[0]
		flags |= windows.CREATE_UNICODE_ENVIRONMENT
	}
	err = windows.CreateProcess(application, command, nil, nil, false, flags, environment, nil, &info.StartupInfo, &process)
	if err != nil {
		windows.ClosePseudoConsole(console)
		reader.Close()
		writer.Close()
		return nil, nil, nil, nil, err
	}
	windows.CloseHandle(process.Thread)
	var once sync.Once
	closeSession := func() {
		once.Do(func() {
			windows.TerminateProcess(process.Process, 1)
			writer.Close()
			// Drain output while ClosePseudoConsole flushes its terminal buffer.
			go io.Copy(io.Discard, reader)
			windows.ClosePseudoConsole(console)
			reader.Close()
			windows.CloseHandle(process.Process)
		})
	}
	wait := func() error {
		_, err := windows.WaitForSingleObject(process.Process, windows.INFINITE)
		if err != nil {
			return err
		}
		var code uint32
		if err := windows.GetExitCodeProcess(process.Process, &code); err != nil {
			return err
		}
		if code != 0 {
			return fmt.Errorf("SSH process exited with status %d", code)
		}
		return nil
	}
	return reader, &consoleInput{writer}, wait, closeSession, nil
}

type consoleInput struct{ *os.File }

func (w *consoleInput) Write(p []byte) (int, error) {
	data := []byte(strings.ReplaceAll(string(p), "\n", "\r"))
	_, err := w.File.Write(data)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *consoleInput) WriteString(value string) (int, error) {
	return w.Write([]byte(value))
}

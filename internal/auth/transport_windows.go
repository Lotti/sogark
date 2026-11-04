//go:build windows

package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	winHTTPAccessTypeAutomaticProxy = 4
	winHTTPOptionAutologonPolicy    = 77
	winHTTPOptionRedirectPolicy     = 88
	winHTTPAutologonPolicyMedium    = 1
	winHTTPRedirectPolicyNever      = 0
	winHTTPFlagSecure               = 0x00800000
	winHTTPAddRequestHeader         = 0x20000000
	winHTTPQueryStatusCode          = 19
	winHTTPQueryRawHeadersCRLF      = 22
	winHTTPQueryFlagNumber          = 0x20000000
	winHTTPErrorInsufficientBuffer  = 122
	maxRequestBytes                 = 4 << 20
)

var winHTTP = windows.NewLazySystemDLL("winhttp.dll")

func newNativeTransport() http.RoundTripper {
	return &windowsProxyTransport{
		system: &winHTTPRoundTripper{},
		env:    http.DefaultTransport,
	}
}

type windowsProxyTransport struct {
	system http.RoundTripper
	env    http.RoundTripper
}

func (t *windowsProxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if hasEnvironmentProxy(req.URL) {
		return t.env.RoundTrip(req)
	}
	return t.system.RoundTrip(req)
}

func hasEnvironmentProxy(u *url.URL) bool {
	name := "HTTP_PROXY"
	if strings.EqualFold(u.Scheme, "https") {
		name = "HTTPS_PROXY"
	}
	value, ok := os.LookupEnv(name)
	return ok && strings.TrimSpace(value) != ""
}

type winHTTPRoundTripper struct{}

func (t *winHTTPRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		defer req.Body.Close()
	}
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	if req.URL == nil || (req.URL.Scheme != "http" && req.URL.Scheme != "https") || req.URL.Hostname() == "" {
		return nil, errors.New("WinHTTP requires an absolute HTTP URL")
	}

	body, err := readRequestBody(req)
	if err != nil {
		return nil, err
	}
	// WinHTTP can retain the payload until WinHttpReceiveResponse completes.
	defer runtime.KeepAlive(body)
	session, err := openWinHTTPSession()
	if err != nil {
		return nil, err
	}
	handles := &winHTTPHandles{session: session}
	fail := func(err error) (*http.Response, error) {
		handles.Close()
		return nil, err
	}

	port := req.URL.Port()
	if port == "" {
		if req.URL.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	portNumber, err := strconv.ParseUint(port, 10, 16)
	if err != nil || portNumber == 0 {
		return fail(errors.New("invalid request port"))
	}
	host, err := syscall.UTF16PtrFromString(req.URL.Hostname())
	if err != nil {
		return fail(err)
	}
	connect, err := openWinHTTPConnection(session, host, uint16(portNumber))
	if err != nil {
		return fail(err)
	}
	handles.connect = connect

	method, err := syscall.UTF16PtrFromString(req.Method)
	if err != nil {
		return fail(err)
	}
	object, err := syscall.UTF16PtrFromString(requestTarget(req.URL))
	if err != nil {
		return fail(err)
	}
	version, _ := syscall.UTF16PtrFromString("HTTP/1.1")
	var flags uintptr
	if req.URL.Scheme == "https" {
		flags = winHTTPFlagSecure
	}
	requestHandle, err := openWinHTTPRequest(connect, method, object, version, flags)
	if err != nil {
		return fail(err)
	}
	handles.request = requestHandle
	if err := setWinHTTPAutologonPolicy(requestHandle); err != nil {
		return fail(err)
	}
	if err := disableWinHTTPRedirects(requestHandle); err != nil {
		return fail(err)
	}
	if err := addWinHTTPHeaders(requestHandle, req.Header); err != nil {
		return fail(err)
	}

	stopCancel := context.AfterFunc(req.Context(), handles.Close)
	if err := sendWinHTTPRequest(requestHandle, body); err != nil {
		stopCancel()
		return fail(err)
	}
	if err := receiveWinHTTPResponse(requestHandle); err != nil {
		stopCancel()
		return fail(err)
	}
	status, err := queryWinHTTPStatus(requestHandle)
	if err != nil {
		stopCancel()
		return fail(err)
	}
	headers, err := queryWinHTTPHeaders(requestHandle)
	if err != nil {
		stopCancel()
		return fail(err)
	}
	if err := req.Context().Err(); err != nil {
		stopCancel()
		return fail(err)
	}

	resp := &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     headers,
		Request:    req,
	}
	if length := headers.Get("Content-Length"); length != "" {
		resp.ContentLength, _ = strconv.ParseInt(length, 10, 64)
	} else {
		resp.ContentLength = -1
	}
	if transfer := headers.Values("Transfer-Encoding"); len(transfer) != 0 {
		resp.TransferEncoding = transfer
		resp.Header.Del("Transfer-Encoding")
		resp.ContentLength = -1
	}
	resp.Body = &winHTTPResponseBody{
		handle: handles,
		stop:   stopCancel,
	}
	return resp, nil
}

func readRequestBody(req *http.Request) ([]byte, error) {
	if req.Body == nil || req.Body == http.NoBody {
		return nil, nil
	}
	body, err := io.ReadAll(io.LimitReader(req.Body, maxRequestBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxRequestBytes {
		return nil, errors.New("request body exceeds size limit")
	}
	return body, nil
}

func requestTarget(u *url.URL) string {
	target := u.EscapedPath()
	if target == "" {
		target = "/"
	}
	if u.RawQuery != "" {
		target += "?" + u.RawQuery
	}
	return target
}

func openWinHTTPSession() (uintptr, error) {
	if err := winHTTP.Load(); err != nil {
		return 0, fmt.Errorf("load WinHTTP: %w", err)
	}
	agent, _ := syscall.UTF16PtrFromString("sogark")
	session, _, callErr := winHTTP.NewProc("WinHttpOpen").Call(
		uintptr(unsafe.Pointer(agent)),
		winHTTPAccessTypeAutomaticProxy,
		0,
		0,
		0,
	)
	if session == 0 {
		return 0, winHTTPCallError("WinHttpOpen", callErr)
	}
	return session, nil
}

func openWinHTTPConnection(session uintptr, host *uint16, port uint16) (uintptr, error) {
	handle, _, callErr := winHTTP.NewProc("WinHttpConnect").Call(session, uintptr(unsafe.Pointer(host)), uintptr(port), 0)
	if handle == 0 {
		return 0, winHTTPCallError("WinHttpConnect", callErr)
	}
	return handle, nil
}

func openWinHTTPRequest(connect uintptr, method, object, version *uint16, flags uintptr) (uintptr, error) {
	handle, _, callErr := winHTTP.NewProc("WinHttpOpenRequest").Call(
		connect,
		uintptr(unsafe.Pointer(method)),
		uintptr(unsafe.Pointer(object)),
		uintptr(unsafe.Pointer(version)),
		0,
		0,
		flags,
	)
	if handle == 0 {
		return 0, winHTTPCallError("WinHttpOpenRequest", callErr)
	}
	return handle, nil
}

func setWinHTTPAutologonPolicy(request uintptr) error {
	policy := uint32(winHTTPAutologonPolicyMedium)
	ok, _, callErr := winHTTP.NewProc("WinHttpSetOption").Call(
		request,
		winHTTPOptionAutologonPolicy,
		uintptr(unsafe.Pointer(&policy)),
		unsafe.Sizeof(policy),
	)
	if ok == 0 {
		return winHTTPCallError("WinHttpSetOption", callErr)
	}
	return nil
}

func disableWinHTTPRedirects(request uintptr) error {
	policy := uint32(winHTTPRedirectPolicyNever)
	ok, _, callErr := winHTTP.NewProc("WinHttpSetOption").Call(
		request,
		winHTTPOptionRedirectPolicy,
		uintptr(unsafe.Pointer(&policy)),
		unsafe.Sizeof(policy),
	)
	if ok == 0 {
		return winHTTPCallError("WinHttpSetOption(redirect policy)", callErr)
	}
	return nil
}

func addWinHTTPHeaders(request uintptr, headers http.Header) error {
	for name, values := range headers {
		switch strings.ToLower(name) {
		case "host", "content-length", "connection", "proxy-connection", "proxy-authorization", "transfer-encoding", "expect":
			continue
		}
		for _, value := range values {
			header, err := syscall.UTF16PtrFromString(name + ": " + value)
			if err != nil {
				return err
			}
			ok, _, callErr := winHTTP.NewProc("WinHttpAddRequestHeaders").Call(
				request,
				uintptr(unsafe.Pointer(header)),
				^uintptr(0),
				winHTTPAddRequestHeader,
			)
			if ok == 0 {
				return winHTTPCallError("WinHttpAddRequestHeaders", callErr)
			}
		}
	}
	return nil
}

func sendWinHTTPRequest(request uintptr, body []byte) error {
	var bodyPointer uintptr
	if len(body) != 0 {
		bodyPointer = uintptr(unsafe.Pointer(&body[0]))
	}
	ok, _, callErr := winHTTP.NewProc("WinHttpSendRequest").Call(
		request,
		0,
		0,
		bodyPointer,
		uintptr(len(body)),
		uintptr(len(body)),
		0,
	)
	if ok == 0 {
		return winHTTPCallError("WinHttpSendRequest", callErr)
	}
	return nil
}

func receiveWinHTTPResponse(request uintptr) error {
	ok, _, callErr := winHTTP.NewProc("WinHttpReceiveResponse").Call(request, 0)
	if ok == 0 {
		return winHTTPCallError("WinHttpReceiveResponse", callErr)
	}
	return nil
}

func queryWinHTTPStatus(request uintptr) (int, error) {
	var status uint32
	size := uint32(unsafe.Sizeof(status))
	ok, _, callErr := winHTTP.NewProc("WinHttpQueryHeaders").Call(
		request,
		winHTTPQueryStatusCode|winHTTPQueryFlagNumber,
		0,
		uintptr(unsafe.Pointer(&status)),
		uintptr(unsafe.Pointer(&size)),
		0,
	)
	if ok == 0 {
		return 0, winHTTPCallError("WinHttpQueryHeaders(status)", callErr)
	}
	return int(status), nil
}

func queryWinHTTPHeaders(request uintptr) (http.Header, error) {
	proc := winHTTP.NewProc("WinHttpQueryHeaders")
	var size uint32
	ok, _, callErr := proc.Call(request, winHTTPQueryRawHeadersCRLF, 0, 0, uintptr(unsafe.Pointer(&size)), 0)
	if ok != 0 || winHTTPErrorCode(callErr) != winHTTPErrorInsufficientBuffer {
		return nil, winHTTPCallError("WinHttpQueryHeaders(headers)", callErr)
	}
	if size == 0 {
		return nil, errors.New("WinHTTP returned empty response headers")
	}
	buffer := make([]uint16, (size+1)/2)
	ok, _, callErr = proc.Call(
		request,
		winHTTPQueryRawHeadersCRLF,
		0,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&size)),
		0,
	)
	if ok == 0 {
		return nil, winHTTPCallError("WinHttpQueryHeaders(headers)", callErr)
	}
	raw := syscall.UTF16ToString(buffer)
	lines := strings.Split(raw, "\r\n")
	headers := make(http.Header)
	for _, line := range lines[1:] {
		if line == "" {
			continue
		}
		name, value, found := strings.Cut(line, ":")
		if !found {
			return nil, errors.New("WinHTTP returned a malformed response header")
		}
		headers.Add(strings.TrimSpace(name), strings.TrimSpace(value))
	}
	return headers, nil
}

func winHTTPCallError(operation string, callErr error) error {
	if errno := winHTTPErrorCode(callErr); errno != 0 {
		return fmt.Errorf("%s failed: %w", operation, windows.Errno(errno))
	}
	return fmt.Errorf("%s failed", operation)
}

func winHTTPErrorCode(callErr error) uint32 {
	if errno, ok := callErr.(syscall.Errno); ok {
		return uint32(errno)
	}
	return 0
}

type winHTTPHandles struct {
	mu      sync.Mutex
	session uintptr
	connect uintptr
	request uintptr
	closed  bool
}

func (h *winHTTPHandles) Close() {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.closed = true
	request, connect, session := h.request, h.connect, h.session
	h.request, h.connect, h.session = 0, 0, 0
	h.mu.Unlock()

	closeProc := winHTTP.NewProc("WinHttpCloseHandle")
	if request != 0 {
		_, _, _ = closeProc.Call(request)
	}
	if connect != 0 {
		_, _, _ = closeProc.Call(connect)
	}
	if session != 0 {
		_, _, _ = closeProc.Call(session)
	}
}

type winHTTPResponseBody struct {
	handle *winHTTPHandles
	stop   func() bool
	once   sync.Once
}

func (b *winHTTPResponseBody) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	var available uint32
	ok, _, callErr := winHTTP.NewProc("WinHttpQueryDataAvailable").Call(
		b.requestHandle(),
		uintptr(unsafe.Pointer(&available)),
	)
	if ok == 0 {
		return 0, winHTTPCallError("WinHttpQueryDataAvailable", callErr)
	}
	if available == 0 {
		_ = b.Close()
		return 0, io.EOF
	}
	if uint64(available) > uint64(len(p)) {
		available = uint32(len(p))
	}
	var read uint32
	ok, _, callErr = winHTTP.NewProc("WinHttpReadData").Call(
		b.requestHandle(),
		uintptr(unsafe.Pointer(&p[0])),
		uintptr(available),
		uintptr(unsafe.Pointer(&read)),
	)
	if ok == 0 {
		return 0, winHTTPCallError("WinHttpReadData", callErr)
	}
	return int(read), nil
}

func (b *winHTTPResponseBody) requestHandle() uintptr {
	b.handle.mu.Lock()
	defer b.handle.mu.Unlock()
	return b.handle.request
}

func (b *winHTTPResponseBody) Close() error {
	b.once.Do(func() {
		if b.stop != nil {
			b.stop()
		}
		b.handle.Close()
	})
	return nil
}

var _ http.RoundTripper = (*winHTTPRoundTripper)(nil)
var _ io.ReadCloser = (*winHTTPResponseBody)(nil)

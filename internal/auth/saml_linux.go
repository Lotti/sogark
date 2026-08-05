//go:build linux

package auth

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	msg "github.com/Lotti/sogark/internal/messages"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

const (
	envAuthMode = "SOGARK_AUTH_MODE"
	envBrowser  = "SOGARK_BROWSER"
)

// findBrowser looks for a Chromium-based browser, preferring Edge over Chrome.
func findBrowser() (string, error) {
	if override := strings.TrimSpace(os.Getenv(envBrowser)); override != "" {
		if !strings.ContainsRune(override, os.PathSeparator) {
			if resolved, err := exec.LookPath(override); err == nil {
				return resolved, nil
			}
		}
		if _, err := os.Stat(override); err == nil {
			return override, nil
		}
		return "", fmt.Errorf(msg.AuthBrowserOverrideMissing, override)
	}
	if p := findEdge(); p != "" {
		return p, nil
	}
	if p := findBrowserInPath(exec.LookPath); p != "" {
		return p, nil
	}
	if p, found := launcher.LookPath(); found {
		return p, nil
	}
	return "", fmt.Errorf(msg.AuthBrowserNotFound)
}

func SAMLPrerequisite() (string, error) {
	if err := ensureLinuxAuthMode(); err != nil {
		return "", err
	}
	return findBrowser()
}

func findEdge() string {
	for _, p := range []string{
		"/usr/bin/microsoft-edge",
		"/usr/bin/microsoft-edge-stable",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func findBrowserInPath(lookPath func(string) (string, error)) string {
	for _, name := range []string{
		"microsoft-edge",
		"microsoft-edge-stable",
		"google-chrome",
		"google-chrome-stable",
		"chromium",
		"chromium-browser",
		"brave-browser",
		"brave-browser-stable",
	} {
		if p, err := lookPath(name); err == nil && strings.TrimSpace(p) != "" {
			return p
		}
	}
	return ""
}

func ensureLinuxAuthMode() error {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(envAuthMode)))
	switch raw {
	case "", "auto", "gui":
		if linuxGUIAvailable() {
			return nil
		}
		return fmt.Errorf(msg.AuthLinuxGUIRequired)
	case "headless":
		return fmt.Errorf(msg.AuthLinuxHeadlessUnsupported)
	default:
		return fmt.Errorf(msg.AuthModeInvalid, raw)
	}
}

func linuxGUIAvailable() bool {
	return strings.TrimSpace(os.Getenv("DISPLAY")) != "" || strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) != ""
}

// SAMLResponse captures the SAML response token from the IDP via browser automation.
// On Linux this is supported only with a graphical Chromium-based browser session.
func SAMLResponse(ctx context.Context, idpURL string, timeoutMinutes int) (string, error) {
	if err := ensureLinuxAuthMode(); err != nil {
		return "", err
	}

	path, err := findBrowser()
	if err != nil {
		return "", err
	}

	l := launcher.New().Bin(path).
		Leakless(false).
		Headless(false).
		Set("disable-gpu").
		Set("disable-dev-shm-usage").
		NoSandbox(true)

	u, err := l.Launch()
	if err != nil {
		return "", fmt.Errorf(msg.AuthBrowserStartErr, err)
	}

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return "", fmt.Errorf(msg.AuthBrowserConnectErr, err)
	}
	defer browser.MustClose()

	page, err := browser.Page(proto.TargetCreateTarget{URL: ""})
	if err != nil {
		return "", fmt.Errorf(msg.AuthBrowserPageErr, err)
	}
	defer page.MustClose()

	maxPostDataSize := 10 * 1024 * 1024
	if err := (proto.NetworkEnable{MaxPostDataSize: &maxPostDataSize}).Call(page); err != nil {
		return "", fmt.Errorf(msg.AuthBrowserPageErr, err)
	}

	samlReqCh := make(chan string, 1)
	go page.EachEvent(func(e *proto.NetworkRequestWillBeSent) {
		if s := samlFromRequest(page, e); s != "" {
			select {
			case samlReqCh <- s:
			default:
			}
		}
	})()

	_, err = page.EvalOnNewDocument(`(function() {
		function readSAMLValue(root) {
			if (!root) return "";
			var el = null;
			if (root.forms) {
				for (var i = 0; i < root.forms.length; i++) {
					var named = root.forms[i].elements && root.forms[i].elements.namedItem('SAMLResponse');
					if (named) {
						el = named;
						break;
					}
				}
			}
			if (!el && root.querySelector) {
				el = root.querySelector('[name="SAMLResponse"]');
			}
			if (!el) return "";
			if (typeof el.value === 'string' && el.value) return el.value;
			if (typeof el.textContent === 'string' && el.textContent.trim()) return el.textContent.trim();
			return "";
		}

		function capture(root) {
			var value = readSAMLValue(root);
			if (value && !window.__sogark_saml) {
				window.__sogark_saml = value;
			}
		}

		var origSubmit = HTMLFormElement.prototype.submit;
		HTMLFormElement.prototype.submit = function() {
			var value = readSAMLValue(this);
			if (value) {
				window.__sogark_saml = value;
				return;
			}
			origSubmit.call(this);
		};

		if (HTMLFormElement.prototype.requestSubmit) {
			var origRequestSubmit = HTMLFormElement.prototype.requestSubmit;
			HTMLFormElement.prototype.requestSubmit = function() {
				var value = readSAMLValue(this);
				if (value) {
					window.__sogark_saml = value;
					return;
				}
				return origRequestSubmit.apply(this, arguments);
			};
		}

		document.addEventListener('submit', function(e) {
			var value = readSAMLValue(e.target);
			if (value) {
				e.preventDefault();
				window.__sogark_saml = value;
			}
		}, true);

		document.addEventListener('DOMContentLoaded', function() { capture(document); });
		document.addEventListener('readystatechange', function() { capture(document); });

		var obs = new MutationObserver(function() { capture(document); });
		obs.observe(document.documentElement || document, { childList: true, subtree: true, attributes: true, characterData: true });
	})()`)
	if err != nil {
		return "", fmt.Errorf(msg.AuthSAMLScriptErr, err)
	}

	err = page.Navigate(idpURL)
	if err != nil {
		return "", fmt.Errorf(msg.AuthNavigateErr, err)
	}

	fmt.Println(msg.AuthBrowserOpening)
	fmt.Println(msg.AuthCompleteInBrowser)

	deadline := time.After(time.Duration(timeoutMinutes) * time.Minute)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-deadline:
			return "", fmt.Errorf(msg.AuthSAMLTimeout)
		case s := <-samlReqCh:
			fmt.Println(msg.AuthComplete)
			return s, nil
		case <-ticker.C:
			saml, evalErr := currentSAML(page)
			if evalErr == nil && saml != "" {
				fmt.Println(msg.AuthComplete)
				return saml, nil
			}
			if s := samlFromURL(page); s != "" {
				fmt.Println(msg.AuthComplete)
				return s, nil
			}
		}
	}
}

func currentSAML(page *rod.Page) (string, error) {
	val, err := page.Eval(`(function() {
		if (window.__sogark_saml) return window.__sogark_saml;
		var forms = document.forms || [];
		for (var i = 0; i < forms.length; i++) {
			var named = forms[i].elements && forms[i].elements.namedItem('SAMLResponse');
			if (named) {
				if (typeof named.value === 'string' && named.value) return named.value;
				if (typeof named.textContent === 'string' && named.textContent.trim()) return named.textContent.trim();
			}
		}
		var el = document.querySelector('[name="SAMLResponse"]');
		if (el) {
			if (typeof el.value === 'string' && el.value) return el.value;
			if (typeof el.textContent === 'string' && el.textContent.trim()) return el.textContent.trim();
		}
		return "";
	})()`)
	if err != nil {
		return "", err
	}
	return val.Value.Str(), nil
}

func samlFromURL(page *rod.Page) string {
	info, err := page.Info()
	if err != nil {
		return ""
	}
	u, err := url.Parse(info.URL)
	if err != nil {
		return ""
	}
	return u.Query().Get("SAMLResponse")
}

func samlFromRequest(page *rod.Page, e *proto.NetworkRequestWillBeSent) string {
	if e == nil || e.Request == nil {
		return ""
	}

	if s := extractSAMLFromPostData(e.Request.PostData); s != "" {
		return s
	}

	if !e.Request.HasPostData {
		return ""
	}

	res, err := proto.NetworkGetRequestPostData{RequestID: e.RequestID}.Call(page)
	if err != nil {
		return ""
	}
	return extractSAMLFromPostData(res.PostData)
}

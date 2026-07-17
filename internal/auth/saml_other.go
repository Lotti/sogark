//go:build !windows

package auth

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"runtime"
	"time"

	msg "github.com/Lotti/sogark/internal/messages"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// findBrowser looks for a Chromium-based browser, preferring Edge over Chrome.
func findBrowser() (string, error) {
	if p := findEdge(); p != "" {
		return p, nil
	}
	if p, found := launcher.LookPath(); found {
		return p, nil
	}
	return "", fmt.Errorf(msg.AuthBrowserNotFound)
}

func SAMLPrerequisite() (string, error) {
	return findBrowser()
}

func findEdge() string {
	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge"}
	default: // linux
		candidates = []string{
			"/usr/bin/microsoft-edge",
			"/usr/bin/microsoft-edge-stable",
		}
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// SAMLResponse captures the SAML response token from the IDP via browser automation.
// Uses go-rod to control a Chromium-based browser, injecting JavaScript to intercept
// the SAMLResponse before the IDP auto-submits the form.
func SAMLResponse(ctx context.Context, idpURL string, timeoutMinutes int) (string, error) {
	path, err := findBrowser()
	if err != nil {
		return "", err
	}

	l := launcher.New().Bin(path).
		Leakless(false).
		Headless(false).
		Set("disable-gpu")

	// RHEL 9 (and other enterprise Linux distributions) restrict kernel user namespaces
	// by default (user.max_user_namespaces=0). Without --no-sandbox the Chromium sandbox
	// fails to start, which makes the CDP connection unreliable even though the browser
	// window itself opens. --disable-dev-shm-usage prevents crashes when /dev/shm is small.
	if runtime.GOOS == "linux" {
		l = l.Set("no-sandbox").Set("disable-dev-shm-usage")
	}

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

	// Inject JS into every new document loaded in this tab.
	// Three interception layers are used for maximum compatibility:
	//  1. HTMLFormElement.prototype.submit override — catches explicit .submit() calls
	//  2. capture-phase submit event listener — catches button clicks and JS dispatched events
	//  3. MutationObserver + DOMContentLoaded — catches cases where the form is already in
	//     the DOM (static HTML) or added dynamically after initial load
	_, err = page.EvalOnNewDocument(`(function() {
		function capture(root) {
			var el = root ? root.querySelector('input[name="SAMLResponse"]') : null;
			if (el && el.value && !window.__sogark_saml) {
				window.__sogark_saml = el.value;
			}
		}

		var origSubmit = HTMLFormElement.prototype.submit;
		HTMLFormElement.prototype.submit = function() {
			var el = this.querySelector('input[name="SAMLResponse"]');
			if (el && el.value) {
				window.__sogark_saml = el.value;
				return; // prevent actual form navigation
			}
			origSubmit.call(this);
		};

		document.addEventListener('submit', function(e) {
			var el = e.target.querySelector && e.target.querySelector('input[name="SAMLResponse"]');
			if (el && el.value) {
				e.preventDefault();
				window.__sogark_saml = el.value;
			}
		}, true);

		// Static form: check once DOM is ready
		document.addEventListener('DOMContentLoaded', function() { capture(document); });

		// Dynamic form: watch for DOM mutations
		var obs = new MutationObserver(function() { capture(document); });
		obs.observe(document.documentElement || document, { childList: true, subtree: true });
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
		case <-ticker.C:
			// Three-layer check: injected variable → live DOM → current URL query param.
			// The URL check handles SAML redirect binding (rare but used by some IDPs).
			val, evalErr := page.Eval(`(function() {
				if (window.__sogark_saml) return window.__sogark_saml;
				var el = document.querySelector('input[name="SAMLResponse"]');
				if (el && el.value) return el.value;
				return "";
			})()`)
			if evalErr != nil {
				// Try URL-based detection as last resort (redirect binding)
				if s := samlFromURL(page); s != "" {
					fmt.Println(msg.AuthComplete)
					return s, nil
				}
				continue
			}
			if s := val.Value.Str(); s != "" {
				fmt.Println(msg.AuthComplete)
				return s, nil
			}
			// Also check URL in case eval succeeded but value was empty
			if s := samlFromURL(page); s != "" {
				fmt.Println(msg.AuthComplete)
				return s, nil
			}
		}
	}
}

// samlFromURL extracts a SAMLResponse query parameter from the browser's current URL.
// This handles the rare SAML HTTP redirect binding.
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


//go:build !windows

package auth

import (
	"context"
	"fmt"
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

	u, err := launcher.New().Bin(path).
		Leakless(false).
		Headless(false).
		Set("disable-gpu").
		Launch()
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

	// Inject JS to intercept form submissions containing SAMLResponse
	_, err = page.EvalOnNewDocument(`(function() {
		var origSubmit = HTMLFormElement.prototype.submit;
		HTMLFormElement.prototype.submit = function() {
			var el = this.querySelector('input[name="SAMLResponse"]');
			if (el && el.value) {
				window.__sogark_saml = el.value;
				return;
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
			val, evalErr := page.Eval(`window.__sogark_saml || ""`)
			if evalErr != nil {
				continue
			}
			if s := val.Value.Str(); s != "" {
				fmt.Println(msg.AuthComplete)
				return s, nil
			}
		}
	}
}

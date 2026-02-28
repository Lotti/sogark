//go:build !windows

package auth

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// findBrowser looks for a Chromium-based browser, preferring Edge over Chrome.
func findBrowser() (string, error) {
	if p := findEdge(); p != "" {
		return p, nil
	}
	if p, found := launcher.LookPath(); found {
		return p, nil
	}
	return "", fmt.Errorf("browser Chromium-based non trovato (Edge, Chrome, Chromium).\n" +
		"Installa Edge o Chrome:\n" +
		"  macOS:  brew install --cask microsoft-edge\n" +
		"  Linux:  sudo apt install microsoft-edge-stable")
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
func SAMLResponse(ctx context.Context, idpURL string) (string, error) {
	path, err := findBrowser()
	if err != nil {
		return "", err
	}

	u := launcher.New().Bin(path).
		Leakless(false).
		Headless(false).
		Set("disable-gpu").
		MustLaunch()

	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("")
	defer page.MustClose()

	// Inject JS to intercept form submissions containing SAMLResponse
	page.MustEvalOnNewDocument(`(function() {
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

	page.MustNavigate(idpURL)

	fmt.Println("[*] Apertura browser per login SAML/MFA...")
	fmt.Println("   Completa l'autenticazione nel browser.")

	deadline := time.After(5 * time.Minute)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			return "", fmt.Errorf("timeout: SAMLResponse non ricevuta (hai completato il login?)")
		case <-ticker.C:
			val, evalErr := page.Eval(`window.__sogark_saml || ""`)
			if evalErr != nil {
				continue
			}
			if s := val.Value.Str(); s != "" {
				fmt.Println("[+] Autenticazione completata")
				return s, nil
			}
		}
	}
}

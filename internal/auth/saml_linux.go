//go:build linux

package auth

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	msg "github.com/Lotti/sogark/internal/messages"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"golang.org/x/term"
)

const (
	envAuthMode = "SOGARK_AUTH_MODE"
	envBrowser  = "SOGARK_BROWSER"
)

type linuxAuthMode string

const (
	linuxAuthModeAuto     linuxAuthMode = "auto"
	linuxAuthModeGUI      linuxAuthMode = "gui"
	linuxAuthModeHeadless linuxAuthMode = "headless"
)

type headlessSnapshot struct {
	Title   string           `json:"title"`
	URL     string           `json:"url"`
	Text    string           `json:"text"`
	Fields  []headlessField  `json:"fields"`
	Buttons []headlessButton `json:"buttons"`
}

type headlessField struct {
	Type         string             `json:"type"`
	Selector     string             `json:"selector,omitempty"`
	Label        string             `json:"label"`
	Name         string             `json:"name,omitempty"`
	Value        string             `json:"value,omitempty"`
	ReadOnly     bool               `json:"readOnly,omitempty"`
	Checked      bool               `json:"checked,omitempty"`
	Options      []headlessOption   `json:"options,omitempty"`
	RadioOptions []headlessRadioOpt `json:"radioOptions,omitempty"`
}

type headlessOption struct {
	Label    string `json:"label"`
	Value    string `json:"value"`
	Selected bool   `json:"selected"`
}

type headlessRadioOpt struct {
	Label    string `json:"label"`
	Value    string `json:"value"`
	Selector string `json:"selector"`
	Checked  bool   `json:"checked"`
}

type headlessButton struct {
	Selector string `json:"selector"`
	Label    string `json:"label"`
}

// findBrowser looks for a Chromium-based browser, preferring Edge over Chrome.
func findBrowser() (string, error) {
	if override := strings.TrimSpace(os.Getenv(envBrowser)); override != "" {
		if _, err := os.Stat(override); err == nil {
			return override, nil
		}
		return "", fmt.Errorf(msg.AuthBrowserOverrideMissing, override)
	}
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

// SAMLResponse captures SAMLResponse on Linux with Rod.
// Default behavior is GUI when a graphical session is available, otherwise
// headless terminal mode. Override with SOGARK_AUTH_MODE=gui|headless|auto.
func SAMLResponse(ctx context.Context, idpURL string, timeoutMinutes int) (string, error) {
	path, err := findBrowser()
	if err != nil {
		return "", err
	}

	mode, err := resolveLinuxAuthMode()
	if err != nil {
		return "", err
	}

	switch mode {
	case linuxAuthModeHeadless:
		return samlResponseHeadless(ctx, path, idpURL, timeoutMinutes)
	default:
		return samlResponseGUI(ctx, path, idpURL, timeoutMinutes)
	}
}

func resolveLinuxAuthMode() (linuxAuthMode, error) {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(envAuthMode)))
	switch raw {
	case "":
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			return linuxAuthModeHeadless, nil
		}
		return linuxAuthModeGUI, nil
	case string(linuxAuthModeAuto):
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			return linuxAuthModeHeadless, nil
		}
		return linuxAuthModeGUI, nil
	case string(linuxAuthModeGUI):
		return linuxAuthModeGUI, nil
	case string(linuxAuthModeHeadless):
		return linuxAuthModeHeadless, nil
	default:
		return "", fmt.Errorf(msg.AuthModeInvalid, raw)
	}
}

func samlResponseGUI(ctx context.Context, browserPath, idpURL string, timeoutMinutes int) (string, error) {
	browser, page, cleanup, err := newBrowserPage(browserPath, false)
	if err != nil {
		return "", err
	}
	defer cleanup()
	_ = browser

	if err := page.Navigate(idpURL); err != nil {
		return "", fmt.Errorf(msg.AuthNavigateErr, err)
	}

	fmt.Println(msg.AuthBrowserOpening)
	fmt.Println(msg.AuthCompleteInBrowser)

	return waitForSAML(ctx, page, timeoutMinutes)
}

func samlResponseHeadless(ctx context.Context, browserPath, idpURL string, timeoutMinutes int) (string, error) {
	_, page, cleanup, err := newBrowserPage(browserPath, true)
	if err != nil {
		return "", err
	}
	defer cleanup()

	if err := page.Navigate(idpURL); err != nil {
		return "", fmt.Errorf(msg.AuthNavigateErr, err)
	}

	fmt.Println(msg.AuthHeadlessOpening)
	fmt.Println(msg.AuthHeadlessInstructions)

	reader := bufio.NewReader(os.Stdin)
	deadline := time.Now().Add(time.Duration(timeoutMinutes) * time.Minute)
	lastSignature := ""

	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf(msg.AuthSAMLTimeout)
		}

		if saml, err := currentSAML(page); err == nil && saml != "" {
			fmt.Println(msg.AuthComplete)
			return saml, nil
		}

		snapshot, err := captureHeadlessSnapshot(page)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		signature := snapshot.signature()
		if signature != lastSignature {
			printHeadlessSnapshot(snapshot)
			lastSignature = signature
		}

		if !snapshot.hasInteractiveControls() {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if err := fillHeadlessFields(page, reader, snapshot.Fields); err != nil {
			return "", err
		}
		waiting, err := runHeadlessAction(page, reader, snapshot)
		if err != nil {
			return "", err
		}
		if waiting {
			if err := waitForHeadlessProgress(ctx, page, deadline, signature); err != nil {
				return "", err
			}
		}

		time.Sleep(750 * time.Millisecond)
	}
}

func newBrowserPage(browserPath string, headless bool) (*rod.Browser, *rod.Page, func(), error) {
	l := launcher.New().
		Bin(browserPath).
		Leakless(false).
		Headless(headless).
		Set("disable-gpu").
		Set("disable-dev-shm-usage")
	if os.Geteuid() == 0 {
		l = l.NoSandbox(true)
	}

	u, err := l.Launch()
	if err != nil {
		return nil, nil, nil, fmt.Errorf(msg.AuthBrowserStartErr, err)
	}

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return nil, nil, nil, fmt.Errorf(msg.AuthBrowserConnectErr, err)
	}

	page, err := browser.Page(proto.TargetCreateTarget{URL: ""})
	if err != nil {
		browser.MustClose()
		return nil, nil, nil, fmt.Errorf(msg.AuthBrowserPageErr, err)
	}

	if _, err := page.EvalOnNewDocument(`(function() {
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
	})()`); err != nil {
		page.MustClose()
		browser.MustClose()
		return nil, nil, nil, fmt.Errorf(msg.AuthSAMLScriptErr, err)
	}

	cleanup := func() {
		page.MustClose()
		browser.MustClose()
	}

	return browser, page, cleanup, nil
}

func waitForSAML(ctx context.Context, page *rod.Page, timeoutMinutes int) (string, error) {
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
			saml, err := currentSAML(page)
			if err != nil {
				continue
			}
			if saml != "" {
				fmt.Println(msg.AuthComplete)
				return saml, nil
			}
		}
	}
}

func currentSAML(page *rod.Page) (string, error) {
	val, err := page.Eval(`() => window.__sogark_saml || ""`)
	if err != nil {
		return "", err
	}
	return val.Value.Str(), nil
}

func captureHeadlessSnapshot(page *rod.Page) (headlessSnapshot, error) {
	val, err := page.Eval(`() => JSON.stringify((() => {
		const norm = (value) => (value || "").replace(/\s+/g, " ").trim();
		const text = (value) => (value || "").replace(/\s+\n/g, "\n").replace(/\n{3,}/g, "\n\n").trim();
		const visible = (el) => {
			if (!el || !el.isConnected) return false;
			const style = window.getComputedStyle(el);
			if (!style || style.visibility === "hidden" || style.display === "none") return false;
			const rect = el.getBoundingClientRect();
			return rect.width > 0 && rect.height > 0;
		};
		const ensureSelector = (el, prefix, index) => {
			if (!el.dataset.sogarkId) {
				el.dataset.sogarkId = prefix + "-" + index + "-" + Math.random().toString(36).slice(2, 8);
			}
			return '[data-sogark-id="' + el.dataset.sogarkId + '"]';
		};
		const labelFor = (el) => {
			if (el.labels && el.labels.length) {
				const label = norm(Array.from(el.labels).map((item) => item.innerText).join(" "));
				if (label) return label;
			}
			const parentLabel = el.closest("label");
			if (parentLabel) {
				const label = norm(parentLabel.innerText);
				if (label) return label;
			}
			return norm(el.getAttribute("aria-label") || el.getAttribute("placeholder") || el.name || el.id || el.type || el.tagName);
		};

		const fields = [];
		const radioGroups = new Map();
		Array.from(document.querySelectorAll("input, textarea, select")).forEach((el, index) => {
			if (!visible(el) || el.disabled) return;

			const tag = el.tagName.toLowerCase();
			const inputType = tag === "input" ? (el.type || "text").toLowerCase() : tag;
			if (["hidden", "submit", "button", "image", "file", "reset"].includes(inputType)) return;

			const selector = ensureSelector(el, "field", index);
			const label = labelFor(el);

			if (inputType === "radio") {
				const key = el.name || selector;
				if (!radioGroups.has(key)) {
					radioGroups.set(key, {
						type: "radio",
						label: label || key,
						name: el.name || "",
						radioOptions: [],
					});
				}
				radioGroups.get(key).radioOptions.push({
					label: label || norm(el.value) || "option",
					value: el.value || "",
					selector,
					checked: !!el.checked,
				});
				return;
			}

			if (tag === "select") {
				fields.push({
					type: "select",
					selector,
					label,
					name: el.name || "",
					value: el.value || "",
					readOnly: !!el.readOnly,
					options: Array.from(el.options).map((option) => ({
						label: norm(option.textContent),
						value: option.value,
						selected: option.selected,
					})),
				});
				return;
			}

			fields.push({
				type: inputType,
				selector,
				label,
				name: el.name || "",
				value: inputType === "password" ? "" : (el.value || ""),
				readOnly: !!el.readOnly,
				checked: inputType === "checkbox" ? !!el.checked : false,
			});
		});

		radioGroups.forEach((value) => fields.push(value));

		const buttons = Array.from(document.querySelectorAll("button, input[type=submit], input[type=button], [role=button]"))
			.filter((el) => visible(el) && !el.disabled)
			.map((el, index) => ({
				selector: ensureSelector(el, "action", index),
				label: norm(el.innerText || el.value || el.getAttribute("aria-label") || el.getAttribute("title") || "action"),
			}))
			.filter((button) => button.label)
			.slice(0, 8);

		return {
			title: document.title || "",
			url: location.href || "",
			text: text(document.body ? document.body.innerText : "").slice(0, 2000),
			fields,
			buttons,
		};
	})())`)
	if err != nil {
		return headlessSnapshot{}, err
	}

	raw := val.Value.Str()
	var snapshot headlessSnapshot
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		return headlessSnapshot{}, err
	}
	return snapshot, nil
}

func (s headlessSnapshot) hasInteractiveControls() bool {
	return len(s.Fields) > 0 || len(s.Buttons) > 0
}

func (s headlessSnapshot) signature() string {
	var b strings.Builder
	b.WriteString(s.Title)
	b.WriteString("|")
	b.WriteString(s.URL)
	b.WriteString("|t:")
	b.WriteString(s.Text)
	for _, field := range s.Fields {
		b.WriteString("|f:")
		b.WriteString(field.Type)
		b.WriteString(":")
		b.WriteString(field.Label)
	}
	for _, button := range s.Buttons {
		b.WriteString("|b:")
		b.WriteString(button.Label)
	}
	return b.String()
}

func waitForHeadlessProgress(ctx context.Context, page *rod.Page, deadline time.Time, lastSignature string) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf(msg.AuthSAMLTimeout)
		}
		if saml, err := currentSAML(page); err == nil && saml != "" {
			return nil
		}
		snapshot, err := captureHeadlessSnapshot(page)
		if err == nil && snapshot.signature() != lastSignature {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func printHeadlessSnapshot(snapshot headlessSnapshot) {
	fmt.Println()
	fmt.Printf(msg.AuthHeadlessPageTitle, snapshot.Title)
	if snapshot.URL != "" {
		fmt.Printf(msg.AuthHeadlessPageURL, snapshot.URL)
	}
	if snapshot.Text != "" {
		fmt.Println(msg.AuthHeadlessPageText)
		fmt.Println(indentLines(limitLines(snapshot.Text, 12)))
	}
	if len(snapshot.Fields) > 0 {
		fmt.Println(msg.AuthHeadlessFields)
		for idx, field := range snapshot.Fields {
			switch field.Type {
			case "radio":
				fmt.Printf("  %d. %s\n", idx+1, fieldLabel(field))
				for optIdx, option := range field.RadioOptions {
					marker := " "
					if option.Checked {
						marker = "*"
					}
					fmt.Printf("      [%s] %d) %s\n", marker, optIdx+1, option.Label)
				}
			case "select":
				fmt.Printf("  %d. %s\n", idx+1, fieldLabel(field))
				for optIdx, option := range field.Options {
					marker := " "
					if option.Selected {
						marker = "*"
					}
					fmt.Printf("      [%s] %d) %s\n", marker, optIdx+1, option.Label)
				}
			case "checkbox":
				state := "off"
				if field.Checked {
					state = "on"
				}
				fmt.Printf("  %d. %s [%s]\n", idx+1, fieldLabel(field), state)
			default:
				fmt.Printf("  %d. %s\n", idx+1, fieldLabel(field))
			}
		}
	}
	if len(snapshot.Buttons) > 0 {
		fmt.Println(msg.AuthHeadlessActions)
		for idx, button := range snapshot.Buttons {
			fmt.Printf("  %d. %s\n", idx+1, button.Label)
		}
	}
}

func fillHeadlessFields(page *rod.Page, reader *bufio.Reader, fields []headlessField) error {
	for _, field := range fields {
		if field.ReadOnly {
			continue
		}
		switch field.Type {
		case "checkbox":
			answer := strings.ToLower(strings.TrimSpace(promptInput(reader, fieldLabel(field)+" [y/n]", yesNoValue(field.Checked))))
			if answer == "" {
				continue
			}
			checked := answer == "y" || answer == "yes" || answer == "1" || answer == "true"
			if err := setChecked(page, field.Selector, checked); err != nil {
				return err
			}
		case "radio":
			defaultChoice := ""
			for idx, option := range field.RadioOptions {
				if option.Checked {
					defaultChoice = strconv.Itoa(idx + 1)
					break
				}
			}
			answer := strings.TrimSpace(promptInput(reader, fieldLabel(field)+" [choose option, blank keeps current]", defaultChoice))
			if answer == "" {
				continue
			}
			index, err := strconv.Atoi(answer)
			if err != nil || index < 1 || index > len(field.RadioOptions) {
				return fmt.Errorf(msg.AuthHeadlessInvalidChoice, answer)
			}
			if err := setChecked(page, field.RadioOptions[index-1].Selector, true); err != nil {
				return err
			}
		case "select":
			defaultChoice := ""
			for idx, option := range field.Options {
				if option.Selected {
					defaultChoice = strconv.Itoa(idx + 1)
					break
				}
			}
			answer := strings.TrimSpace(promptInput(reader, fieldLabel(field)+" [choose option, blank keeps current]", defaultChoice))
			if answer == "" {
				continue
			}
			index, err := strconv.Atoi(answer)
			if err != nil || index < 1 || index > len(field.Options) {
				return fmt.Errorf(msg.AuthHeadlessInvalidChoice, answer)
			}
			if err := setValue(page, field.Selector, field.Options[index-1].Value); err != nil {
				return err
			}
		case "password":
			answer, err := promptSecret(reader, fieldLabel(field))
			if err != nil {
				return err
			}
			if answer == "" {
				continue
			}
			if err := setValue(page, field.Selector, answer); err != nil {
				return err
			}
		default:
			answer := promptInput(reader, fieldLabel(field), field.Value)
			if answer == "" && field.Value == "" {
				continue
			}
			if answer == "" {
				answer = field.Value
			}
			if err := setValue(page, field.Selector, answer); err != nil {
				return err
			}
		}
	}
	return nil
}

func runHeadlessAction(page *rod.Page, reader *bufio.Reader, snapshot headlessSnapshot) (bool, error) {
	if len(snapshot.Buttons) == 0 {
		answer := strings.ToLower(strings.TrimSpace(promptInput(reader, msg.AuthHeadlessSubmitPrompt, "")))
		if answer == "w" || answer == "wait" {
			return true, nil
		}
		return false, page.Keyboard.Press(input.Enter)
	}

	defaultChoice := ""
	if len(snapshot.Buttons) == 1 {
		defaultChoice = "1"
	}
	answer := strings.ToLower(strings.TrimSpace(promptInput(reader, msg.AuthHeadlessActionPrompt, defaultChoice)))
	if answer == "" {
		answer = defaultChoice
	}
	if answer == "" || answer == "w" || answer == "wait" {
		return true, nil
	}

	index, err := strconv.Atoi(answer)
	if err != nil || index < 1 || index > len(snapshot.Buttons) {
		return false, fmt.Errorf(msg.AuthHeadlessInvalidChoice, answer)
	}
	return false, clickSelector(page, snapshot.Buttons[index-1].Selector)
}

func setValue(page *rod.Page, selector, value string) error {
	val, err := page.Eval(`(selector, value) => {
		const el = document.querySelector(selector);
		if (!el) return "field not found";
		el.focus();
		if (el.tagName === "SELECT") {
			el.value = value;
		} else {
			el.value = value;
		}
		el.dispatchEvent(new Event("input", { bubbles: true }));
		el.dispatchEvent(new Event("change", { bubbles: true }));
		return "";
	}`, selector, value)
	if err != nil {
		return err
	}
	if msgText := val.Value.Str(); msgText != "" {
		return fmt.Errorf(msg.AuthHeadlessActionFailed, msgText)
	}
	return nil
}

func setChecked(page *rod.Page, selector string, checked bool) error {
	val, err := page.Eval(`(selector, checked) => {
		const el = document.querySelector(selector);
		if (!el) return "field not found";
		el.focus();
		el.checked = checked;
		el.dispatchEvent(new Event("input", { bubbles: true }));
		el.dispatchEvent(new Event("change", { bubbles: true }));
		return "";
	}`, selector, checked)
	if err != nil {
		return err
	}
	if msgText := val.Value.Str(); msgText != "" {
		return fmt.Errorf(msg.AuthHeadlessActionFailed, msgText)
	}
	return nil
}

func clickSelector(page *rod.Page, selector string) error {
	val, err := page.Eval(`(selector) => {
		const el = document.querySelector(selector);
		if (!el) return "action not found";
		el.click();
		return "";
	}`, selector)
	if err != nil {
		return err
	}
	if msgText := val.Value.Str(); msgText != "" {
		return fmt.Errorf(msg.AuthHeadlessActionFailed, msgText)
	}
	return nil
}

func fieldLabel(field headlessField) string {
	label := strings.TrimSpace(field.Label)
	if label == "" {
		label = strings.TrimSpace(field.Name)
	}
	if label == "" {
		label = field.Type
	}
	return label
}

func promptInput(reader *bufio.Reader, label, defaultValue string) string {
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", label, defaultValue)
	} else {
		fmt.Printf("%s: ", label)
	}
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func promptSecret(reader *bufio.Reader, label string) (string, error) {
	fmt.Printf("%s: ", label)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		value, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		return strings.TrimSpace(string(value)), err
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func yesNoValue(v bool) string {
	if v {
		return "y"
	}
	return "n"
}

func limitLines(text string, n int) string {
	lines := strings.Split(text, "\n")
	if len(lines) <= n {
		return text
	}
	return strings.Join(lines[:n], "\n")
}

func indentLines(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = "    " + line
	}
	return strings.Join(lines, "\n")
}

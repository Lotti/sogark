//go:build linux

package messages

const (
	AuthBrowserOverrideMissing   = "configured browser not found: %s"
	AuthLinuxGUIRequired         = "Linux SAML login requires a graphical browser session (DISPLAY/WAYLAND_DISPLAY not found)."
	AuthLinuxHeadlessUnsupported = "Linux terminal/headless SAML login is currently unsupported. Start a graphical session and use SOGARK_AUTH_MODE=gui (or leave it unset)."
	AuthModeInvalid              = "invalid SOGARK_AUTH_MODE=%q (valid: auto, gui, headless)"
)

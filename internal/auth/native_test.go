package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Lotti/sogark/internal/config"
	"github.com/Lotti/sogark/internal/keys"
)

type recordingQR struct {
	images []string
	output strings.Builder
}

func (p *recordingQR) DisplayQR(image string) error {
	p.images = append(p.images, image)
	return nil
}

func (p *recordingQR) Printf(format string, args ...any) {
	fmt.Fprintf(&p.output, format, args...)
}

func nativeProfile(base, authType string) config.AuthProfile {
	return config.AuthProfile{
		AuthType: authType, TenantID: "EXAMPLE",
		PVWABaseURL:              base + "/vault",
		StartAuthenticationURL:   base + "/identity/start",
		AdvanceAuthenticationURL: base + "/identity/advance",
		SAMLBootstrapURL:         base + "/vault/bootstrap",
		SAMLLogonURL:             base + "/vault/logon",
		OIDCAuthorizeURL:         base + "/vault/authorize",
		OIDCTokenURL:             base + "/vault/token",
		SSHKeysCacheURL:          base + "/vault/cache",
	}
}

func writeIdentity(t *testing.T, w http.ResponseWriter, result identityResult) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(identityResponse{Success: true, Result: result}); err != nil {
		t.Error(err)
	}
}

func requireCookie(t *testing.T, r *http.Request, name, value string) {
	t.Helper()
	cookie, err := r.Cookie(name)
	if err != nil || cookie.Value != value {
		t.Errorf("missing expected %s cookie", name)
	}
}

var keyFixture = sshKeysResponse{Value: []sshKeyEntry{
	{Format: "OpenSSH", PrivateKey: "-----BEGIN OPENSSH PRIVATE KEY-----\nfixture\n-----END OPENSSH PRIVATE KEY-----"},
	{Format: "PEM", PrivateKey: "-----BEGIN PRIVATE KEY-----\nfixture\n-----END PRIVATE KEY-----"},
	{Format: "PPK", PrivateKey: "PuTTY-User-Key-File-3: ssh-rsa\nEncryption: none\nPrivate-MAC: abcdef"},
}}

func TestNativeQRPushPVWAAndKeys(t *testing.T) {
	for _, authType := range []string{"saml", "oidc"} {
		t.Run(authType, func(t *testing.T) {
			var advances, oidcForms atomic.Int32
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/identity/start", "/identity/advance":
					if r.Method != http.MethodPost || r.Header.Get("X-IDAP-NATIVE-CLIENT") != "true" ||
						r.Header.Get("OobIdPAuth") != "" || r.Header.Get("Content-Type") != "application/json" {
						t.Error("Identity method/headers differ from the reference contract")
					}
					var body map[string]string
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
						return
					}
					if r.URL.Path == "/identity/start" {
						expected := map[string]string{"TenantId": "EXAMPLE", "User": "user", "Version": "1.0"}
						if !reflect.DeepEqual(body, expected) {
							t.Errorf("unexpected start payload fields: %v", body)
						}
						http.SetCookie(w, &http.Cookie{Name: "identity-session", Value: "idp", Path: "/identity/"})
						writeIdentity(t, w, identityResult{SessionID: "session", Challenges: []identityChallenge{
							{Mechanisms: []identityMechanism{{Name: "QR", MechanismID: "qr", Image: "fixture-image"}}},
						}})
						return
					}
					requireCookie(t, r, "identity-session", "idp")
					step := advances.Add(1)
					mechanism, action := "qr", "StartOOB"
					switch step {
					case 2:
						action = "Poll"
					case 3:
						mechanism = "otp"
					case 4:
						mechanism, action = "otp", "Poll"
					}
					expected := map[string]string{"TenantId": "EXAMPLE", "SessionId": "session", "MechanismId": mechanism, "Action": action}
					if !reflect.DeepEqual(body, expected) {
						t.Errorf("advance %d differs from reference: %v", step, body)
					}
					switch step {
					case 1:
						writeIdentity(t, w, identityResult{Summary: "OobPending"})
					case 2, 3:
						summary := "NewPackage"
						if step == 3 {
							summary = "StartNextChallenge"
						}
						writeIdentity(t, w, identityResult{Summary: summary, Challenges: []identityChallenge{
							{Mechanisms: []identityMechanism{{Name: "OTP", MechanismID: "otp"}}},
						}})
					case 4:
						writeIdentity(t, w, identityResult{Summary: "LoginSuccess"})
					default:
						t.Error("push was started twice or polling continued after success")
						http.Error(w, "invalid step", 400)
					}
				case "/vault/bootstrap":
					if authType != "saml" || r.Method != http.MethodPost {
						t.Error("unexpected SAML bootstrap")
					}
					_ = r.ParseForm()
					if !reflect.DeepEqual(r.PostForm, samlForm("")) {
						t.Error("SAML bootstrap must include an explicitly empty assertion")
					}
					http.SetCookie(w, &http.Cookie{Name: "pvwa-bootstrap", Value: "bootstrap", Path: "/vault/"})
					_ = json.NewEncoder(w).Encode(server.URL + "/identity/assertion?state=synthetic-private-state")
				case "/identity/assertion":
					requireCookie(t, r, "identity-session", "idp")
					fmt.Fprint(w, `<form><INPUT VALUE='assertion&#x2b;with&#x3d;entities' TYPE='hidden' NAME='SAMLResponse'></form>`)
				case "/vault/logon":
					requireCookie(t, r, "pvwa-bootstrap", "bootstrap")
					_ = r.ParseForm()
					if r.Method != http.MethodPost || !reflect.DeepEqual(r.PostForm, samlForm("assertion+with=entities")) {
						t.Error("SAML logon form differs from reference")
					}
					http.SetCookie(w, &http.Cookie{Name: "pvwa-session", Value: "pvwa", Path: "/vault/"})
					fmt.Fprint(w, `"pvwa-token"`)
				case "/vault/authorize":
					if authType != "oidc" || r.Method != http.MethodGet {
						t.Error("unexpected OIDC authorize")
					}
					requireCookie(t, r, "PreferredCulture", "en")
					http.Redirect(w, r, server.URL+"/identity/oidc-form?state=synthetic-private-state", http.StatusFound)
				case "/identity/oidc-form":
					requireCookie(t, r, "identity-session", "idp")
					oidcForms.Add(1)
					fmt.Fprint(w, `<input value="code&#x2b;value" name="code"><input name='state' value='state&amp;value'>`)
				case "/vault/token":
					_ = r.ParseForm()
					if r.Method != http.MethodPost || r.PostForm.Get("code") != "code+value" || r.PostForm.Get("state") != "state&value" {
						t.Error("OIDC callback form differs from reference")
					}
					http.SetCookie(w, &http.Cookie{Name: "CA66666", Value: "oidc-session", Path: "/vault/"})
					fmt.Fprint(w, `{}`)
				case "/vault/cache":
					if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
						t.Error("invalid key cache method/content type")
					}
					if authType == "saml" {
						requireCookie(t, r, "pvwa-session", "pvwa")
						if r.Header.Get("Authorization") != "pvwa-token" || r.Header.Get("x-ca66666") != "" {
							t.Error("SAML cache credentials differ from reference")
						}
					} else {
						requireCookie(t, r, "CA66666", "oidc-session")
						if r.Header.Get("x-ca66666") != "oidc-session" || r.Header.Get("Authorization") != "" ||
							r.Header.Get("Accept") != "application/json; flat=true; version=1.0" {
							t.Error("OIDC cache credentials differ from reference")
						}
					}
					var payload map[string][]string
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil ||
						!reflect.DeepEqual(payload["formats"], []string{"OpenSSH", "PEM", "PPK"}) {
						t.Error("invalid requested key formats")
					}
					_ = json.NewEncoder(w).Encode(keyFixture)
				default:
					t.Error("client used an endpoint not configured by the profile")
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			client, err := NewNativeClient(nativeProfile(server.URL, authType))
			if err != nil {
				t.Fatal(err)
			}
			client.pollInterval = time.Millisecond
			prompt := &recordingQR{}
			if err := client.AuthenticateIdentity(context.Background(), "user", prompt); err != nil {
				t.Fatal(err)
			}
			if len(prompt.images) != 1 || prompt.images[0] != "fixture-image" || advances.Load() != 4 {
				t.Fatal("QR/push flow was not completed exactly once")
			}
			if err := client.EstablishPVWA(context.Background()); err != nil {
				t.Fatal(err)
			}
			raw, err := client.FetchSSHKeys(context.Background(), []string{"OpenSSH", "PEM", "PPK"})
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := keys.Parse(raw)
			if err != nil {
				t.Fatal(err)
			}
			if err := parsed.RequireFormats([]string{"OpenSSH", "PEM", "PPK"}); err != nil {
				t.Fatal(err)
			}
			if authType == "oidc" && oidcForms.Load() != 2 {
				t.Fatal("OIDC must preserve the reference's redirect and subsequent GET")
			}
		})
	}
}

func TestNativeDoesNotFallBackWhenQRIsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/identity/start" {
			t.Error("client must not answer a different mechanism")
		}
		writeIdentity(t, w, identityResult{SessionID: "session", Challenges: []identityChallenge{
			{Mechanisms: []identityMechanism{{Name: "UP", MechanismID: "password"}}},
		}})
	}))
	defer server.Close()
	client, _ := NewNativeClient(nativeProfile(server.URL, "saml"))
	err := client.AuthenticateIdentity(context.Background(), "user", &recordingQR{})
	if err == nil || !strings.Contains(err.Error(), "selected profile") || client.authenticated {
		t.Fatalf("expected an actionable profile/QR error, got %v", err)
	}
}

func TestNativeApprovalTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := identityResult{Summary: "OobPending"}
		if r.URL.Path == "/identity/start" {
			result.SessionID = "session"
			result.Challenges = []identityChallenge{{Mechanisms: []identityMechanism{{Name: "QR", MechanismID: "qr", Image: "image"}}}}
		}
		writeIdentity(t, w, result)
	}))
	defer server.Close()
	client, _ := NewNativeClient(nativeProfile(server.URL, "saml"))
	client.pollInterval = time.Hour
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := client.AuthenticateIdentity(ctx, "user", &recordingQR{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected cancellable timeout, got %v", err)
	}
}

func TestNativeExternalRedirectIsBlockedAndRedacted(t *testing.T) {
	var externalRequests atomic.Int32
	external := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		externalRequests.Add(1)
	}))
	defer external.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, external.URL+"/access?state=PRIVATE-STATE&code=PRIVATE-CODE", http.StatusFound)
	}))
	defer server.Close()
	client, _ := NewNativeClient(nativeProfile(server.URL, "oidc"))
	client.authenticated = true
	err := client.EstablishPVWA(context.Background())
	if err == nil || !strings.Contains(err.Error(), "access-gateway") {
		t.Fatalf("expected a gateway diagnostic, got %v", err)
	}
	if externalRequests.Load() != 0 || strings.Contains(err.Error(), "PRIVATE") || strings.Contains(err.Error(), "?") {
		t.Fatal("external redirect was followed or its state leaked")
	}
}

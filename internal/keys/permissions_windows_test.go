package keys

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestSaveWindowsPrivateACL(t *testing.T) {
	for _, existing := range []bool{false, true} {
		name := "new"
		if existing {
			name = "replacement"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;WD)")
			if err != nil {
				t.Fatal(err)
			}
			dacl, _, err := sd.DACL()
			if err != nil {
				t.Fatal(err)
			}
			if err := windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT,
				windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
				nil, nil, dacl, nil); err != nil {
				t.Fatal(err)
			}
			openssh, pem, ppk := FileNames("testkey")
			if existing {
				for _, name := range []string{openssh, pem, ppk} {
					if err := os.WriteFile(filepath.Join(dir, name), []byte("old"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			results, err := Save(&Parsed{OpenSSH: "openssh", PEM: "pem", PPK: "ppk"}, dir, "testkey", []string{"OpenSSH", "PEM", "PPK"})
			if err != nil {
				t.Fatal(err)
			}
			user, err := windows.GetCurrentProcessToken().GetTokenUser()
			if err != nil {
				t.Fatal(err)
			}
			for _, result := range results {
				sd, err := windows.GetNamedSecurityInfo(result.Path, windows.SE_FILE_OBJECT,
					windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
				if err != nil {
					t.Fatal(err)
				}
				owner, _, err := sd.Owner()
				if err != nil || owner == nil || !owner.Equals(user.User.Sid) {
					t.Fatalf("%s does not belong to current user: %v", result.Format, err)
				}
				control, _, err := sd.Control()
				if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
					t.Fatalf("%s inherits its DACL: %v", result.Format, err)
				}
				dacl, _, err := sd.DACL()
				if err != nil || dacl == nil || dacl.AceCount != 1 {
					t.Fatalf("%s must have exactly one explicit user ACE: %v", result.Format, err)
				}
				var ace *windows.ACCESS_ALLOWED_ACE
				if err := windows.GetAce(dacl, 0, &ace); err != nil {
					t.Fatal(err)
				}
				sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
				if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE ||
					ace.Header.AceFlags&windows.INHERITED_ACE != 0 ||
					!sid.Equals(user.User.Sid) ||
					uint32(ace.Mask)&windows.FILE_GENERIC_READ != windows.FILE_GENERIC_READ {
					t.Fatalf("%s has an unexpected key access rule", result.Format)
				}
			}
		})
	}
}

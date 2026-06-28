package main

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/Lotti/sogark/internal/auth"
	"github.com/Lotti/sogark/internal/config"
	msg "github.com/Lotti/sogark/internal/messages"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: msg.DoctorShort,
		Long:  msg.DoctorLong,
		RunE: func(cmd *cobra.Command, args []string) error {
			issues := 0

			if path, err := config.Path(); err == nil {
				fmt.Printf(msg.DoctorCheckInfo, "config path", path)
			}

			cfg, err := config.Load()
			if err != nil {
				fmt.Printf(msg.DoctorCheckFailed, "configuration", err)
				issues++
			} else {
				if err := cfg.Validate(); err != nil {
					fmt.Printf(msg.DoctorCheckFailed, "configuration", err)
					issues++
				} else {
					fmt.Printf(msg.DoctorCheckOK, "configuration", "valid")
				}

				if keyDir, err := cfg.ResolveKeyDir(); err != nil {
					fmt.Printf(msg.DoctorCheckFailed, "key directory", err)
					issues++
				} else {
					fmt.Printf(msg.DoctorCheckInfo, "key directory", keyDir)
				}

				fmt.Printf(msg.DoctorCheckInfo, "update source", cfg.ResolvedUpdateRepo())
			}

			if path, err := exec.LookPath("ssh"); err != nil {
				fmt.Printf(msg.DoctorCheckFailed, "ssh client", err)
				issues++
			} else {
				fmt.Printf(msg.DoctorCheckOK, "ssh client", path)
			}

			if path, err := auth.SAMLPrerequisite(); err != nil {
				fmt.Printf(msg.DoctorCheckFailed, "saml prerequisite", err)
				issues++
			} else {
				fmt.Printf(msg.DoctorCheckOK, "saml prerequisite", path)
			}

			if runtime.GOOS != "windows" {
				if path, err := exec.LookPath("tmux"); err == nil {
					fmt.Printf(msg.DoctorCheckInfo, "tmux", path)
				}
			}

			if issues > 0 {
				return fmt.Errorf(msg.DoctorFoundIssues, issues)
			}

			fmt.Println(msg.DoctorHealthy)
			return nil
		},
	}
}

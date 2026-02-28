// Package hosts provides PuTTY session management on Windows.
// This file is a no-op on non-Windows platforms.
//go:build !windows

package hosts

import "fmt"

// UpdatePuTTYSession is a no-op on non-Windows platforms.
func UpdatePuTTYSession(host *Host, username, proxyHost, keyPath string) error {
	return fmt.Errorf("sessioni PuTTY supportate solo su Windows")
}

// RemovePuTTYSession is a no-op on non-Windows platforms.
func RemovePuTTYSession(hostName string) error {
	return fmt.Errorf("sessioni PuTTY supportate solo su Windows")
}

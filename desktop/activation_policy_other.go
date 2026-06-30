//go:build !darwin

package main

// Activation policy is a macOS concept; these are no-ops elsewhere.
func setMacActivationRegular()   {}
func setMacActivationAccessory() {}

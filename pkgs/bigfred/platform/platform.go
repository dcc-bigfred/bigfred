// Package platform reports build-target capabilities for BigFred.
// Call sites should prefer Supports* helpers over IsAndroid so feature
// gates stay readable and Android knowledge stays in one place.
package platform

import "runtime"

// IsAndroid reports whether this binary was built with GOOS=android
// (phone loco-server from `make android` / CI).
func IsAndroid() bool {
	return runtime.GOOS == "android"
}

// SupportsLocoNetSerial is false on Android (no usable USB serial stack).
func SupportsLocoNetSerial() bool {
	return !IsAndroid()
}

// SupportsMicrodns is false on Android: phone builds do not ship or run
// the microdns daemon, so loco-server must not seed microdns.json.
func SupportsMicrodns() bool {
	return !IsAndroid()
}

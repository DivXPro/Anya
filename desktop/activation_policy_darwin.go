//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -mmacosx-version-min=10.13
#cgo LDFLAGS: -framework Cocoa -mmacosx-version-min=10.13
#import <Cocoa/Cocoa.h>

// setMacActivationPolicy changes the running app's activation policy at runtime.
// It is dispatched onto the main queue because AppKit must be touched there.
//   0 = NSApplicationActivationPolicyRegular   (Dock icon + menu bar)
//   1 = NSApplicationActivationPolicyAccessory  (tray-only, no Dock/menu bar)
static void setMacActivationPolicy(int policy) {
	dispatch_async(dispatch_get_main_queue(), ^{
		[NSApp setActivationPolicy:(NSApplicationActivationPolicy)policy];
		if (policy == 0) {
			// Bring the app (and its menu bar) to the front when promoting.
			[NSApp activateIgnoringOtherApps:YES];
		}
	});
}
*/
import "C"

// The app launches as an accessory (tray-only: no Dock icon, no menu bar). We
// promote it to a regular app while the main window is visible so the standard
// macOS menu bar appears, then demote it back to accessory when the window hides.
func setMacActivationRegular() {
	C.setMacActivationPolicy(C.int(0))
}

func setMacActivationAccessory() {
	C.setMacActivationPolicy(C.int(1))
}

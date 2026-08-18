package app

import (
	"strings"
	"time"
)

const googleAuthExpiredStatusMessage = "Google Messages session cookie expired; refreshing and reconnecting..."

// googleAuthExpiredManualMessage is the honest variant for installs with no
// cookie refresh mechanism: nothing is going to reconnect on its own, and the
// operator needs to know what to run.
const googleAuthExpiredManualMessage = "Google Messages session cookie expired; no cookie refresh mechanism is configured, so a manual re-pair is required (run `openmessage pair --google` with fresh cookies)."

// googleAuthReminderEvery paces the log reminder while the session stays
// expired. Auth expiry parks the supervisor deliberately (retrying can't mint
// new cookies), but parking silently buries the incident: the first observed
// one sat unnoticed for 12 hours until a scheduled morning send failed.
const googleAuthReminderEvery = time.Hour

// SetGoogleCookieRefreshAvailable records whether the install can refresh
// cookies on its own (refresh script or native support). Serve wires this at
// startup; it decides which auth-expiry status message is truthful.
func (a *App) SetGoogleCookieRefreshAvailable(available bool) {
	a.googleCookieRefreshAvailable.Store(available)
}

func (a *App) googleAuthExpiredMessage() string {
	if a.googleCookieRefreshAvailable.Load() {
		return googleAuthExpiredStatusMessage
	}
	return googleAuthExpiredManualMessage
}

// IsGoogleAuthExpiredError reports whether a Google Messages API error means
// the linked-device web cookies/session are no longer accepted by Google.
func IsGoogleAuthExpiredError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "http 401") ||
		strings.Contains(msg, "session_cookie_invalid") ||
		strings.Contains(msg, "session cookie expired") ||
		strings.Contains(msg, "invalid authentication credentials")
}

// HandleGoogleAuthExpiredError marks Google disconnected for auth-expiry errors
// so the reconnect watchdog can refresh cookies and reconnect. It deliberately
// does NOT clear the needs-repair flag: whether an expired session is
// recoverable depends on a cookie-refresh script being configured, and that
// decision lives in the watchdog (see the credential repairer wiring in serve).
// Clearing it here would defeat the watchdog's back-off and, with no refresh
// script (e.g. the macOS app), spin a reconnect storm against Google's auth
// endpoint.
func (a *App) HandleGoogleAuthExpiredError(err error) bool {
	if !IsGoogleAuthExpiredError(err) {
		return false
	}
	a.Connected.Store(false)
	firstNotification := a.googleAuthExpired.CompareAndSwap(false, true)
	a.setGoogleLastError(a.googleAuthExpiredMessage())
	a.emitStatusChange(false)
	a.Logger.Warn().Err(err).Msg("Google auth expired; marking disconnected")
	if firstNotification {
		a.reportGoogleLifecycleError(err)
	}
	a.startGoogleAuthReminder()
	return true
}

// startGoogleAuthReminder keeps the expired session visible in the logs until
// it is resolved. Without it the incident is one WRN line at expiry time and
// then silence — on an install with no refresh mechanism that silence lasts
// until the next scheduled send fails.
func (a *App) startGoogleAuthReminder() {
	if !a.googleAuthReminderRunning.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer a.googleAuthReminderRunning.Store(false)
		ticker := time.NewTicker(googleAuthReminderEvery)
		defer ticker.Stop()
		for range ticker.C {
			if !a.googleAuthExpired.Load() {
				return
			}
			a.Logger.Warn().Msg(a.googleAuthExpiredMessage())
		}
	}()
}

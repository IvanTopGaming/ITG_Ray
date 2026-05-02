package bindings

import (
	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
	"github.com/itg-team/itg-ray/internal/hwid"
	"github.com/itg-team/itg-ray/internal/subscription"
)

// resolveIdentity merges global SubscriptionSettings with a per-sub override
// (Stored.UserAgent) and the hwid package's outputs to produce the 5
// identity strings consumed by Subscription.UserAgent / .HWID / etc.
//
// Rules:
//   - UserAgent: per-sub overrides global; if both empty, last-resort
//     "ITGRay/dev" so we never send Go's default "Go-http-client/1.1".
//   - HWID: gated by settings.HWIDEnabled; if false, all 4 ID fields empty.
//   - DeviceOS / Version / Model: each gated by its own settings flag AND
//     the master HWIDEnabled flag (master off = all metadata off).
func resolveIdentity(
	settings hub.SubscriptionSettings,
	sub subscription.Stored,
	hwidValue string,
	info hwid.DeviceInfo,
) (ua, hwid_, os_, ver, model string) {
	ua = sub.UserAgent
	if ua == "" {
		ua = settings.UserAgent
	}
	if ua == "" {
		ua = "ITGRay/dev"
	}
	if !settings.HWIDEnabled {
		return ua, "", "", "", ""
	}
	hwid_ = hwidValue
	if settings.SendDeviceOS {
		os_ = info.OS
	}
	if settings.SendOSVersion {
		ver = info.Version
	}
	if settings.SendDeviceModel {
		model = info.Model
	}
	return
}

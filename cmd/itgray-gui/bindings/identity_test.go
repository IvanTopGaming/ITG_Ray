package bindings

import (
	"testing"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
	"github.com/itg-team/itg-ray/internal/hwid"
	"github.com/itg-team/itg-ray/internal/subscription"
	"github.com/stretchr/testify/require"
)

func base() (hub.SubscriptionSettings, hwid.DeviceInfo) {
	return hub.SubscriptionSettings{
			UserAgent:       "ITGRay/1.0",
			HWIDEnabled:     true,
			SendDeviceOS:    true,
			SendOSVersion:   true,
			SendDeviceModel: true,
		}, hwid.DeviceInfo{
			OS: "Linux", Version: "Ubuntu 24.04", Model: "MacBookPro18,2",
		}
}

func TestResolveIdentity_AllOn_FullSet(t *testing.T) {
	settings, info := base()
	sub := subscription.Stored{}
	ua, h, os_, ver, model := resolveIdentity(settings, sub, "abcd1234", info)
	require.Equal(t, "ITGRay/1.0", ua)
	require.Equal(t, "abcd1234", h)
	require.Equal(t, "Linux", os_)
	require.Equal(t, "Ubuntu 24.04", ver)
	require.Equal(t, "MacBookPro18,2", model)
}

func TestResolveIdentity_HWIDDisabled_AllIDFieldsEmpty(t *testing.T) {
	settings, info := base()
	settings.HWIDEnabled = false
	sub := subscription.Stored{}
	ua, h, os_, ver, model := resolveIdentity(settings, sub, "abcd", info)
	require.Equal(t, "ITGRay/1.0", ua)
	require.Empty(t, h)
	require.Empty(t, os_)
	require.Empty(t, ver)
	require.Empty(t, model)
}

func TestResolveIdentity_PerSubUAOverridesGlobal(t *testing.T) {
	settings, info := base()
	sub := subscription.Stored{UserAgent: "Custom/9.9"}
	ua, _, _, _, _ := resolveIdentity(settings, sub, "abcd", info)
	require.Equal(t, "Custom/9.9", ua)
}

func TestResolveIdentity_PerSubUAEmpty_FallsBackToGlobal(t *testing.T) {
	settings, info := base()
	sub := subscription.Stored{UserAgent: ""}
	ua, _, _, _, _ := resolveIdentity(settings, sub, "abcd", info)
	require.Equal(t, "ITGRay/1.0", ua)
}

func TestResolveIdentity_PartialMetadata(t *testing.T) {
	settings, info := base()
	settings.SendDeviceOS = false
	settings.SendDeviceModel = false
	sub := subscription.Stored{}
	_, _, os_, ver, model := resolveIdentity(settings, sub, "abcd", info)
	require.Empty(t, os_, "os disabled")
	require.Equal(t, "Ubuntu 24.04", ver)
	require.Empty(t, model, "model disabled")
}

func TestResolveIdentity_AllUAEmpty_FallsBackToITGRayDev(t *testing.T) {
	settings, info := base()
	settings.UserAgent = ""
	sub := subscription.Stored{UserAgent: ""}
	ua, _, _, _, _ := resolveIdentity(settings, sub, "abcd", info)
	require.Equal(t, "ITGRay/dev", ua, "last-resort default when both UA fields empty")
}

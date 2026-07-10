package protocol

import (
	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/rules"
)

// Each method's argument struct ends in "Params"; result type either ends
// in "Result" or is a domain type re-exported from another package
// (e.g. hub.Snapshot). Methods returning zero values use Empty.
type Empty struct{}

// AppService — methods under "app." namespace.
type AppService interface {
	Ping(p Empty) (PingResult, error)
	GetSnapshot(p Empty) (hub.Snapshot, error)
	GetPublicIP(p Empty) (string, error)
}

type PingResult struct {
	Pong    int64  `json:"pong"`
	Version string `json:"version"`
}

// RunService — methods under "run." namespace.
type RunService interface {
	Connect(p RunConnectParams) (Empty, error)
	Disconnect(p Empty) (Empty, error)
	Reconnect(p RunReconnectParams) (Empty, error)
	SwitchMode(p RunSwitchModeParams) (Empty, error)
}

type RunConnectParams struct {
	ServerID string `json:"serverId"`
	Mode     string `json:"mode"`
}

type RunReconnectParams struct {
	ServerID string `json:"serverId"`
	Mode     string `json:"mode"`
}

type RunSwitchModeParams struct {
	Mode string `json:"mode"`
}

// ServersService — methods under "servers." namespace.
type ServersService interface {
	List(p Empty) ([]hub.ServerView, error)
	Add(p ServersAddParams) (hub.ServerView, error)
	Edit(p ServersEditParams) (ServersEditResult, error)
	Remove(p ServersRemoveParams) (Empty, error)
	ToggleFavorite(p ServersToggleFavoriteParams) (Empty, error)
	TestLatency(p ServersTestLatencyParams) (Empty, error)
}

type ServersAddParams struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

type ServersEditParams struct {
	ID   string `json:"id"`
	URI  string `json:"uri"`
	Name string `json:"name"`
}

type ServersEditResult struct {
	View         hub.ServerView `json:"view"`
	VlessChanged bool           `json:"vlessChanged"`
}

type ServersRemoveParams struct {
	ID string `json:"id"`
}

type ServersToggleFavoriteParams struct {
	ID string `json:"id"`
}

type ServersTestLatencyParams struct {
	ID string `json:"id"`
}

// SubsService — methods under "subs." namespace.
type SubsService interface {
	List(p Empty) ([]hub.SubView, error)
	Add(p SubsAddParams) (hub.SubView, error)
	Edit(p SubsEditParams) (hub.SubView, error)
	Remove(p SubsRemoveParams) (Empty, error)
	SyncOne(p SubsSyncOneParams) (hub.SubView, error)
	SyncAll(p Empty) (Empty, error)
}

type SubsAddParams struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

type SubsEditParams struct {
	ID   string `json:"id"`
	URL  string `json:"url"`
	Name string `json:"name"`
}

type SubsRemoveParams struct {
	ID string `json:"id"`
}

type SubsSyncOneParams struct {
	ID string `json:"id"`
}

// SettingsService — methods under "settings." namespace.
type SettingsService interface {
	Get(p Empty) (hub.SettingsView, error)
	Update(p SettingsUpdateParams) (hub.SettingsView, error)
}

// SettingsUpdateParams is a per-section partial update. Section is the
// settings group name (matches hub.SettingsView field json tags:
// "general", "network", "killSwitch", "subscriptions", "notifications",
// "debug", "about", "security", "dns"). Patch is a flat object of the
// fields to merge into that section. Mirrors bindings.SettingsService.Update.
type SettingsUpdateParams struct {
	Section string         `json:"section"`
	Patch   map[string]any `json:"patch"`
}

// HelperService — methods under "helper." namespace (Win-only logic;
// other platforms return E_HELPER_DOWN errors).
type HelperService interface {
	Status(p Empty) (HelperStatusResult, error)
	Install(p Empty) (Empty, error)
	Start(p Empty) (Empty, error)
	Stop(p Empty) (Empty, error)
	Restart(p Empty) (Empty, error)
	Reinstall(p Empty) (Empty, error)
	InstallLinux(p Empty) (Empty, error)
	UninstallLinux(p Empty) (Empty, error)
}

type HelperStatusResult struct {
	State string `json:"state"`
}

// OnboardingService — methods under "onboarding." namespace.
type OnboardingService interface {
	GetState(p Empty) (OnboardingStateResult, error)
	Complete(p Empty) (Empty, error)
	Skip(p Empty) (Empty, error)
}

type OnboardingStateResult struct {
	Onboarded bool `json:"onboarded"`
}

// RulesService — methods under "rules." namespace. Wire names flatten
// to a single dot (e.g. rules.groupAdd, not rules.group.add) to match
// the codegen contract used by every other service in this file.
type RulesService interface {
	List(p Empty) (hub.RulesView, error)
	ReplaceAll(p RulesReplaceAllParams) (Empty, error)
	GroupAdd(p RulesGroupAddParams) (RulesGroupAddResult, error)
	GroupEdit(p RulesGroupEditParams) (Empty, error)
	GroupRemove(p RulesGroupRemoveParams) (Empty, error)
	RuleAdd(p RulesRuleAddParams) (RulesRuleAddResult, error)
	RuleEdit(p RulesRuleEditParams) (Empty, error)
	RuleRemove(p RulesRuleRemoveParams) (Empty, error)
	RuleToggle(p RulesRuleToggleParams) (Empty, error)
	RuleMove(p RulesRuleMoveParams) (Empty, error)
}

type RulesReplaceAllParams struct {
	Model rules.Model `json:"model"`
}

type RulesGroupAddParams struct {
	Name string `json:"name"`
}

type RulesGroupAddResult struct {
	ID string `json:"id"`
}

type RulesGroupEditParams struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type RulesGroupRemoveParams struct {
	ID string `json:"id"`
}

type RulesRuleAddParams struct {
	GroupID string     `json:"groupId"`
	Rule    rules.Rule `json:"rule"`
}

type RulesRuleAddResult struct {
	ID string `json:"id"`
}

type RulesRuleEditParams struct {
	Rule rules.Rule `json:"rule"`
}

type RulesRuleRemoveParams struct {
	ID string `json:"id"`
}

type RulesRuleToggleParams struct {
	ID string `json:"id"`
}

type RulesRuleMoveParams struct {
	ID        string `json:"id"`
	ToGroupID string `json:"toGroupId"`
}

// EventTopics enumerates the bridge → main notification topics. The
// codegen tool emits these as a TS string-union type.
type EventTopic string

const (
	TopicVPNStatus      EventTopic = "vpn.status"
	TopicVPNSpeed       EventTopic = "vpn.speed"
	TopicChainError     EventTopic = "chain.error"
	TopicHelperState    EventTopic = "helper.state"
	TopicSubSynced      EventTopic = "sub.synced"
	TopicProbeResult    EventTopic = "probe.result"
	TopicServersChanged EventTopic = "servers.changed"
	TopicRulesChanged   EventTopic = "rules.changed"
	TopicBridgeState    EventTopic = "bridge.state"
)

package handlers

import (
	"context"
	"encoding/json"

	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/rules"
)

// Rules is the surface RulesHandlers needs from bindings.RulesService.
// Declared locally so handlers depend on a narrow interface that the
// real *bindings.RulesService satisfies — same pattern as Servers /
// Subs — and so tests can swap in a fake without dragging in store
// or hub plumbing.
type Rules interface {
	List() (hub.RulesView, error)
	ReplaceAll(model rules.Model) error
	GroupAdd(name string) (string, error)
	GroupEdit(id, name string, enabled bool) error
	GroupRemove(id string) error
	RuleAdd(groupID string, rule rules.Rule) (string, error)
	RuleEdit(rule rules.Rule) error
	RuleRemove(id string) error
	RuleToggle(id string) error
	RuleMove(id, toGroupID string) error
}

// RulesHandlers groups methods under the "rules." namespace. Method
// signatures return (any, error) so they slot directly into the
// dispatcher's Handler type alias; the dispatcher marshals the result
// to JSON before writing the response.
type RulesHandlers struct {
	Svc Rules
}

// Local mirrors of protocol.Rules*Params structs to avoid an import
// cycle into the protocol/codegen package. Kept in sync by the codegen
// drift gate (scripts/check-codegen.sh).

type rulesReplaceAllParams struct {
	Model rules.Model `json:"model"`
}

type rulesGroupAddParams struct {
	Name string `json:"name"`
}

type rulesGroupEditParams struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type rulesGroupRemoveParams struct {
	ID string `json:"id"`
}

type rulesRuleAddParams struct {
	GroupID string     `json:"groupId"`
	Rule    rules.Rule `json:"rule"`
}

type rulesRuleEditParams struct {
	Rule rules.Rule `json:"rule"`
}

type rulesRuleRemoveParams struct {
	ID string `json:"id"`
}

type rulesRuleToggleParams struct {
	ID string `json:"id"`
}

type rulesRuleMoveParams struct {
	ID        string `json:"id"`
	ToGroupID string `json:"toGroupId"`
}

// emptyResult is a stable success-with-no-payload shape — a JSON
// object rather than null so renderer callers can json.Unmarshal
// without special-casing empty bodies.
type emptyResult struct{}

// List returns the persisted rules model projected onto the wire shape.
func (h RulesHandlers) List(_ context.Context, _ json.RawMessage) (any, error) {
	return h.Svc.List()
}

// ReplaceAll atomically swaps the on-disk rules model. Used by the
// drag-reorder flow that touches many positions at once.
func (h RulesHandlers) ReplaceAll(_ context.Context, params json.RawMessage) (any, error) {
	var p rulesReplaceAllParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if err := h.Svc.ReplaceAll(p.Model); err != nil {
		return nil, err
	}
	return emptyResult{}, nil
}

// GroupAdd creates a new enabled group and returns its generated id.
func (h RulesHandlers) GroupAdd(_ context.Context, params json.RawMessage) (any, error) {
	var p rulesGroupAddParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	id, err := h.Svc.GroupAdd(p.Name)
	if err != nil {
		return nil, err
	}
	return map[string]string{"id": id}, nil
}

// GroupEdit renames a group and updates its enabled flag. The binding
// layer rejects mutations targeting the locked safety group.
func (h RulesHandlers) GroupEdit(_ context.Context, params json.RawMessage) (any, error) {
	var p rulesGroupEditParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if err := h.Svc.GroupEdit(p.ID, p.Name, p.Enabled); err != nil {
		return nil, err
	}
	return emptyResult{}, nil
}

// GroupRemove deletes a non-locked group by id.
func (h RulesHandlers) GroupRemove(_ context.Context, params json.RawMessage) (any, error) {
	var p rulesGroupRemoveParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if err := h.Svc.GroupRemove(p.ID); err != nil {
		return nil, err
	}
	return emptyResult{}, nil
}

// RuleAdd appends a new rule to a non-locked group, returning the
// generated rule id.
func (h RulesHandlers) RuleAdd(_ context.Context, params json.RawMessage) (any, error) {
	var p rulesRuleAddParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	id, err := h.Svc.RuleAdd(p.GroupID, p.Rule)
	if err != nil {
		return nil, err
	}
	return map[string]string{"id": id}, nil
}

// RuleEdit replaces an existing rule (matched by ID). The binding
// layer validates rule shape and rejects edits inside locked groups.
func (h RulesHandlers) RuleEdit(_ context.Context, params json.RawMessage) (any, error) {
	var p rulesRuleEditParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if err := h.Svc.RuleEdit(p.Rule); err != nil {
		return nil, err
	}
	return emptyResult{}, nil
}

// RuleRemove deletes a non-locked rule by id.
func (h RulesHandlers) RuleRemove(_ context.Context, params json.RawMessage) (any, error) {
	var p rulesRuleRemoveParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if err := h.Svc.RuleRemove(p.ID); err != nil {
		return nil, err
	}
	return emptyResult{}, nil
}

// RuleToggle flips the enabled flag on a non-locked rule.
func (h RulesHandlers) RuleToggle(_ context.Context, params json.RawMessage) (any, error) {
	var p rulesRuleToggleParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if err := h.Svc.RuleToggle(p.ID); err != nil {
		return nil, err
	}
	return emptyResult{}, nil
}

// RuleMove relocates a non-locked rule to another non-locked group,
// appending at the end of the destination.
func (h RulesHandlers) RuleMove(_ context.Context, params json.RawMessage) (any, error) {
	var p rulesRuleMoveParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if err := h.Svc.RuleMove(p.ID, p.ToGroupID); err != nil {
		return nil, err
	}
	return emptyResult{}, nil
}

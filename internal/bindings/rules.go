// Package bindings: RulesService exposes the on-disk rules.Model to
// the renderer through rules.* JSON-RPC methods. Every mutation
// serializes through s.mu and publishes hub.EventRulesChanged; the
// renderer re-fetches via List rather than diffing payloads.
package bindings

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/rules"
)

// RulesService implements the rules.* bindings. List is shipped in
// this task; Toggle / Create / Update / Delete / Reorder / SetDefault
// land in T6 / T7 and will use the s.mu mutex and s.hub publisher
// declared below.
type RulesService struct {
	mu    sync.Mutex
	store *rules.Store
	hub   *hub.Hub
}

// RulesDeps groups dependencies passed in from main.go. Store owns
// rules.json; Hub is the in-process pub-sub used to publish
// EventRulesChanged on every mutation.
type RulesDeps struct {
	Store *rules.Store
	Hub   *hub.Hub
}

// NewRulesService constructs a new RulesService. Panics when required
// deps are missing — every binding constructor in this package follows
// the same convention so a wiring mistake surfaces at startup rather
// than at first call from the renderer.
func NewRulesService(d RulesDeps) *RulesService {
	if d.Store == nil || d.Hub == nil {
		panic("bindings.NewRulesService: Store and Hub are required")
	}
	return &RulesService{store: d.Store, hub: d.Hub}
}

// List returns the persisted rules model projected onto the wire shape.
// rules.Store.Load returns the canonical default when rules.json is
// missing or corrupt, so callers can rely on a populated view even on
// first run.
func (s *RulesService) List() (hub.RulesView, error) {
	// s.mu serializes List against in-flight mutations from T6/T7 so the
	// renderer always sees a complete post-mutation snapshot rather than
	// a Load racing a Save mid-flight. Store has its own mu for torn-read
	// safety; this one is about caller-visible consistency.
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.store.Load()
	if err != nil {
		return hub.RulesView{}, err
	}
	return toRulesView(m), nil
}

// toRulesView projects a rules.Model onto the hub.RulesView wire shape.
// The Action enum is widened to a plain string so the TS codegen emits
// "proxy" / "direct" / "block" literals without a type alias hop.
func toRulesView(m rules.Model) hub.RulesView {
	groups := make([]hub.GroupView, 0, len(m.Groups))
	for _, g := range m.Groups {
		rs := make([]hub.RuleView, 0, len(g.Rules))
		for _, r := range g.Rules {
			rs = append(rs, hub.RuleView{
				ID:         r.ID,
				Name:       r.Name,
				Enabled:    r.Enabled,
				Action:     string(r.Action),
				Conditions: r.Conditions,
			})
		}
		groups = append(groups, hub.GroupView{
			ID: g.ID, Name: g.Name, Locked: g.Locked, Enabled: g.Enabled,
			Rules: rs,
		})
	}
	return hub.RulesView{DefaultAction: string(m.DefaultAction), Groups: groups}
}

// newID returns a fresh ID of shape <prefix><unix-millis>-<hex4>.
// Used by T6/T7 mutations (Create rule, Create group) to mint
// collision-resistant IDs without dragging in a UUID dependency.
// Declared here so the helper lives next to its consumers.
func newID(prefix string) string {
	var b [2]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%s%d-%s", prefix, time.Now().UnixMilli(), hex.EncodeToString(b[:]))
}

// errLockedGroup is the sentinel returned by mutations that target the
// safety group. Declared here so the upcoming T6/T7 handlers reach for
// the same error string and renderer code can match on it.
var errLockedGroup = errors.New("safety group is locked")

// GroupAdd appends a new enabled group with a freshly minted id and
// publishes EventRulesChanged. The id shape is g<unix-ms>-<hex4>; the
// renderer treats it as an opaque token.
func (s *RulesService) GroupAdd(name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", errors.New("group name cannot be empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.store.Load()
	if err != nil {
		return "", err
	}
	id := newID("g")
	m.Groups = append(m.Groups, rules.Group{
		ID: id, Name: name, Enabled: true,
	})
	if err := s.store.Save(m); err != nil {
		return "", err
	}
	s.publishChanged()
	return id, nil
}

// GroupEdit renames a group and updates its enabled flag. The hardcoded
// "safety" id and any group with Locked=true are rejected before Load
// so we never half-mutate the model.
func (s *RulesService) GroupEdit(id, name string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "safety" {
		return errLockedGroup
	}
	m, err := s.store.Load()
	if err != nil {
		return err
	}
	idx := indexOfGroup(m, id)
	if idx < 0 {
		return fmt.Errorf("group not found: %s", id)
	}
	if m.Groups[idx].Locked {
		return errLockedGroup
	}
	m.Groups[idx].Name = name
	m.Groups[idx].Enabled = enabled
	if err := s.store.Save(m); err != nil {
		return err
	}
	s.publishChanged()
	return nil
}

// GroupRemove deletes a non-locked group by id. Same guard as
// GroupEdit: literal "safety" plus any Locked group are rejected.
func (s *RulesService) GroupRemove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "safety" {
		return errLockedGroup
	}
	m, err := s.store.Load()
	if err != nil {
		return err
	}
	idx := indexOfGroup(m, id)
	if idx < 0 {
		return fmt.Errorf("group not found: %s", id)
	}
	if m.Groups[idx].Locked {
		return errLockedGroup
	}
	m.Groups = append(m.Groups[:idx], m.Groups[idx+1:]...)
	if err := s.store.Save(m); err != nil {
		return err
	}
	s.publishChanged()
	return nil
}

// indexOfGroup returns the slice index of the group with the given id
// or -1 if no match. Linear scan is fine — groups are user-visible and
// expected to stay in the tens.
func indexOfGroup(m rules.Model, id string) int {
	for i := range m.Groups {
		if m.Groups[i].ID == id {
			return i
		}
	}
	return -1
}

// publishChanged fires hub.EventRulesChanged so subscribed renderers
// can refetch via List. Centralized so every mutation path uses the
// same event name.
func (s *RulesService) publishChanged() {
	s.hub.Publish(hub.Event{Name: hub.EventRulesChanged})
}

// RuleAdd appends a new rule to a non-locked group. Returns the
// generated rule id. Validates rule shape before touching the store.
func (s *RulesService) RuleAdd(groupID string, r rules.Rule) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if groupID == "safety" {
		return "", errLockedGroup
	}
	r.ID = newID("r")
	if err := r.Validate(); err != nil {
		return "", err
	}
	m, err := s.store.Load()
	if err != nil {
		return "", err
	}
	idx := indexOfGroup(m, groupID)
	if idx < 0 {
		return "", fmt.Errorf("group not found: %s", groupID)
	}
	if m.Groups[idx].Locked {
		return "", errLockedGroup
	}
	m.Groups[idx].Rules = append(m.Groups[idx].Rules, r)
	if err := s.store.Save(m); err != nil {
		return "", err
	}
	s.publishChanged()
	return r.ID, nil
}

// RuleEdit replaces an existing rule (matched by ID) across all
// non-locked groups. Returns an error if the rule lives in a locked
// group or the new shape fails Validate.
func (s *RulesService) RuleEdit(r rules.Rule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := r.Validate(); err != nil {
		return err
	}
	m, err := s.store.Load()
	if err != nil {
		return err
	}
	gi, ri := findRule(m, r.ID)
	if gi < 0 {
		return fmt.Errorf("rule not found: %s", r.ID)
	}
	if m.Groups[gi].Locked {
		return errLockedGroup
	}
	m.Groups[gi].Rules[ri] = r
	if err := s.store.Save(m); err != nil {
		return err
	}
	s.publishChanged()
	return nil
}

// RuleRemove deletes a non-locked rule by id.
func (s *RulesService) RuleRemove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.store.Load()
	if err != nil {
		return err
	}
	gi, ri := findRule(m, id)
	if gi < 0 {
		return fmt.Errorf("rule not found: %s", id)
	}
	if m.Groups[gi].Locked {
		return errLockedGroup
	}
	m.Groups[gi].Rules = append(m.Groups[gi].Rules[:ri], m.Groups[gi].Rules[ri+1:]...)
	if err := s.store.Save(m); err != nil {
		return err
	}
	s.publishChanged()
	return nil
}

// RuleToggle flips a non-locked rule's enabled flag.
func (s *RulesService) RuleToggle(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.store.Load()
	if err != nil {
		return err
	}
	gi, ri := findRule(m, id)
	if gi < 0 {
		return fmt.Errorf("rule not found: %s", id)
	}
	if m.Groups[gi].Locked {
		return errLockedGroup
	}
	m.Groups[gi].Rules[ri].Enabled = !m.Groups[gi].Rules[ri].Enabled
	if err := s.store.Save(m); err != nil {
		return err
	}
	s.publishChanged()
	return nil
}

// RuleMove relocates a non-locked rule to another non-locked group,
// appending at the end of the destination.
func (s *RulesService) RuleMove(id, toGroupID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if toGroupID == "safety" {
		return errLockedGroup
	}
	m, err := s.store.Load()
	if err != nil {
		return err
	}
	gi, ri := findRule(m, id)
	if gi < 0 {
		return fmt.Errorf("rule not found: %s", id)
	}
	if m.Groups[gi].Locked {
		return errLockedGroup
	}
	to := indexOfGroup(m, toGroupID)
	if to < 0 {
		return fmt.Errorf("group not found: %s", toGroupID)
	}
	if m.Groups[to].Locked {
		return errLockedGroup
	}
	r := m.Groups[gi].Rules[ri]
	m.Groups[gi].Rules = append(m.Groups[gi].Rules[:ri], m.Groups[gi].Rules[ri+1:]...)
	m.Groups[to].Rules = append(m.Groups[to].Rules, r)
	if err := s.store.Save(m); err != nil {
		return err
	}
	s.publishChanged()
	return nil
}

func findRule(m rules.Model, id string) (groupIdx, ruleIdx int) {
	for gi := range m.Groups {
		for ri := range m.Groups[gi].Rules {
			if m.Groups[gi].Rules[ri].ID == id {
				return gi, ri
			}
		}
	}
	return -1, -1
}

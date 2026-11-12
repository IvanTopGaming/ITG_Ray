package bindings

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/itg-team/itg-ray/internal/ruleshare"
)

func providerGroupID(subID string) string { return "src-" + subID }

func providerRuleID(subID string, i int) string { return fmt.Sprintf("src-%s-%d", subID, i) }

func buildProviderGroup(subID, subName, link string) (rules.Group, error) {
	p, err := ruleshare.Decode(link)
	if err != nil {
		return rules.Group{}, err
	}
	name := strings.TrimSpace(p.Name)
	if name == "" {
		name = strings.TrimSpace(subName)
	}
	if name == "" {
		name = subID
	}
	g := rules.Group{ID: providerGroupID(subID), Name: name, Locked: true, Enabled: true, SourceID: subID}
	for _, pg := range p.Groups {
		for _, r := range pg.Rules {
			r.ID = providerRuleID(subID, len(g.Rules))
			r.ServerID = ""
			r.ServerSourceID = ""
			if r.Action == rules.ActionProxy && r.ServerName != "" {
				r.ServerSourceID = subID
			}
			g.Rules = append(g.Rules, r)
		}
	}
	return g, nil
}

func sameGroup(a, b *rules.Group) bool {
	x, errA := json.Marshal(a)
	y, errB := json.Marshal(b)
	return errA == nil && errB == nil && string(x) == string(y)
}

func (s *RulesService) ApplyProviderRouting(subID, subName, link string) error {
	var next *rules.Group
	if link != "" {
		g, err := buildProviderGroup(subID, subName, link)
		if err != nil {
			return err
		}
		next = &g
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.store.Load()
	if err != nil {
		return err
	}
	idx := -1
	for i := range m.Groups {
		if m.Groups[i].SourceID == subID {
			idx = i
			break
		}
	}
	switch {
	case next == nil && idx < 0:
		return nil
	case next == nil:
		m.Groups = append(m.Groups[:idx], m.Groups[idx+1:]...)
	case idx < 0:
		m.Groups = append(m.Groups, *next)
	default:
		prev := &m.Groups[idx]
		next.ID = prev.ID
		next.Enabled = prev.Enabled
		for ri := range next.Rules {
			if ri < len(prev.Rules) {
				old := prev.Rules[ri]
				if old.ID == next.Rules[ri].ID && old.ServerName == next.Rules[ri].ServerName && old.ServerSourceID == next.Rules[ri].ServerSourceID {
					next.Rules[ri].ServerID = old.ServerID
				}
			}
		}
		if sameGroup(prev, next) {
			return nil
		}
		m.Groups[idx] = *next
	}
	if err := s.store.Save(m); err != nil {
		return err
	}
	s.publishChanged()
	return nil
}

func keepStoredSourceGroups(cur, next rules.Model) rules.Model {
	stored := map[string]bool{}
	for _, g := range cur.Groups {
		if g.FromSource() {
			stored[g.ID] = true
		}
	}
	out := rules.Model{DefaultAction: next.DefaultAction}
	for _, g := range next.Groups {
		if g.FromSource() || stored[g.ID] {
			continue
		}
		out.Groups = append(out.Groups, g)
	}
	for _, g := range cur.Groups {
		if g.FromSource() {
			out.Groups = append(out.Groups, g)
		}
	}
	return out
}

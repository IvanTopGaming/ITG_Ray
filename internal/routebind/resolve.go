package routebind

import (
	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/itg-team/itg-ray/internal/server"
)

type State string

const (
	StateOK       State = "ok"
	StateRebound  State = "rebound"
	StateMissing  State = "missing"
	StateUnusable State = "unusable"
)

type Binding struct {
	RuleID     string
	ServerID   string
	ServerName string
	State      State
	Reason     string
}

func Resolve(m rules.Model, servers []server.Server) (rules.Model, map[string]Binding) {
	byID := make(map[string]*server.Server, len(servers))
	for i := range servers {
		byID[servers[i].ID] = &servers[i]
	}
	out := rules.Model{DefaultAction: m.DefaultAction, Groups: make([]rules.Group, len(m.Groups))}
	bindings := make(map[string]Binding)
	for gi, g := range m.Groups {
		g.Rules = append([]rules.Rule(nil), g.Rules...)
		for ri := range g.Rules {
			r := &g.Rules[ri]
			if r.Action != rules.ActionProxy || (r.ServerID == "" && r.ServerName == "") {
				continue
			}
			b := Binding{RuleID: r.ID, ServerID: r.ServerID, ServerName: r.ServerName}
			b.State = StateMissing
			var s *server.Server
			if r.ServerID != "" {
				if found, ok := byID[r.ServerID]; ok {
					s, b.State = found, StateOK
				}
			}
			if s == nil {
				if r.ServerID == "" && r.ServerSourceID == "" {
					s = uniqueByName(servers, anySource, r.ServerName)
				} else {
					s = uniqueByName(servers, r.ServerSourceID, r.ServerName)
				}
				if s != nil {
					b.State = StateRebound
				}
			}
			if s != nil {
				r.ServerID = s.ID
				r.ServerName = s.Name
				r.ServerSourceID = s.SourceID
				b.ServerID = s.ID
				b.ServerName = s.Name
				if s.Disabled {
					b.State = StateUnusable
					b.Reason = "disabled"
				}
			}
			bindings[r.ID] = b
		}
		out.Groups[gi] = g
	}
	return out, bindings
}

const anySource = "\x00any"

func uniqueByName(servers []server.Server, sourceID, name string) *server.Server {
	if sourceID == "" || name == "" {
		return nil
	}
	var found *server.Server
	for i := range servers {
		s := &servers[i]
		if (sourceID != anySource && s.SourceID != sourceID) || s.Name != name {
			continue
		}
		if found != nil {
			return nil
		}
		found = s
	}
	return found
}

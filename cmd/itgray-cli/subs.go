package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/subscription"
	"github.com/spf13/cobra"
)

func subsPath() string    { return filepath.Join(dataDir, "subscriptions.json") }
func serversPath() string { return filepath.Join(dataDir, "servers.json") }

func subsStore() *subscription.FileStore {
	return &subscription.FileStore{Path: subsPath()}
}

func newSubCmd() *cobra.Command {
	sub := &cobra.Command{Use: "sub", Short: "manage subscriptions"}

	addCmd := &cobra.Command{
		Use:   "add [url]",
		Short: "add a subscription URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			ua, _ := cmd.Flags().GetString("ua")
			st := subsStore()
			subs, err := st.Load()
			if err != nil {
				return err
			}
			id := fmt.Sprintf("s%d", time.Now().Unix())
			subs = append(subs, subscription.Stored{
				ID: id, Name: name, URL: args[0], UserAgent: ua,
			})
			if err := st.Save(subs); err != nil {
				return err
			}
			fmt.Println(id)
			return nil
		},
	}
	addCmd.Flags().String("name", "", "display name")
	addCmd.Flags().String("ua", "", "User-Agent override")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "list subscriptions (id, name, last sync, status, url)",
		RunE: func(*cobra.Command, []string) error {
			subs, err := subsStore().Load()
			if err != nil {
				return err
			}
			now := time.Now()
			for _, s := range subs {
				lastSync := "-"
				if !s.LastSyncAt.IsZero() {
					lastSync = humanRelative(now.Sub(s.LastSyncAt))
				}
				status := s.LastStatus
				if status == "" {
					status = "-"
				}
				fmt.Printf("%s\t%s\t%s\t%s\t%s\n", s.ID, s.Name, lastSync, status, s.URL)
			}
			return nil
		},
	}

	removeCmd := &cobra.Command{
		Use:   "remove [id]",
		Short: "remove a subscription by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			st := subsStore()
			subs, err := st.Load()
			if err != nil {
				return err
			}
			out := subs[:0]
			for _, s := range subs {
				if s.ID != args[0] {
					out = append(out, s)
				}
			}
			return st.Save(out)
		},
	}

	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "fetch and merge all subscriptions now",
		RunE: func(*cobra.Command, []string) error {
			st := subsStore()
			subs, err := st.Load()
			if err != nil {
				return err
			}
			existing, err := server.Load(serversPath())
			if err != nil {
				if !os.IsNotExist(err) {
					return err
				}
				existing = nil
			}
			ctx := context.Background()
			for _, s := range subs {
				merged, meta, err := subscription.Sync(ctx, s.ToSyncInput(), existing, 30*time.Second)
				if err != nil {
					fmt.Printf("%s\tERROR: %s\n", s.ID, err.Error())
					_ = st.UpdateMeta(s.ID, time.Now(), "ERROR: "+truncate(err.Error(), 120))
					continue
				}
				existing = merged
				fmt.Printf("%s\t%s\t%s\n", s.ID, meta.Status, meta.Summary)
				_ = st.UpdateMeta(s.ID, time.Now(), "OK "+meta.Summary)
			}
			return server.Save(serversPath(), existing)
		},
	}

	sub.AddCommand(addCmd, listCmd, removeCmd, syncCmd)
	return sub
}

// humanRelative renders a duration like "12s ago", "5m ago", "3h ago", "2d ago".
// Negative durations are treated as 0.
func humanRelative(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// truncate clips s to at most n runes, appending "…" if cut.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

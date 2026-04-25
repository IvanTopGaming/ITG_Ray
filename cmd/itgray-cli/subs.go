package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/subscription"
	"github.com/spf13/cobra"
)

type subsFile struct {
	Subs []storedSub `json:"subs"`
}

type storedSub struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	UserAgent string `json:"user_agent,omitempty"`
}

func subsPath() string    { return filepath.Join(dataDir, "subscriptions.json") }
func serversPath() string { return filepath.Join(dataDir, "servers.json") }

func loadSubs() subsFile {
	b, err := os.ReadFile(subsPath()) //nolint:gosec // path is application-controlled
	if err != nil {
		return subsFile{}
	}
	var f subsFile
	_ = json.Unmarshal(b, &f)
	return f
}

func saveSubs(f subsFile) error {
	b, _ := json.MarshalIndent(f, "", "  ")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(subsPath(), b, 0o600)
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
			f := loadSubs()
			id := fmt.Sprintf("s%d", time.Now().Unix())
			f.Subs = append(f.Subs, storedSub{ID: id, Name: name, URL: args[0], UserAgent: ua})
			if err := saveSubs(f); err != nil {
				return err
			}
			fmt.Println("added:", id)
			return nil
		},
	}
	addCmd.Flags().String("name", "", "display name")
	addCmd.Flags().String("ua", "", "User-Agent")
	sub.AddCommand(addCmd)

	sub.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "list subscriptions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			for _, s := range loadSubs().Subs {
				fmt.Printf("%s\t%s\t%s\n", s.ID, s.Name, s.URL)
			}
			return nil
		},
	})

	sub.AddCommand(&cobra.Command{
		Use:   "sync [id]",
		Short: "fetch and merge a subscription's servers",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			f := loadSubs()
			for _, s := range f.Subs {
				if len(args) == 1 && s.ID != args[0] {
					continue
				}
				existing, _ := server.Load(serversPath())
				merged, meta, err := subscription.Sync(context.Background(),
					subscription.Subscription{ID: s.ID, URL: s.URL, UserAgent: s.UserAgent},
					existing, 30*time.Second)
				if err != nil {
					fmt.Printf("%s\tERROR\t%v\n", s.ID, err)
					continue
				}
				if err := server.Save(serversPath(), merged); err != nil {
					return err
				}
				fmt.Printf("%s\t%s\t%s\n", s.ID, meta.Status, meta.Summary)
			}
			return nil
		},
	})

	return sub
}

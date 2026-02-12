package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jaredpalmer/mogcli/internal/outfmt"
	"github.com/jaredpalmer/mogcli/internal/services/mail"
)

type MailCmd struct {
	List MailListCmd `cmd:"" help:"List messages"`
	Get  MailGetCmd  `cmd:"" help:"Get message by ID"`
	Send MailSendCmd `cmd:"" help:"Send a new message"`
}

type MailListCmd struct {
	Max   int    `name:"max" default:"20" help:"Maximum messages"`
	Query string `name:"query" help:"Search query text"`
	Page  string `name:"page" aliases:"next-token" help:"Resume from next page token"`
	User  string `name:"user" help:"App-only target user override (UPN or user ID)"`
}

func (c *MailListCmd) Run(ctx context.Context) error {
	rt, err := resolveRuntime(ctx, capMailList)
	if err != nil {
		return err
	}
	targetUser, err := resolveAppOnlyTargetUser(rt.Profile, c.User)
	if err != nil {
		return err
	}
	page, err := normalizePageToken(c.Page)
	if err != nil {
		return err
	}

	svc := mail.New(rt.Graph, targetUser)
	items, next, err := svc.List(ctx, c.Max, c.Query, page)
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"messages": items, "next": next})
	}

	printItemTable(ctx, items, []string{"receivedDateTime", "subject", "id", "isRead"})
	printNextPageHint(uiFromContext(ctx), next)
	return nil
}

type MailGetCmd struct {
	ID   string `arg:"" required:"" help:"Message ID"`
	User string `name:"user" help:"App-only target user override (UPN or user ID)"`
}

func (c *MailGetCmd) Run(ctx context.Context) error {
	rt, err := resolveRuntime(ctx, capMailGet)
	if err != nil {
		return err
	}
	targetUser, err := resolveAppOnlyTargetUser(rt.Profile, c.User)
	if err != nil {
		return err
	}

	svc := mail.New(rt.Graph, targetUser)
	item, err := svc.Get(ctx, c.ID)
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, item)
	}

	printSingleMap(ctx, item)
	return nil
}

type MailSendCmd struct {
	To      []string `name:"to" required:"" help:"Recipient email (repeat or comma-separate)"`
	Subject string   `name:"subject" required:"" help:"Email subject"`
	Body    string   `name:"body" required:"" help:"Plain text body"`
	User    string   `name:"user" help:"App-only target user override (UPN or user ID)"`
}

func (c *MailSendCmd) Run(ctx context.Context) error {
	rt, err := resolveRuntime(ctx, capMailSend)
	if err != nil {
		return err
	}
	targetUser, err := resolveAppOnlyTargetUser(rt.Profile, c.User)
	if err != nil {
		return err
	}

	recipients := splitCSV(c.To)
	if len(recipients) == 0 {
		return usage("at least one --to recipient is required")
	}

	svc := mail.New(rt.Graph, targetUser)
	if err := svc.Send(ctx, recipients, c.Subject, c.Body); err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"sent": true, "to": recipients})
	}

	fmt.Fprintf(os.Stdout, "Sent message to %s\n", strings.Join(recipients, ", "))
	return nil
}

func splitCSV(values []string) []string {
	out := make([]string, 0)
	seen := map[string]struct{}{}
	for _, raw := range values {
		for _, part := range strings.Split(raw, ",") {
			item := strings.TrimSpace(part)
			if item == "" {
				continue
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			out = append(out, item)
		}
	}

	return out
}

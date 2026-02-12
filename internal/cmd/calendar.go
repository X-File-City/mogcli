package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jared/mogcli/internal/outfmt"
	"github.com/jared/mogcli/internal/services/calendar"
)

type CalendarCmd struct {
	List   CalendarListCmd   `cmd:"" help:"List events"`
	Get    CalendarGetCmd    `cmd:"" help:"Get event"`
	Create CalendarCreateCmd `cmd:"" help:"Create event"`
	Update CalendarUpdateCmd `cmd:"" help:"Update event"`
	Delete CalendarDeleteCmd `cmd:"" help:"Delete event"`
}

type CalendarListCmd struct {
	From string `name:"from" help:"Start time (RFC3339 or date)"`
	To   string `name:"to" help:"End time (RFC3339 or date)"`
	Max  int    `name:"max" default:"50" help:"Maximum events"`
}

func (c *CalendarListCmd) Run(ctx context.Context) error {
	from, to := normalizeRange(c.From, c.To)
	rt, err := resolveRuntime(ctx, false)
	if err != nil {
		return err
	}

	svc := calendar.New(rt.Graph)
	items, next, err := svc.List(ctx, from, to, c.Max)
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"events": items, "next": next})
	}

	printItemTable(ctx, items, []string{"start", "subject", "id", "organizer"})
	printNextPageHint(uiFromContext(ctx), next)
	return nil
}

type CalendarGetCmd struct {
	ID string `arg:"" required:"" help:"Event ID"`
}

func (c *CalendarGetCmd) Run(ctx context.Context) error {
	rt, err := resolveRuntime(ctx, false)
	if err != nil {
		return err
	}
	svc := calendar.New(rt.Graph)
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

type CalendarCreateCmd struct {
	Subject string `name:"subject" required:"" help:"Event subject"`
	Start   string `name:"start" required:"" help:"Start datetime (RFC3339)"`
	End     string `name:"end" required:"" help:"End datetime (RFC3339)"`
	Body    string `name:"body" help:"Optional body text"`
}

func (c *CalendarCreateCmd) Run(ctx context.Context) error {
	if _, err := time.Parse(time.RFC3339, c.Start); err != nil {
		return usage("--start must be RFC3339")
	}
	if _, err := time.Parse(time.RFC3339, c.End); err != nil {
		return usage("--end must be RFC3339")
	}

	rt, err := resolveRuntime(ctx, false)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"subject": c.Subject,
		"start":   map[string]any{"dateTime": c.Start, "timeZone": "UTC"},
		"end":     map[string]any{"dateTime": c.End, "timeZone": "UTC"},
	}
	if strings.TrimSpace(c.Body) != "" {
		payload["body"] = map[string]any{"contentType": "Text", "content": c.Body}
	}

	svc := calendar.New(rt.Graph)
	item, err := svc.Create(ctx, payload)
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, item)
	}
	fmt.Fprintf(os.Stdout, "Created event %s\n", flattenValue(item["id"]))
	return nil
}

type CalendarUpdateCmd struct {
	ID      string `arg:"" required:"" help:"Event ID"`
	Subject string `name:"subject" help:"Event subject"`
	Start   string `name:"start" help:"Start datetime (RFC3339)"`
	End     string `name:"end" help:"End datetime (RFC3339)"`
	Body    string `name:"body" help:"Body text"`
}

func (c *CalendarUpdateCmd) Run(ctx context.Context) error {
	payload := map[string]any{}
	if strings.TrimSpace(c.Subject) != "" {
		payload["subject"] = c.Subject
	}
	if strings.TrimSpace(c.Start) != "" {
		if _, err := time.Parse(time.RFC3339, c.Start); err != nil {
			return usage("--start must be RFC3339")
		}
		payload["start"] = map[string]any{"dateTime": c.Start, "timeZone": "UTC"}
	}
	if strings.TrimSpace(c.End) != "" {
		if _, err := time.Parse(time.RFC3339, c.End); err != nil {
			return usage("--end must be RFC3339")
		}
		payload["end"] = map[string]any{"dateTime": c.End, "timeZone": "UTC"}
	}
	if strings.TrimSpace(c.Body) != "" {
		payload["body"] = map[string]any{"contentType": "Text", "content": c.Body}
	}
	if len(payload) == 0 {
		return usage("provide at least one field to update")
	}

	rt, err := resolveRuntime(ctx, false)
	if err != nil {
		return err
	}

	svc := calendar.New(rt.Graph)
	if err := svc.Update(ctx, c.ID, payload); err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"updated": c.ID})
	}
	fmt.Fprintf(os.Stdout, "Updated event %s\n", c.ID)
	return nil
}

type CalendarDeleteCmd struct {
	ID string `arg:"" required:"" help:"Event ID"`
}

func (c *CalendarDeleteCmd) Run(ctx context.Context) error {
	flags := rootFlagsFromContext(ctx)
	if flags == nil {
		flags = &RootFlags{}
	}
	if err := confirmDestructive(ctx, flags, fmt.Sprintf("delete event %s", c.ID)); err != nil {
		return err
	}

	rt, err := resolveRuntime(ctx, false)
	if err != nil {
		return err
	}
	if err := calendar.New(rt.Graph).Delete(ctx, c.ID); err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"deleted": c.ID})
	}
	fmt.Fprintf(os.Stdout, "Deleted event %s\n", c.ID)
	return nil
}

func normalizeRange(from string, to string) (string, string) {
	now := time.Now().UTC()
	if strings.TrimSpace(from) == "" {
		from = now.Format(time.RFC3339)
	}
	if strings.TrimSpace(to) == "" {
		to = now.Add(7 * 24 * time.Hour).Format(time.RFC3339)
	}

	if t, err := time.Parse("2006-01-02", from); err == nil {
		from = t.UTC().Format(time.RFC3339)
	}
	if t, err := time.Parse("2006-01-02", to); err == nil {
		to = t.UTC().Format(time.RFC3339)
	}

	return from, to
}

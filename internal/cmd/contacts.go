package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jared/mogcli/internal/outfmt"
	"github.com/jared/mogcli/internal/services/contacts"
)

type ContactsCmd struct {
	List   ContactsListCmd   `cmd:"" help:"List contacts"`
	Get    ContactsGetCmd    `cmd:"" help:"Get contact"`
	Create ContactsCreateCmd `cmd:"" help:"Create contact"`
	Update ContactsUpdateCmd `cmd:"" help:"Update contact"`
	Delete ContactsDeleteCmd `cmd:"" help:"Delete contact"`
}

type ContactsListCmd struct {
	Max int `name:"max" default:"100" help:"Maximum contacts"`
}

func (c *ContactsListCmd) Run(ctx context.Context) error {
	rt, err := resolveRuntime(ctx, capContactsList)
	if err != nil {
		return err
	}

	items, next, err := contacts.New(rt.Graph).List(ctx, c.Max)
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"contacts": items, "next": next})
	}

	printItemTable(ctx, items, []string{"displayName", "id", "emailAddresses", "mobilePhone"})
	printNextPageHint(uiFromContext(ctx), next)
	return nil
}

type ContactsGetCmd struct {
	ID string `arg:"" required:"" help:"Contact ID"`
}

func (c *ContactsGetCmd) Run(ctx context.Context) error {
	rt, err := resolveRuntime(ctx, capContactsGet)
	if err != nil {
		return err
	}

	item, err := contacts.New(rt.Graph).Get(ctx, c.ID)
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, item)
	}
	printSingleMap(ctx, item)
	return nil
}

type ContactsCreateCmd struct {
	DisplayName string `name:"display-name" required:"" help:"Contact display name"`
	Email       string `name:"email" help:"Primary email"`
	MobilePhone string `name:"mobile-phone" help:"Mobile phone"`
}

func (c *ContactsCreateCmd) Run(ctx context.Context) error {
	rt, err := resolveRuntime(ctx, capContactsCreate)
	if err != nil {
		return err
	}

	payload := map[string]any{"displayName": c.DisplayName}
	if strings.TrimSpace(c.Email) != "" {
		payload["emailAddresses"] = []map[string]any{{"address": c.Email, "name": c.DisplayName}}
	}
	if strings.TrimSpace(c.MobilePhone) != "" {
		payload["mobilePhone"] = c.MobilePhone
	}

	item, err := contacts.New(rt.Graph).Create(ctx, payload)
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, item)
	}
	fmt.Fprintf(os.Stdout, "Created contact %s\n", flattenValue(item["id"]))
	return nil
}

type ContactsUpdateCmd struct {
	ID          string `arg:"" required:"" help:"Contact ID"`
	DisplayName string `name:"display-name" help:"Contact display name"`
	Email       string `name:"email" help:"Primary email"`
	MobilePhone string `name:"mobile-phone" help:"Mobile phone"`
}

func (c *ContactsUpdateCmd) Run(ctx context.Context) error {
	payload := map[string]any{}
	if strings.TrimSpace(c.DisplayName) != "" {
		payload["displayName"] = c.DisplayName
	}
	if strings.TrimSpace(c.Email) != "" {
		name := c.DisplayName
		if strings.TrimSpace(name) == "" {
			name = c.Email
		}
		payload["emailAddresses"] = []map[string]any{{"address": c.Email, "name": name}}
	}
	if strings.TrimSpace(c.MobilePhone) != "" {
		payload["mobilePhone"] = c.MobilePhone
	}
	if len(payload) == 0 {
		return usage("provide at least one field to update")
	}

	rt, err := resolveRuntime(ctx, capContactsUpdate)
	if err != nil {
		return err
	}
	if err := contacts.New(rt.Graph).Update(ctx, c.ID, payload); err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"updated": c.ID})
	}
	fmt.Fprintf(os.Stdout, "Updated contact %s\n", c.ID)
	return nil
}

type ContactsDeleteCmd struct {
	ID string `arg:"" required:"" help:"Contact ID"`
}

func (c *ContactsDeleteCmd) Run(ctx context.Context) error {
	flags := rootFlagsFromContext(ctx)
	if flags == nil {
		flags = &RootFlags{}
	}
	if err := confirmDestructive(ctx, flags, fmt.Sprintf("delete contact %s", c.ID)); err != nil {
		return err
	}

	rt, err := resolveRuntime(ctx, capContactsDelete)
	if err != nil {
		return err
	}
	if err := contacts.New(rt.Graph).Delete(ctx, c.ID); err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"deleted": c.ID})
	}
	fmt.Fprintf(os.Stdout, "Deleted contact %s\n", c.ID)
	return nil
}

package cmd

import (
	"context"
	"os"

	"github.com/jared/mogcli/internal/outfmt"
	groupsvc "github.com/jared/mogcli/internal/services/groups"
)

type GroupsCmd struct {
	List    GroupsListCmd    `cmd:"" help:"List groups"`
	Get     GroupsGetCmd     `cmd:"" help:"Get group"`
	Members GroupsMembersCmd `cmd:"" help:"List group members"`
}

type GroupsListCmd struct {
	Max  int    `name:"max" default:"100" help:"Maximum groups"`
	Page string `name:"page" aliases:"next-token" help:"Resume from next page token"`
}

func (c *GroupsListCmd) Run(ctx context.Context) error {
	rt, err := resolveRuntime(ctx, capGroupsList)
	if err != nil {
		return err
	}
	page, err := normalizePageToken(c.Page)
	if err != nil {
		return err
	}

	items, next, err := groupsvc.New(rt.Graph).List(ctx, c.Max, page)
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"groups": items, "next": next})
	}
	printItemTable(ctx, items, []string{"displayName", "id", "mail", "mailEnabled", "securityEnabled"})
	printNextPageHint(uiFromContext(ctx), next)
	return nil
}

type GroupsGetCmd struct {
	ID string `arg:"" required:"" help:"Group ID"`
}

func (c *GroupsGetCmd) Run(ctx context.Context) error {
	rt, err := resolveRuntime(ctx, capGroupsGet)
	if err != nil {
		return err
	}

	item, err := groupsvc.New(rt.Graph).Get(ctx, c.ID)
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, item)
	}
	printSingleMap(ctx, item)
	return nil
}

type GroupsMembersCmd struct {
	ID   string `arg:"" required:"" help:"Group ID"`
	Max  int    `name:"max" default:"100" help:"Maximum members"`
	Page string `name:"page" aliases:"next-token" help:"Resume from next page token"`
}

func (c *GroupsMembersCmd) Run(ctx context.Context) error {
	rt, err := resolveRuntime(ctx, capGroupsMembers)
	if err != nil {
		return err
	}
	page, err := normalizePageToken(c.Page)
	if err != nil {
		return err
	}

	items, next, err := groupsvc.New(rt.Graph).Members(ctx, c.ID, c.Max, page)
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"members": items, "next": next})
	}
	printItemTable(ctx, items, []string{"displayName", "id", "userPrincipalName", "mail"})
	printNextPageHint(uiFromContext(ctx), next)
	return nil
}

package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jared/mogcli/internal/outfmt"
	tasksvc "github.com/jared/mogcli/internal/services/tasks"
)

type TasksCmd struct {
	Lists    TasksListsCmd    `cmd:"" help:"List task lists"`
	List     TasksListCmd     `cmd:"" help:"List tasks in a list"`
	Get      TasksGetCmd      `cmd:"" help:"Get task"`
	Create   TasksCreateCmd   `cmd:"" help:"Create task"`
	Update   TasksUpdateCmd   `cmd:"" help:"Update task"`
	Complete TasksCompleteCmd `cmd:"" help:"Mark task completed"`
	Delete   TasksDeleteCmd   `cmd:"" help:"Delete task"`
}

type TasksListsCmd struct{}

func (c *TasksListsCmd) Run(ctx context.Context) error {
	rt, err := resolveRuntime(ctx, false)
	if err != nil {
		return err
	}
	items, err := tasksvc.New(rt.Graph).Lists(ctx)
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"lists": items})
	}
	printItemTable(ctx, items, []string{"displayName", "id", "isShared", "wellknownListName"})
	return nil
}

type TasksListCmd struct {
	ListID string `name:"list" required:"" help:"Task list ID"`
	Max    int    `name:"max" default:"100" help:"Maximum tasks"`
}

func (c *TasksListCmd) Run(ctx context.Context) error {
	rt, err := resolveRuntime(ctx, false)
	if err != nil {
		return err
	}
	items, next, err := tasksvc.New(rt.Graph).ListTasks(ctx, c.ListID, c.Max)
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"tasks": items, "next": next})
	}
	printItemTable(ctx, items, []string{"title", "id", "status", "importance"})
	printNextPageHint(uiFromContext(ctx), next)
	return nil
}

type TasksGetCmd struct {
	ListID string `name:"list" required:"" help:"Task list ID"`
	TaskID string `name:"task" required:"" help:"Task ID"`
}

func (c *TasksGetCmd) Run(ctx context.Context) error {
	rt, err := resolveRuntime(ctx, false)
	if err != nil {
		return err
	}
	item, err := tasksvc.New(rt.Graph).GetTask(ctx, c.ListID, c.TaskID)
	if err != nil {
		return err
	}
	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, item)
	}
	printSingleMap(ctx, item)
	return nil
}

type TasksCreateCmd struct {
	ListID string `name:"list" required:"" help:"Task list ID"`
	Title  string `name:"title" required:"" help:"Task title"`
	Body   string `name:"body" help:"Task body text"`
}

func (c *TasksCreateCmd) Run(ctx context.Context) error {
	rt, err := resolveRuntime(ctx, false)
	if err != nil {
		return err
	}
	payload := map[string]any{"title": c.Title}
	if strings.TrimSpace(c.Body) != "" {
		payload["body"] = map[string]any{"content": c.Body, "contentType": "text"}
	}
	item, err := tasksvc.New(rt.Graph).CreateTask(ctx, c.ListID, payload)
	if err != nil {
		return err
	}
	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, item)
	}
	fmt.Fprintf(os.Stdout, "Created task %s\n", flattenValue(item["id"]))
	return nil
}

type TasksUpdateCmd struct {
	ListID string `name:"list" required:"" help:"Task list ID"`
	TaskID string `name:"task" required:"" help:"Task ID"`
	Title  string `name:"title" help:"Task title"`
	Body   string `name:"body" help:"Task body text"`
}

func (c *TasksUpdateCmd) Run(ctx context.Context) error {
	payload := map[string]any{}
	if strings.TrimSpace(c.Title) != "" {
		payload["title"] = c.Title
	}
	if strings.TrimSpace(c.Body) != "" {
		payload["body"] = map[string]any{"content": c.Body, "contentType": "text"}
	}
	if len(payload) == 0 {
		return usage("provide at least one field to update")
	}

	rt, err := resolveRuntime(ctx, false)
	if err != nil {
		return err
	}
	if err := tasksvc.New(rt.Graph).UpdateTask(ctx, c.ListID, c.TaskID, payload); err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"updated": c.TaskID})
	}
	fmt.Fprintf(os.Stdout, "Updated task %s\n", c.TaskID)
	return nil
}

type TasksCompleteCmd struct {
	ListID string `name:"list" required:"" help:"Task list ID"`
	TaskID string `name:"task" required:"" help:"Task ID"`
}

func (c *TasksCompleteCmd) Run(ctx context.Context) error {
	rt, err := resolveRuntime(ctx, false)
	if err != nil {
		return err
	}
	payload := map[string]any{"status": "completed"}
	if err := tasksvc.New(rt.Graph).UpdateTask(ctx, c.ListID, c.TaskID, payload); err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"completed": c.TaskID})
	}
	fmt.Fprintf(os.Stdout, "Completed task %s\n", c.TaskID)
	return nil
}

type TasksDeleteCmd struct {
	ListID string `name:"list" required:"" help:"Task list ID"`
	TaskID string `name:"task" required:"" help:"Task ID"`
}

func (c *TasksDeleteCmd) Run(ctx context.Context) error {
	flags := rootFlagsFromContext(ctx)
	if flags == nil {
		flags = &RootFlags{}
	}
	if err := confirmDestructive(ctx, flags, fmt.Sprintf("delete task %s", c.TaskID)); err != nil {
		return err
	}

	rt, err := resolveRuntime(ctx, false)
	if err != nil {
		return err
	}
	if err := tasksvc.New(rt.Graph).DeleteTask(ctx, c.ListID, c.TaskID); err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"deleted": c.TaskID})
	}
	fmt.Fprintf(os.Stdout, "Deleted task %s\n", c.TaskID)
	return nil
}

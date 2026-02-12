package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jaredpalmer/mogcli/internal/outfmt"
)

func writeJSON(ctx context.Context, payload any) error {
	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, payload)
	}
	return nil
}

func printItemTable(ctx context.Context, items []map[string]any, columns []string) {
	w, done := tableWriter(ctx)
	defer done()
	_, _ = fmt.Fprintln(w, strings.Join(columns, "\t"))
	for _, item := range items {
		row := make([]string, 0, len(columns))
		for _, col := range columns {
			row = append(row, flattenValue(item[col]))
		}
		_, _ = fmt.Fprintln(w, strings.Join(row, "\t"))
	}
}

func printSingleMap(ctx context.Context, item map[string]any) {
	if outfmt.IsJSON(ctx) {
		_ = outfmt.WriteJSON(os.Stdout, item)
		return
	}

	keys := make([]string, 0, len(item))
	for key := range item {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		_, _ = fmt.Fprintf(os.Stdout, "%s: %s\n", key, flattenValue(item[key]))
	}
}

func flattenValue(v any) string {
	switch typed := v.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case float64:
		if typed == float64(int64(typed)) {
			return fmt.Sprintf("%d", int64(typed))
		}
		return fmt.Sprintf("%g", typed)
	case int:
		return fmt.Sprintf("%d", typed)
	case int64:
		return fmt.Sprintf("%d", typed)
	case map[string]any:
		if name, ok := typed["name"].(string); ok {
			return name
		}
		if address, ok := typed["address"].(string); ok {
			return address
		}
		return "{...}"
	case []any:
		if len(typed) == 0 {
			return ""
		}
		first := typed[0]
		if m, ok := first.(map[string]any); ok {
			if email, ok := m["emailAddress"].(map[string]any); ok {
				if address, ok := email["address"].(string); ok {
					return address
				}
			}
			if address, ok := m["address"].(string); ok {
				return address
			}
		}
		return fmt.Sprintf("%d items", len(typed))
	default:
		return fmt.Sprintf("%v", typed)
	}
}

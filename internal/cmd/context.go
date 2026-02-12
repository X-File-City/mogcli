package cmd

import (
	"context"

	"github.com/jaredpalmer/mogcli/internal/ui"
)

type rootFlagsContextKey struct{}

func withRootFlags(ctx context.Context, flags *RootFlags) context.Context {
	return context.WithValue(ctx, rootFlagsContextKey{}, flags)
}

func rootFlagsFromContext(ctx context.Context) *RootFlags {
	if ctx == nil {
		return nil
	}
	if v := ctx.Value(rootFlagsContextKey{}); v != nil {
		if flags, ok := v.(*RootFlags); ok {
			return flags
		}
	}

	return nil
}

func uiFromContext(ctx context.Context) *ui.UI {
	return ui.FromContext(ctx)
}

package control

import "context"

type Control struct {
	PreferBackup bool `json:"prefer_backup"`
}

const controlContextKey = "chat-control"

func NewContext(ctx context.Context, ctl *Control) context.Context {
	return context.WithValue(ctx, controlContextKey, ctl)
}

func FromContext(ctx context.Context) *Control {
	u, ok := ctx.Value(controlContextKey).(*Control)
	if !ok {
		return &Control{
			PreferBackup: false,
		}
	}

	return u
}

package web

import "context"

type demoKey struct{}

func WithDemo(ctx context.Context) context.Context {
	return context.WithValue(ctx, demoKey{}, true)
}

func IsDemo(ctx context.Context) bool {
	demo, _ := ctx.Value(demoKey{}).(bool)
	return demo
}

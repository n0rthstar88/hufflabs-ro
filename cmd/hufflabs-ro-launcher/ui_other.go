//go:build !windows

package main

import (
	"context"
	"fmt"
)

// runUI on other systems prints the screens; the real launcher is the Windows exe
// (Linux runs it through Proton). Choices default to Play anyway.
func runUI(ctx context.Context, flow func(context.Context, func(View), <-chan Choice) Result) Result {
	ch := make(chan Choice, 1)
	return flow(ctx, func(v View) {
		fmt.Printf("%s %s\n", v.Title, v.Detail)
		if v.CanPlay {
			select {
			case ch <- PlayAnyway:
			default:
			}
		}
	}, ch)
}

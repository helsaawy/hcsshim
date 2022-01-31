package main

import (
	"context"
	"fmt"
)

const (
	DefaultDisplayWidth = 20
	_displayFormat      = "%-*s: %v\n"
)

func Display(ctx context.Context, title string, value interface{}, width int) {
	if width < 0 {
		width = DefaultDisplayWidth
	}
	fmt.Printf(_displayFormat, width, title, value)
}

//go:build tools

// Package config tools.go tracks tool dependencies that are used by the config package
// but not directly imported in production code yet.
//
// This file ensures these dependencies remain in go.mod and are available
// when their functionality is implemented in subsequent plans.
package config

import (
	// fsnotify is used for hot-reload file watching (Plan 07-03)
	_ "github.com/fsnotify/fsnotify"

	// go-toml/v2 is used for TOML config file parsing (Plan 07-02)
	_ "github.com/pelletier/go-toml/v2"
)

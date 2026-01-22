// Package functional provides documentation and usage examples for samber libraries
// commonly used in cc-relay. This package ensures samber libraries remain as
// direct dependencies and serves as a reference for patterns.
package functional

import (
	// Import samber libraries to ensure they remain in go.mod as direct dependencies.
	"github.com/leanovate/gopter"
	"github.com/samber/do/v2"
	"github.com/samber/lo"
	"github.com/samber/mo"
	"github.com/samber/ro"
)

// VersionInfo captures library versions used in this project.
// This is useful for debugging and ensuring consistency.
var VersionInfo = struct {
	Lo     string
	Do     string
	Mo     string
	Ro     string
	Gopter string
}{
	Lo:     "v1.52.0",
	Do:     "v2.0.0",
	Mo:     "v1.16.0",
	Ro:     "v0.2.0",
	Gopter: "v0.2.11",
}

// VerifyImports ensures all samber libraries can be imported.
// This function is called during tests to verify dependencies are present.
func VerifyImports() bool {
	// lo: functional collection utilities
	_ = lo.Filter([]int{1, 2, 3}, func(x int, _ int) bool { return x > 1 })

	// mo: monads (Option, Result)
	_ = mo.Some(42)
	_ = mo.Ok("success")

	// do: dependency injection
	injector := do.New()
	_ = injector

	// ro: reactive streams
	_ = ro.Just(1)

	// gopter: property-based testing
	_ = gopter.NewProperties(nil)

	return true
}

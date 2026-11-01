package app

import (
	"github.com/vantt/skillstation/internal/catalog"
	"github.com/vantt/skillstation/internal/mutation"
)

// The errors and states below belong to the packages under app. They are named
// here so a delivery surface tells them apart with errors.Is, or compares a
// catalog status, through app alone.
var (
	// ErrWorkspaceBusy reports that another process holds the hub lock.
	ErrWorkspaceBusy = mutation.ErrWorkspaceBusy
	// ErrCatalogUnavailable reports a catalog that is missing, stale or corrupt.
	ErrCatalogUnavailable = catalog.ErrCatalogUnavailable
	// ErrSkillNotServable reports an active skill the catalog cannot serve.
	ErrSkillNotServable = catalog.ErrSkillNotServable
)

// ServingFallback is the serving mode of a catalog that serves an older
// generation because the current one cannot be served.
const ServingFallback = catalog.ServingFallback

package core

import "github.com/yogirk/cascade/internal/tools"

// RegisterAll registers all core file tools with the given registry.
func RegisterAll(registry *tools.Registry) {
	registry.Register(&ReadTool{})
	registry.Register(&WriteTool{})
	registry.Register(&EditTool{})
	registry.Register(&GlobTool{})
	registry.Register(&GrepTool{})
	registry.Register(&BashTool{})
}

package bigquery

import "github.com/yogirk/cascade/internal/tools"

// RegisterAll registers all BigQuery tools with the given registry.
// Unlike core.RegisterAll which creates tools internally, BQ tools require
// external dependencies (client, cache, cost tracker), so they are
// pre-constructed and passed in.
func RegisterAll(registry *tools.Registry, queryTool *QueryTool, schemaTool *SchemaTool) {
	registry.Register(queryTool)
	registry.Register(schemaTool)
}

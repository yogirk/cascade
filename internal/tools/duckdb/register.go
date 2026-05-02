package duckdb

import "github.com/slokam-ai/cascade/internal/tools"

// RegisterAll registers DuckDB tools with the registry. Called by app
// wiring once the duckdb CLI has been detected and the runtime / session
// have been built. Each tool is allowed to be nil — only the ones whose
// dependencies were satisfied get registered, so the agent never sees a
// half-wired bq_to_duckdb when the staging bucket is unset.
func RegisterAll(reg *tools.Registry, query *QueryTool, schema *SchemaTool, bqToDuck *BQToDuckDBTool) {
	if query != nil {
		reg.Register(query)
	}
	if schema != nil {
		reg.Register(schema)
	}
	if bqToDuck != nil {
		reg.Register(bqToDuck)
	}
}

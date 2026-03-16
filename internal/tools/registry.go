package tools

import "github.com/yogirk/cascade/internal/provider"

// Registry manages registered tools and provides lookup and declaration generation.
type Registry struct {
	tools map[string]Tool
	order []string // preserves registration order
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry.
func (r *Registry) Register(tool Tool) {
	name := tool.Name()
	if _, exists := r.tools[name]; !exists {
		r.order = append(r.order, name)
	}
	r.tools[name] = tool
}

// Get returns the tool with the given name, or nil if not found.
func (r *Registry) Get(name string) Tool {
	return r.tools[name]
}

// All returns all registered tools in registration order.
func (r *Registry) All() []Tool {
	result := make([]Tool, 0, len(r.order))
	for _, name := range r.order {
		result = append(result, r.tools[name])
	}
	return result
}

// Declarations generates provider.Declaration entries for all registered tools,
// suitable for passing to an LLM.
func (r *Registry) Declarations() []provider.Declaration {
	decls := make([]provider.Declaration, 0, len(r.order))
	for _, name := range r.order {
		t := r.tools[name]
		decls = append(decls, provider.Declaration{
			Name:        t.Name(),
			Description: t.Description(),
			Schema:      t.InputSchema(),
		})
	}
	return decls
}

package themes

import "sort"

// DefaultName is the theme applied when no explicit theme is configured.
// Kept as classic while peers evaluate the new theses — switching the
// default out of main's palette would change how first-run users experience
// cascade before the new themes have earned that slot.
const DefaultName = "classic"

// registry holds all registered themes keyed by stable Name.
var registry = map[string]Theme{
	Classic.Name:  Classic,
	Verse.Name:    Verse,
	Midnight.Name: Midnight,
}

// Default returns the default theme. Never returns the zero Theme — if the
// registry is empty (shouldn't happen in practice), panics at init time.
func Default() Theme {
	t, ok := registry[DefaultName]
	if !ok {
		// Registry misconfiguration — surface loudly rather than silently
		// returning a zero-value Theme with empty palettes.
		panic("themes: DefaultName " + DefaultName + " not in registry")
	}
	return t
}

// Get returns the theme with the given Name and a bool indicating whether
// a match was found. Unknown names return the default theme and false.
func Get(name string) (Theme, bool) {
	if t, ok := registry[name]; ok {
		return t, true
	}
	return Default(), false
}

// All returns every registered theme in stable alphabetical order by Name.
func All() []Theme {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]Theme, 0, len(names))
	for _, n := range names {
		out = append(out, registry[n])
	}
	return out
}

// Names returns every registered theme Name in stable alphabetical order.
func Names() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

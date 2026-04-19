package themes

import "testing"

func TestDefault_Registered(t *testing.T) {
	d := Default()
	if d.Name != DefaultName {
		t.Fatalf("Default() returned theme %q, expected %q", d.Name, DefaultName)
	}
	if d.DisplayName == "" {
		t.Error("Default() theme has empty DisplayName")
	}
	if d.Description == "" {
		t.Error("Default() theme has empty Description")
	}
}

func TestGet_KnownTheme(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"verse", "verse"},
		{"midnight", "midnight"},
		{"classic", "classic"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Get(tt.name)
			if !ok {
				t.Fatalf("Get(%q) returned ok=false; expected known theme", tt.name)
			}
			if got.Name != tt.want {
				t.Errorf("Get(%q).Name = %q, want %q", tt.name, got.Name, tt.want)
			}
		})
	}
}

func TestGet_UnknownThemeReturnsDefault(t *testing.T) {
	got, ok := Get("no-such-theme")
	if ok {
		t.Error("Get(unknown) returned ok=true; expected false")
	}
	if got.Name != DefaultName {
		t.Errorf("Get(unknown) returned %q; expected default %q", got.Name, DefaultName)
	}
}

func TestAll_StableOrder(t *testing.T) {
	a1 := All()
	a2 := All()
	if len(a1) != len(a2) {
		t.Fatalf("All() returned different lengths across calls: %d vs %d", len(a1), len(a2))
	}
	for i := range a1 {
		if a1[i].Name != a2[i].Name {
			t.Errorf("All() order unstable at index %d: %q vs %q", i, a1[i].Name, a2[i].Name)
		}
	}
	if len(a1) < 2 {
		t.Fatalf("All() returned %d themes; expected at least 2 registered", len(a1))
	}
}

func TestAll_IncludesShippedThemes(t *testing.T) {
	seen := map[string]bool{}
	for _, t := range All() {
		seen[t.Name] = true
	}
	for _, want := range []string{"verse", "midnight", "classic"} {
		if !seen[want] {
			t.Errorf("All() missing shipped theme %q", want)
		}
	}
}

func TestNames_MatchesAll(t *testing.T) {
	names := Names()
	all := All()
	if len(names) != len(all) {
		t.Fatalf("Names() length %d != All() length %d", len(names), len(all))
	}
	for i := range names {
		if names[i] != all[i].Name {
			t.Errorf("Names()/All() disagree at %d: %q vs %q", i, names[i], all[i].Name)
		}
	}
}

func TestPick_ReturnsCorrectVariant(t *testing.T) {
	theme, _ := Get("verse")

	dark := theme.Pick(true)
	light := theme.Pick(false)

	// Palettes must be meaningfully different between light and dark — if
	// they're equal, something is very wrong in the theme definition.
	if dark.Bright == light.Bright {
		t.Error("Pick(dark).Bright == Pick(light).Bright; palettes look identical")
	}
	if dark.InputBg == light.InputBg {
		t.Error("Pick(dark).InputBg == Pick(light).InputBg; palettes look identical")
	}
}

// TestPalettes_NoZeroColors asserts every palette field is populated.
// A zero color.Color (nil interface) would render as terminal default and
// silently drop the theme's visual identity on that element. Surfaces bugs
// where a new Palette field is added without being filled in every theme.
func TestPalettes_NoZeroColors(t *testing.T) {
	for _, theme := range All() {
		t.Run(theme.Name+"/dark", func(t *testing.T) {
			checkPaletteFields(t, theme.Dark, theme.Name+" dark")
		})
		t.Run(theme.Name+"/light", func(t *testing.T) {
			checkPaletteFields(t, theme.Light, theme.Name+" light")
		})
	}
}

func checkPaletteFields(t *testing.T, p Palette, label string) {
	t.Helper()
	fields := []struct {
		name string
		val  interface{}
	}{
		{"Accent", p.Accent}, {"DimText", p.DimText}, {"Text", p.Text},
		{"Bright", p.Bright}, {"Success", p.Success}, {"Warning", p.Warning},
		{"Danger", p.Danger}, {"Tool", p.Tool}, {"Plan", p.Plan},
		{"SettledAccent", p.SettledAccent},
		{"DiffAddBg", p.DiffAddBg}, {"DiffAddFg", p.DiffAddFg},
		{"DiffRemBg", p.DiffRemBg}, {"DiffRemFg", p.DiffRemFg},
		{"InputBorder", p.InputBorder}, {"InputBorderDim", p.InputBorderDim},
		{"InputBg", p.InputBg},
		{"SweepDim", p.SweepDim}, {"SweepMid", p.SweepMid}, {"SweepBright", p.SweepBright},
		{"CascadeDim", p.CascadeDim}, {"CascadeTrail", p.CascadeTrail},
		{"CascadeBright", p.CascadeBright}, {"CascadePeak", p.CascadePeak},
		{"CascadeBg1", p.CascadeBg1}, {"CascadeBg2", p.CascadeBg2}, {"CascadeBg3", p.CascadeBg3},
	}
	for _, f := range fields {
		if f.val == nil {
			t.Errorf("%s: field %q is nil", label, f.name)
		}
	}
}

package permission_test

import (
	"encoding/json"
	"testing"

	"github.com/yogirk/cascade/internal/permission"
)

// mockTool implements ToolRiskProvider for testing.
type mockTool struct {
	name string
	risk permission.RiskLevel
}

func (m *mockTool) Name() string                   { return m.name }
func (m *mockTool) RiskLevel() permission.RiskLevel { return m.risk }

// --- Risk Level Tests ---

func TestRiskLevel_String(t *testing.T) {
	cases := []struct {
		level permission.RiskLevel
		want  string
	}{
		{permission.RiskReadOnly, "READ_ONLY"},
		{permission.RiskDML, "DML"},
		{permission.RiskDDL, "DDL"},
		{permission.RiskDestructive, "DESTRUCTIVE"},
		{permission.RiskAdmin, "ADMIN"},
	}

	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			got := tc.level.String()
			if got != tc.want {
				t.Errorf("RiskLevel(%d).String() = %q, want %q", tc.level, got, tc.want)
			}
		})
	}
}

func TestRiskLevel_Ordering(t *testing.T) {
	if !(permission.RiskReadOnly < permission.RiskDML) {
		t.Error("expected RiskReadOnly < RiskDML")
	}
	if !(permission.RiskDML < permission.RiskDDL) {
		t.Error("expected RiskDML < RiskDDL")
	}
	if !(permission.RiskDDL < permission.RiskDestructive) {
		t.Error("expected RiskDDL < RiskDestructive")
	}
	if !(permission.RiskDestructive < permission.RiskAdmin) {
		t.Error("expected RiskDestructive < RiskAdmin")
	}
}

func TestRequiresConfirmation(t *testing.T) {
	cases := []struct {
		level permission.RiskLevel
		want  bool
	}{
		{permission.RiskReadOnly, false},
		{permission.RiskDML, true},
		{permission.RiskDDL, true},
		{permission.RiskDestructive, true},
		{permission.RiskAdmin, true},
	}

	for _, tc := range cases {
		t.Run(tc.level.String(), func(t *testing.T) {
			got := permission.RequiresConfirmation(tc.level)
			if got != tc.want {
				t.Errorf("RequiresConfirmation(%v) = %v, want %v", tc.level, got, tc.want)
			}
		})
	}
}

func TestRiskLevel_Badge(t *testing.T) {
	cases := []struct {
		level permission.RiskLevel
		want  string
	}{
		{permission.RiskReadOnly, "[READ]"},
		{permission.RiskDML, "[DML]"},
		{permission.RiskDDL, "[DDL]"},
		{permission.RiskDestructive, "[DESTRUCTIVE]"},
		{permission.RiskAdmin, "[ADMIN]"},
	}

	for _, tc := range cases {
		t.Run(tc.level.String(), func(t *testing.T) {
			got := tc.level.Badge()
			if got != tc.want {
				t.Errorf("RiskLevel(%d).Badge() = %q, want %q", tc.level, got, tc.want)
			}
		})
	}
}

// --- Mode Tests ---

func TestMode_String(t *testing.T) {
	cases := []struct {
		mode permission.Mode
		want string
	}{
		{permission.ModeAsk, "ASK"},
		{permission.ModeReadOnly, "READ ONLY"},
		{permission.ModeFullAccess, "FULL ACCESS"},
	}

	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			got := tc.mode.String()
			if got != tc.want {
				t.Errorf("Mode(%d).String() = %q, want %q", tc.mode, got, tc.want)
			}
		})
	}
}

func TestCycleMode(t *testing.T) {
	cases := []struct {
		from permission.Mode
		to   permission.Mode
	}{
		{permission.ModeAsk, permission.ModeReadOnly},
		{permission.ModeReadOnly, permission.ModeFullAccess},
		{permission.ModeFullAccess, permission.ModeAsk},
	}

	for _, tc := range cases {
		t.Run(tc.from.String()+"->"+tc.to.String(), func(t *testing.T) {
			got := permission.CycleMode(tc.from)
			if got != tc.to {
				t.Errorf("CycleMode(%v) = %v, want %v", tc.from, got, tc.to)
			}
		})
	}
}

// --- Engine Tests ---

func TestCheck_AskMode_ReadOnly(t *testing.T) {
	engine := permission.NewEngine(permission.ModeAsk)
	tool := &mockTool{name: "read_file", risk: permission.RiskReadOnly}
	got := engine.Check(tool, json.RawMessage(`{}`))
	if got != permission.Allow {
		t.Errorf("Check(ReadOnly in ASK) = %v, want Allow", got)
	}
}

func TestCheck_AskMode_DML(t *testing.T) {
	engine := permission.NewEngine(permission.ModeAsk)
	tool := &mockTool{name: "write_file", risk: permission.RiskDML}
	got := engine.Check(tool, json.RawMessage(`{}`))
	if got != permission.Confirm {
		t.Errorf("Check(DML in ASK) = %v, want Confirm", got)
	}
}

func TestCheck_AskMode_Destructive(t *testing.T) {
	engine := permission.NewEngine(permission.ModeAsk)
	tool := &mockTool{name: "bash", risk: permission.RiskDestructive}
	got := engine.Check(tool, json.RawMessage(`{}`))
	if got != permission.Confirm {
		t.Errorf("Check(Destructive in ASK) = %v, want Confirm", got)
	}
}

func TestCheck_ReadOnlyMode_ReadOnly(t *testing.T) {
	engine := permission.NewEngine(permission.ModeReadOnly)
	tool := &mockTool{name: "read_file", risk: permission.RiskReadOnly}
	got := engine.Check(tool, json.RawMessage(`{}`))
	if got != permission.Allow {
		t.Errorf("Check(ReadOnly in READ ONLY) = %v, want Allow", got)
	}
}

func TestCheck_ReadOnlyMode_DML(t *testing.T) {
	engine := permission.NewEngine(permission.ModeReadOnly)
	tool := &mockTool{name: "write_file", risk: permission.RiskDML}
	got := engine.Check(tool, json.RawMessage(`{}`))
	if got != permission.Deny {
		t.Errorf("Check(DML in READ ONLY) = %v, want Deny", got)
	}
}

func TestCheck_ReadOnlyMode_Destructive(t *testing.T) {
	engine := permission.NewEngine(permission.ModeReadOnly)
	tool := &mockTool{name: "bash", risk: permission.RiskDestructive}
	got := engine.Check(tool, json.RawMessage(`{}`))
	if got != permission.Deny {
		t.Errorf("Check(Destructive in READ ONLY) = %v, want Deny", got)
	}
}

func TestCheck_FullAccessMode_ReadOnly(t *testing.T) {
	engine := permission.NewEngine(permission.ModeFullAccess)
	tool := &mockTool{name: "read_file", risk: permission.RiskReadOnly}
	got := engine.Check(tool, json.RawMessage(`{}`))
	if got != permission.Allow {
		t.Errorf("Check(ReadOnly in FULL ACCESS) = %v, want Allow", got)
	}
}

func TestCheck_FullAccessMode_DML(t *testing.T) {
	engine := permission.NewEngine(permission.ModeFullAccess)
	tool := &mockTool{name: "write_file", risk: permission.RiskDML}
	got := engine.Check(tool, json.RawMessage(`{}`))
	if got != permission.Allow {
		t.Errorf("Check(DML in FULL ACCESS) = %v, want Allow", got)
	}
}

func TestCheck_FullAccessMode_Destructive(t *testing.T) {
	engine := permission.NewEngine(permission.ModeFullAccess)
	tool := &mockTool{name: "bash", risk: permission.RiskDestructive}
	got := engine.Check(tool, json.RawMessage(`{}`))
	if got != permission.Allow {
		t.Errorf("Check(Destructive in FULL ACCESS) = %v, want Allow", got)
	}
}

func TestCacheDecision(t *testing.T) {
	engine := permission.NewEngine(permission.ModeAsk)
	tool := &mockTool{name: "write_file", risk: permission.RiskDML}
	input := json.RawMessage(`{"file_path": "/tmp/test.txt"}`)

	// Before caching, should need confirmation
	got := engine.Check(tool, input)
	if got != permission.Confirm {
		t.Errorf("before cache: Check = %v, want Confirm", got)
	}

	// Cache an Allow decision
	engine.CacheDecision(tool.Name(), input, permission.Allow)

	// After caching, should auto-allow
	got = engine.Check(tool, input)
	if got != permission.Allow {
		t.Errorf("after cache: Check = %v, want Allow", got)
	}
}

func TestCacheToolDecision(t *testing.T) {
	engine := permission.NewEngine(permission.ModeAsk)
	tool := &mockTool{name: "bash", risk: permission.RiskDML}

	if got := engine.Check(tool, json.RawMessage(`{"command":"echo one"}`)); got != permission.Confirm {
		t.Fatalf("before tool cache: Check = %v, want Confirm", got)
	}

	engine.CacheToolDecision(tool.Name(), permission.Allow)

	if got := engine.Check(tool, json.RawMessage(`{"command":"echo two"}`)); got != permission.Allow {
		t.Fatalf("after tool cache: Check = %v, want Allow", got)
	}
}

func TestEngine_CycleMode(t *testing.T) {
	engine := permission.NewEngine(permission.ModeAsk)

	if engine.Mode() != permission.ModeAsk {
		t.Fatalf("initial mode = %v, want ASK", engine.Mode())
	}

	engine.CycleMode()
	if engine.Mode() != permission.ModeReadOnly {
		t.Errorf("after first cycle: %v, want READ ONLY", engine.Mode())
	}

	engine.CycleMode()
	if engine.Mode() != permission.ModeFullAccess {
		t.Errorf("after second cycle: %v, want FULL ACCESS", engine.Mode())
	}

	engine.CycleMode()
	if engine.Mode() != permission.ModeAsk {
		t.Errorf("after third cycle: %v, want ASK", engine.Mode())
	}
}

func TestEngine_SetMode(t *testing.T) {
	engine := permission.NewEngine(permission.ModeAsk)
	engine.SetMode(permission.ModeFullAccess)
	if engine.Mode() != permission.ModeFullAccess {
		t.Errorf("after SetMode(FullAccess): %v, want FULL ACCESS", engine.Mode())
	}
}

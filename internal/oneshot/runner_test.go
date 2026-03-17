package oneshot

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/cascade-cli/cascade/internal/permission"
	"github.com/cascade-cli/cascade/pkg/types"
)

// TestEventProcessing_Tokens verifies that TokenEvents are written to stdout.
func TestEventProcessing_Tokens(t *testing.T) {
	events := make(chan types.Event, 10)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// Send token events then done
	events <- &types.TokenEvent{Token: "Hello"}
	events <- &types.TokenEvent{Token: " "}
	events <- &types.TokenEvent{Token: "world"}
	events <- &types.DoneEvent{}
	close(events)

	processEvents(events, nil, stdout, stderr)

	got := stdout.String()
	if !strings.Contains(got, "Hello world") {
		t.Errorf("expected stdout to contain 'Hello world', got %q", got)
	}
}

// TestEventProcessing_PermissionDenied verifies auto-deny in non-bypass mode.
func TestEventProcessing_PermissionDenied(t *testing.T) {
	events := make(chan types.Event, 10)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	responseCh := make(chan bool, 1)
	events <- &types.PermissionRequestEvent{
		ToolName:  "bash",
		RiskLevel: "DESTRUCTIVE",
		Response:  responseCh,
	}
	events <- &types.DoneEvent{}
	close(events)

	perms := permission.NewEngine(permission.ModeConfirm)
	processEvents(events, perms, stdout, stderr)

	// Check that permission was denied
	approved := <-responseCh
	if approved {
		t.Error("expected permission to be denied in confirm mode")
	}

	// Check stderr has warning
	if !strings.Contains(stderr.String(), "denied in one-shot mode") {
		t.Errorf("expected denial warning on stderr, got %q", stderr.String())
	}
}

// TestEventProcessing_PermissionBypass verifies auto-approve in bypass mode.
func TestEventProcessing_PermissionBypass(t *testing.T) {
	events := make(chan types.Event, 10)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	responseCh := make(chan bool, 1)
	events <- &types.PermissionRequestEvent{
		ToolName:  "bash",
		RiskLevel: "DESTRUCTIVE",
		Response:  responseCh,
	}
	events <- &types.DoneEvent{}
	close(events)

	perms := permission.NewEngine(permission.ModeBypass)
	processEvents(events, perms, stdout, stderr)

	// Check that permission was approved
	approved := <-responseCh
	if !approved {
		t.Error("expected permission to be approved in bypass mode")
	}
}

// TestEventProcessing_ErrorEvent verifies error events are written to stderr.
func TestEventProcessing_ErrorEvent(t *testing.T) {
	events := make(chan types.Event, 10)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	events <- &types.ErrorEvent{Err: fmt.Errorf("something broke")}
	events <- &types.DoneEvent{}
	close(events)

	processEvents(events, nil, stdout, stderr)

	if !strings.Contains(stderr.String(), "something broke") {
		t.Errorf("expected error on stderr, got %q", stderr.String())
	}
}

// TestEventProcessing_ToolError verifies tool errors are written to stderr.
func TestEventProcessing_ToolError(t *testing.T) {
	events := make(chan types.Event, 10)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	events <- &types.ToolEndEvent{Name: "bash", Content: "command not found", IsError: true}
	events <- &types.DoneEvent{}
	close(events)

	processEvents(events, nil, stdout, stderr)

	if !strings.Contains(stderr.String(), "Tool error (bash)") {
		t.Errorf("expected tool error on stderr, got %q", stderr.String())
	}
}

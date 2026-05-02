// Package oneshot implements one-shot mode for Cascade: send a single prompt
// to the agent, print streaming output to stdout, and exit.
package oneshot

import (
	"context"
	"fmt"
	"io"

	"github.com/slokam-ai/cascade/internal/app"
	"github.com/slokam-ai/cascade/internal/permission"
	"github.com/slokam-ai/cascade/pkg/types"
)

// Run executes a one-shot conversation: sends the prompt to the agent, streams
// tokens to stdout, and exits when the turn is done.
func Run(ctx context.Context, application *app.App, prompt string, stdout io.Writer, stderr io.Writer) error {
	// Start goroutine to consume events and write to stdout
	done := make(chan error, 1)
	go func() {
		processEvents(application.Events, application.Approvals, application.Permissions, stdout, stderr)
		done <- nil
	}()

	// Run single turn
	if err := application.Agent.RunTurn(ctx, prompt); err != nil {
		return err
	}

	return <-done
}

// processEvents reads events from the channel and writes output to stdout/stderr.
// It returns when a DoneEvent is received or the channel is closed.
func processEvents(events <-chan types.Event, approvals <-chan types.ApprovalRequest, perms *permission.Engine, stdout io.Writer, stderr io.Writer) {
	for {
		select {
		case req, ok := <-approvals:
			if !ok {
				approvals = nil
				break
			}
			if perms != nil && perms.Mode() == permission.ModeFullAccess {
				req.Response <- types.ApprovalDecision{Action: types.ApprovalAllowOnce}
			} else {
				fmt.Fprintf(stderr, "\nPermission required for %s [%s] -- denied in one-shot mode. Use --bypass to enable full-access mode.\n",
					req.ToolName, req.RiskLevel)
				req.Response <- types.ApprovalDecision{Action: types.ApprovalDeny}
			}
			continue
		default:
		}

		select {
		case req, ok := <-approvals:
			if !ok {
				approvals = nil
				continue
			}
			// One-shot mode auto-denies DML+ operations for safety
			// unless the user explicitly enabled full-access mode.
			if perms != nil && perms.Mode() == permission.ModeFullAccess {
				req.Response <- types.ApprovalDecision{Action: types.ApprovalAllowOnce}
			} else {
				fmt.Fprintf(stderr, "\nPermission required for %s [%s] -- denied in one-shot mode. Use --bypass to enable full-access mode.\n",
					req.ToolName, req.RiskLevel)
				req.Response <- types.ApprovalDecision{Action: types.ApprovalDeny}
			}
		case event, ok := <-events:
			if !ok {
				return
			}
			switch e := event.(type) {
			case *types.TokenEvent:
				fmt.Fprint(stdout, e.Token)
			case *types.ToolStartEvent:
				// In one-shot mode, tool execution is silent (no spinner)
			case *types.ToolEndEvent:
				if e.IsError {
					fmt.Fprintf(stderr, "\nTool error (%s): %s\n", e.Name, e.Content)
				}
			case *types.ErrorEvent:
				fmt.Fprintf(stderr, "\nError: %v\n", e.Err)
			case *types.DoneEvent:
				fmt.Fprintln(stdout) // Final newline
				return
			}
		}
	}
}

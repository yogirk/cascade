// Package duckdb is the runtime layer for Cascade's DuckDB integration.
// It owns subprocess invocation, the SQL classifier, OAuth-bearer GCS
// auth via httpfs, the BQ-side volume gate, and the per-invocation
// session DB. Tool-facing code lives in internal/tools/duckdb.
package duckdb

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// MinVersion is the floor we require for the `duckdb` CLI. The storage
// format has had cross-minor breakage in DuckDB's history; pinning here
// keeps the session DB readable across an upgrade-on-the-laptop event.
//
// Confirm against deployed installs before bumping.
const MinVersion = "1.2.0"

// Detection holds the resolved CLI state for a Cascade session. The check
// runs once at startup; tools register only when Available is true.
type Detection struct {
	// Path is the resolved CLI path (from config.cli_path or PATH).
	Path string
	// Version is the parsed `duckdb -version` output, e.g. "1.2.1".
	Version string
	// Available reports whether the CLI is present and meets MinVersion.
	Available bool
	// Reason explains a non-Available outcome (missing, too old, error).
	Reason string
}

// Detect locates the duckdb CLI and verifies its version. cliPath, if
// non-empty, overrides PATH lookup. The function never returns an error:
// every failure mode is described in Detection.Reason so tool-registration
// can render a friendly install hint instead of crashing the agent.
func Detect(ctx context.Context, cliPath string) Detection {
	path := cliPath
	if path == "" {
		resolved, err := exec.LookPath("duckdb")
		if err != nil {
			return Detection{Reason: missingHint()}
		}
		path = resolved
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, path, "-version").CombinedOutput()
	if err != nil {
		return Detection{Path: path, Reason: fmt.Sprintf("duckdb -version failed: %v", err)}
	}

	version := parseVersion(string(out))
	if version == "" {
		return Detection{Path: path, Reason: "could not parse duckdb -version output"}
	}

	if compareSemver(version, MinVersion) < 0 {
		return Detection{
			Path:    path,
			Version: version,
			Reason: fmt.Sprintf(
				"duckdb %s is below required %s — upgrade with `brew upgrade duckdb` or your platform equivalent",
				version, MinVersion,
			),
		}
	}

	return Detection{Path: path, Version: version, Available: true}
}

// versionRE matches a leading semver-like token in duckdb's -version output.
// Examples: "v1.2.1 abc1234", "DuckDB v0.10.2".
var versionRE = regexp.MustCompile(`v?(\d+\.\d+\.\d+)`)

func parseVersion(s string) string {
	m := versionRE.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// compareSemver returns -1, 0, or 1 comparing a and b numerically by their
// dotted segments. Both inputs are assumed already validated by the regex.
func compareSemver(a, b string) int {
	as := splitSegments(a)
	bs := splitSegments(b)
	for i := range 3 {
		if as[i] < bs[i] {
			return -1
		}
		if as[i] > bs[i] {
			return 1
		}
	}
	return 0
}

func splitSegments(v string) [3]int {
	var out [3]int
	parts := strings.Split(v, ".")
	for i := 0; i < 3 && i < len(parts); i++ {
		n, _ := strconv.Atoi(parts[i])
		out[i] = n
	}
	return out
}

// missingHint returns the platform-specific install hint surfaced when the
// duckdb CLI is not on PATH. Kept as a single string so the same text
// surfaces in startup banners, tool-registration logs, and structured
// errors.
func missingHint() string {
	return "duckdb CLI not found on PATH. Install:\n" +
		"  macOS:   brew install duckdb\n" +
		"  Linux:   curl https://install.duckdb.org | sh\n" +
		"  Windows: winget install DuckDB.cli"
}

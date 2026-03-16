package gcs

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"cloud.google.com/go/storage"
)

var (
	gcsHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B9FFF")).Bold(true)
	gcsDimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#4B5563"))
	gcsTextStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#D1D5DB"))
	gcsBrightStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F3F4F6")).Bold(true)
	gcsDirStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#38BDF8"))
	gcsSepStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#374151"))
	gcsLineNumStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#4B5563"))
)

// RenderBucketList renders a list of buckets.
func RenderBucketList(buckets []BucketInfo, projectID string) (display string, content string) {
	var db, cb strings.Builder

	db.WriteString("\n  " + gcsHeaderStyle.Render("Buckets") + "  " +
		gcsDimStyle.Render(fmt.Sprintf("%s · %d buckets", projectID, len(buckets))) + "\n")
	db.WriteString("  " + gcsSepStyle.Render(strings.Repeat("─", 60)) + "\n\n")
	cb.WriteString(fmt.Sprintf("Buckets in %s (%d)\n\n", projectID, len(buckets)))

	if len(buckets) == 0 {
		db.WriteString("  " + gcsDimStyle.Render("No buckets found.") + "\n")
		cb.WriteString("No buckets found.\n")
		return db.String(), cb.String()
	}

	for _, b := range buckets {
		db.WriteString(fmt.Sprintf("  %s  %s  %s\n",
			gcsBrightStyle.Render(b.Name),
			gcsDimStyle.Render(b.Location),
			gcsDimStyle.Render(b.StorageClass)))
		cb.WriteString(fmt.Sprintf("  %s  %s  %s\n", b.Name, b.Location, b.StorageClass))
	}

	db.WriteString("\n")
	return db.String(), cb.String()
}

// RenderObjectList renders a list of objects with directory-style browsing.
func RenderObjectList(objects []ObjectInfo, bucket, prefix string, truncated bool) (display string, content string) {
	var db, cb strings.Builder

	path := "gs://" + bucket
	if prefix != "" {
		path += "/" + prefix
	}

	db.WriteString("\n  " + gcsHeaderStyle.Render("Objects") + "  " +
		gcsDimStyle.Render(fmt.Sprintf("%s · %d items", path, len(objects))) + "\n")
	db.WriteString("  " + gcsSepStyle.Render(strings.Repeat("─", 60)) + "\n\n")
	cb.WriteString(fmt.Sprintf("Objects in %s (%d items)\n\n", path, len(objects)))

	if len(objects) == 0 {
		db.WriteString("  " + gcsDimStyle.Render("No objects found.") + "\n")
		cb.WriteString("No objects found.\n")
		return db.String(), cb.String()
	}

	for _, obj := range objects {
		if obj.IsDir {
			// Directory
			name := obj.Name
			if prefix != "" {
				name = strings.TrimPrefix(name, prefix)
			}
			db.WriteString(fmt.Sprintf("  %s  %s\n",
				gcsDirStyle.Render("📁"),
				gcsDirStyle.Render(name)))
			cb.WriteString(fmt.Sprintf("  [DIR] %s\n", obj.Name))
		} else {
			// File
			name := obj.Name
			if prefix != "" {
				name = strings.TrimPrefix(name, prefix)
			}
			db.WriteString(fmt.Sprintf("  %-40s  %s  %s\n",
				gcsTextStyle.Render(truncateName(name, 40)),
				gcsDimStyle.Render(formatSize(obj.Size)),
				gcsDimStyle.Render(obj.Updated.Local().Format("Jan 02 15:04"))))
			cb.WriteString(fmt.Sprintf("  %s  %s  %s\n", obj.Name, formatSize(obj.Size), obj.Updated.Format(time.RFC3339)))
		}
	}

	if truncated {
		db.WriteString("\n  " + gcsDimStyle.Render("... more items (listing capped at 100)") + "\n")
		cb.WriteString("  ... more items (capped)\n")
	}

	db.WriteString("\n")
	return db.String(), cb.String()
}

// RenderFileContent renders the first N lines of a file with line numbers.
func RenderFileContent(lines []string, bucket, object string, attrs *storage.ObjectAttrs, truncated bool, maxLines int) (display string, content string) {
	var db, cb strings.Builder

	path := fmt.Sprintf("gs://%s/%s", bucket, object)
	db.WriteString("\n  " + gcsHeaderStyle.Render(path) + "  " +
		gcsDimStyle.Render(fmt.Sprintf("%s · %s", formatSize(attrs.Size), attrs.ContentType)) + "\n")
	db.WriteString("  " + gcsSepStyle.Render(strings.Repeat("─", 60)) + "\n\n")
	cb.WriteString(fmt.Sprintf("%s (%s, %s)\n\n", path, formatSize(attrs.Size), attrs.ContentType))

	for i, line := range lines {
		lineNum := fmt.Sprintf("%4d", i+1)
		db.WriteString("  " + gcsLineNumStyle.Render(lineNum) + "  " + gcsTextStyle.Render(line) + "\n")
		cb.WriteString(fmt.Sprintf("%4d  %s\n", i+1, line))
	}

	if truncated {
		db.WriteString("\n  " + gcsDimStyle.Render(fmt.Sprintf("... showing first %d lines", maxLines)) + "\n")
		cb.WriteString(fmt.Sprintf("\n... showing first %d lines\n", maxLines))
	}

	db.WriteString("\n")
	return db.String(), cb.String()
}

// RenderObjectMeta renders object metadata.
func RenderObjectMeta(attrs *storage.ObjectAttrs, bucket string, isBinary bool) (display string, content string) {
	var db, cb strings.Builder

	path := fmt.Sprintf("gs://%s/%s", bucket, attrs.Name)
	db.WriteString("\n  " + gcsHeaderStyle.Render(path) + "\n")
	db.WriteString("  " + gcsSepStyle.Render(strings.Repeat("─", 60)) + "\n\n")
	cb.WriteString(fmt.Sprintf("%s\n\n", path))

	info := []struct{ label, value string }{
		{"Size", formatSize(attrs.Size)},
		{"Type", attrs.ContentType},
		{"Updated", attrs.Updated.Local().Format("2006-01-02 15:04:05")},
		{"Created", attrs.Created.Local().Format("2006-01-02 15:04:05")},
		{"Storage class", attrs.StorageClass},
	}

	for _, kv := range info {
		db.WriteString(fmt.Sprintf("  %-16s %s\n",
			gcsDimStyle.Render(kv.label),
			gcsBrightStyle.Render(kv.value)))
		cb.WriteString(fmt.Sprintf("  %s: %s\n", kv.label, kv.value))
	}

	if len(attrs.Metadata) > 0 {
		db.WriteString("\n  " + gcsDimStyle.Render("Custom metadata") + "\n")
		cb.WriteString("\n  Custom metadata:\n")
		for k, v := range attrs.Metadata {
			db.WriteString(fmt.Sprintf("    %s = %s\n",
				gcsDimStyle.Render(k), gcsTextStyle.Render(v)))
			cb.WriteString(fmt.Sprintf("    %s = %s\n", k, v))
		}
	}

	if isBinary {
		db.WriteString("\n  " + gcsDimStyle.Render("Binary file — content not displayed") + "\n")
		cb.WriteString("\n  Binary file — content not displayed\n")
	}

	db.WriteString("\n")
	return db.String(), cb.String()
}

func truncateName(name string, maxLen int) string {
	if len(name) <= maxLen {
		return name
	}
	return name[:maxLen-3] + "..."
}

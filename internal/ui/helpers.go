package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"vaws/internal/model"
)

// formatDuration formats seconds into a human-readable duration string.
func formatDuration(seconds int) string {
	if seconds >= 86400 {
		days := seconds / 86400
		return fmt.Sprintf("%d days", days)
	} else if seconds >= 3600 {
		hours := seconds / 3600
		return fmt.Sprintf("%d hours", hours)
	} else if seconds >= 60 {
		mins := seconds / 60
		return fmt.Sprintf("%d mins", mins)
	}
	return fmt.Sprintf("%ds", seconds)
}

// formatBytes formats bytes into a human-readable string.
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// truncateString truncates a string to fit within maxWidth.
func truncateString(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if len(s) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		return s[:maxWidth]
	}
	return s[:maxWidth-3] + "..."
}

// matchKey checks if a key message matches a key binding.
func matchKey(msg tea.KeyMsg, binding key.Binding) bool {
	for _, k := range binding.Keys() {
		if msg.String() == k {
			return true
		}
	}
	return false
}

// findBestContainer finds the best container for port forwarding.
// It prefers app containers over sidecars (otel, datadog, envoy, etc.)
// and prefers containers with common app ports (80, 8080, etc.)
func findBestContainer(containers []model.Container) *model.Container {
	// First pass: find non-sidecar containers with app ports and RuntimeID
	for i := range containers {
		c := &containers[i]
		if c.RuntimeID != "" && !c.IsSidecar() && c.HasAppPort() {
			return c
		}
	}

	// Second pass: find any non-sidecar container with RuntimeID
	for i := range containers {
		c := &containers[i]
		if c.RuntimeID != "" && !c.IsSidecar() {
			return c
		}
	}

	// Third pass: find any container with RuntimeID (including sidecars)
	for i := range containers {
		c := &containers[i]
		if c.RuntimeID != "" {
			return c
		}
	}

	return nil
}

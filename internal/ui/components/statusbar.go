package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"vaws/internal/ui/theme"
)

// StatusBar renders a single-row header with essential info.
//
// Example:
//
//	vaws v1.1.1  │  ◉ prod-profile  │  us-east-1  │  ⚡3 tunnels  │  ?help  qQuit
type StatusBar struct {
	width         int
	version       string
	profile       string
	region        string
	activeTunnels int
}

// NewStatusBar creates a new StatusBar component.
func NewStatusBar() *StatusBar {
	return &StatusBar{
		version: "dev",
	}
}

// SetWidth sets the status bar width.
func (s *StatusBar) SetWidth(width int) {
	s.width = width
}

// SetVersion sets the version string.
func (s *StatusBar) SetVersion(version string) {
	s.version = version
}

// SetProfile sets the AWS profile name.
func (s *StatusBar) SetProfile(profile string) {
	s.profile = profile
}

// SetRegion sets the AWS region.
func (s *StatusBar) SetRegion(region string) {
	s.region = region
}

// SetActiveTunnels sets the number of active tunnels.
func (s *StatusBar) SetActiveTunnels(count int) {
	s.activeTunnels = count
}

// View renders the status bar.
func (s *StatusBar) View() string {
	// Styles
	bgStyle := lipgloss.NewStyle().
		Background(theme.BgSubtle).
		Width(s.width)

	logoStyle := lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true)

	versionStyle := lipgloss.NewStyle().
		Foreground(theme.TextDim)

	separatorStyle := lipgloss.NewStyle().
		Foreground(theme.Border)

	profileStyle := lipgloss.NewStyle().
		Foreground(theme.Success).
		Bold(true)

	regionStyle := lipgloss.NewStyle().
		Foreground(theme.Info)

	tunnelStyle := lipgloss.NewStyle().
		Foreground(theme.Warning)

	keyStyle := lipgloss.NewStyle().
		Foreground(theme.TextMuted)

	separator := separatorStyle.Render(" │ ")

	// Build left side: logo + version
	left := logoStyle.Render("vaws") + " " + versionStyle.Render(s.version)

	// Build middle: profile + region + tunnels
	var middleParts []string

	if s.profile != "" {
		middleParts = append(middleParts, profileStyle.Render("◉ "+s.profile))
	}

	if s.region != "" {
		middleParts = append(middleParts, regionStyle.Render(s.region))
	}

	if s.activeTunnels > 0 {
		tunnelText := fmt.Sprintf("⚡%d tunnel", s.activeTunnels)
		if s.activeTunnels > 1 {
			tunnelText += "s"
		}
		middleParts = append(middleParts, tunnelStyle.Render(tunnelText))
	}

	middle := strings.Join(middleParts, separator)

	// Build right side: shortcuts
	right := keyStyle.Render("?help") + "  " + keyStyle.Render("q quit")

	// Calculate spacing
	leftWidth := lipgloss.Width(left)
	middleWidth := lipgloss.Width(middle)
	rightWidth := lipgloss.Width(right)

	// Add separators to left
	if middle != "" {
		left = left + separator
		leftWidth = lipgloss.Width(left)
	}

	// Calculate gap between middle and right
	totalUsed := leftWidth + middleWidth + rightWidth
	gap := s.width - totalUsed - 2 // -2 for padding
	if gap < 2 {
		gap = 2
	}

	// Build final string
	content := left + middle + strings.Repeat(" ", gap) + right

	return bgStyle.Padding(0, 1).Render(content)
}

package components

import (
	"github.com/charmbracelet/lipgloss"
	"vaws/internal/ui/theme"
)

// AWSRegions lists common AWS regions grouped by geography
var AWSRegions = []RegionGroup{
	{
		Name: "US",
		Regions: []Region{
			{Code: "us-east-1", Name: "N. Virginia"},
			{Code: "us-east-2", Name: "Ohio"},
			{Code: "us-west-1", Name: "N. California"},
			{Code: "us-west-2", Name: "Oregon"},
		},
	},
	{
		Name: "Europe",
		Regions: []Region{
			{Code: "eu-west-1", Name: "Ireland"},
			{Code: "eu-west-2", Name: "London"},
			{Code: "eu-west-3", Name: "Paris"},
			{Code: "eu-central-1", Name: "Frankfurt"},
			{Code: "eu-north-1", Name: "Stockholm"},
		},
	},
	{
		Name: "Asia Pacific",
		Regions: []Region{
			{Code: "ap-southeast-1", Name: "Singapore"},
			{Code: "ap-southeast-2", Name: "Sydney"},
			{Code: "ap-northeast-1", Name: "Tokyo"},
			{Code: "ap-northeast-2", Name: "Seoul"},
			{Code: "ap-south-1", Name: "Mumbai"},
		},
	},
	{
		Name: "Other",
		Regions: []Region{
			{Code: "sa-east-1", Name: "Sao Paulo"},
			{Code: "ca-central-1", Name: "Canada"},
			{Code: "me-south-1", Name: "Bahrain"},
			{Code: "af-south-1", Name: "Cape Town"},
		},
	},
}

// Region represents an AWS region
type Region struct {
	Code string // e.g., "us-east-1"
	Name string // e.g., "N. Virginia"
}

// RegionGroup groups regions by geography
type RegionGroup struct {
	Name    string
	Regions []Region
}

// RegionSelector allows selecting an AWS region
type RegionSelector struct {
	width         int
	height        int
	cursor        int
	offset        int
	currentRegion string
	flatRegions   []Region // Flattened list for navigation
}

// NewRegionSelector creates a new RegionSelector
func NewRegionSelector() *RegionSelector {
	rs := &RegionSelector{
		flatRegions: flattenRegions(),
	}
	return rs
}

// flattenRegions creates a flat list of all regions
func flattenRegions() []Region {
	var regions []Region
	for _, group := range AWSRegions {
		regions = append(regions, group.Regions...)
	}
	return regions
}

// SetSize sets the selector dimensions
func (r *RegionSelector) SetSize(width, height int) {
	r.width = width
	r.height = height
}

// SetCurrentRegion sets the currently active region
func (r *RegionSelector) SetCurrentRegion(region string) {
	r.currentRegion = region
	// Move cursor to current region
	for i, reg := range r.flatRegions {
		if reg.Code == region {
			r.cursor = i
			r.clampOffset()
			break
		}
	}
}

// Up moves cursor up
func (r *RegionSelector) Up() {
	if r.cursor > 0 {
		r.cursor--
		r.clampOffset()
	}
}

// Down moves cursor down
func (r *RegionSelector) Down() {
	if r.cursor < len(r.flatRegions)-1 {
		r.cursor++
		r.clampOffset()
	}
}

// SelectedRegion returns the currently selected region code
func (r *RegionSelector) SelectedRegion() string {
	if r.cursor >= 0 && r.cursor < len(r.flatRegions) {
		return r.flatRegions[r.cursor].Code
	}
	return ""
}

// visibleCount returns number of visible items
func (r *RegionSelector) visibleCount() int {
	return max(1, r.height-6)
}

// clampOffset ensures offset keeps cursor visible
func (r *RegionSelector) clampOffset() {
	visible := r.visibleCount()
	if r.cursor < r.offset {
		r.offset = r.cursor
	} else if r.cursor >= r.offset+visible {
		r.offset = r.cursor - visible + 1
	}
	maxOffset := max(0, len(r.flatRegions)-visible)
	r.offset = min(r.offset, maxOffset)
	r.offset = max(0, r.offset)
}

// View renders the region selector
func (r *RegionSelector) View() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(theme.TextDim)

	selectedStyle := lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(theme.Text)

	currentStyle := lipgloss.NewStyle().
		Foreground(theme.Success)

	codeStyle := lipgloss.NewStyle().
		Foreground(theme.TextMuted).
		Width(16)

	hintStyle := lipgloss.NewStyle().
		Foreground(theme.TextDim).
		Italic(true)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.BorderFocus).
		Padding(1, 2).
		Width(min(50, r.width-4))

	var content string
	content += titleStyle.Render("Select AWS Region") + "\n"
	content += subtitleStyle.Render("Current: "+r.currentRegion) + "\n\n"

	visible := r.visibleCount()
	end := min(r.offset+visible, len(r.flatRegions))

	for i := r.offset; i < end; i++ {
		region := r.flatRegions[i]
		isSelected := i == r.cursor
		isCurrent := region.Code == r.currentRegion

		var line string
		if isSelected {
			line += selectedStyle.Render("▸ ")
		} else {
			line += "  "
		}

		code := codeStyle.Render(region.Code)

		var name string
		if isCurrent {
			name = currentStyle.Render(region.Name + " (current)")
		} else if isSelected {
			name = selectedStyle.Render(region.Name)
		} else {
			name = normalStyle.Render(region.Name)
		}

		line += code + name
		content += line + "\n"
	}

	content += "\n" + hintStyle.Render("↑↓ navigate • Enter select • Esc cancel")

	return lipgloss.Place(
		r.width,
		r.height,
		lipgloss.Center,
		lipgloss.Center,
		boxStyle.Render(content),
	)
}

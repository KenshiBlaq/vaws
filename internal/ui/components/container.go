package components

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"vaws/internal/ui/theme"
)

// Container renders content inside a bordered box with title and context.
// The title appears at the top, context on the right.
//
// Example:
//
//	┌─ SQS Queues (12) ─────────────────────── us-east-1 ─┐
//	│                                                      │
//	│  (content here)                                      │
//	│                                                      │
//	└──────────────────────────────────────────────────────┘
type Container struct {
	title     string // e.g., "SQS Queues"
	context   string // e.g., "us-east-1"
	itemCount int    // Number of items (shown in title)
	width     int
	height    int
	content   string
	loading   bool
	err       error
	spinner   *Spinner
}

// NewContainer creates a new Container component.
func NewContainer() *Container {
	return &Container{
		spinner: NewSpinner(),
	}
}

// SetTitle sets the container title (shown in top-left of border).
func (c *Container) SetTitle(title string) {
	c.title = title
}

// SetContext sets the context string (shown in top-right of border).
func (c *Container) SetContext(context string) {
	c.context = context
}

// SetItemCount sets the item count displayed after the title.
func (c *Container) SetItemCount(count int) {
	c.itemCount = count
}

// SetSize sets the container dimensions.
func (c *Container) SetSize(width, height int) {
	c.width = width
	c.height = height
}

// SetContent sets the inner content to render.
func (c *Container) SetContent(content string) {
	c.content = content
}

// SetLoading sets the loading state.
func (c *Container) SetLoading(loading bool) {
	c.loading = loading
}

// SetError sets the error state.
func (c *Container) SetError(err error) {
	c.err = err
}

// Spinner returns the container's spinner for tick updates.
func (c *Container) Spinner() *Spinner {
	return c.spinner
}

// ContentWidth returns the available width for inner content.
func (c *Container) ContentWidth() int {
	// Account for border padding
	return c.width - 2
}

// ContentHeight returns the available height for inner content.
func (c *Container) ContentHeight() int {
	// Account for title line (1) + top border (1) + bottom border (1)
	return c.height - 3
}

// View renders the container.
func (c *Container) View() string {
	if c.width < 10 || c.height < 3 {
		return ""
	}

	contentWidth := c.ContentWidth()
	contentHeight := c.ContentHeight()

	// Build title line
	titleText := c.title
	if c.itemCount > 0 {
		titleText = fmt.Sprintf("%s (%d)", c.title, c.itemCount)
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true)

	contextStyle := lipgloss.NewStyle().
		Foreground(theme.Info)

	// Calculate title line with context on right
	titleRendered := titleStyle.Render(titleText)
	contextRendered := ""
	if c.context != "" {
		contextRendered = contextStyle.Render(c.context)
	}

	titleWidth := lipgloss.Width(titleRendered)
	contextWidth := lipgloss.Width(contextRendered)
	gap := c.width - titleWidth - contextWidth
	if gap < 1 {
		gap = 1
	}

	// Title line: "Lambda Functions (809)              us-east-1"
	titleLine := titleRendered + lipgloss.NewStyle().
		Width(gap).
		Render("") + contextRendered

	// Inner content
	var innerContent string
	if c.loading {
		// Centered loading message
		loadingStyle := lipgloss.NewStyle().Foreground(theme.Primary)
		loadingText := loadingStyle.Render(c.spinner.View() + " Loading...")
		innerContent = lipgloss.Place(contentWidth, contentHeight, lipgloss.Center, lipgloss.Center, loadingText)
	} else if c.err != nil {
		// Centered error message
		errorStyle := lipgloss.NewStyle().Foreground(theme.Error)
		errorText := errorStyle.Render("Error: " + c.err.Error())
		innerContent = lipgloss.Place(contentWidth, contentHeight, lipgloss.Center, lipgloss.Center, errorText)
	} else if c.content == "" {
		// Empty state
		emptyStyle := lipgloss.NewStyle().Foreground(theme.TextDim)
		emptyText := emptyStyle.Render("No items")
		innerContent = lipgloss.Place(contentWidth, contentHeight, lipgloss.Center, lipgloss.Center, emptyText)
	} else {
		innerContent = c.content
	}

	// Use lipgloss border for proper styling
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Border).
		Width(c.width - 2).
		Height(contentHeight)

	borderedContent := borderStyle.Render(innerContent)

	// Combine title line with bordered content
	return lipgloss.JoinVertical(lipgloss.Left, titleLine, borderedContent)
}

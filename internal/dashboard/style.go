package dashboard

import "github.com/charmbracelet/lipgloss"

var (
	colorPrimary = lipgloss.Color("#7C3AED")
	colorMuted   = lipgloss.Color("#6B7280")
	colorBright  = lipgloss.Color("#F9FAFB")

	providerColors = map[string]lipgloss.Color{
		"anthropic":  lipgloss.Color("#D97706"),
		"openai":     lipgloss.Color("#10B981"),
		"openrouter": lipgloss.Color("#8B5CF6"),
	}

	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				Padding(0, 2)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary)

	mutedStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	brightStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBright)

	selectedRowStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorPrimary)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#374151")).
			Padding(1, 2)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	sourceTagStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1E1E2E")).
			Background(lipgloss.Color("#10B981")) // green — subscription

	sourceTagKeyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#1E1E2E")).
				Background(lipgloss.Color("#D97706")) // orange — API key

	sourceTagCopilotStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#1E1E2E")).
				Background(lipgloss.Color("#8B5CF6")) // purple — GitHub Copilot

	// Heatmap intensity levels (0 = empty, 1-4 = increasing activity).
	heatmapColors = [5]lipgloss.Color{
		lipgloss.Color("#374151"), // level 0: no data (visible gray)
		lipgloss.Color("#4C1D95"), // level 1: low
		lipgloss.Color("#6D28D9"), // level 2: medium
		lipgloss.Color("#7C3AED"), // level 3: high (matches colorPrimary)
		lipgloss.Color("#A78BFA"), // level 4: max
	}
)

func providerColor(name string) lipgloss.Color {
	if c, ok := providerColors[name]; ok {
		return c
	}
	return lipgloss.Color("#06B6D4")
}

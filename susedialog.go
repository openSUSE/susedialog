package main

import (
	"errors"
	"fmt"
	"image/color"
	"os"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textinput"
	lipgloss "charm.land/lipgloss/v2"
)

type mode int

const (
	modeNone mode = iota
	modeMsgBox
	modeMenu
	modeChecklist
	modeForm
	modeProgress
)

// TickMsg is sent periodically to animate the UI
type TickMsg struct{}

type menuItem struct {
	Tag   string
	Label string
	On    bool
}

type formField struct {
	Label string
	Value string
}

type config struct {
	Title      string
	Backtitle  string
	Text       string
	Height     int
	Width      int
	ListHeight int
	Mode       mode
	Items      []menuItem
	Fields     []formField
	Percent    int
}

type palette struct {
	GeekoGreen    color.Color
	YarrowYellow  color.Color
	Orange        color.Color
	RadishRed     color.Color
	PlumPurple    color.Color
	ButterflyBlue color.Color
	TurquoiseTeal color.Color
	BagelBeige    color.Color
	GabbroGray    color.Color
	MapleMaroon   color.Color
}

// Colors from LCP's new openSUSE color scheme	
var opensuse = palette{
	GeekoGreen:    lipgloss.Color("#42cd42"),
	YarrowYellow:  lipgloss.Color("#d4cb1b"),
	Orange:        lipgloss.Color("#f68946"),
	RadishRed:     lipgloss.Color("#ff5b45"),
	PlumPurple:    lipgloss.Color("#a498ff"),
	ButterflyBlue: lipgloss.Color("#00c8ff"),
	TurquoiseTeal: lipgloss.Color("#20caa3"),
	BagelBeige:    lipgloss.Color("#fff8ee"),
	GabbroGray:    lipgloss.Color("#b8aeab"),
	MapleMaroon:   lipgloss.Color("#301a14"),
}

type model struct {
	cfg        config
	cursor     int
	quitting   bool
	cancelled  bool
	width      int
	height     int
	inputs     []textinput.Model
	focusIndex int
	tick       int
}

func newModel(cfg config) model {
	m := model{cfg: cfg}

	if cfg.Mode == modeForm {
		m.inputs = make([]textinput.Model, 0, len(cfg.Fields))
		for i, f := range cfg.Fields {
			ti := textinput.New()
			ti.SetValue(f.Value)
			ti.Placeholder = ""
			ti.CharLimit = 256
			ti.SetWidth(32)
			if i == 0 {
				ti.Focus()
			}
			m.inputs = append(m.inputs, ti)
		}
	}

	return m
}

func (m model) Init() tea.Cmd {
	return tea.Tick(time.Millisecond*150, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TickMsg:
		m.tick++
		return m, tea.Tick(time.Millisecond*150, func(t time.Time) tea.Msg {
			return TickMsg{}
		})

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		s := msg.String()

		switch m.cfg.Mode {
		case modeMsgBox:
			switch s {
			case "enter":
				m.quitting = true
				return m, tea.Quit
			case "esc", "q", "ctrl+c":
				m.cancelled = true
				m.quitting = true
				return m, tea.Quit
			}

		case modeMenu:
			switch s {
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.cfg.Items)-1 {
					m.cursor++
				}
			case "enter":
				m.quitting = true
				return m, tea.Quit
			case "esc", "q", "ctrl+c":
				m.cancelled = true
				m.quitting = true
				return m, tea.Quit
			}

		case modeChecklist:
			switch s {
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.cfg.Items)-1 {
					m.cursor++
				}
			case " ":
				if len(m.cfg.Items) > 0 {
					m.cfg.Items[m.cursor].On = !m.cfg.Items[m.cursor].On
				}
			case "enter":
				m.quitting = true
				return m, tea.Quit
			case "esc", "q", "ctrl+c":
				m.cancelled = true
				m.quitting = true
				return m, tea.Quit
			}

		case modeForm:
			switch s {
			case "tab", "shift+tab", "up", "down":
				if len(m.inputs) == 0 {
					return m, nil
				}

				m.inputs[m.focusIndex].Blur()

				if s == "shift+tab" || s == "up" {
					m.focusIndex--
				} else {
					m.focusIndex++
				}

				if m.focusIndex >= len(m.inputs) {
					m.focusIndex = 0
				}
				if m.focusIndex < 0 {
					m.focusIndex = len(m.inputs) - 1
				}

				m.inputs[m.focusIndex].Focus()
				return m, nil

			case "enter":
				m.quitting = true
				return m, tea.Quit

			case "esc", "q", "ctrl+c":
				m.cancelled = true
				m.quitting = true
				return m, tea.Quit
			}

			var cmds []tea.Cmd
			for i := range m.inputs {
				var cmd tea.Cmd
				m.inputs[i], cmd = m.inputs[i].Update(msg)
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)

		case modeProgress:
			switch s {
			case "esc", "q", "ctrl+c":
				m.cancelled = true
				m.quitting = true
				return m, tea.Quit
			}
		}
	}

	return m, nil
}

func (m model) View() tea.View {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(opensuse.GeekoGreen)

	backtitleStyle := lipgloss.NewStyle().
		Foreground(opensuse.GabbroGray)

	textStyle := lipgloss.NewStyle().
		Foreground(opensuse.BagelBeige)

	focusedStyle := lipgloss.NewStyle().
		Foreground(opensuse.GeekoGreen).
		Bold(true)

	selectedStyle := lipgloss.NewStyle().
		Foreground(opensuse.GeekoGreen).
		Bold(true)

	mutedStyle := lipgloss.NewStyle().
		Foreground(opensuse.GabbroGray)

	labelStyle := lipgloss.NewStyle().
		Foreground(opensuse.GeekoGreen)

	helpStyle := lipgloss.NewStyle().
		Foreground(opensuse.ButterflyBlue)

	progressPanelStyle := lipgloss.NewStyle().
		Background(opensuse.MapleMaroon).
		Padding(0, 1)

	warningStyle := lipgloss.NewStyle().
		Foreground(opensuse.Orange).
		Bold(true)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(opensuse.GeekoGreen).
		Padding(1, 2)

	selectedBulletStyle := lipgloss.NewStyle().
		Foreground(opensuse.PlumPurple).
		Bold(true)

	unselectedBulletStyle := lipgloss.NewStyle().
		Foreground(opensuse.PlumPurple)

	var b strings.Builder

	if m.cfg.Backtitle != "" {
		b.WriteString(backtitleStyle.Render(m.cfg.Backtitle))
		b.WriteString("\n")
	}

	if m.cfg.Title != "" {
		b.WriteString(titleStyle.Render(m.cfg.Title))
		b.WriteString("\n\n")
	}

	switch m.cfg.Mode {
	case modeMsgBox:
		b.WriteString(boxStyle.Render(textStyle.Render(m.cfg.Text)))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("Press Enter to continue · Esc to cancel"))

	case modeMenu:
		b.WriteString(textStyle.Render(m.cfg.Text))
		b.WriteString("\n\n")

		for i, it := range m.cfg.Items {
			prefix := "  "
			lineStyle := mutedStyle

			if i == m.cursor {
				// Pulse cursor visibility based on tick
				if m.tick%4 < 2 {
					prefix = "› "
				} else {
					prefix = "  "
				}
				lineStyle = focusedStyle
			}

			line := fmt.Sprintf("%s%s  %s", prefix, it.Tag, it.Label)
			b.WriteString(lineStyle.Render(line))
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(helpStyle.Render("↑/↓ move · Enter select · Esc cancel"))

	case modeChecklist:
		b.WriteString(textStyle.Render(m.cfg.Text))
		b.WriteString("\n\n")

		for i, it := range m.cfg.Items {
			cursor := "  "
			if i == m.cursor {
				// Pulse cursor visibility based on tick
				if m.tick%4 < 2 {
					cursor = "› "
				} else {
					cursor = "  "
				}
			}

			bullet := unselectedBulletStyle.Render("•")
			if it.On {
				bullet = selectedBulletStyle.Render("✓")
			}

			var line string
			if it.Label != "" {
				line = fmt.Sprintf("%s%s %s  %s", cursor, bullet, it.Tag, it.Label)
			} else {
				line = fmt.Sprintf("%s%s %s", cursor, bullet, it.Tag)
			}

			if i == m.cursor {
				b.WriteString(focusedStyle.Render(line))
			} else if it.On {
				b.WriteString(selectedStyle.Render(line))
			} else {
				b.WriteString(mutedStyle.Render(line))
			}
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(helpStyle.Render("↑/↓ move · Space toggle · Enter confirm · Esc cancel"))

	case modeForm:
		b.WriteString(textStyle.Render(m.cfg.Text))
		b.WriteString("\n\n")

		for i, f := range m.cfg.Fields {
			line := fmt.Sprintf("%s %s", labelStyle.Render(f.Label), m.inputs[i].View())
			b.WriteString(line)
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(helpStyle.Render("Tab moves focus · Enter confirm · Esc cancel"))

	case modeProgress:
		b.WriteString(focusedStyle.Render(m.cfg.Text))
		b.WriteString("\n\n")

		// Retro 8-bit bar: color follows palette position as progress moves.
		barWidth := 30
		progressPercent := m.cfg.Percent
		if progressPercent <= 0 || progressPercent > 100 {
			progressPercent = (m.tick * 3) % 101
		}
		headPos := (progressPercent * (barWidth - 1)) / 100

		// Full openSUSE palette from green to maroon.
		paletteColors := []color.Color{
			opensuse.GeekoGreen,
			opensuse.YarrowYellow,
			opensuse.Orange,
			opensuse.RadishRed,
			opensuse.PlumPurple,
			opensuse.ButterflyBlue,
			opensuse.TurquoiseTeal,
			opensuse.BagelBeige,
			opensuse.GabbroGray,
			opensuse.MapleMaroon,
		}

		var bar strings.Builder
		bar.WriteString("[")
		for i := 0; i < barWidth; i++ {
			colorIdx := ((i + m.tick/2) * len(paletteColors)) / barWidth
			colorIdx = colorIdx % len(paletteColors)
			if colorIdx >= len(paletteColors) {
				colorIdx = len(paletteColors) - 1
			}
			rainbowStyle := lipgloss.NewStyle().
				Foreground(paletteColors[colorIdx])

			if i == headPos {
				if m.tick%4 < 2 {
					bar.WriteString(rainbowStyle.Render("#"))
				} else {
					bar.WriteString(rainbowStyle.Render("@"))
				}
				continue
			}

			if i < headPos {
				distance := headPos - i
				switch {
				case distance <= 2:
					bar.WriteString(rainbowStyle.Render("#"))
				case distance <= 5:
					bar.WriteString(rainbowStyle.Render("="))
				case distance <= 9:
					bar.WriteString(rainbowStyle.Render("-"))
				default:
					bar.WriteString(rainbowStyle.Render("."))
				}
			} else {
				if (i+m.tick)%8 == 0 {
					bar.WriteString(helpStyle.Render("*"))
				} else {
					bar.WriteString(mutedStyle.Render("."))
				}
			}
		}
		bar.WriteString("]")

		spinner := []string{"|", "/", "-", "\\"}
		spin := spinner[m.tick%len(spinner)]

		line := fmt.Sprintf("%s %3d%%  %s", bar.String(), progressPercent, spin)
		b.WriteString(progressPanelStyle.Render(line))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("Esc cancels"))

	default:
		b.WriteString(warningStyle.Render("Unsupported mode"))
	}

	v := tea.NewView(b.String())
	return v
}

func parseArgs(args []string) (config, error) {
	cfg := config{}
	var i int

	for i < len(args) {
		switch args[i] {
		case "--clear":
			i++

		case "--title":
			if i+1 >= len(args) {
				return cfg, errors.New("missing value for --title")
			}
			cfg.Title = args[i+1]
			i += 2

		case "--backtitle":
			if i+1 >= len(args) {
				return cfg, errors.New("missing value for --backtitle")
			}
			cfg.Backtitle = args[i+1]
			i += 2

		case "--msgbox":
			if i+3 >= len(args) {
				return cfg, errors.New("invalid --msgbox arguments")
			}
			cfg.Mode = modeMsgBox
			cfg.Text = args[i+1]
			cfg.Height, _ = strconv.Atoi(args[i+2])
			cfg.Width, _ = strconv.Atoi(args[i+3])
			return cfg, nil

		case "--menu":
			if i+4 >= len(args) {
				return cfg, errors.New("invalid --menu arguments")
			}
			cfg.Mode = modeMenu
			cfg.Text = args[i+1]
			cfg.Height, _ = strconv.Atoi(args[i+2])
			cfg.Width, _ = strconv.Atoi(args[i+3])
			cfg.ListHeight, _ = strconv.Atoi(args[i+4])

			for j := i + 5; j+1 < len(args); j += 2 {
				if strings.HasPrefix(args[j], "--") {
					break
				}
				cfg.Items = append(cfg.Items, menuItem{
					Tag:   args[j],
					Label: args[j+1],
				})
			}
			return cfg, nil

		case "--checklist":
			if i+4 >= len(args) {
				return cfg, errors.New("invalid --checklist arguments")
			}
			cfg.Mode = modeChecklist
			cfg.Text = args[i+1]
			cfg.Height, _ = strconv.Atoi(args[i+2])
			cfg.Width, _ = strconv.Atoi(args[i+3])
			cfg.ListHeight, _ = strconv.Atoi(args[i+4])

			for j := i + 5; j+2 < len(args); j += 3 {
				if strings.HasPrefix(args[j], "--") {
					break
				}
				cfg.Items = append(cfg.Items, menuItem{
					Tag:   strings.Trim(args[j], `"`),
					Label: args[j+1],
					On:    strings.EqualFold(args[j+2], "on"),
				})
			}
			return cfg, nil

		case "--form":
			if i+4 >= len(args) {
				return cfg, errors.New("invalid --form arguments")
			}
			cfg.Mode = modeForm
			cfg.Text = args[i+1]
			cfg.Height, _ = strconv.Atoi(args[i+2])
			cfg.Width, _ = strconv.Atoi(args[i+3])
			cfg.ListHeight, _ = strconv.Atoi(args[i+4])

			for j := i + 5; j+7 < len(args); j += 8 {
				if strings.HasPrefix(args[j], "--") {
					break
				}
				cfg.Fields = append(cfg.Fields, formField{
					Label: args[j],
					Value: args[j+2],
				})
			}
			return cfg, nil

		case "--progress":
			if i+3 >= len(args) {
				return cfg, errors.New("invalid --progress arguments")
			}
			cfg.Mode = modeProgress
			cfg.Text = args[i+1]
			cfg.Height, _ = strconv.Atoi(args[i+2])
			cfg.Width, _ = strconv.Atoi(args[i+3])
			if i+4 < len(args) && !strings.HasPrefix(args[i+4], "--") {
				cfg.Percent, _ = strconv.Atoi(args[i+4])
				i += 5
			} else {
				i += 4
			}
			return cfg, nil

		default:
			return cfg, fmt.Errorf("unsupported argument: %s", args[i])
		}
	}

	return cfg, errors.New("no widget specified")
}

func emitResult(m model) int {
	if m.cancelled {
		return 1
	}

	switch m.cfg.Mode {
	case modeMsgBox:
		return 0

	case modeMenu:
		if len(m.cfg.Items) == 0 {
			return 1
		}
		_, _ = fmt.Fprintln(os.Stderr, m.cfg.Items[m.cursor].Tag)
		return 0

	case modeChecklist:
		var selected []string
		for _, it := range m.cfg.Items {
			if it.On {
				selected = append(selected, fmt.Sprintf("\"%s\"", it.Tag))
			}
		}
		_, _ = fmt.Fprintln(os.Stderr, strings.Join(selected, " "))
		return 0

	case modeForm:
		for _, in := range m.inputs {
			_, _ = fmt.Fprintln(os.Stderr, in.Value())
		}
		return 0

	case modeProgress:
		return 0

	default:
		return 1
	}
}

func main() {
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	p := tea.NewProgram(newModel(cfg))
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	exitCode := emitResult(finalModel.(model))
	os.Exit(exitCode)
}
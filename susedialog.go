package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

type mode int

// gitCommit is intended to be injected at build time via -ldflags.
var gitCommit = "dev"

const (
	modeNone mode = iota
	modeMsgBox
	modeInfoBox
	modeTextBox
	modeYesNo
	modeMenu
	modeChecklist
	modeForm
	modeProgress
	modeInputBox
	modePasswordBox
)

// TickMsg is sent periodically to animate the UI
type TickMsg struct{}

type menuItem struct {
	Tag   string
	Label string
	On    bool
}

type formField struct {
	Label     string
	Value     string
	FieldType int
}

type config struct {
	Title       string
	Backtitle   string
	OkLabel     string
	CancelLabel string
	ExitLabel   string
	OutputFD    int
	Clear       bool
	DefaultItem string
	NoNLExpand  bool
	NoCollapse  bool
	Insecure    bool
	Text        string
	Height      int
	Width       int
	ListHeight  int
	Mode        mode
	Items       []menuItem
	Fields      []formField
	Percent     int
	Theme       string
	Align       string
	ThemeToggleKey string
	Palette     palette
	Themes      map[string]palette
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

var highContrast = palette{
	GeekoGreen:    lipgloss.Color("#00aa00"),
	YarrowYellow:  lipgloss.Color("#ffff55"),
	Orange:        lipgloss.Color("#ffaa00"),
	RadishRed:     lipgloss.Color("#ff5555"),
	PlumPurple:    lipgloss.Color("#ff55ff"),
	ButterflyBlue: lipgloss.Color("#5555ff"),
	TurquoiseTeal: lipgloss.Color("#55ffff"),
	BagelBeige:    lipgloss.Color("#ffffff"),
	GabbroGray:    lipgloss.Color("#aaaaaa"),
	MapleMaroon:   lipgloss.Color("#000000"),
}

var rainbow = palette{
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

//go:embed themes/*.json
var themeFS embed.FS

type themeFile struct {
	Name    string           `json:"name"`
	Palette themeFilePalette `json:"palette"`
}

type themeFilePalette struct {
	GeekoGreen    string `json:"GeekoGreen"`
	YarrowYellow  string `json:"YarrowYellow"`
	Orange        string `json:"Orange"`
	RadishRed     string `json:"RadishRed"`
	PlumPurple    string `json:"PlumPurple"`
	ButterflyBlue string `json:"ButterflyBlue"`
	TurquoiseTeal string `json:"TurquoiseTeal"`
	BagelBeige    string `json:"BagelBeige"`
	GabbroGray    string `json:"GabbroGray"`
	MapleMaroon   string `json:"MapleMaroon"`
}

type runtimeConfig struct {
	Theme          string
	Align          string
	ThemeToggleKey string
}

type model struct {
	cfg                  config
	cursor               int
	choice               int
	quitting             bool
	cancelled            bool
	debugKeys            bool
	width                int
	height               int
	inputs               []textinput.Model
	focusIndex           int
	tick                 int
	textboxButtonFocused bool
}

func envEnabled(name string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func newModel(cfg config) model {
	if strings.TrimSpace(cfg.Align) == "" {
		cfg.Align = "topleft"
	}
	if strings.TrimSpace(cfg.ThemeToggleKey) == "" {
		cfg.ThemeToggleKey = "ctrl+t"
	}
	m := model{cfg: cfg, debugKeys: envEnabled("SUSEDIALOG_DEBUG_KEYS")}

	if cfg.DefaultItem != "" && (cfg.Mode == modeMenu || cfg.Mode == modeChecklist) {
		for i, it := range cfg.Items {
			if it.Tag == cfg.DefaultItem {
				m.cursor = i
				break
			}
		}
	}

	if cfg.Mode == modeForm {
		m.inputs = make([]textinput.Model, 0, len(cfg.Fields))
		for i, f := range cfg.Fields {
			if f.FieldType == 2 {
				continue
			}
			ti := textinput.New()
			ti.SetValue(f.Value)
			ti.Placeholder = ""
			ti.Prompt = "> "
			ti.CharLimit = 256
			ti.SetWidth(32)
			if f.FieldType == 1 {
				ti.EchoMode = textinput.EchoPassword
				ti.EchoCharacter = '*'
			}
			if len(m.inputs) == 0 || i == 0 {
				ti.Focus()
			}
			m.inputs = append(m.inputs, ti)
		}
	}

	if cfg.Mode == modeInputBox || cfg.Mode == modePasswordBox {
		m.inputs = make([]textinput.Model, 1)
		ti := textinput.New()
		ti.Placeholder = ""
		ti.Prompt = "> "
		ti.CharLimit = 256
		ti.SetWidth(40)
		if cfg.Mode == modePasswordBox {
			ti.EchoMode = textinput.EchoPassword
			ti.EchoCharacter = '*'
		}
		ti.Focus()
		m.inputs[0] = ti
	}

	return m
}

func renderWithBoldMarkers(text string, normalStyle, boldAccentStyle lipgloss.Style) string {
	if !strings.Contains(text, "**") {
		return normalStyle.Render(text)
	}

	parts := strings.Split(text, "**")
	var b strings.Builder
	for i, p := range parts {
		if i%2 == 1 {
			b.WriteString(boldAccentStyle.Render(p))
		} else {
			b.WriteString(normalStyle.Render(p))
		}
	}

	return b.String()
}

func normalizeDialogText(text string, noNLExpand bool) string {
	if noNLExpand {
		return text
	}
	return strings.ReplaceAll(text, `\n`, "\n")
}

func wrapLine(line string, width int) string {
	if width <= 1 {
		return line
	}
	if len([]rune(line)) <= width {
		return line
	}

	words := strings.Fields(line)
	if len(words) == 0 {
		return ""
	}

	var out []string
	current := words[0]

	for _, w := range words[1:] {
		if len([]rune(current))+1+len([]rune(w)) <= width {
			current += " " + w
			continue
		}

		out = append(out, current)
		for len([]rune(w)) > width {
			r := []rune(w)
			out = append(out, string(r[:width]))
			w = string(r[width:])
		}
		current = w
	}

	out = append(out, current)
	return strings.Join(out, "\n")
}

func wrapText(text string, width int) string {
	if width <= 1 {
		return text
	}

	lines := strings.Split(text, "\n")
	wrapped := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			wrapped = append(wrapped, "")
			continue
		}
		wrapped = append(wrapped, wrapLine(line, width))
	}

	return strings.Join(wrapped, "\n")
}

func paletteColors(p palette) []color.Color {
	return []color.Color{
		p.GeekoGreen,
		p.YarrowYellow,
		p.Orange,
		p.RadishRed,
		p.PlumPurple,
		p.ButterflyBlue,
		p.TurquoiseTeal,
		p.BagelBeige,
		p.GabbroGray,
		p.MapleMaroon,
	}
}

func renderRainbowUnderline(length int, p palette) string {
	if length < 12 {
		length = 12
	}

	colors := paletteColors(p)

	var b strings.Builder
	for i := 0; i < length; i++ {
		idx := (i * len(colors)) / length
		if idx >= len(colors) {
			idx = len(colors) - 1
		}
		b.WriteString(lipgloss.NewStyle().Foreground(colors[idx]).Bold(true).Render("━"))
	}

	return b.String()
}

func padRightRunes(s string, target int) string {
	current := len([]rune(s))
	if current >= target {
		return s
	}
	return s + strings.Repeat(" ", target-current)
}

func renderRainbowFrame(content string, p palette) string {
	colors := paletteColors(p)
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}

	maxWidth := 0
	for _, line := range lines {
		if w := len([]rune(line)); w > maxWidth {
			maxWidth = w
		}
	}
	if maxWidth < 4 {
		maxWidth = 4
	}

	horizontal := maxWidth + 2

	renderChar := func(ch string, c color.Color) string {
		return lipgloss.NewStyle().Foreground(c).Bold(true).Render(ch)
	}

	var out strings.Builder
	out.WriteString(renderChar("╭", colors[0]))
	for i := 0; i < horizontal; i++ {
		idx := (i * len(colors)) / horizontal
		if idx >= len(colors) {
			idx = len(colors) - 1
		}
		out.WriteString(renderChar("─", colors[idx]))
	}
	out.WriteString(renderChar("╮", colors[len(colors)-1]))
	out.WriteString("\n")

	for i, line := range lines {
		left := colors[(i*2)%len(colors)]
		right := colors[(i*2+1)%len(colors)]
		out.WriteString(renderChar("│", left))
		out.WriteString(" ")
		out.WriteString(padRightRunes(line, maxWidth))
		out.WriteString(" ")
		out.WriteString(renderChar("│", right))
		if i < len(lines)-1 {
			out.WriteString("\n")
		}
	}
	out.WriteString("\n")

	out.WriteString(renderChar("╰", colors[0]))
	for i := 0; i < horizontal; i++ {
		idx := ((horizontal - 1 - i) * len(colors)) / horizontal
		if idx >= len(colors) {
			idx = len(colors) - 1
		}
		out.WriteString(renderChar("─", colors[idx]))
	}
	out.WriteString(renderChar("╯", colors[len(colors)-1]))

	return out.String()
}

func colorFromHex(v string) (color.Color, error) {
	v = strings.TrimSpace(v)
	if len(v) != 7 || !strings.HasPrefix(v, "#") {
		return nil, fmt.Errorf("invalid color %q", v)
	}
	return lipgloss.Color(v), nil
}

func paletteFromThemeFile(p themeFilePalette) (palette, error) {
	var out palette
	var err error

	out.GeekoGreen, err = colorFromHex(p.GeekoGreen)
	if err != nil {
		return out, err
	}
	out.YarrowYellow, err = colorFromHex(p.YarrowYellow)
	if err != nil {
		return out, err
	}
	out.Orange, err = colorFromHex(p.Orange)
	if err != nil {
		return out, err
	}
	out.RadishRed, err = colorFromHex(p.RadishRed)
	if err != nil {
		return out, err
	}
	out.PlumPurple, err = colorFromHex(p.PlumPurple)
	if err != nil {
		return out, err
	}
	out.ButterflyBlue, err = colorFromHex(p.ButterflyBlue)
	if err != nil {
		return out, err
	}
	out.TurquoiseTeal, err = colorFromHex(p.TurquoiseTeal)
	if err != nil {
		return out, err
	}
	out.BagelBeige, err = colorFromHex(p.BagelBeige)
	if err != nil {
		return out, err
	}
	out.GabbroGray, err = colorFromHex(p.GabbroGray)
	if err != nil {
		return out, err
	}
	out.MapleMaroon, err = colorFromHex(p.MapleMaroon)
	if err != nil {
		return out, err
	}

	return out, nil
}

func parseThemeDefinition(data []byte, fallbackName string) (string, palette, error) {
	var tf themeFile
	if err := json.Unmarshal(data, &tf); err != nil {
		return "", palette{}, err
	}

	name := strings.TrimSpace(strings.ToLower(tf.Name))
	if name == "" {
		name = strings.TrimSuffix(strings.ToLower(fallbackName), ".json")
	}

	p, err := paletteFromThemeFile(tf.Palette)
	if err != nil {
		return "", palette{}, err
	}

	return name, p, nil
}

func loadThemesFromDir(dir string) map[string]palette {
	themes := map[string]palette{}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return themes
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		data, readErr := os.ReadFile(filepath.Join(dir, e.Name()))
		if readErr != nil {
			continue
		}

		name, p, parseErr := parseThemeDefinition(data, e.Name())
		if parseErr != nil {
			continue
		}

		themes[name] = p
	}

	return themes
}

func executableDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}

	resolved, err := filepath.EvalSymlinks(exePath)
	if err == nil {
		exePath = resolved
	}

	return filepath.Dir(exePath)
}

func loadBundledThemes() map[string]palette {
	themes := map[string]palette{
		"opensuse":      opensuse,
		"high-contrast": highContrast,
		"rainbow":       rainbow,
	}

	entries, err := themeFS.ReadDir("themes")
	if err != nil {
		return themes
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		data, readErr := themeFS.ReadFile(filepath.Join("themes", e.Name()))
		if readErr != nil {
			continue
		}

		name, p, parseErr := parseThemeDefinition(data, e.Name())
		if parseErr != nil {
			continue
		}

		themes[name] = p
	}

	if exeDir := executableDir(); exeDir != "" {
		for name, p := range loadThemesFromDir(filepath.Join(exeDir, "themes")) {
			themes[name] = p
		}
	}

	return themes
}

func cleanConfigValue(v string) string {
	v = strings.TrimSpace(v)
	v = strings.Trim(v, `"'`)
	return strings.TrimSpace(v)
}

func parseRuntimeConfig(path string) runtimeConfig {
	rc := runtimeConfig{}
	data, err := os.ReadFile(path)
	if err != nil {
		return rc
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		line = strings.TrimPrefix(line, "export ")
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := cleanConfigValue(parts[1])

		switch key {
		case "theme", "susedialog_theme":
			rc.Theme = strings.ToLower(val)
		case "align", "susedialog_align":
			rc.Align = normalizeAlignment(val)
		case "theme_toggle_key", "susedialog_theme_toggle_key":
			rc.ThemeToggleKey = normalizeKeyBinding(val)
		}
	}

	return rc
}

func normalizeAlignment(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.ReplaceAll(v, "_", "")
	v = strings.ReplaceAll(v, "-", "")
	v = strings.ReplaceAll(v, " ", "")

	switch v {
	case "", "topleft", "left", "top", "start":
		return "topleft"
	case "center", "centred", "middle":
		return "center"
	default:
		return "topleft"
	}
}

func normalizeKeyBinding(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.ReplaceAll(v, " ", "")
	return v
}

func userConfigPath() string {
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, "susedialog", "config")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "susedialog", "config")
}

func resolveThemeName(cliTheme string) string {
	theme := strings.ToLower(strings.TrimSpace(cliTheme))
	if theme != "" {
		return theme
	}

	if envTheme := strings.ToLower(strings.TrimSpace(os.Getenv("SUSEDIALOG_THEME"))); envTheme != "" {
		return envTheme
	}

	if userCfg := userConfigPath(); userCfg != "" {
		if cfg := parseRuntimeConfig(userCfg); cfg.Theme != "" {
			return cfg.Theme
		}
	}

	if cfg := parseRuntimeConfig("/etc/susedialog/config"); cfg.Theme != "" {
		return cfg.Theme
	}

	return "opensuse"
}

func resolveAlignment(cliAlign string) string {
	if v := normalizeAlignment(cliAlign); v != "topleft" || strings.TrimSpace(cliAlign) == "" {
		if strings.TrimSpace(cliAlign) != "" {
			return v
		}
	}

	if envAlign := normalizeAlignment(os.Getenv("SUSEDIALOG_ALIGN")); envAlign != "topleft" || strings.TrimSpace(os.Getenv("SUSEDIALOG_ALIGN")) == "topleft" {
		if strings.TrimSpace(os.Getenv("SUSEDIALOG_ALIGN")) != "" {
			return envAlign
		}
	}

	if userCfg := userConfigPath(); userCfg != "" {
		if cfg := parseRuntimeConfig(userCfg); cfg.Align != "" {
			return normalizeAlignment(cfg.Align)
		}
	}

	if cfg := parseRuntimeConfig("/etc/susedialog/config"); cfg.Align != "" {
		return normalizeAlignment(cfg.Align)
	}

	return "topleft"
}

func resolveThemeToggleKey() string {
	if envKey := normalizeKeyBinding(os.Getenv("SUSEDIALOG_THEME_TOGGLE_KEY")); envKey != "" {
		return envKey
	}

	if userCfg := userConfigPath(); userCfg != "" {
		if cfg := parseRuntimeConfig(userCfg); cfg.ThemeToggleKey != "" {
			return cfg.ThemeToggleKey
		}
	}

	if cfg := parseRuntimeConfig("/etc/susedialog/config"); cfg.ThemeToggleKey != "" {
		return cfg.ThemeToggleKey
	}

	return "ctrl+t"
}

func nextThemeName(current string) string {
	if strings.EqualFold(strings.TrimSpace(current), "high-contrast") {
		return "opensuse"
	}
	return "high-contrast"
}

func resolvePalette(theme string, themes map[string]palette) (string, palette) {
	name := strings.ToLower(strings.TrimSpace(theme))
	if p, ok := themes[name]; ok {
		return name, p
	}
	return "opensuse", opensuse
}

func placeAligned(out, align string, termWidth, termHeight int) string {
	if normalizeAlignment(align) != "center" {
		return out
	}
	if termWidth <= 0 || termHeight <= 0 {
		return out
	}
	return lipgloss.Place(termWidth, termHeight, lipgloss.Center, lipgloss.Center, out)
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func (m model) listVisibleCount(total int) int {
	if total <= 0 {
		return 0
	}

	visible := total
	if m.cfg.ListHeight > 0 {
		visible = clampInt(m.cfg.ListHeight, 1, total)
	}

	if m.height > 0 {
		reserved := 5
		if m.cfg.Backtitle != "" {
			reserved++
		}
		if m.cfg.Title != "" {
			reserved += 3
		}

		textLines := 1
		if m.cfg.Text != "" {
			textLines = strings.Count(m.cfg.Text, "\n") + 1
		}
		reserved += textLines + 2

		maxByTerm := m.height - reserved
		if maxByTerm > 0 {
			if maxByTerm < visible {
				visible = maxByTerm
			}
		}
	}

	if visible < 1 {
		visible = 1
	}

	return clampInt(visible, 1, total)
}

func listWindow(total, visible, cursor int) (int, int) {
	if total <= 0 || visible <= 0 {
		return 0, 0
	}
	if visible >= total {
		return 0, total
	}

	cursor = clampInt(cursor, 0, total-1)
	start := cursor - visible/2
	start = clampInt(start, 0, total-visible)
	end := start + visible
	return start, end
}

func (m model) Init() tea.Cmd {
	tickCmd := tea.Tick(time.Millisecond*150, func(t time.Time) tea.Msg {
		return TickMsg{}
	})

	if m.cfg.Mode == modeForm || m.cfg.Mode == modeInputBox || m.cfg.Mode == modePasswordBox {
		return tea.Batch(tickCmd, textinput.Blink)
	}

	return tickCmd
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
		s := normalizeKeyBinding(msg.String())
		if m.debugKeys {
			_, _ = fmt.Fprintf(os.Stderr, "[susedialog debug] key=%q\n", s)
		}

		toggleKey := normalizeKeyBinding(m.cfg.ThemeToggleKey)
		if toggleKey == "" {
			toggleKey = "ctrl+t"
		}

		if s == toggleKey {
			nextTheme := nextThemeName(m.cfg.Theme)
			m.cfg.Theme, m.cfg.Palette = resolvePalette(nextTheme, m.cfg.Themes)
			return m, nil
		}

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

		case modeInputBox, modePasswordBox:
			switch s {
			case "enter":
				m.quitting = true
				return m, tea.Quit
			case "esc", "q", "ctrl+c":
				m.cancelled = true
				m.quitting = true
				return m, tea.Quit
			default:
				// Delegate to textinput widget
				if len(m.inputs) > 0 {
					m.inputs[0], _ = m.inputs[0].Update(msg)
				}
			}

		case modeTextBox:
			lines := strings.Split(m.cfg.Text, "\n")
			maxOffset := len(lines) - 1
			if maxOffset < 0 {
				maxOffset = 0
			}

			switch s {
			case "up", "k":
				if m.textboxButtonFocused {
					// From button, go back to end of content
					m.textboxButtonFocused = false
					m.cursor = maxOffset
				} else if m.cursor > 0 {
					// Scroll up in content
					m.cursor--
				}
			case "down", "j":
				if m.textboxButtonFocused {
					// Already at button, stay there
				} else if m.cursor < maxOffset {
					// Scroll down in content
					m.cursor++
				} else {
					// At bottom of content, move to button
					m.textboxButtonFocused = true
				}
			case "pgup", "b":
				if !m.textboxButtonFocused {
					m.cursor = clampInt(m.cursor-10, 0, maxOffset)
				}
			case "pgdown", "f", " ", "space":
				if !m.textboxButtonFocused {
					newCursor := clampInt(m.cursor+10, 0, maxOffset)
					if newCursor == maxOffset && m.cursor == maxOffset {
						// Already at bottom, move to button
						m.textboxButtonFocused = true
					} else {
						m.cursor = newCursor
					}
				}
			case "home", "g":
				if !m.textboxButtonFocused {
					m.cursor = 0
				}
			case "end", "G":
				if !m.textboxButtonFocused {
					m.cursor = maxOffset
					m.textboxButtonFocused = true
				}
			case "tab":
				// Tab still works as explicit toggle
				m.textboxButtonFocused = !m.textboxButtonFocused
			case "enter":
				// Enter always activates button
				m.quitting = true
				return m, tea.Quit
			case "esc", "q", "ctrl+c":
				m.quitting = true
				m.cancelled = true
				return m, tea.Quit
			}

		case modeYesNo:
			switch s {
			case "left", "h":
				m.choice = 0
			case "right", "l", "tab", "shift+tab":
				m.choice = 1 - m.choice
			case "y":
				m.choice = 0
				m.quitting = true
				return m, tea.Quit
			case "n":
				m.choice = 1
				m.quitting = true
				return m, tea.Quit
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
			case " ", "space":
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
	p := m.cfg.Palette
	if p.GeekoGreen == nil {
		p = opensuse
	}
	highContrastTheme := strings.EqualFold(m.cfg.Theme, "high-contrast")
	rainbowTheme := strings.EqualFold(m.cfg.Theme, "rainbow")

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(p.ButterflyBlue)

	titleAccentStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(p.PlumPurple)

	backtitleStyle := lipgloss.NewStyle().
		Foreground(p.GabbroGray)

	textStyle := lipgloss.NewStyle().
		Foreground(p.BagelBeige)

	boldTextStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(p.ButterflyBlue)

	focusedStyle := lipgloss.NewStyle().
		Foreground(p.GeekoGreen).
		Bold(true)

	selectedStyle := lipgloss.NewStyle().
		Foreground(p.GeekoGreen).
		Bold(true)

	mutedStyle := lipgloss.NewStyle().
		Foreground(p.GabbroGray)

	labelStyle := lipgloss.NewStyle().
		Foreground(p.GeekoGreen)

	helpStyle := lipgloss.NewStyle().
		Foreground(p.ButterflyBlue)

	inputFieldStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(p.GabbroGray).
		Padding(0, 1)

	focusedInputFieldStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(p.GeekoGreen).
		Padding(0, 1)

	progressPanelStyle := lipgloss.NewStyle().
		Background(p.MapleMaroon).
		Padding(0, 1)

	warningStyle := lipgloss.NewStyle().
		Foreground(p.Orange).
		Bold(true)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.GeekoGreen).
		Padding(1, 2)

	inactiveBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.GabbroGray).
		Padding(1, 2)

	selectedBulletStyle := lipgloss.NewStyle().
		Foreground(p.PlumPurple).
		Bold(true)

	unselectedBulletStyle := lipgloss.NewStyle().
		Foreground(p.PlumPurple)

	if highContrastTheme {
		backtitleStyle = backtitleStyle.Foreground(p.BagelBeige).Background(p.MapleMaroon)
		textStyle = textStyle.Foreground(p.BagelBeige).Background(p.MapleMaroon)
		boldTextStyle = boldTextStyle.Foreground(p.YarrowYellow).Background(p.MapleMaroon)
		titleStyle = titleStyle.Foreground(p.TurquoiseTeal).Background(p.MapleMaroon)
		titleAccentStyle = titleAccentStyle.Foreground(p.BagelBeige).Background(p.MapleMaroon)
		focusedStyle = focusedStyle.Foreground(p.MapleMaroon).Background(p.YarrowYellow)
		selectedStyle = selectedStyle.Foreground(p.MapleMaroon).Background(p.ButterflyBlue)
		mutedStyle = mutedStyle.Foreground(p.GabbroGray).Background(p.MapleMaroon)
		labelStyle = labelStyle.Foreground(p.BagelBeige).Background(p.MapleMaroon)
		helpStyle = helpStyle.Foreground(p.YarrowYellow).Background(p.MapleMaroon)
		inputFieldStyle = inputFieldStyle.Foreground(p.BagelBeige).Background(p.MapleMaroon).BorderForeground(p.BagelBeige)
		focusedInputFieldStyle = focusedInputFieldStyle.Foreground(p.MapleMaroon).Background(p.YarrowYellow).BorderForeground(p.YarrowYellow)
		progressPanelStyle = progressPanelStyle.Foreground(p.BagelBeige).Background(p.MapleMaroon)
		warningStyle = warningStyle.Foreground(p.RadishRed).Background(p.MapleMaroon)
		boxStyle = boxStyle.Foreground(p.BagelBeige).Background(p.MapleMaroon).Border(lipgloss.ThickBorder()).BorderForeground(p.BagelBeige)
		inactiveBoxStyle = inactiveBoxStyle.Foreground(p.GabbroGray).Background(p.MapleMaroon).Border(lipgloss.ThickBorder()).BorderForeground(p.GabbroGray)
		selectedBulletStyle = selectedBulletStyle.Foreground(p.TurquoiseTeal).Background(p.MapleMaroon)
		unselectedBulletStyle = unselectedBulletStyle.Foreground(p.BagelBeige).Background(p.MapleMaroon)
	}

	var b strings.Builder

	if m.cfg.Backtitle != "" {
		b.WriteString(backtitleStyle.Render(m.cfg.Backtitle))
		b.WriteString("\n")
	}

	if m.cfg.Title != "" {
		title := normalizeDialogText(m.cfg.Title, m.cfg.NoNLExpand)
		b.WriteString(renderWithBoldMarkers(title, titleStyle, titleAccentStyle))
		b.WriteString("\n")
		if rainbowTheme {
			b.WriteString(renderRainbowUnderline(40, p))
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(p.PlumPurple).Bold(true).Render(strings.Repeat("━", 40)))
		}
		b.WriteString("\n\n")
	}

	displayText := normalizeDialogText(m.cfg.Text, m.cfg.NoNLExpand)

	switch m.cfg.Mode {
	case modeMsgBox:
		msgBody := renderWithBoldMarkers(displayText, textStyle, boldTextStyle)
		if rainbowTheme {
			b.WriteString(renderRainbowFrame(msgBody, p))
		} else {
			b.WriteString(boxStyle.Render(msgBody))
		}
		b.WriteString("\n\n")
		b.WriteString(focusedStyle.Render(fmt.Sprintf("[ %s ]", m.cfg.OkLabel)))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("Enter confirms · Esc cancels"))

	case modeInputBox:
		b.WriteString(renderWithBoldMarkers(displayText, textStyle, boldTextStyle))
		b.WriteString("\n\n")
		if len(m.inputs) > 0 {
			b.WriteString(focusedInputFieldStyle.Render(m.inputs[0].View()))
		}
		b.WriteString("\n\n")
		okLabel := m.cfg.OkLabel
		if okLabel == "" {
			okLabel = "OK"
		}
		b.WriteString(focusedStyle.Render(fmt.Sprintf("[ %s ]", okLabel)))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("Enter confirms · Esc cancels"))

	case modePasswordBox:
		b.WriteString(renderWithBoldMarkers(displayText, textStyle, boldTextStyle))
		b.WriteString("\n\n")
		if len(m.inputs) > 0 {
			b.WriteString(focusedInputFieldStyle.Render(m.inputs[0].View()))
		}
		b.WriteString("\n\n")
		okLabel := m.cfg.OkLabel
		if okLabel == "" {
			okLabel = "OK"
		}
		b.WriteString(focusedStyle.Render(fmt.Sprintf("[ %s ]", okLabel)))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("Enter confirms · Esc cancels"))

	case modeTextBox:
		lines := strings.Split(displayText, "\n")
		total := len(lines)
		if total == 0 {
			lines = []string{""}
			total = 1
		}

		visible := total
		if m.cfg.Height > 0 {
			// Reserve space: 6 for margins + 3 for button and help text
			visible = clampInt(m.cfg.Height-9, 1, total)
		}
		if m.height > 0 {
			// Reserve space: 10 for margins + 3 for button and help text
			maxByTerm := m.height - 13
			if maxByTerm > 0 && maxByTerm < visible {
				visible = maxByTerm
			}
		}

		maxStart := total - visible
		if maxStart < 0 {
			maxStart = 0
		}
		start := clampInt(m.cursor, 0, maxStart)
		end := start + visible
		if end > total {
			end = total
		}

		body := strings.Join(lines[start:end], "\n")

		contentWidth := 60
		if m.cfg.Width > 0 {
			contentWidth = m.cfg.Width - 6
		}
		if m.width > 0 {
			maxByTerm := m.width - 10
			if maxByTerm > 0 && maxByTerm < contentWidth {
				contentWidth = maxByTerm
			}
		}
		contentWidth = clampInt(contentWidth, 20, 200)
		body = wrapText(body, contentWidth)

		// Use gray border if button is focused, green if scrolling
		currentBoxStyle := boxStyle
		if m.textboxButtonFocused {
			currentBoxStyle = inactiveBoxStyle
		}

		b.WriteString(currentBoxStyle.Width(contentWidth).Render(textStyle.Render(body)))
		b.WriteString("\n\n")

		// Show button with exit label
		exitLabel := m.cfg.ExitLabel
		if exitLabel == "" {
			exitLabel = "OK"
		}

		buttonStyle := mutedStyle
		if m.textboxButtonFocused {
			buttonStyle = focusedStyle
		}
		b.WriteString(buttonStyle.Render(fmt.Sprintf("[ %s ]", exitLabel)))
		b.WriteString("\n")

		// Help text shows focus context
		if m.textboxButtonFocused {
			b.WriteString(helpStyle.Render("↑ back · Enter confirm · Esc cancel"))
		} else {
			b.WriteString(helpStyle.Render("↑/↓ scroll · End/↓ at bottom → button · Esc cancel"))
		}

	case modeYesNo:
		ynBody := renderWithBoldMarkers(displayText, textStyle, boldTextStyle)
		if rainbowTheme {
			b.WriteString(renderRainbowFrame(ynBody, p))
		} else {
			b.WriteString(boxStyle.Render(ynBody))
		}
		b.WriteString("\n\n")

		yesStyle := mutedStyle
		noStyle := mutedStyle
		if m.choice == 0 {
			yesStyle = focusedStyle
		} else {
			noStyle = focusedStyle
		}

		buttons := fmt.Sprintf("%s   %s", yesStyle.Render("[ Yes ]"), noStyle.Render("[ No ]"))
		b.WriteString(buttons)
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("←/→ move · Enter confirm · Esc cancel"))

	case modeMenu:
		b.WriteString(renderWithBoldMarkers(displayText, textStyle, boldTextStyle))
		b.WriteString("\n\n")

		visible := m.listVisibleCount(len(m.cfg.Items))
		start, end := listWindow(len(m.cfg.Items), visible, m.cursor)
		if start > 0 {
			b.WriteString(mutedStyle.Render("  ..."))
			b.WriteString("\n")
		}

		for i := start; i < end; i++ {
			it := m.cfg.Items[i]
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

		if end < len(m.cfg.Items) {
			b.WriteString(mutedStyle.Render("  ..."))
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(helpStyle.Render("↑/↓ move · Enter select · Esc cancel"))

	case modeChecklist:
		b.WriteString(renderWithBoldMarkers(displayText, textStyle, boldTextStyle))
		b.WriteString("\n\n")

		visible := m.listVisibleCount(len(m.cfg.Items))
		start, end := listWindow(len(m.cfg.Items), visible, m.cursor)
		if start > 0 {
			b.WriteString(mutedStyle.Render("  ..."))
			b.WriteString("\n")
		}

		for i := start; i < end; i++ {
			it := m.cfg.Items[i]
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

		if end < len(m.cfg.Items) {
			b.WriteString(mutedStyle.Render("  ..."))
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(helpStyle.Render("↑/↓ move · Space toggle · Enter confirm · Esc cancel"))

	case modeForm:
		b.WriteString(renderWithBoldMarkers(displayText, textStyle, boldTextStyle))
		b.WriteString("\n\n")

		inputIndex := 0
		for _, f := range m.cfg.Fields {
			switch f.FieldType {
			case 2:
				b.WriteString(labelStyle.Render(f.Label))
				b.WriteString("\n")
			case 0, 1:
				if inputIndex < len(m.inputs) {
					fieldStyle := inputFieldStyle
					if inputIndex == m.focusIndex {
						fieldStyle = focusedInputFieldStyle
					}
					b.WriteString(labelStyle.Render(f.Label))
					b.WriteString("\n")
					b.WriteString(fieldStyle.Render(m.inputs[inputIndex].View()))
					b.WriteString("\n")
					b.WriteString("\n")
					inputIndex++
				}
			default:
				b.WriteString(labelStyle.Render(f.Label))
				b.WriteString("\n")
			}
		}

		b.WriteString("\n")
		b.WriteString(focusedStyle.Render(fmt.Sprintf("[ %s ]", m.cfg.OkLabel)))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("Tab moves focus · Enter confirm · Esc cancel"))

	case modeProgress:
		b.WriteString(renderWithBoldMarkers(displayText, focusedStyle, boldTextStyle))
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
			p.GeekoGreen,
			p.YarrowYellow,
			p.Orange,
			p.RadishRed,
			p.PlumPurple,
			p.ButterflyBlue,
			p.TurquoiseTeal,
			p.BagelBeige,
			p.GabbroGray,
			p.MapleMaroon,
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

	out := b.String()
	if highContrastTheme {
		out = lipgloss.NewStyle().Foreground(p.BagelBeige).Background(p.MapleMaroon).Render(out)
	}

	out = placeAligned(out, m.cfg.Align, m.width, m.height)

	v := tea.NewView(out)
	return v
}

func renderInfoBox(cfg config, termWidth, termHeight int) string {
	p := cfg.Palette
	if p.GeekoGreen == nil {
		p = opensuse
	}

	highContrastTheme := strings.EqualFold(cfg.Theme, "high-contrast")
	rainbowTheme := strings.EqualFold(cfg.Theme, "rainbow")

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(p.ButterflyBlue)

	titleAccentStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(p.PlumPurple)

	backtitleStyle := lipgloss.NewStyle().
		Foreground(p.GabbroGray)

	textStyle := lipgloss.NewStyle().
		Foreground(p.BagelBeige)

	boldTextStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(p.ButterflyBlue)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.GeekoGreen).
		Padding(1, 2)

	if highContrastTheme {
		titleStyle = titleStyle.Foreground(p.TurquoiseTeal).Background(p.MapleMaroon)
		titleAccentStyle = titleAccentStyle.Foreground(p.BagelBeige).Background(p.MapleMaroon)
		backtitleStyle = backtitleStyle.Foreground(p.BagelBeige).Background(p.MapleMaroon)
		textStyle = textStyle.Foreground(p.BagelBeige).Background(p.MapleMaroon)
		boldTextStyle = boldTextStyle.Foreground(p.YarrowYellow).Background(p.MapleMaroon)
		boxStyle = boxStyle.Foreground(p.BagelBeige).Background(p.MapleMaroon).Border(lipgloss.ThickBorder()).BorderForeground(p.BagelBeige)
	}

	bodyWidth := cfg.Width - 6
	if bodyWidth <= 0 {
		bodyWidth = 54
	}
	bodyWidth = clampInt(bodyWidth, 20, 200)

	bodyHeight := cfg.Height - 4
	if bodyHeight <= 0 {
		bodyHeight = 6
	}
	bodyHeight = clampInt(bodyHeight, 1, 1000)

	bodyText := normalizeDialogText(cfg.Text, cfg.NoNLExpand)
	bodyText = wrapText(bodyText, bodyWidth)
	lines := strings.Split(bodyText, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}

	if len(lines) > bodyHeight {
		lines = lines[:bodyHeight]
		if bodyHeight > 0 {
			lines[bodyHeight-1] = wrapLine("...", bodyWidth)
		}
	}

	bodyText = strings.Join(lines, "\n")
	bodyText = renderWithBoldMarkers(bodyText, textStyle, boldTextStyle)

	var b strings.Builder
	if cfg.Backtitle != "" {
		b.WriteString(backtitleStyle.Render(cfg.Backtitle))
		b.WriteString("\n")
	}

	if cfg.Title != "" {
		title := normalizeDialogText(cfg.Title, cfg.NoNLExpand)
		b.WriteString(renderWithBoldMarkers(title, titleStyle, titleAccentStyle))
		b.WriteString("\n")
		if rainbowTheme {
			b.WriteString(renderRainbowUnderline(40, p))
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(p.PlumPurple).Bold(true).Render(strings.Repeat("━", 40)))
		}
		b.WriteString("\n\n")
	}

	if rainbowTheme {
		b.WriteString(renderRainbowFrame(bodyText, p))
	} else {
		b.WriteString(boxStyle.Width(bodyWidth).Render(bodyText))
	}

	out := b.String()
	if highContrastTheme {
		out = lipgloss.NewStyle().Foreground(p.BagelBeige).Background(p.MapleMaroon).Render(out)
	}
	out = placeAligned(out, cfg.Align, termWidth, termHeight)

	return out
}

func parseArgs(args []string) (config, error) {
	cfg := config{OkLabel: "OK", ExitLabel: "Exit", OutputFD: 2, Clear: true}
	var i int

	for i < len(args) {
		switch args[i] {
		case "--clear":
			cfg.Clear = true
			i++

			// Common options here are primarily tracked for jeos-firstboot dialog compatibility.

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

		case "--ok-label":
			if i+1 >= len(args) {
				return cfg, errors.New("missing value for --ok-label")
			}
			cfg.OkLabel = args[i+1]
			i += 2

		case "--cancel-label":
			if i+1 >= len(args) {
				return cfg, errors.New("missing value for --cancel-label")
			}
			cfg.CancelLabel = args[i+1]
			i += 2

		case "--exit-label":
			if i+1 >= len(args) {
				return cfg, errors.New("missing value for --exit-label")
			}
			cfg.ExitLabel = args[i+1]
			i += 2

		case "--output-fd":
			if i+1 >= len(args) {
				return cfg, errors.New("missing value for --output-fd")
			}
			fd, err := strconv.Atoi(args[i+1])
			if err != nil || fd < 0 {
				return cfg, errors.New("invalid value for --output-fd")
			}
			cfg.OutputFD = fd
			i += 2

		case "--default-item":
			if i+1 >= len(args) {
				return cfg, errors.New("missing value for --default-item")
			}
			cfg.DefaultItem = args[i+1]
			i += 2

		case "--no-nl-expand":
			cfg.NoNLExpand = true
			i++

		case "--no-collapse":
			cfg.NoCollapse = true
			i++

		case "--insecure":
			cfg.Insecure = true
			i++

		case "--theme":
			if i+1 >= len(args) {
				return cfg, errors.New("missing value for --theme")
			}
			cfg.Theme = args[i+1]
			i += 2

			case "--align":
				if i+1 >= len(args) {
					return cfg, errors.New("missing value for --align")
				}
				cfg.Align = normalizeAlignment(args[i+1])
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

		case "--infobox":
			if i+3 >= len(args) {
				return cfg, errors.New("invalid --infobox arguments")
			}
			cfg.Mode = modeInfoBox
			cfg.Text = args[i+1]
			cfg.Height, _ = strconv.Atoi(args[i+2])
			cfg.Width, _ = strconv.Atoi(args[i+3])
			return cfg, nil

		case "--yesno":
			if i+3 >= len(args) {
				return cfg, errors.New("invalid --yesno arguments")
			}
			cfg.Mode = modeYesNo
			cfg.Text = args[i+1]
			cfg.Height, _ = strconv.Atoi(args[i+2])
			cfg.Width, _ = strconv.Atoi(args[i+3])
			return cfg, nil

		case "--textbox", "--text-box":
			if i+3 >= len(args) {
				return cfg, errors.New("invalid --textbox arguments")
			}
			content, err := os.ReadFile(args[i+1])
			if err != nil {
				return cfg, fmt.Errorf("failed to read textbox file: %w", err)
			}
			cfg.Mode = modeTextBox
			cfg.Text = string(content)
			cfg.Height, _ = strconv.Atoi(args[i+2])
			cfg.Width, _ = strconv.Atoi(args[i+3])
			return cfg, nil

		case "--inputbox":
			if i+3 >= len(args) {
				return cfg, errors.New("invalid --inputbox arguments")
			}
			cfg.Mode = modeInputBox
			cfg.Text = args[i+1]
			cfg.Height, _ = strconv.Atoi(args[i+2])
			cfg.Width, _ = strconv.Atoi(args[i+3])
			return cfg, nil

		case "--passwordbox":
			if i+3 >= len(args) {
				return cfg, errors.New("invalid --passwordbox arguments")
			}
			cfg.Mode = modePasswordBox
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
					Label:     args[j],
					Value:     args[j+3],
					FieldType: 0,
				})
			}
			return cfg, nil

		case "--mixedform":
			if i+4 >= len(args) {
				return cfg, errors.New("invalid --mixedform arguments")
			}
			cfg.Mode = modeForm
			cfg.Text = args[i+1]
			cfg.Height, _ = strconv.Atoi(args[i+2])
			cfg.Width, _ = strconv.Atoi(args[i+3])
			cfg.ListHeight, _ = strconv.Atoi(args[i+4])

			for j := i + 5; j+8 < len(args); j += 9 {
				if strings.HasPrefix(args[j], "--") {
					break
				}
				fieldType, _ := strconv.Atoi(args[j+8])
				cfg.Fields = append(cfg.Fields, formField{
					Label:     args[j],
					Value:     args[j+3],
					FieldType: fieldType,
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

	output := os.Stderr
	if m.cfg.OutputFD != 2 {
		output = os.NewFile(uintptr(m.cfg.OutputFD), fmt.Sprintf("fd-%d", m.cfg.OutputFD))
		if output == nil {
			output = os.Stderr
		}
	}

	switch m.cfg.Mode {
	case modeMsgBox:
		return 0

	case modeInfoBox:
		return 0

	case modeYesNo:
		if m.choice == 1 {
			return 1
		}
		return 0

	case modeMenu:
		if len(m.cfg.Items) == 0 {
			return 1
		}
		_, _ = fmt.Fprintln(output, m.cfg.Items[m.cursor].Tag)
		return 0

	case modeChecklist:
		var selected []string
		for _, it := range m.cfg.Items {
			if it.On {
				selected = append(selected, fmt.Sprintf("\"%s\"", it.Tag))
			}
		}
		_, _ = fmt.Fprintln(output, strings.Join(selected, " "))
		return 0

	case modeForm:
		for _, in := range m.inputs {
			_, _ = fmt.Fprintln(output, in.Value())
		}
		return 0

	case modeProgress:
		return 0

	case modeTextBox:
		return 0

	case modeInputBox, modePasswordBox:
		if len(m.inputs) > 0 {
			_, _ = fmt.Fprintln(output, m.inputs[0].Value())
		}
		return 0

	default:
		return 1
	}
}

func printVersion() {
	fmt.Printf("susedialog version %s\n", gitCommit)
}

func printHelp() {
	name := filepath.Base(os.Args[0])

	fmt.Printf("susedialog (openSUSE Dialog) version %s\n", gitCommit)
	fmt.Println("Copyright 2026 openSUSE")
	fmt.Println("This is free software; see the source for copying conditions.  There is NO")
	fmt.Println("warranty; not even for MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.")
	fmt.Println()
	fmt.Println("* Display dialog-like boxes from shell scripts *")
	fmt.Println()
	fmt.Printf("Usage: %s <options>\n", name)
	fmt.Println("where options are common options, followed by one box option")
	fmt.Println()
	fmt.Println("Special options:")
	fmt.Println("  [--help] [--version]")
	fmt.Println("Common options:")
	fmt.Println("  [--clear] [--title <title>] [--backtitle <backtitle>] [--ok-label <str>] [--cancel-label <str>] [--exit-label <str>] [--output-fd <fd>] [--default-item <str>] [--no-nl-expand] [--no-collapse] [--insecure] [--theme <name>] [--align <topleft|center>]")
	fmt.Println("Box options:")
	fmt.Println("  --msgbox     <text> <height> <width>")
	fmt.Println("  --infobox    <text> <height> <width>")
	fmt.Println("  --textbox    <file> <height> <width>")
	fmt.Println("  --yesno      <text> <height> <width>")
	fmt.Println("  --inputbox   <text> <height> <width>")
	fmt.Println("  --passwordbox <text> <height> <width>")
	fmt.Println("  --mixedform  <text> <height> <width> <form-height> <label1> <l_y1> <l_x1> <item1> <i_y1> <i_x1> <flen1> <ilen1> <itype1>...")
	fmt.Println("  --menu       <text> <height> <width> <menu-height> <tag1> <item1>...")
	fmt.Println("  --checklist  <text> <height> <width> <list-height> <tag1> <item1> <status1>...")
	fmt.Println("  --form       <text> <height> <width> <form-height> <label1> <l_y1> <l_x1> <item1> <i_y1> <i_x1> <flen1> <ilen1>...")
	fmt.Println("  --progress   <text> <height> <width> [<percent>]")
}

func main() {
	if len(os.Args) == 1 {
		printHelp()
		os.Exit(0)
	}

	for _, arg := range os.Args[1:] {
		switch arg {
		case "-h", "--help":
			printHelp()
			os.Exit(0)
		case "--version":
			printVersion()
			os.Exit(0)
		}
	}

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	requestedTheme := cfg.Theme
	bundledThemes := loadBundledThemes()
	resolvedThemeName := resolveThemeName(cfg.Theme)
	cfg.Align = resolveAlignment(cfg.Align)
	cfg.ThemeToggleKey = resolveThemeToggleKey()
	cfg.Themes = bundledThemes
	cfg.Theme, cfg.Palette = resolvePalette(resolvedThemeName, bundledThemes)

	if envEnabled("SUSEDIALOG_DEBUG_THEME") {
		exeDir := executableDir()
		themeDir := ""
		if exeDir != "" {
			themeDir = filepath.Join(exeDir, "themes")
		}
		_, _ = fmt.Fprintf(
			os.Stderr,
			"[susedialog debug] requested_theme=%q resolved_theme=%q executable=%q themes_dir=%q\n",
			requestedTheme,
			cfg.Theme,
			exeDir,
			themeDir,
		)
	}

	if cfg.Clear {
		fmt.Print("\033[2J\033[H")
	}

	if cfg.Mode == modeInfoBox {
		termWidth, _ := strconv.Atoi(strings.TrimSpace(os.Getenv("COLUMNS")))
		termHeight, _ := strconv.Atoi(strings.TrimSpace(os.Getenv("LINES")))
		fmt.Println(renderInfoBox(cfg, termWidth, termHeight))
		os.Exit(0)
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

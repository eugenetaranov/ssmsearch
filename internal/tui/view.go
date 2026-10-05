package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func (m model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.WindowTitle = "ssmsearch"
	return v
}

func (m model) render() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	w := m.width
	var b strings.Builder

	b.WriteString(m.headerLine(w))
	b.WriteByte('\n')
	b.WriteString(m.input.View())
	b.WriteByte('\n')

	rows := m.listRows()
	for i := 0; i < rows; i++ {
		idx := m.offset + i
		if idx < len(m.matches) {
			b.WriteString(m.listLine(idx, w))
		} else if i == 0 && !m.loading {
			b.WriteString(dim.Render("  no matches"))
		}
		b.WriteByte('\n')
	}

	rule := ruleSt.Render(strings.Repeat("─", w))
	b.WriteString(rule)
	b.WriteByte('\n')
	for _, line := range m.valueLines(w, m.valueRows()) {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteString(rule)
	b.WriteByte('\n')
	b.WriteString(m.footerLine(w))
	return b.String()
}

func (m model) headerLine(w int) string {
	left := titleSt.Render("ssmsearch")
	if m.opts.Prefix != "" && m.opts.Prefix != "/" {
		left += dim.Render("  " + m.opts.Prefix)
	}
	var right string
	if m.loading {
		right = m.spin.View() + " " + dim.Render(m.loadMsg)
	} else {
		right = dim.Render(fmt.Sprintf("%d/%d", len(m.matches), len(m.keys)))
	}
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return ansi.Truncate(left+" "+right, w, "…")
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m model) listLine(idx, w int) string {
	name := m.matches[idx]
	avail := w - 2
	if lipgloss.Width(name) > avail && avail > 1 {
		// Keep the tail of long paths; the leaf is the most informative part.
		name = "…" + name[len(name)-(avail-1):]
	}
	if idx == m.cursor {
		return selSt.Render("▌ " + padRight(name, avail))
	}
	return "  " + highlight(name, m.terms)
}

// highlight emphasises every occurrence of the search terms in name.
// SSM names are ASCII, so byte offsets are safe to use.
func highlight(name string, terms []string) string {
	if len(terms) == 0 {
		return name
	}
	lower := strings.ToLower(name)
	mark := make([]bool, len(name))
	for _, t := range terms {
		t = strings.ToLower(t)
		if t == "" {
			continue
		}
		for start := 0; ; {
			i := strings.Index(lower[start:], t)
			if i < 0 {
				break
			}
			for j := start + i; j < start+i+len(t) && j < len(mark); j++ {
				mark[j] = true
			}
			start += i + 1
		}
	}

	var b strings.Builder
	for i := 0; i < len(name); {
		j := i
		for j < len(name) && mark[j] == mark[i] {
			j++
		}
		if mark[i] {
			b.WriteString(matchSt.Render(name[i:j]))
		} else {
			b.WriteString(name[i:j])
		}
		i = j
	}
	return b.String()
}

func (m model) valueLines(w, n int) []string {
	lines := make([]string, 0, n)
	name := m.selected()
	entry, ok := m.values[name]

	switch {
	case name == "":
		lines = append(lines, dim.Render("  nothing selected"))
	case !ok:
		lines = append(lines, m.spin.View()+dim.Render(" fetching value..."))
	case entry.err != nil:
		lines = append(lines, errSt.Render("  "+ansi.Truncate(entry.err.Error(), w-2, "…")))
	default:
		p := entry.param
		meta := p.Type
		if p.Type == "SecureString" {
			meta = secretSt.Render(meta)
		}
		meta += dim.Render(fmt.Sprintf("  v%d", p.Version))
		if !p.LastModified.IsZero() {
			meta += dim.Render("  " + p.LastModified.Local().Format("2006-01-02 15:04"))
		}
		lines = append(lines, meta)

		body := strings.Split(ansi.Hardwrap(sanitize(p.Value), max(1, w), true), "\n")
		room := n - 1
		if len(body) > room {
			more := len(body) - room + 1
			body = append(body[:room-1], dim.Render(fmt.Sprintf("… %d more lines (enter prints full value)", more)))
		}
		lines = append(lines, body...)
	}

	for len(lines) < n {
		lines = append(lines, "")
	}
	return lines[:n]
}

func (m model) footerLine(w int) string {
	if m.status != "" {
		st := okSt
		if m.statusErr {
			st = errSt
		}
		return st.Render(ansi.Truncate(m.status, w, "…"))
	}
	help := []struct{ key, desc string }{
		{"↑↓", "move"},
		{"enter", "print value"},
		{"ctrl+y", "copy"},
		{"ctrl+r", "refresh"},
		{"esc", "quit"},
	}
	parts := make([]string, len(help))
	for i, h := range help {
		parts[i] = helpKeySt.Render(h.key) + " " + dim.Render(h.desc)
	}
	return ansi.Truncate(strings.Join(parts, dim.Render(" · ")), w, "…")
}

// sanitize makes a parameter value safe to draw: tabs become spaces and
// other control characters (including ANSI escapes) are dropped.
func sanitize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n':
			return r
		case r == '\t':
			return ' '
		case r < 0x20 || r == 0x7f:
			return -1
		}
		return r
	}, s)
}

func padRight(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

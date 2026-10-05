// Package tui implements the interactive fuzzy-search interface.
package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"

	"ssmsearch/internal/search"
	"ssmsearch/internal/ssm"
)

// LoadFunc returns all parameter names. When refresh is true it must bypass
// any cache. progress may be called with status text while loading.
type LoadFunc func(ctx context.Context, refresh bool, progress func(string)) ([]string, error)

// Options configures the interactive session.
type Options struct {
	Client  ssm.Client
	Load    LoadFunc
	Prefix  string // only keys under this path are shown
	Query   string // initial filter text
	Decrypt bool
}

// Run starts the TUI on stderr and blocks until the user quits. It returns
// the parameter chosen with enter, or nil if the user cancelled.
func Run(ctx context.Context, opts Options) (*ssm.Parameter, error) {
	m := newModel(ctx, opts)
	p := tea.NewProgram(m, tea.WithContext(ctx), tea.WithOutput(os.Stderr))
	m.send.fn = p.Send

	final, err := p.Run()
	if err != nil {
		return nil, err
	}
	fm := final.(model)
	if fm.err != nil {
		return nil, fm.err
	}
	return fm.chosen, nil
}

const valueDebounce = 150 * time.Millisecond

var (
	accent    = lipgloss.Color("6")
	dim       = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	titleSt   = lipgloss.NewStyle().Bold(true).Foreground(accent)
	selSt     = lipgloss.NewStyle().Reverse(true)
	matchSt   = lipgloss.NewStyle().Foreground(accent).Bold(true)
	errSt     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	okSt      = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	ruleSt    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	secretSt  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	helpKeySt = lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Bold(true)
)

// Messages.
type (
	keysLoadedMsg struct {
		keys []string
		err  error
	}
	progressMsg   string
	valueTickMsg  struct{ seq int }
	valueFetchMsg struct {
		name  string
		param *ssm.Parameter
		err   error
	}
	statusClearMsg struct{ seq int }
)

// sender lets loader goroutines post messages to the running program.
type sender struct{ fn func(tea.Msg) }

func (s *sender) Send(msg tea.Msg) {
	if s.fn != nil {
		s.fn(msg)
	}
}

type valueEntry struct {
	param *ssm.Parameter
	err   error
}

type model struct {
	ctx  context.Context
	opts Options
	send *sender

	input   textinput.Model
	spin    spinner.Model
	loading bool
	loadMsg string

	keys     []string // all keys under the prefix
	matches  []string
	cursor   int
	offset   int
	terms    []string
	lastTerm string

	values   map[string]valueEntry
	fetching map[string]bool
	valueSeq int

	status    string
	statusErr bool
	statusSeq int

	width, height int

	chosen *ssm.Parameter
	err    error
}

func newModel(ctx context.Context, opts Options) model {
	in := textinput.New()
	in.Prompt = "❯ "
	in.Placeholder = "type to filter, space separates terms"
	in.SetValue(opts.Query)
	in.CursorEnd()
	in.Focus()
	st := in.Styles()
	st.Focused.Prompt = st.Focused.Prompt.Foreground(accent).Bold(true)
	in.SetStyles(st)

	sp := spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(lipgloss.NewStyle().Foreground(accent)))

	return model{
		ctx:      ctx,
		opts:     opts,
		send:     &sender{},
		input:    in,
		spin:     sp,
		loading:  true,
		loadMsg:  "Loading parameter names...",
		values:   make(map[string]valueEntry),
		fetching: make(map[string]bool),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spin.Tick, m.loadKeys(false))
}

func (m model) loadKeys(refresh bool) tea.Cmd {
	ctx, load, send := m.ctx, m.opts.Load, m.send
	return func() tea.Msg {
		keys, err := load(ctx, refresh, func(s string) { send.Send(progressMsg(s)) })
		return keysLoadedMsg{keys: keys, err: err}
	}
}

func (m model) fetchValue(name string) tea.Cmd {
	ctx, client, decrypt := m.ctx, m.opts.Client, m.opts.Decrypt
	return func() tea.Msg {
		params, err := client.GetParameters(ctx, []string{name}, decrypt)
		if err != nil {
			return valueFetchMsg{name: name, err: err}
		}
		if len(params) == 0 {
			return valueFetchMsg{name: name, err: fmt.Errorf("parameter not found")}
		}
		return valueFetchMsg{name: name, param: &params[0]}
	}
}

func (m *model) setStatus(s string, isErr bool) tea.Cmd {
	m.statusSeq++
	m.status, m.statusErr = s, isErr
	seq := m.statusSeq
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return statusClearMsg{seq} })
}

func (m model) selected() string {
	if m.cursor < 0 || m.cursor >= len(m.matches) {
		return ""
	}
	return m.matches[m.cursor]
}

// refilter recomputes matches after the query or key set changed, keeping the
// current selection when it is still present.
func (m *model) refilter() tea.Cmd {
	prev := m.selected()
	m.terms = strings.Fields(m.input.Value())
	m.lastTerm = m.input.Value()
	m.matches = search.Filter(m.keys, m.terms)
	m.cursor, m.offset = 0, 0
	for i, k := range m.matches {
		if k == prev {
			m.cursor = i
			break
		}
	}
	m.clampOffset()
	return m.scheduleValue()
}

// scheduleValue debounces fetching the value of the selected parameter so
// scrolling quickly does not fire a request per row.
func (m *model) scheduleValue() tea.Cmd {
	m.valueSeq++
	name := m.selected()
	if name == "" {
		return nil
	}
	if _, ok := m.values[name]; ok || m.fetching[name] {
		return nil
	}
	seq := m.valueSeq
	return tea.Tick(valueDebounce, func(time.Time) tea.Msg { return valueTickMsg{seq} })
}

func (m *model) move(delta int) tea.Cmd {
	if len(m.matches) == 0 {
		return nil
	}
	c := max(0, min(len(m.matches)-1, m.cursor+delta))
	if c == m.cursor {
		return nil
	}
	m.cursor = c
	m.clampOffset()
	return m.scheduleValue()
}

func (m *model) clampOffset() {
	rows := m.listRows()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
	m.offset = max(0, min(m.offset, max(0, len(m.matches)-rows)))
}

// Layout: header(1) input(1) list(n) rule(1) value(v) rule(1) help(1).
func (m model) valueRows() int {
	return max(3, min(12, m.height/3))
}

func (m model) listRows() int {
	return max(1, m.height-m.valueRows()-5)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.SetWidth(max(10, m.width-4))
		m.clampOffset()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case progressMsg:
		m.loadMsg = string(msg)
		return m, nil

	case keysLoadedMsg:
		m.loading = false
		if msg.err != nil {
			if len(m.keys) == 0 {
				m.err = msg.err
				return m, tea.Quit
			}
			return m, m.setStatus("Refresh failed: "+msg.err.Error(), true)
		}
		m.keys = search.ByPrefix(msg.keys, m.opts.Prefix)
		return m, m.refilter()

	case valueTickMsg:
		if msg.seq != m.valueSeq {
			return m, nil
		}
		name := m.selected()
		if name == "" || m.fetching[name] {
			return m, nil
		}
		if _, ok := m.values[name]; ok {
			return m, nil
		}
		m.fetching[name] = true
		return m, m.fetchValue(name)

	case valueFetchMsg:
		delete(m.fetching, msg.name)
		m.values[msg.name] = valueEntry{param: msg.param, err: msg.err}
		return m, nil

	case chooseMsg:
		v := m.values[msg.name]
		if v.param == nil {
			return m, m.setStatus("Cannot read value: "+errText(v.err), true)
		}
		m.chosen = v.param
		return m, tea.Quit

	case copyResultMsg:
		if msg.err != nil {
			return m, m.setStatus("Copy failed: "+msg.err.Error(), true)
		}
		return m, m.setStatus("Copied value of "+msg.name, false)

	case statusClearMsg:
		if msg.seq == m.statusSeq {
			m.status = ""
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		return m, tea.Quit
	case "up", "ctrl+p", "ctrl+k":
		return m, m.move(-1)
	case "down", "ctrl+n", "ctrl+j":
		return m, m.move(1)
	case "pgup":
		return m, m.move(-m.listRows())
	case "pgdown":
		return m, m.move(m.listRows())
	case "home":
		return m, m.move(-len(m.matches))
	case "end":
		return m, m.move(len(m.matches))
	case "enter":
		name := m.selected()
		if name == "" {
			return m, nil
		}
		if v, ok := m.values[name]; ok && v.param != nil {
			m.chosen = v.param
			return m, tea.Quit
		}
		// Value not loaded yet: fetch it first, then choose.
		return m, tea.Sequence(m.fetchValue(name), func() tea.Msg { return chooseMsg{name} })
	case "ctrl+y":
		return m, m.copySelected()
	case "ctrl+r":
		if m.loading {
			return m, nil
		}
		m.loading = true
		m.loadMsg = "Refreshing parameter names..."
		m.values = make(map[string]valueEntry)
		return m, tea.Batch(m.spin.Tick, m.loadKeys(true))
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != m.lastTerm {
		return m, tea.Batch(cmd, m.refilter())
	}
	return m, cmd
}

type chooseMsg struct{ name string }

func (m model) copySelected() tea.Cmd {
	name := m.selected()
	if name == "" {
		return nil
	}
	v, ok := m.values[name]
	if !ok || v.param == nil {
		return func() tea.Msg { return copyResultMsg{err: fmt.Errorf("value not loaded yet")} }
	}
	value := v.param.Value
	// OSC52 covers SSH sessions; the native clipboard covers local terminals
	// that ignore OSC52 (e.g. Terminal.app).
	return tea.Batch(tea.SetClipboard(value), func() tea.Msg {
		return copyResultMsg{name: name, err: clipboard.WriteAll(value)}
	})
}

type copyResultMsg struct {
	name string
	err  error
}

func errText(err error) string {
	if err == nil {
		return "unknown error"
	}
	return err.Error()
}

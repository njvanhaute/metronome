package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	metronomeDisplay    = ""
	bpm                 = 0
	done                = make(chan struct{})
	focusedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	blurredStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursorStyle         = focusedStyle.Copy()
	noStyle             = lipgloss.NewStyle()
	helpStyle           = blurredStyle.Copy()
	cursorModeHelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	currentBarIterator  = 0
	chordsPerBar        = []string{"G", "G", "G", "G", "D", "D", "D", "D"}
	DefaultMetronome    = Metronome{
		Frames: []string{"X ", "    X"},
		FPS:    time.Second / 100,
	}
)

type Metronome struct {
	Frames []string
	FPS    time.Duration
}

func main() {
	configureLog()
	// go tickMetronome()
	if err := tea.NewProgram(initialModel()).Start(); err != nil {
		fmt.Printf("could not start program: %s\n", err)
		os.Exit(1)
	}
}

// write log to file
func configureLog() {
	f, err := os.OpenFile("metronome.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		fmt.Printf("error opening file: %v", err)
	}
	log.SetOutput(f)
}

const debug = true

func lg(output string) {
	if debug {
		log.Println(output)
	}
}

type model struct {
	focusIndex int
	bpmInput   textinput.Model
	Style      lipgloss.Style
	metronome  Metronome
	frame      int
	id         int
	tag        int
	cursorMode textinput.CursorMode
}

func (m model) ID() int {
	return m.id
}

type tickMsg struct {
	Time time.Time
	tag  int
	ID   int
}

func initialModel() model {
	var t textinput.Model
	t = textinput.New()
	t.CursorStyle = cursorStyle
	t.CharLimit = 32

	t.Placeholder = "Beats Per Minute"
	t.Focus()
	t.PromptStyle = focusedStyle
	t.TextStyle = focusedStyle

	return model{
		bpmInput:  t,
		metronome: DefaultMetronome,
		id:        nextID(),
	}
}

var (
	lastID int
	idMtx  sync.Mutex
)

func nextID() int {
	lg("nextID()")
	idMtx.Lock()
	defer idMtx.Unlock()
	lastID++
	return lastID
}

func (m model) Init() tea.Cmd {
	lg("model.Init()")
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	lg("m.Update()")
	lg(fmt.Sprintf("%+v", msg))
	switch msg := msg.(type) {
	case tea.KeyMsg:
		lg("m.Update() tea.KeyMsg")
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "ctrl+r":
			m.cursorMode++
			if m.cursorMode > textinput.CursorHide {
				m.cursorMode = textinput.CursorBlink
			}
			cmds := m.bpmInput.SetCursorMode(m.cursorMode)
			return m, cmds
		default:
			// Handle character input and blinking
			cmd := m.updateInputs(msg)

			return m, cmd
		}
	case tickMsg:
		lg("tickMsg")
		if msg.ID > 0 && msg.ID != m.id {
			return m, nil
		}
		if msg.tag > 0 && msg.tag != m.tag {
			return m, nil
		}
		m.frame++
		if m.frame >= len(m.metronome.Frames) {
			m.frame = 0
		}
		m.tag++
		return m, m.tick(m.id, m.tag)
	}
	lg("m.Update() past msg type switch")
	// return nil,
	return m, nil
}

func (m model) tick(id, tag int) tea.Cmd {
	lg("m.tick()")
	return tea.Tick(m.metronome.FPS, func(t time.Time) tea.Msg {
		return tickMsg{
			Time: t,
			ID:   id,
			tag:  tag,
		}
	})
}

func (m *model) updateInputs(msg tea.Msg) tea.Cmd {
	lg("model.updateInputs()")
	var cmd tea.Cmd

	m.bpmInput, cmd = m.bpmInput.Update(msg)
	setMetronomeBpm(m.bpmInput.Value())

	return cmd
}

// Updates metronome view each bpm
func tickMetronome() {
	var ti = 1
	for {
		// log.Printf("iteratting and bpm is %d\n", bpm)
		if !(bpm > 0) {
			continue
		}
		milliseconds := 60000 / bpm
		select {
		case <-done:
		case <-time.After(time.Duration(milliseconds) * time.Millisecond):
			// lg(ti)
			ti += 1
			spaceToPrepend := strings.Repeat(" ", currentBarIterator)
			currentBarIterator = (currentBarIterator + 1) % len(chordsPerBar)
			metronomeDisplay = spaceToPrepend + chordsPerBar[currentBarIterator]
		}
	}
}

func (m model) View() string {
	lg("m.View()")
	var b strings.Builder

	b.WriteString(m.bpmInput.View())
	// for i := range m.bpmInput {
	// 	b.WriteString(m.bpmInput[i].View())
	// 	if i < len(m.bpmInput)-1 {
	// 		b.WriteRune('\n')
	// 	}
	// }

	// button := &blurredButton
	// if m.focusIndex == 1 {
	// 	button = &focusedButton
	// }
	// fmt.Fprintf(&b, "\n\n%s\n\n", *button)
	fmt.Fprintf(&b, "\n\n%s\n\n", m.Style.Render(m.metronome.Frames[m.frame]))

	// b.WriteString(m.Style.Render(m.metronome.Frames[m.frame]))
	b.WriteString(helpStyle.Render("cursor mode is "))
	b.WriteString(cursorModeHelpStyle.Render(m.cursorMode.String()))
	b.WriteString(helpStyle.Render(" (ctrl+r to change style)"))

	return b.String()
}

func setMetronomeBpm(bpmInput string) {
	intVar, err := strconv.Atoi(bpmInput)
	if err != nil {
		lg("bad number in bpm")
		return
	}
	bpm = intVar
}
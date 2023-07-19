package queryinput

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/itchyny/gojq"
	"github.com/noahgorstein/jqp/tui/theme"
)

type Bubble struct {
	Styles    Styles
	textinput textinput.Model

	history         *list.List
	historyMaxLen   int
	historySelected *list.Element
	inputJson       map[string]any
	possibleKey     string
}

func New(theme theme.Theme, inputJson []byte) (Bubble, error) {

	s := DefaultStyles()
	s.containerStyle.BorderForeground(theme.Primary)
	ti := textinput.New()
	ti.Focus()
	ti.BackgroundStyle.Height(1)
	ti.PromptStyle.Height(1)
	ti.TextStyle.Height(1)
	ti.Prompt = lipgloss.NewStyle().Bold(true).Foreground(theme.Secondary).Render("jq > ")

	data := map[string]any{}
	err := json.Unmarshal(inputJson, &data)
	if err != nil {
		return Bubble{}, err
	}
	s.autocompleteHintStyle = s.autocompleteHintStyle.Foreground(theme.Secondary)

	return Bubble{
		Styles:    s,
		textinput: ti,

		history:       list.New(),
		historyMaxLen: 512,

		inputJson: data,
	}, nil
}

func (b *Bubble) SetBorderColor(color lipgloss.TerminalColor) {
	b.Styles.containerStyle.BorderForeground(color)
}

func (b Bubble) GetInputValue() string {
	return b.textinput.Value()
}

func (b *Bubble) RotateHistory() {
	b.history.PushFront(b.textinput.Value())
	b.historySelected = b.history.Front()
	if b.history.Len() > b.historyMaxLen {
		b.history.Remove(b.history.Back())
	}
}

func (b *Bubble) FillAutocomplete() {
	if b.possibleKey == "" {
		return
	}

	cmps := strings.Split(b.textinput.Value(), ".")
	if len(cmps) < 1 {
		return
	}

	newValue := fmt.Sprintf("%s%s", b.textinput.Value(), b.possibleKey)
	b.textinput.SetValue(newValue)
	b.textinput.SetCursor(len(newValue))
	b.possibleKey = ""
}

func (b *Bubble) ShowAutocomplete() {
	query := strings.TrimSpace(b.textinput.Value())
	if query == "" {
		b.possibleKey = ""
		return
	}

	cmps := strings.Split(query, ".")
	lastKey := ""
	if len(cmps) > 0 {
		lastKey = cmps[len(cmps)-1]
	}

	withoutLastKey := ""
	if len(cmps) > 1 {
		withoutLastKey = fmt.Sprintf(".%s", strings.Join(cmps[1:len(cmps)-1], "."))
	}

	q, err := gojq.Parse(fmt.Sprintf("%s | to_entries[] | select(.key | startswith(\"%s\")) | .key", withoutLastKey, lastKey))
	if err != nil {
		b.possibleKey = ""
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()
	iter := q.RunWithContext(ctx, b.inputJson)
	for {
		v, ok := iter.Next()
		if !ok {
			b.possibleKey = ""
			break
		}
		if _, ok := v.(error); ok {
			b.possibleKey = ""
			break
		}
		if value, ok := v.(string); ok {
			if strings.HasSuffix(query, value) {
				b.possibleKey = ""
				break
			}
			b.possibleKey = strings.TrimPrefix(value, lastKey)
			break
		}
		b.possibleKey = ""
	}
}

func (b Bubble) Init() tea.Cmd {
	return textinput.Blink
}

func (b *Bubble) SetWidth(width int) {
	b.Styles.containerStyle.Width(width - b.Styles.containerStyle.GetHorizontalFrameSize())
	b.textinput.Width = width - b.Styles.containerStyle.GetHorizontalFrameSize() - 1
}

func (b Bubble) View() string {
	sb := strings.Builder{}
	inputView := b.Styles.containerStyle.Render(b.textinput.View())
	sb.WriteString(inputView)
	sb.WriteRune('\n')
	if b.possibleKey != "" {
		padding := lipgloss.Width(b.textinput.Value()) + 6 // 6 is for the prompt
		sb.WriteString(b.Styles.autocompleteHintStyle.PaddingLeft(padding).Render(b.possibleKey))
	}

	return sb.String()
}

func (b Bubble) Update(msg tea.Msg) (Bubble, tea.Cmd) {
	b.ShowAutocomplete()
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.Type {
		case tea.KeyBackspace:
			b.possibleKey = ""

		case tea.KeyCtrlL:
			b.FillAutocomplete()

		case tea.KeyUp:
			if b.history.Len() == 0 {
				break
			}
			n := b.historySelected.Next()
			if n != nil {
				b.textinput.SetValue(n.Value.(string))
				b.textinput.CursorEnd()
				b.historySelected = n
			}
		case tea.KeyDown:
			if b.history.Len() == 0 {
				break
			}
			p := b.historySelected.Prev()
			if p != nil {
				b.textinput.SetValue(p.Value.(string))
				b.textinput.CursorEnd()
				b.historySelected = p
			}
		case tea.KeyEnter:
			b.RotateHistory()
		}
	}

	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	b.textinput, cmd = b.textinput.Update(msg)
	cmds = append(cmds, cmd)

	return b, tea.Batch(cmds...)

}

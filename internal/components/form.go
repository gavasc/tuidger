package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gavasc/tuidger/internal/styles"
)

type FieldType int

const (
	FieldText FieldType = iota
	FieldNumber
	FieldDate
	FieldSelect
	FieldToggle
)

type Field struct {
	Label       string
	Type        FieldType
	Input       textinput.Model
	Options     []string
	SelectedIdx int
	Value       bool   // for FieldToggle
	Required    bool
	Error       string
	Hidden      bool // dynamically hide/show
}

type FormModel struct {
	Fields      []Field
	FocusIdx    int
	Title       string
	SubmitLabel string
}

func NewForm(title, submit string) FormModel {
	return FormModel{Title: title, SubmitLabel: submit}
}

func (f *FormModel) AddTextField(label string, required bool) {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Width = 30
	f.Fields = append(f.Fields, Field{Label: label, Type: FieldText, Input: ti, Required: required})
}

func (f *FormModel) AddNumberField(label string, required bool) {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Width = 20
	f.Fields = append(f.Fields, Field{Label: label, Type: FieldNumber, Input: ti, Required: required})
}

func (f *FormModel) AddDateField(label string, required bool) {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Width = 12
	ti.Placeholder = "YYYY-MM-DD"
	f.Fields = append(f.Fields, Field{Label: label, Type: FieldDate, Input: ti, Required: required})
}

func (f *FormModel) AddSelectField(label string, options []string, required bool) {
	f.Fields = append(f.Fields, Field{Label: label, Type: FieldSelect, Options: options, Required: required})
}

func (f *FormModel) AddToggleField(label string) {
	f.Fields = append(f.Fields, Field{Label: label, Type: FieldToggle})
}

func (f *FormModel) Focus(idx int) {
	for i := range f.Fields {
		if f.Fields[i].Type == FieldText || f.Fields[i].Type == FieldNumber || f.Fields[i].Type == FieldDate {
			if i == idx {
				f.Fields[i].Input.Focus()
			} else {
				f.Fields[i].Input.Blur()
			}
		}
	}
	f.FocusIdx = idx
}

func (f *FormModel) FocusFirst() {
	for i := range f.Fields {
		if !f.Fields[i].Hidden {
			f.Focus(i)
			return
		}
	}
}

func (f *FormModel) visibleCount() int {
	n := 0
	for _, field := range f.Fields {
		if !field.Hidden {
			n++
		}
	}
	return n
}

func (f *FormModel) visibleIndex(absIdx int) int {
	count := 0
	for i := 0; i <= absIdx && i < len(f.Fields); i++ {
		if !f.Fields[i].Hidden {
			count++
		}
	}
	return count - 1
}

func (f *FormModel) absIndexOf(visIdx int) int {
	count := 0
	for i, field := range f.Fields {
		if !field.Hidden {
			if count == visIdx {
				return i
			}
			count++
		}
	}
	return len(f.Fields) - 1
}

func (f *FormModel) NextField() {
	visIdx := f.visibleIndex(f.FocusIdx)
	next := f.absIndexOf(visIdx + 1)
	if next < len(f.Fields) {
		f.Focus(next)
	}
}

func (f *FormModel) PrevField() {
	visIdx := f.visibleIndex(f.FocusIdx)
	if visIdx > 0 {
		prev := f.absIndexOf(visIdx - 1)
		f.Focus(prev)
	}
}

func (f *FormModel) Update(msg tea.Msg) (FormModel, tea.Cmd) {
	idx := f.FocusIdx
	if idx >= len(f.Fields) {
		return *f, nil
	}
	field := &f.Fields[idx]

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			f.NextField()
			return *f, nil
		case "shift+tab":
			f.PrevField()
			return *f, nil
		case "up":
			if field.Type == FieldSelect && len(field.Options) > 0 {
				field.SelectedIdx = (field.SelectedIdx - 1 + len(field.Options)) % len(field.Options)
				return *f, nil
			}
		case "down":
			if field.Type == FieldSelect && len(field.Options) > 0 {
				field.SelectedIdx = (field.SelectedIdx + 1) % len(field.Options)
				return *f, nil
			}
		case " ", "enter":
			if field.Type == FieldToggle {
				field.Value = !field.Value
				return *f, nil
			}
		}
	}

	if field.Type == FieldText || field.Type == FieldNumber || field.Type == FieldDate {
		var cmd tea.Cmd
		f.Fields[idx].Input, cmd = field.Input.Update(msg)
		return *f, cmd
	}
	return *f, nil
}

func (f *FormModel) Validate() bool {
	ok := true
	for i := range f.Fields {
		f.Fields[i].Error = ""
		field := &f.Fields[i]
		if field.Hidden {
			continue
		}
		if field.Required {
			var val string
			switch field.Type {
			case FieldText, FieldNumber, FieldDate:
				val = strings.TrimSpace(field.Input.Value())
			case FieldSelect:
				if len(field.Options) > 0 {
					val = field.Options[field.SelectedIdx]
				}
			}
			if val == "" {
				field.Error = "required"
				ok = false
			}
		}
		if field.Type == FieldNumber && field.Error == "" {
			v := strings.TrimSpace(field.Input.Value())
			if v != "" {
				v = strings.ReplaceAll(v, ",", ".")
				var f64 float64
				if _, err := fmt.Sscanf(v, "%f", &f64); err != nil {
					field.Error = "must be a number"
					ok = false
				}
			}
		}
	}
	return ok
}

func (f *FormModel) Values() map[string]string {
	out := make(map[string]string)
	for _, field := range f.Fields {
		switch field.Type {
		case FieldText, FieldNumber, FieldDate:
			out[field.Label] = field.Input.Value()
		case FieldSelect:
			if len(field.Options) > 0 {
				out[field.Label] = field.Options[field.SelectedIdx]
			}
		case FieldToggle:
			if field.Value {
				out[field.Label] = "true"
			} else {
				out[field.Label] = "false"
			}
		}
	}
	return out
}

func (f *FormModel) Reset() {
	for i := range f.Fields {
		switch f.Fields[i].Type {
		case FieldText, FieldNumber, FieldDate:
			f.Fields[i].Input.SetValue("")
			f.Fields[i].Input.Blur()
		case FieldSelect:
			f.Fields[i].SelectedIdx = 0
		case FieldToggle:
			f.Fields[i].Value = false
		}
		f.Fields[i].Error = ""
	}
	f.FocusIdx = 0
}

func (f *FormModel) View() string {
	var sb strings.Builder
	if f.Title != "" {
		sb.WriteString(styles.Title.Render(f.Title) + "\n")
	}
	for i, field := range f.Fields {
		if field.Hidden {
			continue
		}
		focused := i == f.FocusIdx
		label := field.Label + ": "
		if focused {
			label = lipgloss.NewStyle().Foreground(lipgloss.Color("#4a90d9")).Bold(true).Render(label)
		} else {
			label = styles.Faint.Render(label)
		}

		var valueStr string
		switch field.Type {
		case FieldText, FieldNumber, FieldDate:
			s := styles.Input
			if focused {
				s = styles.InputFocus
			}
			valueStr = s.Render(field.Input.View())
		case FieldSelect:
			val := ""
			if len(field.Options) > 0 {
				val = field.Options[field.SelectedIdx]
			}
			s := styles.Input
			if focused {
				s = styles.InputFocus
			}
			valueStr = s.Render(fmt.Sprintf("← %s →", val))
		case FieldToggle:
			check := "[ ]"
			if field.Value {
				check = "[x]"
			}
			s := styles.Input
			if focused {
				s = styles.InputFocus
			}
			valueStr = s.Render(check + " " + field.Label)
			label = ""
		}

		sb.WriteString(label + valueStr)
		if field.Error != "" {
			sb.WriteString(" " + styles.Error.Render(field.Error))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n" + styles.Button.Render(f.SubmitLabel) + "  " + styles.Faint.Render("Esc to cancel"))
	return sb.String()
}

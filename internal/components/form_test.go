package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func tabKey() tea.Msg  { return tea.KeyMsg{Type: tea.KeyTab} }
func enterKey() tea.Msg { return tea.KeyMsg{Type: tea.KeyEnter} }
func spaceKey() tea.Msg { return tea.KeyMsg{Type: tea.KeySpace} }
func upKey() tea.Msg    { return tea.KeyMsg{Type: tea.KeyUp} }
func downKey() tea.Msg  { return tea.KeyMsg{Type: tea.KeyDown} }

func typeText(f *FormModel, text string) {
	for _, r := range text {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		*f, _ = f.Update(msg)
	}
}

// ── Focus navigation ──────────────────────────────────────────────────────────

func TestForm_tabMovesFocusForward(t *testing.T) {
	f := NewForm("Test", "Submit")
	f.AddTextField("Name", true)
	f.AddTextField("Category", true)
	f.FocusFirst()

	if f.FocusIdx != 0 {
		t.Fatalf("expected focus on field 0, got %d", f.FocusIdx)
	}
	f, _ = f.Update(tabKey())
	if f.FocusIdx != 1 {
		t.Errorf("after Tab: expected focus on field 1, got %d", f.FocusIdx)
	}
}

func TestForm_shiftTabMovesFocusBackward(t *testing.T) {
	f := NewForm("Test", "Submit")
	f.AddTextField("A", true)
	f.AddTextField("B", true)
	f.Focus(1)

	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if f.FocusIdx != 0 {
		t.Errorf("after Shift+Tab: expected focus on field 0, got %d", f.FocusIdx)
	}
}

func TestForm_tabDoesNotWrapBeyondLastField(t *testing.T) {
	f := NewForm("Test", "Submit")
	f.AddTextField("Only", true)
	f.FocusFirst()

	f, _ = f.Update(tabKey())
	// still on last (only) field, no panic
	if f.FocusIdx != 0 {
		t.Errorf("expected to stay on field 0, got %d", f.FocusIdx)
	}
}

func TestForm_tabSkipsHiddenFields(t *testing.T) {
	f := NewForm("Test", "Submit")
	f.AddTextField("A", false)
	f.AddTextField("B", false) // will be hidden
	f.AddTextField("C", false)
	f.Fields[1].Hidden = true
	f.FocusFirst()

	f, _ = f.Update(tabKey())
	// should jump from 0 to 2, skipping the hidden field 1
	if f.FocusIdx != 2 {
		t.Errorf("expected focus on field 2 (skipping hidden), got %d", f.FocusIdx)
	}
}

// ── Select field ──────────────────────────────────────────────────────────────

func TestForm_selectCyclesWithDown(t *testing.T) {
	f := NewForm("Test", "Submit")
	f.AddSelectField("Color", []string{"red", "green", "blue"}, true)
	f.FocusFirst()

	if f.Fields[0].SelectedIdx != 0 {
		t.Fatal("expected initial SelectedIdx=0")
	}
	f, _ = f.Update(downKey())
	if f.Fields[0].SelectedIdx != 1 {
		t.Errorf("expected SelectedIdx=1 after down, got %d", f.Fields[0].SelectedIdx)
	}
}

func TestForm_selectWrapsAroundDown(t *testing.T) {
	f := NewForm("Test", "Submit")
	f.AddSelectField("Type", []string{"a", "b"}, true)
	f.FocusFirst()
	f.Fields[0].SelectedIdx = 1

	f, _ = f.Update(downKey())
	if f.Fields[0].SelectedIdx != 0 {
		t.Errorf("expected wrap-around to 0, got %d", f.Fields[0].SelectedIdx)
	}
}

func TestForm_selectCyclesWithUp(t *testing.T) {
	f := NewForm("Test", "Submit")
	f.AddSelectField("Type", []string{"a", "b", "c"}, true)
	f.FocusFirst()
	f.Fields[0].SelectedIdx = 1

	f, _ = f.Update(upKey())
	if f.Fields[0].SelectedIdx != 0 {
		t.Errorf("expected SelectedIdx=0 after up, got %d", f.Fields[0].SelectedIdx)
	}
}

// ── Toggle field ──────────────────────────────────────────────────────────────

func TestForm_toggleFlipsOnSpace(t *testing.T) {
	f := NewForm("Test", "Submit")
	f.AddToggleField("Active?")
	f.FocusFirst()

	if f.Fields[0].Value {
		t.Fatal("expected toggle to start false")
	}
	f, _ = f.Update(spaceKey())
	if !f.Fields[0].Value {
		t.Error("expected toggle=true after space")
	}
	f, _ = f.Update(spaceKey())
	if f.Fields[0].Value {
		t.Error("expected toggle=false after second space")
	}
}

func TestForm_toggleFlipsOnEnter(t *testing.T) {
	f := NewForm("Test", "Submit")
	f.AddToggleField("On?")
	f.FocusFirst()

	f, _ = f.Update(enterKey())
	if !f.Fields[0].Value {
		t.Error("expected toggle=true after enter")
	}
}

// ── Validation ────────────────────────────────────────────────────────────────

func TestForm_validateRequiredTextField(t *testing.T) {
	f := NewForm("Test", "Submit")
	f.AddTextField("Name", true)
	f.FocusFirst()

	if f.Validate() {
		t.Error("expected validation to fail for empty required text field")
	}
	if f.Fields[0].Error == "" {
		t.Error("expected error message on the failing field")
	}
}

func TestForm_validatePassesWhenFilled(t *testing.T) {
	f := NewForm("Test", "Submit")
	f.AddTextField("Name", true)
	f.FocusFirst()
	typeText(&f, "Alice")

	if !f.Validate() {
		t.Errorf("expected validation to pass, field error: %q", f.Fields[0].Error)
	}
}

func TestForm_validateNumberField(t *testing.T) {
	f := NewForm("Test", "Submit")
	f.AddNumberField("Amount", true)
	f.FocusFirst()
	typeText(&f, "abc")

	if f.Validate() {
		t.Error("expected validation to fail for non-numeric amount")
	}
	if f.Fields[0].Error == "" {
		t.Error("expected error on numeric field")
	}
}

func TestForm_validateNumberFieldAcceptsComma(t *testing.T) {
	f := NewForm("Test", "Submit")
	f.AddNumberField("Amount", true)
	f.FocusFirst()
	typeText(&f, "12,50")

	if !f.Validate() {
		t.Errorf("comma-separated number should be valid, error: %q", f.Fields[0].Error)
	}
}

func TestForm_validateSkipsHiddenFields(t *testing.T) {
	f := NewForm("Test", "Submit")
	f.AddTextField("Visible", true)
	f.AddTextField("Hidden required", true)
	f.Fields[1].Hidden = true
	f.FocusFirst()
	typeText(&f, "hello")

	if !f.Validate() {
		t.Error("hidden required field should not block validation")
	}
}

func TestForm_validateClearsOldErrors(t *testing.T) {
	f := NewForm("Test", "Submit")
	f.AddTextField("Name", true)
	f.FocusFirst()

	f.Validate() // fails, sets error
	typeText(&f, "Bob")
	f.Validate() // should clear error

	if f.Fields[0].Error != "" {
		t.Errorf("expected error to be cleared after valid input, got %q", f.Fields[0].Error)
	}
}

// ── Values ────────────────────────────────────────────────────────────────────

func TestForm_values(t *testing.T) {
	f := NewForm("Test", "Submit")
	f.AddTextField("Name", true)
	f.AddSelectField("Type", []string{"income", "expense"}, true)
	f.AddToggleField("Active?")
	f.FocusFirst()

	typeText(&f, "Alice")
	f.Focus(1)
	f, _ = f.Update(downKey()) // select "expense"
	f.Focus(2)
	f, _ = f.Update(spaceKey()) // toggle on

	vals := f.Values()
	if vals["Name"] != "Alice" {
		t.Errorf("Name: got %q, want %q", vals["Name"], "Alice")
	}
	if vals["Type"] != "expense" {
		t.Errorf("Type: got %q, want %q", vals["Type"], "expense")
	}
	if vals["Active?"] != "true" {
		t.Errorf("Active?: got %q, want %q", vals["Active?"], "true")
	}
}

// ── Reset ─────────────────────────────────────────────────────────────────────

func TestForm_reset(t *testing.T) {
	f := NewForm("Test", "Submit")
	f.AddTextField("Name", true)
	f.AddSelectField("Type", []string{"a", "b"}, true)
	f.AddToggleField("Flag")
	f.FocusFirst()

	typeText(&f, "Alice")
	f.Focus(1)
	f, _ = f.Update(downKey())
	f.Focus(2)
	f, _ = f.Update(spaceKey())
	f.Fields[0].Error = "some error"

	f.Reset()

	if f.Fields[0].Input.Value() != "" {
		t.Error("text field should be empty after reset")
	}
	if f.Fields[1].SelectedIdx != 0 {
		t.Error("select field should be at index 0 after reset")
	}
	if f.Fields[2].Value {
		t.Error("toggle field should be false after reset")
	}
	if f.Fields[0].Error != "" {
		t.Error("errors should be cleared after reset")
	}
	if f.FocusIdx != 0 {
		t.Error("focus should return to 0 after reset")
	}
}

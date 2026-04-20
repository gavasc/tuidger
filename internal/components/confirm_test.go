package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func key(s string) tea.Msg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func escKey() tea.Msg {
	return tea.KeyMsg{Type: tea.KeyEsc}
}

func TestConfirm_yConfirms(t *testing.T) {
	c := NewConfirm(42, "Delete item?")
	c, cmd := c.Update(key("y"))

	if c.Active {
		t.Error("confirm should be inactive after 'y'")
	}
	if cmd == nil {
		t.Fatal("expected a command after 'y'")
	}
	msg := cmd()
	result, ok := msg.(ConfirmResultMsg)
	if !ok {
		t.Fatalf("expected ConfirmResultMsg, got %T", msg)
	}
	if !result.Confirmed {
		t.Error("expected Confirmed=true after 'y'")
	}
	if result.ID != 42 {
		t.Errorf("ID: got %d, want 42", result.ID)
	}
}

func TestConfirm_YConfirms(t *testing.T) {
	c := NewConfirm(1, "Delete?")
	c, cmd := c.Update(key("Y"))
	if c.Active {
		t.Error("confirm should be inactive after 'Y'")
	}
	msg := cmd().(ConfirmResultMsg)
	if !msg.Confirmed {
		t.Error("expected Confirmed=true for uppercase Y")
	}
}

func TestConfirm_nCancels(t *testing.T) {
	c := NewConfirm(7, "Delete?")
	c, cmd := c.Update(key("n"))

	if c.Active {
		t.Error("confirm should be inactive after 'n'")
	}
	msg := cmd().(ConfirmResultMsg)
	if msg.Confirmed {
		t.Error("expected Confirmed=false after 'n'")
	}
}

func TestConfirm_NCancels(t *testing.T) {
	c := NewConfirm(7, "Delete?")
	c, cmd := c.Update(key("N"))
	msg := cmd().(ConfirmResultMsg)
	if msg.Confirmed {
		t.Error("expected Confirmed=false after 'N'")
	}
}

func TestConfirm_escCancels(t *testing.T) {
	c := NewConfirm(3, "Delete?")
	c, cmd := c.Update(escKey())

	if c.Active {
		t.Error("confirm should be inactive after Esc")
	}
	msg := cmd().(ConfirmResultMsg)
	if msg.Confirmed {
		t.Error("expected Confirmed=false after Esc")
	}
}

func TestConfirm_otherKeysIgnored(t *testing.T) {
	c := NewConfirm(5, "Delete?")
	c, cmd := c.Update(key("x"))

	if !c.Active {
		t.Error("confirm should still be active after unrecognised key")
	}
	if cmd != nil {
		t.Error("expected no command for unrecognised key")
	}
}

func TestConfirm_inactiveIgnoresInput(t *testing.T) {
	c := NewConfirm(5, "Delete?")
	c.Active = false
	c, cmd := c.Update(key("y"))

	if c.Active {
		t.Error("inactive confirm should stay inactive")
	}
	if cmd != nil {
		t.Error("inactive confirm should emit no command")
	}
}

func TestConfirm_viewEmptyWhenInactive(t *testing.T) {
	c := NewConfirm(1, "Delete?")
	c.Active = false
	if v := c.View(); v != "" {
		t.Errorf("expected empty view when inactive, got %q", v)
	}
}

func TestConfirm_viewNonEmptyWhenActive(t *testing.T) {
	c := NewConfirm(1, "Delete this item?")
	v := c.View()
	if v == "" {
		t.Error("expected non-empty view when active")
	}
	// message should appear somewhere in the output (stripped of ANSI)
	if !containsAnsi(v, "Delete this item?") {
		t.Errorf("confirm view should contain the message, got %q", v)
	}
}

// containsAnsi checks if the plain text of s (ignoring ANSI codes) contains sub.
func containsAnsi(s, sub string) bool {
	// crude: just check raw string since lipgloss embeds text alongside ANSI
	return len(s) > 0 && (len(sub) == 0 || contains(s, sub))
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

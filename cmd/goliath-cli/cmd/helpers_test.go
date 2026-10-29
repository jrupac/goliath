package cmd

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// The prompts here stand in front of commands that destroy things -- dropping
// feeds, removing a user, rolling a schema back, replacing a deployment. What
// they return decides whether that goes ahead, so the keys that answer them
// are worth holding still: a prompt that silently stopped collecting input
// would read as a confirmation that was never given.

func typeRunes(m tea.Model, s string) tea.Model {
	for _, r := range s {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func TestTextInputCollectsTypingUntilEnter(t *testing.T) {
	m := typeRunes(tea.Model(initialTextInputModel("Type DELETE to confirm:")), "DELETE")

	got := m.(textInputModel)
	if v := got.textInput.Value(); v != "DELETE" {
		t.Errorf("value after typing = %q, want %q", v, "DELETE")
	}
	if v := got.View(); !strings.Contains(v, "Type DELETE to confirm:") {
		t.Errorf("view does not show the prompt: %q", v)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.(textInputModel).quitting {
		t.Error("enter did not finish the prompt")
	}
}

// Backing out has to answer with nothing rather than with whatever had been
// typed so far, or escaping a confirmation could confirm it.
func TestTextInputEscapeAnswersEmpty(t *testing.T) {
	m := typeRunes(tea.Model(initialTextInputModel("confirm:")), "yes")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if v := m.(textInputModel).textInput.Value(); v != "" {
		t.Errorf("value after escape = %q, want empty", v)
	}
}

func TestPasswordPromptKeepsTheValueOffScreen(t *testing.T) {
	m := typeRunes(tea.Model(newTextInputModel("Password:", true)), "correct horse")

	got := m.(textInputModel)
	if got.textInput.EchoMode != textinput.EchoPassword {
		t.Error("password prompt is not hiding what is typed")
	}
	if v := got.textInput.Value(); v != "correct horse" {
		t.Errorf("value = %q, want %q", v, "correct horse")
	}
	if v := got.View(); strings.Contains(v, "correct horse") {
		t.Errorf("password appears in the rendered prompt: %q", v)
	}
}

// Only what the cursor was on when space was pressed comes back, so that a
// list of things to delete never includes a neighbour.
func TestChecklistReturnsOnlyMarkedChoices(t *testing.T) {
	m := tea.Model(initialChecklistModel("Select:", []string{"alpha", "beta", "gamma"}))

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})

	got := m.(checklistModel)
	if len(got.selected) != 1 {
		t.Fatalf("selected %d choices, want 1: %v", len(got.selected), got.selected)
	}
	if _, ok := got.selected[1]; !ok {
		t.Errorf("selected %v, want only the choice under the cursor", got.selected)
	}
}

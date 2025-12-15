package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jrupac/goliath/admin"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// --- Form input ---

type formField struct {
	prompt   string
	input    textinput.Model
	required bool
}

type formModel struct {
	fields     []formField
	focusIndex int
	submitted  bool
	quitting   bool
}

func (m formModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m formModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok && msg.Type == tea.KeyCtrlC {
		m.quitting = true
		return m, tea.Quit
	}

	// Handle character input and blinking
	cmd := m.updateInputs(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if m.focusIndex == len(m.fields) {
				// Submitted
				m.submitted = true
				m.quitting = true
				return m, tea.Quit
			}
			// Fallthrough to next input
			m.nextInput()

		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit

		case tea.KeyTab, tea.KeyShiftTab, tea.KeyUp, tea.KeyDown:
			s := msg.String()

			if s == "up" || s == "shift+tab" {
				m.prevInput()
			}
			if s == "down" || s == "tab" {
				m.nextInput()
			}
		}
	}

	return m, cmd
}

func (m *formModel) updateInputs(msg tea.Msg) tea.Cmd {
	var cmds = make([]tea.Cmd, len(m.fields))

	for i := range m.fields {
		m.fields[i].input, cmds[i] = m.fields[i].input.Update(msg)
	}

	return tea.Batch(cmds...)
}

func (m *formModel) nextInput() {
	m.focusIndex = (m.focusIndex + 1) % (len(m.fields) + 1) // +1 for submit button
	if m.focusIndex == len(m.fields) {
		// Focused on submit button
		for i := range m.fields {
			m.fields[i].input.Blur()
		}
		return
	}
	for i := range m.fields {
		if i == m.focusIndex {
			m.fields[i].input.Focus()
		} else {
			m.fields[i].input.Blur()
		}
	}
}

func (m *formModel) prevInput() {
	m.focusIndex--
	if m.focusIndex < 0 {
		m.focusIndex = len(m.fields)
	}
	if m.focusIndex == len(m.fields) {
		// Focused on submit button
		for i := range m.fields {
			m.fields[i].input.Blur()
		}
		return
	}
	for i := range m.fields {
		if i == m.focusIndex {
			m.fields[i].input.Focus()
		} else {
			m.fields[i].input.Blur()
		}
	}
}

func (m formModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	for i := range m.fields {
		b.WriteString(m.fields[i].input.View())
		if i < len(m.fields)-1 {
			b.WriteRune('\n')
		}
	}

	button := "[ Submit ]"
	if m.focusIndex == len(m.fields) {
		button = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("[ Submit ]")
	}

	fmt.Fprintf(&b, "\n\n%s\n", button)

	return b.String()
}

func promptForForm(fields []formField) (map[string]string, bool) {
	var inputs []textinput.Model
	for i, f := range fields {
		input := textinput.New()
		placeholder := f.prompt
		if !f.required {
			placeholder += " (optional)"
		}
		input.Placeholder = placeholder
		input.Width = 50
		if i == 0 {
			input.Focus()
		}
		fields[i].input = input
		inputs = append(inputs, input)
	}

	program := tea.NewProgram(formModel{fields: fields})
	m, err := program.Run()
	if err != nil {
		fmt.Printf("Error running form prompt: %v\n", err)
		os.Exit(1)
	}

	finalModel, ok := m.(formModel)
	if !ok || !finalModel.submitted {
		return nil, false // Canceled
	}

	results := make(map[string]string)
	for _, f := range finalModel.fields {
		results[f.prompt] = f.input.Value()
	}

	return results, true
}

// --- Checklist input ---

type checklistModel struct {
	prompt   string
	choices  []string
	cursor   int
	selected map[int]struct{}
}

func initialChecklistModel(prompt string, choices []string) checklistModel {
	return checklistModel{
		prompt:   prompt,
		choices:  choices,
		selected: make(map[int]struct{}),
	}
}

func (m checklistModel) Init() tea.Cmd {
	return nil
}

func (m checklistModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			// Clear selection on quit to indicate cancellation
			m.selected = make(map[int]struct{})
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}

		case " ": // Space toggles selection
			_, ok := m.selected[m.cursor]
			if ok {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = struct{}{}
			}

		case "enter": // Enter confirms and quits
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m checklistModel) View() string {
	var s strings.Builder
	s.WriteString(m.prompt + "\n\n")

	for i, choice := range m.choices {
		cursor := " " // no cursor
		if m.cursor == i {
			cursor = ">" // cursor!
		}

		checked := " " // not selected
		if _, ok := m.selected[i]; ok {
			checked = "x" // selected!
		}

		s.WriteString(fmt.Sprintf("%s [%s] %s\n", cursor, checked, choice))
	}

	s.WriteString("\nPress Space to toggle, Enter to confirm, q to quit.\n")

	return s.String()
}

func promptForChecklist(prompt string, choices []string) []string {
	p := tea.NewProgram(initialChecklistModel(prompt, choices))

	m, err := p.Run()
	if err != nil {
		fmt.Printf("Error running prompt: %v\n", err)
		os.Exit(1)
	}

	finalModel, ok := m.(checklistModel)
	if !ok {
		fmt.Println("Error getting final model from prompt")
		os.Exit(1)
	}

	var selectedChoices []string
	for i := range finalModel.selected {
		selectedChoices = append(selectedChoices, finalModel.choices[i])
	}

	return selectedChoices
}

// --- Multi-line text input ---

type textAreaModel struct {
	textarea textarea.Model
	quitting bool
	err      error
}

func initialTextAreaModel() textAreaModel {
	ta := textarea.New()
	ta.Placeholder = "politics, gossip, spoilers..."
	ta.Focus()

	return textAreaModel{
		textarea: ta,
	}
}

func (m textAreaModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m textAreaModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyCtrlD:
			m.quitting = true
			return m, tea.Quit
		}
	}

	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m textAreaModel) View() string {
	if m.quitting {
		return ""
	}
	return fmt.Sprintf(
		"Enter words to add (one per line). Press Ctrl+D or Esc when finished.\n\n%s",
		m.textarea.View(),
	) + "\n"
}

func promptForMultiline() []string {
	p := tea.NewProgram(initialTextAreaModel())

	m, err := p.Run()
	if err != nil {
		fmt.Printf("Error running prompt: %v\n", err)
		os.Exit(1)
	}

	finalModel, ok := m.(textAreaModel)
	if !ok {
		fmt.Println("Error getting final model from prompt")
		os.Exit(1)
	}

	// Split by newline and filter out empty strings
	var lines []string
	for _, line := range strings.Split(finalModel.textarea.Value(), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, strings.TrimSpace(line))
		}
	}

	return lines
}

type (
	errMsg error
)

// Bubbletea model for a single text input
type textInputModel struct {
	prompt    string
	textInput textinput.Model
	quitting  bool
	err       error
}

func initialTextInputModel(prompt string) textInputModel {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 50

	return textInputModel{
		prompt:    prompt,
		textInput: ti,
		err:       nil,
	}
}

func (m textInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m textInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			m.textInput.SetValue("") // Clear input on exit
			return m, tea.Quit
		}

	case errMsg:
		m.err = msg
		return m, nil
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m textInputModel) View() string {
	if m.quitting {
		return ""
	}
	return fmt.Sprintf(
		"%s\n\n%s\n\n%s",
		m.prompt,
		m.textInput.View(),
		"(esc to quit)",
	) + "\n"
}

// Public function to run the bubbletea prompt
func promptForInput(prompt string) string {
	p := tea.NewProgram(initialTextInputModel(prompt))

	m, err := p.Run()
	if err != nil {
		fmt.Printf("Error running prompt: %v\n", err)
		os.Exit(1)
	}

	// Type assertion to get the final model
	finalModel, ok := m.(textInputModel)
	if !ok {
		fmt.Println("Error getting final model from prompt")
		os.Exit(1)
	}

	return finalModel.textInput.Value()
}

func executeDockerCompose(args []string) {
	fmt.Println("Running: docker", args)
	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error running docker command: %v\n", err)
		os.Exit(1)
	}
}

func runDockerUp(env string, attached bool, build bool, extraArgs []string) {
	dockerArgs := []string{"compose", "--profile", env, "up"}
	if build {
		dockerArgs = append(dockerArgs, "--build")

		if env == "prod" {
			buildTimestamp, err := exec.Command("date", "+%s").Output()
			if err != nil {
				fmt.Printf("Error getting build timestamp: %v\n", err)
				os.Exit(1)
			}
			os.Setenv("BUILD_TIMESTAMP", strings.TrimSpace(string(buildTimestamp)))

			buildHash, err := exec.Command("git", "rev-parse", "HEAD").Output()
			if err != nil {
				fmt.Printf("Error getting build hash: %v\n", err)
				os.Exit(1)
			}
			os.Setenv("BUILD_HASH", strings.TrimSpace(string(buildHash)))
		}
	}
	if !attached {
		dockerArgs = append(dockerArgs, "-d")
	}
	dockerArgs = append(dockerArgs, extraArgs...)
	executeDockerCompose(dockerArgs)
}

func runDockerDown(env string, extraArgs []string) {
	dockerArgs := []string{"compose", "--profile", env, "down"}
	dockerArgs = append(dockerArgs, extraArgs...)
	executeDockerCompose(dockerArgs)
}

func addEnvFlag(cmd *cobra.Command) {
	cmd.Flags().String("env", "prod", "Environment ('prod', 'dev', or 'debug')")
}

func addAttachedFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("attached", false, "Starts containers in attached mode")
}

func addBuildFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("build", false, "Build images before starting containers")
}

func addUserFlag(cmd *cobra.Command) {
	cmd.Flags().String("user", "", "The user for whom this operation should be performed")
}

func getUser(cmd *cobra.Command) string {
	user, _ := cmd.Flags().GetString("user")
	if user == "" {
		user = promptForInput("Enter User:")
	}
	return user
}

func addGrpcAddressFlag(cmd *cobra.Command) {
	cmd.Flags().String("grpc-address", "localhost:9997", "Address of the gRPC server")
}

func getAdminClient(cmd *cobra.Command) (admin.AdminServiceClient, *grpc.ClientConn) {
	address, _ := cmd.Flags().GetString("grpc-address")
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Printf("Error connecting to gRPC server: %v\n", err)
		os.Exit(1)
	}

	client := admin.NewAdminServiceClient(conn)
	return client, conn
}

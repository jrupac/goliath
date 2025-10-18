package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jrupac/goliath/admin"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ... (existing content) ...

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

// ... (rest of the file is the same)

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

package fetch

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
	"strings"
	"testing"

	"github.com/jrupac/goliath/utils"
)

// mockCommandRunner is a mock implementation of the CommandRunner interface for testing.
type mockCommandRunner struct {
	mockOutput []byte
	mockError  error
}

func (m mockCommandRunner) Output(command string, args ...string) ([]byte, error) {
	return m.mockOutput, m.mockError
}

func TestMaybeParseArticleContent(t *testing.T) {
	// Save original values and restore them after the test.
	originalCmdRunner := cmdRunner
	originalParseArticles := *parseArticles
	originalMercuryCli := *mercuryCli
	defer func() {
		cmdRunner = originalCmdRunner
		*parseArticles = originalParseArticles
		*mercuryCli = originalMercuryCli
	}()

	successResponse := mercuryResponse{
		Content: "<h1>Hello</h1><p>World</p>",
	}
	successOutput, _ := json.Marshal(successResponse)

	// Build expected output for success case dynamically
	var expectedBuf bytes.Buffer
	tmpl, _ := template.New("parsedContent").Parse(contentTemplate)
	_ = tmpl.Execute(&expectedBuf, mercuryResponse{HtmlContent: template.HTML(successResponse.Content)})
	expectedSuccessOutput := expectedBuf.String()

	testCases := []struct {
		name           string
		parseArticles  bool
		mercuryCli     string
		mockRunner     utils.CommandRunner
		expectedOutput string
	}{
		{
			name:           "parsing disabled",
			parseArticles:  false,
			expectedOutput: "",
		},
		{
			name:           "mercury cli not set",
			parseArticles:  true,
			mercuryCli:     "",
			expectedOutput: "",
		},
		{
			name:          "command execution fails",
			parseArticles: true,
			mercuryCli:    "/usr/bin/true",
			mockRunner: mockCommandRunner{
				mockError: errors.New("command failed"),
			},
			expectedOutput: "",
		},
		{
			name:          "invalid json output",
			parseArticles: true,
			mercuryCli:    "/usr/bin/true",
			mockRunner: mockCommandRunner{
				mockOutput: []byte("this is not json"),
			},
			expectedOutput: "",
		},
		{
			name:          "empty content in response",
			parseArticles: true,
			mercuryCli:    "/usr/bin/true",
			mockRunner: mockCommandRunner{
				mockOutput: []byte(`{"content": ""}`),
			},
			expectedOutput: "",
		},
		{
			name:          "successful parsing",
			parseArticles: true,
			mercuryCli:    "/usr/bin/true",
			mockRunner: mockCommandRunner{
				mockOutput: successOutput,
			},
			expectedOutput: expectedSuccessOutput,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			*parseArticles = tc.parseArticles
			*mercuryCli = tc.mercuryCli
			if tc.mockRunner != nil {
				cmdRunner = tc.mockRunner
			} else {
				cmdRunner = mockCommandRunner{} // Default mock runner
			}

			result := maybeParseArticleContent("http://example.com")

			if strings.TrimSpace(result) != strings.TrimSpace(tc.expectedOutput) {
				t.Errorf("expected %q, got %q", tc.expectedOutput, result)
			}
		})
	}
}

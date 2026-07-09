package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/buildwithgo/berrygem/providers"
	"github.com/buildwithgo/berrygem/tools"
)

// ChoiceOption is one option in a ChoicePrompt.
type ChoiceOption struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// ChoicePrompt is a closed-ended question the agent asked, waiting on the
// user's answer via SubmitChoice. ID is the chat_messages.id of the
// persisted "choice_prompt" row — the same value the client must send back
// as SubmitChoiceRequest.choice_id.
type ChoicePrompt struct {
	ID          string
	Question    string
	Options     []ChoiceOption
	AllowCustom bool
}

// newAskChoiceTool builds the generic "ask the user a closed-ended
// question" tool. berrygem's providers.Property has no array/object
// nesting, so `options` is declared as a JSON-encoded string parameter —
// the model must emit a JSON array of {"key","label","description"}
// objects as a string, which this tool parses.
//
// askChoice is called synchronously from within Execute; the caller
// (Service.runTurn) is responsible for persisting the prompt and
// cancelling the run's context after this returns nil.
func newAskChoiceTool(askChoice func(question string, options []ChoiceOption, allowCustom bool) error) tools.Tool {
	return tools.NewFunc(
		"ask_choice",
		"Ask the user a closed-ended question with a small set of options and wait for their answer. "+
			"Use this whenever you need the user to confirm or pick among specific choices before proceeding "+
			"(for example, before calling create_workbench). This ends your turn immediately — do not produce "+
			"any further text or tool calls after invoking it; the conversation resumes automatically once the "+
			"user answers.",
		map[string]providers.Property{
			"question": {
				Type:        "string",
				Description: "The question to ask the user.",
			},
			"options": {
				Type: "string",
				Description: `A JSON array of options, e.g. ` +
					`[{"key":"A","label":"Yes","description":"optional detail"},{"key":"B","label":"No"}]. ` +
					`Each option needs "key" and "label"; "description" is optional. Must not be empty.`,
			},
			"allow_custom": {
				Type:        "boolean",
				Description: "Whether the user may type a free-form answer instead of picking an option. Defaults to false.",
			},
		},
		[]string{"question", "options"},
		func(_ context.Context, args string) (string, error) {
			var parsed struct {
				Question    string `json:"question"`
				Options     string `json:"options"`
				AllowCustom bool   `json:"allow_custom"`
			}
			if err := json.Unmarshal([]byte(args), &parsed); err != nil {
				return "", fmt.Errorf("ask_choice: invalid arguments: %w", err)
			}
			var options []ChoiceOption
			if err := json.Unmarshal([]byte(parsed.Options), &options); err != nil {
				return "", fmt.Errorf("ask_choice: options is not a valid JSON array: %w", err)
			}
			if len(options) == 0 {
				return "", fmt.Errorf("ask_choice: options must not be empty")
			}
			if err := askChoice(parsed.Question, options, parsed.AllowCustom); err != nil {
				return "", err
			}
			return "Question sent to the user. Waiting for their answer.", nil
		},
	)
}

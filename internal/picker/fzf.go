package picker

import (
	"fmt"

	fzf "github.com/junegunn/fzf/src"
)

func fzfPick(items []string, opts Options) (string, error) {
	if len(items) == 0 {
		return "", fmt.Errorf("no items to pick from")
	}

	prompt := opts.Prompt
	if prompt == "" {
		prompt = "directory> "
	}
	fzfArgs := []string{
		"--layout=reverse",
		"--prompt", prompt,
		"--no-multi",
	}
	if opts.Query != "" {
		fzfArgs = append(fzfArgs, "--query", opts.Query)
	}

	fzfOpts, err := fzf.ParseOptions(true, fzfArgs)
	if err != nil {
		return "", fmt.Errorf("parsing fzf options: %w", err)
	}

	inputChan := make(chan string)
	outputChan := make(chan string, 1)
	fzfOpts.Input = inputChan
	fzfOpts.Output = outputChan

	go func() {
		for _, item := range items {
			inputChan <- item
		}
		close(inputChan)
	}()

	code, err := fzf.Run(fzfOpts)
	if err != nil {
		return "", fmt.Errorf("fzf error: %w", err)
	}

	if code == 130 {
		return "", ErrCancelled
	}
	if code != 0 {
		return "", fmt.Errorf("fzf exited with code %d", code)
	}

	select {
	case result := <-outputChan:
		return result, nil
	default:
		return "", ErrCancelled
	}
}

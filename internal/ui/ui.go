package ui

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Result struct {
	Key    string
	Choice string
}

func RunFZF(items []string, prompt string, previewCmd string, expectKeys []string) (*Result, error) {
	args := []string{
		"--prompt=" + prompt,
		"--preview-window=right:60%:wrap",
	}
	
	if previewCmd != "" {
		args = append(args, "--preview", previewCmd)
	}
	
	if len(expectKeys) > 0 {
		args = append(args, fmt.Sprintf("--expect=%s", strings.Join(expectKeys, ",")))
	}
	
	cmd := exec.Command("fzf", args...)
	
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	
	go func() {
		defer stdin.Close()
		for _, item := range items {
			fmt.Fprintln(stdin, item)
		}
	}()
	
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr // FZF UI goes to stderr
	
	if err := cmd.Run(); err != nil {
		// FZF returns non-zero if no match or cancelled?
		// exit status 1 empty list, 2 error, 130 interrupt
	}
	
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) == 0 {
		return nil, nil // Cancelled
	}
	
	res := &Result{}
	if len(expectKeys) > 0 {
		if len(lines) >= 2 {
			res.Key = lines[0]
			res.Choice = lines[1]
		} else {
			res.Key = lines[0] // user just pressed key without selection?
		}
	} else {
		res.Choice = lines[0]
	}
	
	return res, nil
}

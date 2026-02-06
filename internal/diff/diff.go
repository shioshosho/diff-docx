package diff

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

// ShowDiff displays the diff between two files using delta
func ShowDiff(file1, file2 string) error {
	cmd := exec.Command("delta", file1, file2)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return nil
			}
		}
		return fmt.Errorf("delta failed: %w", err)
	}

	return nil
}

// ShowDiffWithFallback tries delta first, falls back to diff
func ShowDiffWithFallback(file1, file2 string) error {
	if _, err := exec.LookPath("delta"); err != nil {
		return showStandardDiff(file1, file2)
	}
	return ShowDiff(file1, file2)
}

func showStandardDiff(file1, file2 string) error {
	cmd := exec.Command("diff", "-u", "--color=auto", file1, file2)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return nil
			}
		}
		return fmt.Errorf("diff failed: %w", err)
	}

	return nil
}

// GenerateDiffFile writes a unified diff of two files to outputPath
func GenerateDiffFile(file1, file2, outputPath string) error {
	cmd := exec.Command("diff", "-u", file1, file2)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() > 1 {
				return fmt.Errorf("diff failed: %w", err)
			}
		} else {
			return fmt.Errorf("diff failed: %w", err)
		}
	}

	var wrapped bytes.Buffer
	wrapped.WriteString("```diff\n")
	wrapped.Write(stdout.Bytes())
	if wrapped.Len() > 0 && wrapped.Bytes()[wrapped.Len()-1] != '\n' {
		wrapped.WriteByte('\n')
	}
	wrapped.WriteString("```\n")

	return os.WriteFile(outputPath, wrapped.Bytes(), 0644)
}

// CheckDependencies checks if required external tools are available
func CheckDependencies() error {
	tools := []string{"markitdown", "delta", "magick"}
	var missing []string

	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			missing = append(missing, tool)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required tools: %v\nPlease install them before using ddx", missing)
	}

	return nil
}

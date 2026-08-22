package mutation

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

type mutationResult struct {
	path      string
	total     int
	covered   int
	uncovered int
	selected  int
	killed    int
	survived  int
	executed  bool
}

func parseMutationReport(payload []byte) (mutationResult, error) {
	result := mutationResult{}
	scanner := bufio.NewScanner(bytes.NewReader(payload))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case strings.HasPrefix(line, "Mutation run: "):
			result.path = strings.TrimPrefix(line, "Mutation run: ")
			result.executed = true
		case strings.HasPrefix(line, "Mutation scan: "):
			result.path = strings.TrimPrefix(line, "Mutation scan: ")
		case strings.HasPrefix(line, "Total mutation sites: "):
			value, err := parseMutationCount(line, "Total mutation sites: ")
			if err != nil {
				return mutationResult{}, err
			}
			result.total = value
		case strings.HasPrefix(line, "Covered mutation sites: "):
			value, err := parseMutationCount(line, "Covered mutation sites: ")
			if err != nil {
				return mutationResult{}, err
			}
			result.covered = value
		case strings.HasPrefix(line, "Uncovered mutation sites: "):
			value, err := parseMutationCount(line, "Uncovered mutation sites: ")
			if err != nil {
				return mutationResult{}, err
			}
			result.uncovered = value
		case strings.HasPrefix(line, "Selected mutation sites: "):
			value, err := parseMutationCount(line, "Selected mutation sites: ")
			if err != nil {
				return mutationResult{}, err
			}
			result.selected = value
		case strings.HasPrefix(line, "Killed: "):
			value, err := parseMutationCount(line, "Killed: ")
			if err != nil {
				return mutationResult{}, err
			}
			result.killed = value
		case strings.HasPrefix(line, "Survived: "):
			value, err := parseMutationCount(line, "Survived: ")
			if err != nil {
				return mutationResult{}, err
			}
			result.survived = value
		case strings.HasPrefix(line, "Uncovered: "):
			value, err := parseMutationCount(line, "Uncovered: ")
			if err != nil {
				return mutationResult{}, err
			}
			result.uncovered = value
		}
	}
	if err := scanner.Err(); err != nil {
		return mutationResult{}, fmt.Errorf("read mutate4go report: %w", err)
	}
	if result.path == "" {
		return mutationResult{}, fmt.Errorf("mutate4go report has no source path")
	}
	return result, nil
}

func parseMutationCount(line, prefix string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, prefix)))
	if err != nil {
		return 0, fmt.Errorf("parse mutate4go count from %q: %w", line, err)
	}
	return value, nil
}

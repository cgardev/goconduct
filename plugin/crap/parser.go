package crap

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/cgardev/goconduct/failure"
)

var reportRowPattern = regexp.MustCompile(
	`^(.+?)\s+(\S+)\s+([0-9]+)\s+([0-9]+(?:\.[0-9]+)?)%\s+([0-9]+(?:\.[0-9]+)?)$`,
)

type functionMetric struct {
	function   string
	packageID  string
	complexity int
	coverage   float64
	score      float64
}

func parseReport(payload []byte) ([]functionMetric, error) {
	metrics := make([]functionMetric, 0)
	for _, line := range strings.Split(string(payload), "\n") {
		matches := reportRowPattern.FindStringSubmatch(strings.TrimSpace(line))
		if matches == nil {
			continue
		}
		complexity, err := strconv.Atoi(matches[3])
		if err != nil {
			return nil, failure.DataIntegrity(fmt.Sprintf("parse complexity for %q", matches[1]), err)
		}
		coverage, err := strconv.ParseFloat(matches[4], 64)
		if err != nil {
			return nil, failure.DataIntegrity(fmt.Sprintf("parse coverage for %q", matches[1]), err)
		}
		score, err := strconv.ParseFloat(matches[5], 64)
		if err != nil {
			return nil, failure.DataIntegrity(fmt.Sprintf("parse CRAP score for %q", matches[1]), err)
		}
		metrics = append(metrics, functionMetric{
			function: matches[1], packageID: matches[2], complexity: complexity,
			coverage: coverage, score: score,
		})
	}
	slices.SortFunc(metrics, func(left, right functionMetric) int {
		if comparison := strings.Compare(left.packageID, right.packageID); comparison != 0 {
			return comparison
		}
		return strings.Compare(left.function, right.function)
	})
	return metrics, nil
}

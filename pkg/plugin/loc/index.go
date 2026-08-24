package loc

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/cgardev/goconduct/pkg/failure"
	"github.com/cgardev/goconduct/pkg/plugin"
)

type metricIndex struct {
	byID       map[string]plugin.Metric
	byNamePath map[string]plugin.Metric
	files      []FileOverview
	functions  []FunctionOverview
}

func newMetricIndex(report plugin.Report) (metricIndex, error) {
	index := metricIndex{
		byID:       make(map[string]plugin.Metric, len(report.Metrics)),
		byNamePath: make(map[string]plugin.Metric, len(report.Metrics)),
	}
	for _, metric := range report.Metrics {
		index.byID[metric.ID] = metric
		index.byNamePath[metricKey(metric.Name, metric.Path)] = metric
	}
	for _, metric := range report.Metrics {
		if metric.Name == "loc.file.files.total" {
			index.files = append(index.files, index.file(metric.Path))
		}
		if metric.Name != "loc.function.lines.total" {
			continue
		}
		function, err := index.function(metric)
		if err != nil {
			return metricIndex{}, err
		}
		index.functions = append(index.functions, function)
	}
	return index, nil
}

func (index metricIndex) aggregate(scope, repositoryPath string) AggregateOverview {
	prefix := "loc." + scope + "."
	totalLines := index.integer(prefix+"lines.total", repositoryPath)
	files := CountBreakdown{
		Total:     index.integer(prefix+"files.total", repositoryPath),
		Test:      index.integer(prefix+"files.test", repositoryPath),
		Generated: index.integer(prefix+"files.generated", repositoryPath),
	}
	lines := LineBreakdown{
		Total:     totalLines,
		Code:      index.integer(prefix+"lines.code", repositoryPath),
		Comment:   index.integer(prefix+"lines.comment", repositoryPath),
		Blank:     index.integer(prefix+"lines.blank", repositoryPath),
		Test:      index.integer(prefix+"lines.test", repositoryPath),
		Generated: index.integer(prefix+"lines.generated", repositoryPath),
	}
	functions := CountBreakdown{
		Total:     index.integer(prefix+"functions.total", repositoryPath),
		Test:      index.integer(prefix+"functions.test", repositoryPath),
		Generated: index.integer(prefix+"functions.generated", repositoryPath),
	}
	overlapFiles, overlapLines, overlapFunctions := index.generatedTestOverlap(scope, repositoryPath)
	files.Handwritten = files.Total - files.Test - files.Generated + overlapFiles
	lines.Handwritten = lines.Total - lines.Test - lines.Generated + overlapLines
	functions.Handwritten = functions.Total - functions.Test - functions.Generated + overlapFunctions
	lines.HandwrittenPercent = percentage(lines.Handwritten, lines.Total)
	lines.TestPercent = percentage(lines.Test, lines.Total)
	lines.GeneratedPercent = percentage(lines.Generated, lines.Total)
	return AggregateOverview{
		Path:      repositoryPath,
		Files:     files,
		Lines:     lines,
		Functions: functions,
		FunctionLines: FunctionLineBreakdown{
			Average: index.number(prefix+"function-lines.average", repositoryPath),
			P95:     index.integer(prefix+"function-lines.p95", repositoryPath),
			Maximum: index.integer(prefix+"function-lines.maximum", repositoryPath),
		},
	}
}

func (index metricIndex) file(repositoryPath string) FileOverview {
	totalLines := index.integer("loc.file.lines.total", repositoryPath)
	test := index.integer("loc.file.files.test", repositoryPath) == 1
	generated := index.integer("loc.file.files.generated", repositoryPath) == 1
	handwrittenLines := 0
	if !test && !generated {
		handwrittenLines = totalLines
	}
	return FileOverview{
		Path: repositoryPath, Test: test, Generated: generated,
		Lines: LineBreakdown{
			Total: totalLines, Code: index.integer("loc.file.lines.code", repositoryPath),
			Comment:            index.integer("loc.file.lines.comment", repositoryPath),
			Blank:              index.integer("loc.file.lines.blank", repositoryPath),
			Handwritten:        handwrittenLines,
			Test:               boolCount(test) * totalLines,
			Generated:          boolCount(generated) * totalLines,
			HandwrittenPercent: percentage(handwrittenLines, totalLines),
			TestPercent:        percentage(boolCount(test)*totalLines, totalLines),
			GeneratedPercent:   percentage(boolCount(generated)*totalLines, totalLines),
		},
		Functions: index.integer("loc.file.functions.total", repositoryPath),
	}
}

func (index metricIndex) function(totalMetric plugin.Metric) (FunctionOverview, error) {
	baseID := strings.TrimSuffix(totalMetric.ID, ":lines:total")
	identity := strings.TrimPrefix(baseID, "loc:function:")
	prefix := totalMetric.Path + ":"
	if identity == baseID || !strings.HasPrefix(identity, prefix) {
		return FunctionOverview{}, failure.DataIntegrity(
			fmt.Sprintf("LOC function metric %q has an invalid identifier", totalMetric.ID),
			nil,
		)
	}
	locationAndName := strings.TrimPrefix(identity, prefix)
	separator := strings.IndexByte(locationAndName, ':')
	if separator <= 0 || separator == len(locationAndName)-1 {
		return FunctionOverview{}, failure.DataIntegrity(
			fmt.Sprintf("LOC function metric %q has an invalid identity", totalMetric.ID),
			nil,
		)
	}
	startLine, err := strconv.Atoi(locationAndName[:separator])
	if err != nil {
		return FunctionOverview{}, failure.DataIntegrity(
			fmt.Sprintf("LOC function metric %q has an invalid start line", totalMetric.ID),
			err,
		)
	}
	file := index.file(totalMetric.Path)
	totalLines := int(totalMetric.Value)
	handwrittenLines := 0
	if !file.Test && !file.Generated {
		handwrittenLines = totalLines
	}
	return FunctionOverview{
		ID: identity, Name: locationAndName[separator+1:], Path: totalMetric.Path,
		StartLine: startLine, EndLine: int(index.byID[baseID+":end-line"].Value),
		Test: file.Test, Generated: file.Generated,
		Lines: LineBreakdown{
			Total: totalLines, Code: int(index.byID[baseID+":lines:code"].Value),
			Comment:            int(index.byID[baseID+":lines:comment"].Value),
			Blank:              int(index.byID[baseID+":lines:blank"].Value),
			Handwritten:        handwrittenLines,
			Test:               boolCount(file.Test) * totalLines,
			Generated:          boolCount(file.Generated) * totalLines,
			HandwrittenPercent: percentage(handwrittenLines, totalLines),
			TestPercent:        percentage(boolCount(file.Test)*totalLines, totalLines),
			GeneratedPercent:   percentage(boolCount(file.Generated)*totalLines, totalLines),
		},
	}, nil
}

func (index metricIndex) generatedTestOverlap(scope, repositoryPath string) (int, int, int) {
	files := 0
	lines := 0
	functions := 0
	for _, file := range index.files {
		if !file.Test || !file.Generated || !aggregateContains(scope, repositoryPath, file.Path) {
			continue
		}
		files++
		lines += file.Lines.Total
		functions += file.Functions
	}
	return files, lines, functions
}

func (index metricIndex) integer(name, repositoryPath string) int {
	return int(index.number(name, repositoryPath))
}

func (index metricIndex) number(name, repositoryPath string) float64 {
	return index.byNamePath[metricKey(name, repositoryPath)].Value
}

func metricKey(name, repositoryPath string) string {
	return name + "\x00" + repositoryPath
}

func aggregateContains(scope, aggregatePath, filePath string) bool {
	return scope == "repository" || scope == "package" && path.Dir(filePath) == aggregatePath
}

func boolCount(value bool) int {
	if value {
		return 1
	}
	return 0
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-23T21:34:41Z","module_hash":"fe1d0cdbe71744727f7fdee1e16fd92d5c6dca97f91c6cd43b8f8fb73e12f178","functions":[{"id":"func/newMetricIndex","name":"newMetricIndex","line":20,"end_line":43,"hash":"719bc4c8a1cdbee7671a8ecf9c7e3b4f79aa2788d0900f177ec021d9bdccca5d"},{"id":"func/metricIndex.aggregate","name":"metricIndex.aggregate","line":45,"end_line":84,"hash":"ee425d3b8d06062066d0dfafb6e2a59ab53fc34ab790c76084643d0942f5e673"},{"id":"func/metricIndex.file","name":"metricIndex.file","line":86,"end_line":109,"hash":"60cfa7cd5f3e6fb294438a8def92daeba8e5bdebb15d986cd1274057edff596b"},{"id":"func/metricIndex.function","name":"metricIndex.function","line":111,"end_line":158,"hash":"c01dbba5cee1ed2551577243bae28ae2c26b1dd2db05071b6db6ed307d7991fc"},{"id":"func/metricIndex.generatedTestOverlap","name":"metricIndex.generatedTestOverlap","line":160,"end_line":173,"hash":"50a4788add2c19829b37682a992d554425e474655440aa4b0931dcf429efb971"},{"id":"func/metricIndex.integer","name":"metricIndex.integer","line":175,"end_line":177,"hash":"0eaf27d5a95d395a18e15e31bd5c4816f38c84674dc9f830c46f5bf014d95789"},{"id":"func/metricIndex.number","name":"metricIndex.number","line":179,"end_line":181,"hash":"0b80c933127208af9f32a00e1e74c3ca367d2a07247e2d4b83fbc72fb619af69"},{"id":"func/metricKey","name":"metricKey","line":183,"end_line":185,"hash":"0dbf186394d7125cbdedb1dd1a65804ea1fb7c52245d6e68306cac2f33319f06"},{"id":"func/aggregateContains","name":"aggregateContains","line":187,"end_line":189,"hash":"6d681346ac0e3dea8f1e2f079789e188e0a8765439f4ff27269791fcff491cfb"},{"id":"func/boolCount","name":"boolCount","line":191,"end_line":196,"hash":"6cc79984cd85815ebd762c5f8b06c690507edb9f0b1db606fc72c5cf883c1a6e"}]}
// mutate4go-manifest-end

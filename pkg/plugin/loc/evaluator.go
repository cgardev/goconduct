package loc

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/cgardev/goconduct/internal/library/goloc"
	"github.com/cgardev/goconduct/internal/library/gosource"
	"github.com/cgardev/goconduct/pkg/failure"
	"github.com/cgardev/goconduct/pkg/plugin"
	"github.com/cgardev/goconduct/pkg/policy"
)

const ruleLOCThreshold = "loc-threshold"

type generatedClassifier struct {
	standardMarker   bool
	pathPatterns     *policy.PathSelector
	headerPatterns   []*regexp.Regexp
	forceHandwritten *policy.PathSelector
	hasPathPatterns  bool
	hasOverrides     bool
}

type pathSelector interface {
	Select(repositoryPath string) (bool, error)
}

type moduleDescriptor struct {
	directory string
	path      string
}

type measuredFile struct {
	path        string
	packageDir  string
	module      moduleDescriptor
	test        bool
	generated   bool
	measurement goloc.File
}

type aggregate struct {
	files              int
	testFiles          int
	generatedFiles     int
	lines              goloc.Lines
	testLines          int
	generatedLines     int
	functions          int
	testFunctions      int
	generatedFunctions int
	functionCodeLines  []int
}

// Evaluator measures physical Go lines and declared functions.
type Evaluator struct {
	configuration Configuration
	selector      pathSelector
	generated     generatedClassifier
	resolver      *policy.Resolver
}

var _ plugin.Evaluator = (*Evaluator)(nil)

// NewEvaluator validates configuration and creates a LOC evaluator.
func NewEvaluator(configuration Configuration) (*Evaluator, error) {
	if len(configuration.Selection.Paths) == 0 {
		return nil, failure.Validation("LOC selection paths are empty", nil)
	}
	for _, selectedPath := range configuration.Selection.Paths {
		if strings.TrimSpace(selectedPath) == "" {
			return nil, failure.Validation("LOC selection path is empty", nil)
		}
	}
	selector, err := policy.NewPathSelector(policy.PathSelection{
		Include: configuration.Selection.Include,
		Exclude: configuration.Selection.Exclude,
	})
	if err != nil {
		return nil, fmt.Errorf("validate LOC selection: %w", err)
	}
	generated, err := newGeneratedClassifier(configuration.Generated)
	if err != nil {
		return nil, err
	}
	resolver, err := policy.NewResolver(configuration.Policies)
	if err != nil {
		return nil, fmt.Errorf("validate LOC policies: %w", err)
	}
	return &Evaluator{
		configuration: cloneConfiguration(configuration),
		selector:      selector,
		generated:     generated,
		resolver:      resolver,
	}, nil
}

// Name returns the stable evaluator identifier.
func (*Evaluator) Name() string { return "loc" }

// Evaluate measures every selected Go file and returns normalized evidence.
func (evaluator *Evaluator) Evaluate(
	ctx context.Context,
	request plugin.Request,
) (plugin.Report, error) {
	root, err := filepath.Abs(cmp.Or(request.RepositoryRoot, "."))
	if err != nil {
		return plugin.Report{}, failure.Internal("resolve LOC repository root", err)
	}
	selected := evaluator.configuration.Selection.Paths
	if len(request.Paths) != 0 {
		selected = slices.Clone(request.Paths)
	}
	paths, err := gosource.AllFiles(root, selected)
	if err != nil {
		return plugin.Report{}, err
	}
	files, err := evaluator.measureFiles(ctx, root, paths)
	if err != nil {
		return plugin.Report{}, err
	}
	metrics := buildMetrics(files)
	findings, err := evaluator.findings(metrics)
	if err != nil {
		return plugin.Report{}, err
	}
	return plugin.NewReport("loc", metrics, findings)
}

func newGeneratedClassifier(configuration GeneratedConfiguration) (generatedClassifier, error) {
	pathPatterns, err := policy.NewPathSelector(policy.PathSelection{Include: configuration.PathPatterns})
	if err != nil {
		return generatedClassifier{}, fmt.Errorf("validate generated file paths: %w", err)
	}
	forceHandwritten, err := policy.NewPathSelector(policy.PathSelection{
		Include: configuration.ForceHandwrittenPaths,
	})
	if err != nil {
		return generatedClassifier{}, fmt.Errorf("validate handwritten file paths: %w", err)
	}
	headerPatterns := make([]*regexp.Regexp, 0, len(configuration.HeaderPatterns))
	for _, pattern := range configuration.HeaderPatterns {
		if strings.TrimSpace(pattern) == "" {
			return generatedClassifier{}, failure.Validation("generated header pattern is empty", nil)
		}
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return generatedClassifier{}, failure.Validation(
				fmt.Sprintf("compile generated header pattern %q", pattern),
				err,
			)
		}
		headerPatterns = append(headerPatterns, compiled)
	}
	return generatedClassifier{
		standardMarker:   configuration.StandardMarker,
		pathPatterns:     pathPatterns,
		headerPatterns:   headerPatterns,
		forceHandwritten: forceHandwritten,
		hasPathPatterns:  len(configuration.PathPatterns) != 0,
		hasOverrides:     len(configuration.ForceHandwrittenPaths) != 0,
	}, nil
}

func (classifier generatedClassifier) classify(repositoryPath string, file goloc.File) (bool, error) {
	if classifier.hasOverrides {
		handwritten, err := classifier.forceHandwritten.Select(repositoryPath)
		if err != nil {
			return false, err
		}
		if handwritten {
			return false, nil
		}
	}
	if classifier.hasPathPatterns {
		generated, err := classifier.pathPatterns.Select(repositoryPath)
		if err != nil {
			return false, err
		}
		if generated {
			return true, nil
		}
	}
	if classifier.standardMarker && file.StandardGenerated {
		return true, nil
	}
	for _, pattern := range classifier.headerPatterns {
		if pattern.MatchString(file.Header) {
			return true, nil
		}
	}
	return false, nil
}

func (evaluator *Evaluator) measureFiles(
	ctx context.Context,
	root string,
	paths []string,
) ([]measuredFile, error) {
	files := make([]measuredFile, 0, len(paths))
	modules := make(map[string]moduleDescriptor)
	for _, repositoryPath := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		selected, err := evaluator.selector.Select(repositoryPath)
		if err != nil {
			return nil, err
		}
		if !selected {
			continue
		}
		measurement, err := goloc.Analyze(filepath.Join(root, filepath.FromSlash(repositoryPath)))
		if err != nil {
			return nil, err
		}
		generated, err := evaluator.generated.classify(repositoryPath, measurement)
		if err != nil {
			return nil, err
		}
		module, err := locateModule(root, repositoryPath, modules)
		if err != nil {
			return nil, err
		}
		files = append(files, measuredFile{
			path:        repositoryPath,
			packageDir:  path.Dir(repositoryPath),
			module:      module,
			test:        strings.HasSuffix(repositoryPath, "_test.go"),
			generated:   generated,
			measurement: measurement,
		})
	}
	return files, nil
}

func locateModule(
	root string,
	repositoryPath string,
	cache map[string]moduleDescriptor,
) (moduleDescriptor, error) {
	directory := path.Dir(repositoryPath)
	var visited []string
	for {
		if cached, found := cache[directory]; found {
			for _, candidate := range visited {
				cache[candidate] = cached
			}
			return cached, nil
		}
		visited = append(visited, directory)
		moduleFile := filepath.Join(root, filepath.FromSlash(directory), "go.mod")
		payload, err := os.ReadFile(moduleFile)
		switch {
		case err == nil:
			modulePath := modfile.ModulePath(payload)
			if modulePath == "" {
				return moduleDescriptor{}, failure.DataIntegrity(
					fmt.Sprintf("module file %q declares no module path", repositoryModuleFile(directory)),
					nil,
				)
			}
			module := moduleDescriptor{directory: directory, path: modulePath}
			for _, candidate := range visited {
				cache[candidate] = module
			}
			return module, nil
		case !errors.Is(err, os.ErrNotExist):
			return moduleDescriptor{}, failure.Unavailable(
				fmt.Sprintf("read module file %q", repositoryModuleFile(directory)),
				err,
			)
		}
		if directory == "." {
			return moduleDescriptor{}, failure.Validation(
				fmt.Sprintf("Go source %q belongs to no module in the repository", repositoryPath),
				nil,
			)
		}
		directory = path.Dir(directory)
	}
}

func repositoryModuleFile(directory string) string {
	if directory == "." {
		return "go.mod"
	}
	return directory + "/go.mod"
}

func (summary *aggregate) add(file measuredFile) {
	summary.files++
	if file.test {
		summary.testFiles++
		summary.testLines += file.measurement.Lines.Total
	}
	if file.generated {
		summary.generatedFiles++
		summary.generatedLines += file.measurement.Lines.Total
	}
	summary.lines.Total += file.measurement.Lines.Total
	summary.lines.Code += file.measurement.Lines.Code
	summary.lines.Comment += file.measurement.Lines.Comment
	summary.lines.Blank += file.measurement.Lines.Blank
	for _, function := range file.measurement.Functions {
		summary.functions++
		if file.test {
			summary.testFunctions++
		}
		if file.generated {
			summary.generatedFunctions++
		}
		summary.functionCodeLines = append(summary.functionCodeLines, function.Lines.Code)
	}
}

func buildMetrics(files []measuredFile) []plugin.Metric {
	repository := aggregate{}
	modules := make(map[string]*aggregate)
	moduleDescriptors := make(map[string]moduleDescriptor)
	packages := make(map[string]*aggregate)
	var metrics []plugin.Metric
	for _, file := range files {
		repository.add(file)
		moduleKey := file.module.directory + "\x00" + file.module.path
		if modules[moduleKey] == nil {
			modules[moduleKey] = &aggregate{}
			moduleDescriptors[moduleKey] = file.module
		}
		modules[moduleKey].add(file)
		if packages[file.packageDir] == nil {
			packages[file.packageDir] = &aggregate{}
		}
		packages[file.packageDir].add(file)
		fileSummary := aggregate{}
		fileSummary.add(file)
		metrics = appendAggregateMetrics(metrics, "file", file.path, file.path, fileSummary)
		metrics = appendFunctionMetrics(metrics, file)
	}
	metrics = appendAggregateMetrics(metrics, "repository", "repository", "", repository)
	for _, key := range sortedKeys(modules) {
		descriptor := moduleDescriptors[key]
		identity := descriptor.directory + "@" + descriptor.path
		metrics = appendAggregateMetrics(
			metrics,
			"module",
			identity,
			repositoryModuleFile(descriptor.directory),
			*modules[key],
		)
	}
	for _, directory := range sortedKeys(packages) {
		metrics = appendAggregateMetrics(metrics, "package", directory, directory, *packages[directory])
	}
	return metrics
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func appendAggregateMetrics(
	metrics []plugin.Metric,
	scope string,
	identity string,
	repositoryPath string,
	summary aggregate,
) []plugin.Metric {
	prefix := "loc:" + scope + ":" + identity + ":"
	namePrefix := "loc." + scope + "."
	metrics = append(metrics,
		countMetric(prefix+"files:total", repositoryPath, namePrefix+"files.total", summary.files),
		countMetric(prefix+"files:test", repositoryPath, namePrefix+"files.test", summary.testFiles),
		countMetric(prefix+"files:generated", repositoryPath, namePrefix+"files.generated", summary.generatedFiles),
		lineMetric(prefix+"lines:total", repositoryPath, namePrefix+"lines.total", summary.lines.Total),
		lineMetric(prefix+"lines:code", repositoryPath, namePrefix+"lines.code", summary.lines.Code),
		lineMetric(prefix+"lines:comment", repositoryPath, namePrefix+"lines.comment", summary.lines.Comment),
		lineMetric(prefix+"lines:blank", repositoryPath, namePrefix+"lines.blank", summary.lines.Blank),
		lineMetric(prefix+"lines:test", repositoryPath, namePrefix+"lines.test", summary.testLines),
		lineMetric(prefix+"lines:generated", repositoryPath, namePrefix+"lines.generated", summary.generatedLines),
		countMetric(prefix+"functions:total", repositoryPath, namePrefix+"functions.total", summary.functions),
		countMetric(prefix+"functions:test", repositoryPath, namePrefix+"functions.test", summary.testFunctions),
		countMetric(
			prefix+"functions:generated",
			repositoryPath,
			namePrefix+"functions.generated",
			summary.generatedFunctions,
		),
		plugin.Metric{
			ID: prefix + "lines:generated:percent", Path: repositoryPath,
			Name:  namePrefix + "lines.generated.percent",
			Value: percentage(summary.generatedLines, summary.lines.Total), Unit: "percent",
		},
	)
	if scope == "repository" || scope == "module" || scope == "package" {
		average, percentile95, maximum := functionLineStatistics(summary.functionCodeLines)
		metrics = append(metrics,
			plugin.Metric{
				ID: prefix + "function-lines:average", Path: repositoryPath,
				Name: namePrefix + "function-lines.average", Value: average, Unit: "lines",
			},
			lineMetric(
				prefix+"function-lines:p95",
				repositoryPath,
				namePrefix+"function-lines.p95",
				percentile95,
			),
			lineMetric(
				prefix+"function-lines:maximum",
				repositoryPath,
				namePrefix+"function-lines.maximum",
				maximum,
			),
		)
	}
	return metrics
}

func appendFunctionMetrics(metrics []plugin.Metric, file measuredFile) []plugin.Metric {
	for _, function := range file.measurement.Functions {
		identity := file.path + ":" + strconv.Itoa(function.StartLine) + ":" + function.Name
		prefix := "loc:function:" + identity + ":"
		metrics = append(metrics,
			lineMetric(prefix+"lines:total", file.path, "loc.function.lines.total", function.Lines.Total),
			lineMetric(prefix+"lines:code", file.path, "loc.function.lines.code", function.Lines.Code),
			lineMetric(
				prefix+"lines:comment",
				file.path,
				"loc.function.lines.comment",
				function.Lines.Comment,
			),
			lineMetric(prefix+"lines:blank", file.path, "loc.function.lines.blank", function.Lines.Blank),
			countMetric(prefix+"start-line", file.path, "loc.function.start-line", function.StartLine),
			countMetric(prefix+"end-line", file.path, "loc.function.end-line", function.EndLine),
		)
	}
	return metrics
}

func countMetric(identifier, repositoryPath, name string, value int) plugin.Metric {
	return plugin.Metric{
		ID: identifier, Path: repositoryPath, Name: name, Value: float64(value), Unit: "count",
	}
}

func lineMetric(identifier, repositoryPath, name string, value int) plugin.Metric {
	return plugin.Metric{
		ID: identifier, Path: repositoryPath, Name: name, Value: float64(value), Unit: "lines",
	}
}

func percentage(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) * 100 / float64(denominator)
}

func functionLineStatistics(values []int) (float64, int, int) {
	if len(values) == 0 {
		return 0, 0, 0
	}
	ordered := slices.Clone(values)
	slices.Sort(ordered)
	total := 0
	for _, value := range ordered {
		total += value
	}
	percentileIndex := int(math.Ceil(float64(len(ordered))*0.95)) - 1
	return float64(total) / float64(len(ordered)), ordered[percentileIndex], ordered[len(ordered)-1]
}

func (evaluator *Evaluator) findings(metrics []plugin.Metric) ([]plugin.Finding, error) {
	findings := make([]plugin.Finding, 0)
	for _, metric := range metrics {
		if metric.Path == "" {
			continue
		}
		policyPath := metric.Path
		if policyPath == "." {
			policyPath = "go.mod"
		}
		threshold, found, err := evaluator.resolver.Resolve(policyPath, metric.Name)
		if err != nil {
			return nil, err
		}
		if !found || threshold.Passes(metric.Value) {
			continue
		}
		actual := metric.Value
		limit := threshold.Value
		findings = append(findings, plugin.Finding{
			ID:   "loc:" + threshold.PolicyID + ":" + metric.ID,
			Rule: ruleLOCThreshold, Path: metric.Path, Severity: threshold.Severity,
			Message: fmt.Sprintf(
				"metric %s is %.2f, outside the %s limit %.2f from policy %q",
				metric.Name,
				actual,
				threshold.Comparison,
				limit,
				threshold.PolicyID,
			),
			Actual: &actual,
			Limit:  &limit,
		})
	}
	return findings, nil
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-23T19:49:37Z","module_hash":"900942528601743a98e5d11e4c32fb13a782a509b0e0101a61aa69e2e0ae4e36","functions":[{"id":"func/NewEvaluator","name":"NewEvaluator","line":79,"end_line":109,"hash":"eae4d0191cd36914e1895f8ea066c18f6af149e84dcec66ff636dd8ca47516b0"},{"id":"func/Evaluator.Name","name":"Evaluator.Name","line":112,"end_line":112,"hash":"fbe2aa1b34843f7310eb5505ec9ba60f1be05e488d330e5ea412a6463d222669"},{"id":"func/Evaluator.Evaluate","name":"Evaluator.Evaluate","line":115,"end_line":141,"hash":"797e581780f1c1cc012425eed5979ef8e774a69a2135f118ccfea864401a5cb0"},{"id":"func/newGeneratedClassifier","name":"newGeneratedClassifier","line":143,"end_line":176,"hash":"ca2b4bcaecea1e53dc4b3f687d01604a86b6b376bead58250d6f7ef42bea25d6"},{"id":"func/generatedClassifier.classify","name":"generatedClassifier.classify","line":178,"end_line":206,"hash":"7aebf097eaacccb883bcc80bec832e6ac3bd23ad0d7be78462c389ef175c6e4f"},{"id":"func/Evaluator.measureFiles","name":"Evaluator.measureFiles","line":208,"end_line":248,"hash":"0cd984745a06726e9c6b19b95011ef7bac933c0ff32cf23426149d71ee7699d1"},{"id":"func/locateModule","name":"locateModule","line":250,"end_line":295,"hash":"9bd4e6ed1ec4642e30ebbaf4c034a4787c6d1af0d8d4b429e71029db1bf65053"},{"id":"func/repositoryModuleFile","name":"repositoryModuleFile","line":297,"end_line":302,"hash":"7b0036dec4c4e431abb495fd3c2d4fa3c7f10277996e99b2e7186afc33827668"},{"id":"func/aggregate.add","name":"aggregate.add","line":304,"end_line":328,"hash":"befbb28eb4e8168c46f0af9a99226f916b4a92cbd44cdef4976a4ab93ce69e04"},{"id":"func/buildMetrics","name":"buildMetrics","line":330,"end_line":369,"hash":"a4b83268f40be687fc221ec13173887d11084ee3f8157544ab97939adb67cbf2"},{"id":"func/sortedKeys","name":"sortedKeys","line":371,"end_line":378,"hash":"3e5d8f3558aecb1fbc37a76e05d0093b8ea24cd955bf51599a47fa28b2180259"},{"id":"func/appendAggregateMetrics","name":"appendAggregateMetrics","line":380,"end_line":435,"hash":"5a631b13b1e9d36dcdc9f99c85b612375b73f5ee96e8a57070836dbbdcf7fb61"},{"id":"func/appendFunctionMetrics","name":"appendFunctionMetrics","line":437,"end_line":456,"hash":"52e70ebe7a109b38a425548b24045b1f246553e870e90a660e65cd0fb727fcd2"},{"id":"func/countMetric","name":"countMetric","line":458,"end_line":462,"hash":"5a393213e9134544318c97f376448381584e36663f4b1b1a5d77633fabb81419"},{"id":"func/lineMetric","name":"lineMetric","line":464,"end_line":468,"hash":"876e54ac3c198a4ced1bbf52b2331fd89cef0ea3730a245aace4e12d27046e44"},{"id":"func/percentage","name":"percentage","line":470,"end_line":475,"hash":"094af0cda5cc904e9a53a10b631ea9769ec26b850dec349a4415456ad0d82956"},{"id":"func/functionLineStatistics","name":"functionLineStatistics","line":477,"end_line":489,"hash":"b9a48e6d52bd4903f4a741d8b0aedee0e6cc6f15f2287a17efecc5e1e87465b2"},{"id":"func/Evaluator.findings","name":"Evaluator.findings","line":491,"end_line":526,"hash":"8980ff3cd16b8aff9a0bcd3b47ddd9d8b9e1520f1e9bb9478308b7d7275a6f9a"}]}
// mutate4go-manifest-end

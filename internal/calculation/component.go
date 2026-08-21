package calculation

import (
	"cmp"
	"slices"
	"sort"
	"strings"

	"digginginsights.com/v3/internal/devtool/dependencygraph/internal/architecture"
)

const (
	stableLowAbstractionMaximumInstability  = 0.2
	stableLowAbstractionMaximumAbstractness = 0.2
)

type componentDescriptor struct {
	identifier  string
	name        string
	role        componentRole
	category    string
	application string
}

type sourceFile struct {
	relativePath    string
	packagePath     string
	component       componentDescriptor
	test            bool
	imports         []sourceImport
	abstractTypes   int
	concreteTypes   int
	diagnostics     []Diagnostic
	hasFunctionData bool
}

type sourceImport struct {
	packagePath string
	component   componentDescriptor
	site        ImportSite
}

type componentAccumulator struct {
	descriptor      componentDescriptor
	packages        stringSet
	sourceFiles     stringSet
	productionFiles stringSet
	testFiles       stringSet
	abstractTypes   int
	concreteTypes   int
}

type relationshipKey struct {
	source string
	target string
}

type architectureRelationshipKey struct {
	source   string
	target   string
	testOnly bool
}

type relationshipAccumulator struct {
	productionFiles          stringSet
	testFiles                stringSet
	sourcePackages           stringSet
	productionSourcePackages stringSet
	testSourcePackages       stringSet
	targetPackages           stringSet
	importSites              []ImportSite
}

type couplingCalculation struct {
	allDependencies            map[string]stringSet
	allImportingComponents     map[string]stringSet
	productionDependencies     map[string]stringSet
	productionImporting        map[string]stringSet
	incomingPackages           map[string]stringSet
	incomingProductionPackages map[string]stringSet
	incomingTestPackages       map[string]stringSet
	testOnlyIncoming           map[string]stringSet
	testOnlyOutgoing           map[string]stringSet
}

type stringSet map[string]struct{}

func collectComponentFile(components map[string]*componentAccumulator, file sourceFile) {
	component := getOrCreateComponent(components, file.component)
	component.packages.add(file.packagePath)
	component.sourceFiles.add(file.relativePath)
	if file.test {
		component.testFiles.add(file.relativePath)
		return
	}
	component.productionFiles.add(file.relativePath)
	component.abstractTypes += file.abstractTypes
	component.concreteTypes += file.concreteTypes
}

func collectRelationships(
	components map[string]*componentAccumulator,
	relationships map[relationshipKey]*relationshipAccumulator,
	file sourceFile,
) {
	for _, imported := range file.imports {
		target := imported.component
		if target.identifier == file.component.identifier {
			continue
		}
		getOrCreateComponent(components, target)
		key := relationshipKey{source: file.component.identifier, target: target.identifier}
		relationship, exists := relationships[key]
		if !exists {
			relationship = &relationshipAccumulator{
				productionFiles:          make(stringSet),
				testFiles:                make(stringSet),
				sourcePackages:           make(stringSet),
				productionSourcePackages: make(stringSet),
				testSourcePackages:       make(stringSet),
				targetPackages:           make(stringSet),
				importSites:              make([]ImportSite, 0),
			}
			relationships[key] = relationship
		}
		relationship.sourcePackages.add(file.packagePath)
		relationship.targetPackages.add(imported.packagePath)
		relationship.importSites = append(relationship.importSites, imported.site)
		if file.test {
			relationship.testFiles.add(file.relativePath)
			relationship.testSourcePackages.add(file.packagePath)
			continue
		}
		relationship.productionFiles.add(file.relativePath)
		relationship.productionSourcePackages.add(file.packagePath)
	}
}

func getOrCreateComponent(
	components map[string]*componentAccumulator,
	descriptor componentDescriptor,
) *componentAccumulator {
	component, exists := components[descriptor.identifier]
	if exists {
		return component
	}
	component = &componentAccumulator{
		descriptor:      descriptor,
		packages:        make(stringSet),
		sourceFiles:     make(stringSet),
		productionFiles: make(stringSet),
		testFiles:       make(stringSet),
	}
	components[descriptor.identifier] = component
	return component
}

func buildGraph(
	modulePath string,
	componentData map[string]*componentAccumulator,
	relationshipData map[relationshipKey]*relationshipAccumulator,
	diagnostics []Diagnostic,
) Graph {
	return buildGraphWithRules(
		modulePath,
		componentData,
		relationshipData,
		diagnostics,
		architecture.DefaultRegistry(),
	)
}

func buildGraphWithRules(
	modulePath string,
	componentData map[string]*componentAccumulator,
	relationshipData map[relationshipKey]*relationshipAccumulator,
	diagnostics []Diagnostic,
	rules architecture.Registry,
) Graph {
	identifiers := sortedMapKeys(componentData)
	relationshipKeys := sortedRelationshipKeys(relationshipData)
	coupling := newCouplingCalculation(identifiers)
	relationships := calculateRelationships(relationshipKeys, relationshipData, coupling)
	cycles := stronglyConnectedComponents(identifiers, coupling.productionDependencies)
	components := calculateComponents(identifiers, componentData, coupling, cycles)

	slices.SortFunc(diagnostics, func(first, second Diagnostic) int {
		return cmp.Or(
			strings.Compare(first.Path, second.Path),
			strings.Compare(first.Message, second.Message),
		)
	})
	graph := Graph{
		SchemaVersion:  graphSchemaVersion,
		ModulePath:     modulePath,
		Policy:         defaultAnalysisPolicy(),
		Components:     components,
		Relationships:  relationships,
		Functions:      make([]Function, 0),
		FunctionCalls:  make([]FunctionCall, 0),
		FunctionCycles: make([][]string, 0),
		Cycles:         cycles,
		Diagnostics:    diagnostics,
	}
	applyArchitectureRules(&graph, rules)
	graph.Summary = summarizeGraph(graph)
	return graph
}

func sortedRelationshipKeys(
	relationshipData map[relationshipKey]*relationshipAccumulator,
) []relationshipKey {
	relationshipKeys := make([]relationshipKey, 0, len(relationshipData))
	for key := range relationshipData {
		relationshipKeys = append(relationshipKeys, key)
	}
	slices.SortFunc(relationshipKeys, func(first, second relationshipKey) int {
		return cmp.Or(
			strings.Compare(first.source, second.source),
			strings.Compare(first.target, second.target),
		)
	})
	return relationshipKeys
}

func newCouplingCalculation(identifiers []string) *couplingCalculation {
	calculation := &couplingCalculation{
		allDependencies:            newAdjacency(identifiers),
		allImportingComponents:     newAdjacency(identifiers),
		productionDependencies:     newAdjacency(identifiers),
		productionImporting:        newAdjacency(identifiers),
		incomingPackages:           make(map[string]stringSet, len(identifiers)),
		incomingProductionPackages: make(map[string]stringSet, len(identifiers)),
		incomingTestPackages:       make(map[string]stringSet, len(identifiers)),
		testOnlyIncoming:           make(map[string]stringSet, len(identifiers)),
		testOnlyOutgoing:           make(map[string]stringSet, len(identifiers)),
	}
	for _, identifier := range identifiers {
		calculation.incomingPackages[identifier] = make(stringSet)
		calculation.incomingProductionPackages[identifier] = make(stringSet)
		calculation.incomingTestPackages[identifier] = make(stringSet)
		calculation.testOnlyIncoming[identifier] = make(stringSet)
		calculation.testOnlyOutgoing[identifier] = make(stringSet)
	}
	return calculation
}

func calculateRelationships(
	keys []relationshipKey,
	data map[relationshipKey]*relationshipAccumulator,
	coupling *couplingCalculation,
) []Relationship {
	relationships := make([]Relationship, 0, len(keys))
	for _, key := range keys {
		relationshipData := data[key]
		testOnly := len(relationshipData.productionFiles) == 0
		coupling.allDependencies[key.source].add(key.target)
		coupling.allImportingComponents[key.target].add(key.source)
		if testOnly {
			coupling.testOnlyIncoming[key.target].add(key.source)
			coupling.testOnlyOutgoing[key.source].add(key.target)
		} else {
			coupling.productionDependencies[key.source].add(key.target)
			coupling.productionImporting[key.target].add(key.source)
		}
		coupling.incomingPackages[key.target].addAll(relationshipData.sourcePackages)
		coupling.incomingProductionPackages[key.target].addAll(
			relationshipData.productionSourcePackages,
		)
		coupling.incomingTestPackages[key.target].addAll(relationshipData.testSourcePackages)
		relationships = append(relationships, Relationship{
			Source:                     key.source,
			Target:                     key.target,
			ProductionReferencingFiles: len(relationshipData.productionFiles),
			TestReferencingFiles:       len(relationshipData.testFiles),
			SourcePackages:             sortedSet(relationshipData.sourcePackages),
			TargetPackages:             sortedSet(relationshipData.targetPackages),
			ImportSites:                sortedImportSites(relationshipData.importSites),
			TestOnly:                   testOnly,
			RuleViolations:             []string{},
		})
	}
	return relationships
}

func calculateComponents(
	identifiers []string,
	componentData map[string]*componentAccumulator,
	coupling *couplingCalculation,
	cycles [][]string,
) []Component {
	cycleMembers := make(stringSet)
	for _, cycle := range cycles {
		cycleMembers.addAll(newStringSet(cycle...))
	}

	components := make([]Component, 0, len(identifiers))
	for _, identifier := range identifiers {
		data := componentData[identifier]
		transitiveDependencies := reachable(identifier, coupling.productionDependencies)
		transitiveImportingComponents := reachable(identifier, coupling.productionImporting)
		usingApplications := applicationsUsingComponent(
			identifier,
			coupling.productionImporting,
			componentData,
		)
		afferentCoupling := len(coupling.productionImporting[identifier])
		efferentCoupling := len(coupling.productionDependencies[identifier])
		componentInstability := instability(afferentCoupling, efferentCoupling)
		componentAbstractness := abstractness(data.abstractTypes, data.concreteTypes)
		category := data.descriptor.category
		if category == "" {
			category = string(data.descriptor.role)
		}
		components = append(components, Component{
			Identifier:                    identifier,
			Name:                          data.descriptor.name,
			Role:                          data.descriptor.role,
			Category:                      category,
			Application:                   data.descriptor.application,
			Packages:                      len(data.packages),
			SourceFiles:                   len(data.sourceFiles),
			ProductionFiles:               len(data.productionFiles),
			TestFiles:                     len(data.testFiles),
			DirectDependencies:            len(coupling.allDependencies[identifier]),
			ProductionDependencies:        len(coupling.productionDependencies[identifier]),
			TestOnlyDependencies:          len(coupling.testOnlyOutgoing[identifier]),
			DirectImportingComponents:     len(coupling.allImportingComponents[identifier]),
			ProductionImportingComponents: afferentCoupling,
			TestOnlyImportingComponents:   len(coupling.testOnlyIncoming[identifier]),
			TransitiveDependencies:        len(transitiveDependencies),
			TransitiveImportingComponents: len(transitiveImportingComponents),
			ImporterPackages:              len(coupling.incomingPackages[identifier]),
			ProductionImporterPackages:    len(coupling.incomingProductionPackages[identifier]),
			TestImporterPackages:          len(coupling.incomingTestPackages[identifier]),
			UsingApplicationCount:         len(usingApplications),
			UsingApplications:             usingApplications,
			AfferentCoupling:              afferentCoupling,
			EfferentCoupling:              efferentCoupling,
			Instability:                   componentInstability,
			AbstractTypes:                 data.abstractTypes,
			ConcreteTypes:                 data.concreteTypes,
			Abstractness:                  componentAbstractness,
			MainSequenceDistance:          mainSequenceDistance(componentAbstractness, componentInstability),
			IsStableWithLowAbstraction: isStableWithLowAbstraction(
				afferentCoupling,
				componentInstability,
				componentAbstractness,
			),
			InCycle: cycleMembers.contains(identifier),
		})
	}
	return components
}

func sortedImportSites(sites []ImportSite) []ImportSite {
	result := slices.Clone(sites)
	slices.SortFunc(result, func(first, second ImportSite) int {
		return cmp.Or(
			strings.Compare(first.Path, second.Path),
			cmp.Compare(first.Line, second.Line),
			strings.Compare(first.TargetPackage, second.TargetPackage),
			strings.Compare(first.Alias, second.Alias),
		)
	})
	return result
}

func defaultAnalysisPolicy() AnalysisPolicy {
	return AnalysisPolicy{
		InstabilityFormula:          "Ce/(Ca+Ce)",
		FunctionInstabilityFormula:  "functionCe/(functionCa+functionCe)",
		FunctionCouplingDefinition:  "unique caller functions and callee functions from resolved production calls",
		FunctionCallResolution:      "Go static type information",
		IsolatedInstability:         0,
		AbstractnessFormula:         "abstractTypes/(abstractTypes+concreteTypes)",
		UntypedAbstractness:         0,
		MainSequenceDistanceFormula: "abs(A+I-1)",
		StableLowAbstraction: StableLowAbstractionPolicy{
			MinimumAfferentCoupling: 1,
			MaximumInstability:      stableLowAbstractionMaximumInstability,
			MaximumAbstractness:     stableLowAbstractionMaximumAbstractness,
		},
		StableDependencyPrinciple: StableDependencyPrinciplePolicy{
			RequiredRelation: "targetInstability <= sourceInstability",
			ProductionOnly:   true,
		},
	}
}

func summarizeGraph(graph Graph) GraphSummary {
	summary := GraphSummary{
		Components:     len(graph.Components),
		Relationships:  len(graph.Relationships),
		Functions:      len(graph.Functions),
		FunctionCalls:  len(graph.FunctionCalls),
		FunctionCycles: len(graph.FunctionCycles),
		Cycles:         len(graph.Cycles),
		Findings:       len(graph.Findings),
		Categories:     make(map[string]int),
	}
	for _, function := range graph.Functions {
		if function.Test {
			summary.TestFunctions++
		} else {
			summary.ProductionFunctions++
		}
	}
	for _, call := range graph.FunctionCalls {
		summary.FunctionCallSites += call.Calls
		if call.CrossComponent {
			summary.CrossComponentFunctionCallSites += call.Calls
		}
	}
	for _, component := range graph.Components {
		summary.Categories[component.Category]++
		if component.IsStableWithLowAbstraction {
			summary.StableLowAbstractionComponents++
		}
		switch component.Role {
		case componentRoleApplication:
			summary.Applications++
		case componentRoleApplicationModule:
			summary.ApplicationModules++
		case componentRoleSharedModule:
			summary.SharedModules++
		case componentRoleLibrary:
			summary.Libraries++
		case componentRoleInfrastructure:
			summary.Infrastructure++
		case componentRoleDevelopment:
			summary.DevelopmentTools++
		}
	}
	for _, relationship := range graph.Relationships {
		if relationship.TestOnly {
			summary.TestOnlyRelationships++
		} else {
			summary.ProductionRelationships++
		}
		if relationship.ViolatesStableDependencyPrinciple {
			summary.StableDependencyPrincipleViolations++
		}
		summary.RelationshipRuleViolations += len(relationship.RuleViolations)
	}
	for _, finding := range graph.Findings {
		switch finding.Severity {
		case findingSeverityError:
			summary.Errors++
		case findingSeverityWarning:
			summary.Warnings++
		}
	}
	return summary
}

func detectFindings(graph Graph) []Finding {
	return mapArchitectureFindings(
		architecture.DefaultRegistry().Evaluate(toArchitectureGraph(graph)),
	)
}

func relationshipRuleViolations(
	source componentDescriptor,
	target componentDescriptor,
	testOnly bool,
) []string {
	graph := Graph{
		Components: []Component{
			componentFromDescriptor("source", source),
			componentFromDescriptor("target", target),
		},
		Relationships: []Relationship{{
			Source:   "source",
			Target:   "target",
			TestOnly: testOnly,
		}},
	}
	violations := make(stringSet)
	for _, finding := range detectFindings(graph) {
		if finding.Source != "" && finding.Target != "" {
			violations.add(finding.Rule)
		}
	}
	return sortedSet(violations)
}

func newAdjacency(identifiers []string) map[string]stringSet {
	adjacency := make(map[string]stringSet, len(identifiers))
	for _, identifier := range identifiers {
		adjacency[identifier] = make(stringSet)
	}
	return adjacency
}

func reachable(start string, adjacency map[string]stringSet) stringSet {
	visited := make(stringSet)
	stack := sortedSet(adjacency[start])
	for len(stack) > 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if current == start || visited.contains(current) {
			continue
		}
		visited.add(current)
		stack = append(stack, sortedSet(adjacency[current])...)
	}
	return visited
}

func applicationsUsingComponent(
	start string,
	importingComponents map[string]stringSet,
	components map[string]*componentAccumulator,
) []string {
	importingComponentIdentifiers := reachable(start, importingComponents)
	importingComponentIdentifiers.add(start)
	applications := make(stringSet)
	for identifier := range importingComponentIdentifiers {
		descriptor := components[identifier].descriptor
		if descriptor.application != "" {
			applications.add(descriptor.application)
		}
	}
	return sortedSet(applications)
}

func instability(fanIn, fanOut int) float64 {
	return Instability(fanIn, fanOut)
}

func abstractness(abstractTypes, concreteTypes int) float64 {
	return Abstractness(abstractTypes, concreteTypes)
}

func mainSequenceDistance(componentAbstractness, componentInstability float64) float64 {
	return MainSequenceDistance(componentAbstractness, componentInstability)
}

func isStableWithLowAbstraction(
	afferentCoupling int,
	componentInstability float64,
	componentAbstractness float64,
) bool {
	return StableWithLowAbstraction(
		afferentCoupling,
		componentInstability,
		componentAbstractness,
		stableLowAbstractionMaximumInstability,
		stableLowAbstractionMaximumAbstractness,
	)
}

func annotateStableDependencyPrincipleViolations(relationships []Relationship, components []Component) {
	graph := Graph{Components: components, Relationships: relationships}
	applyArchitectureRules(
		&graph,
		architecture.NewRegistry(architecture.StableDependencyPrincipleRule{}),
	)
}

func applyArchitectureRules(graph *Graph, registry architecture.Registry) {
	findings := mapArchitectureFindings(registry.Evaluate(toArchitectureGraph(*graph)))
	violations := make(map[architectureRelationshipKey]stringSet, len(graph.Relationships))
	for _, relationship := range graph.Relationships {
		key := architectureRelationshipKey{
			source:   relationship.Source,
			target:   relationship.Target,
			testOnly: relationship.TestOnly,
		}
		violations[key] = newStringSet(relationship.RuleViolations...)
	}
	for _, finding := range findings {
		if finding.Source == "" || finding.Target == "" {
			continue
		}
		key := architectureRelationshipKey{source: finding.Source, target: finding.Target}
		rules, exists := violations[key]
		if exists {
			rules.add(finding.Rule)
		}
	}
	for index := range graph.Relationships {
		relationship := &graph.Relationships[index]
		key := architectureRelationshipKey{
			source:   relationship.Source,
			target:   relationship.Target,
			testOnly: relationship.TestOnly,
		}
		relationship.RuleViolations = sortedSet(violations[key])
		relationship.ViolatesStableDependencyPrinciple =
			violations[key].contains(architecture.RuleStableDependencyPrinciple)
	}
	graph.Findings = findings
}

func toArchitectureGraph(graph Graph) architecture.Graph {
	components := make([]architecture.Component, len(graph.Components))
	for index, component := range graph.Components {
		components[index] = architecture.Component{
			Identifier:               component.Identifier,
			Role:                     component.Role,
			Application:              component.Application,
			AfferentCoupling:         component.AfferentCoupling,
			EfferentCoupling:         component.EfferentCoupling,
			Instability:              component.Instability,
			Abstractness:             component.Abstractness,
			MainSequenceDistance:     component.MainSequenceDistance,
			StableWithLowAbstraction: component.IsStableWithLowAbstraction,
		}
	}
	relationships := make([]architecture.Relationship, len(graph.Relationships))
	for index, relationship := range graph.Relationships {
		relationships[index] = architecture.Relationship{
			Source:   relationship.Source,
			Target:   relationship.Target,
			TestOnly: relationship.TestOnly,
		}
	}
	diagnostics := make([]architecture.Diagnostic, len(graph.Diagnostics))
	for index, diagnostic := range graph.Diagnostics {
		diagnostics[index] = architecture.Diagnostic{
			Path:    diagnostic.Path,
			Message: diagnostic.Message,
		}
	}
	return architecture.Graph{
		Components:    components,
		Relationships: relationships,
		Cycles:        graph.Cycles,
		Diagnostics:   diagnostics,
	}
}

func mapArchitectureFindings(source []architecture.Finding) []Finding {
	findings := make([]Finding, 0, len(source))
	for _, finding := range source {
		findings = append(findings, Finding{
			Rule:       finding.Rule,
			Severity:   finding.Severity,
			Subject:    finding.Subject,
			Message:    finding.Message,
			Source:     finding.Source,
			Target:     finding.Target,
			Components: slices.Clone(finding.Components),
			Metrics:    cloneMetrics(finding.Metrics),
		})
	}
	return findings
}

func cloneMetrics(source map[string]float64) map[string]float64 {
	if source == nil {
		return nil
	}
	metrics := make(map[string]float64, len(source))
	for name, value := range source {
		metrics[name] = value
	}
	return metrics
}

func componentFromDescriptor(identifier string, descriptor componentDescriptor) Component {
	return Component{
		Identifier:  identifier,
		Role:        descriptor.role,
		Category:    descriptor.category,
		Application: descriptor.application,
	}
}

func stronglyConnectedComponents(
	identifiers []string,
	adjacency map[string]stringSet,
) [][]string {
	var index int
	indices := make(map[string]int, len(identifiers))
	lowLinks := make(map[string]int, len(identifiers))
	onStack := make(stringSet)
	var stack []string
	var cycles [][]string

	var connect func(string)
	connect = func(identifier string) {
		indices[identifier] = index
		lowLinks[identifier] = index
		index++
		stack = append(stack, identifier)
		onStack.add(identifier)

		for _, dependency := range sortedSet(adjacency[identifier]) {
			dependencyIndex, visited := indices[dependency]
			if !visited {
				connect(dependency)
				lowLinks[identifier] = min(lowLinks[identifier], lowLinks[dependency])
				continue
			}
			if onStack.contains(dependency) {
				lowLinks[identifier] = min(lowLinks[identifier], dependencyIndex)
			}
		}

		if lowLinks[identifier] != indices[identifier] {
			return
		}
		var component []string
		for {
			last := len(stack) - 1
			member := stack[last]
			stack = stack[:last]
			delete(onStack, member)
			component = append(component, member)
			if member == identifier {
				break
			}
		}
		if len(component) > 1 {
			sort.Strings(component)
			cycles = append(cycles, component)
		}
	}

	for _, identifier := range identifiers {
		if _, visited := indices[identifier]; !visited {
			connect(identifier)
		}
	}
	slices.SortFunc(cycles, func(first, second []string) int {
		return strings.Compare(strings.Join(first, "\x00"), strings.Join(second, "\x00"))
	})
	return cycles
}

func sortedMapKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func newStringSet(values ...string) stringSet {
	set := make(stringSet, len(values))
	for _, value := range values {
		set.add(value)
	}
	return set
}

func (set stringSet) add(value string) {
	set[value] = struct{}{}
}

func (set stringSet) addAll(other stringSet) {
	for value := range other {
		set.add(value)
	}
}

func (set stringSet) contains(value string) bool {
	_, exists := set[value]
	return exists
}

func sortedSet(set stringSet) []string {
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T16:35:59Z","module_hash":"2db94cc9bef071bc60a4370fc83caf3eeb4599a2fcc2f64d41b283ec46ded699","functions":[{"id":"func/collectComponentFile","name":"collectComponentFile","line":88,"end_line":99,"hash":"a70c99d39e1cf9690418b4cad659ce282170988986f24dd4a1b960fb9429b736"},{"id":"func/collectRelationships","name":"collectRelationships","line":101,"end_line":137,"hash":"6c85c6086f3529a2a300a56086d29b2f189234880147a45c518173d2decf5ff2"},{"id":"func/getOrCreateComponent","name":"getOrCreateComponent","line":139,"end_line":156,"hash":"d3dc86dfbf70cd198928ccdd72a709a20fff7ee7eb95914f8137750aaa9156dc"},{"id":"func/buildGraph","name":"buildGraph","line":158,"end_line":171,"hash":"2e4e9bcc6b940591042d78197ba0d03cb7f82fe53e1c1bfc3547b8ae530f2201"},{"id":"func/buildGraphWithRules","name":"buildGraphWithRules","line":173,"end_line":208,"hash":"4ea0f8f359b8f1e26c2cae0766e08c74533495be4d7c94baf376c395205ac4fc"},{"id":"func/sortedRelationshipKeys","name":"sortedRelationshipKeys","line":210,"end_line":224,"hash":"1646942b49ba3246ab0d5db9cb0649cb7f840f7ae3465669658bcab6d24bd099"},{"id":"func/newCouplingCalculation","name":"newCouplingCalculation","line":226,"end_line":246,"hash":"765613eedd35790ce01eb74678b4ae2c2bb733ca65e42a2eebb3ba2b26b56fb4"},{"id":"func/calculateRelationships","name":"calculateRelationships","line":248,"end_line":284,"hash":"48e76c06a134b7fdbdbe33e7687de4776c5cdd107bd3a7add032313b8b730a76"},{"id":"func/calculateComponents","name":"calculateComponents","line":286,"end_line":354,"hash":"99deb176c00cd6f6b3cf18318c3417b666dddfea09b63f0a14774bd2f6110362"},{"id":"func/sortedImportSites","name":"sortedImportSites","line":356,"end_line":367,"hash":"729ddd6163295c1811de2ff72c769c334ffb3aa7196882021d35a9d8051fbd9e"},{"id":"func/defaultAnalysisPolicy","name":"defaultAnalysisPolicy","line":369,"end_line":389,"hash":"fd82f877b195b3ea92747a091a8e500501aa6c7f4c73a6bab753f47fdc09cfce"},{"id":"func/summarizeGraph","name":"summarizeGraph","line":391,"end_line":455,"hash":"f680a73fb2c62e9c706a153c98fbd11554c4c348d53ffa2225c6097b6920101e"},{"id":"func/detectFindings","name":"detectFindings","line":457,"end_line":461,"hash":"9a4e8abcce3e69d79f1a89022e11526ea36c0aa84c926a413e15c06f83b469a2"},{"id":"func/relationshipRuleViolations","name":"relationshipRuleViolations","line":463,"end_line":486,"hash":"a26b4b8c99a6ce0b2d9da2bc1e0de89866f42e7a096f9ba26ad197d6c2f111d9"},{"id":"func/newAdjacency","name":"newAdjacency","line":488,"end_line":494,"hash":"6eceeef34cd2d6d3abcf78165a79bba7d1befb6c0cde92110a15075a4e38efea"},{"id":"func/reachable","name":"reachable","line":496,"end_line":510,"hash":"c071168a27fe3dd7a2d070ee43f8de98c49bf6159c3acfdda5112a3fb0b0af4e"},{"id":"func/applicationsUsingComponent","name":"applicationsUsingComponent","line":512,"end_line":527,"hash":"3ec05464a25ddd856be98a5dd7635ee19162293241ec4e6900762c403099a464"},{"id":"func/instability","name":"instability","line":529,"end_line":531,"hash":"5c7cc5ef5857dd4106d5d59146403def47f3445df2b57820d8647e39f8fae948"},{"id":"func/abstractness","name":"abstractness","line":533,"end_line":535,"hash":"0bb6536e7ba3f81c4686512eb118ceef7924b40b1588fb86102656b9adb375ec"},{"id":"func/mainSequenceDistance","name":"mainSequenceDistance","line":537,"end_line":539,"hash":"92a293cb8ded468ab426b684be1cf1f2418e773058a075b151c5e7be758ba485"},{"id":"func/isStableWithLowAbstraction","name":"isStableWithLowAbstraction","line":541,"end_line":553,"hash":"79b525d709b5b5fd5977e7b20f665e25715674b83f52c967911795f997b404ba"},{"id":"func/annotateStableDependencyPrincipleViolations","name":"annotateStableDependencyPrincipleViolations","line":555,"end_line":561,"hash":"f9d77ed1648a74b9c574ae19fd794053f1eb0f99278ded67fb003a61596c889d"},{"id":"func/applyArchitectureRules","name":"applyArchitectureRules","line":563,"end_line":596,"hash":"4827218e3324d0e8539ccfea5c196fc5515a1840b179d8ead64d3f96cdca2faf"},{"id":"func/toArchitectureGraph","name":"toArchitectureGraph","line":598,"end_line":634,"hash":"ee1e613144d2851f1db740e1ad8b51ec11f6655d9655da8e838dfb9a8e88f5b0"},{"id":"func/mapArchitectureFindings","name":"mapArchitectureFindings","line":636,"end_line":651,"hash":"d821f446dbf23fde5e9255c46bcaa2d3779952d2656d0505a62275d66648530c"},{"id":"func/cloneMetrics","name":"cloneMetrics","line":653,"end_line":662,"hash":"982c48d7986d5e8ea689045f42bdc4824725dd215061a3e2b3c8ea4d2239a54d"},{"id":"func/componentFromDescriptor","name":"componentFromDescriptor","line":664,"end_line":671,"hash":"ae0c8d963e8be25bcf9624b17d89048fb7e871addc700e3920ca53338265bd5b"},{"id":"func/stronglyConnectedComponents","name":"stronglyConnectedComponents","line":673,"end_line":733,"hash":"0b241044e11ec0aa3ed51cabf7b9067999e1f1ba448741781fd8337113f0a95a"},{"id":"func/sortedMapKeys","name":"sortedMapKeys","line":735,"end_line":742,"hash":"1d5deae91c55d6844f6f56d5f0c16073c50098cda817844e28cbc5c01bf8341a"},{"id":"func/newStringSet","name":"newStringSet","line":744,"end_line":750,"hash":"b730b1c23697d3e1870fca737c8d3c49fecf63e9776f5332ea9e200b97dff445"},{"id":"func/stringSet.add","name":"stringSet.add","line":752,"end_line":754,"hash":"45b2c2c013dfaa1cd9a65c1a644f1f282c32a010db5cc7a3de75e4409c0a535c"},{"id":"func/stringSet.addAll","name":"stringSet.addAll","line":756,"end_line":760,"hash":"90d3236ccd4c3035b96c98f45e83750e147e8ebc92f8436e0178619ca48dffb1"},{"id":"func/stringSet.contains","name":"stringSet.contains","line":762,"end_line":765,"hash":"6f2d5791a86876cd618cecf68b5b4c02454923aa193152eb605bca630ed89f3f"},{"id":"func/sortedSet","name":"sortedSet","line":767,"end_line":774,"hash":"ec41880a5d2594ae1abb2611395a53d9162461b669743e28a537e2cb79566ba1"}]}
// mutate4go-manifest-end

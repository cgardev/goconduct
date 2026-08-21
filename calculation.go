package main

import (
	"cmp"
	"math"
	"slices"
	"sort"
	"strings"
)

const (
	stableLowAbstractionMaximumInstability  = 0.2
	stableLowAbstractionMaximumAbstractness = 0.2
)

// componentDescriptor identifies one component for the calculation functions.
type componentDescriptor struct {
	identifier  string
	name        string
	kind        componentKind
	application string
}

// sourceFile contains the source data that the analyzer collected.
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

type relationshipAccumulator struct {
	productionFiles          stringSet
	testFiles                stringSet
	sourcePackages           stringSet
	productionSourcePackages stringSet
	testSourcePackages       stringSet
	targetPackages           stringSet
	importSites              []ImportSite
}

type stringSet map[string]struct{}

func collectComponentFile(components map[string]*componentAccumulator, file sourceFile) {
	component := ensureComponent(components, file.component)
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
		ensureComponent(components, target)
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

func ensureComponent(
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
	identifiers := sortedMapKeys(componentData)
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

	allDependencies := newAdjacency(identifiers)
	allImportingComponents := newAdjacency(identifiers)
	productionDependencies := newAdjacency(identifiers)
	productionImportingComponents := newAdjacency(identifiers)
	incomingPackages := make(map[string]stringSet, len(identifiers))
	incomingProductionPackages := make(map[string]stringSet, len(identifiers))
	incomingTestPackages := make(map[string]stringSet, len(identifiers))
	productionIncoming := make(map[string]stringSet, len(identifiers))
	testOnlyIncoming := make(map[string]stringSet, len(identifiers))
	testOnlyOutgoing := make(map[string]stringSet, len(identifiers))
	for _, identifier := range identifiers {
		incomingPackages[identifier] = make(stringSet)
		incomingProductionPackages[identifier] = make(stringSet)
		incomingTestPackages[identifier] = make(stringSet)
		productionIncoming[identifier] = make(stringSet)
		testOnlyIncoming[identifier] = make(stringSet)
		testOnlyOutgoing[identifier] = make(stringSet)
	}

	relationships := make([]Relationship, 0, len(relationshipKeys))
	for _, key := range relationshipKeys {
		data := relationshipData[key]
		testOnly := len(data.productionFiles) == 0
		allDependencies[key.source].add(key.target)
		allImportingComponents[key.target].add(key.source)
		if testOnly {
			testOnlyIncoming[key.target].add(key.source)
			testOnlyOutgoing[key.source].add(key.target)
		} else {
			productionDependencies[key.source].add(key.target)
			productionImportingComponents[key.target].add(key.source)
			productionIncoming[key.target].add(key.source)
		}
		incomingPackages[key.target].addAll(data.sourcePackages)
		incomingProductionPackages[key.target].addAll(data.productionSourcePackages)
		incomingTestPackages[key.target].addAll(data.testSourcePackages)
		relationships = append(relationships, Relationship{
			Source:               key.source,
			Target:               key.target,
			ProductionReferences: len(data.productionFiles),
			TestReferences:       len(data.testFiles),
			SourcePackages:       sortedSet(data.sourcePackages),
			TargetPackages:       sortedSet(data.targetPackages),
			ImportSites:          sortedImportSites(data.importSites),
			TestOnly:             testOnly,
			RuleViolations: relationshipRuleViolations(
				componentData[key.source].descriptor,
				componentData[key.target].descriptor,
				testOnly,
			),
		})
	}

	cycles := stronglyConnectedComponents(identifiers, productionDependencies)
	cycleMembers := make(stringSet)
	for _, cycle := range cycles {
		cycleMembers.addAll(newStringSet(cycle...))
	}

	components := make([]Component, 0, len(identifiers))
	for _, identifier := range identifiers {
		data := componentData[identifier]
		transitiveDependencies := reachable(identifier, productionDependencies)
		transitiveImportingComponents := reachable(identifier, productionImportingComponents)
		usingApplications := applicationsUsingComponent(identifier, productionImportingComponents, componentData)
		afferentCoupling := len(productionIncoming[identifier])
		efferentCoupling := len(productionDependencies[identifier])
		componentInstability := instability(afferentCoupling, efferentCoupling)
		componentAbstractness := abstractness(data.abstractTypes, data.concreteTypes)
		components = append(components, Component{
			Identifier:                    identifier,
			Name:                          data.descriptor.name,
			Kind:                          data.descriptor.kind,
			Application:                   data.descriptor.application,
			Packages:                      len(data.packages),
			SourceFiles:                   len(data.sourceFiles),
			ProductionFiles:               len(data.productionFiles),
			TestFiles:                     len(data.testFiles),
			DirectDependencies:            len(allDependencies[identifier]),
			ProductionDependencies:        len(productionDependencies[identifier]),
			TestOnlyDependencies:          len(testOnlyOutgoing[identifier]),
			DirectImportingComponents:     len(allImportingComponents[identifier]),
			ProductionImportingComponents: afferentCoupling,
			TestOnlyImportingComponents:   len(testOnlyIncoming[identifier]),
			TransitiveDependencies:        len(transitiveDependencies),
			TransitiveImportingComponents: len(transitiveImportingComponents),
			ImporterPackages:              len(incomingPackages[identifier]),
			ProductionImporterPackages:    len(incomingProductionPackages[identifier]),
			TestImporterPackages:          len(incomingTestPackages[identifier]),
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
	annotateStableDependencyPrincipleViolations(relationships, components)

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
	graph.Findings = detectFindings(graph)
	graph.Summary = summarizeGraph(graph)
	return graph
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
		FunctionCouplingDefinition:  "unique resolved production caller and called functions",
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
		Components:            len(graph.Components),
		Relationships:         len(graph.Relationships),
		Functions:             len(graph.Functions),
		FunctionRelationships: len(graph.FunctionCalls),
		FunctionCycles:        len(graph.FunctionCycles),
		Cycles:                len(graph.Cycles),
		Findings:              len(graph.Findings),
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
		if component.IsStableWithLowAbstraction {
			summary.StableLowAbstractionComponents++
		}
		switch component.Kind {
		case componentKindApplication:
			summary.Applications++
		case componentKindApplicationModule:
			summary.ApplicationModules++
		case componentKindSharedModule:
			summary.SharedModules++
		case componentKindLibrary:
			summary.Libraries++
		case componentKindInfrastructure:
			summary.Infrastructure++
		case componentKindDevelopment:
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
	components := make(map[string]Component, len(graph.Components))
	for _, component := range graph.Components {
		components[component.Identifier] = component
	}

	findings := make([]Finding, 0, len(graph.Cycles)+len(graph.Diagnostics))
	for _, cycle := range graph.Cycles {
		findings = append(findings, Finding{
			Rule:       "dependency-cycle",
			Severity:   findingSeverityError,
			Subject:    strings.Join(cycle, " -> "),
			Message:    "Production dependencies form a cycle.",
			Components: cycle,
			Metrics: map[string]float64{
				"componentCount": float64(len(cycle)),
			},
		})
	}
	for _, component := range graph.Components {
		if !component.IsStableWithLowAbstraction {
			continue
		}
		findings = append(findings, Finding{
			Rule:     "stable-component-low-abstraction",
			Severity: findingSeverityWarning,
			Subject:  component.Identifier,
			Message:  "One or more production components import this stable component, which has low abstraction.",
			Metrics: map[string]float64{
				"abstractness":         component.Abstractness,
				"afferentCoupling":     float64(component.AfferentCoupling),
				"efferentCoupling":     float64(component.EfferentCoupling),
				"instability":          component.Instability,
				"mainSequenceDistance": component.MainSequenceDistance,
			},
		})
	}
	for _, relationship := range graph.Relationships {
		for _, ruleViolation := range relationship.RuleViolations {
			finding := Finding{
				Rule:     ruleViolation,
				Severity: findingSeverityWarning,
				Subject:  relationship.Source + " -> " + relationship.Target,
				Message:  findingMessage(ruleViolation),
				Source:   relationship.Source,
				Target:   relationship.Target,
			}
			if ruleViolation == "stable-dependency-principle" {
				finding.Metrics = map[string]float64{
					"sourceInstability": components[relationship.Source].Instability,
					"targetInstability": components[relationship.Target].Instability,
				}
			}
			findings = append(findings, finding)
		}
	}
	for _, diagnostic := range graph.Diagnostics {
		findings = append(findings, Finding{
			Rule:     "source-diagnostic",
			Severity: findingSeverityError,
			Subject:  diagnostic.Path,
			Message:  diagnostic.Message,
		})
	}
	slices.SortFunc(findings, func(first, second Finding) int {
		return cmp.Or(
			strings.Compare(first.Rule, second.Rule),
			strings.Compare(first.Subject, second.Subject),
		)
	})
	return findings
}

func findingMessage(rule string) string {
	switch rule {
	case "cross-application-module-import":
		return "An application module imports a module from another application."
	case "library-imports-feature":
		return "A shared library imports a feature module."
	case "production-imports-development":
		return "Production code imports development code."
	case "shared-component-imports-application":
		return "A shared component imports application-specific code."
	case "stable-dependency-principle":
		return "The source component imports a less stable target component."
	default:
		return "The dependency violates an architecture rule."
	}
}

func relationshipRuleViolations(
	source componentDescriptor,
	target componentDescriptor,
	testOnly bool,
) []string {
	if testOnly {
		return []string{}
	}
	ruleViolations := make(stringSet)
	if target.kind == componentKindDevelopment {
		ruleViolations.add("production-imports-development")
	}
	if source.kind == componentKindLibrary && isFeatureKind(target.kind) {
		ruleViolations.add("library-imports-feature")
	}
	if (source.kind == componentKindSharedModule || source.kind == componentKindInfrastructure) &&
		(target.kind == componentKindApplication || target.kind == componentKindApplicationModule) {
		ruleViolations.add("shared-component-imports-application")
	}
	if source.kind == componentKindApplicationModule &&
		target.kind == componentKindApplicationModule &&
		source.application != target.application {
		ruleViolations.add("cross-application-module-import")
	}
	return sortedSet(ruleViolations)
}

func isFeatureKind(kind componentKind) bool {
	return kind == componentKindApplication ||
		kind == componentKindApplicationModule ||
		kind == componentKindSharedModule
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
	total := fanIn + fanOut
	if total == 0 {
		return 0
	}
	return float64(fanOut) / float64(total)
}

func abstractness(abstractTypes, concreteTypes int) float64 {
	total := abstractTypes + concreteTypes
	if total == 0 {
		return 0
	}
	return float64(abstractTypes) / float64(total)
}

func mainSequenceDistance(componentAbstractness, componentInstability float64) float64 {
	return math.Abs(componentAbstractness + componentInstability - 1)
}

func isStableWithLowAbstraction(
	afferentCoupling int,
	componentInstability float64,
	componentAbstractness float64,
) bool {
	return afferentCoupling > 0 &&
		componentInstability <= stableLowAbstractionMaximumInstability &&
		componentAbstractness <= stableLowAbstractionMaximumAbstractness
}

func violatesStableDependencyPrinciple(testOnly bool, sourceInstability, targetInstability float64) bool {
	return !testOnly && targetInstability > sourceInstability
}

func annotateStableDependencyPrincipleViolations(relationships []Relationship, components []Component) {
	instabilities := make(map[string]float64, len(components))
	for _, component := range components {
		instabilities[component.Identifier] = component.Instability
	}
	for index := range relationships {
		relationship := &relationships[index]
		relationship.ViolatesStableDependencyPrinciple = violatesStableDependencyPrinciple(
			relationship.TestOnly,
			instabilities[relationship.Source],
			instabilities[relationship.Target],
		)
		if !relationship.ViolatesStableDependencyPrinciple {
			continue
		}
		ruleViolations := newStringSet(relationship.RuleViolations...)
		ruleViolations.add("stable-dependency-principle")
		relationship.RuleViolations = sortedSet(ruleViolations)
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
// {"version":1,"tested_at":"2026-08-21T10:47:51Z","module_hash":"6ef7c8087b8f027a9f4b829ffaeb6080d84f69e86c72ab9d9b5d3cf06d85c09f","functions":[{"id":"func/collectComponentFile","name":"collectComponentFile","line":70,"end_line":81,"hash":"938958d970c744bf18e07ec11548edd3d1346ec9f3fd309db911319b46f7f58e"},{"id":"func/collectRelationships","name":"collectRelationships","line":83,"end_line":119,"hash":"505ddc4c1bbbc2a12e78bb77cb0dc48b091ba0c75fb59141f171ed9257df5e03"},{"id":"func/ensureComponent","name":"ensureComponent","line":121,"end_line":138,"hash":"b2ee1c6ea29e2fdafeba1ced76b4314eb3377cadcc6ca9df30f818f22c5898c6"},{"id":"func/buildGraph","name":"buildGraph","line":140,"end_line":287,"hash":"9877ff60c958b2b1f79f13d7e566a189331f09871b639090e8c35cfe524adde9"},{"id":"func/sortedImportSites","name":"sortedImportSites","line":289,"end_line":300,"hash":"729ddd6163295c1811de2ff72c769c334ffb3aa7196882021d35a9d8051fbd9e"},{"id":"func/defaultAnalysisPolicy","name":"defaultAnalysisPolicy","line":302,"end_line":322,"hash":"82d2dafcbf5aaa30a36f8f2e7253ac398b703f04020b4ad474d86a14387d22f1"},{"id":"func/summarizeGraph","name":"summarizeGraph","line":324,"end_line":386,"hash":"35e57c829f505dc352ca8704dddaef55f1f27b2a0cddfd2eaf0b48e1aad8f07d"},{"id":"func/detectFindings","name":"detectFindings","line":388,"end_line":459,"hash":"10160266034dbfcba96067821cce31fccc572641c44c0c51b96498ef1cb4430a"},{"id":"func/findingMessage","name":"findingMessage","line":461,"end_line":476,"hash":"af28466375cc484b6fc143060bc0b46703f0dac317a2c946fe5ba8890410d24f"},{"id":"func/relationshipRuleViolations","name":"relationshipRuleViolations","line":478,"end_line":503,"hash":"4e0c0ceeb3050332f3f5ca90ddfd3a3afe1d3879cdf3ba43b4dd9fe2897c0a5c"},{"id":"func/isFeatureKind","name":"isFeatureKind","line":505,"end_line":509,"hash":"c5e429066186e1a57f6e0a2a5a508a9aa37fea218938be30d8c82cdb1b3731a9"},{"id":"func/newAdjacency","name":"newAdjacency","line":511,"end_line":517,"hash":"6eceeef34cd2d6d3abcf78165a79bba7d1befb6c0cde92110a15075a4e38efea"},{"id":"func/reachable","name":"reachable","line":519,"end_line":533,"hash":"c071168a27fe3dd7a2d070ee43f8de98c49bf6159c3acfdda5112a3fb0b0af4e"},{"id":"func/applicationsUsingComponent","name":"applicationsUsingComponent","line":535,"end_line":550,"hash":"3ec05464a25ddd856be98a5dd7635ee19162293241ec4e6900762c403099a464"},{"id":"func/instability","name":"instability","line":552,"end_line":558,"hash":"b40772ba78c4b17186225502c99e12c720ed760823c4a596ca210a0cf34eaac4"},{"id":"func/abstractness","name":"abstractness","line":560,"end_line":566,"hash":"2c152b5d594bfe9b3e0c9bc4a72def1c7649d32e3fdcc167586438f35dbab972"},{"id":"func/mainSequenceDistance","name":"mainSequenceDistance","line":568,"end_line":570,"hash":"8ba4f3225162b2c9fa687623fb476836a5e0aec75d20bb08fc4b1c8a95bc1f8c"},{"id":"func/isStableWithLowAbstraction","name":"isStableWithLowAbstraction","line":572,"end_line":580,"hash":"91f2d822a4a3f3f3ca141788e5122df7be0219ad72beeef98c0cf2159734fbeb"},{"id":"func/violatesStableDependencyPrinciple","name":"violatesStableDependencyPrinciple","line":582,"end_line":584,"hash":"63c9e0e5d52643962f073958c80183d3b7f31977ff7339d0a0ebe9de394c82fb"},{"id":"func/annotateStableDependencyPrincipleViolations","name":"annotateStableDependencyPrincipleViolations","line":586,"end_line":605,"hash":"9c70a90cb4c621bc16fdd6b02e940eb659425e009e0ab06755aff0269a50cd34"},{"id":"func/stronglyConnectedComponents","name":"stronglyConnectedComponents","line":607,"end_line":667,"hash":"0b241044e11ec0aa3ed51cabf7b9067999e1f1ba448741781fd8337113f0a95a"},{"id":"func/sortedMapKeys","name":"sortedMapKeys","line":669,"end_line":676,"hash":"1d5deae91c55d6844f6f56d5f0c16073c50098cda817844e28cbc5c01bf8341a"},{"id":"func/newStringSet","name":"newStringSet","line":678,"end_line":684,"hash":"b730b1c23697d3e1870fca737c8d3c49fecf63e9776f5332ea9e200b97dff445"},{"id":"func/stringSet.add","name":"stringSet.add","line":686,"end_line":688,"hash":"45b2c2c013dfaa1cd9a65c1a644f1f282c32a010db5cc7a3de75e4409c0a535c"},{"id":"func/stringSet.addAll","name":"stringSet.addAll","line":690,"end_line":694,"hash":"90d3236ccd4c3035b96c98f45e83750e147e8ebc92f8436e0178619ca48dffb1"},{"id":"func/stringSet.contains","name":"stringSet.contains","line":696,"end_line":699,"hash":"6f2d5791a86876cd618cecf68b5b4c02454923aa193152eb605bca630ed89f3f"},{"id":"func/sortedSet","name":"sortedSet","line":701,"end_line":708,"hash":"ec41880a5d2594ae1abb2611395a53d9162461b669743e28a537e2cb79566ba1"}]}
// mutate4go-manifest-end

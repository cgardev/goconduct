package main

import (
	"cmp"
	"math"
	"slices"
	"sort"
	"strings"
)

const (
	zoneOfPainMaximumInstability  = 0.2
	zoneOfPainMaximumAbstractness = 0.2
)

// componentDescriptor is normalized strategic identity used by pure calculations.
type componentDescriptor struct {
	identifier  string
	name        string
	kind        componentKind
	application string
}

// sourceFile is normalized source evidence produced by the repository adapter.
type sourceFile struct {
	relativePath  string
	packagePath   string
	component     componentDescriptor
	test          bool
	imports       []sourceImport
	abstractTypes int
	concreteTypes int
	diagnostics   []Diagnostic
}

type sourceImport struct {
	packagePath string
	component   componentDescriptor
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
			}
			relationships[key] = relationship
		}
		relationship.sourcePackages.add(file.packagePath)
		relationship.targetPackages.add(imported.packagePath)
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
	allDependants := newAdjacency(identifiers)
	productionDependencies := newAdjacency(identifiers)
	productionDependants := newAdjacency(identifiers)
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
		allDependants[key.target].add(key.source)
		if testOnly {
			testOnlyIncoming[key.target].add(key.source)
			testOnlyOutgoing[key.source].add(key.target)
		} else {
			productionDependencies[key.source].add(key.target)
			productionDependants[key.target].add(key.source)
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
			TestOnly:             testOnly,
			Concerns: relationshipConcerns(
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
		transitiveDependants := reachable(identifier, productionDependants)
		applications := reachedApplications(identifier, productionDependants, componentData)
		fanIn := len(productionIncoming[identifier])
		fanOut := len(productionDependencies[identifier])
		componentInstability := instability(fanIn, fanOut)
		componentAbstractness := abstractness(data.abstractTypes, data.concreteTypes)
		components = append(components, Component{
			Identifier:                 identifier,
			Name:                       data.descriptor.name,
			Kind:                       data.descriptor.kind,
			Application:                data.descriptor.application,
			Packages:                   len(data.packages),
			SourceFiles:                len(data.sourceFiles),
			ProductionFiles:            len(data.productionFiles),
			TestFiles:                  len(data.testFiles),
			DirectDependencies:         len(allDependencies[identifier]),
			ProductionDependencies:     len(productionDependencies[identifier]),
			TestOnlyDependencies:       len(testOnlyOutgoing[identifier]),
			DirectDependants:           len(allDependants[identifier]),
			ProductionDependants:       fanIn,
			TestOnlyDependants:         len(testOnlyIncoming[identifier]),
			TransitiveDependencies:     len(transitiveDependencies),
			TransitiveDependants:       len(transitiveDependants),
			ImporterPackages:           len(incomingPackages[identifier]),
			ProductionImporterPackages: len(incomingProductionPackages[identifier]),
			TestImporterPackages:       len(incomingTestPackages[identifier]),
			ApplicationReach:           len(applications),
			Applications:               applications,
			AfferentCoupling:           fanIn,
			EfferentCoupling:           fanOut,
			Instability:                componentInstability,
			AbstractTypes:              data.abstractTypes,
			ConcreteTypes:              data.concreteTypes,
			Abstractness:               componentAbstractness,
			MainSequenceDistance:       mainSequenceDistance(componentAbstractness, componentInstability),
			InZoneOfPain:               inZoneOfPain(fanIn, componentInstability, componentAbstractness),
			InCycle:                    cycleMembers.contains(identifier),
		})
	}
	annotateStableDependencyViolations(relationships, components)

	slices.SortFunc(diagnostics, func(first, second Diagnostic) int {
		return cmp.Or(
			strings.Compare(first.Path, second.Path),
			strings.Compare(first.Message, second.Message),
		)
	})
	graph := Graph{
		SchemaVersion: graphSchemaVersion,
		ModulePath:    modulePath,
		Policy:        defaultAnalysisPolicy(),
		Components:    components,
		Relationships: relationships,
		Cycles:        cycles,
		Diagnostics:   diagnostics,
	}
	graph.Findings = detectFindings(graph)
	graph.Summary = summarizeGraph(graph)
	return graph
}

func defaultAnalysisPolicy() AnalysisPolicy {
	return AnalysisPolicy{
		InstabilityFormula:          "Ce/(Ca+Ce)",
		IsolatedInstability:         0,
		AbstractnessFormula:         "abstractTypes/(abstractTypes+concreteTypes)",
		UntypedAbstractness:         0,
		MainSequenceDistanceFormula: "abs(A+I-1)",
		ZoneOfPain: ZoneOfPainPolicy{
			MinimumAfferentCoupling: 1,
			MaximumInstability:      zoneOfPainMaximumInstability,
			MaximumAbstractness:     zoneOfPainMaximumAbstractness,
		},
		StableDependency: StableDependencyPolicy{
			RequiredRelation: "targetInstability <= sourceInstability",
			ProductionOnly:   true,
		},
	}
}

func summarizeGraph(graph Graph) GraphSummary {
	summary := GraphSummary{
		Components:    len(graph.Components),
		Relationships: len(graph.Relationships),
		Cycles:        len(graph.Cycles),
		Findings:      len(graph.Findings),
	}
	for _, component := range graph.Components {
		if component.InZoneOfPain {
			summary.ZonesOfPain++
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
		if relationship.StableDependencyViolation {
			summary.StableDependencyViolations++
		}
		summary.Concerns += len(relationship.Concerns)
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
		if !component.InZoneOfPain {
			continue
		}
		findings = append(findings, Finding{
			Rule:     "zone-of-pain",
			Severity: findingSeverityWarning,
			Subject:  component.Identifier,
			Message:  "A stable concrete component has high incoming responsibility.",
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
		for _, concern := range relationship.Concerns {
			finding := Finding{
				Rule:     concern,
				Severity: findingSeverityWarning,
				Subject:  relationship.Source + " -> " + relationship.Target,
				Message:  findingMessage(concern),
				Source:   relationship.Source,
				Target:   relationship.Target,
			}
			if concern == "stable-dependency-principle" {
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
	case "cross-application-module-dependency":
		return "An application module depends on a module owned by another application."
	case "library-depends-on-feature":
		return "A shared library depends on a feature module."
	case "production-depends-on-development":
		return "Production code depends on development-only tooling."
	case "shared-foundation-depends-on-application":
		return "Shared foundation code depends on application-specific code."
	case "stable-dependency-principle":
		return "A dependency points to a less stable component."
	default:
		return "A strategic dependency rule is violated."
	}
}

func relationshipConcerns(
	source componentDescriptor,
	target componentDescriptor,
	testOnly bool,
) []string {
	if testOnly {
		return []string{}
	}
	concerns := make(stringSet)
	if target.kind == componentKindDevelopment {
		concerns.add("production-depends-on-development")
	}
	if source.kind == componentKindLibrary && isFeatureKind(target.kind) {
		concerns.add("library-depends-on-feature")
	}
	if (source.kind == componentKindSharedModule || source.kind == componentKindInfrastructure) &&
		(target.kind == componentKindApplication || target.kind == componentKindApplicationModule) {
		concerns.add("shared-foundation-depends-on-application")
	}
	if source.kind == componentKindApplicationModule &&
		target.kind == componentKindApplicationModule &&
		source.application != target.application {
		concerns.add("cross-application-module-dependency")
	}
	return sortedSet(concerns)
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

func reachedApplications(
	start string,
	dependants map[string]stringSet,
	components map[string]*componentAccumulator,
) []string {
	reached := reachable(start, dependants)
	reached.add(start)
	applications := make(stringSet)
	for identifier := range reached {
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

func inZoneOfPain(
	afferentCoupling int,
	componentInstability float64,
	componentAbstractness float64,
) bool {
	return afferentCoupling > 0 &&
		componentInstability <= zoneOfPainMaximumInstability &&
		componentAbstractness <= zoneOfPainMaximumAbstractness
}

func stableDependencyViolation(testOnly bool, sourceInstability, targetInstability float64) bool {
	return !testOnly && targetInstability > sourceInstability
}

func annotateStableDependencyViolations(relationships []Relationship, components []Component) {
	instabilities := make(map[string]float64, len(components))
	for _, component := range components {
		instabilities[component.Identifier] = component.Instability
	}
	for index := range relationships {
		relationship := &relationships[index]
		relationship.StableDependencyViolation = stableDependencyViolation(
			relationship.TestOnly,
			instabilities[relationship.Source],
			instabilities[relationship.Target],
		)
		if !relationship.StableDependencyViolation {
			continue
		}
		concerns := newStringSet(relationship.Concerns...)
		concerns.add("stable-dependency-principle")
		relationship.Concerns = sortedSet(concerns)
	}
}

func stronglyConnectedComponents(
	identifiers []string,
	adjacency map[string]stringSet,
) [][]string {
	index := 0
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
		for len(stack) > 0 {
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

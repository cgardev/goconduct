package main

import (
	"sort"

	"digginginsights.com/v3/internal/devtool/dependencygraph/internal/architecture"
	calculationdomain "digginginsights.com/v3/internal/devtool/dependencygraph/internal/calculation"
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

type componentAccumulator = calculationdomain.ComponentAccumulator

type relationshipKey struct {
	source string
	target string
}

type relationshipAccumulator = calculationdomain.RelationshipAccumulator

type stringSet map[string]struct{}

func collectComponentFile(components map[string]*componentAccumulator, file sourceFile) {
	calculationdomain.CollectComponentFile(components, calculationSourceFile(file))
}

func collectRelationships(
	components map[string]*componentAccumulator,
	relationships map[relationshipKey]*relationshipAccumulator,
	file sourceFile,
) {
	calculatedRelationships := calculationRelationships(relationships)
	calculationdomain.CollectRelationships(components, calculatedRelationships, calculationSourceFile(file))
	for key, relationship := range calculatedRelationships {
		relationships[relationshipKey{source: key.Source, target: key.Target}] = relationship
	}
}

func getOrCreateComponent(
	components map[string]*componentAccumulator,
	descriptor componentDescriptor,
) *componentAccumulator {
	return calculationdomain.GetOrCreateComponent(components, calculationComponentDescriptor(descriptor))
}

func buildGraph(
	modulePath string,
	components map[string]*componentAccumulator,
	relationships map[relationshipKey]*relationshipAccumulator,
	diagnostics []Diagnostic,
) Graph {
	return calculationdomain.BuildGraph(
		modulePath,
		components,
		calculationRelationships(relationships),
		diagnostics,
	)
}

func buildGraphWithRules(
	modulePath string,
	components map[string]*componentAccumulator,
	relationships map[relationshipKey]*relationshipAccumulator,
	diagnostics []Diagnostic,
	rules architecture.Registry,
) Graph {
	return calculationdomain.BuildGraphWithRules(
		modulePath,
		components,
		calculationRelationships(relationships),
		diagnostics,
		rules,
	)
}

func summarizeGraph(graph Graph) GraphSummary {
	return calculationdomain.SummarizeGraph(graph)
}

func detectFindings(graph Graph) []Finding {
	return calculationdomain.DetectFindings(graph)
}

func relationshipRuleViolations(
	source componentDescriptor,
	target componentDescriptor,
	testOnly bool,
) []string {
	return calculationdomain.RelationshipRuleViolations(
		calculationComponentDescriptor(source),
		calculationComponentDescriptor(target),
		testOnly,
	)
}

func instability(fanIn, fanOut int) float64 {
	return calculationdomain.Instability(fanIn, fanOut)
}

func abstractness(abstractTypes, concreteTypes int) float64 {
	return calculationdomain.Abstractness(abstractTypes, concreteTypes)
}

func mainSequenceDistance(componentAbstractness, componentInstability float64) float64 {
	return calculationdomain.MainSequenceDistance(componentAbstractness, componentInstability)
}

func isStableWithLowAbstraction(
	afferentCoupling int,
	componentInstability float64,
	componentAbstractness float64,
) bool {
	return calculationdomain.StableWithLowAbstraction(
		afferentCoupling,
		componentInstability,
		componentAbstractness,
		stableLowAbstractionMaximumInstability,
		stableLowAbstractionMaximumAbstractness,
	)
}

func annotateStableDependencyPrincipleViolations(relationships []Relationship, components []Component) {
	calculationdomain.AnnotateStableDependencyPrincipleViolations(relationships, components)
}

func applyArchitectureRules(graph *Graph, registry architecture.Registry) {
	calculationdomain.ApplyArchitectureRules(graph, registry)
}

func newAdjacency(identifiers []string) map[string]stringSet {
	adjacency := make(map[string]stringSet, len(identifiers))
	for identifier, targets := range calculationdomain.NewAdjacency(identifiers) {
		adjacency[identifier] = localStringSet(targets)
	}
	return adjacency
}

func reachable(start string, adjacency map[string]stringSet) stringSet {
	return localStringSet(calculationdomain.Reachable(start, calculationAdjacency(adjacency)))
}

func stronglyConnectedComponents(
	identifiers []string,
	adjacency map[string]stringSet,
) [][]string {
	return calculationdomain.StronglyConnectedComponents(identifiers, calculationAdjacency(adjacency))
}

func sortedMapKeys[T any](values map[string]T) []string {
	keys := []string{}
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
	values := []string{}
	for value := range set {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

func calculationComponentDescriptor(descriptor componentDescriptor) calculationdomain.ComponentDescriptor {
	return calculationdomain.ComponentDescriptor{
		Identifier:  descriptor.identifier,
		Name:        descriptor.name,
		Role:        descriptor.role,
		Category:    descriptor.category,
		Application: descriptor.application,
	}
}

func calculationSourceFile(file sourceFile) calculationdomain.SourceFile {
	imports := make([]calculationdomain.SourceImport, len(file.imports))
	for index, imported := range file.imports {
		imports[index] = calculationdomain.SourceImport{
			PackagePath: imported.packagePath,
			Component:   calculationComponentDescriptor(imported.component),
			Site:        imported.site,
		}
	}
	return calculationdomain.SourceFile{
		RelativePath:  file.relativePath,
		PackagePath:   file.packagePath,
		Component:     calculationComponentDescriptor(file.component),
		Test:          file.test,
		Imports:       imports,
		AbstractTypes: file.abstractTypes,
		ConcreteTypes: file.concreteTypes,
	}
}

func calculationRelationships(
	relationships map[relationshipKey]*relationshipAccumulator,
) map[calculationdomain.RelationshipKey]*calculationdomain.RelationshipAccumulator {
	result := make(
		map[calculationdomain.RelationshipKey]*calculationdomain.RelationshipAccumulator,
		len(relationships),
	)
	for key, relationship := range relationships {
		result[calculationdomain.RelationshipKey{Source: key.source, Target: key.target}] = relationship
	}
	return result
}

func calculationAdjacency(adjacency map[string]stringSet) map[string]calculationdomain.StringSet {
	result := make(map[string]calculationdomain.StringSet, len(adjacency))
	for identifier, targets := range adjacency {
		values := []string{}
		for target := range targets {
			values = append(values, target)
		}
		result[identifier] = calculationdomain.NewStringSet(values...)
	}
	return result
}

func localStringSet(source calculationdomain.StringSet) stringSet {
	result := make(stringSet, len(source))
	for value := range source {
		result.add(value)
	}
	return result
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-21T17:15:20Z","module_hash":"0b769047f38ee054f6c8dc83dbac915d4d3666d32243c6101b3c6d0825afefbc","functions":[{"id":"func/collectComponentFile","name":"collectComponentFile","line":52,"end_line":54,"hash":"7f0130572b5801c898f71ad6e42e35c41d72251dd1ba3d98f2ca243d7f3cfa47"},{"id":"func/collectRelationships","name":"collectRelationships","line":56,"end_line":66,"hash":"a5a705446dec25d6c9a4f063e90a4ccb41988f34d3c69f90b9ce751e9eea2104"},{"id":"func/getOrCreateComponent","name":"getOrCreateComponent","line":68,"end_line":73,"hash":"abc5d1235461244f3d20578355898df74241063c3b52baca7f64fd58d5addec8"},{"id":"func/buildGraph","name":"buildGraph","line":75,"end_line":87,"hash":"59861e6399a6edaa6ca3bb364c0ca174d31aad327137cd247c5d1380476945f9"},{"id":"func/buildGraphWithRules","name":"buildGraphWithRules","line":89,"end_line":103,"hash":"172deaf42861c5c055dd8dbb6b7718bf84d7e037af6a041bcba4dbce7c590784"},{"id":"func/summarizeGraph","name":"summarizeGraph","line":105,"end_line":107,"hash":"9a3630c9879b25ea299e45798d06be1a89605fce9df2470c6faca6eb54a7f30b"},{"id":"func/detectFindings","name":"detectFindings","line":109,"end_line":111,"hash":"17cd85b04c91c8b25b51443302628852a780660621faf0ac3cc497297eedda03"},{"id":"func/relationshipRuleViolations","name":"relationshipRuleViolations","line":113,"end_line":123,"hash":"1dca0e9f9ed790c3cda3a1201cfed8bb6789d77f211590671b247e5a9edb250e"},{"id":"func/instability","name":"instability","line":125,"end_line":127,"hash":"25a79f647e467996d2f3912ad498678417abe907f6058234d1c0116c30055a01"},{"id":"func/abstractness","name":"abstractness","line":129,"end_line":131,"hash":"d55537d2e07f70d50c69203db919a60204a55a9374849d2f8fad17614f972161"},{"id":"func/mainSequenceDistance","name":"mainSequenceDistance","line":133,"end_line":135,"hash":"27ab87bf768b96aa1add6adafc46f7abc552783513bfef394cd3a01479335719"},{"id":"func/isStableWithLowAbstraction","name":"isStableWithLowAbstraction","line":137,"end_line":149,"hash":"e17108061e4ccd6afd42e44f2623ba315c9845395812c7588f88e2c3ed8bea85"},{"id":"func/annotateStableDependencyPrincipleViolations","name":"annotateStableDependencyPrincipleViolations","line":151,"end_line":153,"hash":"82d52bb91e5ceba76886360502829752554c55f7eb7c2973ce4262dd9a9746db"},{"id":"func/applyArchitectureRules","name":"applyArchitectureRules","line":155,"end_line":157,"hash":"3739356e95aa3bb7d03e4befc78d2fb27bccacaa416e0164f61d91d8a2ec4fca"},{"id":"func/newAdjacency","name":"newAdjacency","line":159,"end_line":165,"hash":"ff86ab56bfa15b8207835f5ec48653b01144b87d7ca09ec1a6aacc3f44a39172"},{"id":"func/reachable","name":"reachable","line":167,"end_line":169,"hash":"78af4198c0001658e73f939f3a17351e4546fa8fa4376708ab0d5e87a36b7225"},{"id":"func/stronglyConnectedComponents","name":"stronglyConnectedComponents","line":171,"end_line":176,"hash":"838c7eaf0069b4057ba148996dfe395affa761ab7babe6bf4635be4421187ded"},{"id":"func/sortedMapKeys","name":"sortedMapKeys","line":178,"end_line":185,"hash":"1d5deae91c55d6844f6f56d5f0c16073c50098cda817844e28cbc5c01bf8341a"},{"id":"func/newStringSet","name":"newStringSet","line":187,"end_line":193,"hash":"b730b1c23697d3e1870fca737c8d3c49fecf63e9776f5332ea9e200b97dff445"},{"id":"func/stringSet.add","name":"stringSet.add","line":195,"end_line":197,"hash":"45b2c2c013dfaa1cd9a65c1a644f1f282c32a010db5cc7a3de75e4409c0a535c"},{"id":"func/stringSet.addAll","name":"stringSet.addAll","line":199,"end_line":203,"hash":"90d3236ccd4c3035b96c98f45e83750e147e8ebc92f8436e0178619ca48dffb1"},{"id":"func/stringSet.contains","name":"stringSet.contains","line":205,"end_line":208,"hash":"6f2d5791a86876cd618cecf68b5b4c02454923aa193152eb605bca630ed89f3f"},{"id":"func/sortedSet","name":"sortedSet","line":210,"end_line":217,"hash":"ec41880a5d2594ae1abb2611395a53d9162461b669743e28a537e2cb79566ba1"},{"id":"func/calculationComponentDescriptor","name":"calculationComponentDescriptor","line":219,"end_line":227,"hash":"0ac4642afed9f5ba8b982a0515e94340e9cba876c2867bff4ffc5d8b4273161b"},{"id":"func/calculationSourceFile","name":"calculationSourceFile","line":229,"end_line":247,"hash":"5b4f76cf0835c4c99024ee90ef019009aac5e55a2888db0501ec283f0aefe816"},{"id":"func/calculationRelationships","name":"calculationRelationships","line":249,"end_line":260,"hash":"5d082c0c8a6872fb6b3be4c1efd225f61cc705e7fbd783e41ad00513853c0a9d"},{"id":"func/calculationAdjacency","name":"calculationAdjacency","line":262,"end_line":272,"hash":"5b44b074a0acb122a2a702a58701f530f2b992ed39ac82d069497f16dd9e47e1"},{"id":"func/localStringSet","name":"localStringSet","line":274,"end_line":280,"hash":"f09e58fc791b3a04bd1a83f46ee5ca09def9f891b5b7b486877d6d9f4ee16b27"}]}
// mutate4go-manifest-end

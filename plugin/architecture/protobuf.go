package architecture

import (
	"slices"

	goconductv1 "github.com/cgardev/goconduct/internal/protogen/v1"
)

func graphToProto(graph Graph) *goconductv1.Graph {
	return &goconductv1.Graph{
		SchemaVersion:  countToProto(graph.SchemaVersion),
		Revision:       graph.Revision,
		ModulePath:     graph.ModulePath,
		Scope:          analysisScopeToProto(graph.Scope),
		Policy:         analysisPolicyToProto(graph.Policy),
		Summary:        graphSummaryToProto(graph.Summary),
		Components:     componentsToProto(graph.Components),
		Relationships:  relationshipsToProto(graph.Relationships),
		Functions:      functionsToProto(graph.Functions),
		FunctionCalls:  functionCallsToProto(graph.FunctionCalls),
		FunctionCycles: cyclesToProto(graph.FunctionCycles),
		Cycles:         cyclesToProto(graph.Cycles),
		Diagnostics:    diagnosticsToProto(graph.Diagnostics),
		Findings:       findingsToProto(graph.Findings),
	}
}

func analysisScopeToProto(scope AnalysisScope) *goconductv1.AnalysisScope {
	return &goconductv1.AnalysisScope{
		Paths:        slices.Clone(scope.Paths),
		IgnoredPaths: slices.Clone(scope.IgnoredPaths),
		Components:   componentRulesToProto(scope.Components),
	}
}

func componentRulesToProto(rules ComponentRules) *goconductv1.ComponentRules {
	taxonomy := make([]*goconductv1.ComponentCategoryRule, 0, len(rules.Taxonomy))
	for _, rule := range rules.Taxonomy {
		taxonomy = append(taxonomy, &goconductv1.ComponentCategoryRule{
			Id:    rule.Identifier,
			Role:  string(rule.Role),
			Paths: slices.Clone(rule.Paths),
		})
	}
	return &goconductv1.ComponentRules{
		Applications:       slices.Clone(rules.Applications),
		ApplicationModules: slices.Clone(rules.ApplicationModules),
		SharedModules:      slices.Clone(rules.SharedModules),
		Libraries:          slices.Clone(rules.Libraries),
		Infrastructure:     slices.Clone(rules.Infrastructure),
		DevelopmentTools:   slices.Clone(rules.DevelopmentTools),
		Taxonomy:           taxonomy,
	}
}

func analysisPolicyToProto(policy AnalysisPolicy) *goconductv1.AnalysisPolicy {
	return &goconductv1.AnalysisPolicy{
		InstabilityFormula:          policy.InstabilityFormula,
		FunctionInstabilityFormula:  policy.FunctionInstabilityFormula,
		FunctionCouplingDefinition:  policy.FunctionCouplingDefinition,
		FunctionCallResolution:      policy.FunctionCallResolution,
		IsolatedInstability:         policy.IsolatedInstability,
		AbstractnessFormula:         policy.AbstractnessFormula,
		UntypedAbstractness:         policy.UntypedAbstractness,
		MainSequenceDistanceFormula: policy.MainSequenceDistanceFormula,
		StableLowAbstraction: &goconductv1.StableLowAbstractionPolicy{
			MinimumAfferentCoupling: countToProto(
				policy.StableLowAbstraction.MinimumAfferentCoupling,
			),
			MaximumInstability:  policy.StableLowAbstraction.MaximumInstability,
			MaximumAbstractness: policy.StableLowAbstraction.MaximumAbstractness,
		},
		StableDependencyPrinciple: &goconductv1.StableDependencyPrinciplePolicy{
			RequiredRelation: policy.StableDependencyPrinciple.RequiredRelation,
			ProductionOnly:   policy.StableDependencyPrinciple.ProductionOnly,
		},
	}
}

func graphSummaryToProto(summary GraphSummary) *goconductv1.GraphSummary {
	categories := make(map[string]uint32, len(summary.Categories))
	for category, count := range summary.Categories {
		categories[category] = countToProto(count)
	}
	return &goconductv1.GraphSummary{
		Components:                          countToProto(summary.Components),
		Relationships:                       countToProto(summary.Relationships),
		Functions:                           countToProto(summary.Functions),
		ProductionFunctions:                 countToProto(summary.ProductionFunctions),
		TestFunctions:                       countToProto(summary.TestFunctions),
		FunctionCalls:                       countToProto(summary.FunctionCalls),
		FunctionCallSites:                   countToProto(summary.FunctionCallSites),
		CrossComponentFunctionCallSites:     countToProto(summary.CrossComponentFunctionCallSites),
		FunctionCycles:                      countToProto(summary.FunctionCycles),
		ProductionRelationships:             countToProto(summary.ProductionRelationships),
		TestOnlyRelationships:               countToProto(summary.TestOnlyRelationships),
		Applications:                        countToProto(summary.Applications),
		ApplicationModules:                  countToProto(summary.ApplicationModules),
		SharedModules:                       countToProto(summary.SharedModules),
		Libraries:                           countToProto(summary.Libraries),
		Infrastructure:                      countToProto(summary.Infrastructure),
		DevelopmentTools:                    countToProto(summary.DevelopmentTools),
		Categories:                          categories,
		Cycles:                              countToProto(summary.Cycles),
		RelationshipRuleViolations:          countToProto(summary.RelationshipRuleViolations),
		StableDependencyPrincipleViolations: countToProto(summary.StableDependencyPrincipleViolations),
		StableLowAbstractionComponents:      countToProto(summary.StableLowAbstractionComponents),
		Findings:                            countToProto(summary.Findings),
		Errors:                              countToProto(summary.Errors),
		Warnings:                            countToProto(summary.Warnings),
	}
}

func componentsToProto(components []Component) []*goconductv1.Component {
	result := make([]*goconductv1.Component, 0, len(components))
	for _, component := range components {
		result = append(result, &goconductv1.Component{
			Id:                            component.Identifier,
			Name:                          component.Name,
			Role:                          string(component.Role),
			Category:                      component.Category,
			Application:                   component.Application,
			Packages:                      countToProto(component.Packages),
			SourceFiles:                   countToProto(component.SourceFiles),
			ProductionFiles:               countToProto(component.ProductionFiles),
			TestFiles:                     countToProto(component.TestFiles),
			DirectDependencies:            countToProto(component.DirectDependencies),
			ProductionDependencies:        countToProto(component.ProductionDependencies),
			TestOnlyDependencies:          countToProto(component.TestOnlyDependencies),
			DirectImportingComponents:     countToProto(component.DirectImportingComponents),
			ProductionImportingComponents: countToProto(component.ProductionImportingComponents),
			TestOnlyImportingComponents:   countToProto(component.TestOnlyImportingComponents),
			TransitiveDependencies:        countToProto(component.TransitiveDependencies),
			TransitiveImportingComponents: countToProto(component.TransitiveImportingComponents),
			ImporterPackages:              countToProto(component.ImporterPackages),
			ProductionImporterPackages:    countToProto(component.ProductionImporterPackages),
			TestImporterPackages:          countToProto(component.TestImporterPackages),
			UsingApplicationCount:         countToProto(component.UsingApplicationCount),
			UsingApplications:             slices.Clone(component.UsingApplications),
			AfferentCoupling:              countToProto(component.AfferentCoupling),
			EfferentCoupling:              countToProto(component.EfferentCoupling),
			Instability:                   component.Instability,
			AbstractTypes:                 countToProto(component.AbstractTypes),
			ConcreteTypes:                 countToProto(component.ConcreteTypes),
			Abstractness:                  component.Abstractness,
			MainSequenceDistance:          component.MainSequenceDistance,
			IsStableWithLowAbstraction:    component.IsStableWithLowAbstraction,
			InCycle:                       component.InCycle,
			ProductionFunctions:           countToProto(component.ProductionFunctions),
			TestFunctions:                 countToProto(component.TestFunctions),
			ProductionIncomingCallSites:   countToProto(component.ProductionIncomingCallSites),
			ProductionOutgoingCallSites:   countToProto(component.ProductionOutgoingCallSites),
			TestIncomingCallSites:         countToProto(component.TestIncomingCallSites),
			TestOutgoingCallSites:         countToProto(component.TestOutgoingCallSites),
		})
	}
	return result
}

func relationshipsToProto(relationships []Relationship) []*goconductv1.Relationship {
	result := make([]*goconductv1.Relationship, 0, len(relationships))
	for _, relationship := range relationships {
		result = append(result, &goconductv1.Relationship{
			Source:                            relationship.Source,
			Target:                            relationship.Target,
			ProductionReferencingFiles:        countToProto(relationship.ProductionReferencingFiles),
			TestReferencingFiles:              countToProto(relationship.TestReferencingFiles),
			SourcePackages:                    slices.Clone(relationship.SourcePackages),
			TargetPackages:                    slices.Clone(relationship.TargetPackages),
			ImportSites:                       importSitesToProto(relationship.ImportSites),
			ProductionFunctionCallSites:       countToProto(relationship.ProductionFunctionCallSites),
			TestFunctionCallSites:             countToProto(relationship.TestFunctionCallSites),
			CallerFunctions:                   countToProto(relationship.CallerFunctions),
			CalleeFunctions:                   countToProto(relationship.CalleeFunctions),
			TestOnly:                          relationship.TestOnly,
			ViolatesStableDependencyPrinciple: relationship.ViolatesStableDependencyPrinciple,
			RuleViolations:                    slices.Clone(relationship.RuleViolations),
		})
	}
	return result
}

func importSitesToProto(sites []ImportSite) []*goconductv1.ImportSite {
	result := make([]*goconductv1.ImportSite, 0, len(sites))
	for _, site := range sites {
		result = append(result, &goconductv1.ImportSite{
			SourcePackage: site.SourcePackage,
			TargetPackage: site.TargetPackage,
			Path:          site.Path,
			Line:          countToProto(site.Line),
			Alias:         site.Alias,
			Test:          site.Test,
		})
	}
	return result
}

func functionsToProto(functions []Function) []*goconductv1.Function {
	result := make([]*goconductv1.Function, 0, len(functions))
	for _, function := range functions {
		result = append(result, &goconductv1.Function{
			Id:                            function.Identifier,
			Name:                          function.Name,
			Package:                       function.Package,
			Component:                     function.Component,
			Path:                          function.Path,
			Line:                          countToProto(function.Line),
			Receiver:                      function.Receiver,
			Method:                        function.Method,
			Exported:                      function.Exported,
			Test:                          function.Test,
			Synthetic:                     function.Synthetic,
			InAnalysisScope:               function.InAnalysisScope,
			AfferentCoupling:              countToProto(function.AfferentCoupling),
			EfferentCoupling:              countToProto(function.EfferentCoupling),
			TestAfferentCoupling:          countToProto(function.TestAfferentCoupling),
			TestEfferentCoupling:          countToProto(function.TestEfferentCoupling),
			IncomingCallSites:             countToProto(function.IncomingCallSites),
			OutgoingCallSites:             countToProto(function.OutgoingCallSites),
			TestIncomingCallSites:         countToProto(function.TestIncomingCallSites),
			TestOutgoingCallSites:         countToProto(function.TestOutgoingCallSites),
			CrossComponentCallerFunctions: countToProto(function.CrossComponentCallerFunctions),
			CrossComponentCalleeFunctions: countToProto(function.CrossComponentCalleeFunctions),
			TransitiveCallerFunctions:     countToProto(function.TransitiveCallerFunctions),
			TransitiveCalleeFunctions:     countToProto(function.TransitiveCalleeFunctions),
			UsingApplicationCount:         countToProto(function.UsingApplicationCount),
			UsingApplications:             slices.Clone(function.UsingApplications),
			InCycle:                       function.InCycle,
			Instability:                   function.Instability,
		})
	}
	return result
}

func functionCallsToProto(calls []FunctionCall) []*goconductv1.FunctionCall {
	result := make([]*goconductv1.FunctionCall, 0, len(calls))
	for _, call := range calls {
		result = append(result, &goconductv1.FunctionCall{
			Source:          call.Source,
			Target:          call.Target,
			SourceComponent: call.SourceComponent,
			TargetComponent: call.TargetComponent,
			CallSites:       callSitesToProto(call.CallSites),
			Calls:           countToProto(call.Calls),
			TestOnly:        call.TestOnly,
			CrossComponent:  call.CrossComponent,
		})
	}
	return result
}

func callSitesToProto(sites []CallSite) []*goconductv1.CallSite {
	result := make([]*goconductv1.CallSite, 0, len(sites))
	for _, site := range sites {
		result = append(result, &goconductv1.CallSite{
			Path:   site.Path,
			Line:   countToProto(site.Line),
			Column: countToProto(site.Column),
		})
	}
	return result
}

func cyclesToProto(cycles [][]string) []*goconductv1.Cycle {
	result := make([]*goconductv1.Cycle, 0, len(cycles))
	for _, cycle := range cycles {
		result = append(result, &goconductv1.Cycle{Members: slices.Clone(cycle)})
	}
	return result
}

func diagnosticsToProto(diagnostics []Diagnostic) []*goconductv1.Diagnostic {
	result := make([]*goconductv1.Diagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		result = append(result, &goconductv1.Diagnostic{
			Path:    diagnostic.Path,
			Message: diagnostic.Message,
		})
	}
	return result
}

func findingsToProto(findings []Finding) []*goconductv1.Finding {
	result := make([]*goconductv1.Finding, 0, len(findings))
	for _, finding := range findings {
		result = append(result, &goconductv1.Finding{
			Rule:       finding.Rule,
			Severity:   string(finding.Severity),
			Subject:    finding.Subject,
			Message:    finding.Message,
			Source:     finding.Source,
			Target:     finding.Target,
			Components: slices.Clone(finding.Components),
			Metrics:    cloneFloatMap(finding.Metrics),
		})
	}
	return result
}

func cloneFloatMap(source map[string]float64) map[string]float64 {
	if source == nil {
		return nil
	}
	result := make(map[string]float64, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func countToProto(value int) uint32 {
	if value <= 0 {
		return 0
	}
	const maximum = ^uint32(0)
	if uint64(value) > uint64(maximum) {
		return maximum
	}
	return uint32(value)
}

package quality

import (
	"context"

	"connectrpc.com/connect"
	"github.com/samber/do/v2"

	"github.com/cgardev/goconduct/internal/appmodule"
	goconductv1 "github.com/cgardev/goconduct/internal/protogen/v1"
	"github.com/cgardev/goconduct/internal/protogen/v1/goconductv1connect"
	"github.com/cgardev/goconduct/plugin"
)

// QualityAPI maps the application quality use cases onto Connect RPC.
type QualityAPI struct {
	goconductv1connect.UnimplementedQualityServiceHandler
	scopes appmodule.ScopeResolver
}

var _ goconductv1connect.QualityServiceHandler = (*QualityAPI)(nil)

func newQualityAPIInjector() func(do.Injector) {
	return do.Lazy[*QualityAPI](func(injector do.Injector) (*QualityAPI, error) {
		scopes, err := do.Invoke[appmodule.ScopeResolver](injector)
		if err != nil {
			return nil, err
		}
		return NewQualityAPI(scopes), nil
	})
}

// NewQualityAPI creates the Connect quality handler.
func NewQualityAPI(scopes appmodule.ScopeResolver) *QualityAPI {
	return &QualityAPI{scopes: scopes}
}

// ListPlugins returns every evaluator registered in the active scope.
func (api *QualityAPI) ListPlugins(
	ctx context.Context,
	_ *connect.Request[goconductv1.ListPluginsRequest],
) (*connect.Response[goconductv1.ListPluginsResponse], error) {
	useCase, err := appmodule.Resolve[*ListPluginsUseCase](api.scopes, ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	names, err := useCase.Execute(ctx, ListPluginsUseCaseParams{})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	descriptors := make([]*goconductv1.PluginDescriptor, 0, len(names))
	for _, name := range names {
		descriptors = append(descriptors, &goconductv1.PluginDescriptor{Name: name})
	}
	return connect.NewResponse(&goconductv1.ListPluginsResponse{Plugins: descriptors}), nil
}

// RunCheck executes selected evaluators and returns normalized evidence.
func (api *QualityAPI) RunCheck(
	ctx context.Context,
	request *connect.Request[goconductv1.RunCheckRequest],
) (*connect.Response[goconductv1.RunCheckResponse], error) {
	useCase, err := appmodule.Resolve[*RunCheckUseCase](api.scopes, ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	result, err := useCase.Execute(ctx, RunCheckUseCaseParams{
		RepositoryRoot: request.Msg.GetRepositoryRoot(),
		Plugins:        request.Msg.GetPlugins(),
		Paths:          request.Msg.GetPaths(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return connect.NewResponse(checkResultToProto(result)), nil
}

func checkResultToProto(result CheckResult) *goconductv1.RunCheckResponse {
	reports := make([]*goconductv1.PluginReport, 0, len(result.Reports))
	for _, report := range result.Reports {
		reports = append(reports, pluginReportToProto(report))
	}
	return &goconductv1.RunCheckResponse{
		Summary: &goconductv1.QualitySummary{
			Plugins: uint32(result.Summary.Plugins), Findings: uint32(result.Summary.Findings),
			Metrics: uint32(result.Summary.Metrics), Notices: uint32(result.Summary.Notices),
			Warnings: uint32(result.Summary.Warnings), Errors: uint32(result.Summary.Errors),
		},
		Reports: reports,
	}
}

func pluginReportToProto(report plugin.Report) *goconductv1.PluginReport {
	metrics := make([]*goconductv1.QualityMetric, 0, len(report.Metrics))
	for _, metric := range report.Metrics {
		metrics = append(metrics, &goconductv1.QualityMetric{
			Id: metric.ID, Path: metric.Path, Name: metric.Name, Value: metric.Value, Unit: metric.Unit,
		})
	}
	findings := make([]*goconductv1.QualityFinding, 0, len(report.Findings))
	for _, finding := range report.Findings {
		findings = append(findings, &goconductv1.QualityFinding{
			Id: finding.ID, Rule: finding.Rule, Path: finding.Path,
			Severity: severityToProto(finding.Severity), Message: finding.Message,
			Actual: finding.Actual, Limit: finding.Limit,
		})
	}
	return &goconductv1.PluginReport{
		SchemaVersion: uint32(report.SchemaVersion), Plugin: report.Plugin,
		Metrics: metrics, Findings: findings,
	}
}

func severityToProto(severity plugin.Severity) goconductv1.FindingSeverity {
	switch severity {
	case plugin.SeverityNotice:
		return goconductv1.FindingSeverity_FINDING_SEVERITY_NOTICE
	case plugin.SeverityWarning:
		return goconductv1.FindingSeverity_FINDING_SEVERITY_WARNING
	case plugin.SeverityError:
		return goconductv1.FindingSeverity_FINDING_SEVERITY_ERROR
	default:
		return goconductv1.FindingSeverity_FINDING_SEVERITY_UNSPECIFIED
	}
}

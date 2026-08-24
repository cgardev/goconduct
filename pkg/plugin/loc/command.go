package loc

import (
	"encoding/json"
	"io"

	"github.com/spf13/cobra"

	querymodel "github.com/cgardev/goconduct/internal/query"
	"github.com/cgardev/goconduct/pkg/failure"
	"github.com/cgardev/goconduct/pkg/plugin"
)

type locReportLoader func(*cobra.Command) (plugin.Report, error)

func newLOCSummaryCommand(loadReport locReportLoader) *cobra.Command {
	return &cobra.Command{
		Use:   "summary",
		Short: "Write repository-wide LOC evidence as focused JSON.",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			report, err := loadReport(command)
			if err != nil {
				return err
			}
			result, err := Summary(report)
			if err != nil {
				return err
			}
			return writeLOCQueryJSON(command.OutOrStdout(), result)
		},
	}
}

func newLOCPackagesCommand(loadReport locReportLoader) *cobra.Command {
	var sortOrder string
	var limit int
	command := &cobra.Command{
		Use:   "packages",
		Short: "Write sorted package LOC evidence as focused JSON.",
		Example: "  goconduct loc packages --sort handwritten --limit 20\n" +
			"  goconduct loc packages --sort maximum-function --limit 10",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			selectedSort, err := ParseAggregateSort(sortOrder)
			if err != nil {
				return err
			}
			if err := queryLimit(limit); err != nil {
				return err
			}
			report, err := loadReport(command)
			if err != nil {
				return err
			}
			result, err := Packages(report, PackagesParams{Sort: selectedSort, Limit: limit})
			if err != nil {
				return err
			}
			return writeLOCQueryJSON(command.OutOrStdout(), result)
		},
	}
	command.Flags().StringVar(
		&sortOrder,
		"sort",
		string(AggregateSortTotal),
		"Select path, total, handwritten, test, generated, code, comment, blank, functions, "+
			"average-function, p95-function, or maximum-function.",
	)
	command.Flags().IntVar(&limit, "limit", 20, "Return at most this number of packages. Zero returns all packages.")
	return command
}

func newLOCFilesCommand(loadReport locReportLoader) *cobra.Command {
	var packagePath string
	var kind string
	var sortOrder string
	var limit int
	command := &cobra.Command{
		Use:   "files",
		Short: "Write filtered and sorted file LOC evidence as focused JSON.",
		Example: "  goconduct loc files --kind handwritten --sort total --limit 20\n" +
			"  goconduct loc files --package internal/query --sort functions",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			selectedKind, err := ParseSourceKind(kind)
			if err != nil {
				return err
			}
			selectedSort, err := ParseFileSort(sortOrder)
			if err != nil {
				return err
			}
			if err := queryLimit(limit); err != nil {
				return err
			}
			report, err := loadReport(command)
			if err != nil {
				return err
			}
			result, err := Files(report, FilesParams{
				Package: packagePath, Kind: selectedKind, Sort: selectedSort, Limit: limit,
			})
			if err != nil {
				return err
			}
			return writeLOCQueryJSON(command.OutOrStdout(), result)
		},
	}
	command.Flags().StringVar(&packagePath, "package", "", "Select one exact repository package path.")
	command.Flags().StringVar(
		&kind,
		"kind",
		string(SourceKindHandwritten),
		"Select all, handwritten, test, or generated sources.",
	)
	command.Flags().StringVar(
		&sortOrder,
		"sort",
		string(FileSortTotal),
		"Select path, total, code, comment, blank, or functions.",
	)
	command.Flags().IntVar(&limit, "limit", 20, "Return at most this number of files. Zero returns all files.")
	return command
}

func newLOCFunctionsCommand(loadReport locReportLoader) *cobra.Command {
	var packagePath string
	var filePath string
	var kind string
	var sortOrder string
	var limit int
	command := &cobra.Command{
		Use:   "functions",
		Short: "Write filtered and sorted function LOC evidence as focused JSON.",
		Example: "  goconduct loc functions --kind handwritten --sort code --limit 20\n" +
			"  goconduct loc functions --package internal/query --sort total",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			selectedKind, err := ParseSourceKind(kind)
			if err != nil {
				return err
			}
			selectedSort, err := ParseFunctionSort(sortOrder)
			if err != nil {
				return err
			}
			if err := queryLimit(limit); err != nil {
				return err
			}
			report, err := loadReport(command)
			if err != nil {
				return err
			}
			result, err := Functions(report, FunctionsParams{
				Package: packagePath, File: filePath, Kind: selectedKind,
				Sort: selectedSort, Limit: limit,
			})
			if err != nil {
				return err
			}
			return writeLOCQueryJSON(command.OutOrStdout(), result)
		},
	}
	command.Flags().StringVar(&packagePath, "package", "", "Select one exact repository package path.")
	command.Flags().StringVar(&filePath, "file", "", "Select one exact repository file path.")
	command.Flags().StringVar(
		&kind,
		"kind",
		string(SourceKindHandwritten),
		"Select all, handwritten, test, or generated sources.",
	)
	command.Flags().StringVar(
		&sortOrder,
		"sort",
		string(FunctionSortCode),
		"Select identifier, total, code, comment, or blank.",
	)
	command.Flags().IntVar(
		&limit,
		"limit",
		20,
		"Return at most this number of functions. Zero returns all functions.",
	)
	return command
}

func queryLimit(limit int) error {
	return querymodel.ValidateLimit(limit)
}

func writeLOCQueryJSON(destination io.Writer, result any) error {
	encoder := json.NewEncoder(destination)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		return failure.Unavailable("write LOC query JSON", err)
	}
	return nil
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-23T21:34:41Z","module_hash":"5e359ce86bc4b620ebf6ebf7de86904a2f4c08c55b26c93bc592b2cc597f612e","functions":[{"id":"func/newLOCSummaryCommand","name":"newLOCSummaryCommand","line":16,"end_line":33,"hash":"c73ffa1f893a298fd03be88ffa0e30be2595a99b1615c6be981e4bdfb9129f6d"},{"id":"func/newLOCPackagesCommand","name":"newLOCPackagesCommand","line":35,"end_line":72,"hash":"c1b6f2b347f691e40b7d897d426944dc00636ab627c67631121978e01e6f0be7"},{"id":"func/newLOCFilesCommand","name":"newLOCFilesCommand","line":74,"end_line":125,"hash":"72b72b57b2100f02ba2d9253c79e6921e0092719c2290ae08ff365c129638a7f"},{"id":"func/newLOCFunctionsCommand","name":"newLOCFunctionsCommand","line":127,"end_line":186,"hash":"2e915d9148163fcfc0c6391451774101be71ad8eb974a492a5b46f9f672a9b18"},{"id":"func/queryLimit","name":"queryLimit","line":188,"end_line":190,"hash":"ce2c11b48272b549e69c9c36faee9a8128ecfb75b30fb6e6031c0f47ce16a396"},{"id":"func/writeLOCQueryJSON","name":"writeLOCQueryJSON","line":192,"end_line":200,"hash":"461265bfb17cbc6acf03d061caf3755bbe2308cd7ae22f1ecd548e2fbc8cfd13"}]}
// mutate4go-manifest-end

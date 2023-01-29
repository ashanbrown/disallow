package analyzer

import (
	"flag"
	"go/ast"

	"github.com/ashanbrown/forbidigo/forbidigo"
	"github.com/pkg/errors"
	"golang.org/x/tools/go/analysis"
)

type listVar struct {
	values *[]string
}

func (v *listVar) Set(value string) error {
	*v.values = append(*v.values, value)
	if value == "" {
		return errors.New("value cannot be empty")
	}
	return nil
}

func (v *listVar) String() string {
	return ""
}

type analyzer struct {
	patterns           []string
	usePermitDirective bool
	includeExamples    bool
	expand             bool
	debugLog           func(format string, args ...interface{})
}

// NewAnalyzer returns a go/analysis-compatible analyzer
// The "-p" argument can be used to add a pattern.
// Set "-examples" to analyze godoc examples
// Set "-permit=false" to ignore "//permit:<identifier>" directives.
func NewAnalyzer() *analysis.Analyzer {
	return newAnalyzer(nil /* no debug output */)
}

func newAnalyzer(debugLog func(format string, args ...interface{})) *analysis.Analyzer {
	var flags flag.FlagSet
	a := analyzer{
		usePermitDirective: true,
		includeExamples:    true,
		debugLog:           debugLog,
	}

	flags.Var(&listVar{values: &a.patterns}, "p", "pattern")
	flags.BoolVar(&a.includeExamples, "examples", false, "check godoc examples")
	flags.BoolVar(&a.usePermitDirective, "permit", true, `when set, lines with "//permit" directives will be ignored`)
	flags.BoolVar(&a.expand, "analyze_types", false, `when set, expressions get expanded instead of matching the literal source code`)
	return &analysis.Analyzer{
		Name:  "forbidigo",
		Doc:   "forbid identifiers",
		Run:   a.runAnalysis,
		Flags: flags,
	}
}

func (a *analyzer) runAnalysis(pass *analysis.Pass) (interface{}, error) {
	if a.patterns == nil {
		a.patterns = forbidigo.DefaultPatterns()
	}
	linter, err := forbidigo.NewLinter(a.patterns,
		forbidigo.OptionIgnorePermitDirectives(!a.usePermitDirective),
		forbidigo.OptionExcludeGodocExamples(!a.includeExamples),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to configure linter")
	}
	nodes := make([]ast.Node, 0, len(pass.Files))
	for _, f := range pass.Files {
		nodes = append(nodes, f)
	}
	config := forbidigo.RunConfig{Fset: pass.Fset, DebugLog: a.debugLog, Pkg: pass.Pkg}
	if a.expand {
		config.TypesInfo = pass.TypesInfo
	}
	issues, err := linter.RunWithConfig(config, nodes...)
	if err != nil {
		return nil, err
	}
	reportIssues(pass, issues)
	return nil, nil
}

func reportIssues(pass *analysis.Pass, issues []forbidigo.Issue) {
	for _, i := range issues {
		diag := analysis.Diagnostic{
			Pos:      i.Pos(),
			Message:  i.Details(),
			Category: "restriction",
		}
		pass.Report(diag)
	}
}

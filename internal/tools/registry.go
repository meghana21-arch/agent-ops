package tools

var builtins = map[string]*ToolDef{
	"list_files": {
		Name:        "list_files",
		Description: "List files in a directory within the workspace",
		Risk:        RiskLow,
	},
	"read_file": {
		Name:        "read_file",
		Description: "Read the contents of a file within the workspace",
		Risk:        RiskLow,
	},
	"search_code": {
		Name:        "search_code",
		Description: "Search for a pattern across files in the workspace",
		Risk:        RiskLow,
	},
	"run_tests": {
		Name:        "run_tests",
		Description: "Execute the test suite in the workspace directory",
		Risk:        RiskMedium,
	},
	"summarize_logs": {
		Name:        "summarize_logs",
		Description: "Parse log text and extract errors, warnings, and a summary",
		Risk:        RiskMedium,
	},
	"create_patch": {
		Name:        "create_patch",
		Description: "Generate a unified diff patch between current file and new content",
		Risk:        RiskHigh,
	},
	"apply_patch": {
		Name:        "apply_patch",
		Description: "Apply a unified diff patch to files in the workspace",
		Risk:        RiskCritical,
	},
}

type Registry struct{}

func NewRegistry() *Registry { return &Registry{} }

func (r *Registry) Get(name string) (*ToolDef, bool) {
	def, ok := builtins[name]
	return def, ok
}

func (r *Registry) All() []*ToolDef {
	out := make([]*ToolDef, 0, len(builtins))
	for _, v := range builtins {
		out = append(out, v)
	}
	return out
}

package execution

import (
	"encoding/json"
	"github.com/jensneuse/graphql-go-tools/pkg/ast"
	"github.com/jensneuse/graphql-go-tools/pkg/introspection"
	"github.com/jensneuse/graphql-go-tools/pkg/operationreport"
	"io"
)

func NewSchemaDataSourcePlanner(definition *ast.Document, report *operationreport.Report, baseDataSourcePlanner BaseDataSourcePlanner) *SchemaDataSourcePlanner {
	gen := introspection.NewGenerator()
	var data introspection.Data
	gen.Generate(definition, report, &data)
	schemaBytes, err := json.Marshal(data)
	if err != nil {
		report.AddInternalError(err)
	}
	return &SchemaDataSourcePlanner{
		schemaBytes:           schemaBytes,
		BaseDataSourcePlanner: baseDataSourcePlanner,
	}
}

type SchemaDataSourcePlanner struct {
	BaseDataSourcePlanner
	schemaBytes []byte
}

func (s *SchemaDataSourcePlanner) DirectiveDefinition() []byte {
	return nil
}

func (s *SchemaDataSourcePlanner) DataSourceName() string {
	return "resolveSchema"
}

func (s *SchemaDataSourcePlanner) Initialize(config DataSourcePlannerConfiguration) (err error) {
	return nil
}

func (s *SchemaDataSourcePlanner) EnterInlineFragment(ref int) {

}

func (s *SchemaDataSourcePlanner) LeaveInlineFragment(ref int) {

}

func (s *SchemaDataSourcePlanner) EnterSelectionSet(ref int) {

}

func (s *SchemaDataSourcePlanner) LeaveSelectionSet(ref int) {

}

func (s *SchemaDataSourcePlanner) EnterField(ref int) {

}

func (s *SchemaDataSourcePlanner) LeaveField(ref int) {

}

func (s *SchemaDataSourcePlanner) Plan() (DataSource, []Argument) {
	return &SchemaDataSource{
		schemaBytes: s.schemaBytes,
	}, s.args
}

type SchemaDataSource struct {
	schemaBytes []byte
}

func (s *SchemaDataSource) Resolve(ctx Context, args ResolvedArgs, out io.Writer) Instruction {
	_, _ = out.Write(s.schemaBytes)
	return CloseConnectionIfNotStream
}

package execution

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/jensneuse/graphql-go-tools/internal/pkg/unsafebytes"
	"github.com/jensneuse/graphql-go-tools/pkg/ast"
	"github.com/jensneuse/graphql-go-tools/pkg/astvisitor"
	"github.com/jensneuse/graphql-go-tools/pkg/lexer/literal"
	"github.com/jensneuse/graphql-go-tools/pkg/operationreport"
	"github.com/jensneuse/pipeline/pkg/pipe"
	"io"
	"os"
)

type Planner struct {
	walker  *astvisitor.Walker
	visitor *planningVisitor
}

type DataSourceDefinition struct {
	// the type name to which the data source is attached
	TypeName []byte
	// the field on the type to which the data source is attached
	FieldName []byte
	// a factory method to return a new planner
	DataSourcePlannerFactory func() DataSourcePlanner
}

type TypeFieldConfiguration struct {
	TypeName   string
	FieldName  string
	Mapping    *MappingConfiguration
	DataSource DataSourceConfig `json:"data_source"`
}

type DataSourceConfig struct {
	// Kind defines the unique identifier of the DataSource
	// Kind needs to match to the DataSourcePlanner "DataSourceName" name
	Name string `json:"kind"`
	// Config is the DataSource specific configuration object
	// Each DataSourcePlanner needs to make sure to parse their Config Object correctly
	Config json.RawMessage `json:"config"`
}

type MappingConfiguration struct {
	Disabled bool
	Path     string
}

type PlannerConfiguration struct {
	TypeFieldConfigurations []TypeFieldConfiguration
}

func (p PlannerConfiguration) dataSourceNameForTypeField(typeName, fieldName string) *string {
	for i := range p.TypeFieldConfigurations {
		if p.TypeFieldConfigurations[i].TypeName == typeName && p.TypeFieldConfigurations[i].FieldName == fieldName {
			return &p.TypeFieldConfigurations[i].DataSource.Name
		}
	}
	return nil
}

func (p PlannerConfiguration) dataSourceConfigForTypeField(typeName, fieldName string) io.Reader {
	for i := range p.TypeFieldConfigurations {
		if p.TypeFieldConfigurations[i].TypeName == typeName && p.TypeFieldConfigurations[i].FieldName == fieldName {
			return bytes.NewReader(p.TypeFieldConfigurations[i].DataSource.Config)
		}
	}
	return nil
}

func (p PlannerConfiguration) mappingForTypeField(typeName, fieldName string) *MappingConfiguration {
	for i := range p.TypeFieldConfigurations {
		if p.TypeFieldConfigurations[i].TypeName == typeName && p.TypeFieldConfigurations[i].FieldName == fieldName {
			return p.TypeFieldConfigurations[i].Mapping
		}
	}
	return nil
}

type ResolverDefinitions []DataSourceDefinition

func (r ResolverDefinitions) DefinitionForTypeField(typeName, fieldName []byte, definition *DataSourceDefinition) (exists bool) {
	for i := 0; i < len(r); i++ {
		if bytes.Equal(typeName, r[i].TypeName) && bytes.Equal(fieldName, r[i].FieldName) {
			*definition = r[i]
			return true
		}
	}
	return false
}

func NewPlanner(resolverDefinitions ResolverDefinitions, config PlannerConfiguration) *Planner {
	walker := astvisitor.NewWalker(48)
	visitor := planningVisitor{
		Walker:              &walker,
		resolverDefinitions: resolverDefinitions,
		config:              config,
	}

	walker.RegisterEnterDocumentVisitor(&visitor)
	walker.RegisterEnterFieldVisitor(&visitor)
	walker.RegisterLeaveFieldVisitor(&visitor)
	walker.RegisterEnterSelectionSetVisitor(&visitor)
	walker.RegisterLeaveSelectionSetVisitor(&visitor)
	walker.RegisterEnterInlineFragmentVisitor(&visitor)
	walker.RegisterLeaveInlineFragmentVisitor(&visitor)

	return &Planner{
		walker:  &walker,
		visitor: &visitor,
	}
}

func (p *Planner) Plan(operation, definition *ast.Document, report *operationreport.Report) RootNode {
	p.walker.Walk(operation, definition, report)
	return p.visitor.rootNode
}

type planningVisitor struct {
	*astvisitor.Walker
	config                PlannerConfiguration
	resolverDefinitions   ResolverDefinitions
	operation, definition *ast.Document
	rootNode              RootNode
	currentNode           []Node
	planners              []dataSourcePlannerRef
}

type dataSourcePlannerRef struct {
	path     ast.Path
	fieldRef int
	planner  DataSourcePlanner
}

func (p *planningVisitor) EnterDocument(operation, definition *ast.Document) {
	p.operation, p.definition = operation, definition
	obj := &Object{}
	p.rootNode = &Object{
		operationType: operation.OperationDefinitions[0].OperationType,
		Fields: []Field{
			{
				Name:  literal.DATA,
				Value: obj,
			},
		},
	}
	p.currentNode = p.currentNode[:0]
	p.currentNode = append(p.currentNode, obj)
}

func (p *planningVisitor) EnterInlineFragment(ref int) {
	if len(p.planners) != 0 {
		p.planners[len(p.planners)-1].planner.EnterInlineFragment(ref)
	}
}

func (p *planningVisitor) LeaveInlineFragment(ref int) {
	if len(p.planners) != 0 {
		p.planners[len(p.planners)-1].planner.LeaveInlineFragment(ref)
	}
}

func (p *planningVisitor) EnterField(ref int) {

	definition, exists := p.FieldDefinition(ref)
	if !exists {
		return
	}

	resolverTypeName := p.definition.NodeResolverTypeName(p.EnclosingTypeDefinition, p.Path)

	var resolverDefinition DataSourceDefinition
	hasResolverDefinition := p.resolverDefinitions.DefinitionForTypeField(resolverTypeName, p.operation.FieldNameBytes(ref), &resolverDefinition)
	if hasResolverDefinition {

		p.planners = append(p.planners, dataSourcePlannerRef{
			path:     p.Path,
			fieldRef: ref,
			planner:  resolverDefinition.DataSourcePlannerFactory(),
		})

		config := DataSourcePlannerConfiguration{
			operation:               p.operation,
			definition:              p.definition,
			walker:                  p.Walker,
			dataSourceConfiguration: p.fieldDataSourceConfig(ref),
		}

		err := p.planners[len(p.planners)-1].planner.Initialize(config)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
	}

	if len(p.planners) != 0 {
		p.planners[len(p.planners)-1].planner.EnterField(ref)
	}

	switch parent := p.currentNode[len(p.currentNode)-1].(type) {
	case *Object:

		var skipCondition BooleanCondition
		ancestor := p.Ancestors[len(p.Ancestors)-2]
		if ancestor.Kind == ast.NodeKindInlineFragment {
			typeConditionName := p.operation.InlineFragmentTypeConditionName(ancestor.Ref)
			skipCondition = &IfNotEqual{
				Left: &ObjectVariableArgument{
					PathSelector: PathSelector{
						Path: "__typename",
					},
				},
				Right: &StaticVariableArgument{
					Value: typeConditionName,
				},
			}
		}

		dataResolvingConfig := p.fieldDataResolvingConfig(ref)

		var value Node
		fieldDefinitionType := p.definition.FieldDefinitionType(definition)
		if p.definition.TypeIsList(fieldDefinitionType) {

			if !p.operation.FieldHasSelections(ref) {
				value = &Value{
					ValueType: p.jsonValueType(fieldDefinitionType),
				}
			} else {
				value = &Object{}
			}

			list := &List{
				DataResolvingConfig: dataResolvingConfig,
				Value:               value,
			}

			firstNValue, ok := p.FieldDefinitionDirectiveArgumentValueByName(ref, []byte("ListFilterFirstN"), []byte("n"))
			if ok {
				if firstNValue.Kind == ast.ValueKindInteger {
					firstN := p.definition.IntValueAsInt(firstNValue.Ref)
					list.Filter = &ListFilterFirstN{
						FirstN: int(firstN),
					}
				}
			}

			parent.Fields = append(parent.Fields, Field{
				Name:  p.operation.FieldNameBytes(ref),
				Value: list,
				Skip:  skipCondition,
			})

			p.currentNode = append(p.currentNode, value)
			return
		}

		if !p.operation.FieldHasSelections(ref) {
			value = &Value{
				DataResolvingConfig: dataResolvingConfig,
				ValueType:           p.jsonValueType(fieldDefinitionType),
			}
		} else {
			value = &Object{
				DataResolvingConfig: dataResolvingConfig,
			}
		}

		parent.Fields = append(parent.Fields, Field{
			Name:  p.operation.FieldObjectNameBytes(ref),
			Value: value,
			Skip:  skipCondition,
		})

		p.currentNode = append(p.currentNode, value)
	}
}

func (p *planningVisitor) LeaveField(ref int) {

	var plannedDataSource DataSource
	var plannedArgs []Argument

	if len(p.planners) != 0 {

		p.planners[len(p.planners)-1].planner.LeaveField(ref)

		if p.planners[len(p.planners)-1].path.Equals(p.Path) && p.planners[len(p.planners)-1].fieldRef == ref {
			plannedDataSource, plannedArgs = p.planners[len(p.planners)-1].planner.Plan()
			p.planners = p.planners[:len(p.planners)-1]

			if len(p.currentNode) >= 2 {
				switch parent := p.currentNode[len(p.currentNode)-2].(type) {
				case *Object:
					for i := 0; i < len(parent.Fields); i++ {
						if bytes.Equal(p.operation.FieldObjectNameBytes(ref), parent.Fields[i].Name) {

							pathName := p.operation.FieldObjectNameString(ref)
							parent.Fields[i].HasResolvedData = true

							singleFetch := &SingleFetch{
								Source: &DataSourceInvocation{
									Args:       plannedArgs,
									DataSource: plannedDataSource,
								},
								BufferName: pathName,
							}

							if parent.Fetch == nil {
								parent.Fetch = singleFetch
							} else {
								switch fetch := parent.Fetch.(type) {
								case *ParallelFetch:
									fetch.Fetches = append(fetch.Fetches, singleFetch)
								case *SerialFetch:
									fetch.Fetches = append(fetch.Fetches, singleFetch)
								case *SingleFetch:
									first := *fetch
									parent.Fetch = &ParallelFetch{
										Fetches: []Fetch{
											&first,
											singleFetch,
										},
									}
								}
							}
						}
					}
				}
			}
		}
	}

	p.currentNode = p.currentNode[:len(p.currentNode)-1]
}

func (p *planningVisitor) EnterSelectionSet(ref int) {
	if len(p.planners) != 0 {
		p.planners[len(p.planners)-1].planner.EnterSelectionSet(ref)
	}
}

func (p *planningVisitor) LeaveSelectionSet(ref int) {
	if len(p.planners) != 0 {
		p.planners[len(p.planners)-1].planner.LeaveSelectionSet(ref)
	}
}

func (p *planningVisitor) jsonValueType(valueType int) JSONValueType {
	typeName := p.definition.ResolveTypeName(valueType)
	switch {
	case bytes.Equal(typeName, literal.INT):
		return IntegerValueType
	case bytes.Equal(typeName, literal.BOOLEAN):
		return BooleanValueType
	case bytes.Equal(typeName, literal.FLOAT):
		return FloatValueType
	default:
		return StringValueType
	}
}

func (p *planningVisitor) fieldDataResolvingConfig(ref int) DataResolvingConfig {
	return DataResolvingConfig{
		PathSelector:   p.fieldPathSelector(ref),
		Transformation: p.fieldTransformation(ref),
	}
}

func (p *planningVisitor) fieldDataSourceConfig(ref int) io.Reader {
	fieldName := p.operation.FieldNameString(ref)
	typeName := p.definition.NodeNameString(p.EnclosingTypeDefinition)
	return p.config.dataSourceConfigForTypeField(typeName, fieldName)
}

func (p *planningVisitor) fieldPathSelector(ref int) (selector PathSelector) {
	fieldName := p.operation.FieldNameString(ref)
	typeName := p.definition.NodeNameString(p.EnclosingTypeDefinition)
	mapping := p.config.mappingForTypeField(typeName, fieldName)
	if mapping == nil {
		selector.Path = fieldName
		return
	}
	if mapping.Disabled {
		return
	}
	selector.Path = mapping.Path
	return
}

func (p *planningVisitor) fieldTransformation(ref int) Transformation {
	definition, ok := p.FieldDefinition(ref)
	if !ok {
		return nil
	}
	transformationDirective, ok := p.definition.FieldDefinitionDirectiveByName(definition, literal.TRANSFORMATION)
	if !ok {
		return nil
	}
	modeValue, ok := p.definition.DirectiveArgumentValueByName(transformationDirective, literal.MODE)
	if !ok || modeValue.Kind != ast.ValueKindEnum {
		return nil
	}
	mode := unsafebytes.BytesToString(p.definition.EnumValueNameBytes(modeValue.Ref))
	switch mode {
	case "PIPELINE":
		return p.pipelineTransformation(transformationDirective)
	default:
		return nil
	}
}

func (p *planningVisitor) pipelineTransformation(directive int) *PipelineTransformation {
	var configReader io.Reader
	configFileStringValue, ok := p.definition.DirectiveArgumentValueByName(directive, literal.PIPELINE_CONFIG_FILE)
	if ok && configFileStringValue.Kind == ast.ValueKindString {
		reader, err := os.Open(p.definition.StringValueContentString(configFileStringValue.Ref))
		if err != nil {
			return nil
		}
		defer reader.Close()
		configReader = reader
	}
	configStringValue, ok := p.definition.DirectiveArgumentValueByName(directive, literal.PIPELINE_CONFIG_STRING)
	if ok && configStringValue.Kind == ast.ValueKindString {
		configReader = bytes.NewReader(p.definition.StringValueContentBytes(configStringValue.Ref))
	}
	if configReader == nil {
		return nil
	}
	var pipeline pipe.Pipeline
	err := pipeline.FromConfig(configReader)
	if err != nil {
		return nil
	}
	return &PipelineTransformation{
		pipeline: pipeline,
	}
}

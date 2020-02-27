package execution

import (
	"bytes"
	"encoding/json"
	"github.com/buger/jsonparser"
	log "github.com/jensneuse/abstractlogger"
	"github.com/jensneuse/graphql-go-tools/internal/pkg/unsafebytes"
	"github.com/jensneuse/graphql-go-tools/pkg/ast"
	"github.com/jensneuse/graphql-go-tools/pkg/astimport"
	"github.com/jensneuse/graphql-go-tools/pkg/astprinter"
	"github.com/jensneuse/graphql-go-tools/pkg/lexer/literal"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

// GraphQLDataSourceConfig is the configuration for the GraphQL DataSource
type GraphQLDataSourceConfig struct {
	// Host is the hostname of the upstream
	Host string
	// URL is the url of the upstream
	URL string
	// Method is the http.Method of the upstream, defaults to POST (optional)
	Method *string
}

type GraphQLDataSourcePlanner struct {
	BaseDataSourcePlanner
	importer                *astimport.Importer
	nodes                   []ast.Node
	resolveDocument         *ast.Document
	dataSourceConfiguration GraphQLDataSourceConfig
}

func NewGraphQLDataSourcePlanner(baseDataSourcePlanner BaseDataSourcePlanner) *GraphQLDataSourcePlanner {
	return &GraphQLDataSourcePlanner{
		BaseDataSourcePlanner: baseDataSourcePlanner,
		importer:              &astimport.Importer{},
	}
}

func (g *GraphQLDataSourcePlanner) DataSourceName() string {
	return "GraphQLDataSource"
}

func (g *GraphQLDataSourcePlanner) Initialize(config DataSourcePlannerConfiguration) (err error) {
	g.walker, g.operation, g.definition = config.walker, config.operation, config.definition
	g.resolveDocument = &ast.Document{}
	if config.dataSourceConfiguration == nil {
		return nil
	}
	return json.NewDecoder(config.dataSourceConfiguration).Decode(&g.dataSourceConfiguration)
}

func (g *GraphQLDataSourcePlanner) EnterInlineFragment(ref int) {
	if len(g.nodes) == 0 {
		return
	}
	current := g.nodes[len(g.nodes)-1]
	if current.Kind != ast.NodeKindSelectionSet {
		return
	}
	inlineFragmentType := g.importer.ImportType(g.operation.InlineFragments[ref].TypeCondition.Type, g.operation, g.resolveDocument)
	g.resolveDocument.InlineFragments = append(g.resolveDocument.InlineFragments, ast.InlineFragment{
		TypeCondition: ast.TypeCondition{
			Type: inlineFragmentType,
		},
		SelectionSet: -1,
	})
	inlineFragmentRef := len(g.resolveDocument.InlineFragments) - 1
	g.resolveDocument.Selections = append(g.resolveDocument.Selections, ast.Selection{
		Kind: ast.SelectionKindInlineFragment,
		Ref:  inlineFragmentRef,
	})
	selectionRef := len(g.resolveDocument.Selections) - 1
	g.resolveDocument.SelectionSets[current.Ref].SelectionRefs = append(g.resolveDocument.SelectionSets[current.Ref].SelectionRefs, selectionRef)
	g.nodes = append(g.nodes, ast.Node{
		Kind: ast.NodeKindInlineFragment,
		Ref:  inlineFragmentRef,
	})
}

func (g *GraphQLDataSourcePlanner) LeaveInlineFragment(ref int) {
	g.nodes = g.nodes[:len(g.nodes)-1]
}

func (g *GraphQLDataSourcePlanner) EnterSelectionSet(ref int) {

	fieldOrInlineFragment := g.nodes[len(g.nodes)-1]

	set := ast.SelectionSet{}
	g.resolveDocument.SelectionSets = append(g.resolveDocument.SelectionSets, set)
	setRef := len(g.resolveDocument.SelectionSets) - 1

	switch fieldOrInlineFragment.Kind {
	case ast.NodeKindField:
		g.resolveDocument.Fields[fieldOrInlineFragment.Ref].HasSelections = true
		g.resolveDocument.Fields[fieldOrInlineFragment.Ref].SelectionSet = setRef
	case ast.NodeKindInlineFragment:
		g.resolveDocument.InlineFragments[fieldOrInlineFragment.Ref].HasSelections = true
		g.resolveDocument.InlineFragments[fieldOrInlineFragment.Ref].SelectionSet = setRef
	}

	g.nodes = append(g.nodes, ast.Node{
		Kind: ast.NodeKindSelectionSet,
		Ref:  setRef,
	})
}

func (g *GraphQLDataSourcePlanner) LeaveSelectionSet(ref int) {
	g.nodes = g.nodes[:len(g.nodes)-1]
}

func (g *GraphQLDataSourcePlanner) EnterField(ref int) {
	if !g.rootField.isDefined {
		g.rootField.setIfNotDefined(ref)

		typeName := g.definition.NodeNameString(g.walker.EnclosingTypeDefinition)
		fieldNameStr := g.operation.FieldNameString(ref)
		fieldName := g.operation.FieldNameBytes(ref)
		mapping := g.config.mappingForTypeField(typeName, fieldNameStr)
		if mapping != nil && !mapping.Disabled {
			fieldName = unsafebytes.StringToBytes(mapping.Path)
		}

		hasArguments := g.operation.FieldHasArguments(ref)
		var argumentRefs []int
		if hasArguments {
			argumentRefs = g.importer.ImportArguments(g.operation.FieldArguments(ref), g.operation, g.resolveDocument)
		}

		field := ast.Field{
			Name: g.resolveDocument.Input.AppendInputBytes(fieldName),
			Arguments: ast.ArgumentList{
				Refs: argumentRefs,
			},
			HasArguments: hasArguments,
		}
		g.resolveDocument.Fields = append(g.resolveDocument.Fields, field)
		fieldRef := len(g.resolveDocument.Fields) - 1
		selection := ast.Selection{
			Kind: ast.SelectionKindField,
			Ref:  fieldRef,
		}
		g.resolveDocument.Selections = append(g.resolveDocument.Selections, selection)
		selectionRef := len(g.resolveDocument.Selections) - 1
		set := ast.SelectionSet{
			SelectionRefs: []int{selectionRef},
		}
		g.resolveDocument.SelectionSets = append(g.resolveDocument.SelectionSets, set)
		setRef := len(g.resolveDocument.SelectionSets) - 1
		hasVariableDefinitions := len(g.operation.OperationDefinitions[g.walker.Ancestors[0].Ref].VariableDefinitions.Refs) != 0
		var variableDefinitionsRefs []int
		if hasVariableDefinitions {
			variableDefinitionsRefs = g.importer.ImportVariableDefinitions(g.operation.OperationDefinitions[g.walker.Ancestors[0].Ref].VariableDefinitions.Refs, g.operation, g.resolveDocument)
			for _, i := range variableDefinitionsRefs {
				name := g.resolveDocument.VariableDefinitionNameBytes(i)
				g.args = append(g.args, &ContextVariableArgument{
					Name:         name,
					VariableName: name,
				})
			}
		}
		operationDefinition := ast.OperationDefinition{
			Name:          g.resolveDocument.Input.AppendInputBytes([]byte("o")),
			OperationType: g.operation.OperationDefinitions[g.walker.Ancestors[0].Ref].OperationType,
			SelectionSet:  setRef,
			HasSelections: true,
			VariableDefinitions: ast.VariableDefinitionList{
				Refs: variableDefinitionsRefs,
			},
			HasVariableDefinitions: hasVariableDefinitions,
		}
		g.resolveDocument.OperationDefinitions = append(g.resolveDocument.OperationDefinitions, operationDefinition)
		operationDefinitionRef := len(g.resolveDocument.OperationDefinitions) - 1
		g.resolveDocument.RootNodes = append(g.resolveDocument.RootNodes, ast.Node{
			Kind: ast.NodeKindOperationDefinition,
			Ref:  operationDefinitionRef,
		})
		g.nodes = append(g.nodes, ast.Node{
			Kind: ast.NodeKindOperationDefinition,
			Ref:  operationDefinitionRef,
		})
		g.nodes = append(g.nodes, ast.Node{
			Kind: ast.NodeKindSelectionSet,
			Ref:  setRef,
		})
		g.nodes = append(g.nodes, ast.Node{
			Kind: ast.NodeKindField,
			Ref:  fieldRef,
		})
	} else {
		field := ast.Field{
			Name: g.resolveDocument.Input.AppendInputBytes(g.operation.FieldNameBytes(ref)),
		}
		g.resolveDocument.Fields = append(g.resolveDocument.Fields, field)
		fieldRef := len(g.resolveDocument.Fields) - 1
		set := g.nodes[len(g.nodes)-1]
		selection := ast.Selection{
			Kind: ast.SelectionKindField,
			Ref:  fieldRef,
		}
		g.resolveDocument.Selections = append(g.resolveDocument.Selections, selection)
		selectionRef := len(g.resolveDocument.Selections) - 1
		g.resolveDocument.SelectionSets[set.Ref].SelectionRefs = append(g.resolveDocument.SelectionSets[set.Ref].SelectionRefs, selectionRef)
		g.nodes = append(g.nodes, ast.Node{
			Kind: ast.NodeKindField,
			Ref:  fieldRef,
		})
	}
}

func (g *GraphQLDataSourcePlanner) LeaveField(ref int) {
	defer func() {
		g.nodes = g.nodes[:len(g.nodes)-1]
	}()
	if g.rootField.ref != ref {
		return
	}
	buff := bytes.Buffer{}
	err := astprinter.Print(g.resolveDocument, nil, &buff)
	if err != nil {
		g.walker.StopWithInternalErr(err)
		return
	}
	g.args = append(g.args, &StaticVariableArgument{
		Name:  literal.HOST,
		Value: []byte(g.dataSourceConfiguration.Host),
	})
	g.args = append(g.args, &StaticVariableArgument{
		Name:  literal.URL,
		Value: []byte(g.dataSourceConfiguration.URL),
	})
	g.args = append(g.args, &StaticVariableArgument{
		Name:  literal.QUERY,
		Value: buff.Bytes(),
	})
	if g.dataSourceConfiguration.Method == nil {
		g.args = append(g.args, &StaticVariableArgument{
			Name:  literal.METHOD,
			Value: literal.HTTP_METHOD_POST,
		})
	} else {
		g.args = append(g.args, &StaticVariableArgument{
			Name:  literal.URL,
			Value: []byte(*g.dataSourceConfiguration.Method),
		})
	}
}

func (g *GraphQLDataSourcePlanner) Plan() (DataSource, []Argument) {
	return &GraphQLDataSource{
		log: g.log,
	}, g.args
}

type GraphQLDataSource struct {
	log log.Logger
}

func (g *GraphQLDataSource) Resolve(ctx Context, args ResolvedArgs, out io.Writer) Instruction {

	hostArg := args.ByKey(literal.HOST)
	urlArg := args.ByKey(literal.URL)
	queryArg := args.ByKey(literal.QUERY)

	g.log.Debug("GraphQLDataSource.Resolve.args",
		log.Strings("resolvedArgs", args.Dump()),
	)

	if hostArg == nil || urlArg == nil || queryArg == nil {
		g.log.Error("GraphQLDataSource.args invalid")
		return CloseConnectionIfNotStream
	}

	url := string(hostArg) + string(urlArg)
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		url = "https://" + url
	}

	variables := map[string]interface{}{}
	for i := 0; i < len(args); i++ {
		key := args[i].Key
		switch {
		case bytes.Equal(key, literal.HOST):
		case bytes.Equal(key, literal.URL):
		case bytes.Equal(key, literal.QUERY):
		default:
			variables[string(key)] = string(args[i].Value)
		}
	}

	variablesJson, err := json.Marshal(variables)
	if err != nil {
		g.log.Error("GraphQLDataSource.json.Marshal(variables)",
			log.Error(err),
		)
		return CloseConnectionIfNotStream
	}

	gqlRequest := GraphqlRequest{
		OperationName: "o",
		Variables:     variablesJson,
		Query:         string(queryArg),
	}

	gqlRequestData, err := json.MarshalIndent(gqlRequest, "", "  ")
	if err != nil {
		g.log.Error("GraphQLDataSource.json.MarshalIndent",
			log.Error(err),
		)
		return CloseConnectionIfNotStream
	}

	g.log.Debug("GraphQLDataSource.request",
		log.String("url", url),
		log.ByteString("data", gqlRequestData),
	)

	client := http.Client{
		Timeout: time.Second * 10,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 1024,
			TLSHandshakeTimeout: 0 * time.Second,
		},
	}

	request, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(gqlRequestData))
	if err != nil {
		g.log.Error("GraphQLDataSource.http.NewRequest",
			log.Error(err),
		)
		return CloseConnectionIfNotStream
	}

	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Accept", "application/json")

	res, err := client.Do(request)
	if err != nil {
		g.log.Error("GraphQLDataSource.client.Do",
			log.Error(err),
		)
		return CloseConnectionIfNotStream
	}
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		g.log.Error("GraphQLDataSource.ioutil.ReadAll",
			log.Error(err),
		)
		return CloseConnectionIfNotStream
	}

	data = bytes.ReplaceAll(data, literal.BACKSLASH, nil)
	data, _, _, err = jsonparser.Get(data, "data")
	if err != nil {
		g.log.Error("GraphQLDataSource.jsonparser.Get",
			log.Error(err),
		)
		return CloseConnectionIfNotStream
	}
	_, err = out.Write(data)
	if err != nil {
		g.log.Error("GraphQLDataSource.out.Write",
			log.Error(err),
		)
		return CloseConnectionIfNotStream
	}
	return CloseConnectionIfNotStream
}

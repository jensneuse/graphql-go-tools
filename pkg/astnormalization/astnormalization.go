/*Package astnormalization helps to transform parsed GraphQL AST's into a easier to use structure.

Example

This examples shows how the normalization package helps "simplifying" a GraphQL AST.

Input:

 subscription sub {
 	... multipleSubscriptions
	... on Subscription {
		newMessage {
			body
			sender
		}
	}
 }
 fragment newMessageFields on Message {
 	body: body
 	sender
 	... on Body {
 		body
 	}
 }
 fragment multipleSubscriptions on Subscription {
 	newMessage {
 		body
 		sender
 	}
 	newMessage {
 		... newMessageFields
 	}
 	newMessage {
 		body
 		body
		sender
 	}
 	... on Subscription {
 		newMessage {
 			body
 			sender
 		}
 	}
 	disallowedSecondRootField
 }

Output:

 subscription sub {
 	newMessage {
 		body
 		sender
 	}
 	disallowedSecondRootField
 }
 fragment newMessageFields on Message {
 	body
 	sender
 }
 fragment multipleSubscriptions on Subscription {
 	newMessage {
 		body
 		sender
 	}
 	disallowedSecondRootField
 }
*/
package astnormalization

import (
	"github.com/jensneuse/graphql-go-tools/pkg/ast"
	"github.com/jensneuse/graphql-go-tools/pkg/astvisitor"
	"github.com/jensneuse/graphql-go-tools/pkg/operationreport"
)

// NormalizeOperation creates a default Normalizer and applies all rules to a given AST
// In case you're using OperationNormalizer in a hot path you shouldn't be using this function.
// Create a new OperationNormalizer using NewNormalizer() instead and re-use it.
func NormalizeOperation(operation, definition *ast.Document, report *operationreport.Report) {
	normalizer := NewNormalizer(false, false)
	normalizer.NormalizeOperation(operation, definition, report)
}

func NormalizeNamedOperation(operation, definition *ast.Document, operationName []byte, report *operationreport.Report) {
	normalizer := NewNormalizer(true, true)
	normalizer.NormalizeNamedOperation(operation, definition, operationName, report)
}

type registerNormalizeFunc func(walker *astvisitor.Walker)

type registerNormalizeVariablesFunc func(walker *astvisitor.Walker) *variablesExtractionVisitor

type registerNormalizeDeleteVariablesFunc func(walker *astvisitor.Walker) *deleteUnusedVariablesVisitor

// OperationNormalizer walks a given AST and applies all registered rules
type OperationNormalizer struct {
	operationWalkers        []*astvisitor.Walker
	prepareDefinitionWalker *astvisitor.Walker
	variablesExtraction     *variablesExtractionVisitor
	options                 options
}

// NewNormalizer creates a new OperationNormalizer and sets up all default rules
func NewNormalizer(removeFragmentDefinitions, extractVariables bool) *OperationNormalizer {
	normalizer := &OperationNormalizer{
		options: options{
			removeUnusedVariables: removeFragmentDefinitions,
			extractVariables:      extractVariables,
		},
	}
	normalizer.setupOperationWalkers()
	normalizer.setupDefinitionWalkers()
	return normalizer
}

// NewWithOpts creates a new OperationNormalizer with Options
func NewWithOpts(opts ...Option) *OperationNormalizer {
	var options options
	for _, opt := range opts {
		opt(&options)
	}
	normalizer := &OperationNormalizer{
		options: options,
	}
	normalizer.setupOperationWalkers()
	normalizer.setupDefinitionWalkers()
	return normalizer
}

type options struct {
	removeFragmentDefinitions bool
	extractVariables          bool
	removeUnusedVariables     bool
}

type Option func(options *options)

func WithExtractVariables() Option {
	return func(options *options) {
		options.extractVariables = true
	}
}

func WithRemoveFragmentDefinitions() Option {
	return func(options *options) {
		options.removeFragmentDefinitions = true
	}
}

func WithRemoveUnusedVariables() Option {
	return func(options *options) {
		options.removeUnusedVariables = true
	}
}

func (o *OperationNormalizer) setupOperationWalkers() {
	fragmentInline := astvisitor.NewWalker(48)
	fragmentSpreadInline(&fragmentInline)
	directiveIncludeSkip(&fragmentInline)

	other := astvisitor.NewWalker(48)
	removeSelfAliasing(&other)
	mergeInlineFragments(&other)
	mergeFieldSelections(&other)
	deduplicateFields(&other)
	if o.options.extractVariables {
		o.variablesExtraction = extractVariables(&other)
	}
	if o.options.removeFragmentDefinitions {
		removeFragmentDefinitions(&other)
	}
	if o.options.removeUnusedVariables {
		deleteUnusedVariables(&other)
	}
	o.operationWalkers = append(o.operationWalkers, &fragmentInline, &other)
}

func (o *OperationNormalizer) setupDefinitionWalkers() {
	walker := astvisitor.NewWalker(48)

	extendObjectTypeDefinition(&walker)
	extendInputObjectTypeDefinition(&walker)
	extendEnumTypeDefinition(&walker)
	extendInterfaceTypeDefinition(&walker)
	extendScalarTypeDefinition(&walker)
	extendUnionTypeDefinition(&walker)
	removeMergedTypeExtensions(&walker)

	o.prepareDefinitionWalker = &walker
}

func (o *OperationNormalizer) prepareDefinition(definition *ast.Document, report *operationreport.Report) {
	o.prepareDefinitionWalker.Walk(definition, nil, report)
	if report.HasErrors() {
		return
	}
}

// NormalizeOperation applies all registered rules to the AST
func (o *OperationNormalizer) NormalizeOperation(operation, definition *ast.Document, report *operationreport.Report) {
	o.prepareDefinition(definition, report)

	for i := range o.operationWalkers {
		o.operationWalkers[i].Walk(operation, definition, report)
		if report.HasErrors() {
			return
		}
	}
}

// NormalizeNamedOperation applies all registered rules to one specific named operation in the AST
func (o *OperationNormalizer) NormalizeNamedOperation(operation, definition *ast.Document, operationName []byte, report *operationreport.Report) {
	o.prepareDefinition(definition, report)

	if o.variablesExtraction != nil {
		o.variablesExtraction.operationName = operationName
	}
	for i := range o.operationWalkers {
		o.operationWalkers[i].Walk(operation, definition, report)
		if report.HasErrors() {
			return
		}
	}
}

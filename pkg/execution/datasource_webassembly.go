package execution

import (
	"github.com/jensneuse/graphql-go-tools/pkg/ast"
	"github.com/jensneuse/graphql-go-tools/pkg/astvisitor"
	"github.com/jensneuse/graphql-go-tools/pkg/lexer/literal"
	wasm "github.com/wasmerio/go-ext-wasm/wasmer"
	"go.uber.org/zap"
	"io"
	"sync"
)

func NewWasmDataSourcePlanner(baseDataSourcePlanner BaseDataSourcePlanner) *WasmDataSourcePlanner {
	return &WasmDataSourcePlanner{
		BaseDataSourcePlanner: baseDataSourcePlanner,
	}
}

type WasmDataSourcePlanner struct {
	BaseDataSourcePlanner
}

func (w *WasmDataSourcePlanner) DirectiveDefinition() []byte {
	data, _ := w.graphqlDefinitions.Find("directives/wasm_datasource.graphql")
	return data
}

func (w *WasmDataSourcePlanner) DirectiveName() []byte {
	return []byte("WasmDataSource")
}

func (w *WasmDataSourcePlanner) Initialize(walker *astvisitor.Walker, operation, definition *ast.Document, args []Argument, resolverParameters []ResolverParameter) {
	w.walker, w.operation, w.definition, w.args = walker, operation, definition, args
}

func (w *WasmDataSourcePlanner) EnterInlineFragment(ref int) {

}

func (w *WasmDataSourcePlanner) LeaveInlineFragment(ref int) {

}

func (w *WasmDataSourcePlanner) EnterSelectionSet(ref int) {

}

func (w *WasmDataSourcePlanner) LeaveSelectionSet(ref int) {

}

func (w *WasmDataSourcePlanner) EnterField(ref int) {
	fieldDefinition, ok := w.walker.FieldDefinition(ref)
	if !ok {
		return
	}
	directive, ok := w.definition.FieldDefinitionDirectiveByName(fieldDefinition, w.DirectiveName())
	if !ok {
		return
	}

	value, ok := w.definition.DirectiveArgumentValueByName(directive, literal.WASMFILE)
	if !ok {
		return
	}
	staticValue := w.definition.StringValueContentBytes(value.Ref)
	w.args = append(w.args, &StaticVariableArgument{
		Name:  literal.WASMFILE,
		Value: staticValue,
	})

	value, ok = w.definition.DirectiveArgumentValueByName(directive, literal.INPUT)
	if !ok {
		return
	}
	staticValue = w.definition.StringValueContentBytes(value.Ref)
	w.args = append(w.args, &StaticVariableArgument{
		Name:  literal.INPUT,
		Value: staticValue,
	})
}

func (w *WasmDataSourcePlanner) LeaveField(ref int) {

}

func (w *WasmDataSourcePlanner) Plan() (DataSource, []Argument) {
	return &WasmDataSource{
		log:w.log,
	}, w.args
}

type WasmDataSource struct {
	log      *zap.Logger
	instance wasm.Instance
	once     sync.Once
}

func (s *WasmDataSource) Resolve(ctx Context, args ResolvedArgs, out io.Writer) Instruction {

	input := args.ByKey(literal.INPUT)
	wasmFile := args.ByKey(literal.WASMFILE)

	s.once.Do(func() {
		wasmData, err := wasm.ReadBytes(string(wasmFile))
		if err != nil {
			s.log.Error("WasmDataSource.wasm.ReadBytes(string(wasmFile))",
				zap.Error(err),
			)
			return
		}
		s.instance, err = wasm.NewInstance(wasmData)
		if err != nil {
			s.log.Error("WasmDataSource.wasm.NewInstance(wasmData)",
				zap.Error(err),
			)
		}
	})

	inputLen := len(input)

	allocateInputResult, err := s.instance.Exports["allocate"](inputLen)
	if err != nil {
		s.log.Error("WasmDataSource.instance.Exports[\"allocate\"](inputLen)",
			zap.Error(err),
		)
		return CloseConnectionIfNotStream
	}

	inputPointer := allocateInputResult.ToI32()

	memory := s.instance.Memory.Data()[inputPointer:]

	for i := 0; i < inputLen; i++ {
		memory[i] = input[i]
	}

	memory[inputLen] = 0

	result, err := s.instance.Exports["invoke"](inputPointer)
	if err != nil {
		s.log.Error("WasmDataSource.instance.Exports[\"invoke\"](inputPointer)",
			zap.Error(err),
		)
		return CloseConnectionIfNotStream
	}

	start := result.ToI32()
	memory = s.instance.Memory.Data()

	var stop int32

	for i := start; i < int32(len(memory)); i++ {
		if memory[i] == 0 {
			stop = i
			break
		}
	}

	_,err = out.Write(memory[start:stop])
	if err != nil {
		s.log.Error("WasmDataSource.out.Write(memory[start:stop])",
			zap.Error(err),
		)
		return CloseConnectionIfNotStream
	}

	deallocate := s.instance.Exports["deallocate"]
	_, err = deallocate(inputPointer, inputLen)
	if err != nil {
		s.log.Error("WasmDataSource.deallocate(inputPointer, inputLen)",
			zap.Error(err),
		)
		return CloseConnectionIfNotStream
	}

	_, err = deallocate(start, stop-start)
	if err != nil {
		s.log.Error("WasmDataSource.deallocate(start, stop-start)",
			zap.Error(err),
		)
		return CloseConnectionIfNotStream
	}

	return CloseConnectionIfNotStream
}

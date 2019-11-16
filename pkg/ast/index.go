package ast

// Index is a struct to easily look up objects in a document, e.g. find Nodes (type/interface/union definitions) by name
type Index struct {
	QueryTypeName           ByteSlice
	MutationTypeName        ByteSlice
	SubscriptionTypeName    ByteSlice
	Nodes                   map[uint64]Node
	ReplacedFragmentSpreads []int
	MergedTypeExtensions    []Node
}

func (i *Index) Reset() {
	i.QueryTypeName = i.QueryTypeName[:0]
	i.MutationTypeName = i.MutationTypeName[:0]
	i.SubscriptionTypeName = i.SubscriptionTypeName[:0]
	i.ReplacedFragmentSpreads = i.ReplacedFragmentSpreads[:0]
	i.MergedTypeExtensions = i.MergedTypeExtensions[:0]
	for j := range i.Nodes {
		delete(i.Nodes, j)
	}
}

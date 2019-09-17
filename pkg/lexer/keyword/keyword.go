//go:generate stringer -type=Keyword
package keyword

type Keyword int

const (
	UNDEFINED Keyword = iota
	IDENT
	EXTEND
	COMMENT
	EOF

	COLON
	BANG
	LINETERMINATOR
	TAB
	SPACE
	COMMA
	AT
	DOT
	SPREAD
	PIPE
	SLASH
	EQUALS
	SUB
	AND
	ON
	QUOTE

	IMPLEMENTS
	SCHEMA
	SCALAR
	TYPE
	INTERFACE
	UNION
	ENUM
	INPUT
	DIRECTIVE

	DOLLAR
	STRING
	BLOCKSTRING
	INTEGER
	FLOAT
	TRUE
	FALSE
	NULL
	QUERY
	MUTATION
	SUBSCRIPTION
	FRAGMENT

	LPAREN
	RPAREN
	LBRACK
	RBRACK
	LBRACE
	RBRACE
)
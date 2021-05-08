package testsgo

import (
	"testing"

	"github.com/jensneuse/graphql-go-tools/pkg/astvalidation/reference/helpers"
)

func TestNoUndefinedVariablesRule(t *testing.T) {

	expectErrors := func(queryStr string) helpers.ResultCompare {
		return helpers.ExpectValidationErrors("NoUndefinedVariablesRule", queryStr)
	}

	expectValid := func(queryStr string) {
		expectErrors(queryStr)(`[]`)
	}

	t.Run("Validate: No undefined variables", func(t *testing.T) {
		t.Run("all variables defined", func(t *testing.T) {
			expectValid(`
      query Foo($a: String, $b: String, $c: String) {
        field(a: $a, b: $b, c: $c)
      }
    `)
		})

		t.Run("all variables deeply defined", func(t *testing.T) {
			expectValid(`
      query Foo($a: String, $b: String, $c: String) {
        field(a: $a) {
          field(b: $b) {
            field(c: $c)
          }
        }
      }
    `)
		})

		t.Run("all variables deeply in inline fragments defined", func(t *testing.T) {
			expectValid(`
      query Foo($a: String, $b: String, $c: String) {
        ... on Type {
          field(a: $a) {
            field(b: $b) {
              ... on Type {
                field(c: $c)
              }
            }
          }
        }
      }
    `)
		})

		t.Run("all variables in fragments deeply defined", func(t *testing.T) {
			expectValid(`
      query Foo($a: String, $b: String, $c: String) {
        ...FragA
      }
      fragment FragA on Type {
        field(a: $a) {
          ...FragB
        }
      }
      fragment FragB on Type {
        field(b: $b) {
          ...FragC
        }
      }
      fragment FragC on Type {
        field(c: $c)
      }
    `)
		})

		t.Run("variable within single fragment defined in multiple operations", func(t *testing.T) {
			expectValid(`
      query Foo($a: String) {
        ...FragA
      }
      query Bar($a: String) {
        ...FragA
      }
      fragment FragA on Type {
        field(a: $a)
      }
    `)
		})

		t.Run("variable within fragments defined in operations", func(t *testing.T) {
			expectValid(`
      query Foo($a: String) {
        ...FragA
      }
      query Bar($b: String) {
        ...FragB
      }
      fragment FragA on Type {
        field(a: $a)
      }
      fragment FragB on Type {
        field(b: $b)
      }
    `)
		})

		t.Run("variable within recursive fragment defined", func(t *testing.T) {
			expectValid(`
      query Foo($a: String) {
        ...FragA
      }
      fragment FragA on Type {
        field(a: $a) {
          ...FragA
        }
      }
    `)
		})

		t.Run("variable not defined", func(t *testing.T) {
			expectErrors(`
      query Foo($a: String, $b: String, $c: String) {
        field(a: $a, b: $b, c: $c, d: $d)
      }
    `)(`[
      {
        message: 'Variable "$d" is not defined by operation "Foo".',
        locations: [
          { line: 3, column: 39 },
          { line: 2, column: 7 },
        ],
      },
]`)
		})

		t.Run("variable not defined by un-named query", func(t *testing.T) {
			expectErrors(`
      {
        field(a: $a)
      }
    `)(`[
      {
        message: 'Variable "$a" is not defined.',
        locations: [
          { line: 3, column: 18 },
          { line: 2, column: 7 },
        ],
      },
]`)
		})

		t.Run("multiple variables not defined", func(t *testing.T) {
			expectErrors(`
      query Foo($b: String) {
        field(a: $a, b: $b, c: $c)
      }
    `)(`[
      {
        message: 'Variable "$a" is not defined by operation "Foo".',
        locations: [
          { line: 3, column: 18 },
          { line: 2, column: 7 },
        ],
      },
      {
        message: 'Variable "$c" is not defined by operation "Foo".',
        locations: [
          { line: 3, column: 32 },
          { line: 2, column: 7 },
        ],
      },
]`)
		})

		t.Run("variable in fragment not defined by un-named query", func(t *testing.T) {
			expectErrors(`
      {
        ...FragA
      }
      fragment FragA on Type {
        field(a: $a)
      }
    `)(`[
      {
        message: 'Variable "$a" is not defined.',
        locations: [
          { line: 6, column: 18 },
          { line: 2, column: 7 },
        ],
      },
]`)
		})

		t.Run("variable in fragment not defined by operation", func(t *testing.T) {
			expectErrors(`
      query Foo($a: String, $b: String) {
        ...FragA
      }
      fragment FragA on Type {
        field(a: $a) {
          ...FragB
        }
      }
      fragment FragB on Type {
        field(b: $b) {
          ...FragC
        }
      }
      fragment FragC on Type {
        field(c: $c)
      }
    `)(`[
      {
        message: 'Variable "$c" is not defined by operation "Foo".',
        locations: [
          { line: 16, column: 18 },
          { line: 2, column: 7 },
        ],
      },
]`)
		})

		t.Run("multiple variables in fragments not defined", func(t *testing.T) {
			expectErrors(`
      query Foo($b: String) {
        ...FragA
      }
      fragment FragA on Type {
        field(a: $a) {
          ...FragB
        }
      }
      fragment FragB on Type {
        field(b: $b) {
          ...FragC
        }
      }
      fragment FragC on Type {
        field(c: $c)
      }
    `)(`[
      {
        message: 'Variable "$a" is not defined by operation "Foo".',
        locations: [
          { line: 6, column: 18 },
          { line: 2, column: 7 },
        ],
      },
      {
        message: 'Variable "$c" is not defined by operation "Foo".',
        locations: [
          { line: 16, column: 18 },
          { line: 2, column: 7 },
        ],
      },
]`)
		})

		t.Run("single variable in fragment not defined by multiple operations", func(t *testing.T) {
			expectErrors(`
      query Foo($a: String) {
        ...FragAB
      }
      query Bar($a: String) {
        ...FragAB
      }
      fragment FragAB on Type {
        field(a: $a, b: $b)
      }
    `)(`[
      {
        message: 'Variable "$b" is not defined by operation "Foo".',
        locations: [
          { line: 9, column: 25 },
          { line: 2, column: 7 },
        ],
      },
      {
        message: 'Variable "$b" is not defined by operation "Bar".',
        locations: [
          { line: 9, column: 25 },
          { line: 5, column: 7 },
        ],
      },
]`)
		})

		t.Run("variables in fragment not defined by multiple operations", func(t *testing.T) {
			expectErrors(`
      query Foo($b: String) {
        ...FragAB
      }
      query Bar($a: String) {
        ...FragAB
      }
      fragment FragAB on Type {
        field(a: $a, b: $b)
      }
    `)(`[
      {
        message: 'Variable "$a" is not defined by operation "Foo".',
        locations: [
          { line: 9, column: 18 },
          { line: 2, column: 7 },
        ],
      },
      {
        message: 'Variable "$b" is not defined by operation "Bar".',
        locations: [
          { line: 9, column: 25 },
          { line: 5, column: 7 },
        ],
      },
]`)
		})

		t.Run("variable in fragment used by other operation", func(t *testing.T) {
			expectErrors(`
      query Foo($b: String) {
        ...FragA
      }
      query Bar($a: String) {
        ...FragB
      }
      fragment FragA on Type {
        field(a: $a)
      }
      fragment FragB on Type {
        field(b: $b)
      }
    `)(`[
      {
        message: 'Variable "$a" is not defined by operation "Foo".',
        locations: [
          { line: 9, column: 18 },
          { line: 2, column: 7 },
        ],
      },
      {
        message: 'Variable "$b" is not defined by operation "Bar".',
        locations: [
          { line: 12, column: 18 },
          { line: 5, column: 7 },
        ],
      },
]`)
		})

		t.Run("multiple undefined variables produce multiple errors", func(t *testing.T) {
			expectErrors(`
      query Foo($b: String) {
        ...FragAB
      }
      query Bar($a: String) {
        ...FragAB
      }
      fragment FragAB on Type {
        field1(a: $a, b: $b)
        ...FragC
        field3(a: $a, b: $b)
      }
      fragment FragC on Type {
        field2(c: $c)
      }
    `)(`[
      {
        message: 'Variable "$a" is not defined by operation "Foo".',
        locations: [
          { line: 9, column: 19 },
          { line: 2, column: 7 },
        ],
      },
      {
        message: 'Variable "$a" is not defined by operation "Foo".',
        locations: [
          { line: 11, column: 19 },
          { line: 2, column: 7 },
        ],
      },
      {
        message: 'Variable "$c" is not defined by operation "Foo".',
        locations: [
          { line: 14, column: 19 },
          { line: 2, column: 7 },
        ],
      },
      {
        message: 'Variable "$b" is not defined by operation "Bar".',
        locations: [
          { line: 9, column: 26 },
          { line: 5, column: 7 },
        ],
      },
      {
        message: 'Variable "$b" is not defined by operation "Bar".',
        locations: [
          { line: 11, column: 26 },
          { line: 5, column: 7 },
        ],
      },
      {
        message: 'Variable "$c" is not defined by operation "Bar".',
        locations: [
          { line: 14, column: 19 },
          { line: 5, column: 7 },
        ],
      },
]`)
		})
	})

}

package testsgo

import (
	"testing"

	"github.com/jensneuse/graphql-go-tools/pkg/astvalidation/reference/helpers"
)

func TestFieldsOnCorrectTypeRule(t *testing.T) {

	expectErrors := func(queryStr string) helpers.ResultCompare {
		return helpers.ExpectValidationErrors("FieldsOnCorrectTypeRule", queryStr)
	}

	expectValid := func(queryStr string) {
		expectErrors(queryStr)(`[]`)
	}

	t.Run("Validate: Fields on correct type", func(t *testing.T) {
		t.Run("Object field selection", func(t *testing.T) {
			expectValid(`
      fragment objectFieldSelection on Dog {
        __typename
        name
      }
    `)
		})

		t.Run("Aliased object field selection", func(t *testing.T) {
			expectValid(`
      fragment aliasedObjectFieldSelection on Dog {
        tn : __typename
        otherName : name
      }
    `)
		})

		t.Run("Interface field selection", func(t *testing.T) {
			expectValid(`
      fragment interfaceFieldSelection on Pet {
        __typename
        name
      }
    `)
		})

		t.Run("Aliased interface field selection", func(t *testing.T) {
			expectValid(`
      fragment interfaceFieldSelection on Pet {
        otherName : name
      }
    `)
		})

		t.Run("Lying alias selection", func(t *testing.T) {
			expectValid(`
      fragment lyingAliasSelection on Dog {
        name : nickname
      }
    `)
		})

		t.Run("Ignores fields on unknown type", func(t *testing.T) {
			expectValid(`
      fragment unknownSelection on UnknownType {
        unknownField
      }
    `)
		})

		t.Run("reports errors when type is known again", func(t *testing.T) {
			expectErrors(`
      fragment typeKnownAgain on Pet {
        unknown_pet_field {
          ... on Cat {
            unknown_cat_field
          }
        }
      }
    `)(`[
      {
        message: 'Cannot query field "unknown_pet_field" on type "Pet".',
        locations: [{ line: 3, column: 9 }],
      },
      {
        message: 'Cannot query field "unknown_cat_field" on type "Cat".',
        locations: [{ line: 5, column: 13 }],
      },
]`)
		})

		t.Run("Field not defined on fragment", func(t *testing.T) {
			expectErrors(`
      fragment fieldNotDefined on Dog {
        meowVolume
      }
    `)(`[
      {
        message:
          'Cannot query field "meowVolume" on type "Dog". Did you mean "barkVolume"?',
        locations: [{ line: 3, column: 9 }],
      },
]`)
		})

		t.Run("Ignores deeply unknown field", func(t *testing.T) {
			expectErrors(`
      fragment deepFieldNotDefined on Dog {
        unknown_field {
          deeper_unknown_field
        }
      }
    `)(`[
      {
        message: 'Cannot query field "unknown_field" on type "Dog".',
        locations: [{ line: 3, column: 9 }],
      },
]`)
		})

		t.Run("Sub-field not defined", func(t *testing.T) {
			expectErrors(`
      fragment subFieldNotDefined on Human {
        pets {
          unknown_field
        }
      }
    `)(`[
      {
        message: 'Cannot query field "unknown_field" on type "Pet".',
        locations: [{ line: 4, column: 11 }],
      },
]`)
		})

		t.Run("Field not defined on inline fragment", func(t *testing.T) {
			expectErrors(`
      fragment fieldNotDefined on Pet {
        ... on Dog {
          meowVolume
        }
      }
    `)(`[
      {
        message:
          'Cannot query field "meowVolume" on type "Dog". Did you mean "barkVolume"?',
        locations: [{ line: 4, column: 11 }],
      },
]`)
		})

		t.Run("Aliased field target not defined", func(t *testing.T) {
			expectErrors(`
      fragment aliasedFieldTargetNotDefined on Dog {
        volume : mooVolume
      }
    `)(`[
      {
        message:
          'Cannot query field "mooVolume" on type "Dog". Did you mean "barkVolume"?',
        locations: [{ line: 3, column: 9 }],
      },
]`)
		})

		t.Run("Aliased lying field target not defined", func(t *testing.T) {
			expectErrors(`
      fragment aliasedLyingFieldTargetNotDefined on Dog {
        barkVolume : kawVolume
      }
    `)(`[
      {
        message:
          'Cannot query field "kawVolume" on type "Dog". Did you mean "barkVolume"?',
        locations: [{ line: 3, column: 9 }],
      },
]`)
		})

		t.Run("Not defined on interface", func(t *testing.T) {
			expectErrors(`
      fragment notDefinedOnInterface on Pet {
        tailLength
      }
    `)(`[
      {
        message: 'Cannot query field "tailLength" on type "Pet".',
        locations: [{ line: 3, column: 9 }],
      },
]`)
		})

		t.Run("Defined on implementors but not on interface", func(t *testing.T) {
			expectErrors(`
      fragment definedOnImplementorsButNotInterface on Pet {
        nickname
      }
    `)(`[
      {
        message:
          'Cannot query field "nickname" on type "Pet". Did you mean to use an inline fragment on "Cat" or "Dog"?',
        locations: [{ line: 3, column: 9 }],
      },
]`)
		})

		t.Run("Meta field selection on union", func(t *testing.T) {
			expectValid(`
      fragment directFieldSelectionOnUnion on CatOrDog {
        __typename
      }
    `)
		})

		t.Run("Direct field selection on union", func(t *testing.T) {
			expectErrors(`
      fragment directFieldSelectionOnUnion on CatOrDog {
        directField
      }
    `)(`[
      {
        message: 'Cannot query field "directField" on type "CatOrDog".',
        locations: [{ line: 3, column: 9 }],
      },
]`)
		})

		t.Run("Defined on implementors queried on union", func(t *testing.T) {
			expectErrors(`
      fragment definedOnImplementorsQueriedOnUnion on CatOrDog {
        name
      }
    `)(`[
      {
        message:
          'Cannot query field "name" on type "CatOrDog". Did you mean to use an inline fragment on "Being", "Pet", "Canine", "Cat", or "Dog"?',
        locations: [{ line: 3, column: 9 }],
      },
]`)
		})

		t.Run("valid field in inline fragment", func(t *testing.T) {
			expectValid(`
      fragment objectFieldSelection on Pet {
        ... on Dog {
          name
        }
        ... {
          name
        }
      }
    `)
		})

		t.Run("Fields on correct type error message", func(t *testing.T) {
			expectErrorMessage := func(schema string, queryStr string) func(string) {
				return func(string) {
					// TODO: fix me
				}
			}

			t.Run("Works with no suggestions", func(t *testing.T) {
				schema := helpers.BuildSchema(`
        type T {
          fieldWithVeryLongNameThatWillNeverBeSuggested: String
        }
        type Query { t: T }
      `)

				expectErrorMessage(schema, "{ t { f } }")(
					`Cannot query field "f" on type "T".`,
				)
			})

			t.Run("Works with no small numbers of type suggestions", func(t *testing.T) {
				schema := helpers.BuildSchema(`
        union T = A | B
        type Query { t: T }

        type A { f: String }
        type B { f: String }
      `)

				expectErrorMessage(schema, "{ t { f } }")(
					`Cannot query field "f" on type "T". Did you mean to use an inline fragment on "A" or "B"?`,
				)
			})

			t.Run("Works with no small numbers of field suggestions", func(t *testing.T) {
				schema := helpers.BuildSchema(`
        type T {
          y: String
          z: String
        }
        type Query { t: T }
      `)

				expectErrorMessage(schema, "{ t { f } }")(
					`Cannot query field "f" on type "T". Did you mean "y" or "z"?`,
				)
			})

			t.Run("Only shows one set of suggestions at a time, preferring types", func(t *testing.T) {
				schema := helpers.BuildSchema(`
        interface T {
          y: String
          z: String
        }
        type Query { t: T }

        type A implements T {
          f: String
          y: String
          z: String
        }
        type B implements T {
          f: String
          y: String
          z: String
        }
      `)

				expectErrorMessage(schema, "{ t { f } }")(
					`Cannot query field "f" on type "T". Did you mean to use an inline fragment on "A" or "B"?`,
				)
			})

			t.Run("Sort type suggestions based on inheritance order", func(t *testing.T) {
				schema := helpers.BuildSchema(`
        interface T { bar: String }
        type Query { t: T }

        interface Z implements T {
          foo: String
          bar: String
        }

        interface Y implements Z & T {
          foo: String
          bar: String
        }

        type X implements Y & Z & T {
          foo: String
          bar: String
        }
      `)

				expectErrorMessage(schema, "{ t { foo } }")(
					`Cannot query field "foo" on type "T". Did you mean to use an inline fragment on "Z", "Y", or "X"?`,
				)
			})

			t.Run("Limits lots of type suggestions", func(t *testing.T) {
				schema := helpers.BuildSchema(`
        union T = A | B | C | D | E | F
        type Query { t: T }

        type A { f: String }
        type B { f: String }
        type C { f: String }
        type D { f: String }
        type E { f: String }
        type F { f: String }
      `)

				expectErrorMessage(schema, "{ t { f } }")(
					`Cannot query field "f" on type "T". Did you mean to use an inline fragment on "A", "B", "C", "D", or "E"?`,
				)
			})

			t.Run("Limits lots of field suggestions", func(t *testing.T) {
				schema := helpers.BuildSchema(`
        type T {
          u: String
          v: String
          w: String
          x: String
          y: String
          z: String
        }
        type Query { t: T }
      `)

				expectErrorMessage(schema, "{ t { f } }")(
					`Cannot query field "f" on type "T". Did you mean "u", "v", "w", "x", or "y"?`,
				)
			})
		})
	})

}

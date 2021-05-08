package testsgo

import (
	"testing"

	"github.com/jensneuse/graphql-go-tools/pkg/astvalidation/reference/helpers"
)

func TestNoFragmentCyclesRule(t *testing.T) {

	expectErrors := func(queryStr string) helpers.ResultCompare {
		return helpers.ExpectValidationErrors("NoFragmentCyclesRule", queryStr)
	}

	expectValid := func(queryStr string) {
		expectErrors(queryStr)(`[]`)
	}

	t.Run("Validate: No circular fragment spreads", func(t *testing.T) {
		t.Run("single reference is valid", func(t *testing.T) {
			expectValid(`
      fragment fragA on Dog { ...fragB }
      fragment fragB on Dog { name }
    `)
		})

		t.Run("spreading twice is not circular", func(t *testing.T) {
			expectValid(`
      fragment fragA on Dog { ...fragB, ...fragB }
      fragment fragB on Dog { name }
    `)
		})

		t.Run("spreading twice indirectly is not circular", func(t *testing.T) {
			expectValid(`
      fragment fragA on Dog { ...fragB, ...fragC }
      fragment fragB on Dog { ...fragC }
      fragment fragC on Dog { name }
    `)
		})

		t.Run("double spread within abstract types", func(t *testing.T) {
			expectValid(`
      fragment nameFragment on Pet {
        ... on Dog { name }
        ... on Cat { name }
      }

      fragment spreadsInAnon on Pet {
        ... on Dog { ...nameFragment }
        ... on Cat { ...nameFragment }
      }
    `)
		})

		t.Run("does not false positive on unknown fragment", func(t *testing.T) {
			expectValid(`
      fragment nameFragment on Pet {
        ...UnknownFragment
      }
    `)
		})

		t.Run("spreading recursively within field fails", func(t *testing.T) {
			expectErrors(`
      fragment fragA on Human { relatives { ...fragA } },
    `)(`[
      {
        message: 'Cannot spread fragment "fragA" within itself.',
        locations: [{ line: 2, column: 45 }],
      },
]`)
		})

		t.Run("no spreading itself directly", func(t *testing.T) {
			expectErrors(`
      fragment fragA on Dog { ...fragA }
    `)(`[
      {
        message: 'Cannot spread fragment "fragA" within itself.',
        locations: [{ line: 2, column: 31 }],
      },
]`)
		})

		t.Run("no spreading itself directly within inline fragment", func(t *testing.T) {
			expectErrors(`
      fragment fragA on Pet {
        ... on Dog {
          ...fragA
        }
      }
    `)(`[
      {
        message: 'Cannot spread fragment "fragA" within itself.',
        locations: [{ line: 4, column: 11 }],
      },
]`)
		})

		t.Run("no spreading itself indirectly", func(t *testing.T) {
			expectErrors(`
      fragment fragA on Dog { ...fragB }
      fragment fragB on Dog { ...fragA }
    `)(`[
      {
        message: 'Cannot spread fragment "fragA" within itself via "fragB".',
        locations: [
          { line: 2, column: 31 },
          { line: 3, column: 31 },
        ],
      },
]`)
		})

		t.Run("no spreading itself indirectly reports opposite order", func(t *testing.T) {
			expectErrors(`
      fragment fragB on Dog { ...fragA }
      fragment fragA on Dog { ...fragB }
    `)(`[
      {
        message: 'Cannot spread fragment "fragB" within itself via "fragA".',
        locations: [
          { line: 2, column: 31 },
          { line: 3, column: 31 },
        ],
      },
]`)
		})

		t.Run("no spreading itself indirectly within inline fragment", func(t *testing.T) {
			expectErrors(`
      fragment fragA on Pet {
        ... on Dog {
          ...fragB
        }
      }
      fragment fragB on Pet {
        ... on Dog {
          ...fragA
        }
      }
    `)(`[
      {
        message: 'Cannot spread fragment "fragA" within itself via "fragB".',
        locations: [
          { line: 4, column: 11 },
          { line: 9, column: 11 },
        ],
      },
]`)
		})

		t.Run("no spreading itself deeply", func(t *testing.T) {
			expectErrors(`
      fragment fragA on Dog { ...fragB }
      fragment fragB on Dog { ...fragC }
      fragment fragC on Dog { ...fragO }
      fragment fragX on Dog { ...fragY }
      fragment fragY on Dog { ...fragZ }
      fragment fragZ on Dog { ...fragO }
      fragment fragO on Dog { ...fragP }
      fragment fragP on Dog { ...fragA, ...fragX }
    `)(`[
      {
        message:
          'Cannot spread fragment "fragA" within itself via "fragB", "fragC", "fragO", "fragP".',
        locations: [
          { line: 2, column: 31 },
          { line: 3, column: 31 },
          { line: 4, column: 31 },
          { line: 8, column: 31 },
          { line: 9, column: 31 },
        ],
      },
      {
        message:
          'Cannot spread fragment "fragO" within itself via "fragP", "fragX", "fragY", "fragZ".',
        locations: [
          { line: 8, column: 31 },
          { line: 9, column: 41 },
          { line: 5, column: 31 },
          { line: 6, column: 31 },
          { line: 7, column: 31 },
        ],
      },
]`)
		})

		t.Run("no spreading itself deeply two paths", func(t *testing.T) {
			expectErrors(`
      fragment fragA on Dog { ...fragB, ...fragC }
      fragment fragB on Dog { ...fragA }
      fragment fragC on Dog { ...fragA }
    `)(`[
      {
        message: 'Cannot spread fragment "fragA" within itself via "fragB".',
        locations: [
          { line: 2, column: 31 },
          { line: 3, column: 31 },
        ],
      },
      {
        message: 'Cannot spread fragment "fragA" within itself via "fragC".',
        locations: [
          { line: 2, column: 41 },
          { line: 4, column: 31 },
        ],
      },
]`)
		})

		t.Run("no spreading itself deeply two paths -- alt traverse order", func(t *testing.T) {
			expectErrors(`
      fragment fragA on Dog { ...fragC }
      fragment fragB on Dog { ...fragC }
      fragment fragC on Dog { ...fragA, ...fragB }
    `)(`[
      {
        message: 'Cannot spread fragment "fragA" within itself via "fragC".',
        locations: [
          { line: 2, column: 31 },
          { line: 4, column: 31 },
        ],
      },
      {
        message: 'Cannot spread fragment "fragC" within itself via "fragB".',
        locations: [
          { line: 4, column: 41 },
          { line: 3, column: 31 },
        ],
      },
]`)
		})

		t.Run("no spreading itself deeply and immediately", func(t *testing.T) {
			expectErrors(`
      fragment fragA on Dog { ...fragB }
      fragment fragB on Dog { ...fragB, ...fragC }
      fragment fragC on Dog { ...fragA, ...fragB }
    `)(`[
      {
        message: 'Cannot spread fragment "fragB" within itself.',
        locations: [{ line: 3, column: 31 }],
      },
      {
        message:
          'Cannot spread fragment "fragA" within itself via "fragB", "fragC".',
        locations: [
          { line: 2, column: 31 },
          { line: 3, column: 41 },
          { line: 4, column: 31 },
        ],
      },
      {
        message: 'Cannot spread fragment "fragB" within itself via "fragC".',
        locations: [
          { line: 3, column: 41 },
          { line: 4, column: 41 },
        ],
      },
]`)
		})
	})

}

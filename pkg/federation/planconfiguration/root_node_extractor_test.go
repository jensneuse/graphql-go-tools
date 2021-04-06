package planconfiguration

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jensneuse/graphql-go-tools/internal/pkg/unsafeparser"
)

func runRootNodeExtractor(t *testing.T, SDL string, expected []TypeField) {
	document := unsafeparser.ParseGraphqlDocumentString(SDL)
	extractor := NewRootNodeExtractor(&document)
	got := extractor.GetAllRootNodes()
	assert.Equal(t, expected, got)
}

func TestRootNodeExtractor_GetAllRootNodes(t *testing.T) {
	t.Run("non Entity object", func(t *testing.T) {
		runRootNodeExtractor(t, `
		type Review {
			body: String!
			author: User! @provides(fields: "username")
			product: Product!
		}
		`, nil)
	})
	t.Run("non Entity object extension", func(t *testing.T) {
		runRootNodeExtractor(t, `
		type Review {
			body: String!
		}

		extend type Review {
			title: String!
		}
		`, nil)
	})
	t.Run("Entity object", func(t *testing.T) {
		runRootNodeExtractor(t, `
		type Review @key(fields: "id"){
			id: Int!
			body: String!
			title: String
		}
		`, []TypeField{{TypeName: "Review", FieldNames: []string{"id", "body", "title"}}})
	})
	t.Run("Entity object extension", func(t *testing.T) {
		runRootNodeExtractor(t, `
		extend type Review @key(fields: "id"){
			id: Int! @external
			body: String! @external
			title: String @requires(fields: "id body")
			author: String!
		}
		`, []TypeField{{TypeName: "Review", FieldNames: []string{"title", "author"}}})
	})
	t.Run("Root operation types", func(t *testing.T) {
		runRootNodeExtractor(t, `
		extend type Query {
			me: User
		} 

		extend type Mutation {
			addUser(id: ID!): User
			deleteUser(id: ID!): User
		}

		extend type Subscription {
			userChanges(id: ID!): User!
		}

		type User { 
			id: ID! 
		}
		

		`, []TypeField{
			{ TypeName: "Query", FieldNames: []string{"me"}},
			{ TypeName: "Mutation", FieldNames: []string{"addUser", "deleteUser"}},
			{ TypeName: "Subscription", FieldNames: []string{"userChanges"}},
		})
	})
}

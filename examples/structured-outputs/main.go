// Package main demonstrates OpenAI Structured Outputs with AgentKit
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/darkostanimirovic/agentkit"
)

type CreateUserParams struct {
	Email     string `json:"email" required:"true" desc:"User email address"`
	FirstName string `json:"first_name" required:"true" desc:"User's first name"`
	LastName  string `json:"last_name" required:"true" desc:"User's last name"`
	Nickname  string `json:"nickname" desc:"Optional nickname"`
	Age       int    `json:"age" desc:"Optional age"`
}

func main() {
	// Example 1: Using fluent API with Structured Outputs (default)
	createUserTool := agentkit.NewTool("create_user").
		WithDescription("Create a new user account").
		WithParameter("email", agentkit.String().Required().WithDescription("User email")).
		WithParameter("first_name", agentkit.String().Required()).
		WithParameter("last_name", agentkit.String().Required()).
		WithParameter("nickname", agentkit.String().Optional()). // Uses anyOf with null
		WithParameter("age", agentkit.String().Optional()).      // Uses anyOf with null
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			return map[string]any{
				"id":      "user_123",
				"created": true,
			}, nil
		}).
		Build()

	fmt.Println("✅ Tool 1: Fluent API with Structured Outputs enabled")
	fmt.Printf("   Tool created: %s\n", createUserTool.Name())

	// Example 2: Using struct-based schemas (also uses Structured Outputs)
	updateUserToolBuilder, err := agentkit.NewStructTool("update_user",
		func(ctx context.Context, args CreateUserParams) (any, error) {
			return map[string]any{
				"id":      "user_123",
				"updated": true,
			}, nil
		})
	if err != nil {
		log.Fatal(err)
	}

	updateUserTool := updateUserToolBuilder.
		WithDescription("Update an existing user").
		Build()

	fmt.Println("\n✅ Tool 2: Struct-based with automatic Structured Outputs")
	fmt.Printf("   Tool created: %s\n", updateUserTool.Name())

	// Example 3: Complex nested objects using StructToSchema helper
	type SearchFilters struct {
		EmailDomain string `json:"email_domain" desc:"Filter by email domain"`
		AgeRange    struct {
			Min int `json:"min" desc:"Minimum age"`
			Max int `json:"max" desc:"Maximum age"`
		} `json:"age_range"`
		Status string `json:"status" required:"true" enum:"active,inactive,pending" desc:"User status"`
	}

	filtersSchema, _ := agentkit.StructToSchema[SearchFilters]()
	searchTool := agentkit.NewTool("search_users").
		WithDescription("Search for users with complex filters using struct-based schema").
		WithParameter("filters", filtersSchema).
		WithParameter("limit", agentkit.String().Optional()).
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			return map[string]any{"results": []string{"user1", "user2"}}, nil
		}).
		Build()

	fmt.Println("\n✅ Tool 3: Struct-based nested objects with StructToSchema")
	fmt.Printf("   Tool created: %s\n", searchTool.Name())

	// Example 4: Complex nested objects using manual fluent API (alternative approach)
	searchTool2 := agentkit.NewTool("search_users_manual").
		WithDescription("Search for users with complex filters using manual schema building").
		WithParameter("filters", agentkit.Object().
			WithProperty("email_domain", agentkit.String().Optional()).
			WithProperty("age_range", agentkit.Object().
				WithProperty("min", agentkit.String().Optional()).
				WithProperty("max", agentkit.String().Optional()),
			).
			WithProperty("status", agentkit.String().WithEnum("active", "inactive", "pending").Required()).
			Required(),
		).
		WithParameter("limit", agentkit.String().Optional()).
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			return map[string]any{"results": []string{"user1", "user2"}}, nil
		}).
		Build()

	fmt.Println("\n✅ Tool 4: Manual nested objects (alternative to StructToSchema)")
	fmt.Printf("   Tool created: %s\n", searchTool2.Name())

	fmt.Println("\n✅ Tool 4: Manual nested objects (alternative to StructToSchema)")
	fmt.Printf("   Tool created: %s\n", searchTool2.Name())

	// Example 5: Array of objects
	batchCreateTool := agentkit.NewTool("batch_create_users").
		WithDescription("Create multiple users at once").
		WithParameter("users", agentkit.ArrayOf(
			agentkit.Object().
				WithProperty("email", agentkit.String().Required()).
				WithProperty("name", agentkit.String().Required()).
				WithProperty("nickname", agentkit.String().Optional()),
		).Required()).
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			return map[string]any{"created": 5}, nil
		}).
		Build()

	fmt.Println("\n✅ Tool 5: Array of complex objects")
	fmt.Printf("   Tool created: %s\n", batchCreateTool.Name())

	// Example 6: Disabling strict mode (not recommended)
	legacyTool := agentkit.NewTool("legacy_tool").
		WithDescription("Legacy tool without strict schema validation").
		WithParameter("data", agentkit.String().Optional()).
		WithStrictMode(false). // Explicitly disable Structured Outputs
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			return map[string]any{"status": "ok"}, nil
		}).
		Build()

	fmt.Println("\n⚠️  Tool 6: Strict mode disabled (not recommended)")
	fmt.Printf("   Tool created: %s\n", legacyTool.Name())

	fmt.Println("\n✨ All tools created successfully with OpenAI Structured Outputs!")
	fmt.Println("📖 Key benefits:")
	fmt.Println("   • Guaranteed schema adherence - no hallucinated fields")
	fmt.Println("   • Type safety - model output always matches your schema")
	fmt.Println("   • Optional fields use anyOf with null for proper validation")
	fmt.Println("   • additionalProperties: false prevents unexpected fields")
	fmt.Println("\n💡 Two ways to define complex schemas:")
	fmt.Println("   1. StructToSchema[T]() - Use Go structs with tags (recommended for complex types)")
	fmt.Println("   2. Fluent API - Chain WithProperty() calls (good for simple inline schemas)")
}

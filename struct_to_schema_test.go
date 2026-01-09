package agentkit

import (
	"testing"
)

func TestStructToSchema(t *testing.T) {
	type SearchFilters struct {
		EmailDomain string `json:"email_domain" desc:"Filter by email domain"`
		Status      string `json:"status" required:"true" enum:"active,inactive,pending" desc:"User status"`
		AgeRange    struct {
			Min int `json:"min" desc:"Minimum age"`
			Max int `json:"max" desc:"Maximum age"`
		} `json:"age_range"`
	}

	schema, err := StructToSchema[SearchFilters]()
	if err != nil {
		t.Fatalf("StructToSchema failed: %v", err)
	}

	if schema == nil {
		t.Fatal("Expected non-nil schema")
	}

	// Convert to map to inspect
	schemaMap := schema.ToMap()

	// Check it's an object
	if schemaMap["type"] != "object" {
		t.Errorf("Expected type 'object', got %v", schemaMap["type"])
	}

	// Check properties exist
	props, ok := schemaMap["properties"].(map[string]any)
	if !ok {
		t.Fatal("Expected properties to be a map")
	}

	// Check email_domain field
	emailDomain, ok := props["email_domain"]
	if !ok {
		t.Error("Expected email_domain property")
	} else {
		emailMap := emailDomain.(map[string]any)
		if desc := emailMap["description"]; desc != "Filter by email domain" {
			t.Errorf("Expected email_domain description, got %v", desc)
		}
	}

	// Check status field
	status, ok := props["status"]
	if !ok {
		t.Error("Expected status property")
	} else {
		statusMap := status.(map[string]any)
		// Status is required, so no anyOf wrapping
		if enum, ok := statusMap["enum"]; ok {
			enumSlice, ok := enum.([]string)
			if !ok {
				t.Errorf("Expected enum to be []string, got %T", enum)
			} else if len(enumSlice) != 3 {
				t.Errorf("Expected 3 enum values for status, got %d", len(enumSlice))
			}
		} else {
			t.Logf("Status map: %+v", statusMap)
			t.Error("Expected status to have enum values")
		}
	}

	// Check nested age_range object
	ageRange, ok := props["age_range"]
	if !ok {
		t.Error("Expected age_range property")
	} else {
		// age_range is optional, so it may be wrapped in anyOf
		ageRangeMap, isMap := ageRange.(map[string]any)
		if !isMap {
			t.Fatalf("age_range should be a map, got %T", ageRange)
		}

		// Check if it's wrapped in anyOf (optional field)
		if anyOf, hasAnyOf := ageRangeMap["anyOf"]; hasAnyOf {
			anyOfSlice := anyOf.([]map[string]any)
			// Find the object schema (not null)
			for _, schema := range anyOfSlice {
				if schema["type"] == "object" {
					ageRangeMap = schema
					break
				}
			}
		}

		if ageRangeMap["type"] != "object" {
			t.Logf("age_range map: %+v", ageRangeMap)
			t.Errorf("Expected age_range type 'object', got %v", ageRangeMap["type"])
		}
		
		ageRangeProps, ok := ageRangeMap["properties"].(map[string]any)
		if !ok {
			t.Logf("age_range map: %+v", ageRangeMap)
			t.Error("Expected age_range to have properties")
		} else {
			if _, hasMin := ageRangeProps["min"]; !hasMin {
				t.Error("Expected min property in age_range")
			}
			if _, hasMax := ageRangeProps["max"]; !hasMax {
				t.Error("Expected max property in age_range")
			}
		}
	}

	// Check additionalProperties is false (strict mode)
	if schemaMap["additionalProperties"] != false {
		t.Errorf("Expected additionalProperties to be false, got %v", schemaMap["additionalProperties"])
	}
}

func TestStructToSchemaWithTool(t *testing.T) {
	type CreateUserParams struct {
		Email     string `json:"email" required:"true" desc:"User email address"`
		FirstName string `json:"first_name" required:"true"`
		Nickname  string `json:"nickname" desc:"Optional nickname"`
	}

	// Test that StructToSchema works with WithParameter
	schema, err := StructToSchema[CreateUserParams]()
	if err != nil {
		t.Fatalf("StructToSchema failed: %v", err)
	}

	tool := NewTool("create_user").
		WithDescription("Create a user").
		WithParameter("user", schema).
		Build()

	if tool.Name() != "create_user" {
		t.Errorf("Expected tool name 'create_user', got %s", tool.Name())
	}

	// Verify the schema was set correctly by accessing tool's private parameters field
	// We can check this by looking at the tool struct directly (in same package)
	props, ok := tool.parameters["properties"].(map[string]any)
	if !ok {
		t.Fatal("Expected properties in tool parameters")
	}

	// The "user" parameter should contain our struct schema
	userParam, ok := props["user"]
	if !ok {
		t.Fatal("Expected 'user' parameter")
	}

	userMap := userParam.(map[string]any)
	userProps, ok := userMap["properties"].(map[string]any)
	if !ok {
		t.Fatal("Expected properties in user parameter")
	}

	// Check email field exists
	if _, hasEmail := userProps["email"]; !hasEmail {
		t.Error("Expected email property in user parameter")
	}

	// Check first_name field exists
	if _, hasFirstName := userProps["first_name"]; !hasFirstName {
		t.Error("Expected first_name property in user parameter")
	}

	// Check nickname field exists (optional)
	if _, hasNickname := userProps["nickname"]; !hasNickname {
		t.Error("Expected nickname property in user parameter")
	}
}

func TestStructToSchemaOptionalFields(t *testing.T) {
	type UserUpdate struct {
		Email    string `json:"email" required:"true"`
		Nickname string `json:"nickname"` // No required tag = optional
		Age      int    `json:"age"`      // No required tag = optional
	}

	schema, err := StructToSchema[UserUpdate]()
	if err != nil {
		t.Fatalf("StructToSchema failed: %v", err)
	}

	schemaMap := schema.ToMap()
	props := schemaMap["properties"].(map[string]any)

	// Check nickname is wrapped in anyOf with null (optional)
	nickname := props["nickname"].(map[string]any)
	if anyOf, ok := nickname["anyOf"]; ok {
		anyOfSlice := anyOf.([]map[string]any)
		if len(anyOfSlice) != 2 {
			t.Errorf("Expected anyOf with 2 elements for optional field, got %d", len(anyOfSlice))
		}
		// One should be the type, one should be null
		hasNull := false
		for _, schema := range anyOfSlice {
			if schema["type"] == "null" {
				hasNull = true
			}
		}
		if !hasNull {
			t.Error("Expected anyOf to include null type for optional field")
		}
	} else {
		t.Error("Expected optional field to use anyOf with null")
	}
}

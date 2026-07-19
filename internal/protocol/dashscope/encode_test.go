package dashscope

import (
	"encoding/json"
	"testing"
)

func TestSessionUpdateEventEncodesFunctionToolSchema(t *testing.T) {
	additionalProperties := false
	minLength := 1
	maxLength := 64
	minimum := 0.0
	maximum := 10.0
	tools := []FunctionToolPayload{{
		Type: "function",
		Function: FunctionDefinitionPayload{
			Name:        "lookup_weather",
			Description: "Look up weather",
			Parameters: &JSONSchemaPayload{
				Type: "object",
				Properties: map[string]*JSONSchemaPayload{
					"city": {Type: "string", MinLength: &minLength, MaxLength: &maxLength},
					"days": {Type: "number", Minimum: &minimum, Maximum: &maximum},
					"tags": {Type: "array", Items: &JSONSchemaPayload{Type: "string"}},
					"units": {AnyOf: []*JSONSchemaPayload{
						{Type: "string", Enum: []any{"celsius", "fahrenheit"}},
						{Type: "null"},
					}},
				},
				Required:             []string{"city"},
				AdditionalProperties: &additionalProperties,
			},
		},
	}}
	event := SessionUpdateEvent("event_1", SessionUpdatePayload{Tools: &tools})

	encoded, err := Marshal(event)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	session := got["session"].(map[string]any)
	tool := session["tools"].([]any)[0].(map[string]any)
	function := tool["function"].(map[string]any)
	parameters := function["parameters"].(map[string]any)
	if function["name"] != "lookup_weather" || parameters["type"] != "object" {
		t.Fatalf("unexpected function payload: %#v", function)
	}
	if parameters["additionalProperties"] != false {
		t.Fatalf("additionalProperties = %#v, want false", parameters["additionalProperties"])
	}
	properties := parameters["properties"].(map[string]any)
	if properties["city"].(map[string]any)["minLength"] != float64(1) {
		t.Fatalf("city schema = %#v", properties["city"])
	}
	if properties["city"].(map[string]any)["maxLength"] != float64(64) {
		t.Fatalf("city schema = %#v", properties["city"])
	}
	if properties["days"].(map[string]any)["minimum"] != float64(0) {
		t.Fatalf("days schema = %#v", properties["days"])
	}
	if properties["days"].(map[string]any)["maximum"] != float64(10) {
		t.Fatalf("days schema = %#v", properties["days"])
	}
	if properties["tags"].(map[string]any)["items"].(map[string]any)["type"] != "string" {
		t.Fatalf("tags schema = %#v", properties["tags"])
	}
	if len(properties["units"].(map[string]any)["anyOf"].([]any)) != 2 {
		t.Fatalf("units schema = %#v", properties["units"])
	}
}

func TestConversationItemCreateFunctionOutputEvent(t *testing.T) {
	event := ConversationItemCreateFunctionOutputEvent("event_2", FunctionCallOutputPayload{
		Type:   "function_call_output",
		CallID: "call_1",
		Output: "  {\"ok\":true}\n",
	})
	encoded, err := Marshal(event)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	want := `{"event_id":"event_2","item":{"type":"function_call_output","call_id":"call_1","output":"  {\"ok\":true}\n"},"type":"conversation.item.create"}`
	if string(encoded) != want {
		t.Fatalf("Marshal() = %s, want %s", encoded, want)
	}
}

package schema

import (
	"encoding/json"
	"testing"

	"github.com/openany/openany-mcp/internal/model"
)

func TestNonEmptyPolicyDoesNotRepairInvalidLengthConstraints(t *testing.T) {
	for _, minimum := range []string{"-1", "-1e-400", "1e-400", "0.5", `"0"`} {
		r := model.Record{UnknownParameters: "reject", Inputs: []model.Input{{Name: "tags", Type: "array", NonEmpty: true, Schema: json.RawMessage(`{"type":"array","minItems":` + minimum + `}`)}}}
		if _, err := Compile(r); err == nil {
			t.Errorf("invalid minItems %s was silently repaired", minimum)
		}
	}
}

func TestNonEmptyPolicyRecognizesExactZero(t *testing.T) {
	for _, minimum := range []string{"0", "-0", "0.0", "0e4", "-0.0E-4"} {
		r := model.Record{UnknownParameters: "reject", Inputs: []model.Input{{Name: "tags", Type: "array", NonEmpty: true, Schema: json.RawMessage(`{"type":"array","minItems":` + minimum + `}`)}}}
		c, err := Compile(r)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.Validate([]byte(`{"tags":[]}`)); err == nil {
			t.Errorf("zero minItems %s advertised an unsupported empty array", minimum)
		}
		if _, err := c.Validate([]byte(`{"tags":["a"]}`)); err != nil {
			t.Fatal(err)
		}
	}
}

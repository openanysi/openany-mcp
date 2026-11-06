package schema

import (
	"encoding/json"
	"github.com/openany/openany-mcp/internal/model"
	"strings"
	"testing"
)

func TestValidationDetailsAreFieldLevelBoundedAndNeverEchoValues(t *testing.T) {
	c, err := Compile(model.Record{UnknownParameters: "reject", Inputs: []model.Input{{Name: "body", Required: true, Schema: json.RawMessage(`{"type":"object","required":["company"],"properties":{"count":{"type":"integer"},"region":{"enum":["CN","US"]}}}`)}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Validate([]byte(`{"body":{"count":"secret-count-value","region":"secret-region-value"}}`))
	details := ValidationDetails(err)
	raw, _ := json.Marshal(details)
	if len(details) != 3 || !strings.Contains(string(raw), "/body/company") || !strings.Contains(string(raw), "/body/count") || !strings.Contains(string(raw), "enum") || strings.Contains(string(raw), "secret-") {
		t.Fatal(string(raw))
	}
	_, err = c.Validate([]byte(`{"body":null,"body":"secret-duplicate"}`))
	raw, _ = json.Marshal(ValidationDetails(err))
	if strings.Contains(string(raw), "secret-") {
		t.Fatal(string(raw))
	}
}

package model

import "encoding/json"

type Input struct {
	Name        string          `json:"name"`
	Location    string          `json:"location"`
	Type        string          `json:"type"`
	Pointer     *string         `json:"pointer,omitempty"`
	Required    bool            `json:"required"`
	Nullable    bool            `json:"nullable"`
	Description string          `json:"description"`
	ItemType    string          `json:"item_type,omitempty"`
	Schema      json.RawMessage `json:"json_schema,omitempty"`
	Enum        json.RawMessage `json:"enum_values,omitempty"`
	Minimum     json.RawMessage `json:"minimum,omitempty"`
	Maximum     json.RawMessage `json:"maximum,omitempty"`
}
type Record struct {
	BillingMode        string  `json:"billing_mode"`
	APIID              string  `json:"api_id"`
	Name               string  `json:"name"`
	Description        string  `json:"description"`
	ProviderID         string  `json:"provider_id"`
	ToolName           *string `json:"public_mcp_tool_name"`
	Method             string  `json:"method"`
	PathTemplate       string  `json:"path_template"`
	PublicPath         string  `json:"public_path"`
	Status             string  `json:"status"`
	Revision           int     `json:"revision"`
	UpstreamProtocol   string  `json:"upstream_protocol"`
	RequestContentType string  `json:"request_content_type"`
	UnknownParameters  string  `json:"unknown_parameters"`
	ConfigPresent      bool    `json:"config_present"`
	Inputs             []Input `json:"inputs"`
	InputError         string  `json:"input_error,omitempty"`
}

// Request contains only the data needed to encode and dispatch a gateway call.
type Request struct {
	Method              string
	PublicPath          string
	RequestContentType  string
	Inputs              []Parameter
	QueryArrays         map[string]ArrayPolicy
	UnsupportedResponse bool
}

type Parameter struct {
	Name     string
	Location string
	Pointer  *string
	OmitNull bool
}

type ArrayPolicy struct {
	Encoding string
	Empty    string
}

type ArrayEncoding struct {
	Encoding string `json:"encoding"`
	Empty    string `json:"empty"`
	Source   string `json:"source"`
}
type Override struct {
	Description       string                   `json:"public_description,omitempty"`
	QueryArrays       map[string]ArrayEncoding `json:"query_arrays,omitempty"`
	ResponseMode      string                   `json:"response_mode,omitempty"`
	UnsupportedReason string                   `json:"unsupported_reason,omitempty"`
}
type Overrides struct {
	Version int                 `json:"version"`
	APIs    map[string]Override `json:"apis"`
}

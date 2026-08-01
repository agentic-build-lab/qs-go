package oracle

import (
	"encoding/json"
	"errors"
	"fmt"
)

const (
	// ProtocolSchema is shared by every request and response in stage one.
	ProtocolSchema = "qs_go_oracle/v1"
	// MaxRequestBytes bounds one NDJSON request, including its trailing newline.
	MaxRequestBytes = 1024 * 1024
	// MaxResponseBytes bounds one NDJSON response, including its trailing newline.
	MaxResponseBytes = 2 * 1024 * 1024
)

var (
	ErrRequestTooLarge  = errors.New("qsgo oracle: request exceeds byte limit")
	ErrResponseTooLarge = errors.New("qsgo oracle: response exceeds byte limit")
	ErrClientClosed     = errors.New("qsgo oracle: client is closed")
)

// TestDigest is one frozen upstream test-file digest.
type TestDigest struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// UpstreamIdentity identifies the exact source snapshot used by the oracle.
type UpstreamIdentity struct {
	Repository   string `json:"repository"`
	Commit       string `json:"commit"`
	Describe     string `json:"describe"`
	TestTreeSHA1 string `json:"test_tree_sha1"`
}

// Baseline records the frozen JavaScript test result.
type Baseline struct {
	AssertionsPassed int `json:"assertions_passed"`
	AssertionsFailed int `json:"assertions_failed"`
	ExplicitSkips    int `json:"explicit_skips"`
}

// Limits are the byte limits enforced by the Node oracle.
type Limits struct {
	MaxRequestBytes  int `json:"max_request_bytes"`
	MaxResponseBytes int `json:"max_response_bytes"`
}

// Subset describes the intentionally limited stage-one wire representation.
type Subset struct {
	Parse           string `json:"parse"`
	Stringify       string `json:"stringify"`
	Callbacks       bool   `json:"callbacks"`
	RegexpDelimiter bool   `json:"regexp_delimiter"`
	TaggedValues    bool   `json:"tagged_values"`
}

// Handshake is returned only after the Node process verifies the manifest,
// Git commit, Git test tree, and every frozen SHA-256 digest.
type Handshake struct {
	Protocol string            `json:"protocol"`
	Upstream UpstreamIdentity  `json:"upstream"`
	Tests    []TestDigest      `json:"tests"`
	Baseline Baseline          `json:"baseline"`
	Runtime  map[string]string `json:"runtime"`
	Limits   Limits            `json:"limits"`
	Subset   Subset            `json:"subset"`
}

// RemoteError is a structured error returned by the JavaScript oracle.
type RemoteError struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Message string `json:"message"`
}

func (err *RemoteError) Error() string {
	if err == nil {
		return "qsgo oracle: unknown remote error"
	}
	return fmt.Sprintf("qsgo oracle: %s (%s): %s", err.Code, err.Name, err.Message)
}

type wireRequest struct {
	Schema  string          `json:"schema"`
	ID      string          `json:"id"`
	Op      string          `json:"op"`
	Input   json.RawMessage `json:"input,omitempty"`
	Options json.RawMessage `json:"options,omitempty"`
}

type wireResponse struct {
	Schema string          `json:"schema"`
	ID     string          `json:"id"`
	OK     bool            `json:"ok"`
	Value  json.RawMessage `json:"value"`
	Error  *RemoteError    `json:"error"`
}

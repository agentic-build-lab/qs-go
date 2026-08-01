package oracle

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const manifestSchema = "qs_go_oracle_manifest/v1"

type manifest struct {
	Schema   string           `json:"schema"`
	Upstream UpstreamIdentity `json:"upstream"`
	Tests    []TestDigest     `json:"tests"`
	Baseline Baseline         `json:"baseline"`
}

func loadManifest(filePath string) (manifest, error) {
	contents, err := os.ReadFile(filePath)
	if err != nil {
		return manifest{}, fmt.Errorf("read oracle manifest: %w", err)
	}
	var result manifest
	if err := json.Unmarshal(contents, &result); err != nil {
		return manifest{}, fmt.Errorf("decode oracle manifest: %w", err)
	}
	if result.Schema != manifestSchema {
		return manifest{}, fmt.Errorf("unsupported oracle manifest schema %q", result.Schema)
	}
	if result.Upstream.Commit == "" || result.Upstream.TestTreeSHA1 == "" || len(result.Tests) != 4 {
		return manifest{}, fmt.Errorf("oracle manifest is incomplete")
	}
	return result, nil
}

func validateHandshake(expected manifest, actual Handshake) error {
	if actual.Protocol != ProtocolSchema {
		return fmt.Errorf("oracle protocol mismatch: got %q", actual.Protocol)
	}
	if actual.Upstream.Repository != expected.Upstream.Repository {
		return fmt.Errorf("oracle repository mismatch")
	}
	if actual.Upstream.Commit != expected.Upstream.Commit {
		return fmt.Errorf("oracle commit mismatch")
	}
	if actual.Upstream.Describe != expected.Upstream.Describe {
		return fmt.Errorf("oracle describe mismatch")
	}
	if actual.Upstream.TestTreeSHA1 != expected.Upstream.TestTreeSHA1 {
		return fmt.Errorf("oracle test tree mismatch")
	}
	if actual.Baseline != expected.Baseline {
		return fmt.Errorf("oracle baseline mismatch")
	}
	if len(actual.Tests) != len(expected.Tests) {
		return fmt.Errorf("oracle returned %d test digests; expected %d", len(actual.Tests), len(expected.Tests))
	}

	actualDigests := make(map[string]string, len(actual.Tests))
	for _, digest := range actual.Tests {
		if _, exists := actualDigests[digest.Path]; exists {
			return fmt.Errorf("oracle returned duplicate digest path %q", digest.Path)
		}
		actualDigests[digest.Path] = strings.ToLower(digest.SHA256)
	}
	for _, digest := range expected.Tests {
		if actualDigests[digest.Path] != strings.ToLower(digest.SHA256) {
			return fmt.Errorf("oracle digest mismatch for %q", digest.Path)
		}
	}
	if actual.Limits.MaxRequestBytes != MaxRequestBytes || actual.Limits.MaxResponseBytes != MaxResponseBytes {
		return fmt.Errorf("oracle byte limits do not match the Go protocol")
	}
	return nil
}

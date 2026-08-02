package differential

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/agentic-build-lab/qs-go/internal/oracle"
)

const (
	oracleTestEnvironment = "QSGO_RUN_ORACLE_TESTS"
	expectedCommit        = "3a890d4ecd3deb72a45d90be36f4f8c5970467c7"
	expectedTestTree      = "bef346f180a38793ec6d47e11f25f88a7eb579ca"
)

var expectedTestDigests = map[string]string{
	"test/empty-keys-cases.js": "9190f374bd3129f552289579ed9812944899b690b6feac065b55c2877a1e71ab",
	"test/parse.js":            "7a658caf90053d617cf920d20d21e7449da52dce807a1e55ccb4fb0152d9c725",
	"test/stringify.js":        "a1c6f1e35af7a35e027a3e391a9784a011c58086f93de400f32767bc79f529c8",
	"test/utils.js":            "980f98ae1d2dacfdfc49191c58785ad49e9b1539807c60908bd7c0cd6e8f351b",
}

func TestFrozenOracleHandshakeAndBasicCases(t *testing.T) {
	moduleRoot := findModuleRoot(t)
	if os.Getenv(oracleTestEnvironment) != "1" {
		t.Skipf("set %s=1 to run the frozen Node oracle integration test", oracleTestEnvironment)
	}
	upstreamRoot := filepath.Join(moduleRoot, "..", "upstream_qs")
	if _, err := os.Stat(filepath.Join(upstreamRoot, ".git")); err != nil {
		t.Fatalf("%s=1 requires the frozen upstream checkout at %s: %v", oracleTestEnvironment, upstreamRoot, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := oracle.Start(ctx, oracle.Config{
		ScriptPath:       filepath.Join(moduleRoot, "internal", "oracle", "node_oracle.cjs"),
		ManifestPath:     filepath.Join(moduleRoot, "testdata", "oracle", "oracle_manifest.json"),
		WorkingDirectory: moduleRoot,
		StartupTimeout:   10 * time.Second,
		RequestTimeout:   3 * time.Second,
	})
	if err != nil {
		t.Fatalf("start frozen oracle: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := client.Close(); closeErr != nil {
			t.Errorf("close frozen oracle: %v", closeErr)
		}
	})

	handshake := client.Handshake()
	if handshake.Upstream.Commit != expectedCommit {
		t.Fatalf("commit = %q, want %q", handshake.Upstream.Commit, expectedCommit)
	}
	if handshake.Upstream.TestTreeSHA1 != expectedTestTree {
		t.Fatalf("test tree = %q, want %q", handshake.Upstream.TestTreeSHA1, expectedTestTree)
	}
	if len(handshake.Tests) != len(expectedTestDigests) {
		t.Fatalf("digest count = %d, want %d", len(handshake.Tests), len(expectedTestDigests))
	}
	for _, digest := range handshake.Tests {
		if expectedTestDigests[digest.Path] != digest.SHA256 {
			t.Errorf("digest for %q = %q", digest.Path, digest.SHA256)
		}
	}
	if handshake.Baseline.AssertionsPassed != 1045 || handshake.Baseline.AssertionsFailed != 0 {
		t.Fatalf("unexpected upstream baseline: %+v", handshake.Baseline)
	}
	if handshake.Limits.MaxRequestBytes != oracle.MaxRequestBytes || handshake.Limits.MaxResponseBytes != oracle.MaxResponseBytes {
		t.Fatalf("unexpected oracle limits: %+v", handshake.Limits)
	}

	parsed, err := client.Parse(ctx, "a[b]=c&a[d][]=e&a[d][]=f", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("basic parse: %v", err)
	}
	assertJSONEqual(t, parsed, json.RawMessage(`{"a":{"b":"c","d":["e","f"]}}`))

	parsed, err = client.Parse(ctx, "a.b=first&a.b=last", json.RawMessage(`{"allowDots":true,"duplicates":"last"}`))
	if err != nil {
		t.Fatalf("parse with options: %v", err)
	}
	assertJSONEqual(t, parsed, json.RawMessage(`{"a":{"b":"last"}}`))

	stringified, err := client.Stringify(
		ctx,
		json.RawMessage(`{"a":{"b":"c"},"list":["x","y"]}`),
		json.RawMessage(`{"encode":false,"arrayFormat":"brackets"}`),
	)
	if err != nil {
		t.Fatalf("basic stringify: %v", err)
	}
	if stringified != "a[b]=c&list[]=x&list[]=y" {
		t.Fatalf("stringify = %q", stringified)
	}

	_, err = client.Parse(ctx, "a=b", json.RawMessage(`{"decoder":"not_allowed"}`))
	var remoteError *oracle.RemoteError
	if !errors.As(err, &remoteError) || remoteError.Code != "unsupported_option" {
		t.Fatalf("callback option error = %v, want unsupported_option", err)
	}

	_, err = client.Parse(ctx, strings.Repeat("x", oracle.MaxRequestBytes), nil)
	if !errors.Is(err, oracle.ErrRequestTooLarge) {
		t.Fatalf("oversized request error = %v, want ErrRequestTooLarge", err)
	}
}

func findModuleRoot(t *testing.T) string {
	t.Helper()
	_, fileName, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(fileName), "..", ".."))
}

func assertJSONEqual(t *testing.T, actual json.RawMessage, expected json.RawMessage) {
	t.Helper()
	var actualCompact bytes.Buffer
	if err := json.Compact(&actualCompact, actual); err != nil {
		t.Fatalf("compact actual JSON: %v", err)
	}
	var expectedCompact bytes.Buffer
	if err := json.Compact(&expectedCompact, expected); err != nil {
		t.Fatalf("compact expected JSON: %v", err)
	}
	if !bytes.Equal(actualCompact.Bytes(), expectedCompact.Bytes()) {
		t.Fatalf("JSON mismatch\nactual:   %s\nexpected: %s", actualCompact.Bytes(), expectedCompact.Bytes())
	}
}

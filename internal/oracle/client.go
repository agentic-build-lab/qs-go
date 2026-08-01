package oracle

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

const (
	defaultStartupTimeout = 5 * time.Second
	defaultRequestTimeout = 2 * time.Second
	closeGracePeriod      = 500 * time.Millisecond
	maxStderrBytes        = 64 * 1024
)

// Config contains explicit local paths. The oracle never resolves or fetches
// network resources.
type Config struct {
	NodeBinary       string
	ScriptPath       string
	ManifestPath     string
	WorkingDirectory string
	StartupTimeout   time.Duration
	RequestTimeout   time.Duration
	MaxRequestBytes  int
	MaxResponseBytes int
}

// Client serializes calls over one long-lived Node NDJSON process.
type Client struct {
	mu               sync.Mutex
	command          *exec.Cmd
	stdin            io.WriteCloser
	stdout           *bufio.Reader
	wait             <-chan error
	stderr           *boundedBuffer
	closed           bool
	nextID           uint64
	requestTimeout   time.Duration
	maxRequestBytes  int
	maxResponseBytes int
	handshake        Handshake
}

type transactionResult struct {
	line []byte
	err  error
}

// Start launches Node, performs the mandatory handshake, and verifies the
// returned identity against the local frozen manifest.
func Start(ctx context.Context, config Config) (*Client, error) {
	if config.NodeBinary == "" {
		config.NodeBinary = "node"
	}
	if config.ScriptPath == "" || config.ManifestPath == "" {
		return nil, fmt.Errorf("qsgo oracle: script and manifest paths are required")
	}
	if config.StartupTimeout <= 0 {
		config.StartupTimeout = defaultStartupTimeout
	}
	if config.RequestTimeout <= 0 {
		config.RequestTimeout = defaultRequestTimeout
	}
	if config.MaxRequestBytes <= 0 {
		config.MaxRequestBytes = MaxRequestBytes
	}
	if config.MaxResponseBytes <= 0 {
		config.MaxResponseBytes = MaxResponseBytes
	}
	if config.MaxRequestBytes > MaxRequestBytes || config.MaxResponseBytes > MaxResponseBytes {
		return nil, fmt.Errorf("qsgo oracle: configured byte limits exceed the protocol maximum")
	}

	expected, err := loadManifest(config.ManifestPath)
	if err != nil {
		return nil, err
	}

	command := exec.Command(config.NodeBinary, config.ScriptPath)
	command.Dir = config.WorkingDirectory
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("create oracle stdin: %w", err)
	}
	stdoutPipe, err := command.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create oracle stdout: %w", err)
	}
	stderrPipe, err := command.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("create oracle stderr: %w", err)
	}
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start Node oracle: %w", err)
	}

	waitChannel := make(chan error, 1)
	go func() {
		waitChannel <- command.Wait()
	}()
	stderr := newBoundedBuffer(maxStderrBytes)
	go func() {
		_, _ = io.Copy(stderr, stderrPipe)
	}()

	client := &Client{
		command:          command,
		stdin:            stdin,
		stdout:           bufio.NewReaderSize(stdoutPipe, 64*1024),
		wait:             waitChannel,
		stderr:           stderr,
		requestTimeout:   config.RequestTimeout,
		maxRequestBytes:  config.MaxRequestBytes,
		maxResponseBytes: config.MaxResponseBytes,
	}

	startupContext, cancel := withDefaultTimeout(ctx, config.StartupTimeout)
	defer cancel()
	handshakeValue, err := client.call(startupContext, "handshake", nil, nil)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("oracle handshake: %w", err)
	}
	if err := json.Unmarshal(handshakeValue, &client.handshake); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("decode oracle handshake: %w", err)
	}
	if err := validateHandshake(expected, client.handshake); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("validate oracle handshake: %w", err)
	}
	return client, nil
}

// Handshake returns the immutable verified identity reported at startup.
func (client *Client) Handshake() Handshake {
	client.mu.Lock()
	defer client.mu.Unlock()
	result := client.handshake
	result.Tests = append([]TestDigest(nil), client.handshake.Tests...)
	if client.handshake.Runtime != nil {
		result.Runtime = make(map[string]string, len(client.handshake.Runtime))
		for key, value := range client.handshake.Runtime {
			result.Runtime[key] = value
		}
	}
	return result
}

// Parse evaluates qs.parse for the JSON-compatible dense stage-one subset.
func (client *Client) Parse(ctx context.Context, input string, options json.RawMessage) (json.RawMessage, error) {
	encodedInput, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("encode parse input: %w", err)
	}
	return client.call(ctx, "parse", encodedInput, normalizeOptions(options))
}

// Stringify evaluates qs.stringify for a JSON-compatible input tree.
func (client *Client) Stringify(ctx context.Context, input json.RawMessage, options json.RawMessage) (string, error) {
	if len(input) == 0 || !json.Valid(input) {
		return "", fmt.Errorf("qsgo oracle: stringify input is not valid JSON")
	}
	value, err := client.call(ctx, "stringify", input, normalizeOptions(options))
	if err != nil {
		return "", err
	}
	var result string
	if err := json.Unmarshal(value, &result); err != nil {
		return "", fmt.Errorf("decode stringify result: %w", err)
	}
	return result, nil
}

func normalizeOptions(options json.RawMessage) json.RawMessage {
	if len(options) == 0 {
		return json.RawMessage(`{}`)
	}
	return options
}

func (client *Client) call(ctx context.Context, operation string, input json.RawMessage, options json.RawMessage) (json.RawMessage, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closed {
		return nil, ErrClientClosed
	}
	if len(input) > 0 && !json.Valid(input) {
		return nil, fmt.Errorf("qsgo oracle: input is not valid JSON")
	}
	if len(options) > 0 && !json.Valid(options) {
		return nil, fmt.Errorf("qsgo oracle: options are not valid JSON")
	}

	client.nextID++
	requestID := "go-" + strconv.FormatUint(client.nextID, 10)
	request := wireRequest{
		Schema:  ProtocolSchema,
		ID:      requestID,
		Op:      operation,
		Input:   input,
		Options: options,
	}
	encodedRequest, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("encode oracle request: %w", err)
	}
	if len(encodedRequest)+1 > client.maxRequestBytes {
		return nil, ErrRequestTooLarge
	}
	encodedRequest = append(encodedRequest, '\n')

	requestContext, cancel := withDefaultTimeout(ctx, client.requestTimeout)
	defer cancel()
	resultChannel := make(chan transactionResult, 1)
	go func() {
		if _, writeErr := client.stdin.Write(encodedRequest); writeErr != nil {
			resultChannel <- transactionResult{err: writeErr}
			return
		}
		line, readErr := readLineLimited(client.stdout, client.maxResponseBytes)
		resultChannel <- transactionResult{line: line, err: readErr}
	}()

	var transaction transactionResult
	select {
	case transaction = <-resultChannel:
	case <-requestContext.Done():
		client.breakProcessLocked()
		return nil, fmt.Errorf("qsgo oracle: request timed out: %w", requestContext.Err())
	}
	if transaction.err != nil {
		client.breakProcessLocked()
		return nil, fmt.Errorf("qsgo oracle transport: %w; stderr: %s", transaction.err, client.stderr.String())
	}

	var response wireResponse
	if err := json.Unmarshal(transaction.line, &response); err != nil {
		client.breakProcessLocked()
		return nil, fmt.Errorf("decode oracle response: %w", err)
	}
	if response.Schema != ProtocolSchema {
		client.breakProcessLocked()
		return nil, fmt.Errorf("qsgo oracle: response schema mismatch")
	}
	if !response.OK {
		if response.Error == nil {
			return nil, fmt.Errorf("qsgo oracle: remote failure without structured error")
		}
		return nil, response.Error
	}
	if response.ID != requestID {
		client.breakProcessLocked()
		return nil, fmt.Errorf("qsgo oracle: response id mismatch")
	}
	return append(json.RawMessage(nil), response.Value...), nil
}

// Close terminates the isolated Node process. It is safe to call repeatedly.
func (client *Client) Close() error {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closed {
		return nil
	}
	client.closed = true
	_ = client.stdin.Close()
	select {
	case err := <-client.wait:
		if err != nil && !isExpectedExit(err) {
			return fmt.Errorf("wait for oracle: %w; stderr: %s", err, client.stderr.String())
		}
		return nil
	case <-time.After(closeGracePeriod):
		if client.command.Process != nil {
			_ = client.command.Process.Kill()
		}
		select {
		case <-client.wait:
		case <-time.After(closeGracePeriod):
		}
		return nil
	}
}

func (client *Client) breakProcessLocked() {
	if client.closed {
		return
	}
	client.closed = true
	_ = client.stdin.Close()
	if client.command.Process != nil {
		_ = client.command.Process.Kill()
	}
}

func readLineLimited(reader *bufio.Reader, maximum int) ([]byte, error) {
	var result []byte
	for {
		fragment, err := reader.ReadSlice('\n')
		if len(result)+len(fragment) > maximum {
			return nil, ErrResponseTooLarge
		}
		result = append(result, fragment...)
		if err == nil {
			result = bytes.TrimSuffix(result, []byte{'\n'})
			result = bytes.TrimSuffix(result, []byte{'\r'})
			return result, nil
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if errors.Is(err, io.EOF) && len(result) > 0 {
			return result, nil
		}
		return nil, err
	}
}

func withDefaultTimeout(parent context.Context, duration time.Duration) (context.Context, context.CancelFunc) {
	if _, hasDeadline := parent.Deadline(); hasDeadline || duration <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, duration)
}

func isExpectedExit(err error) bool {
	var exitError *exec.ExitError
	return errors.As(err, &exitError)
}

type boundedBuffer struct {
	mu        sync.Mutex
	remaining int
	contents  bytes.Buffer
}

func newBoundedBuffer(maximum int) *boundedBuffer {
	return &boundedBuffer{remaining: maximum}
}

func (buffer *boundedBuffer) Write(value []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	originalLength := len(value)
	if buffer.remaining > 0 {
		writeLength := len(value)
		if writeLength > buffer.remaining {
			writeLength = buffer.remaining
		}
		_, _ = buffer.contents.Write(value[:writeLength])
		buffer.remaining -= writeLength
	}
	return originalLength, nil
}

func (buffer *boundedBuffer) String() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.contents.String()
}

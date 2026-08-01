'use strict';

const childProcess = require('node:child_process');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');

const PROTOCOL_SCHEMA = 'qs_go_oracle/v1';
const MAX_REQUEST_BYTES = 1024 * 1024;
const MAX_RESPONSE_BYTES = 2 * 1024 * 1024;
const MAX_ID_LENGTH = 128;

const moduleRoot = path.resolve(__dirname, '..', '..');
const manifestPath = path.join(moduleRoot, 'testdata', 'oracle', 'oracle_manifest.json');
const upstreamRoot = path.resolve(moduleRoot, '..', 'upstream_qs');

class OracleError extends Error {
    constructor(code, message) {
        super(message);
        this.name = 'OracleError';
        this.code = code;
    }
}

class UnsupportedValueError extends OracleError {
    constructor(message) {
        super('unsupported_result', message);
        this.name = 'UnsupportedValueError';
    }
}

const isRecord = function isRecord(value) {
    return value !== null && typeof value === 'object' && !Array.isArray(value);
};

const sha256File = function sha256File(filePath) {
    return crypto.createHash('sha256').update(fs.readFileSync(filePath)).digest('hex');
};

const runGit = function runGit(args) {
    const safeDirectory = upstreamRoot.replace(/\\/g, '/');
    return childProcess.execFileSync(
        'git',
        ['-c', 'safe.directory=' + safeDirectory, '-C', upstreamRoot].concat(args),
        {
            encoding: 'utf8',
            windowsHide: true,
            stdio: ['ignore', 'pipe', 'pipe']
        }
    ).trim();
};

const resolveUpstreamFile = function resolveUpstreamFile(relativePath) {
    if (typeof relativePath !== 'string' || relativePath.length === 0 || path.isAbsolute(relativePath)) {
        throw new OracleError('invalid_manifest', 'manifest test path must be a nonempty relative path');
    }

    const resolved = path.resolve(upstreamRoot, relativePath);
    const relative = path.relative(upstreamRoot, resolved);
    if (relative.startsWith('..' + path.sep) || relative === '..' || path.isAbsolute(relative)) {
        throw new OracleError('invalid_manifest', 'manifest test path escapes the upstream directory');
    }
    return resolved;
};

const loadAndVerifyManifest = function loadAndVerifyManifest() {
    const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
    if (!isRecord(manifest) || manifest.schema !== 'qs_go_oracle_manifest/v1') {
        throw new OracleError('invalid_manifest', 'unsupported oracle manifest schema');
    }
    if (!isRecord(manifest.upstream) || !Array.isArray(manifest.tests) || manifest.tests.length !== 4) {
        throw new OracleError('invalid_manifest', 'oracle manifest must contain upstream metadata and four test digests');
    }

    const commit = runGit(['rev-parse', 'HEAD']);
    const describe = runGit(['describe', '--tags', '--always', '--dirty']);
    const testTree = runGit(['rev-parse', 'HEAD:test']);

    if (commit !== manifest.upstream.commit) {
        throw new OracleError('manifest_mismatch', 'upstream commit does not match the frozen manifest');
    }
    if (describe !== manifest.upstream.describe) {
        throw new OracleError('manifest_mismatch', 'upstream describe value does not match the frozen manifest');
    }
    if (testTree !== manifest.upstream.test_tree_sha1) {
        throw new OracleError('manifest_mismatch', 'upstream test tree does not match the frozen manifest');
    }

    const seenPaths = Object.create(null);
    const tests = manifest.tests.map(function verifyTest(testEntry) {
        if (!isRecord(testEntry) || typeof testEntry.path !== 'string' || typeof testEntry.sha256 !== 'string') {
            throw new OracleError('invalid_manifest', 'each manifest test entry must contain path and sha256');
        }
        if (seenPaths[testEntry.path]) {
            throw new OracleError('invalid_manifest', 'manifest contains a duplicate test path');
        }
        seenPaths[testEntry.path] = true;

        const actualDigest = sha256File(resolveUpstreamFile(testEntry.path));
        if (actualDigest !== testEntry.sha256.toLowerCase()) {
            throw new OracleError('manifest_mismatch', 'test digest mismatch for ' + testEntry.path);
        }
        return { path: testEntry.path, sha256: actualDigest };
    });

    return {
        protocol: PROTOCOL_SCHEMA,
        upstream: {
            repository: manifest.upstream.repository,
            commit: commit,
            describe: describe,
            test_tree_sha1: testTree
        },
        tests: tests,
        baseline: manifest.baseline,
        runtime: { node: process.version },
        limits: {
            max_request_bytes: MAX_REQUEST_BYTES,
            max_response_bytes: MAX_RESPONSE_BYTES
        },
        subset: {
            parse: 'json_compatible_dense_values',
            stringify: 'json_compatible_inputs',
            callbacks: false,
            regexp_delimiter: false,
            tagged_values: false
        }
    };
};

const booleanOption = function booleanOption(value, key) {
    if (typeof value !== 'boolean') {
        throw new OracleError('invalid_option', key + ' must be a boolean');
    }
};

const integerOption = function integerOption(value, key) {
    if (!Number.isSafeInteger(value)) {
        throw new OracleError('invalid_option', key + ' must be a safe integer in the stage-one oracle');
    }
};

const stringOption = function stringOption(value, key) {
    if (typeof value !== 'string') {
        throw new OracleError('invalid_option', key + ' must be a string');
    }
};

const enumOption = function enumOption(values) {
    return function validateEnum(value, key) {
        if (typeof value !== 'string' || values.indexOf(value) < 0) {
            throw new OracleError('invalid_option', key + ' has an unsupported value');
        }
    };
};

const depthOption = function depthOption(value, key) {
    if (value !== false && !Number.isSafeInteger(value)) {
        throw new OracleError('invalid_option', key + ' must be false or a safe integer');
    }
};

const parseOptionValidators = Object.freeze({
    allowDots: booleanOption,
    allowEmptyArrays: booleanOption,
    allowPrototypes: booleanOption,
    allowSparse: booleanOption,
    arrayLimit: integerOption,
    charset: enumOption(['utf-8', 'iso-8859-1']),
    charsetSentinel: booleanOption,
    comma: booleanOption,
    decodeDotInKeys: booleanOption,
    delimiter: stringOption,
    depth: depthOption,
    duplicates: enumOption(['combine', 'first', 'last']),
    ignoreQueryPrefix: booleanOption,
    interpretNumericEntities: booleanOption,
    parameterLimit: integerOption,
    parseArrays: booleanOption,
    plainObjects: booleanOption,
    strictDepth: booleanOption,
    strictMerge: booleanOption,
    strictNullHandling: booleanOption,
    throwOnLimitExceeded: booleanOption
});

const stringifyOptionValidators = Object.freeze({
    addQueryPrefix: booleanOption,
    allowDots: booleanOption,
    allowEmptyArrays: booleanOption,
    arrayFormat: enumOption(['indices', 'brackets', 'repeat', 'comma']),
    charset: enumOption(['utf-8', 'iso-8859-1']),
    charsetSentinel: booleanOption,
    commaRoundTrip: booleanOption,
    delimiter: stringOption,
    depth: integerOption,
    encode: booleanOption,
    encodeDotInKeys: booleanOption,
    encodeValuesOnly: booleanOption,
    format: enumOption(['RFC3986', 'RFC1738']),
    indices: booleanOption,
    skipNulls: booleanOption,
    strictNullHandling: booleanOption
});

const validateOptions = function validateOptions(options, validators) {
    if (typeof options === 'undefined') {
        return {};
    }
    if (!isRecord(options) || Object.getPrototypeOf(options) !== Object.prototype) {
        throw new OracleError('invalid_options', 'options must be a JSON object');
    }

    Object.keys(options).forEach(function validateOption(key) {
        if (!Object.prototype.hasOwnProperty.call(validators, key)) {
            throw new OracleError('unsupported_option', key + ' is not supported by the stage-one oracle');
        }
        validators[key](options[key], key);
    });
    return options;
};

const cloneJSONCompatible = function cloneJSONCompatible(value, active) {
    if (value === null || typeof value === 'string' || typeof value === 'boolean') {
        return value;
    }
    if (typeof value === 'number') {
        if (!Number.isFinite(value)) {
            throw new UnsupportedValueError('non-finite numbers require the tagged-value protocol');
        }
        return value;
    }
    if (typeof value !== 'object') {
        throw new UnsupportedValueError('result contains a JavaScript-only value');
    }
    if (active.has(value)) {
        throw new UnsupportedValueError('cyclic results require the tagged-value protocol');
    }

    active.add(value);
    try {
        if (Array.isArray(value)) {
            const keys = Object.keys(value);
            if (keys.length !== value.length) {
                throw new UnsupportedValueError('sparse arrays or arrays with own properties require the tagged-value protocol');
            }
            const result = new Array(value.length);
            for (let index = 0; index < value.length; index += 1) {
                if (!Object.prototype.hasOwnProperty.call(value, index)) {
                    throw new UnsupportedValueError('sparse arrays require the tagged-value protocol');
                }
                result[index] = cloneJSONCompatible(value[index], active);
            }
            return result;
        }

        const prototype = Object.getPrototypeOf(value);
        if (prototype !== Object.prototype && prototype !== null) {
            throw new UnsupportedValueError('non-plain objects require the tagged-value protocol');
        }
        const result = Object.create(null);
        Object.keys(value).forEach(function cloneProperty(key) {
            result[key] = cloneJSONCompatible(value[key], active);
        });
        return result;
    } finally {
        active.delete(value);
    }
};

const classifyExecutionError = function classifyExecutionError(error) {
    if (error instanceof OracleError) {
        return error.code;
    }
    if (error instanceof RangeError && /limit exceeded/i.test(error.message)) {
        return 'limit_exceeded';
    }
    if (error instanceof RangeError && /depth/i.test(error.message)) {
        return 'depth_exceeded';
    }
    return 'execution_error';
};

const errorPayload = function errorPayload(error) {
    return {
        code: classifyExecutionError(error),
        name: error && typeof error.name === 'string' ? error.name : 'Error',
        message: error && typeof error.message === 'string' ? error.message : String(error)
    };
};

let verifiedState;
let qs;
try {
    verifiedState = loadAndVerifyManifest();
    qs = require(path.join(upstreamRoot, 'lib', 'index.js'));
} catch (error) {
    const fatal = JSON.stringify({
        schema: PROTOCOL_SCHEMA,
        id: null,
        ok: false,
        error: {
            code: error instanceof OracleError ? error.code : 'startup_failed',
            name: error && error.name ? error.name : 'Error',
            message: error && error.message ? error.message : String(error)
        }
    }) + '\n';
    fs.writeSync(1, fatal);
    process.exit(78);
}

let handshaken = false;

const handleRequest = function handleRequest(request) {
    if (!isRecord(request)) {
        throw new OracleError('invalid_request', 'request must be a JSON object');
    }
    if (request.schema !== PROTOCOL_SCHEMA) {
        throw new OracleError('invalid_schema', 'unsupported request schema');
    }
    if (typeof request.id !== 'string' || request.id.length === 0 || request.id.length > MAX_ID_LENGTH) {
        throw new OracleError('invalid_request', 'request id must be a nonempty bounded string');
    }

    if (request.op === 'handshake') {
        handshaken = true;
        return verifiedState;
    }
    if (!handshaken) {
        throw new OracleError('handshake_required', 'the first successful request must be a handshake');
    }

    if (request.op === 'parse') {
        if (typeof request.input !== 'string') {
            throw new OracleError('invalid_input', 'parse input must be a string');
        }
        const parseOptions = validateOptions(request.options, parseOptionValidators);
        return cloneJSONCompatible(qs.parse(request.input, parseOptions), new Set());
    }

    if (request.op === 'stringify') {
        const stringifyOptions = validateOptions(request.options, stringifyOptionValidators);
        return qs.stringify(request.input, stringifyOptions);
    }

    throw new OracleError('unsupported_operation', 'operation must be handshake, parse, or stringify');
};

const writeResponse = function writeResponse(response) {
    let encoded = Buffer.from(JSON.stringify(response) + '\n', 'utf8');
    if (encoded.length > MAX_RESPONSE_BYTES) {
        encoded = Buffer.from(JSON.stringify({
            schema: PROTOCOL_SCHEMA,
            id: response.id || null,
            ok: false,
            error: {
                code: 'response_too_large',
                name: 'OracleError',
                message: 'oracle response exceeds the configured byte limit'
            }
        }) + '\n', 'utf8');
    }
    fs.writeSync(1, encoded);
};

const processLine = function processLine(line) {
    let request;
    let requestID = null;
    try {
        request = JSON.parse(line.toString('utf8'));
        if (isRecord(request) && typeof request.id === 'string') {
            requestID = request.id;
        }
        const value = handleRequest(request);
        writeResponse({ schema: PROTOCOL_SCHEMA, id: requestID, ok: true, value: value });
    } catch (error) {
        writeResponse({
            schema: PROTOCOL_SCHEMA,
            id: requestID,
            ok: false,
            error: errorPayload(error)
        });
    }
};

let pendingParts = [];
let pendingLength = 0;
let droppingOversizedLine = false;

const appendSegment = function appendSegment(segment) {
    if (droppingOversizedLine || segment.length === 0) {
        return;
    }
    // NDJSON limits include the mandatory trailing newline, matching the Go
    // client check before it writes a request.
    if (pendingLength + segment.length + 1 > MAX_REQUEST_BYTES) {
        pendingParts = [];
        pendingLength = 0;
        droppingOversizedLine = true;
        return;
    }
    pendingParts.push(segment);
    pendingLength += segment.length;
};

const finishLine = function finishLine() {
    if (droppingOversizedLine) {
        writeResponse({
            schema: PROTOCOL_SCHEMA,
            id: null,
            ok: false,
            error: {
                code: 'request_too_large',
                name: 'OracleError',
                message: 'request exceeds the configured byte limit'
            }
        });
    } else if (pendingLength > 0) {
        let line = Buffer.concat(pendingParts, pendingLength);
        if (line.length > 0 && line[line.length - 1] === 0x0D) {
            line = line.subarray(0, line.length - 1);
        }
        if (line.length > 0) {
            processLine(line);
        }
    }

    pendingParts = [];
    pendingLength = 0;
    droppingOversizedLine = false;
};

process.stdin.on('data', function onData(chunk) {
    let start = 0;
    for (let index = 0; index < chunk.length; index += 1) {
        if (chunk[index] === 0x0A) {
            appendSegment(chunk.subarray(start, index));
            finishLine();
            start = index + 1;
        }
    }
    appendSegment(chunk.subarray(start));
});

process.stdin.on('end', function onEnd() {
    if (pendingLength > 0 || droppingOversizedLine) {
        finishLine();
    }
});

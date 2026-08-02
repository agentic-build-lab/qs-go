'use strict';

const crypto = require('node:crypto');
const path = require('node:path');

const configuredUpstreamRoot = process.env.QSGO_BENCH_UPSTREAM_ROOT;
const expectedUpstreamRootSHA256 = process.env.QSGO_BENCH_UPSTREAM_ROOT_SHA256;
if (!configuredUpstreamRoot || !expectedUpstreamRootSHA256) {
    throw new Error('QSGO_BENCH_UPSTREAM_ROOT and QSGO_BENCH_UPSTREAM_ROOT_SHA256 are required');
}
const upstreamRoot = path.resolve(configuredUpstreamRoot);
const upstreamRootIdentity = process.platform === 'win32' ? upstreamRoot.toLowerCase() : upstreamRoot;
const upstreamRootSHA256 = crypto.createHash('sha256').update(upstreamRootIdentity, 'utf8').digest('hex');
if (upstreamRootSHA256 !== expectedUpstreamRootSHA256.toLowerCase()) {
    throw new Error('configured upstream root does not match its verified identity');
}
const qs = require(path.join(upstreamRoot, 'lib', 'index.js'));
const result = qs.parse('a%5Bb%5D=c');

if (!result.a || result.a.b !== 'c') {
    process.exitCode = 1;
}

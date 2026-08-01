'use strict';

const path = require('node:path');

const upstreamRoot = path.resolve(__dirname, '..', '..', 'upstream_qs');
const qs = require(path.join(upstreamRoot, 'lib', 'index.js'));
const result = qs.parse('a%5Bb%5D=c');

if (!result.a || result.a.b !== 'c') {
    process.exitCode = 1;
}

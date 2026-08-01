'use strict';

const path = require('node:path');

const upstreamRoot = path.resolve(__dirname, '..', '..', 'upstream_qs');
const qs = require(path.join(upstreamRoot, 'lib', 'index.js'));

const SAMPLE_COUNT = 40;
const ITERATIONS_PER_SAMPLE = 500;
const WARMUP_ITERATIONS = 2000;
let benchmarkSink = 0;

const percentile = function percentile(sorted, quantile) {
    const index = Math.floor((sorted.length - 1) * quantile + 0.5);
    return sorted[index];
};

const measure = function measure(name, operation) {
    for (let index = 0; index < WARMUP_ITERATIONS; index += 1) {
        benchmarkSink += operation();
    }
    const samples = [];
    let checksum = 0;
    for (let sample = 0; sample < SAMPLE_COUNT; sample += 1) {
        const started = process.hrtime.bigint();
        for (let iteration = 0; iteration < ITERATIONS_PER_SAMPLE; iteration += 1) {
            checksum += operation();
        }
        const elapsed = Number(process.hrtime.bigint() - started);
        samples.push(elapsed / ITERATIONS_PER_SAMPLE);
    }
    benchmarkSink += checksum;
    const sorted = samples.slice().sort(function sortNumbers(left, right) { return left - right; });
    return {
        name: name,
        median_ns_per_op: percentile(sorted, 0.50),
        p95_ns_per_op: percentile(sorted, 0.95),
        p99_ns_per_op: percentile(sorted, 0.99),
        minimum_ns_per_op: sorted[0],
        maximum_ns_per_op: sorted[sorted.length - 1],
        operations: SAMPLE_COUNT * ITERATIONS_PER_SAMPLE,
        checksum: checksum
    };
};

const flatWorkload = function flatWorkload(size) {
    const parts = [];
    const value = {};
    for (let index = 0; index < size; index += 1) {
        const key = 'k' + index;
        const item = 'value' + index;
        parts.push(key + '=' + item);
        value[key] = item;
    }
    return { query: parts.join('&'), value: value };
};

const nestedWorkload = function nestedWorkload(size) {
    const parts = [];
    const group = {};
    for (let index = 0; index < size; index += 1) {
        const key = 'k' + index;
        const item = 'value' + index;
        parts.push('root[group][' + key + ']=' + item);
        group[key] = item;
    }
    return { query: parts.join('&'), value: { root: { group: group } } };
};

if (global.gc) {
    global.gc();
}
const memoryBefore = process.memoryUsage();
const flat = flatWorkload(100);
const nested = nestedWorkload(20);
const workloads = [
    measure('parse_flat_100', function parseFlat() { return Object.keys(qs.parse(flat.query)).length; }),
    measure('parse_nested_20', function parseNested() { return Object.keys(qs.parse(nested.query)).length; }),
    measure('stringify_flat_100', function stringifyFlat() { return qs.stringify(flat.value).length; }),
    measure('stringify_nested_20', function stringifyNested() { return qs.stringify(nested.value).length; })
];
const memoryAfter = process.memoryUsage();

console.log(JSON.stringify({
    schema: 'qs_original_benchmark/v1',
    runtime: process.version,
    platform: process.platform,
    architecture: process.arch,
    samples: SAMPLE_COUNT,
    iterations_per_sample: ITERATIONS_PER_SAMPLE,
    memory: {
        rss_before_bytes: memoryBefore.rss,
        rss_after_bytes: memoryAfter.rss,
        heap_used_before_bytes: memoryBefore.heapUsed,
        heap_used_after_bytes: memoryAfter.heapUsed
    },
    workloads: workloads,
    sink: benchmarkSink
}, null, 2));

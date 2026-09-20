// Thin client for the sensor API used by every scenario. Each call tags the
// request with a stable `route` so latency can be broken down per endpoint
// in Grafana instead of per concrete URL.
import http from 'k6/http';
import { check, fail } from 'k6';

export const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

const JSON_HEADERS = { 'Content-Type': 'application/json' };

// Bounding box roughly covering Greater London; keeps nearest-sensor queries
// realistic (sensors clustered, queries inside the cluster).
const BOX = { latMin: 51.28, latMax: 51.69, lonMin: -0.51, lonMax: 0.33 };

const TAG_POOL = ['temperature', 'humidity', 'co2', 'roof', 'basement', 'lab', 'outdoor'];

export function randomLocation() {
  return {
    latitude: BOX.latMin + Math.random() * (BOX.latMax - BOX.latMin),
    longitude: BOX.lonMin + Math.random() * (BOX.lonMax - BOX.lonMin),
  };
}

export function randomTags() {
  const n = 1 + Math.floor(Math.random() * 3);
  const out = new Set();
  while (out.size < n) out.add(TAG_POOL[Math.floor(Math.random() * TAG_POOL.length)]);
  return [...out];
}

export function randomTag() {
  return TAG_POOL[Math.floor(Math.random() * TAG_POOL.length)];
}

function params(route, extra = {}) {
  return { headers: JSON_HEADERS, tags: { route, name: route }, ...extra };
}

export function waitUntilReady(timeoutSec = 30) {
  const deadline = Date.now() + timeoutSec * 1000;
  while (Date.now() < deadline) {
    const res = http.get(`${BASE_URL}/readyz`, params('GET /readyz'));
    if (res.status === 200) return;
    // k6's sleep() is not allowed in setup; a short busy-wait is fine here.
    const until = Date.now() + 500;
    while (Date.now() < until) { /* spin */ }
  }
  fail(`API at ${BASE_URL} not ready after ${timeoutSec}s`);
}

export function createSensor(name, location = randomLocation(), tags = randomTags()) {
  const res = http.post(
    `${BASE_URL}/v1/sensors`,
    JSON.stringify({ name, location, tags }),
    params('POST /v1/sensors'),
  );
  check(res, { 'create: 201': (r) => r.status === 201 });
  return res;
}

export function getSensor(name) {
  const res = http.get(`${BASE_URL}/v1/sensors/${name}`, params('GET /v1/sensors/{name}'));
  check(res, { 'get: 200': (r) => r.status === 200 });
  return res;
}

export function listSensors(tag) {
  const url = tag ? `${BASE_URL}/v1/sensors?tag=${tag}` : `${BASE_URL}/v1/sensors`;
  const res = http.get(url, params('GET /v1/sensors'));
  check(res, { 'list: 200': (r) => r.status === 200 });
  return res;
}

export function nearestSensor(location = randomLocation()) {
  const res = http.get(
    `${BASE_URL}/v1/sensors/nearest?lat=${location.latitude}&lon=${location.longitude}`,
    params('GET /v1/sensors/nearest'),
  );
  check(res, { 'nearest: 200': (r) => r.status === 200 });
  return res;
}

export function replaceSensor(name, location = randomLocation(), tags = randomTags()) {
  const res = http.put(
    `${BASE_URL}/v1/sensors/${name}`,
    JSON.stringify({ location, tags }),
    params('PUT /v1/sensors/{name}'),
  );
  check(res, { 'replace: 200': (r) => r.status === 200 });
  return res;
}

export function patchSensor(name, body = { tags: randomTags() }) {
  const res = http.patch(
    `${BASE_URL}/v1/sensors/${name}`,
    JSON.stringify(body),
    params('PATCH /v1/sensors/{name}'),
  );
  check(res, { 'patch: 200': (r) => r.status === 200 });
  return res;
}

export function deleteSensor(name) {
  const res = http.del(`${BASE_URL}/v1/sensors/${name}`, null, params('DELETE /v1/sensors/{name}'));
  check(res, { 'delete: 204': (r) => r.status === 204 });
  return res;
}

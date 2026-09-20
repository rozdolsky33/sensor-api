// Smoke test: one VU walks every endpoint for 30s. Proves the stack is wired
// up (API reachable, metrics flowing to Grafana) before a real load run.
import { sleep } from 'k6';
import {
  waitUntilReady, createSensor, getSensor, listSensors, nearestSensor,
  replaceSensor, patchSensor, deleteSensor, randomTag,
} from './lib/api.js';

export const options = {
  vus: 1,
  duration: '30s',
  thresholds: {
    http_req_failed: ['rate<0.01'],
    checks: ['rate>0.99'],
    http_req_duration: ['p(95)<200'],
  },
};

export function setup() {
  waitUntilReady();
  return { runId: Date.now().toString(36) };
}

export default function ({ runId }) {
  const name = `smoke-${runId}-${__VU}-${__ITER}`;
  createSensor(name);
  getSensor(name);
  listSensors(randomTag());
  nearestSensor();
  replaceSensor(name);
  patchSensor(name);
  deleteSensor(name);
  sleep(0.2);
}

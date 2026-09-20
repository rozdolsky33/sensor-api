// Load test: ramps to 100 concurrent users against a realistic traffic mix.
//
//   setup()   seeds SEED_COUNT sensors so reads have something to hit
//   default   each iteration picks one weighted action:
//               60%  GET  /v1/sensors/{name}        (random seeded sensor)
//               20%  GET  /v1/sensors/nearest       (random coords)
//                5%  GET  /v1/sensors?tag=          (list, filtered)
//               15%  write lifecycle: POST -> PATCH -> DELETE one new sensor
//
// Thresholds give the run a pass/fail verdict; the dashboard shows the shape.
import { sleep } from 'k6';
import {
  waitUntilReady, createSensor, getSensor, listSensors, nearestSensor,
  patchSensor, deleteSensor, randomTag,
} from './lib/api.js';

const SEED_COUNT = Number(__ENV.SEED_COUNT || 500);

export const options = {
  scenarios: {
    ramp: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 50 },   // warm up
        { duration: '60s', target: 50 },   // steady
        { duration: '30s', target: 100 },  // push
        { duration: '60s', target: 100 },  // steady at peak
        { duration: '30s', target: 0 },    // drain
      ],
      gracefulRampDown: '5s',
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    checks: ['rate>0.99'],
    http_req_duration: ['p(95)<50', 'p(99)<150'],
    'http_req_duration{route:GET /v1/sensors/nearest}': ['p(95)<50'],
    'http_req_duration{route:GET /v1/sensors/{name}}': ['p(95)<20'],
  },
  summaryTrendStats: ['avg', 'min', 'med', 'p(90)', 'p(95)', 'p(99)', 'max'],
};

export function setup() {
  waitUntilReady();
  const runId = Date.now().toString(36);
  const names = [];
  for (let i = 0; i < SEED_COUNT; i++) {
    const name = `seed-${runId}-${i}`;
    if (createSensor(name).status === 201) names.push(name);
  }
  return { runId, names };
}

function pick(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

export default function ({ runId, names }) {
  const roll = Math.random();
  if (roll < 0.60) {
    getSensor(pick(names));
  } else if (roll < 0.80) {
    nearestSensor();
  } else if (roll < 0.85) {
    listSensors(randomTag());
  } else {
    const name = `perf-${runId}-${__VU}-${__ITER}`;
    if (createSensor(name).status === 201) {
      patchSensor(name);
      deleteSensor(name);
    }
  }
  // ~10 req/s per VU at 100 VUs ≈ 1000 rps; enough to load an in-memory API
  // without making the k6 container itself the bottleneck.
  sleep(0.1);
}

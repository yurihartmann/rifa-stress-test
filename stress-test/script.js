import { config } from './lib/config.js';
import { prepareRaffle } from './lib/setup.js';
import { checkout } from './scenarios/checkout.js';
import { funnel } from './scenarios/funnel.js';
import { overCapacity } from './scenarios/over-capacity.js';
import { vitrine } from './scenarios/vitrine.js';
import { webhookReplay } from './scenarios/webhook-replay.js';

const runners = {
  vitrine: vitrine,
  checkout: checkout,
  funnel: funnel,
  'webhook-replay': webhookReplay,
  'over-capacity': overCapacity,
};

const scenarios = {};
// Executor id stays alphanumeric. K6_SCENARIO itself keeps the hyphenated name.
scenarios[config.scenario.replace(/-/g, '_')] = {
  executor: 'constant-vus',
  vus: config.vus,
  duration: config.duration,
  gracefulStop: config.gracefulStop,
};

export const options = {
  scenarios: scenarios,
  setupTimeout: config.setupTimeout,
  thresholds: {
    // Placeholders until a measured baseline exists (ADR-0008). Do not treat these as SLOs.
    // 'http_req_failed': ['rate<0.01'],
    // 'http_req_duration{name:GET /v1/raffles/{slug}}': ['p(95)<300'],
    // 'http_req_duration{name:POST /v1/raffles/{slug}/orders}': ['p(95)<800'],
    // 'http_req_duration{name:POST /v1/payments/webhooks}': ['p(95)<800'],
    // 'http_req_duration{name:POST /v1/payments/webhooks replay}': ['p(95)<800'],
    // 'http_req_duration{name:GET /v1/purchases}': ['p(95)<500'],
    // 'checks': ['rate>0.99'],
    // 'ticket_issue_wait_ms': ['p(95)<5000'],
  },
};

export function setup() {
  return prepareRaffle();
}

export default function (data) {
  runners[config.scenario](data);
}

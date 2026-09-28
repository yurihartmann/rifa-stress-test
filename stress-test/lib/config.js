const scenarioNames = [
  'vitrine',
  'checkout',
  'funnel',
  'webhook-replay',
  'over-capacity',
];

function envString(name, fallback) {
  const value = __ENV[name];
  if (value === undefined || value === '') {
    return fallback;
  }
  return value;
}

function envNumber(name, fallback) {
  const raw = __ENV[name];
  if (raw === undefined || raw === '') {
    return fallback;
  }
  const parsed = Number(raw);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    throw new Error(name + ' must be a positive number, got ' + raw);
  }
  return parsed;
}

function envInt(name, fallback) {
  const parsed = envNumber(name, fallback);
  if (Math.floor(parsed) !== parsed) {
    throw new Error(name + ' must be an integer, got ' + parsed);
  }
  return parsed;
}

const scenario = envString('K6_SCENARIO', 'funnel');
if (scenarioNames.indexOf(scenario) === -1) {
  throw new Error(
    'K6_SCENARIO must be one of: ' + scenarioNames.join(', ') + '. Got: ' + scenario,
  );
}

export const config = {
  baseUrl: envString('K6_BASE_URL', 'http://localhost:8080').replace(/\/+$/, ''),
  webhookSecret: envString('K6_WEBHOOK_SECRET', 'lab-webhook-secret'),
  adminToken: envString('K6_ADMIN_TOKEN', 'lab-admin-token'),
  raffleSlug: envString('K6_RAFFLE_SLUG', ''),
  totalTickets: envInt('K6_TOTAL_TICKETS', 1000),
  ticketPriceCents: envInt('K6_TICKET_PRICE_CENTS', 1000),
  quantity: envInt('K6_QUANTITY', 1),
  scenario: scenario,
  scenarioNames: scenarioNames,
  vus: envInt('K6_VUS', 2),
  duration: envString('K6_DURATION', '20s'),
  gracefulStop: envString('K6_GRACEFUL_STOP', '30s'),
  setupTimeout: envString('K6_SETUP_TIMEOUT', '120s'),
  httpTimeout: envString('K6_HTTP_TIMEOUT', '60s'),
  pollTimeoutMs: envInt('K6_POLL_TIMEOUT_MS', 15000),
  pollIntervalSec: envNumber('K6_POLL_INTERVAL_SEC', 0.5),
};

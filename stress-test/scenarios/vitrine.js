import { check } from 'k6';
import { raffleChecks } from '../lib/contract.js';
import { getRaffle } from '../lib/http.js';
import { readJson } from '../lib/json.js';

export function vitrine(data) {
  const res = getRaffle(data.slug);
  check(res, raffleChecks(readJson(res), data.slug));
}

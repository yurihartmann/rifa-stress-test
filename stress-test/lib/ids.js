export function stressEmail() {
  return ('vu-' + __VU + '-iter-' + __ITER + '@stress.local').toLowerCase();
}

export function uniqueEventId(paymentId) {
  return ('vu-' + __VU + '-iter-' + __ITER + '-' + paymentId).toLowerCase();
}

export function replayEventId() {
  return ('replay-vu-' + __VU + '-iter-' + __ITER).toLowerCase();
}

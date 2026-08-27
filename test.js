import http from 'k6/http';
import { check } from 'k6';

export const options = {
  vus: 100, // 100 concurrent virtual users
  duration: '10s', // Blast the gateway for 10 seconds
};

// BASE_URL defaults to localhost, since running k6 directly on the host
// (`k6 run test.js`) is what the README's Quickstart describes. Override
// it with `k6 run -e BASE_URL=http://host.docker.internal:8080 test.js` if
// running k6 from inside a container against a gateway on the host.
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function () {
  const res = http.get(`${BASE_URL}/users`);
  check(res, { 'status was 200': (r) => r.status == 200 });
}
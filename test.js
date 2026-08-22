import http from 'k6/http';
import { check } from 'k6';

export const options = {
  vus: 100, // 100 concurrent virtual users
  duration: '10s', // Blast the gateway for 10 seconds
};

export default function () {
  // host.docker.internal allows this k6 container to talk to your Mac's localhost
  const res = http.get('http://host.docker.internal:8080/users');
  check(res, { 'status was 200': (r) => r.status == 200 });
}
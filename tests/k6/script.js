import { getUrl, options, endpoints, payloads, headers } from './test-config.js';
import ws from 'k6/ws';
import http from 'k6/http';
import { check } from 'k6';
export {options};     // This will be the options from the config. They will be used to run the test.

/*
  TODO: Find way to hook up a web dashboard to this.
  k6 run --out json=results.json script.js
  k6 run --summary-export=summary.json script.js   
*/

function runGamelaunch() {
  const url = getUrl('http', 'gamelaunch');
  const res = http.post(url, payloads.gamelaunch(), { headers });

  check(res, {
    'status is 200': (r) => r.status === 200,
    'has token': (r) => JSON.parse(r.body).token !== undefined,
    'has url': (r) => JSON.parse(r.body).url !== undefined,
  });

  return res;
}

export default function () {
  runGamelaunch()
}

import { getUrlHttps, getUrlHttp, options, endpoints, payloads, headers, toWsUrl } from './test-config.js';
import ws from 'k6/ws';
import http from 'k6/http';
import { check, fail } from 'k6';
export {options};     // This will be the options from the config. They will be used to run the test.
import { Trend } from 'k6/metrics';

export let connectTime = new Trend('ws_connect_message_time', true);

/*
  TODO: Find way to hook up a web dashboard to this.
  k6 run --out json=results.json script.js
  k6 run --summary-export=summary.json script.js   
*/

function runGamelaunch() {
  const url = getUrlHttps('http', 'gamelaunch');
  const res = http.post(url, payloads.gamelaunch(), { headers });

  console.log(`Status: ${res.status}`);
  console.log(`Body: ${res.body}`);

  let data;
  try {
    data = JSON.parse(res.body);
  } catch (e) {
    fail(`Invalid JSON response: ${res.body}`);
  }
  console.log('Parsed data:', JSON.stringify(data));
  console.log('Data keys:', Object.keys(data));

  check(res, {
    'status is 200': (r) => r.status === 200,
  });

  return data.url;
}

export function connectWebSocket(wsConUrl) {
  console.log('Connecting to websocket');
  const start = Date.now();

  const res = ws.connect(wsConUrl, null, function (socket) {
    socket.on('open', () => {
      console.log('WebSocket connection opened');
    });

    socket.on('message', (msg) => {
      console.log(`Received: ${msg}`);
      const data = JSON.parse(msg);

      if (data.command === 'connected') {
        const elapsed = Date.now() - start;
        console.log(`Connected message received after ${elapsed} ms`);
        connectTime.add(elapsed); // record metric
      }
    });

    socket.on('close', () => {
      console.log('WebSocket closed');
    });

    socket.setTimeout(() => {
      console.log('Closing socket after 3s');
      socket.close();
    }, 3000);
  });

  check(res, {
    'ws connection status is 101': (r) => r && r.status === 101,
  });
}

export default function () {
  const gameUrl = runGamelaunch();
  console.log("Game launch finished")
  const wsUrl = toWsUrl(gameUrl);
  console.log("Url transformed")
  connectWebSocket(wsUrl);
}
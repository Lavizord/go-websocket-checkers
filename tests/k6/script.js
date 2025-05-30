import { getUrlHttps, getUrlHttp, options, endpoints, payloads, headers, toWsUrl } from './test-config.js';
import ws from 'k6/ws';
import http from 'k6/http';
import { check, fail } from 'k6';
export {options};     // This will be the options from the config. They will be used to run the test.
import { Trend } from 'k6/metrics';
import { getMsgLeaveQueue, getMsgQueueRequest } from './helpers.js';

export let gamelaunchResponseTime = new Trend('http_gamelaunch_response_time', true);
export let connectedMsgTime = new Trend('ws_connected_message_time', true);
export let queueConfirmationResponseTime = new Trend('ws_queue_confirmation_response_message_time', true);
export let pairedResponseTime = new Trend('ws_paired_response_message_time', true);
export let connectTime = new Trend('ws_connect_time', true);

/*
  TODO: Find way to hook up a web dashboard to this.
  k6 run --out json=results.json script.js
  k6 run --summary-export=summary.json script.js   
*/

function runGamelaunch() {
  const url = getUrlHttps('http', 'gamelaunch');
  const start = Date.now();
  const res = http.post(url, payloads.gamelaunch(), { headers });
  const elapsed = Date.now() - start; 
  gamelaunchResponseTime.add(elapsed);
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

function connectWebSocket(wsConUrl) {
  console.log('Connecting to websocket');
  const start = Date.now();
  let startQueueRequest;

  const res = ws.connect(wsConUrl, null, function (socket) {
    socket.on('open', () => {
      const elapsed = Date.now() - start; 
      connectTime.add(elapsed)
      console.log('WebSocket connection opened');
    });
    
    socket.on('message', (msg) => {
      console.log(`Received: ${msg}`);
      const data = JSON.parse(msg);

      if (data.command === 'connected') {
        const elapsed = Date.now() - start;
        console.log(`Connected message received after ${elapsed} ms`);
        connectedMsgTime.add(elapsed); // record metric
        socket.send(
          getMsgQueueRequest()
        ) 
        startQueueRequest = Date.now()
        // TODO: fazer um check para termos recebido a connection.
        //check(res, {
        //  'ws connection status is 101': (r) => r && r.status === 101,
        //});
      }
      
      if (data.command === 'queue_confirmation' && data.value) {
        const elapsed = Date.now() - startQueueRequest;
        console.log(`Queue Confirmation message received after ${elapsed} ms`);
        queueConfirmationResponseTime.add(elapsed); // record metric
         // TODO: fazer um check para termos recebido a queue confirmation.
        //check(res, {
        //  'ws connection status is 101': (r) => r && r.status === 101,
        //});
      }

      if(data.command === "paired" && data.value)
      {
        const elapsed = Date.now() - startQueueRequest;
        const { value } = data;
        console.log({value});

        pairedResponseTime.add(elapsed);
      }
    });

    socket.on('close', () => {
      console.log('WebSocket closed');
    });

    socket.setTimeout(() => {
      console.log('Closing socket after 3s');
      socket.send(
        getMsgLeaveQueue()
      )
      socket.close();
    }, 3000);
  });

  check(res, {
    'ws connection status is 101': (r) => r && r.status === 101,
  });
}

export default function () {
  // Player 1
  const gameUrl = runGamelaunch();
  console.log("Game launch finished")
  const wsUrl = toWsUrl(gameUrl);
  console.log("Url transformed")
  connectWebSocket(wsUrl);
  
  // Player2
  const gameUrl_2 = runGamelaunch();
  console.log("Game launch finished")
  const wsUrl_2 = toWsUrl(gameUrl_2);
  console.log("Url transformed")
  connectWebSocket(wsUrl_2);
}
import ws from 'k6/ws';
import http from 'k6/http';
import { check, fail, sleep } from 'k6';
import { Trend } from 'k6/metrics';
import {
  getUrlHttps,
  toWsUrl,
  headers,
  options as customOptions,
  payloads
} from './test-config.js';
import { getMsgQueueRequest, getMsgLeaveQueue, getMsgReadyRoom, getMsgLeaveRoom } from './helpers.js';

export let gamelaunchResponseTime = new Trend('http_gamelaunch_response_time', true);
export let connectedMsgTime = new Trend('ws_connected_message_time', true);
export let queueConfirmationResponseTime = new Trend('ws_queue_confirmation_response_message_time', true);
export let pairedResponseTime = new Trend('ws_paired_response_message_time', true);
export let connectTime = new Trend('ws_connect_time', true);


export let options = {
  insecureSkipTLSVerify: true,
  scenarios: {
    player1: {
      executor: 'per-vu-iterations',
      vus: 2,
      iterations: 1,
      exec: 'player1'
    }
  },
};

function runGamelaunch() {
  const url = getUrlHttps('http', 'gamelaunch');
  let start = Date.now();
  const res = http.post(url, payloads.gamelaunch(), { headers });
  const elapsed = Date.now() - start; 
  gamelaunchResponseTime.add(elapsed);
  console.log(`${__VU} Status: ${res.status}`);
  console.log(`${__VU} Body: ${res.body}`);

  let data;
  try {
    data = JSON.parse(res.body);
  } catch (e) {
    fail(`${__VU} Invalid JSON response: ${res.body}`);
  }
  console.log(`${__VU} - Parsed data:`, JSON.stringify(data));
  console.log(`${__VU} - Data keys:`, Object.keys(data));

  check(res, {
    'status is 200': (r) => r.status === 200,
  });
  return data.url;
}

function runPlayerVU(responseDelay = 0) {

  let opened = false;
  let wsUrl = runGamelaunch() 
  console.log(`${__VU} - Game launch finished`)
  wsUrl = toWsUrl(wsUrl);
  console.log(`${__VU} - Url transformed`)

  let start = Date.now();
  ws.connect(wsUrl, null, function (socket) {
    let queueSent = false;
    let roomTimerReceived = false;
    let gameInfo = false;
    let queueConfirmationReceived = false;
    let pairedReceived = false;
    let startQueueRequest;
    let startPairedTimer;

    setTimeout(() => {
      if (!opened) {
        check(false, { 'WebSocket opened': (v) => v === true });
      }
    }, 3000); // 3s timeout
    
    socket.on('open', () => {
      const elapsed = Date.now() - start; 
      connectTime.add(elapsed)
      opened = true;
      check(true, { 'WebSocket opened': (v) => v === true });
      console.log(`${__VU} - WebSocket opened`);
    });

    socket.on('message', (msg) => {
      const data = JSON.parse(msg);
      console.log(`${__VU} - received:`, data);

      if (data.command === 'connected' && !queueSent) {
        const elapsed = Date.now() - start;
        connectedMsgTime.add(elapsed); // record metric 
        check(true, { 'Connected message received': (v) => v === true });       
        //sleep(responseDelay); // Optional delay to control timing
        socket.send(getMsgQueueRequest({ value: 100 }));
        startQueueRequest = Date.now()
        queueSent = true;
      }

      if (data.command === 'queue_confirmation' && data.value) {
        const elapsed = Date.now() - startQueueRequest;
        queueConfirmationResponseTime.add(elapsed); // record metric
        check(true, { 'Queue confirmation message received': (v) => v === true });       
        //console.log(`${__VU} - Queue Confirmation message received after ${elapsed} ms`);
        startPairedTimer = Date.now();
      }

      if (data.command === 'paired' && data.value) {
        const elapsed = Date.now() - startPairedTimer;
        pairedResponseTime.add(elapsed);
        check(true, { 'Paired message received': (v) => v === true });       
        console.log(`${__VU} - received paired:`, data.value);
        //sleep(responseDelay)
        socket.send(getMsgReadyRoom({ value: true })); // simulate readiness
        pairedReceived = true;
      }

      if (data.command === 'game_info' && !queueSent) {
        check(true, { 'Game info message received': (v) => v === true });   
        gameInfo = true    
      }
      if (data.command === 'room_timer' && !queueSent) {
        check(true, { 'Room timer message received': (v) => v === true });   
        roomTimerReceived = true;    
      }

    });

    socket.setTimeout(() => {
      if (!pairedReceived) {
        check(false, { 'Paired message received': (v) => v === true });   
      }
      if (!gameInfo) {
        check(false, { 'Game info message received': (v) => v === true });   
      }
      if (!roomTimerReceived) {
        check(false, { 'Room timer message received': (v) => v === true });   
      }
      socket.send(getMsgLeaveQueue());
      socket.send(getMsgLeaveRoom());
      socket.close();
    }, 30000);
  });
}

export function player1() {
  runPlayerVU(0.2);
}

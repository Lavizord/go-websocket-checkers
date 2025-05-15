export const options = {
  vus: 1,
  //duration: '10s',
  iterations: 1,
  insecureSkipTLSVerify: true,    // For now, while we dont have the SSL stuff handled.
  //thresholds: {
  //  http_req_duration: ['p(95)<500'],
  //},
};

export const baseUrl = "checkers-alb-1448726329.eu-central-1.elb.amazonaws.com"
//export const baseUrl = "localhost"

export const endpoints = {
  gamelaunch: '/api/gamelaunch',
  gameCon: '/ws/checkers',
};

export function getUrlHttps(type, endpointKey) {
  const protocol = type === 'ws' ? 'wss://' : 'https://';
  return `${protocol}${baseUrl}${endpoints[endpointKey]}`;
}
export function getUrlHttp(type, endpointKey) {
  const protocol = type === 'ws' ? 'ws://' : 'http://';
  return `${protocol}${baseUrl}${endpoints[endpointKey]}`;
}

export function toWsUrl(originalUrl) {
  const queryIndex = originalUrl.indexOf('?');
  const query = originalUrl.substring(queryIndex + 1);
  const wsUrlBase = getUrlHttps("ws", "gameCon");
  return `${wsUrlBase}?${query}`;
}

export const headers = {
	'Content-Type': 'application/json',
  Authorization: 'Bearer token',
};

export const payloads = {
  httpPayload: JSON.stringify({ key: 'value' }),

  gamelaunch: () => JSON.stringify({
    currency: 'BRL',
    operator_name: 'TestOp',
    gameid: 'damasSokkerDuel',
    language: 'pt',
    token: Math.random().toString(36).substring(2), // dummy random token. Used with TestOp
  }),

  wsMessages: [
    { type: 'ping' },
    { type: 'subscribe', channel: 'news' },
  ],
};

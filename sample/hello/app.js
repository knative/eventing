const http = require('http');
const os = require('os');

console.log("server starting...");

var handler = function(request, response) {
  if (request.method !== 'POST') {
    response.writeHead(405);
    response.end();
    return;
  }
  if (request.url !== '/') {
    response.writeHead(404);
    response.end();
    return;
  }
  const hostname = os.hostname();
  console.log(`received request to ${hostname} from ${request.connection.remoteAddress}`);
  response.writeHead(200);
  response.end(`hello from ${hostname}\n`);
};

var www = http.createServer(handler);
www.listen(8080);


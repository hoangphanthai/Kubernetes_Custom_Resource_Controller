const http = require('http');
const os = require('os');
console.log("Server starting...");

var handler = function(request, response) {
  console.log("Received request from " + request.connection.remoteAddress);
  response.setHeader('Content-Type', 'application/json');
  response.writeHead(200);
  const obj = {POSTGRES_DB: "dbRB", USER: "admin", PASSWORD: "pwd" };
  response.end(JSON.stringify(obj));
};
var www = http.createServer(handler);
www.listen(8080);
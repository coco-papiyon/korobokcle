const http = require('http')
const fs = require('fs')
const path = require('path')

const port = Number(process.env.PORT || 4173)
const root = __dirname

function contentType(filePath) {
  switch (path.extname(filePath)) {
    case '.html':
      return 'text/html; charset=utf-8'
    case '.css':
      return 'text/css; charset=utf-8'
    case '.js':
      return 'text/javascript; charset=utf-8'
    case '.json':
      return 'application/json; charset=utf-8'
    default:
      return 'text/plain; charset=utf-8'
  }
}

const server = http.createServer((req, res) => {
  const requestPath = req.url === '/' ? '/index.html' : req.url || '/index.html'
  const filePath = path.join(root, requestPath.split('?')[0])

  fs.readFile(filePath, (err, data) => {
    if (err) {
      res.statusCode = 404
      res.setHeader('Content-Type', 'text/plain; charset=utf-8')
      res.end('Not Found')
      return
    }
    res.statusCode = 200
    res.setHeader('Content-Type', contentType(filePath))
    res.end(data)
  })
})

server.listen(port, () => {
  console.log(`Mock app running at http://localhost:${port}`)
})

import http.server
import socketserver
import os

os.chdir(os.path.dirname(os.path.abspath(__file__)))

class ReusableServer(socketserver.TCPServer):
    allow_reuse_address = True

Handler = http.server.SimpleHTTPRequestHandler
Handler.extensions_map.update({
    '.js': 'application/javascript',
    '.css': 'text/css',
    '.html': 'text/html'
})

with ReusableServer(("127.0.0.1", 8000), Handler) as httpd:
    print("Serving frontend at http://127.0.0.1:8000", flush=True)
    httpd.serve_forever()

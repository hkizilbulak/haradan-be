import sys
filepath = '/Users/admin/Desktop/projects/haradan-be/internal/transport/http/handler/server.go'
with open(filepath, 'r') as f:
    content = f.read()

content = content.replace('http.StatusNotImplemented', '501')

with open(filepath, 'w') as f:
    f.write(content)

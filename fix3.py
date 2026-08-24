import sys
server_path = '/Users/admin/Desktop/projects/haradan-be/internal/transport/http/handler/server.go'
with open(server_path, 'r') as f:
    content = f.read()

if 'openapi_types "github.com/oapi-codegen/runtime/types"' not in content:
    content = content.replace('import (', 'import (\n\topenapi_types "github.com/oapi-codegen/runtime/types"\n\t"net/http"\n')

content = content.replace('s.NotImplemented(c)', 'c.Status(http.StatusNotImplemented)')

with open(server_path, 'w') as f:
    f.write(content)


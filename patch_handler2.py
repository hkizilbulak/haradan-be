import sys
filepath = '/Users/admin/Desktop/projects/haradan-be/internal/transport/http/handler/studfarm/handler.go'
with open(filepath, 'r') as f:
    content = f.read()

content = content.replace('Email:     req.Email,', 'Email:     string(req.Email),')
content = content.replace('Email:     sf.Email,', 'Email:     openapi_types.Email(sf.Email),')

with open(filepath, 'w') as f:
    f.write(content)

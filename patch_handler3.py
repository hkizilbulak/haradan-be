import sys
filepath = '/Users/admin/Desktop/projects/haradan-be/internal/transport/http/handler/studfarm/handler.go'
with open(filepath, 'r') as f:
    content = f.read()

content = content.replace('Email:     openapi_types.Email(sf.Email),', 'Email:     sf.Email,')
content = content.replace('Email:     string(req.Email),', 'Email:     string(req.Email),')

with open(filepath, 'w') as f:
    f.write(content)

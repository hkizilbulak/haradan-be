import sys

handler_path = '/Users/admin/Desktop/projects/haradan-be/internal/transport/http/handler/studfarm/handler.go'
with open(handler_path, 'r') as f:
    lines = f.readlines()

with open(handler_path, 'w') as f:
    f.writelines(lines[:124])


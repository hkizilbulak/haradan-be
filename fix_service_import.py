import sys
filepath = '/Users/admin/Desktop/projects/haradan-be/internal/application/studfarm/service.go'
with open(filepath, 'r') as f:
    content = f.read()

content = content.replace('"github.com/hkizilbulak/haradan-be/internal/platform/apperr"', '"github.com/hkizilbulak/haradan-be/internal/domain/apperr"')

with open(filepath, 'w') as f:
    f.write(content)
print("Fixed service.go import")

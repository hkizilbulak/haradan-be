import sys
filepath = '/Users/admin/Desktop/projects/haradan-be/internal/infrastructure/postgres/studfarm/repository.go'
with open(filepath, 'r') as f:
    content = f.read()

content = content.replace('if pg.IsUniqueViolation(err) {', 'if err != nil && strings.Contains(err.Error(), "unique constraint") {')

# Add "strings" to imports if not there
if '"strings"' not in content:
    content = content.replace('import (\n', 'import (\n\t"strings"\n')

with open(filepath, 'w') as f:
    f.write(content)

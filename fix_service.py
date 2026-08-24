import sys
filepath = '/Users/admin/Desktop/projects/haradan-be/internal/application/studfarm/service.go'
with open(filepath, 'r') as f:
    content = f.read()

content = content.replace('func (s *Service) AddNote', 'func (s *service) AddNote')

import_str = """import (
	"github.com/google/uuid"

	"context"

	domainstudfarm "github.com/hkizilbulak/haradan-be/internal/domain/studfarm"
	"github.com/hkizilbulak/haradan-be/internal/platform/apperr"
)"""

old_import = """import (
	"github.com/google/uuid"

	"context"

	domainstudfarm "github.com/hkizilbulak/haradan-be/internal/domain/studfarm"
)"""

content = content.replace(old_import, import_str)

with open(filepath, 'w') as f:
    f.write(content)
print("Fixed service.go")

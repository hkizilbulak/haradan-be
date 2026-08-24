import sys
filepath = '/Users/admin/Desktop/projects/haradan-be/internal/infrastructure/postgres/studfarm/repository.go'
with open(filepath, 'r') as f:
    content = f.read()

target = '''
	q := `
		INSERT INTO hrd_stud_farms (first_name, last_name, email, phone, location)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, first_name, last_name, email, phone, location, created_at, updated_at
	`
	var sf domainstudfarm.StudFarm
	err := r.db.QueryRow(ctx, q, param.FirstName, param.LastName, param.Email, param.Phone, param.Location).Scan(
'''

replacement = '''
	id := uuid.New()
	now := time.Now().UTC()
	q := `
		INSERT INTO hrd_stud_farms (id, first_name, last_name, email, phone, location, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, first_name, last_name, email, phone, location, created_at, updated_at
	`
	var sf domainstudfarm.StudFarm
	err := r.db.QueryRow(ctx, q, id, param.FirstName, param.LastName, param.Email, param.Phone, param.Location, now, now).Scan(
'''
content = content.replace(target, replacement)

if '"github.com/google/uuid"' not in content:
    content = content.replace('import (', 'import (\n\t"github.com/google/uuid"')

with open(filepath, 'w') as f:
    f.write(content)

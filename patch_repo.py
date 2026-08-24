import sys
filepath = '/Users/admin/Desktop/projects/haradan-be/internal/infrastructure/postgres/studfarm/repository.go'
with open(filepath, 'r') as f:
    content = f.read()

new_func = """
func (r *Repository) AddNote(ctx context.Context, param domainstudfarm.NoteCreateParam) error {
	id := uuid.New()
	now := time.Now().UTC()
	q := `
		INSERT INTO hrd_stud_farm_notes (id, stud_farm_id, interviewer_name, interview_date, notes_url, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(ctx, q, id, param.StudFarmID, param.InterviewerName, param.InterviewDate, param.Notes, now)
	if err != nil {
		return err
	}
	return nil
}
"""

if 'func (r *Repository) AddNote' not in content:
    content += '\n' + new_func
    with open(filepath, 'w') as f:
        f.write(content)
        print("Successfully patched repository.go")
else:
    print("Already patched repo")

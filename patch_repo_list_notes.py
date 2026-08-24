import sys
filepath = '/Users/admin/Desktop/projects/haradan-be/internal/infrastructure/postgres/studfarm/repository.go'
with open(filepath, 'r') as f:
    content = f.read()

new_func = """
func (r *Repository) ListNotes(ctx context.Context, studFarmId uuid.UUID) ([]domainstudfarm.Note, error) {
	q := `
		SELECT id, stud_farm_id, interviewer_name, interview_date, notes_url, created_at
		FROM hrd_stud_farm_notes
		WHERE stud_farm_id = $1
		ORDER BY interview_date DESC, created_at DESC
	`
	rows, err := r.db.Query(ctx, q, studFarmId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []domainstudfarm.Note
	for rows.Next() {
		var n domainstudfarm.Note
		if err := rows.Scan(&n.ID, &n.StudFarmID, &n.InterviewerName, &n.InterviewDate, &n.Notes, &n.CreatedAt); err != nil {
			return nil, err
		}
		notes = append(notes, n)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return notes, nil
}
"""

if 'func (r *Repository) ListNotes' not in content:
    content += '\n' + new_func
    with open(filepath, 'w') as f:
        f.write(content)
        print("Successfully patched repository.go with ListNotes")
else:
    print("Already patched repo with ListNotes")

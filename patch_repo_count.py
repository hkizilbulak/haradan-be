import sys
filepath = '/Users/admin/Desktop/projects/haradan-be/internal/infrastructure/postgres/studfarm/repository.go'
with open(filepath, 'r') as f:
    content = f.read()

old_query = """		SELECT 
			f.id, f.first_name, f.last_name, f.email, f.phone, f.location, f.created_at, f.updated_at,
			n.interview_date, n.interviewer_name, n.notes_url
		FROM hrd_stud_farms f
		LEFT JOIN LATERAL (
			SELECT interview_date, interviewer_name, notes_url
			FROM hrd_stud_farm_notes
			WHERE stud_farm_id = f.id
			ORDER BY interview_date DESC
			LIMIT 1
		) n ON true"""

new_query = """		SELECT 
			f.id, f.first_name, f.last_name, f.email, f.phone, f.location, f.created_at, f.updated_at,
			n.interview_date, n.interviewer_name, n.notes_url,
			COALESCE(c.cnt, 0) as interview_count
		FROM hrd_stud_farms f
		LEFT JOIN LATERAL (
			SELECT interview_date, interviewer_name, notes_url
			FROM hrd_stud_farm_notes
			WHERE stud_farm_id = f.id
			ORDER BY interview_date DESC
			LIMIT 1
		) n ON true
		LEFT JOIN LATERAL (
			SELECT COUNT(*) as cnt
			FROM hrd_stud_farm_notes
			WHERE stud_farm_id = f.id
		) c ON true"""

old_scan = """		err := rows.Scan(
			&item.ID, &item.FirstName, &item.LastName, &item.Email, &item.Phone, &item.Location, &item.CreatedAt, &item.UpdatedAt,
			&item.LatestInterviewDate, &item.InterviewerName, &item.InterviewNotesURL,
		)"""

new_scan = """		err := rows.Scan(
			&item.ID, &item.FirstName, &item.LastName, &item.Email, &item.Phone, &item.Location, &item.CreatedAt, &item.UpdatedAt,
			&item.LatestInterviewDate, &item.InterviewerName, &item.InterviewNotesURL, &item.InterviewCount,
		)"""

if old_query in content:
    content = content.replace(old_query, new_query)
    content = content.replace(old_scan, new_scan)
    with open(filepath, 'w') as f:
        f.write(content)
        print("Successfully patched repository.go with query")
else:
    print("Failed or already patched query")

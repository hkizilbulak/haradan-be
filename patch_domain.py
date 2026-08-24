import sys
filepath = '/Users/admin/Desktop/projects/haradan-be/internal/domain/studfarm/studfarm.go'
with open(filepath, 'r') as f:
    content = f.read()

new_param = """
// NoteCreateParam holds data to create a new stud farm note.
type NoteCreateParam struct {
	StudFarmID      uuid.UUID
	InterviewerName string
	InterviewDate   time.Time
	Notes           string
}
"""

if 'NoteCreateParam' not in content:
    content = content.replace(
        'type CreateParam struct {',
        new_param + '\n' + 'type CreateParam struct {'
    )
    content = content.replace(
        'Delete(ctx context.Context, id uuid.UUID) error',
        'Delete(ctx context.Context, id uuid.UUID) error\n\tAddNote(ctx context.Context, param NoteCreateParam) error'
    )
    with open(filepath, 'w') as f:
        f.write(content)
        print("Successfully patched studfarm.go")
else:
    print("Already patched")

import sys
filepath = '/Users/admin/Desktop/projects/haradan-be/internal/domain/studfarm/studfarm.go'
with open(filepath, 'r') as f:
    content = f.read()

new_note_struct = """
// Note represents a stud farm note.
type Note struct {
	ID              uuid.UUID
	StudFarmID      uuid.UUID
	InterviewerName string
	InterviewDate   time.Time
	Notes           string
	CreatedAt       time.Time
}
"""

if 'type Note struct {' not in content:
    content = content.replace(
        'type NoteCreateParam struct {',
        new_note_struct + '\n' + 'type NoteCreateParam struct {'
    )
    content = content.replace(
        'AddNote(ctx context.Context, param NoteCreateParam) error',
        'AddNote(ctx context.Context, param NoteCreateParam) error\n\tListNotes(ctx context.Context, studFarmId uuid.UUID) ([]Note, error)'
    )
    with open(filepath, 'w') as f:
        f.write(content)
        print("Successfully patched studfarm.go with ListNotes")
else:
    print("studfarm.go already patched with Note")

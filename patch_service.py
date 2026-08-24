import sys
filepath = '/Users/admin/Desktop/projects/haradan-be/internal/application/studfarm/service.go'
with open(filepath, 'r') as f:
    content = f.read()

new_func = """
func (s *Service) AddNote(ctx context.Context, param domainstudfarm.NoteCreateParam) error {
	if param.InterviewerName == "" {
		return apperr.Validation("interviewer name is required")
	}
	if param.Notes == "" {
		return apperr.Validation("notes are required")
	}
	return s.repo.AddNote(ctx, param)
}
"""

if 'func (s *Service) AddNote' not in content:
    content += '\n' + new_func
    with open(filepath, 'w') as f:
        f.write(content)
        print("Successfully patched service.go")
else:
    print("Already patched service")

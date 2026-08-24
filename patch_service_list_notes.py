import sys
filepath = '/Users/admin/Desktop/projects/haradan-be/internal/application/studfarm/service.go'
with open(filepath, 'r') as f:
    content = f.read()

new_func = """
func (s *service) ListNotes(ctx context.Context, studFarmId uuid.UUID) ([]domainstudfarm.Note, error) {
	notes, err := s.repo.ListNotes(ctx, studFarmId)
	if err != nil {
		return nil, err
	}
	if notes == nil {
		notes = make([]domainstudfarm.Note, 0)
	}
	return notes, nil
}
"""

if 'func (s *service) ListNotes' not in content:
    content += '\n' + new_func
    with open(filepath, 'w') as f:
        f.write(content)
        print("Successfully patched service.go with ListNotes")
else:
    print("Already patched service with ListNotes")

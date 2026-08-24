import sys

# 1. Update repository.go
repo_path = '/Users/admin/Desktop/projects/haradan-be/internal/infrastructure/postgres/studfarm/repository.go'
with open(repo_path, 'r') as f:
    repo_content = f.read()

repo_delete = '''
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, "DELETE FROM hrd_stud_farms WHERE id = $1", id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
'''

if 'func (r *Repository) Delete' not in repo_content:
    with open(repo_path, 'a') as f:
        f.write('\n' + repo_delete)

# 2. Update service.go
svc_path = '/Users/admin/Desktop/projects/haradan-be/internal/application/studfarm/service.go'
with open(svc_path, 'r') as f:
    svc_content = f.read()

svc_delete = '''
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
'''
if 'func (s *Service) Delete' not in svc_content:
    with open(svc_path, 'a') as f:
        f.write('\n' + svc_delete)

# 3. Update handler.go
handler_path = '/Users/admin/Desktop/projects/haradan-be/internal/transport/http/handler/studfarm/handler.go'
with open(handler_path, 'r') as f:
    handler_content = f.read()

handler_delete = '''
// DeleteStudFarm implements generated.ServerInterface.
func (h *Handler) DeleteStudFarm(c *gin.Context, studFarmId string) {
	id, err := uuid.Parse(studFarmId)
	if err != nil {
		h.respondError(c, h.logger, err) // Or return 400
		return
	}
	
	err = h.svc.Delete(c.Request.Context(), id)
	if err != nil {
		h.respondError(c, h.logger, err)
		return
	}
	
	c.Status(http.StatusNoContent)
}
'''
if 'func (h *Handler) DeleteStudFarm' not in handler_content:
    if '"github.com/google/uuid"' not in handler_content:
        handler_content = handler_content.replace('import (', 'import (\n\t"github.com/google/uuid"\n')
    
    with open(handler_path, 'w') as f:
        f.write(handler_content + '\n' + handler_delete)


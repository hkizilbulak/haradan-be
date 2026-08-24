import sys
import re

server_path = '/Users/admin/Desktop/projects/haradan-be/internal/transport/http/handler/server.go'
with open(server_path, 'r') as f:
    content = f.read()

# Fix server.go
content = re.sub(
    r'func \(s \*Server\) DeleteStudFarm\(c \*gin\.Context, studFarmId string\) \{[\s\S]*?\}',
    '''func (s *Server) DeleteStudFarm(c *gin.Context, studFarmId openapi_types.UUID) {
	if s.studfarm != nil {
		s.studfarm.DeleteStudFarm(c, studFarmId)
	} else {
		s.NotImplemented(c)
	}
}''',
    content
)

with open(server_path, 'w') as f:
    f.write(content)

handler_path = '/Users/admin/Desktop/projects/haradan-be/internal/transport/http/handler/studfarm/handler.go'
with open(handler_path, 'r') as f:
    content2 = f.read()

content2 = re.sub(
    r'func \(h \*Handler\) DeleteStudFarm\(c \*gin\.Context, studFarmId string\) \{[\s\S]*?\}',
    '''func (h *Handler) DeleteStudFarm(c *gin.Context, studFarmId openapi_types.UUID) {
	err := h.svc.Delete(c.Request.Context(), uuid.UUID(studFarmId))
	if err != nil {
		h.respondError(c, h.logger, err)
		return
	}
	
	c.Status(http.StatusNoContent)
}''',
    content2
)

with open(handler_path, 'w') as f:
    f.write(content2)


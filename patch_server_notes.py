import sys
import re

server_path = '/Users/admin/Desktop/projects/haradan-be/internal/transport/http/handler/server.go'
with open(server_path, 'r') as f:
    content = f.read()

new_func = """
func (s *Server) AddStudFarmNote(c *gin.Context, studFarmId openapi_types.UUID) {
	if s.studfarm != nil {
		s.studfarm.AddStudFarmNote(c, studFarmId)
	} else {
		c.Status(http.StatusNotImplemented)
	}
}
"""

if 'func (s *Server) AddStudFarmNote' not in content:
    content += '\n' + new_func
    with open(server_path, 'w') as f:
        f.write(content)
        print("Successfully patched server.go")
else:
    print("Already patched server")

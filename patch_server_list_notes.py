import sys
filepath = '/Users/admin/Desktop/projects/haradan-be/internal/transport/http/handler/server.go'
with open(filepath, 'r') as f:
    content = f.read()

new_func = """
func (s *Server) ListStudFarmNotes(c *gin.Context, studFarmId openapi_types.UUID) {
	if s.studfarm != nil {
		s.studfarm.ListStudFarmNotes(c, studFarmId)
	} else {
		c.Status(http.StatusNotImplemented)
	}
}
"""

if 'func (s *Server) ListStudFarmNotes' not in content:
    content += '\n' + new_func
    with open(filepath, 'w') as f:
        f.write(content)
        print("Successfully patched server.go with ListStudFarmNotes")
else:
    print("Already patched server with ListStudFarmNotes")

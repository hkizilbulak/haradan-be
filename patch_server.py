import sys
server_path = '/Users/admin/Desktop/projects/haradan-be/internal/transport/http/handler/server.go'
with open(server_path, 'r') as f:
    content = f.read()

delete_method = '''
func (s *Server) DeleteStudFarm(c *gin.Context, studFarmId string) {
	s.studFarmHandler.DeleteStudFarm(c, studFarmId)
}
'''

if 'func (s *Server) DeleteStudFarm' not in content:
    with open(server_path, 'a') as f:
        f.write('\n' + delete_method)


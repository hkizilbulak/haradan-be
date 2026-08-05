package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

func (s *Server) TriggerTJKSync(c *gin.Context) {
	if s.tjk == nil {
		respondNotImplemented(c)
		return
	}
	s.tjk.Trigger(c)
}
func (s *Server) ListTJKSyncRuns(c *gin.Context, p generated.ListTJKSyncRunsParams) {
	if s.tjk == nil {
		respondNotImplemented(c)
		return
	}
	s.tjk.List(c, p)
}
func (s *Server) GetTJKSyncRun(c *gin.Context, id generated.RunIdPath) {
	if s.tjk == nil {
		respondNotImplemented(c)
		return
	}
	s.tjk.Get(c, id)
}
func (s *Server) CancelTJKSync(c *gin.Context, id generated.RunIdPath) {
	if s.tjk == nil {
		respondNotImplemented(c)
		return
	}
	s.tjk.Cancel(c, id)
}
func (s *Server) ListTJKSyncItemErrors(c *gin.Context, id generated.RunIdPath, p generated.ListTJKSyncItemErrorsParams) {
	if s.tjk == nil {
		respondNotImplemented(c)
		return
	}
	s.tjk.ListErrors(c, id, p)
}
func (s *Server) ResolveTJKSyncItemError(c *gin.Context, id generated.ErrorIdPath) {
	if s.tjk == nil {
		respondNotImplemented(c)
		return
	}
	s.tjk.Resolve(c, id)
}
func (s *Server) IgnoreTJKSyncItemError(c *gin.Context, id generated.ErrorIdPath) {
	if s.tjk == nil {
		respondNotImplemented(c)
		return
	}
	s.tjk.Ignore(c, id)
}

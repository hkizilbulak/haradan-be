import sys
filepath = '/Users/admin/Desktop/projects/haradan-be/internal/transport/http/handler/studfarm/handler.go'
with open(filepath, 'r') as f:
    content = f.read()

new_func = """
// AddStudFarmNote implements generated.ServerInterface.
func (h *Handler) AddStudFarmNote(c *gin.Context, studFarmId openapi_types.UUID) {
	var req generated.StudFarmNoteCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, h.logger, err)
		return
	}

	param := domainstudfarm.NoteCreateParam{
		StudFarmID:      uuid.UUID(studFarmId),
		InterviewerName: req.InterviewerName,
		InterviewDate:   req.InterviewDate,
		Notes:           req.Notes,
	}

	if err := h.svc.AddNote(c.Request.Context(), param); err != nil {
		h.respondError(c, h.logger, err)
		return
	}

	c.Status(http.StatusCreated)
}
"""

if 'func (h *Handler) AddStudFarmNote' not in content:
    content += '\n' + new_func
    with open(filepath, 'w') as f:
        f.write(content)
        print("Successfully patched handler.go")
else:
    print("Already patched handler")

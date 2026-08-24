import sys
filepath = '/Users/admin/Desktop/projects/haradan-be/internal/transport/http/handler/studfarm/handler.go'
with open(filepath, 'r') as f:
    content = f.read()

new_func = """
// ListStudFarmNotes implements generated.ServerInterface.
func (h *Handler) ListStudFarmNotes(c *gin.Context, studFarmId openapi_types.UUID) {
	notes, err := h.svc.ListNotes(c.Request.Context(), uuid.UUID(studFarmId))
	if err != nil {
		h.respondError(c, h.logger, err)
		return
	}

	res := generated.StudFarmNoteListResponse{
		Items: make([]generated.StudFarmNoteResponse, len(notes)),
	}
	for i, n := range notes {
		res.Items[i] = generated.StudFarmNoteResponse{
			Id:              openapi_types.UUID(n.ID),
			InterviewDate:   n.InterviewDate,
			InterviewerName: n.InterviewerName,
			Notes:           n.Notes,
			CreatedAt:       n.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, res)
}
"""

if 'func (h *Handler) ListStudFarmNotes' not in content:
    content += '\n' + new_func
    with open(filepath, 'w') as f:
        f.write(content)
        print("Successfully patched handler.go with ListStudFarmNotes")
else:
    print("Already patched handler with ListStudFarmNotes")

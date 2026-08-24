import sys
filepath = '/Users/admin/Desktop/projects/haradan-be/api/openapi.yaml'
with open(filepath, 'r') as f:
    lines = f.readlines()

new_get_endpoint = """    get:
      operationId: ListStudFarmNotes
      summary: ListStudFarmNotes
      description: 'Exposure BO_AUTH. Actor: admin+ADMIN_BO. Requires role=admin and session clientContext=ADMIN_BO.'
      tags:
      - StudFarms
      x-function-id: STUD-FARM-ADMIN-05
      x-exposure: BO_AUTH
      x-actor: admin+ADMIN_BO
      parameters:
      - name: studFarmId
        in: path
        required: true
        schema:
          type: string
          format: uuid
      responses:
        '200':
          description: Success
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/StudFarmNoteListResponse"
        '400':
          $ref: "#/components/responses/BadRequest"
        '401':
          $ref: "#/components/responses/Unauthorized"
        '403':
          $ref: "#/components/responses/Forbidden"
        '404':
          $ref: "#/components/responses/NotFound"
        '500':
          $ref: "#/components/responses/InternalError"
      security:
      - BearerAuth: []
      x-error-codes:
        '400':
        - VALIDATION_ERROR
        '401':
        - UNAUTHENTICATED
        - SESSION_REVOKED
        '403':
        - FORBIDDEN
        - ACCOUNT_INACTIVE
        '404':
        - NOT_FOUND
        '500':
        - INTERNAL_ERROR
"""

if 'operationId: ListStudFarmNotes' not in "".join(lines):
    # Insert after "/v1/stud-farms/{studFarmId}/notes":
    index = -1
    for i, line in enumerate(lines):
        if line.startswith('  "/v1/stud-farms/{studFarmId}/notes":'):
            index = i + 1
            break
    
    if index != -1:
        lines.insert(index, new_get_endpoint)
        with open(filepath, 'w') as f:
            f.writelines(lines)
            print("Successfully patched openapi.yaml with GET endpoint")
else:
    print("GET endpoint already patched")

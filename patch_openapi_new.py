import sys
filepath = '/Users/admin/Desktop/projects/haradan-be/api/openapi.yaml'
with open(filepath, 'r') as f:
    lines = f.readlines()

delete_endpoint = """  "/v1/stud-farms/{studFarmId}":
    delete:
      operationId: DeleteStudFarm
      summary: DeleteStudFarm
      description: 'Exposure BO_AUTH. Actor: admin+ADMIN_BO. Requires role=admin and session clientContext=ADMIN_BO.'
      tags:
      - StudFarms
      x-function-id: STUD-FARM-ADMIN-03
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
        '204':
          description: No Content
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

if '"/v1/stud-farms/{studFarmId}":' not in "".join(lines):
    # Find the line for "/v1/stud-farms":
    index = -1
    for i, line in enumerate(lines):
        if line.startswith('  "/v1/stud-farms":'):
            index = i
            break
    
    if index != -1:
        lines.insert(index, delete_endpoint)
        with open(filepath, 'w') as f:
            f.writelines(lines)
            print("Successfully patched openapi.yaml")
else:
    print("Already patched")

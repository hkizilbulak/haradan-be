import sys
filepath = '/Users/admin/Desktop/projects/haradan-be/api/openapi.yaml'
with open(filepath, 'r') as f:
    content = f.read()

target = '''  "/v1/stud-farms":
    get:'''

delete_endpoint = '''  "/v1/stud-farms/{studFarmId}":
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
  "/v1/stud-farms":
    get:'''

if '"/v1/stud-farms/{studFarmId}":' not in content:
    content = content.replace(target, delete_endpoint)
    with open(filepath, 'w') as f:
        f.write(content)
        print("Updated openapi.yaml")
else:
    print("Delete endpoint already in openapi.yaml")

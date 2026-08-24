import sys
filepath = '/Users/admin/Desktop/projects/haradan-be/api/openapi.yaml'
with open(filepath, 'r') as f:
    lines = f.readlines()

new_schemas = """    StudFarmNoteResponse:
      type: object
      additionalProperties: false
      required:
      - id
      - interview_date
      - interviewer_name
      - notes
      - created_at
      properties:
        id:
          type: string
          format: uuid
        interview_date:
          type: string
          format: date-time
        interviewer_name:
          type: string
        notes:
          type: string
        created_at:
          type: string
          format: date-time
    StudFarmNoteListResponse:
      type: object
      additionalProperties: false
      required:
      - items
      properties:
        items:
          type: array
          items:
            $ref: "#/components/schemas/StudFarmNoteResponse"
"""

if 'StudFarmNoteResponse:' not in "".join(lines):
    # Insert before StudFarmNoteCreateRequest:
    index = -1
    for i, line in enumerate(lines):
        if line.startswith('    StudFarmNoteCreateRequest:'):
            index = i
            break
    
    if index != -1:
        lines.insert(index, new_schemas)
        with open(filepath, 'w') as f:
            f.writelines(lines)
            print("Successfully patched openapi.yaml with schemas")
else:
    print("Schemas already patched")

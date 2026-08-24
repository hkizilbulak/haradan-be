import sys
filepath = '/Users/admin/Desktop/projects/haradan-be/api/openapi.yaml'
with open(filepath, 'r') as f:
    lines = f.readlines()

new_schema = """    StudFarmNoteCreateRequest:
      type: object
      additionalProperties: false
      required:
      - interview_date
      - interviewer_name
      - notes
      properties:
        interview_date:
          type: string
          format: date-time
        interviewer_name:
          type: string
          minLength: 1
          maxLength: 100
        notes:
          type: string
          minLength: 1
"""

if 'StudFarmNoteCreateRequest:' not in "".join(lines):
    # Insert before StudFarmCreateRequest:
    index = -1
    for i, line in enumerate(lines):
        if line.startswith('    StudFarmCreateRequest:'):
            index = i
            break
    
    if index != -1:
        lines.insert(index, new_schema)
        with open(filepath, 'w') as f:
            f.writelines(lines)
            print("Successfully patched openapi.yaml with schema")
else:
    print("Schema already patched")

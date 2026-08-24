import sys
filepath = '/Users/admin/Desktop/projects/haradan-be/api/openapi.yaml'
with open(filepath, 'r') as f:
    content = f.read()

old_str = """        interview_notes_url:
          type: string
          nullable: true"""
new_str = """        interview_notes_url:
          type: string
          nullable: true
        interview_count:
          type: integer"""

if old_str in content and 'interview_count:' not in content:
    content = content.replace(old_str, new_str)
    with open(filepath, 'w') as f:
        f.write(content)
        print("Successfully patched openapi.yaml with interview_count")
else:
    print("Failed or already patched")

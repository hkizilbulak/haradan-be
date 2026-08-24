import sys
filepath = '/Users/admin/Desktop/projects/haradan-be/internal/domain/studfarm/studfarm.go'
with open(filepath, 'r') as f:
    content = f.read()

old_str = """	InterviewerName     *string
	InterviewNotesURL   *string"""
new_str = """	InterviewerName     *string
	InterviewNotesURL   *string
	InterviewCount      int"""

if old_str in content and 'InterviewCount' not in content:
    content = content.replace(old_str, new_str)
    with open(filepath, 'w') as f:
        f.write(content)
        print("Successfully patched studfarm.go with InterviewCount")
else:
    print("Failed or already patched")

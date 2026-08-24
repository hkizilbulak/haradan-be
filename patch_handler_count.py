import sys
filepath = '/Users/admin/Desktop/projects/haradan-be/internal/transport/http/handler/studfarm/handler.go'
with open(filepath, 'r') as f:
    content = f.read()

old_str = """			LatestInterviewDate: item.LatestInterviewDate,
			InterviewerName:     item.InterviewerName,
			InterviewNotesUrl:   item.InterviewNotesURL,
		}"""

new_str = """			LatestInterviewDate: item.LatestInterviewDate,
			InterviewerName:     item.InterviewerName,
			InterviewNotesUrl:   item.InterviewNotesURL,
			InterviewCount:      &item.InterviewCount,
		}"""

if old_str in content:
    content = content.replace(old_str, new_str)
    with open(filepath, 'w') as f:
        f.write(content)
        print("Successfully patched handler.go with InterviewCount")
else:
    print("Failed or already patched handler")

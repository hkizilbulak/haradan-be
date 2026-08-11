package main

import "testing"

func TestMergeEnvironmentProcessValuesWin(t *testing.T) {
	merged := mergeEnvironment(
		map[string]string{"DATABASE_URL": "from-file", "FILE_ONLY": "present"},
		[]string{"DATABASE_URL=from-process", "PROCESS_ONLY=present"},
		false,
	)

	if got := merged["DATABASE_URL"].value; got != "from-process" {
		t.Fatalf("DATABASE_URL precedence = %q, want process value", got)
	}
	if got := merged["FILE_ONLY"].value; got != "present" {
		t.Fatalf("FILE_ONLY = %q, want file value", got)
	}
	if got := merged["PROCESS_ONLY"].value; got != "present" {
		t.Fatalf("PROCESS_ONLY = %q, want process value", got)
	}
}

func TestMergeEnvironmentWindowsNamesAreCaseInsensitive(t *testing.T) {
	merged := mergeEnvironment(
		map[string]string{"Path": "from-file"},
		[]string{"PATH=from-process"},
		true,
	)

	if len(merged) != 1 {
		t.Fatalf("merged environment length = %d, want 1", len(merged))
	}
	if got := merged["PATH"].value; got != "from-process" {
		t.Fatalf("PATH precedence = %q, want process value", got)
	}
}

func TestSetLocalAPIAddress(t *testing.T) {
	tests := []struct {
		name       string
		initial    map[string]environmentValue
		processEnv []string
		expected   string
	}{
		{name: "missing", initial: map[string]environmentValue{}, expected: localAPIAddress},
		{name: "file value", initial: map[string]environmentValue{"HTTP_ADDR": {name: "HTTP_ADDR", value: ":3001"}}, expected: localAPIAddress},
		{name: "blank process value", initial: map[string]environmentValue{"HTTP_ADDR": {name: "HTTP_ADDR", value: ""}}, processEnv: []string{"HTTP_ADDR="}, expected: localAPIAddress},
		{name: "process value", initial: map[string]environmentValue{"HTTP_ADDR": {name: "HTTP_ADDR", value: ":4000"}}, processEnv: []string{"HTTP_ADDR=:4000"}, expected: ":4000"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setLocalAPIAddress(test.initial, test.processEnv, false)
			if got := test.initial["HTTP_ADDR"].value; got != test.expected {
				t.Fatalf("HTTP_ADDR = %q, want %q", got, test.expected)
			}
		})
	}
}

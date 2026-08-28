package config

import "testing"

func TestParseBytes(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  int64
	}{
		{name: "plain bytes", value: "1024", want: 1024},
		{name: "megabytes", value: "2MB", want: 2 * 1024 * 1024},
		{name: "gibibytes", value: "1GiB", want: 1024 * 1024 * 1024},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseBytes(test.value)
			if err != nil {
				t.Fatalf("parseBytes(%q): %v", test.value, err)
			}
			if got != test.want {
				t.Fatalf("parseBytes(%q) = %d, want %d", test.value, got, test.want)
			}
		})
	}
}

func TestParseBytesRejectsInvalidInput(t *testing.T) {
	if _, err := parseBytes("not-a-size"); err == nil {
		t.Fatal("parseBytes accepted invalid input")
	}
}

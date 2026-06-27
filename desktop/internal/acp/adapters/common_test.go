package adapters

import "testing"

func TestSanitizeACPText(t *testing.T) {
	cases := []struct {
		input    string
		want     string
		wantOK   bool
	}{
		{"turn_startedturn_starteditem_started你好！item_completedturn_completed", "你好！", true},
		{"turn_startedturn_starteditem_started你好！我是 Claude，一个 AI 助手。有什么我可以帮你的吗？item_completedturn_completed", "你好！我是 Claude，一个 AI 助手。有什么我可以帮你的吗？", true},
		{"turn_started", "", false},
		{"item_starteditem_completed", "", false},
		{"hello world", "hello world", true},
		{"  turn_started  hello  turn_completed  ", "hello", true},
		{"", "", false},
	}

	for _, c := range cases {
		got, ok := sanitizeACPText(c.input)
		if ok != c.wantOK {
			t.Fatalf("sanitizeACPText(%q) ok=%v, want %v", c.input, ok, c.wantOK)
		}
		if got != c.want {
			t.Fatalf("sanitizeACPText(%q)=%q, want %q", c.input, got, c.want)
		}
	}
}

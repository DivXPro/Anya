package appupdate

import "testing"

func TestIsNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
		wantErr         bool
	}{
		{"v1.2.4", "1.2.3", true, false},
		{"v1.2.3", "v1.2.3", false, false},
		{"v1.2.3", "1.3.0", false, false},
		{"v2.0.0", "1.9.9", true, false},
		{"v1.2.3", "dev", false, false},
		{"v1.2.3-beta", "1.2.2", true, false},
		{"garbage", "1.2.3", false, true},
	}
	for _, c := range cases {
		got, err := IsNewer(c.latest, c.current)
		if (err != nil) != c.wantErr {
			t.Fatalf("IsNewer(%q,%q) err=%v wantErr=%v", c.latest, c.current, err, c.wantErr)
		}
		if got != c.want {
			t.Errorf("IsNewer(%q,%q)=%v want %v", c.latest, c.current, got, c.want)
		}
	}
}

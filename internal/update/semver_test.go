package update

import "testing"

func TestNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v1.5.0", "v1.4.2", true},
		{"1.5.0", "v1.4.2", true},
		{"v1.5.0", "1.4.2", true},
		{"v2.0.0", "v1.99.99", true},
		{"v1.10.0", "v1.9.0", true},
		{"v1.4.3", "v1.4.2", true},
		{"v1.4.2", "v1.4.2", false},
		{"v1.4.2", "v1.5.0", false},
		{"dev", "v1.0.0", false},
		{"v1.5.0", "dev", false},
		{"garbage", "v1.0.0", false},
		{"v1.0.0", "garbage", false},
		{"v1.5", "v1.4.2", false},
		{"v1.5.0.1", "v1.4.2", false},
		{"", "v1.4.2", false},
		{"v1.5.0", "", false},
	}
	for _, c := range cases {
		if got := Newer(c.latest, c.current); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

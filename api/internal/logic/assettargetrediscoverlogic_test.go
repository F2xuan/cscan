package logic

import "testing"

func TestTokensForTarget(t *testing.T) {
	testCases := []struct {
		name      string
		rawTarget string
		tType     string
		tValue    string
		expected  []string
	}{
		{"exact domain", "example.com", "domain", "example.com", []string{"example.com"}},
		{"subdomain resolves to root", "www.example.com", "domain", "example.com", []string{"www.example.com"}},
		{"url token takes hostname", "https://www.example.com/path", "domain", "example.com", []string{"https://www.example.com/path"}},
		{"mixed targets picks matching", "other.org\nwww.example.com\n8.8.8.8", "domain", "example.com", []string{"www.example.com"}},
		{"ip exact match", "8.8.8.8\nexample.com", "ip", "8.8.8.8", []string{"8.8.8.8"}},
		{"ip no match", "8.8.4.4", "ip", "8.8.8.8", nil},
		{"different domain no match", "notexample.com", "domain", "example.com", nil},
		{"sibling domain no match", "example.com.evil.com", "domain", "example.com", nil},
		{"type mismatch no match", "8.8.8.8", "domain", "example.com", nil},
		{"dedupe tokens", "example.com\nexample.com", "domain", "example.com", []string{"example.com"}},
		{"cidr skipped", "192.168.1.0/24", "ip", "192.168.1.0", nil},
		{"empty target", "", "domain", "example.com", nil},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tokensForTarget(tc.rawTarget, tc.tType, tc.tValue)
			if len(got) != len(tc.expected) {
				t.Fatalf("tokensForTarget(%q, %q, %q) = %v, expected %v", tc.rawTarget, tc.tType, tc.tValue, got, tc.expected)
			}
			for i := range got {
				if got[i] != tc.expected[i] {
					t.Fatalf("tokensForTarget(%q, %q, %q) = %v, expected %v", tc.rawTarget, tc.tType, tc.tValue, got, tc.expected)
				}
			}
		})
	}
}

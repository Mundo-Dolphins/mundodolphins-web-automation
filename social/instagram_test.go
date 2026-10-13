package social

import "testing"

func TestInstagramShortcode(t *testing.T) {
	cases := map[string]string{
		"https://www.instagram.com/p/AbCdEf12345/":             "AbCdEf12345",
		"https://www.instagram.com/reel/XyZ-987/":              "XyZ-987",
		"https://www.instagram.com/some_profile/p/Q1w2E3/":     "Q1w2E3",
		"https://www.instagram.com/p/AbCdEf12345/?img_index=2": "AbCdEf12345",
		"https://www.instagram.com/":                           "",
		"https://example.com/p/notinstagram/":                  "",
	}
	for in, want := range cases {
		if got := InstagramShortcode(in); got != want {
			t.Errorf("InstagramShortcode(%q) = %q, want %q", in, got, want)
		}
	}
}

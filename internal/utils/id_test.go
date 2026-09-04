package utils

import "testing"

func TestValidID(t *testing.T) {
	if !ValidID("11111111-1111-1111-1111-111111111111") {
		t.Fatal("want valid")
	}
	if !ValidID("018f3a2b-9c4d-7e80-8f12-3456789abcde") {
		t.Fatal("want valid uuidv7-shaped")
	}
	for _, in := range []string{"", "not-a-uuid", "111111111111111111111111111111111111", "11111111-1111-1111-1111-11111111111g"} {
		if ValidID(in) {
			t.Errorf("ValidID(%q): want false", in)
		}
	}
}


package cloud

import "testing"

func TestGenerateRandomString_LengthAndUniqueness(t *testing.T) {
	a := generateRandomString(16)
	b := generateRandomString(16)
	if len(a) != 16 || len(b) != 16 {
		t.Fatalf("unexpected length: a=%d b=%d", len(a), len(b))
	}
	if a == b {
		t.Fatalf("expected different random strings, got same value: %s", a)
	}
}


package cloud

import "testing"

func TestGetNextPort_Increments(t *testing.T) {
	m := &CloudManager{portCounter: 5432}
	if p := m.getNextPort(); p != 5433 {
		t.Fatalf("expected first getNextPort to be 5433, got %d", p)
	}
	if p := m.getNextPort(); p != 5434 {
		t.Fatalf("expected second getNextPort to be 5434, got %d", p)
	}
}

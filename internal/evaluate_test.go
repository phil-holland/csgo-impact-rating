package internal

import "testing"

func TestBToF64(t *testing.T) {
	tr := bToF64(true)
	if tr != 1.0 {
		t.Errorf("Got bToF64(%v) = %v, expected bToF64(%v) = %v", true, tr, true, 1.0)
	}

	fa := bToF64(false)
	if fa != 0.0 {
		t.Errorf("Got bToF64(%v) = %v, expected bToF64(%v) = %v", false, fa, false, 0.0)
	}
}

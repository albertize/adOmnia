package extensions

import "testing"

func TestCheckEngineCompatibility(t *testing.T) {
	for _, test := range []struct {
		version    string
		rangeValue string
		valid      bool
	}{
		{"1.4.2", ">=1.0.0 <2", true},
		{"2.0.0", ">=1.0.0 <2", false},
		{"1.4.2", "^1.2.0", true},
		{"2.0.0", "^1.2.0", false},
		{"0.2.5", "^0.2.0", true},
		{"0.3.0", "^0.2.0", false},
		{"1.2.9", "~1.2.0", true},
		{"1.3.0", "~1.2.0", false},
		{"dev", ">=99.0.0", true},
	} {
		err := CheckEngineCompatibility(test.version, test.rangeValue)
		if (err == nil) != test.valid {
			t.Errorf("CheckEngineCompatibility(%q, %q) error=%v, valid=%v", test.version, test.rangeValue, err, test.valid)
		}
	}
}

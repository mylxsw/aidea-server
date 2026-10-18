package controllers

import "testing"

func TestStrictPriceFen(t *testing.T) {
	for input, expected := range map[string]int64{"1": 100, "1.2": 120, "1.20": 120, "0.01": 1} {
		got, err := strictPriceFen(input)
		if err != nil || got != expected {
			t.Errorf("strictPriceFen(%q) = %d, %v", input, got, err)
		}
	}
	for _, input := range []string{"1.234", "-1", "1e2", "", "1."} {
		if _, err := strictPriceFen(input); err == nil {
			t.Errorf("accepted invalid amount %q", input)
		}
	}
}

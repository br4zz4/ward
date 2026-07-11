package cmd

import "testing"

func TestTruncateValue_empty(t *testing.T) {
	// arrange + act
	got := truncateValue("", 10)

	// assert
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestTruncateValue_short_unchanged(t *testing.T) {
	// arrange
	val := "abc123"

	// act
	got := truncateValue(val, 10)

	// assert
	if got != val {
		t.Errorf("expected %q unchanged, got %q", val, got)
	}
}

func TestTruncateValue_exact_limit_unchanged(t *testing.T) {
	// arrange
	val := "1234567890" // exactly 10 chars

	// act
	got := truncateValue(val, 10)

	// assert
	if got != val {
		t.Errorf("expected %q unchanged at exact limit, got %q", val, got)
	}
}

func TestTruncateValue_over_limit_truncated(t *testing.T) {
	// arrange
	val := "12345678901" // 11 chars, over limit of 10

	// act
	got := truncateValue(val, 10)

	// assert
	if got != "1234567890…" {
		t.Errorf("expected truncated with ellipsis, got %q", got)
	}
}

func TestTruncateValue_long_value_truncated_at_120(t *testing.T) {
	// arrange: 200 ASCII chars
	val := ""
	for i := 0; i < 200; i++ {
		val += "x"
	}

	// act
	got := truncateValue(val, 120)

	// assert: 120 x's + ellipsis rune = 121 runes
	if len([]rune(got)) != 121 {
		t.Errorf("expected 121 runes (120 + ellipsis), got %d: %q", len([]rune(got)), got)
	}
	if got[len(got)-3:] != "…" {
		t.Errorf("expected ellipsis at end, got %q", got)
	}
}

func TestTruncateValue_visible_len_after_truncation(t *testing.T) {
	// arrange: value longer than cap — visibleLen counts bytes, "…" is 3 bytes
	val := "abcdefghij_extra" // 16 chars, cap = 10

	// act
	got := truncateValue(val, 10)

	// assert: 10 ASCII bytes + 3 bytes for "…" = 13 visible bytes
	vl := visibleLen(got)
	if vl != 13 {
		t.Errorf("expected visible len 13, got %d: %q", vl, got)
	}
}

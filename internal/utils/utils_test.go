package utils

import (
	"math"
	"testing"
	"time"
)

func TestConvertFromStrToInt32(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int32
		wantErr bool
	}{
		{"valid positive", "42", 42, false},
		{"valid zero", "0", 0, false},
		{"valid negative", "-1", -1, false},
		{"min int32", "-2147483648", -2147483648, false},
		{"max int32", "2147483647", 2147483647, false},
		{"empty string", "", 0, true},
		{"non-numeric", "abc", 0, true},
		{"float", "3.14", 0, true},
		{"overflow int32", "2147483648", 0, true},
		{"underflow int32", "-2147483649", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertFromStrToInt32(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertFromStrToInt32(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ConvertFromStrToInt32(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestConvertFromStrToInt32_MaxBoundary(t *testing.T) {
	maxStr := "2147483647"
	got, err := ConvertFromStrToInt32(maxStr)
	if err != nil {
		t.Fatalf("unexpected error for max int32: %v", err)
	}
	if got != math.MaxInt32 {
		t.Errorf("got %d, want %d", got, math.MaxInt32)
	}
}

func TestConvertFromStrToInt32_MinBoundary(t *testing.T) {
	minStr := "-2147483648"
	got, err := ConvertFromStrToInt32(minStr)
	if err != nil {
		t.Fatalf("unexpected error for min int32: %v", err)
	}
	if got != math.MinInt32 {
		t.Errorf("got %d, want %d", got, math.MinInt32)
	}
}

func TestGetEnv_FallbackWhenNotSet(t *testing.T) {
	result := GetEnv("DEFINITELY_NOT_SET_VAR_XYZ", "fallback")
	if result != "fallback" {
		t.Errorf("GetEnv = %q, want %q", result, "fallback")
	}
}

func TestGetEnv_ReturnsValueWhenSet(t *testing.T) {
	t.Setenv("TEST_GET_ENV_VAR", "hello")
	result := GetEnv("TEST_GET_ENV_VAR", "fallback")
	if result != "hello" {
		t.Errorf("GetEnv = %q, want %q", result, "hello")
	}
}

func TestConvertFromStrToPGTypeDate(t *testing.T) {
	t.Run("empty string produces invalid date", func(t *testing.T) {
		result := ConvertFromStrToPGTypeDate("")
		if result.Valid {
			t.Error("expected Valid=false for empty string")
		}
	})

	t.Run("valid date string produces correct date", func(t *testing.T) {
		result := ConvertFromStrToPGTypeDate("2025-06-15")
		if !result.Valid {
			t.Fatal("expected Valid=true for valid date")
		}
		if result.Time.Year() != 2025 || result.Time.Month() != time.June || result.Time.Day() != 15 {
			t.Errorf("date = %v, want 2025-06-15", result.Time)
		}
	})

	t.Run("invalid date format sets Valid=true but zero time", func(t *testing.T) {
		result := ConvertFromStrToPGTypeDate("not-a-date")
		if !result.Valid {
			t.Error("expected Valid=true for non-empty string regardless of format")
		}
		if !result.Time.IsZero() {
			t.Error("expected zero time for unparseable date")
		}
	})
}

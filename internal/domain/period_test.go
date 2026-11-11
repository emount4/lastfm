package domain

import (
	"errors"
	"testing"
	"time"
)

func TestPeriodRange(t *testing.T) {
	msk := time.FixedZone("MSK", 3*60*60)
	now := time.Date(2024, time.March, 31, 18, 30, 15, 500, msk)
	leapDay := time.Date(2024, time.February, 29, 18, 30, 15, 0, msk)
	for _, tt := range []struct {
		input string
		now   time.Time
		from  time.Time
	}{
		{input: " 10D ", now: now, from: now.Add(-10 * 24 * time.Hour)},
		{input: "2w", now: now, from: now.Add(-14 * 24 * time.Hour)},
		{input: "1m", now: now, from: leapDay},
		{input: "1y", now: leapDay, from: time.Date(2023, time.February, 28, 18, 30, 15, 0, msk)},
	} {
		t.Run(tt.input, func(t *testing.T) {
			period, err := ParsePeriod(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			got, err := period.Range(tt.now)
			want := ScrobbleRange{From: tt.from.Unix(), To: tt.now.Unix()}
			if err != nil || got != want {
				t.Fatalf("range = %+v, error = %v, want %+v", got, err, want)
			}
		})
	}
}

func TestParsePeriodRejectsInvalidInputs(t *testing.T) {
	for _, input := range []string{"0w", "-1d", "+2d", "1.5m", "2 w", "4h", "1w 2d", "999999999999999999999d"} {
		if _, err := ParsePeriod(input); !errors.Is(err, ErrInvalidPeriod) {
			t.Errorf("ParsePeriod(%q) = %v, want ErrInvalidPeriod", input, err)
		}
	}
	period, err := ParsePeriod("999999999999y")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := period.Range(time.Date(2026, time.November, 11, 0, 0, 0, 0, time.UTC)); !errors.Is(err, ErrInvalidPeriod) {
		t.Fatalf("range error = %v, want ErrInvalidPeriod", err)
	}
}

package clock

import (
	"testing"
	"time"
)

func TestGameDateUsesJST(t *testing.T) {
	tests := []struct {
		name string
		in   time.Time
		want string
	}{
		{
			name: "just before JST midnight stays on the current day",
			in:   time.Date(2026, 8, 16, 23, 59, 59, 0, JST),
			want: "2026-08-16",
		},
		{
			name: "exactly JST midnight starts the new day",
			in:   time.Date(2026, 8, 17, 0, 0, 0, 0, JST),
			want: "2026-08-17",
		},
		{
			name: "UTC 15:00 is already the next JST day",
			in:   time.Date(2026, 8, 16, 15, 0, 0, 0, time.UTC),
			want: "2026-08-17",
		},
		{
			name: "UTC 14:59 is still the same JST day",
			in:   time.Date(2026, 8, 16, 14, 59, 59, 0, time.UTC),
			want: "2026-08-16",
		},
		{
			name: "a non-JST zone is normalised, not truncated locally",
			in:   time.Date(2026, 8, 16, 20, 0, 0, 0, time.FixedZone("UTC-5", -5*3600)),
			want: "2026-08-17",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatGameDate(GameDate(tc.in))
			if got != tc.want {
				t.Fatalf("GameDate(%s) = %s, want %s", tc.in, got, tc.want)
			}
		})
	}
}

func TestGameDateIsMidnightJST(t *testing.T) {
	d := GameDate(time.Date(2026, 8, 16, 13, 45, 12, 999, time.UTC))
	h, m, s := d.Clock()
	if h != 0 || m != 0 || s != 0 {
		t.Fatalf("game date is not midnight: %s", d)
	}
	if _, offset := d.Zone(); offset != 9*3600 {
		t.Fatalf("game date offset = %d, want 32400", offset)
	}
}

func TestDayEndAndRemaining(t *testing.T) {
	now := time.Date(2026, 8, 16, 20, 17, 42, 0, JST)

	end := DayEnd(now)
	want := time.Date(2026, 8, 17, 0, 0, 0, 0, JST)
	if !end.Equal(want) {
		t.Fatalf("DayEnd = %s, want %s", end, want)
	}

	if got := Remaining(now); got != 3*time.Hour+42*time.Minute+18*time.Second {
		t.Fatalf("Remaining = %s, want 03:42:18", got)
	}
}

func TestDayEndAtMidnightIsFullDayLater(t *testing.T) {
	midnight := time.Date(2026, 8, 17, 0, 0, 0, 0, JST)
	if got := Remaining(midnight); got != 24*time.Hour {
		t.Fatalf("Remaining at midnight = %s, want 24h", got)
	}
}

func TestFakeClock(t *testing.T) {
	c := NewFakeJST(2026, time.August, 16, 23, 59, 0)
	if got := FormatGameDate(Today(c)); got != "2026-08-16" {
		t.Fatalf("Today = %s, want 2026-08-16", got)
	}

	c.Advance(time.Minute)
	if got := FormatGameDate(Today(c)); got != "2026-08-17" {
		t.Fatalf("Today after advance = %s, want 2026-08-17", got)
	}
}

func TestRealClockImplementsClock(t *testing.T) {
	var c Clock = Real{}
	if c.Now().IsZero() {
		t.Fatal("Real.Now returned zero time")
	}
}

package service

import (
	"testing"
	"time"

	"meetroom/internal/model"
)

func ptrInt(i int) *int       { return &i }
func ptrStr(s string) *string { return &s }

func TestParseTime(t *testing.T) {
	tests := []struct {
		input   string
		h, m    int
		wantErr bool
	}{
		{"09:00", 9, 0, false},
		{"18:00", 18, 0, false},
		{"00:00", 0, 0, false},
		{"23:59", 23, 59, false},
		{"25:99", 0, 0, true},
		{"9:00", 9, 0, false},
		{"09:60", 0, 0, true},
		{"abc", 0, 0, true},
		{"", 0, 0, true},
	}
	for _, tt := range tests {
		result, err := parseTime(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseTime(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if tt.wantErr {
			continue
		}
		if result.h != tt.h || result.m != tt.m {
			t.Errorf("parseTime(%q) = %d:%d, want %d:%d", tt.input, result.h, result.m, tt.h, tt.m)
		}
	}
}

func TestIsSameDay(t *testing.T) {
	day1 := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 7, 28, 23, 59, 0, 0, time.UTC)
	day3 := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)

	if !isSameDay(day1, day2) {
		t.Error("same day should return true")
	}
	if isSameDay(day1, day3) {
		t.Error("different days should return false")
	}
}

func TestIsWithinAvailability_SpecificDate(t *testing.T) {
	avails := []model.Availability{
		{
			SpecificDate: ptrStr("2026-07-28"),
			StartTime:    "09:00",
			EndTime:      "18:00",
		},
	}

	base := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		start   time.Time
		end     time.Time
		wantOK  bool
		wantErr bool
	}{
		{"within", base.Add(9 * time.Hour), base.Add(10 * time.Hour), true, false},
		{"exactly matches", base.Add(9 * time.Hour), base.Add(18 * time.Hour), true, false},
		{"starts before", base.Add(8 * time.Hour), base.Add(10 * time.Hour), false, false},
		{"ends after", base.Add(10 * time.Hour), base.Add(19 * time.Hour), false, false},
		{"fully outside", base.Add(19 * time.Hour), base.Add(20 * time.Hour), false, false},
		{"different day", base.AddDate(0, 0, 1).Add(10 * time.Hour), base.AddDate(0, 0, 1).Add(11 * time.Hour), false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := isWithinAvailability(tt.start, tt.end, avails)
			if (err != nil) != tt.wantErr {
				t.Errorf("isWithinAvailability() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if ok != tt.wantOK {
				t.Errorf("isWithinAvailability() = %v, want %v", ok, tt.wantOK)
			}
		})
	}
}

func TestIsWithinAvailability_Recurring(t *testing.T) {
	monday := 1
	avails := []model.Availability{
		{
			DayOfWeek: &monday,
			StartTime: "09:00",
			EndTime:   "18:00",
		},
	}

	mondayDate := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)
	tuesdayDate := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		start  time.Time
		end    time.Time
		wantOK bool
	}{
		{"monday within", mondayDate.Add(9 * time.Hour), mondayDate.Add(10 * time.Hour), true},
		{"tuesday no avail", tuesdayDate.Add(9 * time.Hour), tuesdayDate.Add(10 * time.Hour), false},
		{"monday cross-day", mondayDate.Add(8 * time.Hour), mondayDate.Add(19 * time.Hour), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := isWithinAvailability(tt.start, tt.end, avails)
			if err != nil {
				t.Fatal(err)
			}
			if ok != tt.wantOK {
				t.Errorf("isWithinAvailability() = %v, want %v", ok, tt.wantOK)
			}
		})
	}
}

func TestIsWithinAvailability_MultiDay(t *testing.T) {
	mon, tue, wed := 1, 2, 3
	avails := []model.Availability{
		{DayOfWeek: &mon, StartTime: "09:00", EndTime: "18:00"},
		{DayOfWeek: &tue, StartTime: "09:00", EndTime: "18:00"},
		{DayOfWeek: &wed, StartTime: "09:00", EndTime: "18:00"},
	}

	monDate := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)
	tueDate := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	wedDate := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	thuDate := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)

	t.Run("continuous multi-day should fail", func(t *testing.T) {
		ok, err := isWithinAvailability(
			monDate.Add(10*time.Hour),
			wedDate.Add(15*time.Hour),
			avails,
		)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Error("continuous multi-day booking spanning nights should fail")
		}
	})

	t.Run("spanning day without availability should fail", func(t *testing.T) {
		ok, err := isWithinAvailability(
			monDate.Add(10*time.Hour),
			thuDate.Add(15*time.Hour),
			avails,
		)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Error("booking spanning a day without availability should fail")
		}
	})

	t.Run("single day within availability should pass", func(t *testing.T) {
		ok, err := isWithinAvailability(
			monDate.Add(10*time.Hour),
			monDate.Add(12*time.Hour),
			avails,
		)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Error("single day booking within availability should pass")
		}
	})

	t.Run("start at availability open should pass", func(t *testing.T) {
		ok, err := isWithinAvailability(
			monDate.Add(9*time.Hour),
			monDate.Add(10*time.Hour),
			avails,
		)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Error("booking starting at availability open should pass")
		}
	})

	t.Run("end at availability close should pass", func(t *testing.T) {
		ok, err := isWithinAvailability(
			monDate.Add(17*time.Hour),
			monDate.Add(18*time.Hour),
			avails,
		)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Error("booking ending at availability close should pass")
		}
	})

	t.Run("back to back same day should pass", func(t *testing.T) {
		ok, err := isWithinAvailability(
			tueDate.Add(10*time.Hour),
			tueDate.Add(12*time.Hour),
			avails,
		)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Error("back to back booking within same day availability should pass")
		}
	})
}

func TestSubtractBookings(t *testing.T) {
	base := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	availStart := base.Add(9 * time.Hour)
	availEnd := base.Add(18 * time.Hour)

	tests := []struct {
		name     string
		bookings []model.Booking
		want     int
	}{
		{
			"no bookings",
			nil,
			1,
		},
		{
			"single booking in middle",
			[]model.Booking{
				{StartTime: base.Add(12 * time.Hour), EndTime: base.Add(13 * time.Hour)},
			},
			2,
		},
		{
			"back-to-back bookings",
			[]model.Booking{
				{StartTime: base.Add(10 * time.Hour), EndTime: base.Add(12 * time.Hour)},
				{StartTime: base.Add(12 * time.Hour), EndTime: base.Add(14 * time.Hour)},
			},
			2,
		},
		{
			"booking at start boundary",
			[]model.Booking{
				{StartTime: availStart, EndTime: base.Add(12 * time.Hour)},
			},
			1,
		},
		{
			"booking at end boundary",
			[]model.Booking{
				{StartTime: base.Add(12 * time.Hour), EndTime: availEnd},
			},
			1,
		},
		{
			"booking covering entire range",
			[]model.Booking{
				{StartTime: availStart, EndTime: availEnd},
			},
			0,
		},
		{
			"multiple non-overlapping",
			[]model.Booking{
				{StartTime: base.Add(10 * time.Hour), EndTime: base.Add(11 * time.Hour)},
				{StartTime: base.Add(12 * time.Hour), EndTime: base.Add(13 * time.Hour)},
				{StartTime: base.Add(14 * time.Hour), EndTime: base.Add(15 * time.Hour)},
			},
			4,
		},
		{
			"booking partially outside (start before avail)",
			[]model.Booking{
				{StartTime: base.Add(8 * time.Hour), EndTime: base.Add(10 * time.Hour)},
			},
			1,
		},
		{
			"booking partially outside (end after avail)",
			[]model.Booking{
				{StartTime: base.Add(17 * time.Hour), EndTime: base.Add(19 * time.Hour)},
			},
			1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := subtractBookings(availStart, availEnd, tt.bookings)
			if len(got) != tt.want {
				t.Errorf("subtractBookings() returned %d slots, want %d", len(got), tt.want)
				for i, s := range got {
					t.Logf("  slot %d: %s - %s", i, s.Start.Format(time.RFC3339), s.End.Format(time.RFC3339))
				}
			}
		})
	}
}

func TestCalculateFreeSlots(t *testing.T) {
	mon := 1
	base := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)

	avails := []model.Availability{
		{DayOfWeek: &mon, StartTime: "09:00", EndTime: "18:00"},
	}

	bookings := []model.Booking{
		{StartTime: base.Add(12 * time.Hour), EndTime: base.Add(13 * time.Hour)},
	}

	from := base.Add(8 * time.Hour)
	to := base.Add(20 * time.Hour)

	slots := calculateFreeSlots(from, to, avails, bookings)

	if len(slots) != 2 {
		t.Errorf("expected 2 free slots, got %d", len(slots))
		for i, s := range slots {
			t.Logf("slot %d: %s - %s", i, s.Start.Format(time.RFC3339), s.End.Format(time.RFC3339))
		}
	}
}

func TestCalculateFreeSlots_BackToBack(t *testing.T) {
	mon := 1
	base := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)

	avails := []model.Availability{
		{DayOfWeek: &mon, StartTime: "09:00", EndTime: "18:00"},
	}

	bookings := []model.Booking{
		{StartTime: base.Add(10 * time.Hour), EndTime: base.Add(12 * time.Hour)},
		{StartTime: base.Add(12 * time.Hour), EndTime: base.Add(14 * time.Hour)},
	}

	slots := calculateFreeSlots(base.Add(9*time.Hour), base.Add(18*time.Hour), avails, bookings)

	if len(slots) != 2 {
		t.Errorf("expected 2 free slots (before and after back-to-back bookings), got %d", len(slots))
	}
}

func TestCalculateFreeSlots_NoAvailability(t *testing.T) {
	tue := 2
	base := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)

	avails := []model.Availability{
		{DayOfWeek: &tue, StartTime: "09:00", EndTime: "18:00"},
	}

	slots := calculateFreeSlots(base.Add(9*time.Hour), base.Add(18*time.Hour), avails, nil)

	if len(slots) == 0 {
		t.Error("expected free slots for available day")
	}
}

func TestCalculateFreeSlots_EdgeCases(t *testing.T) {
	t.Run("zero length availability", func(t *testing.T) {
		mon := 1
		base := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)
		avails := []model.Availability{
			{DayOfWeek: &mon, StartTime: "09:00", EndTime: "09:00"},
		}
		slots := calculateFreeSlots(base.Add(9*time.Hour), base.Add(18*time.Hour), avails, nil)
		if len(slots) != 0 {
			t.Error("zero-length availability should produce no slots")
		}
	})

	t.Run("booking at exact boundary", func(t *testing.T) {
		mon := 1
		base := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)
		avails := []model.Availability{
			{DayOfWeek: &mon, StartTime: "09:00", EndTime: "18:00"},
		}
		bookings := []model.Booking{
			{StartTime: base.Add(10 * time.Hour), EndTime: base.Add(10 * time.Hour)},
		}
		slots := calculateFreeSlots(base.Add(9*time.Hour), base.Add(18*time.Hour), avails, bookings)
		if len(slots) != 1 {
			t.Errorf("zero-length booking should be ignored, got %d slots", len(slots))
		}
	})
}

func TestFilterBookingsForDay(t *testing.T) {
	base := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	dayStart := base
	dayEnd := base.AddDate(0, 0, 1)

	bookings := []model.Booking{
		{StartTime: base.Add(10 * time.Hour), EndTime: base.Add(12 * time.Hour)},
		{StartTime: base.AddDate(0, 0, 1).Add(10 * time.Hour), EndTime: base.AddDate(0, 0, 1).Add(12 * time.Hour)},
	}

	filtered := filterBookingsForDay(bookings, dayStart, dayEnd)
	if len(filtered) != 1 {
		t.Errorf("expected 1 booking for the day, got %d", len(filtered))
	}
}

func TestMaxTime(t *testing.T) {
	a := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	b := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

	if !maxTime(a, b).Equal(b) {
		t.Error("maxTime should return b")
	}
	if !maxTime(b, a).Equal(b) {
		t.Error("maxTime should return b")
	}
}

func TestMinTime(t *testing.T) {
	a := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	b := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

	if !minTime(a, b).Equal(a) {
		t.Error("minTime should return a")
	}
	if !minTime(b, a).Equal(a) {
		t.Error("minTime should return a")
	}
}

func TestMergeIntervals(t *testing.T) {
	base := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		intervals []TimeSlot
		want      []TimeSlot
	}{
		{
			"empty",
			nil,
			nil,
		},
		{
			"single",
			[]TimeSlot{{base.Add(9 * time.Hour), base.Add(12 * time.Hour)}},
			[]TimeSlot{{base.Add(9 * time.Hour), base.Add(12 * time.Hour)}},
		},
		{
			"disjoint kept separate",
			[]TimeSlot{
				{base.Add(9 * time.Hour), base.Add(12 * time.Hour)},
				{base.Add(14 * time.Hour), base.Add(18 * time.Hour)},
			},
			[]TimeSlot{
				{base.Add(9 * time.Hour), base.Add(12 * time.Hour)},
				{base.Add(14 * time.Hour), base.Add(18 * time.Hour)},
			},
		},
		{
			"back-to-back merged",
			[]TimeSlot{
				{base.Add(9 * time.Hour), base.Add(12 * time.Hour)},
				{base.Add(12 * time.Hour), base.Add(18 * time.Hour)},
			},
			[]TimeSlot{{base.Add(9 * time.Hour), base.Add(18 * time.Hour)}},
		},
		{
			"overlapping merged",
			[]TimeSlot{
				{base.Add(9 * time.Hour), base.Add(13 * time.Hour)},
				{base.Add(12 * time.Hour), base.Add(18 * time.Hour)},
			},
			[]TimeSlot{{base.Add(9 * time.Hour), base.Add(18 * time.Hour)}},
		},
		{
			"unsorted input",
			[]TimeSlot{
				{base.Add(14 * time.Hour), base.Add(18 * time.Hour)},
				{base.Add(9 * time.Hour), base.Add(12 * time.Hour)},
			},
			[]TimeSlot{
				{base.Add(9 * time.Hour), base.Add(12 * time.Hour)},
				{base.Add(14 * time.Hour), base.Add(18 * time.Hour)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeIntervals(tt.intervals)
			if len(got) != len(tt.want) {
				t.Fatalf("mergeIntervals() returned %d intervals, want %d: %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if !got[i].Start.Equal(tt.want[i].Start) || !got[i].End.Equal(tt.want[i].End) {
					t.Errorf("interval %d = %s-%s, want %s-%s",
						i, got[i].Start.Format(time.RFC3339), got[i].End.Format(time.RFC3339),
						tt.want[i].Start.Format(time.RFC3339), tt.want[i].End.Format(time.RFC3339))
				}
			}
		})
	}
}

func TestIsWithinAvailability_AdjacentWindows(t *testing.T) {
	mon := 1
	avails := []model.Availability{
		{DayOfWeek: &mon, StartTime: "09:00", EndTime: "12:00"},
		{DayOfWeek: &mon, StartTime: "12:00", EndTime: "18:00"},
	}

	monDate := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)

	t.Run("booking spanning adjacent windows should pass", func(t *testing.T) {
		ok, err := isWithinAvailability(monDate.Add(10*time.Hour), monDate.Add(15*time.Hour), avails)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Error("booking spanning back-to-back windows should be within availability")
		}
	})
}

func TestIsWithinAvailability_WindowGap(t *testing.T) {
	mon := 1
	avails := []model.Availability{
		{DayOfWeek: &mon, StartTime: "09:00", EndTime: "12:00"},
		{DayOfWeek: &mon, StartTime: "14:00", EndTime: "18:00"},
	}

	monDate := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		start time.Time
		end   time.Time
		want  bool
	}{
		{"inside first window", monDate.Add(10 * time.Hour), monDate.Add(11 * time.Hour), true},
		{"inside second window", monDate.Add(15 * time.Hour), monDate.Add(16 * time.Hour), true},
		{"spanning the gap", monDate.Add(11 * time.Hour), monDate.Add(15 * time.Hour), false},
		{"exactly the gap", monDate.Add(12 * time.Hour), monDate.Add(14 * time.Hour), false},
		{"boundary touching both edges", monDate.Add(12 * time.Hour), monDate.Add(14 * time.Hour), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := isWithinAvailability(tt.start, tt.end, avails)
			if err != nil {
				t.Fatal(err)
			}
			if ok != tt.want {
				t.Errorf("isWithinAvailability() = %v, want %v", ok, tt.want)
			}
		})
	}
}

func TestCalculateFreeSlots_MultipleWindowsWithGap(t *testing.T) {
	mon := 1
	base := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)

	avails := []model.Availability{
		{DayOfWeek: &mon, StartTime: "09:00", EndTime: "12:00"},
		{DayOfWeek: &mon, StartTime: "14:00", EndTime: "18:00"},
	}

	slots := calculateFreeSlots(base.Add(9*time.Hour), base.Add(18*time.Hour), avails, nil)

	if len(slots) != 2 {
		t.Fatalf("expected 2 free slots, got %d", len(slots))
	}
	wantStart := [2]int{9, 14}
	wantEnd := [2]int{12, 18}
	for i, s := range slots {
		if s.Start.Hour() != wantStart[i] || s.End.Hour() != wantEnd[i] {
			t.Errorf("slot %d = %s-%s, want %02d:00-%02d:00",
				i, s.Start.Format(time.RFC3339), s.End.Format(time.RFC3339), wantStart[i], wantEnd[i])
		}
	}
}

func TestValidateAvailabilityTimes(t *testing.T) {
	tests := []struct {
		start, end string
		wantErr    bool
	}{
		{"09:00", "18:00", false},
		{"09:00", "09:00", true},
		{"18:00", "09:00", true},
		{"25:00", "26:00", true},
		{"not-a-time", "18:00", true},
		{"09:60", "18:00", true},
	}
	for _, tt := range tests {
		err := validateAvailabilityTimes(tt.start, tt.end)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateAvailabilityTimes(%q, %q) error = %v, wantErr %v", tt.start, tt.end, err, tt.wantErr)
		}
	}
}

func TestSubtractBookings_UnsortedInput(t *testing.T) {
	base := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	availStart := base.Add(9 * time.Hour)
	availEnd := base.Add(18 * time.Hour)

	bookings := []model.Booking{
		{StartTime: base.Add(14 * time.Hour), EndTime: base.Add(15 * time.Hour)},
		{StartTime: base.Add(10 * time.Hour), EndTime: base.Add(11 * time.Hour)},
		{StartTime: base.Add(12 * time.Hour), EndTime: base.Add(13 * time.Hour)},
	}

	slots := subtractBookings(availStart, availEnd, bookings)

	if len(slots) != 4 {
		t.Errorf("expected 4 free slots for unsorted bookings, got %d", len(slots))
	}
}

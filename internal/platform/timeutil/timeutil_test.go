package timeutil_test

import (
	"testing"
	"time"

	"github.com/overplane/overplane/internal/platform/timeutil"
)

func TestTimeutil(t *testing.T) {
	tm, err := timeutil.Parse("2026-06-09T21:14:03.123456789-07:00")
	if err != nil {
		t.Fatal(err)
	}
	if got := timeutil.Stamp(tm); got != "2026-06-10T04:14:03Z" {
		t.Fatalf("stamp = %s", got)
	}
	if got := timeutil.StampMillis(tm); got != "2026-06-10T04:14:03.123Z" {
		t.Fatalf("stamp millis = %s", got)
	}
	old := timeutil.Default
	timeutil.Default = timeutil.Fixed(tm)
	defer func() { timeutil.Default = old }()
	if !timeutil.NowUTC().Equal(tm.UTC()) {
		t.Fatal("fixed clock not used")
	}
	durations := []time.Duration{
		time.Hour + 12*time.Minute,
		3*time.Second + 200*time.Millisecond,
		415 * time.Millisecond,
	}
	for _, d := range durations {
		if timeutil.HumanDuration(d) == "" {
			t.Fatalf("empty duration for %s", d)
		}
	}
}

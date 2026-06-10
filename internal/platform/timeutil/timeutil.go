package timeutil

import (
	"strconv"
	"time"
)

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func Real() Clock {
	return realClock{}
}

func (realClock) Now() time.Time {
	return time.Now()
}

type fixedClock struct {
	t time.Time
}

func Fixed(t time.Time) Clock {
	return fixedClock{t: t}
}

func (c fixedClock) Now() time.Time {
	return c.t
}

var Default = Real()

func NowUTC() time.Time {
	return Default.Now().UTC()
}

func Stamp(t time.Time) string {
	return t.UTC().Truncate(time.Second).Format(time.RFC3339)
}

func StampMillis(t time.Time) string {
	return t.UTC().Truncate(time.Millisecond).Format("2006-01-02T15:04:05.000Z")
}

func Parse(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}

func HumanDuration(d time.Duration) string {
	if d < 0 {
		return "-" + HumanDuration(-d)
	}
	switch {
	case d >= time.Hour:
		h := d / time.Hour
		m := (d % time.Hour) / time.Minute
		if m == 0 {
			return strconv.FormatInt(int64(h), 10) + "h"
		}
		return strconv.FormatInt(int64(h), 10) + "h" + strconv.FormatInt(int64(m), 10) + "m"
	case d >= time.Minute:
		m := d / time.Minute
		s := (d % time.Minute) / time.Second
		if s == 0 {
			return strconv.FormatInt(int64(m), 10) + "m"
		}
		return strconv.FormatInt(int64(m), 10) + "m" + strconv.FormatInt(int64(s), 10) + "s"
	case d >= time.Second:
		return strconv.FormatFloat(float64(d)/float64(time.Second), 'f', 1, 64) + "s"
	default:
		return strconv.FormatInt(int64(d/time.Millisecond), 10) + "ms"
	}
}

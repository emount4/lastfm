package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidPeriod = errors.New("invalid period")

type Period struct {
	count int64
	unit  byte
}

type ScrobbleRange struct {
	From int64
	To   int64
}

func ParsePeriod(text string) (Period, error) {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return Period{}, nil
	}
	number, unit := text[:len(text)-1], text[len(text)-1]
	if number == "" || !strings.ContainsRune("dwmy", rune(unit)) || strings.IndexFunc(number, func(r rune) bool {
		return r < '0' || r > '9'
	}) >= 0 {
		return Period{}, ErrInvalidPeriod
	}
	count, err := strconv.ParseInt(number, 10, 64)
	if err != nil {
		return Period{}, fmt.Errorf("%w: %w", ErrInvalidPeriod, err)
	}
	if count == 0 {
		return Period{}, ErrInvalidPeriod
	}
	return Period{count: count, unit: unit}, nil
}

func (p Period) AllTime() bool {
	return p.unit == 0
}

func (p Period) String() string {
	if p.AllTime() {
		return ""
	}
	return strconv.FormatInt(p.count, 10) + string(p.unit)
}

// Range фиксирует полуоткрытый диапазон [From, To) с точностью до секунды.
func (p Period) Range(now time.Time) (ScrobbleRange, error) {
	end := now.Unix()
	if end <= 0 {
		return ScrobbleRange{}, ErrInvalidPeriod
	}
	var start int64
	switch p.unit {
	case 'd', 'w':
		seconds := int64(24 * 60 * 60)
		if p.unit == 'w' {
			seconds *= 7
		}
		if p.count > end/seconds {
			return ScrobbleRange{}, ErrInvalidPeriod
		}
		start = end - p.count*seconds
	case 'm', 'y':
		available := int64(now.Year()-1970)*12 + int64(now.Month()-1)
		months := p.count
		if p.unit == 'y' {
			if p.count > available/12 {
				return ScrobbleRange{}, ErrInvalidPeriod
			}
			months *= 12
		}
		if months > available {
			return ScrobbleRange{}, ErrInvalidPeriod
		}
		first := time.Date(now.Year(), now.Month()-time.Month(months), 1,
			now.Hour(), now.Minute(), now.Second(), 0, now.Location())
		lastDay := first.AddDate(0, 1, -1).Day()
		start = first.AddDate(0, 0, min(now.Day(), lastDay)-1).Unix()
	default:
		return ScrobbleRange{}, ErrInvalidPeriod
	}
	if start < 0 || start >= end {
		return ScrobbleRange{}, ErrInvalidPeriod
	}
	return ScrobbleRange{From: start, To: end}, nil
}

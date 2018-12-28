package main

import (
	"errors"
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	throttle := 2
	period := 1 * time.Second
	now := time.Now()
	sessionID := SessionID(1)

	tests := []struct {
		count int
		after time.Time

		err error
	}{
		// within 1 second window
		{1, now, nil},
		{2, now, errors.New("403 Forbidden")},
		{3, now, errors.New("403 Forbidden")},

		//  10 seconds window
		{1, now.Add(10 * time.Second), nil},
		{2, now.Add(10 * time.Second), nil},
		{3, now.Add(10 * time.Second), nil},
	}

	for _, test := range tests {

		sessions := make(map[SessionID]*freq)
		sessions[sessionID] = &freq{Count: test.count, StartTime: now}

		err := rateLimiter(sessions, sessionID, throttle, period, test.after)
		if err != nil && err.Error() != test.err.Error() {
			t.Errorf("want(%s), got(%s).", err, test.err)
		}
	}

}

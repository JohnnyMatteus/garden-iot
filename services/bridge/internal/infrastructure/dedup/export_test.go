package dedup

import "time"

func SetClock(m *Memory, now func() time.Time) { m.now = now }

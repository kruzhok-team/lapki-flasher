package main

import (
	"sync"
	"time"
)

// управление состоянием блокировки
type Cooldown struct {
	// продолжительность блокировки
	duration time.Duration
	// время, когда блокировку вызвали в последний раз
	lastTimeCalled time.Time
	// для синхронизации
	mu sync.Mutex
	// true = блокировка находится в замороженном состоянии
	frozen bool
}

func newCooldown(duration time.Duration) *Cooldown {
	var cd Cooldown
	cd.duration = duration
	cd.lastTimeCalled = time.Time{}
	return &cd
}

func (cd *Cooldown) isBlocked() bool {
	cd.mu.Lock()
	defer cd.mu.Unlock()
	return cd.frozen || time.Now().Sub(cd.lastTimeCalled) < cd.duration
}

// начать блокировку, которая закончится через указанное в duration время
func (cd *Cooldown) start() {
	cd.mu.Lock()
	defer cd.mu.Unlock()
	cd.frozen = false
	cd.lastTimeCalled = time.Now()
}

// заморозить блокировку, до тех пор пока start или unlock не будут вызваны
func (cd *Cooldown) freeze() {
	cd.mu.Lock()
	defer cd.mu.Unlock()
	cd.frozen = true
}

// снять блокировку
func (cd *Cooldown) unlock() {
	cd.mu.Lock()
	defer cd.mu.Unlock()
	cd.frozen = false
	cd.lastTimeCalled = time.Time{}
}

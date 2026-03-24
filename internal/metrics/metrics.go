package metrics

import (
	"log"
	"sync/atomic"
	"time"
)

// Counters tracks operational worker metrics.
type Counters struct {
	MessagesReceived uint64
	MessagesDeleted  uint64
	ObjectsRead      uint64
	RecordsRead      uint64
	RecordsDropped   uint64
	RecordsPushed    uint64
	Loki429          uint64
	Errors           uint64
}

// IncMessagesReceived increments total SQS messages fetched.
func (c *Counters) IncMessagesReceived() { atomic.AddUint64(&c.MessagesReceived, 1) }

// IncMessagesDeleted increments total SQS messages deleted.
func (c *Counters) IncMessagesDeleted() { atomic.AddUint64(&c.MessagesDeleted, 1) }

// AddObjectsRead increments object read count.
func (c *Counters) AddObjectsRead(n int) {
	for i := 0; i < n; i++ {
		atomic.AddUint64(&c.ObjectsRead, 1)
	}
}

// AddRecordsRead increments parsed line count.
func (c *Counters) AddRecordsRead(n int) {
	for i := 0; i < n; i++ {
		atomic.AddUint64(&c.RecordsRead, 1)
	}
}

// AddRecordsDrop increments dropped line count.
func (c *Counters) AddRecordsDrop(n int) {
	for i := 0; i < n; i++ {
		atomic.AddUint64(&c.RecordsDropped, 1)
	}
}

// AddRecordsPush increments pushed line count.
func (c *Counters) AddRecordsPush(n int) {
	for i := 0; i < n; i++ {
		atomic.AddUint64(&c.RecordsPushed, 1)
	}
}

// Inc429 increments Loki 429 counter.
func (c *Counters) Inc429() { atomic.AddUint64(&c.Loki429, 1) }

// IncErr increments generic error counter.
func (c *Counters) IncErr() { atomic.AddUint64(&c.Errors, 1) }

// StartLogger periodically emits metric counters as logs.
func StartLogger(c *Counters, every time.Duration, stop <-chan struct{}) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			log.Printf("metrics messages_received=%d messages_deleted=%d objects_read=%d records_read=%d records_dropped=%d records_pushed=%d loki_429=%d errors=%d",
				atomic.LoadUint64(&c.MessagesReceived),
				atomic.LoadUint64(&c.MessagesDeleted),
				atomic.LoadUint64(&c.ObjectsRead),
				atomic.LoadUint64(&c.RecordsRead),
				atomic.LoadUint64(&c.RecordsDropped),
				atomic.LoadUint64(&c.RecordsPushed),
				atomic.LoadUint64(&c.Loki429),
				atomic.LoadUint64(&c.Errors),
			)
		}
	}
}

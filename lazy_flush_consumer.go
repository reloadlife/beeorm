package beeorm

import (
	"time"

	"github.com/go-sql-driver/mysql"
)

const LazyFlushChannelName = "orm-lazy-flush-stream"
const LazyFlushGroupName = "orm-lazy-flush-consumer"

type LazyFlushConsumer struct {
	eventConsumerBase
	consumer                     *eventsConsumer
	lazyFlushQueryErrorResolvers []LazyFlushQueryErrorResolver
	flusher                      *flusher
}

func NewLazyFlushConsumer(c Context) *LazyFlushConsumer {
	lfc := &LazyFlushConsumer{}
	lfc.c = c.(*contextImplementation)
	lfc.c.SetMetaData("lazy", "1")
	lfc.block = true
	lfc.blockTime = time.Second * 30
	lfc.flusher = &flusher{c: c}
	return lfc
}

type LazyFlushQueryErrorResolver func(c Context, event EventEntityFlushed, queryError *mysql.MySQLError) error

func (lfc *LazyFlushConsumer) RegisterLazyFlushQueryErrorResolver(resolver LazyFlushQueryErrorResolver) {
	lfc.lazyFlushQueryErrorResolvers = append(lfc.lazyFlushQueryErrorResolvers, resolver)
}

func (lfc *LazyFlushConsumer) Digest() bool {
	lfc.consumer = lfc.c.EventBroker().Consumer(LazyFlushGroupName).(*eventsConsumer)
	lfc.consumer.eventConsumerBase = lfc.eventConsumerBase
	return lfc.consumer.Consume(lfc.c, 500, func(events []Event) {
		lazyEvents := make([]*entitySQLFlush, 0)
		for _, e := range events {
			var data []*entitySQLFlush
			e.Unserialize(&data)
			lazyEvents = append(lazyEvents, data...)
		}
		lfc.handleEvents(events, lazyEvents)
	})
}

func (lfc *LazyFlushConsumer) handleEvents(events []Event, lazyEvents []*entitySQLFlush) {
	defer func() {
		if rec := recover(); rec != nil {
			_, isMySQLError := rec.(*mysql.MySQLError)
			if !isMySQLError {
				panic(rec)
			}
			for i, e := range lazyEvents {
				f := &flusher{c: lfc.c}
				f.events = []*entitySQLFlush{e}
				func() {
					defer func() {
						if rec2 := recover(); rec2 != nil {
							mySQLError, stillMySQLError := rec.(*mysql.MySQLError)
							if !stillMySQLError {
								panic(rec2)
							}
							for _, errorResolver := range lfc.lazyFlushQueryErrorResolvers {
								if errorResolver(lfc.c, e, mySQLError) == nil {
									events[i].Ack(lfc.c)
									return
								}
							}
							panic(rec2)
						}
						events[i].Ack(lfc.c)
					}()
					f.execute(false, true)
				}()
			}
		}
	}()
	f := &flusher{c: lfc.c}
	f.events = lazyEvents
	f.execute(false, true)
	f.flushCacheSetters()
}

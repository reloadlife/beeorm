package beeorm

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type lazyReceiverEntity struct {
	ORM          `orm:"localCache;redisCache;asyncRedisLazyFlush=default"`
	ID           uint
	Name         string `orm:"unique=name"`
	Age          uint64
	EnumNullable string `orm:"enum=beeorm.TestEnum"`
	RefOne       *lazyReceiverReference
	IndexAll     *CachedQuery `query:""`
}

type lazyReceiverReference struct {
	ORM
	ID   uint
	Name string
}

func TestBackgroundConsumer(t *testing.T) {
	var entity *lazyReceiverEntity
	var ref *lazyReceiverReference

	registry := &Registry{}
	registry.RegisterEnum("beeorm.TestEnum", []string{"a", "b", "c"})
	engine, def := prepareTables(t, registry, 5, "", "2.0", entity, ref)
	defer def()
	engine.GetRedis().FlushDB()

	receiver := NewBackgroundConsumer(engine)
	receiver.DisableLoop()
	receiver.blockTime = time.Millisecond

	e := &lazyReceiverEntity{Name: "John", Age: 18}
	engine.FlushLazy(e)

	sample := receiver.GetLazyFlushEventsSample(1000)
	assert.Len(t, sample, 1)

	e = &lazyReceiverEntity{}
	loaded := engine.LoadByID(1, e)
	assert.False(t, loaded)

	receiver.Digest(context.Background())

	engine.GetLocalCache().Clear()
	loaded = engine.LoadByID(1, e)
	assert.True(t, loaded)
	assert.Equal(t, "John", e.Name)
	assert.Equal(t, uint64(18), e.Age)

	e.Name = "Tom"
	engine.FlushLazy(e)

	e.Age = 30
	engine.FlushLazy(e)

	e = &lazyReceiverEntity{}
	loaded = engine.LoadByID(1, e)
	assert.True(t, loaded)
	assert.Equal(t, "John", e.Name)
	assert.Equal(t, uint64(18), e.Age)

	engine.GetLocalCache().Clear()
	e = &lazyReceiverEntity{}
	loaded = engine.LoadByID(1, e)
	assert.True(t, loaded)
	assert.Equal(t, "John", e.Name)

	receiver.Digest(context.Background())

	e = &lazyReceiverEntity{}
	loaded = engine.LoadByID(1, e)
	assert.True(t, loaded)
	assert.Equal(t, "Tom", e.Name)

	e = &lazyReceiverEntity{}
	e.Name = "Monica"
	e.Age = 18
	engine.Flush(e)

	engine.LoadByID(uint64(e.ID), e)
	e.Name = "Ivona"
	engine.FlushLazy(e)

	e2 := &lazyReceiverEntity{}
	e2.Name = "Adam"
	e2.Age = 20
	engine.FlushLazy(e2)

	e.Age = 60
	engine.FlushLazy(e)

	receiver.Digest(context.Background())

	e = &lazyReceiverEntity{}
	loaded = engine.LoadByID(2, e)
	assert.True(t, loaded)
	assert.Equal(t, "Ivona", e.Name)
	assert.Equal(t, uint64(60), e.Age)

	loaded = engine.LoadByID(3, e2)
	assert.True(t, loaded)
	assert.Equal(t, "Adam", e2.Name)
	assert.Equal(t, uint64(20), e2.Age)

	e1 := &lazyReceiverEntity{}
	e2 = &lazyReceiverEntity{}
	e3 := &lazyReceiverEntity{}
	engine.LoadByID(1, e1)
	engine.LoadByID(2, e2)
	engine.LoadByID(3, e3)
	e1.Name = "Tommy"
	e2.Name = "Tommy2"
	e3.Name = "Tommy3"
	engine.FlushLazyMany(e1, e2, e3)
	receiver.SetLazyFlushWorkers(2)
	receiver.Digest(context.Background())
	e1 = &lazyReceiverEntity{}
	e2 = &lazyReceiverEntity{}
	e3 = &lazyReceiverEntity{}
	engine.LoadByID(1, e1)
	engine.LoadByID(2, e2)
	engine.LoadByID(3, e3)
	assert.Equal(t, "Tommy", e1.Name)
	assert.Equal(t, "Tommy2", e2.Name)
	assert.Equal(t, "Tommy3", e3.Name)

	e = &lazyReceiverEntity{Name: "Tom"}
	e.SetOnDuplicateKeyUpdate(map[string]interface{}{"Age": 38})
	assert.PanicsWithError(t, "lazy flush on duplicate key is not supported", func() {
		engine.FlushLazy(e)
	})

	e = &lazyReceiverEntity{Name: "Adam", RefOne: &lazyReceiverReference{Name: "Test"}}
	assert.PanicsWithError(t, "lazy flush for unsaved references is not supported", func() {
		engine.FlushLazy(e)
	})

	e = &lazyReceiverEntity{}
	engine.LoadByID(1, e)
	engine.NewFlusher().Delete(e).FlushLazy()
	receiver.Digest(context.Background())
	loaded = engine.LoadByID(1, e)
	assert.False(t, loaded)

	e = &lazyReceiverEntity{ID: 100}
	engine.Flush(e)
	engine.DeleteLazy(e)
	e = &lazyReceiverEntity{}
	receiver.Digest(context.Background())
	engine.GetLocalCache().Clear()
	engine.GetRedis().FlushDB()
	assert.False(t, engine.LoadByID(100, e))

	e2 = &lazyReceiverEntity{}
	e3 = &lazyReceiverEntity{}
	engine.LoadByID(2, e2)
	engine.LoadByID(3, e3)
	e2.Name = "John"
	e3.Name = "Ivona"
	engine.GetMysql().Begin()
	engine.FlushLazyMany(e2, e3)
	engine.GetMysql().Commit()
	receiver.Digest(context.Background())
	engine.GetLocalCache().Clear()
	engine.GetRedis().FlushDB()
	e2 = &lazyReceiverEntity{}
	e3 = &lazyReceiverEntity{}
	engine.LoadByID(2, e2)
	engine.LoadByID(3, e3)
	assert.Equal(t, "John", e2.Name)
	assert.Equal(t, "Ivona", e3.Name)
}

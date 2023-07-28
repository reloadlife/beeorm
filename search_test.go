package beeorm

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type searchEntity struct {
	ORM          `orm:"localCache;redisCache"`
	ID           uint64
	Name         string
	ReferenceOne *searchEntityReference
}

type searchEntityReference struct {
	ORM
	ID   uint64
	Name string
}

func TestSearch(t *testing.T) {
	var entity *searchEntity
	var reference *searchEntityReference
	c := PrepareTables(t, &Registry{}, 5, 6, "", entity, reference)

	for i := 1; i <= 10; i++ {
		c.Flusher().Track(&searchEntity{Name: fmt.Sprintf("name %d", i), ReferenceOne: &searchEntityReference{Name: fmt.Sprintf("name %d", i)}})
	}
	c.Flusher().Flush()
	entity = &searchEntity{ID: 1}
	Load(c, entity)
	c.Flusher().Track(entity).Flush()

	var rows []*searchEntity
	GetByIDs(c, []uint64{1, 2, 20}, &rows)
	assert.Len(t, rows, 3)
	assert.Equal(t, uint64(1), rows[0].GetID())
	assert.Equal(t, uint64(2), rows[1].GetID())
	assert.Nil(t, rows[2])

	entity, found := SearchOne[*searchEntity](c, NewWhere("ID = ?", 1), "ReferenceOne")
	assert.True(t, found)
	assert.Equal(t, uint64(1), entity.GetID())
	assert.Equal(t, "name 1", entity.Name)
	assert.Equal(t, "name 1", entity.ReferenceOne.Name)
	assert.True(t, entity.ReferenceOne.IsLoaded())

	Search(c, NewWhere("ID > 0"), nil, &rows, "ReferenceOne")
	assert.Len(t, rows, 10)
	assert.Equal(t, uint64(1), rows[0].GetID())
	assert.Equal(t, "name 1", rows[0].Name)
	assert.Equal(t, "name 1", rows[0].ReferenceOne.Name)
	assert.True(t, rows[0].ReferenceOne.IsLoaded())

	total := SearchWithCount(c, NewWhere("ID > 2"), nil, &rows)
	assert.Equal(t, 8, total)
	assert.Len(t, rows, 8)

	ids, total := SearchIDsWithCount[*searchEntity](c, NewWhere("ID > 2"), nil)
	assert.Equal(t, 8, total)
	assert.Len(t, ids, 8)
	assert.Equal(t, uint64(3), ids[0])

	ids = SearchIDs[*searchEntity](c, NewWhere("ID > 2"), nil)
	assert.Len(t, ids, 8)
	assert.Equal(t, uint64(3), ids[0])

	c = PrepareTables(t, &Registry{}, 5, 6, "")
	assert.PanicsWithError(t, "entity 'beeorm.searchEntity' is not registered", func() {
		Search(c, NewWhere("ID > 0"), nil, &rows)
	})
}

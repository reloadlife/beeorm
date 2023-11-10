package modified

import (
	"testing"
	"time"

	"github.com/latolukasz/beeorm/v3"

	"github.com/stretchr/testify/assert"
)

type testPluginModifiedEntity struct {
	ID                     uint64
	Name                   string
	AddedAtDate            time.Time
	AddedAtDateOptional    *time.Time
	ModifiedAtDate         time.Time
	ModifiedAtDateOptional *time.Time
	AddedAtTime            time.Time  `orm:"time"`
	AddedAtTimeOptional    *time.Time `orm:"time"`
	ModifiedAtTime         time.Time  `orm:"time"`
	ModifiedAtTimeOptional *time.Time `orm:"time"`
	AddedAtIgnored         time.Time  `orm:"ignore"`
}

func TestPlugin(t *testing.T) {
	registry := beeorm.NewRegistry()
	registry.RegisterPlugin(New("AddedAtDate", "ModifiedAtDate"))
	c := beeorm.PrepareTables(t, registry, testPluginModifiedEntity{})

	now := time.Now().UTC()
	entity := beeorm.NewEntity[testPluginModifiedEntity](c)
	entity.Name = "a"
	assert.NoError(t, c.Flush())
	assert.NotNil(t, entity.AddedAtDate)
	assert.Equal(t, entity.AddedAtDate.Format(time.DateOnly), now.Format(time.DateOnly))
	assert.Equal(t, "0001-01-01", entity.ModifiedAtDate.Format(time.DateOnly))
	entity = beeorm.GetByID[testPluginModifiedEntity](c, entity.ID)
	assert.Equal(t, entity.AddedAtDate.Format(time.DateOnly), now.Format(time.DateOnly))
	assert.Equal(t, "0001-01-01", entity.ModifiedAtDate.Format(time.DateOnly))

	registry = beeorm.NewRegistry()
	registry.RegisterPlugin(New("AddedAtTime", "ModifiedAtTime"))
	c = beeorm.PrepareTables(t, registry, testPluginModifiedEntity{})
	now = time.Now().UTC()
	entity = beeorm.NewEntity[testPluginModifiedEntity](c)
	entity.Name = "b"
	assert.NoError(t, c.Flush())
	assert.NotNil(t, entity.AddedAtTime)
	assert.Equal(t, entity.AddedAtTime.Format(time.DateTime), now.Format(time.DateTime))
	assert.Equal(t, "0001-01-01 00:00:00", entity.ModifiedAtTime.Format(time.DateTime))
	entity = beeorm.GetByID[testPluginModifiedEntity](c, entity.ID)
	assert.Equal(t, entity.AddedAtTime.Format(time.DateTime), now.Format(time.DateTime))
	assert.Equal(t, "0001-01-01 00:00:00", entity.ModifiedAtTime.Format(time.DateTime))

	registry = beeorm.NewRegistry()
	registry.RegisterPlugin(New("AddedAtTimeOptional", "ModifiedAtTimeOptional"))
	c = beeorm.PrepareTables(t, registry, testPluginModifiedEntity{})
	now = time.Now().UTC()
	entity = beeorm.NewEntity[testPluginModifiedEntity](c)
	entity.Name = "d"
	assert.NoError(t, c.Flush())
	assert.NotNil(t, entity.AddedAtTimeOptional)
	assert.Equal(t, entity.AddedAtTimeOptional.Format(time.DateTime), now.Format(time.DateTime))
	assert.Nil(t, entity.ModifiedAtTimeOptional)
	entity = beeorm.GetByID[testPluginModifiedEntity](c, entity.ID)
	assert.Equal(t, entity.AddedAtTimeOptional.Format(time.DateTime), now.Format(time.DateTime))
	assert.Nil(t, entity.ModifiedAtTimeOptional)

	registry = beeorm.NewRegistry()
	registry.RegisterPlugin(New("AddedAtDateOptional", "ModifiedAtDateOptional"))
	c = beeorm.PrepareTables(t, registry, testPluginModifiedEntity{})
	now = time.Now().UTC()
	entity = beeorm.NewEntity[testPluginModifiedEntity](c)
	entity.Name = "d"
	assert.NoError(t, c.Flush())
	assert.NotNil(t, entity.AddedAtDateOptional)
	assert.Equal(t, entity.AddedAtDateOptional.Format(time.DateOnly), now.Format(time.DateOnly))
	assert.Nil(t, entity.ModifiedAtDateOptional)
	entity = beeorm.GetByID[testPluginModifiedEntity](c, entity.ID)
	assert.Equal(t, entity.AddedAtDateOptional.Format(time.DateOnly), now.Format(time.DateOnly))
	assert.Nil(t, entity.ModifiedAtDateOptional)

	registry = beeorm.NewRegistry()
	registry.RegisterPlugin(New("Invalid", "Invalid"))
	c = beeorm.PrepareTables(t, registry, testPluginModifiedEntity{})
	now = time.Now().UTC()
	entity = beeorm.NewEntity[testPluginModifiedEntity](c)
	entity.Name = "e"
	assert.NoError(t, c.Flush())
	entity = beeorm.GetByID[testPluginModifiedEntity](c, entity.ID)
	assert.Equal(t, "e", entity.Name)

	registry = beeorm.NewRegistry()
	registry.RegisterPlugin(New("AddedAtIgnored", "AddedAtIgnored"))
	c = beeorm.PrepareTables(t, registry, testPluginModifiedEntity{})
	now = time.Now().UTC()
	entity = beeorm.NewEntity[testPluginModifiedEntity](c)
	entity.Name = "f"
	assert.NoError(t, c.Flush())
	entity = beeorm.GetByID[testPluginModifiedEntity](c, entity.ID)
	assert.Equal(t, "f", entity.Name)

	registry = beeorm.NewRegistry()
	registry.RegisterPlugin(New("Name", "Name"))
	c = beeorm.PrepareTables(t, registry, testPluginModifiedEntity{})
	now = time.Now().UTC()
	entity = beeorm.NewEntity[testPluginModifiedEntity](c)
	entity.Name = "g"
	assert.NoError(t, c.Flush())
	entity = beeorm.GetByID[testPluginModifiedEntity](c, entity.ID)
	assert.Equal(t, "g", entity.Name)

	beeorm.DeleteEntity[testPluginModifiedEntity](c, entity)
	assert.NoError(t, c.Flush())

	assert.PanicsWithError(t, "at least one column name must be defined", func() {
		New("", "")
	})
	assert.PanicsWithError(t, "addedAt field 'a' must be public", func() {
		New("a", "b")
	})
	assert.PanicsWithError(t, "modifiedAtField field 'b' must be public", func() {
		New("A", "b")
	})
}

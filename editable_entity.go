package beeorm

import (
	"reflect"
)

type FlushType int

const (
	Insert FlushType = iota
	Update
	Delete
)

type EntityFlush interface {
	ID() uint64
	Schema() *entitySchema
	flushType() FlushType
}

type entityFlushInsert interface {
	EntityFlush
	getBind() (Bind, error)
	getEntity() any
	getValue() reflect.Value
}

type entityFlushDelete interface {
	EntityFlush
	getOldBind() (Bind, error)
	getValue() reflect.Value
}

type entityFlushUpdate interface {
	EntityFlush
	getBind() (new, old Bind, err error)
	getValue() reflect.Value
	getSourceValue() reflect.Value
	getEntity() any
}

type EntityFlushedEvent interface {
	FlushType() FlushType
}

type writableEntity struct {
	orm    ORM
	schema *entitySchema
}

func (w *writableEntity) Schema() *entitySchema {
	return w.schema
}

type insertableEntity struct {
	writableEntity
	entity any
	id     uint64
	value  reflect.Value
}

func (m *insertableEntity) ID() uint64 {
	return m.id
}

func (m *insertableEntity) getEntity() any {
	return m.entity
}

func (m *insertableEntity) flushType() FlushType {
	return Insert
}

func (m *insertableEntity) getValue() reflect.Value {
	return m.value
}

func (e *editableEntity) flushType() FlushType {
	return Update
}

func (e *editableEntity) getValue() reflect.Value {
	return e.value
}

func (e *editableEntity) getEntity() any {
	return e.entity
}

func (e *editableEntity) getSourceValue() reflect.Value {
	return e.sourceValue
}

type removableEntity struct {
	writableEntity
	id     uint64
	value  reflect.Value
	source any
}

func (r *removableEntity) flushType() FlushType {
	return Delete
}

func (r *removableEntity) SourceEntity() any {
	return r.source
}

func (r *removableEntity) getValue() reflect.Value {
	return r.value
}

type editableEntity struct {
	writableEntity
	entity      any
	id          uint64
	value       reflect.Value
	sourceValue reflect.Value
	source      any
}

type editableFields struct {
	writableEntity
	id      uint64
	value   reflect.Value
	newBind Bind
	oldBind Bind
}

func (f *editableFields) ID() uint64 {
	return f.id
}

func (f *editableFields) flushType() FlushType {
	return Update
}

func (f *editableFields) getBind() (new, old Bind, err error) {
	return f.newBind, f.oldBind, nil
}

func (f *editableFields) getEntity() any {
	return nil
}

func (f *editableFields) getSourceValue() reflect.Value {
	return f.value
}

func (f *editableFields) getValue() reflect.Value {
	return f.value
}

func (e *editableEntity) ID() uint64 {
	return e.id
}

func (r *removableEntity) ID() uint64 {
	return r.id
}

func (e *editableEntity) TrackedEntity() any {
	return e.entity
}

func (e *editableEntity) SourceEntity() any {
	return e.source
}

func NewEntity[E any](orm ORM) *E {
	return newEntity(orm, getEntitySchema[E](orm)).(*E)
}

func newEntityInsertable(orm ORM, schema *entitySchema) *insertableEntity {
	entity := &insertableEntity{}
	entity.orm = orm
	entity.schema = schema
	value := reflect.New(schema.t)
	elem := value.Elem()
	initNewEntity(elem, schema.fields)
	entity.entity = value.Interface()
	id := schema.uuid(orm)
	entity.id = id
	elem.Field(0).SetUint(id)
	entity.value = value
	orm.trackEntity(entity)
	return entity
}

func newEntity(orm ORM, schema *entitySchema) any {
	return newEntityInsertable(orm, schema).entity
}

func DeleteEntity[E any](orm ORM, source E) {
	toRemove := &removableEntity{}
	toRemove.orm = orm
	toRemove.source = source
	toRemove.value = reflect.ValueOf(source).Elem()
	toRemove.id = toRemove.value.Field(0).Uint()
	schema := getEntitySchema[E](orm)
	toRemove.schema = schema
	orm.trackEntity(toRemove)
}

func EditEntity[E any](orm ORM, source E) E {
	writable := copyToEdit(orm, source)
	writable.id = writable.value.Elem().Field(0).Uint()
	writable.source = source
	orm.trackEntity(writable)
	return writable.entity.(E)
}

func initNewEntity(elem reflect.Value, fields *tableFields) {
	for k, i := range fields.stringsEnums {
		def := fields.enums[k]
		if def.required {
			elem.Field(i).SetString(def.defaultValue)
		}
	}
	for k, i := range fields.sliceStringsSets {
		def := fields.enums[k]
		if def.required {
			f := elem.Field(i)
			setValues := reflect.MakeSlice(f.Type(), 1, 1)
			setValues.Index(0).SetString(def.defaultValue)
			f.Set(setValues)
		}
	}
}

func IsDirty[E any](orm ORM, id uint64) (oldValues, newValues Bind, hasChanges bool) {
	return isDirty(orm, getEntitySchema[E](orm), id)
}

func isDirty(orm ORM, schema *entitySchema, id uint64) (oldValues, newValues Bind, hasChanges bool) {
	tracked := orm.(*ormImplementation).trackedEntities
	if tracked == nil {
		return nil, nil, false
	}
	if schema == nil {
		return nil, nil, false
	}
	values, has := tracked.Load(schema.index)
	if !has {
		return nil, nil, false
	}
	row, has := values.Load(id)
	if !has {
		return nil, nil, false
	}
	editable, isUpdate := row.(entityFlushUpdate)
	if !isUpdate {
		insertable, isInsert := row.(entityFlushInsert)
		if isInsert {
			bind, _ := insertable.getBind()
			return nil, bind, true
		}
		return nil, nil, false
	}
	oldValues, newValues, _ = editable.getBind()
	if len(oldValues) == 0 && len(newValues) == 0 {
		return nil, nil, false
	}
	return oldValues, newValues, true
}

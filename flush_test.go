package beeorm

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

type flushStruct struct {
	Name2    string
	Age      int
	Sub      flushSubStruct
	TestTime *time.Time `orm:"time=true"`
}

type flushSubStruct struct {
	Name3 string
	Age3  int
}

type flushStructAnonymous struct {
	SubName string
	SubAge  float32 `orm:"decimal=9,5;unsigned=false"`
}

type testEnum Enum
type testSet []testEnum

func (s testEnum) EnumValues() interface{} {
	return testEnumDefinition
}
func (s testSet) EnumValues() interface{} {
	return testEnumDefinition
}

var testEnumDefinition = struct {
	A testEnum
	B testEnum
	C testEnum
}{
	A: "a",
	B: "b",
	C: "c",
}

type flushEntity struct {
	ID                   uint64 `orm:"localCache;redisCache"`
	City                 string `orm:"unique=city"`
	Name                 string `orm:"unique=name;required"`
	Age                  int
	Uint                 uint
	UintNullable         *uint
	IntNullable          *int
	Year                 uint16  `orm:"year"`
	YearNullable         *uint16 `orm:"year"`
	BoolNullable         *bool
	FloatNullable        *float64 `orm:"precision=10"`
	Float32Nullable      *float32 `orm:"precision=4"`
	SetNullable          testSet
	SetNotNull           testSet `orm:"required"`
	EnumNullable         testEnum
	EnumNotNull          testEnum `orm:"required"`
	Ignored              []string `orm:"ignore"`
	Blob                 []uint8
	Bool                 bool
	Float64              float64  `orm:"precision=10"`
	Decimal              float64  `orm:"decimal=5,2"`
	DecimalNullable      *float64 `orm:"decimal=5,2"`
	Float64Default       float64  `orm:"unsigned"`
	Float64Signed        float64
	Time                 time.Time
	TimeWithTime         time.Time `orm:"time"`
	TimeNullable         *time.Time
	TimeWithTimeNullable *time.Time `orm:"time"`
	FlushStruct          flushStruct
	Int8Nullable         *int8
	Int16Nullable        *int16
	Int32Nullable        *int32
	Int64Nullable        *int64
	Uint8Nullable        *uint8
	Uint16Nullable       *uint16
	Uint32Nullable       *uint32
	Uint64Nullable       *uint64
	flushStructAnonymous
}

func (e *flushEntity) GetID() uint64 {
	return e.ID
}

func TestFlushLocalRedis(t *testing.T) {
	testFlush(t, true, true)
}

func TestFlushLocal(t *testing.T) {
	testFlush(t, true, false)
}

func TestFlushNoCache(t *testing.T) {
	testFlush(t, false, false)
}

func TestFlushRedis(t *testing.T) {
	testFlush(t, false, true)
}

func testFlush(t *testing.T, local bool, redis bool) {
	registry := &Registry{}
	c := PrepareTables(t, registry, "", &flushEntity{})

	schema := GetEntitySchema[*flushEntity](c)
	schema.DisableCache(!local, !redis)

	c.EnableQueryDebug()

	// Adding empty entity
	newEntity := NewEntity[*flushEntity](c).TrackedEntity()
	assert.NotEmpty(t, newEntity.ID)
	assert.PanicsWithError(t, "enum EnumNotNull cannot be empty", func() {
		c.Flush()
	})
	newEntity.EnumNotNull = testEnumDefinition.B
	assert.PanicsWithError(t, "set SetNotNull cannot be empty", func() {
		c.Flush()
	})
	newEntity.SetNotNull = testSet{testEnumDefinition.A, testEnumDefinition.C}
	c.Flush()

	entity := GetByID[*flushEntity](c, newEntity.ID)
	assert.NotNil(t, entity)
	assert.Equal(t, newEntity.ID, entity.ID)
	assert.Equal(t, "", entity.City)
	assert.Equal(t, "", entity.Name)
	assert.Equal(t, 0, entity.Age)
	assert.Equal(t, uint(0), entity.Uint)
	assert.Nil(t, entity.UintNullable)
	assert.Nil(t, entity.IntNullable)
	assert.Equal(t, uint16(0), entity.Year)
	assert.Nil(t, entity.YearNullable)
	assert.Nil(t, entity.BoolNullable)
	assert.Nil(t, entity.FloatNullable)
	assert.Nil(t, entity.Float32Nullable)
	assert.Nil(t, entity.SetNullable)
	assert.Equal(t, testSet{testEnumDefinition.A, testEnumDefinition.C}, entity.SetNotNull)
	assert.Equal(t, testEnum(""), entity.EnumNullable)
	assert.Equal(t, testEnumDefinition.B, entity.EnumNotNull)
	assert.Nil(t, entity.Blob)
	assert.False(t, entity.Bool)
	assert.Equal(t, 0.0, entity.Float64)
	assert.Equal(t, 0.0, entity.Decimal)
	assert.Nil(t, entity.DecimalNullable)
	assert.Equal(t, 0.0, entity.Float64Default)
	assert.Equal(t, 0.0, entity.Float64Signed)
	assert.Equal(t, new(time.Time).UTC(), entity.Time)
	assert.Equal(t, new(time.Time).UTC(), entity.TimeWithTime)
	assert.Nil(t, entity.TimeNullable)
	assert.Nil(t, entity.TimeWithTimeNullable)
	assert.Equal(t, "", entity.FlushStruct.Name2)
	assert.Equal(t, 0, entity.FlushStruct.Age)
	assert.Equal(t, "", entity.FlushStruct.Sub.Name3)
	assert.Equal(t, 0, entity.FlushStruct.Sub.Age3)
	assert.Nil(t, entity.FlushStruct.TestTime)
	assert.Nil(t, entity.Int8Nullable)
	assert.Nil(t, entity.Int16Nullable)
	assert.Nil(t, entity.Int32Nullable)
	assert.Nil(t, entity.Int64Nullable)
	assert.Nil(t, entity.Uint8Nullable)
	assert.Nil(t, entity.Uint16Nullable)
	assert.Nil(t, entity.Uint32Nullable)
	assert.Nil(t, entity.Uint64Nullable)
	assert.Equal(t, "", entity.SubName)
	assert.Equal(t, float32(0), entity.SubAge)

	// Adding full entity
	newEntity = NewEntity[*flushEntity](c).TrackedEntity()
	newEntity.City = "New York"
	newEntity.Name = "Test name"
	newEntity.Age = -19
	newEntity.Uint = 134
	uintNullable := uint(23)
	newEntity.UintNullable = &uintNullable
	intNullable := -45
	newEntity.IntNullable = &intNullable
	newEntity.Year = 2023
	yearNullable := uint16(2025)
	newEntity.YearNullable = &yearNullable
	boolNullable := true
	newEntity.BoolNullable = &boolNullable
	floatNullable := 12.23
	newEntity.FloatNullable = &floatNullable
	float32Nullable := float32(12.23)
	newEntity.Float32Nullable = &float32Nullable
	newEntity.SetNullable = testSet{testEnumDefinition.B, testEnumDefinition.C}
	newEntity.SetNotNull = testSet{testEnumDefinition.A, testEnumDefinition.C}
	newEntity.EnumNullable = testEnumDefinition.C
	newEntity.EnumNotNull = testEnumDefinition.A
	newEntity.Blob = []byte("test binary")
	newEntity.Bool = true
	newEntity.Float64 = 986.2322
	newEntity.Decimal = 78.2357
	decimalNullable := 123.2320932
	newEntity.DecimalNullable = &decimalNullable
	newEntity.Float64Default = 8932.299423
	newEntity.Float64Signed = -352.120321
	newEntity.Time = time.Date(2023, 11, 12, 22, 12, 34, 0, time.UTC)
	newEntity.TimeWithTime = time.Date(2023, 8, 16, 12, 23, 11, 0, time.UTC)
	timeNullable := time.Date(2024, 1, 2, 3, 4, 5, 6, time.UTC)
	newEntity.TimeNullable = &timeNullable
	timeWithTimeNullable := time.Date(2025, 11, 4, 21, 0, 5, 6, time.UTC)
	newEntity.TimeWithTimeNullable = &timeWithTimeNullable
	newEntity.FlushStruct.Name2 = "Tom"
	newEntity.FlushStruct.Age = 23
	newEntity.FlushStruct.Sub.Name3 = "Zoya"
	newEntity.FlushStruct.Sub.Age3 = 18
	testTime := time.Date(1982, 11, 4, 21, 0, 5, 6, time.UTC)
	newEntity.FlushStruct.TestTime = &testTime
	int8Nullable := int8(23)
	newEntity.Int8Nullable = &int8Nullable
	int16Nullable := int16(-29)
	newEntity.Int16Nullable = &int16Nullable
	int32Nullable := int32(-2923)
	newEntity.Int32Nullable = &int32Nullable
	int64Nullable := int64(98872)
	newEntity.Int64Nullable = &int64Nullable
	uint8Nullable := uint8(23)
	newEntity.Uint8Nullable = &uint8Nullable
	uint16Nullable := uint16(29)
	newEntity.Uint16Nullable = &uint16Nullable
	uint32Nullable := uint32(2923)
	newEntity.Uint32Nullable = &uint32Nullable
	uint64Nullable := uint64(98872)
	newEntity.Uint64Nullable = &uint64Nullable
	newEntity.SubName = "sub name"
	newEntity.SubAge = 123
	c.Flush()
	entity = GetByID[*flushEntity](c, newEntity.ID)
	assert.NotNil(t, entity)
	assert.Equal(t, newEntity.ID, entity.ID)
	assert.Equal(t, "", entity.City)
	assert.Equal(t, "", entity.Name)
	assert.Equal(t, 0, entity.Age)
	assert.Equal(t, uint(0), entity.Uint)
	assert.Nil(t, entity.UintNullable)
	assert.Nil(t, entity.IntNullable)
	assert.Equal(t, uint16(0), entity.Year)
	assert.Nil(t, entity.YearNullable)
	assert.Nil(t, entity.BoolNullable)
	assert.Nil(t, entity.FloatNullable)
	assert.Nil(t, entity.Float32Nullable)
	assert.Nil(t, entity.SetNullable)
	assert.Equal(t, testSet{testEnumDefinition.A, testEnumDefinition.C}, entity.SetNotNull)
	assert.Equal(t, testEnum(""), entity.EnumNullable)
	assert.Equal(t, testEnumDefinition.B, entity.EnumNotNull)
	assert.Nil(t, entity.Blob)
	assert.False(t, entity.Bool)
	assert.Equal(t, 0.0, entity.Float64)
	assert.Equal(t, 0.0, entity.Decimal)
	assert.Nil(t, entity.DecimalNullable)
	assert.Equal(t, 0.0, entity.Float64Default)
	assert.Equal(t, 0.0, entity.Float64Signed)
	assert.Equal(t, new(time.Time).UTC(), entity.Time)
	assert.Equal(t, new(time.Time).UTC(), entity.TimeWithTime)
	assert.Nil(t, entity.TimeNullable)
	assert.Nil(t, entity.TimeWithTimeNullable)
	assert.Equal(t, "", entity.FlushStruct.Name2)
	assert.Equal(t, 0, entity.FlushStruct.Age)
	assert.Equal(t, "", entity.FlushStruct.Sub.Name3)
	assert.Equal(t, 0, entity.FlushStruct.Sub.Age3)
	assert.Nil(t, entity.FlushStruct.TestTime)
	assert.Nil(t, entity.Int8Nullable)
	assert.Nil(t, entity.Int16Nullable)
	assert.Nil(t, entity.Int32Nullable)
	assert.Nil(t, entity.Int64Nullable)
	assert.Nil(t, entity.Uint8Nullable)
	assert.Nil(t, entity.Uint16Nullable)
	assert.Nil(t, entity.Uint32Nullable)
	assert.Nil(t, entity.Uint64Nullable)
	assert.Equal(t, "", entity.SubName)
	assert.Equal(t, float32(0), entity.SubAge)
}

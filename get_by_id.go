package beeorm

import (
	"reflect"
	"strconv"
)

func GetByID[E Entity, I ID](c Context, id I, references ...string) (entity E) {
	entity = getByID[E, I](c, id, nil, references...)
	return
}

func getByID[E Entity, I ID](c Context, id I, entityToFill Entity, references ...string) (entity E) {
	schema := GetEntitySchema[E]().(*entitySchema)

	cacheLocal, hasLocalCache := schema.GetLocalCache(c)
	cacheRedis, hasRedis := schema.GetRedisCache(c)
	var cacheKey string
	if hasLocalCache {
		e, has := cacheLocal.Get(id)
		if has {
			if e == cacheNilValue {
				return
			}
			entity = e.(reflect.Value).Interface().(E)
			//if len(references) > 0 {
			//	warmUpReferences(c, schema, orm.value, references, false)
			//}
			return
		}
	}
	if hasRedis {
		cacheKey = strconv.FormatUint(uint64(id), 10)
		row, has := cacheRedis.HGet(schema.cachePrefix, cacheKey)
		if has {
			if row == cacheNilValue {
				if hasLocalCache {
					cacheLocal.Set(uint64(id), cacheNilValue)
				}
				return
			}
			if entityToFill == nil {
				entity = schema.newEntity().(E)
			} else {
				entity = entityToFill.(E)
			}
			fillFromBinary(c, schema, []byte(row), entity)
			//if len(references) > 0 {
			//	warmUpReferences(c, schema, orm.value, references, false)
			//}
			if hasLocalCache {
				cacheLocal.Set(uint64(id), entity.getORM().value)
			}
			return
		}
	}
	entity = searchRow[E](c, NewWhere("`ID` = ?", id), nil, false, nil)
	if entity != nil {
		if hasLocalCache {
			cacheLocal.Set(uint64(id), cacheNilValue)
		}
		if hasRedis {
			cacheRedis.HSet(schema.cachePrefix, cacheKey, cacheNilValue)
		}
		return
	}
	if hasLocalCache {
		cacheLocal.Set(cacheKey, entity.getORM().value)
	}
	if hasRedis {
		cacheRedis.HSet(schema.cachePrefix, cacheKey, string(entity.getORM().binary))
	}
	return
}

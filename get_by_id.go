package beeorm

import (
	"reflect"
	"strconv"
)

func GetByID[E Entity](c Context, id uint64) (entity E) {
	entity = getByID[E](c.(*contextImplementation), id, nil)
	return
}

func getByID[E Entity](c *contextImplementation, id uint64, entityToFill Entity) (entity E) {
	schema := c.engine.registry.entitySchemas[reflect.TypeOf(entity)]
	if schema.hasLocalCache {
		e, has := schema.localCache.getEntity(c, id)
		if has {
			if e == emptyReflect {
				return
			}
			entity = e.Interface().(E)
			return
		}
	}
	cacheRedis, hasRedis := schema.GetRedisCache()
	var cacheKey string
	if hasRedis {
		cacheKey = schema.GetCacheKey() + ":" + strconv.FormatUint(id, 10)
		row := cacheRedis.LRange(c, cacheKey, 0, int64(len(schema.columnNames)+1))
		l := len(row)
		if len(row) > 0 {
			if l == 1 {
				if schema.hasLocalCache {
					schema.localCache.setEntity(c, id, emptyReflect)
				}
				return
			}
			var value reflect.Value
			if entityToFill == nil {
				value = reflect.New(schema.tElem)
				entity = value.Interface().(E)
			} else {
				entity = entityToFill.(E)
				value = reflect.ValueOf(entity)
			}
			if deserializeFromRedis(row, schema, value.Elem()) {
				if schema.hasLocalCache {
					schema.localCache.setEntity(c, id, value)
				}
				return
			}
		}
	}
	entity, found := searchRow[E](c, NewWhere("`ID` = ?", id), nil, false)
	if !found {
		if schema.hasLocalCache {
			schema.localCache.setEntity(c, id, emptyReflect)
		}
		if hasRedis {
			p := c.RedisPipeLine(cacheRedis.GetCode())
			p.Del(c, cacheKey)
			p.RPush(c, cacheKey, cacheNilValue)
			p.Exec(c)
		}
		return
	}
	if schema.hasLocalCache {
		schema.localCache.setEntity(c, id, reflect.ValueOf(entity))
	}
	if hasRedis {
		bind := make(Bind)
		err := fillBindFromOneSource(c, bind, reflect.ValueOf(entity).Elem(), schema.fields, "")
		if err != nil {
			panic(err)
		}
		values := convertBindToRedisValue(bind, schema)
		cacheRedis.RPush(c, cacheKey, values...)
	}
	return
}

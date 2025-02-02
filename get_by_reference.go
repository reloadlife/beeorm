package beeorm

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
)

const redisValidSetValue = "Y"

func GetByReference[E any](orm ORM, referenceName string, id uint64) EntityIterator[E] {
	if id == 0 {
		return nil
	}
	var e E
	schema := orm.(*ormImplementation).engine.registry.entitySchemas[reflect.TypeOf(e)]
	if schema == nil {
		panic(fmt.Errorf("entity '%T' is not registered", e))
	}
	def, has := schema.references[referenceName]
	if !has {
		panic(fmt.Errorf("unknow reference name `%s`", referenceName))
	}
	lc, hasLocalCache := schema.GetLocalCache()
	if !def.Cached {
		return Search[E](orm, NewWhere("`"+referenceName+"` = ?", id), nil)
	}
	defSchema := orm.Engine().Registry().EntitySchema(def.Type).(*entitySchema)
	return getCachedList[E](orm, referenceName, id, hasLocalCache, lc, schema, defSchema)
}

func getCachedList[E any](orm ORM, referenceName string, id uint64, hasLocalCache bool, lc LocalCache, schema, resultSchema *entitySchema) EntityIterator[E] {
	if hasLocalCache {
		fromCache, hasInCache := lc.getReference(orm, referenceName, id)
		if hasInCache {
			if fromCache == cacheNilValue {
				return &emptyResultsIterator[E]{}
			}

			if resultSchema.hasLocalCache {
				results := &entityIterator[E]{index: -1}
				results.rows = fromCache.([]*E)
				return results
			}
			return GetByIDs[E](orm, fromCache.([]uint64)...)
		}
	}
	rc := orm.Engine().Redis(schema.getForcedRedisCode())
	redisSetKey := schema.cacheKey + ":" + referenceName
	if id > 0 {
		idAsString := strconv.FormatUint(id, 10)
		redisSetKey += ":" + idAsString
	}
	fromRedis := rc.SMembers(orm, redisSetKey)
	if len(fromRedis) > 0 {
		ids := make([]uint64, len(fromRedis))
		k := 0
		hasValidValue := false
		for _, value := range fromRedis {
			if value == redisValidSetValue {
				hasValidValue = true
				continue
			} else if value == cacheNilValue {
				continue
			}
			ids[k], _ = strconv.ParseUint(value, 10, 64)
			k++
		}
		if hasValidValue {
			if k == 0 {
				if hasLocalCache {
					lc.setReference(orm, referenceName, id, cacheNilValue)
				}
				return &emptyResultsIterator[E]{}
			}
			ids = ids[0:k]
			slices.Sort(ids)
			values := GetByIDs[E](orm, ids...)
			if hasLocalCache {
				if values.Len() == 0 {
					lc.setReference(orm, referenceName, id, cacheNilValue)
				} else {
					if resultSchema.hasLocalCache {
						lc.setReference(orm, referenceName, id, values.All())
					} else {
						lc.setReference(orm, referenceName, id, ids)
					}
				}
			}
			return values
		}
	}
	if hasLocalCache {
		var where Where
		if id > 0 {
			where = NewWhere("`"+referenceName+"` = ?", id)
		} else {
			where = allEntitiesWhere
		}
		ids := SearchIDs[E](orm, where, nil)
		if len(ids) == 0 {
			lc.setReference(orm, referenceName, id, cacheNilValue)
			rc.SAdd(orm, redisSetKey, cacheNilValue)
			return &emptyResultsIterator[E]{}
		}
		idsForRedis := make([]any, len(ids))
		for i, value := range ids {
			idsForRedis[i] = strconv.FormatUint(value, 10)
		}
		p := orm.RedisPipeLine(rc.GetCode())
		p.Del(redisSetKey)
		p.SAdd(redisSetKey, redisValidSetValue)
		p.SAdd(redisSetKey, idsForRedis...)
		p.Exec(orm)
		values := GetByIDs[E](orm, ids...)
		if resultSchema.hasLocalCache {
			lc.setReference(orm, referenceName, id, values.All())
		} else {
			lc.setReference(orm, referenceName, id, ids)
		}
		return values
	}
	var where Where
	if id > 0 {
		where = NewWhere("`"+referenceName+"` = ?", id)
	} else {
		where = allEntitiesWhere
	}
	values := Search[E](orm, where, nil)
	if values.Len() == 0 {
		rc.SAdd(orm, redisSetKey, redisValidSetValue, cacheNilValue)
	} else {
		idsForRedis := make([]any, values.Len()+1)
		idsForRedis[0] = redisValidSetValue
		i := 0
		for values.Next() {
			idsForRedis[i+1] = strconv.FormatUint(reflect.ValueOf(values.Entity()).Elem().Field(0).Uint(), 10)
			i++
		}
		values.Reset()
		rc.SAdd(orm, redisSetKey, idsForRedis...)
	}
	return values
}

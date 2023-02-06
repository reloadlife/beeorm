package log_table

import (
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/latolukasz/beeorm/v2/plugins/crud_stream"

	"github.com/go-sql-driver/mysql"
	"github.com/latolukasz/beeorm/v2"

	jsoniter "github.com/json-iterator/go"
)

const PluginCode = "github.com/latolukasz/beeorm/plugins/log_table"
const ConsumerGroupName = "log-tables-consumer"
const defaultTagName = "log-table"
const poolOption = "pool"
const tableNameOption = "log-table"

type Plugin struct {
	options *Options
}
type Options struct {
	TagName          string
	DefaultMySQLPool string
}

func Init(options *Options) *Plugin {
	if options == nil {
		options = &Options{}
	}
	if options.DefaultMySQLPool == "" {
		options.DefaultMySQLPool = "default"
	}
	if options.TagName == "" {
		options.TagName = defaultTagName
	}
	return &Plugin{options}
}

func (p *Plugin) GetCode() string {
	return PluginCode
}

func (p *Plugin) PluginInterfaceInitRegistry(registry *beeorm.Registry) {
	registry.RegisterRedisStreamConsumerGroups(crud_stream.ChannelName, ConsumerGroupName)
}

func (p *Plugin) InterfaceInitTableSchema(schema beeorm.SettableTableSchema, _ *beeorm.Registry) error {
	logPoolName := schema.GetTag("ORM", p.options.TagName, p.options.DefaultMySQLPool, "")
	if logPoolName == "" {
		return nil
	}
	schema.SetOption(PluginCode, poolOption, logPoolName)
	schema.SetOption(PluginCode, tableNameOption, fmt.Sprintf("_log_%s_%s", logPoolName, schema.GetTableName()))
	return nil
}

func (p *Plugin) PluginInterfaceSchemaCheck(engine beeorm.Engine, schema beeorm.TableSchema) (alters []beeorm.Alter, keepTables map[string][]string) {
	poolName := schema.GetOptionString(PluginCode, poolOption)
	if poolName == "" {
		return nil, nil
	}
	tableName := schema.GetOptionString(PluginCode, tableNameOption)
	db := engine.GetMysql(poolName)
	var tableDef string
	hasLogTable := db.QueryRow(beeorm.NewWhere(fmt.Sprintf("SHOW TABLES LIKE '%s'", tableName)), &tableDef)
	var logTableSchema string
	if db.GetPoolConfig().GetVersion() == 5 {
		logTableSchema = fmt.Sprintf("CREATE TABLE `%s`.`%s` (\n  `id` bigint(11) unsigned NOT NULL AUTO_INCREMENT,\n  "+
			"`entity_id` int(10) unsigned NOT NULL,\n  `added_at` datetime NOT NULL,\n  `meta` json DEFAULT NULL,\n  `before` json DEFAULT NULL,\n  `changes` json DEFAULT NULL,\n  "+
			"PRIMARY KEY (`id`),\n  KEY `entity_id` (`entity_id`)\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 ROW_FORMAT=COMPRESSED KEY_BLOCK_SIZE=8;",
			db.GetPoolConfig().GetDatabase(), tableName)
	} else {
		logTableSchema = fmt.Sprintf("CREATE TABLE `%s`.`%s` (\n  `id` bigint unsigned NOT NULL AUTO_INCREMENT,\n  "+
			"`entity_id` int unsigned NOT NULL,\n  `added_at` datetime NOT NULL,\n  `meta` json DEFAULT NULL,\n  `before` json DEFAULT NULL,\n  `changes` json DEFAULT NULL,\n  "+
			"PRIMARY KEY (`id`),\n  KEY `entity_id` (`entity_id`)\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_%s ROW_FORMAT=COMPRESSED KEY_BLOCK_SIZE=8;",
			db.GetPoolConfig().GetDatabase(), tableName, engine.GetRegistry().GetSourceRegistry().GetDefaultCollate())
	}

	if !hasLogTable {
		alters = append(alters, beeorm.Alter{SQL: logTableSchema, Safe: true, Pool: poolName})
	} else {
		var skip, createTableDB string
		db.QueryRow(beeorm.NewWhere(fmt.Sprintf("SHOW CREATE TABLE `%s`", tableName)), &skip, &createTableDB)
		createTableDB = strings.Replace(createTableDB, "CREATE TABLE ", fmt.Sprintf("CREATE TABLE `%s`.", db.GetPoolConfig().GetDatabase()), 1) + ";"
		re := regexp.MustCompile(" AUTO_INCREMENT=[0-9]+ ")
		createTableDB = re.ReplaceAllString(createTableDB, " ")
		if logTableSchema != createTableDB {
			db.QueryRow(beeorm.NewWhere("1"))
			isEmpty := !db.QueryRow(beeorm.NewWhere(fmt.Sprintf("SELECT ID FROM `%s`", tableName)))
			dropTableSQL := fmt.Sprintf("DROP TABLE `%s`.`%s`;", db.GetPoolConfig().GetDatabase(), tableName)
			alters = append(alters, beeorm.Alter{SQL: dropTableSQL, Safe: isEmpty, Pool: poolName})
			alters = append(alters, beeorm.Alter{SQL: logTableSchema, Safe: true, Pool: poolName})
		}
	}
	return alters, map[string][]string{poolName: {tableName}}
}

type logEvent struct {
	crudEvent *crud_stream.CrudEvent
	source    beeorm.Event
}

func NewEventHandler(engine beeorm.Engine) beeorm.EventConsumerHandler {
	return func(events []beeorm.Event) {
		values := make(map[string][]*logEvent)
		for _, event := range events {
			var data crud_stream.CrudEvent
			event.Unserialize(&data)
			schema := engine.GetRegistry().GetTableSchema(data.EntityName)
			if schema == nil {
				continue
			}
			poolName := schema.GetOptionString(PluginCode, poolOption)
			_, has := values[poolName]
			if !has {
				values[poolName] = make([]*logEvent, 0)
			}
			values[poolName] = append(values[poolName], &logEvent{crudEvent: &data, source: event})
		}
		handleLogEvents(engine, values)
	}
}

type EntityLog struct {
	LogID    uint64
	EntityID uint64
	Date     time.Time
	Meta     map[string]interface{}
	Before   map[string]interface{}
	Changes  map[string]interface{}
}

func GetEntityLogs(engine beeorm.Engine, tableSchema beeorm.TableSchema, entityID uint64, pager *beeorm.Pager, where *beeorm.Where) []EntityLog {
	var results []EntityLog
	poolName := tableSchema.GetOptionString(PluginCode, poolOption)
	if poolName == "" {
		return results
	}
	db := engine.GetMysql(poolName)
	if pager == nil {
		pager = beeorm.NewPager(1, 1000)
	}
	if where == nil {
		where = beeorm.NewWhere("1")
	}
	tableName := tableSchema.GetOptionString(PluginCode, tableNameOption)
	fullQuery := "SELECT `id`, `added_at`, `meta`, `before`, `changes` FROM " + tableName + " WHERE "
	fullQuery += "entity_id = " + strconv.FormatUint(entityID, 10) + " "
	fullQuery += "AND " + where.String() + " " + pager.String()
	rows, closeF := db.Query(fullQuery, where.GetParameters()...)
	defer closeF()
	id := uint64(0)
	addedAt := ""
	meta := sql.NullString{}
	before := sql.NullString{}
	changes := sql.NullString{}
	for rows.Next() {
		rows.Scan(&id, &addedAt, &meta, &before, &changes)
		log := EntityLog{}
		log.LogID = id
		log.EntityID = entityID
		if meta.Valid {
			err := jsoniter.ConfigFastest.UnmarshalFromString(meta.String, &log.Meta)
			if err != nil {
				panic(err)
			}
		}
		if before.Valid {
			err := jsoniter.ConfigFastest.UnmarshalFromString(before.String, &log.Before)
			if err != nil {
				panic(err)
			}
		}
		if changes.Valid {
			err := jsoniter.ConfigFastest.UnmarshalFromString(changes.String, &log.Changes)
			if err != nil {
				panic(err)
			}
		}
		results = append(results, log)
	}
	return results
}

func handleLogEvents(engine beeorm.Engine, values map[string][]*logEvent) {
	for poolName, rows := range values {
		poolDB := engine.GetMysql(poolName)
		if len(rows) > 1 {
			poolDB.Begin()
		}
		func() {
			defer poolDB.Rollback()
			for _, value := range rows {
				schema := engine.GetRegistry().GetTableSchema(value.crudEvent.EntityName)
				tableName := schema.GetOptionString(PluginCode, tableNameOption)
				query := "INSERT INTO `" + tableName + "`(`entity_id`, `added_at`, `meta`, `before`, `changes`) VALUES(?, ?, ?, ?, ?)"
				params := make([]interface{}, 5)
				params[0] = value.crudEvent.ID
				params[1] = value.crudEvent.Updated.Format(beeorm.TimeFormat)
				meta := value.source.Meta()
				if len(meta) > 0 {
					params[2], _ = jsoniter.ConfigFastest.MarshalToString(meta)
				}
				if len(value.crudEvent.Before) > 0 {
					params[3], _ = jsoniter.ConfigFastest.MarshalToString(value.crudEvent.Before)
				}
				if len(value.crudEvent.Changes) > 0 {
					params[4], _ = jsoniter.ConfigFastest.MarshalToString(value.crudEvent.Changes)
				}
				func() {
					defer func() {
						if rec := recover(); rec != nil {
							asMySQLError, isMySQLError := rec.(*mysql.MySQLError)
							if isMySQLError && asMySQLError.Number == 1146 { // table was removed
								return
							}
							panic(rec)
						}
					}()
					poolDB.Exec(query, params...)
				}()
			}
			if poolDB.IsInTransaction() {
				poolDB.Commit()
			}
		}()
	}
}

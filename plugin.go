package beeorm

type Plugin interface {
	GetCode() string
}

type PluginInterfaceInitRegistry interface {
	PluginInterfaceInitRegistry(registry *Registry)
}

type PluginInterfaceInitTableSchema interface {
	InterfaceInitTableSchema(schema SettableTableSchema, registry *Registry) error
}

type PluginInterfaceSchemaCheck interface {
	PluginInterfaceSchemaCheck(engine Engine, schema TableSchema) (alters []Alter, keepTables map[string][]string)
}

type PluginInterfaceEntityFlushed interface {
	PluginInterfaceEntityFlushed(engine Engine, data *EntitySQLFlush, cacheFlusher FlusherCacheSetter)
}

package hooks

type HookType string

const (
	//Database lifecycle hooks
	HookDatabaseStart HookType = "database.start"
	HookDatabaseStop  HookType = "database.stop"

	//Connection hooks
	HookConnectionOpen  HookType = "connection.open"
	HookConnectionClose HookType = "connection.close"

	//Transaction hooks
	HookTransactionBegin    HookType = "transaction.begin"
	HookTransactionCommit   HookType = "transaction.commit"
	HookTransactionRollback HookType = "transaction.rollback"

	//Query execution hooks
	HookQueryExecute HookType = "query.execute"
	HookQueryParse   HookType = "query.parse"
	HookQueryPlan    HookType = "query.plan"
	HookQueryResult  HookType = "query.result"

	//Schema hooks
	HookTableCreate HookType = "table.create"
	HookTableDrop   HookType = "table.drop"
	HookIndexCreate HookType = "index.create"
	HookIndexDrop   HookType = "index.drop"

	//Data modification hooks
	HookRowInsert HookType = "row.insert"
	HookRowUpdate HookType = "row.update"
	HookRowDelete HookType = "row.delete"

	//Authentication hooks
	HookAuthAttempt HookType = "auth.attempt"
	HookAuthSuccess HookType = "auth.success"
	HookAuthFailure HookType = "auth.failure"

	//Plugin hooks
	HookPluginLoad   HookType = "plugin.load"
	HookPluginUnload HookType = "plugin.unload"
)

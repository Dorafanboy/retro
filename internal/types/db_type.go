package types

// DBType defines the type of database used.
type DBType string

const (
	Postgres DBType = "postgres"
	SQLite   DBType = "sqlite"
	None     DBType = "none"
)

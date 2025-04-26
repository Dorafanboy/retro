package types

// DBType defines the type of database used.
type DBType string

const (
	// Postgres represents the PostgreSQL database.
	Postgres DBType = "postgres"
	//SQLite represents the SQLite database.
	SQLite DBType = "sqlite"
	// None indicates no database is used.
	None DBType = "none"
)

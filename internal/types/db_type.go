package types

// DBType определяет тип используемой базы данных.
// EN: DBType defines the type of database used.
type DBType string

const (
	// Postgres представляет базу данных PostgreSQL.
	// EN: Postgres represents the PostgreSQL database.
	Postgres DBType = "postgres"
	// SQLite представляет базу данных SQLite.
	// EN: SQLite represents the SQLite database.
	SQLite DBType = "sqlite"
	// None указывает на отсутствие базы данных.
	// EN: None indicates no database is used.
	None DBType = "none"
)

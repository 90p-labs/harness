package dbal

type Engine string

const EngineSQLite Engine = "sqlite"

type Dialect interface {
	Engine() Engine
	DriverName() string
	DriverDSN(path string) string
	IsUniqueViolation(err error) bool
}

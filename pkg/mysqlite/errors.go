package mysqlite

import (
	"github.com/mattn/go-sqlite3"
	"github.com/samber/lo"

	"go-backend/pkg/myerr"
)

// IsUniqueViolation reports whether err is a SQLite UNIQUE/PRIMARY KEY constraint failure.
func IsUniqueViolation(err error) bool {
	sqlErr, casted := lo.ErrorsAs[sqlite3.Error](err)
	if !casted {
		return false
	}

	return sqlErr.ExtendedCode == sqlite3.ErrConstraintUnique ||
		sqlErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey
}

// IsForeignKeyViolation reports whether err is a SQLite FOREIGN KEY constraint failure.
func IsForeignKeyViolation(err error) bool {
	sqlErr, casted := lo.ErrorsAs[sqlite3.Error](err)
	if !casted {
		return false
	}

	return sqlErr.ExtendedCode == sqlite3.ErrConstraintForeignKey
}

// GetType maps a SQLite driver error onto one of the myerr sentinels, so services and
// the REST layer can turn it into a meaningful status code. Errors that did not come
// from the driver are returned untouched.
func GetType(err error) error {
	switch {
	case IsUniqueViolation(err):
		return myerr.ErrAlreadyExists
	case IsForeignKeyViolation(err):
		return myerr.ErrNotFound
	}

	if _, casted := lo.ErrorsAs[sqlite3.Error](err); casted {
		return myerr.ErrInternal
	}

	return err
}

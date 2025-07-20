package mymysql

import (
	"github.com/go-sql-driver/mysql"
	"github.com/samber/lo"

	"go-backend/pkg/myerr"
)

const DublicateEntryNumber uint16 = 1062

const ForeignKeyViolation uint16 = 1452

func GetType(err error) error {
	if sqlErr, casted := lo.ErrorsAs[*mysql.MySQLError](err); casted {
		if sqlErr.Number == DublicateEntryNumber {
			return myerr.ErrAlreadyExists
		}
		if sqlErr.Number == ForeignKeyViolation {
			return myerr.ErrNotFound
		}
		return myerr.ErrInternal
	}

	return err
}

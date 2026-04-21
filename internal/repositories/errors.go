package repositories

import (
	"errors"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

const mysqlErrDup uint16 = 1062

func isDuplicate(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == mysqlErrDup
}

func isNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

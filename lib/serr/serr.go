package serr

import (
	"errors"
	"fmt"
	"log/slog"

	"gorm.io/gorm"
)

func Ferr(op string, err string) error {
	return fmt.Errorf("%s: %s", op, err)
}

func LogFerr(err error, op, logErr string, log *slog.Logger) (bool, error) {
	if err != nil {
		log.Error(
			logErr,
			slog.String("error", err.Error()))
		return false, Ferr(op, logErr)
	}
	return true, nil
}

func Gerr(op, errNotFound, errStd string, log *slog.Logger, rowsError error) (bool, error) {
	if rowsError != nil {
		if errors.Is(rowsError, gorm.ErrRecordNotFound) {
			log.Error(
				errNotFound,
				slog.String("op", op),
				slog.String("error", rowsError.Error()))
			return false, Ferr(op, errStd)
		}
		return false, Ferr(op, errStd)
	}

	return true, nil
}

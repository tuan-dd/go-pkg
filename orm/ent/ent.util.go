package entOrm

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/casbin/ent-adapter/ent"
	"github.com/go-sql-driver/mysql"
	"github.com/tuan-dd/go-common/response"
)

func EntMysqlToAppErr(err error, messagePrefix string) *response.AppError {
	if err == nil {
		return nil
	}

	// 1. Check for ent.NotFoundError
	if ent.IsNotFound(err) {
		return response.NotFoundErr(messagePrefix + " not found")
	}
	slog.Warn("ent error encountered", "error", err)
	// 2. Check for ent.ValidationError
	if ent.IsValidationError(err) {
		return response.QueryInvalidErr(messagePrefix + ": Validation failed.")
	}

	// 3. Check for ent.ConstraintError (most complex path)
	if ent.IsConstraintError(err) {
		underlyingErr := errors.Unwrap(err)
		if underlyingErr == nil { // Should not happen if IsConstraintError is true, but good practice
			underlyingErr = err
		}

		// Attempt to unwrap to *mysql.MySQLError (MySQL)
		var mysqlErr *mysql.MySQLError
		if errors.As(underlyingErr, &mysqlErr) {
			switch mysqlErr.Number {
			case 1062: // ER_DUP_ENTRY
				return response.Duplicate(messagePrefix + " already exists")
			case 1451: // ER_ROW_IS_REFERENCED_2 (cannot delete/update parent)
				return response.Conflict(messagePrefix + " cannot be modified because it is being used by other records")
			case 1452: // ER_NO_REFERENCED_ROW_2 (cannot add/update child)
				return response.QueryInvalidErr(messagePrefix + " contains invalid reference data")
			case 1048: // ER_BAD_NULL_ERROR
				return response.QueryInvalidErr(messagePrefix + " is missing required information")
			case 1216, 1217: // ER_NO_REFERENCED_ROW_1, ER_ROW_IS_REFERENCED
				return response.QueryInvalidErr(messagePrefix + " contains invalid reference data")
			case 1205, 1213: // ER_LOCK_DEADLOCK
				slog.Error("Database deadlock occurred", "error", mysqlErr)
				return response.ServerError("Operation temporarily unavailable, please try again")
			default:
				slog.Error("Database constraint error", "error", mysqlErr)
				return response.ServerError("Unable to complete the operation")
			}
		}
	}

	// 4. Check for ent.NotLoadedError
	if ent.IsNotLoaded(err) {
		// This is typically a server-side logic error.
		// Log it for developer attention.
		var nle *ent.NotLoadedError
		edgeName := "unknown edge"
		if errors.As(err, &nle) {
			edgeName = nle.Error()
		}
		developerMsg := fmt.Sprintf("Server logic error: edge '%s' was not loaded before access.", edgeName)
		slog.Error(developerMsg, "error", err)
		return response.ServerError("")
	}

	// 5. Check for ent.NotSingularError
	if ent.IsNotSingular(err) {
		slog.Warn("NotSingularError encountered", "error", err)
		return response.NotFoundErr(messagePrefix + ": The requested item is not uniquely identifiable or not found.")
	}

	// Fallback for other/unknown errors (as in user's original function)
	return response.ConvertDatabaseError(err)
}

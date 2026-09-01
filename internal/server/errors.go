package server

import (
	"context"
	"errors"
	"strings"

	"github.com/Muxcore-Media/core/pkg/contracts"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func mapDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, err.Error())
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, err.Error())
	}
	if errors.Is(err, contracts.ErrMigrationTargetNotFound) {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	if isClientPGError(err) {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	return status.Error(codes.Internal, err.Error())
}

func isClientPGError(err error) bool {
	msg := strings.ToLower(err.Error())
	clientPatterns := []string{
		"postgres exec:",
		"postgres query:",
		"duplicate key",
		"unique constraint",
		"foreign key constraint",
		"not-null constraint",
		"check constraint",
		"syntax error",
		"invalid input syntax",
		"undefined table",
		"undefined column",
		"column reference",
		"invalid column",
		"operator does not exist",
	}
	for _, p := range clientPatterns {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
}

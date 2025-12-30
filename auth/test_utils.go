package auth

import (
	"context"
)

// Helper functions for tests

func createTestContextFunc() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

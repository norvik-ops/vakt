package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorkerConfigLoads(t *testing.T) {
	srv, mux := buildServer(nil)
	assert.NotNil(t, srv, "asynq server must not be nil")
	assert.NotNil(t, mux, "asynq mux must not be nil")
}

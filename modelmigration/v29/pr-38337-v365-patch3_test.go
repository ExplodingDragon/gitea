// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm/schemas"
)

type codespaceBeforeQueuedTimeoutIndexRemoval struct {
	ID                    int64
	OperationStartedUnix  int64
	OperationCreatedUnix  int64
	OperationDeadlineUnix int64
}

func (*codespaceBeforeQueuedTimeoutIndexRemoval) TableName() string {
	return "codespace"
}

func (*codespaceBeforeQueuedTimeoutIndexRemoval) TableIndices() []*schemas.Index {
	queuedTimeout := schemas.NewIndex("queued_timeout", schemas.IndexType)
	queuedTimeout.AddColumn("operation_started_unix", "operation_created_unix", "id")
	runningTimeout := schemas.NewIndex("running_timeout", schemas.IndexType)
	runningTimeout.AddColumn("operation_deadline_unix", "id")
	return []*schemas.Index{queuedTimeout, runningTimeout}
}

func TestDropCodespaceQueuedTimeoutIndex(t *testing.T) {
	x, cleanup := migrationtest.PrepareTestEnv(t, 0, new(codespaceBeforeQueuedTimeoutIndexRemoval))
	defer cleanup()
	if x == nil || t.Failed() {
		return
	}

	row := &codespaceBeforeQueuedTimeoutIndexRemoval{
		OperationStartedUnix:  10,
		OperationCreatedUnix:  20,
		OperationDeadlineUnix: 30,
	}
	_, err := x.Insert(row)
	require.NoError(t, err)

	indexes, err := x.Dialect().GetIndexes(x.DB(), t.Context(), "codespace")
	require.NoError(t, err)
	require.Contains(t, indexes, "queued_timeout")
	require.Contains(t, indexes, "running_timeout")

	require.NoError(t, DropCodespaceQueuedTimeoutIndex(t.Context(), x))
	require.NoError(t, DropCodespaceQueuedTimeoutIndex(t.Context(), x))

	indexes, err = x.Dialect().GetIndexes(x.DB(), t.Context(), "codespace")
	require.NoError(t, err)
	assert.NotContains(t, indexes, "queued_timeout")
	assert.Contains(t, indexes, "running_timeout")

	var stored codespaceBeforeQueuedTimeoutIndexRemoval
	found, err := x.ID(row.ID).Get(&stored)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, row.OperationStartedUnix, stored.OperationStartedUnix)
	assert.Equal(t, row.OperationCreatedUnix, stored.OperationCreatedUnix)
	assert.Equal(t, row.OperationDeadlineUnix, stored.OperationDeadlineUnix)
}

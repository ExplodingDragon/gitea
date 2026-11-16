// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package modelmigration

import (
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrations(t *testing.T) {
	defer test.MockVariableValue(&preparedMigrations)()
	preparedMigrations = []*migration{
		{idNumber: 70},
		{idNumber: 71},
	}
	assert.EqualValues(t, 72, calcDBVersion(preparedMigrations))
	assert.EqualValues(t, 72, ExpectedDBVersion())

	assert.EqualValues(t, 71, migrationIDNumberToDBVersion(70))

	assert.Equal(t, []*migration{{idNumber: 70}, {idNumber: 71}}, getPendingMigrations(70, preparedMigrations))
	assert.Equal(t, []*migration{{idNumber: 71}}, getPendingMigrations(71, preparedMigrations))
	assert.Equal(t, []*migration{}, getPendingMigrations(72, preparedMigrations))
}

func TestSelfMigrationTail(t *testing.T) {
	defer test.MockVariableValue(&preparedMigrations, nil)()
	migrations := prepareMigrationTasks()

	assert.EqualValues(t, 374, calcDBVersion(migrations))

	pending := getPendingMigrations(372, migrations)
	require.Len(t, pending, 2)
	assert.EqualValues(t, 372, pending[0].idNumber)
	assert.Equal(t, "Add unique index for team name", pending[0].description)
	assert.EqualValues(t, 373, pending[1].idNumber)
	assert.Equal(t, "Add runner_name to action_task", pending[1].description)

	require.Len(t, getPendingMigrations(373, migrations), 1)
	assert.Empty(t, getPendingMigrations(374, migrations))
}

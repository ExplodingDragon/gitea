// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package modelmigration

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type teamBeforeSelfMigration372 struct {
	ID              int64 `xorm:"pk autoincr"`
	OrgID           int64 `xorm:"INDEX"`
	LowerName, Name string
}

func (*teamBeforeSelfMigration372) TableName() string {
	return "team"
}

type actionTaskBeforeSelfMigration373 struct {
	ID       int64 `xorm:"pk"`
	RunnerID int64 `xorm:"index"`
}

func (*actionTaskBeforeSelfMigration373) TableName() string {
	return "action_task"
}

type actionRunnerBeforeSelfMigration373 struct {
	ID   int64  `xorm:"pk"`
	Name string `xorm:"VARCHAR(255)"`
}

func (*actionRunnerBeforeSelfMigration373) TableName() string {
	return "action_runner"
}

func TestMain(m *testing.M) {
	migrationtest.MainTest(m)
}

func TestUpgradeSelfDatabaseFrom372(t *testing.T) {
	x, cleanup := migrationtest.PrepareTestEnv(t, 0,
		new(Version),
		new(teamBeforeSelfMigration372),
		new(actionTaskBeforeSelfMigration373),
		new(actionRunnerBeforeSelfMigration373),
	)
	defer cleanup()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(&Version{ID: 1, Version: 372})
	require.NoError(t, err)
	_, err = x.Insert(
		&teamBeforeSelfMigration372{OrgID: 1, Name: "Team", LowerName: "team"},
		&teamBeforeSelfMigration372{OrgID: 1, Name: "Team", LowerName: "team"},
	)
	require.NoError(t, err)
	_, err = x.Insert(&actionRunnerBeforeSelfMigration373{ID: 1, Name: "self-runner"})
	require.NoError(t, err)
	_, err = x.Insert(&actionTaskBeforeSelfMigration373{ID: 1, RunnerID: 1})
	require.NoError(t, err)

	require.NoError(t, Migrate(t.Context(), x))

	version := new(Version)
	found, err := x.ID(1).Get(version)
	require.NoError(t, err)
	require.True(t, found)
	assert.EqualValues(t, 374, version.Version)

	var teams []teamBeforeSelfMigration372
	require.NoError(t, x.Asc("id").Find(&teams))
	require.Len(t, teams, 2)
	assert.Equal(t, "team", teams[0].LowerName)
	assert.Equal(t, "team-2", teams[1].LowerName)

	var runnerNames []string
	require.NoError(t, x.Table("action_task").Cols("runner_name").Find(&runnerNames))
	assert.Equal(t, []string{"self-runner"}, runnerNames)
}

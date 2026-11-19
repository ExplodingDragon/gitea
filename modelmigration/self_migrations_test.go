// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package modelmigration

import (
	"testing"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modelmigration/migrationtest"
	"gitea.dev/modelmigration/v29"
	codespace_model "gitea.dev/models/codespace"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm/schemas"
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

func prepareSelfCodespaceDatabase(t *testing.T, x base.EngineMigration) {
	t.Helper()
	require.NoError(t, v29.AddCodespaceTables(t.Context(), x))
	require.NoError(t, v29.AddCodespaceManagerRegistrationTokenTable(t.Context(), x))
	require.NoError(t, v29.DropCodespaceQueuedTimeoutIndex(t.Context(), x))
	type PublicKey struct {
		ID   int64 `xorm:"pk autoincr"`
		Type int
	}
	type DbfsMeta struct {
		ID       int64 `xorm:"pk autoincr"`
		FullPath string
	}
	type DbfsData struct {
		ID     int64 `xorm:"pk autoincr"`
		MetaID int64
	}
	require.NoError(t, x.Sync(new(PublicKey), new(DbfsMeta), new(DbfsData)))
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
	prepareSelfCodespaceDatabase(t, x)

	_, err := x.Insert(&Version{Version: 372})
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
	assert.EqualValues(t, 375, version.Version)

	var teams []teamBeforeSelfMigration372
	require.NoError(t, x.Asc("id").Find(&teams))
	require.Len(t, teams, 2)
	assert.Equal(t, "team", teams[0].LowerName)
	assert.Equal(t, "team-2", teams[1].LowerName)

	var runnerNames []string
	require.NoError(t, x.Table("action_task").Cols("runner_name").Find(&runnerNames))
	assert.Equal(t, []string{"self-runner"}, runnerNames)
}

func TestUpgradeSelfDatabaseFrom374(t *testing.T) {
	x, cleanup := migrationtest.PrepareTestEnv(t, 0, new(Version))
	defer cleanup()
	if x == nil || t.Failed() {
		return
	}
	prepareSelfCodespaceDatabase(t, x)
	_, err := x.Insert(&Version{Version: 374})
	require.NoError(t, err)
	_, err = x.Table("codespace").Insert(map[string]any{
		"uuid": "12345678-1234-4234-8234-123456789012", "user_id": 1, "repo_id": 1,
		"ref_name": "main", "environment_tag": "standard", "dev_container_content": `{}`, "status": "stopped",
	})
	require.NoError(t, err)

	require.NoError(t, Migrate(t.Context(), x))
	version, err := GetCurrentDBVersion(x)
	require.NoError(t, err)
	assert.EqualValues(t, 375, version)
	count, err := x.Table("codespace").Count()
	require.NoError(t, err)
	assert.Zero(t, count)

	newInstance := &codespace_model.Codespace{UserID: 1, RepoID: 1, Status: codespace_model.StatusCreating, DevContainerContent: `{}`, SecretsEncrypted: "new-instance-ciphertext"}
	_, err = x.Insert(newInstance)
	require.NoError(t, err)
	newToken := &codespace_model.GiteaToken{CodespaceID: newInstance.ID, TokenHash: "new-token-hash", TokenSalt: "salt", TokenLastEight: "12345678", TokenEncrypted: "ciphertext"}
	_, err = x.Insert(newToken)
	require.NoError(t, err)
	require.NoError(t, Migrate(t.Context(), x))
	var loaded codespace_model.Codespace
	found, err := x.ID(newInstance.ID).Get(&loaded)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, *newInstance, loaded)
	var loadedToken codespace_model.GiteaToken
	found, err = x.ID(newInstance.ID).Get(&loadedToken)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, *newToken, loadedToken)

	tables := migrationtest.LoadTableSchemasMap(t, x)
	for _, bean := range []any{new(codespace_model.Codespace), new(codespace_model.UserSecret)} {
		model, err := x.TableInfo(bean)
		require.NoError(t, err)
		actual := tables[model.Name]
		require.NotNil(t, actual)
		assert.ElementsMatch(t, model.ColumnsSeq(), actual.ColumnsSeq())
		assert.Equal(t, model.Indexes, actual.Indexes)
	}
}

func TestUpgradeSelfDatabaseFrom374RetriesFailedSchemaChange(t *testing.T) {
	x, cleanup := migrationtest.PrepareTestEnv(t, 0, new(Version))
	defer cleanup()
	if x == nil || t.Failed() {
		return
	}
	if x.Dialect().URI().DBType != schemas.SQLITE {
		t.Skip("SQLite view dependency exercises an interrupted DROP COLUMN")
	}
	prepareSelfCodespaceDatabase(t, x)
	_, err := x.Insert(&Version{Version: 374})
	require.NoError(t, err)
	_, err = x.Exec("CREATE VIEW self_migration374_blocker AS SELECT uuid FROM codespace")
	require.NoError(t, err)
	require.Error(t, Migrate(t.Context(), x))
	version, err := GetCurrentDBVersion(x)
	require.NoError(t, err)
	assert.EqualValues(t, 374, version)
	_, err = x.Exec("DROP VIEW self_migration374_blocker")
	require.NoError(t, err)
	require.NoError(t, Migrate(t.Context(), x))
	version, err = GetCurrentDBVersion(x)
	require.NoError(t, err)
	assert.EqualValues(t, 375, version)
}

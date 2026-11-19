// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"fmt"
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type codespaceResetPublicKey struct {
	ID   int64 `xorm:"pk autoincr"`
	Type int
	Name string
}

func (*codespaceResetPublicKey) TableName() string { return "public_key" }

type codespaceResetDBFSMeta struct {
	ID              int64  `xorm:"pk autoincr"`
	FullPath        string `xorm:"VARCHAR(500) UNIQUE NOT NULL"`
	BlockSize       int64
	FileSize        int64
	CreateTimestamp int64
	ModifyTimestamp int64
}

func (*codespaceResetDBFSMeta) TableName() string { return "dbfs_meta" }

type codespaceResetDBFSData struct {
	ID         int64 `xorm:"pk autoincr"`
	Revision   int64
	MetaID     int64 `xorm:"index(meta_offset)"`
	BlobOffset int64 `xorm:"index(meta_offset)"`
	BlobSize   int64
	BlobData   []byte `xorm:"BLOB"`
}

func (*codespaceResetDBFSData) TableName() string { return "dbfs_data" }

func TestResetLegacyCodespaceInstancesAndUpdateSchema(t *testing.T) {
	x, cleanup := migrationtest.PrepareTestEnv(t, 0,
		new(codespaceResetPublicKey), new(codespaceResetDBFSMeta), new(codespaceResetDBFSData),
	)
	defer cleanup()
	if x == nil || t.Failed() {
		return
	}
	require.NoError(t, AddCodespaceTables(t.Context(), x))
	require.NoError(t, AddCodespaceManagerRegistrationTokenTable(t.Context(), x))
	require.NoError(t, DropCodespaceQueuedTimeoutIndex(t.Context(), x))

	var lastID int64
	for i, status := range []string{"creating", "running", "stopped", "failed", "deleting"} {
		instance := &codespace{UUID: fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1), UserID: 1, RepoID: 1, Status: status}
		_, err := x.Insert(instance)
		require.NoError(t, err)
		lastID = instance.ID
	}
	// A previously deleted highest ID must not be reused after the table becomes empty.
	_, err := x.ID(lastID).Delete(new(codespace))
	require.NoError(t, err)

	for _, keyType := range []int{1, 2, 3, 4, 4} {
		_, err := x.Insert(&codespaceResetPublicKey{Type: keyType, Name: fmt.Sprintf("key-%d", keyType)})
		require.NoError(t, err)
	}
	for table, row := range map[string]map[string]any{
		"codespace_gitea_token":                {"codespace_id": lastID, "token_hash": "hash", "token_salt": "salt", "token_last_eight": "12345678", "token_encrypted": "ciphertext"},
		"codespace_ssh_key":                    {"codespace_id": lastID, "key_id": 4},
		"codespace_open_token":                 {"code_hash": "open-code", "codespace_id": lastID, "user_id": 1, "manager_id": 1, "endpoint_id": "workspace", "expires_unix": 100},
		"codespace_user_secret":                {"user_id": 1, "name": "TOKEN", "data_encrypted": "personal-ciphertext", "data_size": 20, "all_repositories": true, "created_unix": 10, "updated_unix": 20},
		"codespace_user_secret_repository":     {"secret_id": 1, "repo_id": 1},
		"codespace_manager_registration_token": {"token": "registration-token", "user_id": 1, "is_active": true},
		"codespace_permission_authorization":   {"user_id": 1, "source_repo_id": 1, "request_hash": "request-hash"},
		"codespace_permission_repository":      {"authorization_id": 1, "target_repo_id": 2, "unit_type": 1, "requested_mode": 2, "granted_mode": 2},
	} {
		_, err := x.Table(table).Insert(row)
		require.NoError(t, err, table)
	}
	_, err = x.Insert(&codespaceManager{Name: "manager", SecretHash: "manager-hash", SecretSalt: "salt"})
	require.NoError(t, err)

	var keptMeta []codespaceResetDBFSMeta
	var keptData []codespaceResetDBFSData
	for i, path := range []string{
		"1:codespace_log/00000000-0000-4000-8000-000000000001.log",
		"1:codespace_log/orphan.log",
		"1:codespaceXlog/keep.log",
		"1:actions_log/keep.log",
		"1:codespace_log_other/keep.log",
	} {
		meta := codespaceResetDBFSMeta{FullPath: path, FileSize: 4, BlockSize: 4096}
		_, err := x.Insert(&meta)
		require.NoError(t, err)
		data := codespaceResetDBFSData{MetaID: meta.ID, BlobSize: 4, BlobData: []byte("logs")}
		_, err = x.Insert(&data)
		require.NoError(t, err)
		if i >= 2 {
			keptMeta = append(keptMeta, meta)
			keptData = append(keptData, data)
		}
	}
	kept := map[string][]map[string][]byte{}
	for _, table := range []string{"codespace_manager", "codespace_manager_registration_token", "codespace_permission_authorization", "codespace_permission_repository", "codespace_dev_container_template"} {
		kept[table], err = x.Query("SELECT * FROM " + table)
		require.NoError(t, err)
	}
	beforeIndexes, err := x.Dialect().GetIndexes(x.DB(), t.Context(), "codespace")
	require.NoError(t, err)
	for name, index := range beforeIndexes {
		if len(index.Cols) == 1 && index.Cols[0] == "uuid" {
			delete(beforeIndexes, name)
		}
	}
	secretIndexes, err := x.Dialect().GetIndexes(x.DB(), t.Context(), "codespace_user_secret")
	require.NoError(t, err)

	require.NoError(t, ResetLegacyCodespaceInstancesAndUpdateSchema(t.Context(), x))
	// A retry before version advancement must also accept already completed DDL.
	require.NoError(t, ResetLegacyCodespaceInstancesAndUpdateSchema(t.Context(), x))
	for _, table := range []string{"codespace", "codespace_gitea_token", "codespace_ssh_key", "codespace_open_token"} {
		count, err := x.Table(table).Count()
		require.NoError(t, err)
		assert.Zero(t, count, table)
	}
	for table, expected := range kept {
		actual, err := x.Query("SELECT * FROM " + table)
		require.NoError(t, err)
		assert.Equal(t, expected, actual, table)
	}
	var keyTypes []int
	require.NoError(t, x.Table("public_key").Asc("id").Cols("type").Find(&keyTypes))
	assert.Equal(t, []int{1, 2, 3}, keyTypes)
	var actualMeta []codespaceResetDBFSMeta
	var actualData []codespaceResetDBFSData
	require.NoError(t, x.Asc("id").Find(&actualMeta))
	require.NoError(t, x.Asc("id").Find(&actualData))
	assert.Equal(t, keptMeta, actualMeta)
	assert.Equal(t, keptData, actualData)

	tables := migrationtest.LoadTableSchemasMap(t, x)
	assert.Nil(t, tables["codespace"].GetColumn("uuid"))
	assert.NotNil(t, tables["codespace"].GetColumn("secrets_encrypted"))
	assert.Nil(t, tables["codespace_user_secret"].GetColumn("data_size"))
	assert.Nil(t, tables["codespace_user_secret"].GetColumn("all_repositories"))
	assert.NotContains(t, tables, "codespace_user_secret_repository")
	indexes, err := x.Dialect().GetIndexes(x.DB(), t.Context(), "codespace")
	require.NoError(t, err)
	assert.Equal(t, beforeIndexes, indexes)
	indexes, err = x.Dialect().GetIndexes(x.DB(), t.Context(), "codespace_user_secret")
	require.NoError(t, err)
	assert.Equal(t, secretIndexes, indexes)

	type CodespaceUserSecret struct {
		ID            int64
		UserID        int64
		Name          string
		DataEncrypted string
		CreatedUnix   int64
		UpdatedUnix   int64
	}
	var saved CodespaceUserSecret
	found, err := x.Get(&saved)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "personal-ciphertext", saved.DataEncrypted)
	assert.EqualValues(t, 10, saved.CreatedUnix)
	assert.EqualValues(t, 20, saved.UpdatedUnix)
	_, err = x.Insert(&CodespaceUserSecret{UserID: 1, Name: "TOKEN", DataEncrypted: "duplicate"})
	require.Error(t, err)

	type Codespace struct {
		ID                  int64
		RefName             string
		EnvironmentTag      string
		DevContainerContent string
		SecretsEncrypted    string
	}
	instance := &Codespace{SecretsEncrypted: "instance-ciphertext"}
	_, err = x.Insert(instance)
	require.NoError(t, err)
	assert.Greater(t, instance.ID, lastID)
	var loaded Codespace
	found, err = x.ID(instance.ID).Get(&loaded)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, instance.SecretsEncrypted, loaded.SecretsEncrypted)
}

func TestResetLegacyCodespaceInstancesCleanupRollback(t *testing.T) {
	x, cleanup := migrationtest.PrepareTestEnv(t, 0, new(codespaceResetPublicKey), new(codespaceResetDBFSMeta))
	defer cleanup()
	if x == nil || t.Failed() {
		return
	}
	_, err := x.Insert(&codespaceResetPublicKey{Type: 4, Name: "runtime"})
	require.NoError(t, err)
	// Missing DBFS data makes cleanup fail after deleting public keys.
	require.Error(t, ResetLegacyCodespaceInstancesAndUpdateSchema(t.Context(), x))
	count, err := x.Table("public_key").Count()
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
}

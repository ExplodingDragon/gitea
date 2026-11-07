// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type codespaceManagerRegistrationTokenV367 struct {
	ID       int64
	Token    string `xorm:"UNIQUE"`
	UserID   int64  `xorm:"index"`
	IsActive bool
	Created  int64 `xorm:"created"`
	Updated  int64 `xorm:"updated"`
}

func (*codespaceManagerRegistrationTokenV367) TableName() string {
	return "codespace_manager_registration_token"
}

func TestAddCodespaceManagerRegistrationTokenTable(t *testing.T) {
	x, deferrable := migrationtest.PrepareTestEnv(t, 0)
	defer deferrable()
	if x == nil || t.Failed() {
		return
	}

	require.NoError(t, AddCodespaceManagerRegistrationTokenTable(t.Context(), x))
	require.NoError(t, AddCodespaceManagerRegistrationTokenTable(t.Context(), x))

	table, ok := migrationtest.LoadTableSchemasMap(t, x)["codespace_manager_registration_token"]
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"id", "token", "user_id", "is_active", "created", "updated"}, table.ColumnsSeq())
	var indexedColumns [][]string
	for _, index := range table.Indexes {
		indexedColumns = append(indexedColumns, index.Cols)
	}
	assert.Contains(t, indexedColumns, []string{"token"})
	assert.Contains(t, indexedColumns, []string{"user_id"})

	_, err := x.Insert(&codespaceManagerRegistrationTokenV367{Token: "registration-token", UserID: 1, IsActive: true})
	require.NoError(t, err)
	_, err = x.Insert(&codespaceManagerRegistrationTokenV367{Token: "registration-token", UserID: 2, IsActive: true})
	require.Error(t, err)
}

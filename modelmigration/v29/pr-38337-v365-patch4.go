// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"
	"fmt"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

func ResetLegacyCodespaceInstancesAndUpdateSchema(ctx context.Context, x base.EngineMigration) error {
	// Old runtime identities and secret delivery cannot be reused by the new protocol.
	if err := base.EngineTransaction(x, func(sess base.Session) error {
		// Type 4 is the published Codespace key type, including orphaned runtime keys.
		if _, err := sess.Context(ctx).Exec("DELETE FROM public_key WHERE type = ?", 4); err != nil {
			return err
		}
		// DBFS prefixes paths with their depth; escape '_' to keep other namespaces intact.
		const logPath = "1:codespace!_log/%"
		if _, err := sess.Context(ctx).Exec("DELETE FROM dbfs_data WHERE meta_id IN (SELECT id FROM dbfs_meta WHERE full_path LIKE ? ESCAPE '!')", logPath); err != nil {
			return err
		}
		if _, err := sess.Context(ctx).Exec("DELETE FROM dbfs_meta WHERE full_path LIKE ? ESCAPE '!'", logPath); err != nil {
			return err
		}
		for _, table := range []string{"codespace_gitea_token", "codespace_open_token", "codespace_ssh_key", "codespace"} {
			if _, err := sess.Context(ctx).Exec("DELETE FROM " + table); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("ResetLegacyCodespaceInstancesAndUpdateSchema: clear legacy instances: %w", err)
	}

	type Codespace struct {
		SecretsEncrypted string `xorm:"LONGTEXT"`
	}
	if _, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true, IgnoreConstrains: true}, new(Codespace)); err != nil {
		return fmt.Errorf("ResetLegacyCodespaceInstancesAndUpdateSchema: add instance secrets: %w", err)
	}

	indexes, err := x.Dialect().GetIndexes(x.DB(), ctx, "codespace")
	if err != nil {
		return err
	}
	for _, index := range indexes {
		// PostgreSQL reports the published partial index by its full physical name.
		if len(index.Cols) == 1 && index.Cols[0] == "uuid" {
			if _, err := x.Context(ctx).Exec(x.Dialect().DropIndexSQL("codespace", index)); err != nil {
				return err
			}
		}
	}
	for _, column := range []struct{ table, name string }{
		{"codespace", "uuid"},
		{"codespace_user_secret", "data_size"},
		{"codespace_user_secret", "all_repositories"},
	} {
		exists, err := x.Dialect().IsColumnExist(x.DB(), ctx, column.table, column.name)
		if err != nil {
			return err
		}
		if !exists {
			continue // A failed DDL migration can be retried before its version is recorded.
		}
		if err := base.EngineTransaction(x, func(sess base.Session) error {
			if x.Dialect().URI().DBType == schemas.SQLITE {
				// Native DROP COLUMN retains the empty instance table's sequence and other indexes.
				_, err := sess.Context(ctx).Exec(fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN `%s`", column.table, column.name))
				return err
			}
			return base.DropTableColumns(sess, column.table, column.name)
		}); err != nil {
			return fmt.Errorf("ResetLegacyCodespaceInstancesAndUpdateSchema: drop %s.%s: %w", column.table, column.name, err)
		}
	}
	return x.DropTables("codespace_user_secret_repository")
}

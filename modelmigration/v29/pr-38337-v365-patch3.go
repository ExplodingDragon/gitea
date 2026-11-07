// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
)

// DropCodespaceQueuedTimeoutIndex removes the index made obsolete by the
// Codespace scheduling changes in PR #38337.
func DropCodespaceQueuedTimeoutIndex(ctx context.Context, x base.EngineMigration) error {
	indexes, err := x.Dialect().GetIndexes(x.DB(), ctx, "codespace")
	if err != nil {
		return err
	}
	if idx, ok := indexes["queued_timeout"]; ok {
		_, err = x.Exec(x.Dialect().DropIndexSQL("codespace", idx))
	}
	return err
}

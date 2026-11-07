// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func AddCodespaceManagerRegistrationTokenTable(_ context.Context, x base.EngineMigration) error {
	type codespaceManagerRegistrationToken struct {
		ID       int64
		Token    string `xorm:"UNIQUE"`
		UserID   int64  `xorm:"index"`
		IsActive bool
		Created  int64 `xorm:"created"`
		Updated  int64 `xorm:"updated"`
	}

	return x.Sync(new(codespaceManagerRegistrationToken))
}

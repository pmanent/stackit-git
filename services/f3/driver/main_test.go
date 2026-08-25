// Copyright Earl Warren <contact@earl-warren.org>
// Copyright Loïc Dachary <loic@dachary.org>
// SPDX-License-Identifier: MIT

package driver

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"forgejo.org/models/unittest"
	"forgejo.org/modules/log"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	driver_options "forgejo.org/services/f3/driver/options"

	_ "forgejo.org/models"
	_ "forgejo.org/models/actions"
	_ "forgejo.org/models/activities"
	_ "forgejo.org/models/perm/access"
	_ "forgejo.org/services/f3/driver/tests"

	tests_f3 "code.forgejo.org/f3/gof3/v3/tree/tests/f3"
	"github.com/stretchr/testify/require"
)

func TestF3(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.SSH.RootPath, t.TempDir())()
	log.SetConsoleLogger(log.DEFAULT, "console", log.TRACE)
	defer func() {
		log.SetConsoleLogger(log.DEFAULT, "console", log.INFO)
	}()
	tests_f3.ForgeCompliance(t, driver_options.Name)
}

func TestMain(m *testing.M) {
	// gof3's git subprocesses use a sparse env (only GIT_TERMINAL_PROMPT=0),
	// so they don't inherit GIT_CONFIG_GLOBAL from the parent process. They do
	// read the system /etc/gitconfig, which on some machines sets
	// init.defaultBranch=main. gof3 hardcodes "master" as the branch name, so
	// we must ensure repos are created with "master". Install a git shim that
	// prepends -c init.defaultBranch=master; git's -c flag overrides all
	// config levels including system. exec.LookPath runs in the parent's PATH,
	// so gof3's exec.Command("git", ...) will resolve to the shim.
	if realGit, err := exec.LookPath("git"); err == nil {
		if shimDir, err := os.MkdirTemp("", "git-shim-*"); err == nil {
			shimContent := fmt.Sprintf("#!/bin/sh\nexec %s -c init.defaultBranch=master \"$@\"\n", realGit)
			shimPath := filepath.Join(shimDir, "git")
			if os.WriteFile(shimPath, []byte(shimContent), 0o755) == nil {
				os.Setenv("PATH", shimDir+":"+os.Getenv("PATH"))
				defer os.RemoveAll(shimDir)
			}
		}
	}
	unittest.MainTest(m)
}

package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/actions-go/toolkit/core"
	"github.com/actions-go/toolkit/github"
	"github.com/google/uuid"
)

const authConfigKey = `http.https://github.com/.extraheader`

var now = func() time.Time {
	return time.Now()
}

func git(command string, args ...string) func() error {
	return func() error {
		cmd := exec.Command("git", append([]string{command}, args...)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
}

func firstError(funcs ...func() error) error {
	for _, f := range funcs {
		if err := f(); err != nil {
			return err
		}
	}
	return nil
}

func basicAuth(token string) string {
	return fmt.Sprintf("AUTHORIZATION: basic %s", token)
}

func setupCredentials(repo string) func() error {
	return func() error {
		placeHolder := uuid.New().String()

		// Configure a placeholder value. This approach avoids the credential being captured
		// by process creation audit events, which are commonly logged. For more information,
		// refer to https://docs.microsoft.com/en-us/windows-server/identity/ad-ds/manage/component-updates/command-line-process-auditing
		err := git("-C", repo, "config", authConfigKey, basicAuth(placeHolder))()
		if err != nil {
			return err
		}
		fd, err := os.OpenFile(filepath.Join(repo, ".git/config"), os.O_RDWR, 0)
		if err != nil {
			return err
		}
		defer fd.Close()
		_, err = fd.Seek(0, 0)
		if err != nil {
			return err
		}
		token, ok := core.GetInput("token")
		if !ok {
			return fmt.Errorf("missing token input")
		}
		authHeader := base64.StdEncoding.WithPadding(base64.StdPadding).EncodeToString([]byte(fmt.Sprintf("x-access-token:%s", token)))
		core.SetSecret(authHeader)
		b := bytes.NewBuffer(nil)
		_, err = io.Copy(b, fd)
		if err != nil {
			return err
		}
		_, err = fd.Seek(0, 0)
		if err != nil {
			return err
		}
		written, err := fd.Write([]byte(strings.ReplaceAll(b.String(), basicAuth(placeHolder), basicAuth(authHeader))))
		if err != nil {
			return err
		}
		return fd.Truncate(int64(written))
	}
}

func rename() ([]string, error) {
	renames := map[string]string{
		"main-windows-latest": "main_windows.exe",
		"main-macos-latest":   "main_darwin",
		"main-ubuntu-latest":  "main_linux",
	}
	targets := []string{}
	err := os.MkdirAll("dist", 0755)
	if err != nil {
		return []string{}, err
	}
	for k, v := range renames {
		k = filepath.Join(k, "main")
		dest := filepath.Join("dist", v)
		err := os.Rename(k, dest)
		if err != nil {
			core.Warning(fmt.Sprintf("unable to rename expected file %s: %v", k, err))
			continue
		}
		targets = append(targets, dest)
		err = os.Chmod(dest, 0755)
		if err != nil {
			core.Warning(fmt.Sprintf("unable to change permissions to file %s: %v", k, err))
		}
	}

	return targets, nil
}

func gitAdd() error {
	r, err := rename()
	if err != nil {
		return err
	}
	return git("add", r...)()
}

func push(branch string) func() error {
	return func() error {
		b := bytes.NewBuffer(nil)
		cmd := exec.Command("git", "status", "-s", "-uno")
		cmd.Stdout = b
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			return err
		}
		if strings.TrimSpace(b.String()) == "" {
			core.Info("No file updated. Skipping push")
			return nil
		}
		return firstError(
			gitAdd,
			git("config", "user.email", "actions-go@users.noreply.github.com"),
			git("config", "user.name", "actions-go-bot"),
			git("commit", "-m", "[auto] Add golang action entrypoints"),
			git("push", "origin", branch),
		)
	}
}

func runMain() error {
	if github.Context.Payload.Repository == nil || github.Context.Payload.Repository.CloneURL == nil {
		return fmt.Errorf("missing repository URL in event")
	}
	branch := strings.TrimPrefix(github.Context.Ref, "refs/heads/")
	return firstError(
		git("init", "."),
		git("remote", "add", "origin", *github.Context.Payload.Repository.CloneURL),
		setupCredentials("."),
		git("fetch", "origin"),
		git("checkout", branch),
		push(branch),
	)
}

func main() {
	if err := runMain(); err != nil {
		core.Error(err.Error())
		os.Exit(1)
	}
}

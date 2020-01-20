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

func setupCredentials() func() error {
	return func() error {
		placeHolder := uuid.New().String()

		// Configure a placeholder value. This approach avoids the credential being captured
		// by process creation audit events, which are commonly logged. For more information,
		// refer to https://docs.microsoft.com/en-us/windows-server/identity/ad-ds/manage/component-updates/command-line-process-auditing
		err := git("config", authConfigKey, basicAuth(placeHolder))()
		if err != nil {
			return err
		}
		fd, err := os.OpenFile(".git/config", os.O_RDWR, 0)
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
		b := bytes.NewBuffer(nil)
		_, err = base64.NewEncoder(base64.StdEncoding, b).Write([]byte(fmt.Sprintf("x-access-token:%s", token)))
		if err != nil {
			return err
		}
		authHeader := b.String()
		core.SetSecret(authHeader)
		b = bytes.NewBuffer(nil)
		_, err = io.Copy(b, fd)
		if err != nil {
			return err
		}
		_, err = fd.Seek(0, 0)
		if err != nil {
			return err
		}
		written, err := fd.Write([]byte(strings.ReplaceAll(b.String(), basicAuth(placeHolder), basicAuth(authHeader))))
		fd.Truncate(int64(written))
		return err
	}
}

func rename() ([]string, error) {
	renames := map[string]string{
		"main-windows-latest": "main_windows.exe",
		"main-macos-latest":   "main_darwin",
		"main-linux-latest":   "main_linux",
	}
	targets := []string{}
	err := os.MkdirAll("dist", 0755)
	if err != nil {
		return []string{}, err
	}
	for k, v := range renames {
		dest := filepath.Join("dist", v)
		err := os.Rename(k, dest)
		if err != nil {
			core.Warning(fmt.Sprintf("unable to rename expected file %s: %v", k, err))
		} else {
			targets = append(targets, dest)
		}
	}

	return targets, nil
}

func runMain() error {
	if github.Context.Payload.Repository == nil || github.Context.Payload.Repository.CloneURL == nil {
		return fmt.Errorf("missing repository URL in event")
	}
	names, err := rename()
	if err != nil {
		return err
	}
	if len(names) < 1 {
		return fmt.Errorf("no file to commit")
	}
	branch := strings.TrimPrefix(github.Context.Ref, "refs/heads/")

	return firstError(
		git("init", "."),
		git("remote", "add", "origin", *github.Context.Payload.Repository.CloneURL),
		setupCredentials(),
		git("fetch", "origin"),
		git("checkout", branch),
		git("add", names...),
		git("config", "user.email", "actions-go@users.noreply.github.com"),
		git("config", "user.name", "actions-go-bot"),
		git("commit", "-m", "[auto] Add golang action entrypoints"),
		git("push", "origin", branch),
	)
}

func main() {
	if err := runMain(); err != nil {
		core.Error(err.Error())
		os.Exit(1)
	}
}

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/actions-go/toolkit/core"
	"github.com/actions-go/toolkit/github"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func writeTestFile(t *testing.T, folder, file string) {
	p := filepath.Join(folder, file)
	fmt.Println(p)
	d := filepath.Dir(p)
	assert.NoError(t, os.MkdirAll(d, 0755))
	assert.NoError(t, ioutil.WriteFile(p, []byte(file), 0755))
}

func gitDir(directory string) *string {
	directory = filepath.Join(directory, ".git")
	return &directory
}

func TestAuthenticate(t *testing.T) {
	if _, ok := core.GetInput("token"); !ok {
		t.Skip("missing test token")
		t.SkipNow()
	}
	wd, _ := os.Getwd()
	defer os.Chdir(wd)

	testID := uuid.New().String()
	dir, err := ioutil.TempDir("", "test-workdir-"+testID)
	assert.NoError(t, err)
	defer os.RemoveAll(dir)
	assert.NoError(t, os.Chdir(dir))

	assert.NoError(t, firstError(
		git("init", dir),
		git("-C", dir, "remote", "-vvv"),
		git("-C", dir, "remote", "add", "origin", "https://github.com/actions-go/releaser.git"),
		setupCredentials(dir),
		git("-C", dir, "fetch", "origin"),
	))
}

func TestRunMain(t *testing.T) {
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	defer os.Setenv("INPUT_TOKEN", os.Getenv("INPUT_TOKEN"))
	os.Setenv("INPUT_TOKEN", "some-token")
	wd, err := os.Getwd()
	assert.NoError(t, err)

	// The simulated push event
	releaseBranch := "release/v1"

	os.Setenv("GITHUB_EVENT_PATH", filepath.Join(wd, "resources/push_event.json"))
	os.Setenv("GITHUB_REF", releaseBranch)
	testID := uuid.New().String()

	github.Context = github.ParseActionEnv()

	testUpstream, err := ioutil.TempDir("", "test-upstream-"+testID)
	assert.NotNil(t, github.Context.Payload.Repository)
	if github.Context.Payload.Repository != nil {
		github.Context.Payload.Repository.CloneURL = gitDir(testUpstream)
	}
	assert.NoError(t, err)
	defer os.RemoveAll(testUpstream)

	assert.NoError(t, err)

	assert.NoError(t, git("init", testUpstream)())
	writeTestFile(t, testUpstream, "README.md")
	writeTestFile(t, testUpstream, "dist/main_linux")
	writeTestFile(t, testUpstream, "dist/main_windows.exe")
	writeTestFile(t, testUpstream, "dist/main_darwin")
	assert.NoError(t, git("-C", testUpstream, "config", "user.email", "actions-go@users.noreply.github.com")())
	assert.NoError(t, git("-C", testUpstream, "config", "user.name", "actions-go-bot")())
	assert.NoError(t, git("-C", testUpstream, "add", "README.md", "dist")())
	assert.NoError(t, git("-C", testUpstream, "commit", "-m", "add initial files")())

	assert.NoError(t, git("-C", testUpstream, "branch", releaseBranch)())

	dir, err := ioutil.TempDir("", "test-workdir-"+testID)
	assert.NoError(t, err)
	defer os.RemoveAll(dir)
	assert.NoError(t, os.Chdir(dir))

	writeTestFile(t, dir, filepath.Join("main-windows-latest", "main"))
	writeTestFile(t, dir, filepath.Join("main-macos-latest", "main"))
	writeTestFile(t, dir, filepath.Join("main-ubuntu-latest", "main"))

	assert.NoError(t, runMain())

	assert.NoError(t, git("-C", testUpstream, "checkout", releaseBranch)())
	assert.FileExists(t, filepath.Join(testUpstream, "dist", "main_linux"))
	assert.FileExists(t, filepath.Join(testUpstream, "dist", "main_darwin"))
	assert.FileExists(t, filepath.Join(testUpstream, "dist", "main_windows.exe"))
}

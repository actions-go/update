package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/actions-go/toolkit/github"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func writeTestFile(t *testing.T, folder, file string) {
	assert.NoError(t, ioutil.WriteFile(filepath.Join(folder, file), []byte(file), 0755))
}

func gitDir(directory string) *string {
	directory = filepath.Join(directory, ".git")
	return &directory
}

func TestRunMain(t *testing.T) {
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
	assert.NoError(t, git("-C", testUpstream, "add", "README.md")())
	assert.NoError(t, git("-C", testUpstream, "commit", "-m", "add initial files")())

	assert.NoError(t, git("-C", testUpstream, "branch", releaseBranch)())

	dir, err := ioutil.TempDir("", "test-workdir-"+testID)
	assert.NoError(t, err)
	defer os.RemoveAll(dir)
	err = os.Chdir(dir)

	writeTestFile(t, dir, "main-windows-latest")
	writeTestFile(t, dir, "main-macos-latest")
	writeTestFile(t, dir, "main-linux-latest")

	assert.NoError(t, runMain())

	assert.NoError(t, git("-C", testUpstream, "checkout", releaseBranch)())
	assert.FileExists(t, filepath.Join(testUpstream, "dist", "main_linux"))
	assert.FileExists(t, filepath.Join(testUpstream, "dist", "main_darwin"))
	assert.FileExists(t, filepath.Join(testUpstream, "dist", "main_windows.exe"))
}

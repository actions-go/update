# Build a golang action

This action allows you to build an action written in go.

## Use this action

Yet to be tested and adjusted

Create the workflow to build your release. For example in `.github/workflows/build-dist.yml

```yaml
name: "build-dist"
on: # rebuild any branch changes
  push:
    paths:
      - '*'
      - '**/*'
      - '!dist/**'
    branches:
      - '*'
    tags-ignore:
      - '*'

jobs:
  build:
    strategy:
      matrix:
        runs-on: [ubuntu-latest, macos-latest, windows-latest]
    runs-on: ${{ matrix.runs-on }}
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v1
        with:
          go-version: '1.17'
      - run: go build -v -o dist/main
      - uses: actions/upload-artifact@v1
        with:
          name: main-${{ matrix.runs-on }}
          path: dist/main
  publish:
    name: Publish new action version
    runs-on: ubuntu-latest
    needs: [build]
    steps:
      - uses: actions/download-artifact@v1
        with:
          name: main-ubuntu-latest
      - uses: actions/download-artifact@v1
        with:
          name: main-macos-latest
      - uses: actions/download-artifact@v1
        with:
          name: main-windows-latest
      - uses: actions-go/update@v1

```

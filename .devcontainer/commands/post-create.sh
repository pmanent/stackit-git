#!/bin/bash

PROJECT_ROOT=$PWD
source "$PWD/.devcontainer/commands/utils/colors.sh"

printf "[${BRed}${On_Yellow}STARTING${RESET}] ${Purple}->${RESET} Post create\n"

PROJECT_ROOT=$PWD
# SRC_DIR="$PROJECT_ROOT/src/core"
# OUT_FILE="$PROJECT_ROOT/src/core/core"

go version
go env
# go build -o "$OUT_FILE" '-gcflags=all=-N -l' "$SRC_DIR/main.go"
# mkdir -p "$PROJECT_ROOT/src/core/data/migrations/postgresql"
# cp -r "$PROJECT_ROOT/make/migrations/postgresql" "$PROJECT_ROOT/src/core/data/migrations"

printf "[${BRed}${On_Yellow}Ends${RESET}] ${Purple}->${RESET} Post create\n"

make deps
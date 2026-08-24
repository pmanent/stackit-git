#!/bin/bash

PROJECT_ROOT=$PWD
source "$PWD/.devcontainer/commands/utils/colors.sh"

printf "[${BRed}${On_Yellow}STARTING${RESET}] ${Purple}->${RESET} Post attach\n"

go install -v golang.org/x/tools/gopls@v0.17.0

printf "[${BRed}${On_Yellow}Ends${RESET}] ${Purple}->${RESET} Post attach\n"

sleep 2
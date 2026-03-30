#!/bin/bash

PROJECT_ROOT=$PWD
source "$PWD/.devcontainer/commands/utils/colors.sh"

printf "[${BRed}${On_Yellow}STARTING${RESET}] ${Purple}->${RESET} Post start\n"

go version


printf "[${BRed}${On_Yellow}ENDS${RESET}] ${Purple}->${RESET} Post start\n"
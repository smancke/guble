#!/usr/bin/env bash

GO_TEST_DISABLED=true go test -v -short $(go list ./... | grep -v /vendor/)
TESTRESULT=$?

RED='\033[0;31m'
GREEN='\033[0;32m'
NOCOLOR='\033[0m'

case ${TESTRESULT} in
0)
  MESSAGE="${GREEN}OK"
  ;;
1)
  MESSAGE="${RED}Test(s) failing"
  ;;
2)
  MESSAGE="${RED}Compilation error"
  ;;
*)
  MESSAGE="${RED}Error(s)"
  ;;
esac

echo -e "${MESSAGE}${NOCOLOR}\n"

exit ${TESTRESULT}

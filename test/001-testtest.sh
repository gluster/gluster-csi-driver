#!/bin/bash

# This "test" exists merely to exercise the test "framework"

if [ "$CSI_TEST_TEST" ]; then
	exit "$CSI_TEST_TEST"
fi
exit 0

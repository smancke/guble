#!/usr/bin/env bash

xxd -p $1 | tr -d '\n'

#!/usr/bin/env bash
# Prints all env vars injected by ward exec, sorted — use to compare with ward envs output.
env | grep -v '^_=' | sort

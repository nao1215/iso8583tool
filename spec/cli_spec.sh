#!/bin/sh
# shellcheck shell=sh
#
# CLI surface: help, version, unknown commands, and per-subcommand help. These
# do not need a fixture, so they run the binary directly.

Describe 'iso8583tool CLI surface'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  Describe 'root help'
    It 'prints help with no arguments'
      When run iso8583tool
      The status should be success
      The output should include 'Commands:'
      The output should include 'view'
      The output should include 'convert'
      The output should include 'validate'
    End
  End

  Describe 'version'
    It 'prints the version'
      When run iso8583tool version
      The status should be success
      The output should include 'iso8583tool'
    End
  End

  Describe 'unknown command'
    It 'fails and shows the command list'
      When run iso8583tool frobnicate
      The status should be failure
      The stderr should include 'unknown command'
      The stderr should include 'Commands:'
    End
  End

  Describe 'subcommand help'
    It 'describes convert and exits 0'
      When run iso8583tool help convert
      The status should be success
      The output should include 'Usage: iso8583tool convert'
      The output should include '--to'
    End

    It 'describes view and lists --filter'
      When run iso8583tool view --help
      The status should be success
      The output should include 'Usage: iso8583tool view'
      The output should include '--filter'
      # Successful help must go to stdout, not stderr (bug 36).
      The stderr should equal ''
    End
  End

  Describe 'root flags reject extra arguments'
    It 'rejects --help with a trailing argument'
      When run iso8583tool --help view
      The status should be failure
      The stderr should include 'takes no arguments'
    End

    It 'rejects --version with a trailing argument'
      When run iso8583tool --version view
      The status should be failure
      The stderr should include 'takes no arguments'
    End
  End
End

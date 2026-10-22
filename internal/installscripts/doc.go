// Package installscripts holds the checks for the one-click installer
// scripts in scripts/. The installers are a POSIX shell script and a
// PowerShell script that users pipe into their shell, so they carry no
// test files of their own and this package exercises them from the
// repository's own test runner instead.
package installscripts

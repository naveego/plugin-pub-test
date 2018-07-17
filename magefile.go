// +build mage

package main

import (
	"github.com/naveego/dataflow-contracts/plugins"
	"github.com/magefile/mage/sh"
)

// Default target to run when none is specified
// If not set, running mage will list available targets
// var Default = Build

// A build step that requires additional params, or platform specific steps for example
func Build() error {
	return sh.Run("go", "build", "-o", "./bin/plugin-test-pub", "github.com/naveego/plugin-pub-test")
}

func Install() error {
	return sh.Run("go", "install", "github.com/naveego/plugin-pub-test")
}

func GenerateGRPC() error {
	destDir := "./internal/pub"
	return plugins.GeneratePublisher(destDir)
}
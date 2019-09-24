// Author  <dorzheho@cisco.com>

package main

import (
	"github.com/sirupsen/logrus"

	"cisco.com/son/apphcd/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		logrus.Fatal(err)
	}
}

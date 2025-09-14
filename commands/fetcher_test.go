package commands

import (
	"context"
	"github.com/ZenLiuCN/fn"
	"testing"
)

func TestFetchTypes(t *testing.T) {
	fn.Panic(Commands().Run(context.Background(), []string{
		"units",
		"types",
		"-o",
		"./typing",
		"D:\\Dev\\store\\vanPlusMany\\vans\\babylon\\tar\\@babylonjs_addons-8.27.0.tgz",
	}))
}
func TestFetchNPM(t *testing.T) {
	fn.Panic(Commands().Run(context.Background(), []string{
		"units",
		"npm",
		"@babylonjs/core",
	}))
}
func TestFetchMaven(t *testing.T) {
	fn.Panic(Commands().Run(context.Background(), []string{
		"units",
		"mvn",
		"io.vertx:vertx-shell:",
	}))
}

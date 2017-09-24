package main

import (
	"fmt"
	"os"

	"github.com/golang/glog"
	"github.com/monopole/mdrip/config"
	"github.com/monopole/mdrip/subshell"
	"github.com/monopole/mdrip/tmux"
	"github.com/monopole/mdrip/tutorial"
	"github.com/monopole/mdrip/webserver"
)

func main() {
	c := config.GetConfig()
	switch c.Mode() {
	case config.ModeTmux:
		t := tmux.NewTmux(tmux.Path)
		if !t.IsUp() {
			glog.Fatal(tmux.Path, " not running")
		}
		// Steal the first fileName as a host address argument.
		t.Adapt(string(c.FileNames()[0]))
	case config.ModeWeb:
		webserver.NewServer(c.FileNames()).Serve(c.HostAndPort())
	case config.ModeTest:
		p, err := tutorial.NewProgramFromPaths(c.Label(), c.FileNames())
		if err != nil {
			fmt.Println(err)
			return
		}
		s := subshell.NewSubshell(c.BlockTimeOut(), p)
		if r := s.Run(); r.Problem() != nil {
			r.Print(c.Label())
			if !c.IgnoreTestFailure() {
				glog.Fatal(r.Problem())
			}
		}
	default:
		p, err := tutorial.NewProgramFromPaths(c.Label(), c.FileNames())
		if err != nil {
			fmt.Println(err)
			return
		}
		if c.Preambled() > 0 {
			p.PrintPreambled(os.Stdout, c.Preambled())
		} else {
			p.PrintNormal(os.Stdout)
		}
	}
}

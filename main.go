package main

import (
	"fmt"
	"os"

	"github.com/golang/glog"
	"github.com/monopole/mdrip/config"
	"github.com/monopole/mdrip/loader"
	"github.com/monopole/mdrip/program"
	"github.com/monopole/mdrip/subshell"
	"github.com/monopole/mdrip/tmux"
	"github.com/monopole/mdrip/webserver"
)

func trueMain(c *config.Config) error {
	switch c.Mode() {
	case config.ModeTmux:
		t := tmux.NewTmux(tmux.Path)
		if !t.IsUp() {
			glog.Fatal(tmux.Path, " not running")
		}
		// Treat the first arg as a host address argument.
		t.Adapt(c.DataSet().FirstArg().Raw())
	case config.ModeDemo:
		l := loader.NewLoader(c.DataSet())
		s, err := webserver.NewServer(l)
		if err != nil {
			return err
		}
		err = s.Serve(c.HostAndPort())
		if err != nil {
			return err
		}
	case config.ModeTest:
		t, err := loader.NewLoader(c.DataSet()).Load()
		if err != nil {
			return err
		}
		p := program.NewProgramFromTutorial(c.Label(), t)
		s := subshell.NewSubshell(c.BlockTimeOut(), p)
		if r := s.Run(); r.Problem() != nil {
			r.Print(c.Label())
			if !c.IgnoreTestFailure() {
				glog.Fatal(r.Problem())
			}
		}
	default:
		t, err := loader.NewLoader(c.DataSet()).Load()
		if err != nil {
			return err
		}
		p := program.NewProgramFromTutorial(c.Label(), t)
		if c.Preambled() > 0 {
			p.PrintPreambled(os.Stdout, c.Preambled())
		} else {
			p.PrintNormal(os.Stdout)
		}
	}
	return nil
}

func main() {
	c, err := config.GetConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		config.Usage()
		os.Exit(1)
	}
	err = trueMain(c)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		config.Usage()
		os.Exit(1)
	}
}

package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"

	_ "cluster/gateway"
	"common"
	"engine"
	"logger"
	"rest"
)

func main() {
	var exitCode int
	var err error

	var cpuProfile *os.File
	if common.CpuProfileFile != "" {
		cpuProfile, err = os.Create(common.CpuProfileFile)
		if err != nil {
			logger.Errorf("[initialize cpu profile failed: %v]", err)
			exitCode = 1
			return
		}

		pprof.StartCPUProfile(cpuProfile)

		logger.Infof("[cpu profiling started, output to %s]", common.CpuProfileFile)
	}

	defer func() {
		if common.CpuProfileFile != "" {
			pprof.StopCPUProfile()

			if cpuProfile != nil {
				cpuProfile.Close()
			}
		}

		os.Exit(exitCode)
	}()

	gateway, err := engine.NewGateway()
	if err != nil {
		logger.Errorf("[initialize gateway engine failed: %v]", err)
		exitCode = 2
		return
	}

	api, err := rest.NewRest(gateway)
	if err != nil {
		logger.Errorf("[initialize rest interface failed: %v]", err)
		exitCode = 3
		return
	}

	setupExitSignalHandler(gateway)

	done1, err := gateway.Run()
	if err != nil {
		logger.Errorf("[start gateway engine failed: %v]", err)
		exitCode = 4
		return
	} else {
		logger.Infof("[gateway engine started]")
	}

	done2, listenAddr, err := api.Start()
	if err != nil {
		logger.Errorf("[start rest interface at %s failed: %s]", listenAddr, err)
		exitCode = 5
		return
	} else {
		logger.Infof("[rest interface started at %s]", listenAddr)
	}

	var msg string
	select {
	case err = <-done1:
		msg = "gateway engine"
	case err = <-done2:
		msg = "api server"
	}

	if err != nil {
		msg = fmt.Sprintf("[exit from %s due to error: %v]", msg, err)
		logger.Warnf(msg)
	} else {
		msg = fmt.Sprintf("[exit from %s without error]", msg)
		logger.Infof(msg)
	}

	// interrupt by signal
	gateway.Close()
	api.Close()

	logger.Infof("[gateway exited normally]")

	return
}

func setupExitSignalHandler(gateway *engine.Gateway) {
	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		for times := 0; sigChannel != nil; times++ {
			sig := <-sigChannel
			if sig == nil {
				return // channel closed by normal exit process
			}

			switch times {
			case 0:
				go func() {
					logger.Infof("[%s signal received, shutting down gateway]", sig)
					gateway.Stop()
					close(sigChannel)
					sigChannel = nil
				}()
			case 1:
				logger.Infof("[%s signal received, terminating gateway immediately]", sig)
				close(sigChannel)
				sigChannel = nil
				os.Exit(255)
			}
		}
	}()
}

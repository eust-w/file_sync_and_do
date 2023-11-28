package main

import (
	"file_sync_and_do/rfsnotify"
	"flag"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"os"
	"path"
)

var peer = flag.String("peer", "", "peer node IP address")

var uiProxyPath = "/tmp/lt/test"
var uiProxyCmds = []string{
	"sudo echo 1 >> /tmp/lt/txt",
	"sudo echo 2 >> /tmp/lt/txt",
	"sudo echo 3 >> /tmp/lt/txt",
	"sudo uname -a >> /tmp/lt/fds",
}

func uiProxyMonitor(peer string) {
	fullUiproxyPath := uiProxyPath
	go dirSyncAndRunCmdsMonitor(fullUiproxyPath, peer, uiProxyCmds)
}

func dirSyncAndRunCmdsMonitor(dir, peer string, cmds []string) {
	watcher, err := rfsnotify.NewWatcher()
	if err != nil {
		fmt.Printf("fsnotify: %s", err)
		return
	}
	defer watcher.Close()
	done := make(chan bool)
	go func() {
		fmt.Printf("syncing dir from %s to %s, and run cmds %v", dir, peer, cmds)
		for true {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					done <- true
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write ||
					event.Op&fsnotify.Remove == fsnotify.Remove ||
					event.Op&fsnotify.Rename == fsnotify.Rename {
					fmt.Printf("syncing dir from %s to %s", dir, peer)
					go syncDirAndRunCmds(dir, peer, cmds)
				}
				if event.Op&fsnotify.Create == fsnotify.Create {
					fmt.Printf("new create event, watch[%s] and syncing to %s", event.Name, peer)
					err = watcher.AddRecursive(dir)
					if err != nil {
						fmt.Printf("fsnotify: %s", err)
					}
					fmt.Printf("syncing dir from %s to %s", dir, peer)
					go syncDirAndRunCmds(dir, peer, cmds)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					done <- true
					return
				}
				fmt.Printf("fsnotify: %s", err)
			}
		}
	}()

	err = watcher.AddRecursive(dir)
	if err != nil {
		fmt.Printf("fsnotify: %s", err)
	}
	<-done
}

func syncDirAndRunCmds(dir, peer string, cmds []string) {
	doRsyncToRemote(dir, peer, false)
	runCmds(cmds)
}

func doRsyncToRemote(fpath, peer string, skipNewer bool) {
	opt := "-rtogz"
	if skipNewer {
		opt += "u"
	}
	bash := Bash{
		Command: fmt.Sprintf(
			`/usr/bin/rsync --delete %s -e "ssh -o BatchMode=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null" %s %s`,
			opt,
			fpath,
			fmt.Sprintf("root@%s:%s", peer, path.Dir(fpath))),
	}

	_, stdout, stderr, err := bash.RunWithReturn()
	if err != nil {
		fmt.Printf("rsync %s failed: %s", fpath, stdout+stderr)
	}
	return
}

func runCmds(cmds []string) {
	for _, cmd := range cmds {
		bash := Bash{
			Command: fmt.Sprintf(cmd),
		}
		_, stdout, stderr, err := bash.RunWithReturn()
		if err != nil {
			fmt.Printf("run cmds %s failed: %s", cmd, stdout+stderr)
		}
	}
	return
}

func uiProxyInit(peer string) {
	doRsyncToRemote(uiProxyPath, peer, true)
	runCmds(uiProxyCmds)
}

func main() {
	flag.Parse()

	if *peer == "" {
		fmt.Println("peer node IP address is required")
		os.Exit(1)
	}

	uiProxyInit(*peer)
	uiProxyMonitor(*peer)
	select {}
}

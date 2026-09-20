package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"branch-office/internal/config"
	"branch-office/internal/server"
)

//go:embed all:dist
var embeddedAssets embed.FS

func main() {
	defaultAddr := os.Getenv("BROFFICE_ADDR")
	if defaultAddr == "" {
		port := os.Getenv("PORT")
		if port != "" {
			defaultAddr = "0.0.0.0:" + port
		} else {
			defaultAddr = "0.0.0.0:8080"
		}
	}

	addr := flag.String("addr", defaultAddr, "HTTP service address to listen on")
	repoDir := flag.String("dir", "", "Path to a Git repository to auto-register")
	configPath := flag.String("config", "", "Custom path to config.json")
	sshKey := flag.String("ssh-key", os.Getenv("BROFFICE_SSH_KEY"), "Path to a dedicated SSH private key (bypasses system SSH agent)")
	noSign := flag.Bool("no-sign", os.Getenv("BROFFICE_NO_SIGN") == "true" || os.Getenv("BROFFICE_NO_SIGN") == "1", "Disable Git commit GPG/SSH signing")
	flag.Parse()

	store, err := config.NewStore(*configPath)
	if err != nil {
		log.Fatalf("Failed to initialize config store: %v", err)
	}

	if *sshKey != "" {
		_ = store.SetSSHKey(*sshKey)
	}
	if *noSign {
		_ = store.SetDisableSigning(true)
	}

	// Auto-register target repo if specified or if running inside a git repo
	targetDir := *repoDir
	if targetDir == "" {
		// Check current directory
		if cwd, err := os.Getwd(); err == nil {
			if resolved, err := config.ValidateAndResolveGitPath(cwd); err == nil {
				targetDir = resolved
			}
		}
	}

	if targetDir != "" {
		if registered, err := store.AddRepo(targetDir); err == nil {
			fmt.Printf("Registered active repository: %s (%s)\n", registered.Name, registered.Path)
		}
	}

	subFS, err := fs.Sub(embeddedAssets, "dist")
	if err != nil {
		log.Fatalf("Failed to load embedded web assets: %v", err)
	}

	srv := server.NewServer(store, subFS)

	activeSSH := store.GetSSHKey()
	if activeSSH == "" {
		activeSSH = "System Agent (Default)"
	}
	signStatus := "Enabled (Git Default)"
	if store.GetDisableSigning() {
		signStatus = "Disabled (No Biometric Prompts)"
	}

	host, port, err := net.SplitHostPort(*addr)
	if err != nil {
		port = "8080"
		host = *addr
	}

	repoCount := len(store.ListRepos())
	repoLabel := "repositories"
	if repoCount == 1 {
		repoLabel = "repository"
	}

	fmt.Println()
	fmt.Println("==================================================")
	fmt.Println("             Branch Office (broffice)             ")
	fmt.Println("        Mobile Remote Git Control Center          ")
	fmt.Println("==================================================")

	if host == "0.0.0.0" || host == "" {
		fmt.Printf(" Local Access   : http://localhost:%s\n", port)

		if addrs, err := net.InterfaceAddrs(); err == nil {
			for _, a := range addrs {
				if ipNet, ok := a.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
					if ip4 := ipNet.IP.To4(); ip4 != nil {
						ipStr := ip4.String()
						if strings.HasPrefix(ipStr, "100.") {
							fmt.Printf(" Tailscale      : http://%s:%s\n", ipStr, port)
						} else {
							fmt.Printf(" Network (LAN)  : http://%s:%s\n", ipStr, port)
						}
					}
				}
			}
		}

		if hostname, err := os.Hostname(); err == nil {
			shortHost := strings.Split(hostname, ".")[0]
			fmt.Printf(" Bonjour/mDNS   : http://%s.local:%s\n", shortHost, port)
		}
	} else {
		fmt.Printf(" Access URL     : http://%s:%s\n", host, port)
	}

	fmt.Printf(" Registered     : %d %s\n", repoCount, repoLabel)
	fmt.Printf(" SSH Key Auth   : %s\n", activeSSH)
	fmt.Printf(" Commit Signing : %s\n", signStatus)
	fmt.Println("==================================================")
	fmt.Println()

	if err := http.ListenAndServe(*addr, srv.Routes()); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

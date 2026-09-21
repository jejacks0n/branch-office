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
	"os/exec"
	"runtime"
	"strings"

	"branch-office/internal/config"
	"branch-office/internal/server"

	"github.com/skip2/go-qrcode"
)

//go:embed all:dist
var embeddedAssets embed.FS

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

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
	local := flag.Bool("local", os.Getenv("BROFFICE_LOCAL") == "true" || os.Getenv("BROFFICE_LOCAL") == "1", "Listen on localhost only (127.0.0.1) instead of all interfaces; keeps the port from -addr")
	noAuth := flag.Bool("no-auth", os.Getenv("BROFFICE_NO_AUTH") == "true" || os.Getenv("BROFFICE_NO_AUTH") == "1", "Disable token authentication (anyone who can reach the port gets full control)")
	showVersion := flag.Bool("version", false, "Print version information and exit")
	flag.Parse()

	if *local {
		localised, err := localAddr(*addr)
		if err != nil {
			log.Fatalf("Invalid -addr %q: %v", *addr, err)
		}
		*addr = localised
	}

	if *showVersion {
		fmt.Printf("broffice version %s (%s, %s)\n", version, commit, date)
		return
	}

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

	var authToken string
	if !*noAuth {
		authToken, err = store.EnsureToken()
		if err != nil {
			log.Fatalf("Failed to generate auth token: %v", err)
		}
		srv.SetAuthToken(authToken)
	}

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
	if version != "dev" {
		fmt.Printf("         Branch Office (broffice) v%s\n", version)
	} else {
		fmt.Println("             Branch Office (broffice)             ")
	}
	fmt.Println("        Mobile Remote Git Control Center          ")
	fmt.Println("==================================================")

	lanMode := host == "0.0.0.0" || host == ""
	bonjourHost := localHostname()

	if lanMode {
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

		if bonjourHost != "" {
			fmt.Printf(" Bonjour/mDNS   : http://%s.local:%s\n", bonjourHost, port)
		}
	} else {
		fmt.Printf(" Access URL     : http://%s:%s\n", host, port)
	}

	if authToken != "" {
		if lanMode {
			fmt.Printf(" Auth Token     : %s\n", authToken)
			fmt.Println("                  Append #token=<Auth Token> to any URL above, or scan")
			fmt.Println("                  the QR below with your phone.")
		} else {
			fmt.Printf(" Auth Token URL : http://%s:%s/#token=%s\n", host, port, authToken)
			if *local {
				fmt.Println(" Remote Access  : off (-local). Drop -local for LAN access, or use 'tailscale serve'")
			}
		}
	} else if !isLoopbackHost(host) {
		fmt.Println(" WARNING        : Authentication is DISABLED (-no-auth) and this server is")
		fmt.Println("                  reachable beyond this machine. Anyone who can reach the")
		fmt.Println("                  port can stage, discard, commit, and push in every")
		fmt.Println("                  registered repository. Add -local, or drop -no-auth.")
	} else {
		fmt.Println(" Auth           : DISABLED (-no-auth). Any local process has full")
		fmt.Println("                  control of your repositories")
	}

	fmt.Printf(" Registered     : %d %s\n", repoCount, repoLabel)
	fmt.Printf(" SSH Key Auth   : %s\n", activeSSH)
	fmt.Printf(" Commit Signing : %s\n", signStatus)
	fmt.Println("==================================================")

	if lanMode && bonjourHost != "" {
		qrURL := fmt.Sprintf("http://%s.local:%s", bonjourHost, port)
		if authToken != "" {
			qrURL += "/?token=" + authToken
		}
		fmt.Println()
		fmt.Print(scanQRBlock(qrURL))
	}

	fmt.Println()

	if err := http.ListenAndServe(*addr, srv.Routes()); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// isLoopbackHost reports whether a listen host only accepts connections from this
// machine. An empty host or 0.0.0.0 listens on every interface, so it is not.
func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// localAddr rewrites addr to listen on loopback only, keeping its port.
func localAddr(addr string) (string, error) {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", err
	}
	return net.JoinHostPort("127.0.0.1", port), nil
}

// localHostname returns the machine's Bonjour/mDNS name for the banner. On
// macOS, os.Hostname() reflects the kernel hostname (often hijacked by
// Tailscale MagicDNS or DHCP), so query the actual LocalHostName instead.
func localHostname() string {
	if runtime.GOOS == "darwin" {
		if out, err := exec.Command("scutil", "--get", "LocalHostName").Output(); err == nil {
			if name := strings.TrimSpace(string(out)); name != "" {
				return strings.ReplaceAll(strings.TrimSuffix(name, ".local"), " ", "-")
			}
		}
	}
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}
	if i := strings.Index(hostname, "."); i > 0 {
		return hostname[:i]
	}
	return hostname
}

// scanQRBlock renders url as a compact terminal QR code with a banner-style
// label, for scanning with a phone. Dark modules are drawn as terminal
// background (inverseColor=true), which displays correctly on dark terminals.
// Quiet-zone rows render as blank lines; the terminal background provides the
// clear margin. Returns "" when the URL cannot be encoded.
func scanQRBlock(url string) string {
	qr, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		return ""
	}
	var buf strings.Builder
	labelled := false
	for _, line := range strings.Split(strings.TrimRight(qr.ToSmallString(true), "\n"), "\n") {
		line = strings.TrimRight(line, " ")
		switch {
		case line == "":
			buf.WriteString("\n")
		case !labelled:
			buf.WriteString(" Scan (QR)      : " + line + "\n")
			labelled = true
		default:
			buf.WriteString("                  " + line + "\n")
		}
	}
	return buf.String()
}

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/viper"
)

type Provider interface {
	Setup()
	Query(string) []Item
}

var providers map[string]Provider

type Item struct {
	Labels     []string
	Icon       string
	Identifier string
	Provider   string
	score      int
}

type QueryRequest struct {
	Autoselect bool     `json:"autoselect"`
	Providers  []string `json:"providers"`
	Query      string   `json:"query"`
}

type ActivationType int

const (
	Primary ActivationType = iota
	Secondary
)

type ActivationRequest struct {
	Identifier string         `json:"identifier"`
	Provider   string         `json:"provider"`
	Type       ActivationType `json:"type"`
	Terminal   bool           `json:"terminal"`
}

var (
	queryRequestIdentifier      = "query"
	activationRequestIdentifier = "activation"
	socket                      = "request.sock"
)

func main() {
	readConfig()

	providers = make(map[string]Provider)

	listen(filepath.Join(tmpDir(), socket))
}

func readConfig() {
	slog.Info("reading config")

	dir, err := os.UserConfigDir()
	if err != nil {
		panic(err)
	}

	dir = filepath.Join(dir, "runner")

	viper.SetConfigName("config")
	viper.AddConfigPath(dir)
	viper.SetDefault("providers", []string{"applications", "runner"})

	err = viper.ReadInConfig()
	if err != nil {
		panic(err)
	}

	setTerminal()

	fmt.Println(viper.AllSettings())
}

func setTerminal() {
	slog.Info("setting terminal")

	t := []string{
		"Eterm",
		"alacritty",
		"aterm",
		"foot",
		"gnome-terminal",
		"guake",
		"hyper",
		"kitty",
		"konsole",
		"lilyterm",
		"lxterminal",
		"mate-terminal",
		"qterminal",
		"roxterm",
		"rxvt",
		"st",
		"terminator",
		"terminix",
		"terminology",
		"termit",
		"termite",
		"tilda",
		"tilix",
		"urxvt",
		"uxterm",
		"wezterm",
		"x-terminal-emulator",
		"xfce4-terminal",
		"xterm",
	}

	term := viper.GetString("terminal")

	if term != "" {
		t = append([]string{term}, t...)
	}

	for _, v := range []string{"TERM", "TERMINAL"} {
		val, ok := os.LookupEnv(v)
		if ok {
			t = append([]string{val}, t...)
		}
	}

	for _, v := range t {
		path, _ := exec.LookPath(v)

		if path != "" {
			viper.Set("terminal", v)
			break
		}
	}

	slog.Info("terminal set to", "term", viper.GetString("terminal"))
}

func listen(sock string) {
	slog.Info("start listening...")

	err := os.Remove(sock)
	if err != nil {
		panic(err)
	}

	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: sock})
	if err != nil {
		panic(err)
	}
	defer l.Close()

	for {
		conn, err := l.AcceptUnix()
		if err != nil {
			slog.Error("error accepting connection", "error", err.Error())
		}

		slog.Info("new connection")

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 5120)

	n, err := conn.Read(buf)
	if err != nil {
		slog.Error("error reading request", "error", err.Error())
		return
	}

	cmd := string(bytes.Trim(buf[:20], "\x00"))

	// routing
	switch cmd {
	case queryRequestIdentifier:
		slog.Info("query received")

		req, err := parseQueryRequest(buf[20:n])
		if err != nil {
			slog.Error("error parsing query request", "error", err.Error())

			return
		} else {
			query(req)
		}
	case activationRequestIdentifier:
		slog.Info("activation received")
	default:
		slog.Info("unknown command received")

		return
	}

	// response
	_, err = conn.Write(buf[:n])
	if err != nil {
		slog.Error("error writing response", "error", err.Error())
		return
	}

	slog.Info("response sent")
}

func parseActivationRequest(buf []byte) (ActivationRequest, error) {
	var request ActivationRequest

	err := json.Unmarshal(buf, &request)
	if err != nil {
		return ActivationRequest{}, err
	}

	return request, nil
}

func parseQueryRequest(buf []byte) (QueryRequest, error) {
	var request QueryRequest

	err := json.Unmarshal(buf, &request)
	if err != nil {
		return QueryRequest{}, err
	}

	return request, nil
}

func query(req QueryRequest) {
	slog.Info("processing query")
	fmt.Println(req)
}

func activate(req ActivationRequest) {
	fmt.Println(req)
}

func tmpDir() string {
	tmpRoot := os.TempDir()
	tmpDir := filepath.Join(tmpRoot, "runner")

	err := os.MkdirAll(tmpDir, 0700)
	if err != nil {
		panic(err)
	}

	return tmpDir
}

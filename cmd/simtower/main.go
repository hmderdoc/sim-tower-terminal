// SimTower door: a faithful SimTower (Maxis, 1994) for BBS terminals.
//
// Door mode: the BBS hands us DOOR32.SYS (-dropfile); line 1 type 2 means we
// talk over the inherited telnet socket. Without a dropfile the door runs on
// stdin/stdout for local play and development.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"simtower/internal/term"
	"simtower/internal/ui"
)

var version = "dev"

func main() {
	dropfile := flag.String("dropfile", "", "path to DOOR32.SYS (door mode)")
	user := flag.String("user", "", "player name override (default: dropfile alias, else $USER)")
	dataDir := flag.String("data", "", "save directory (default: <binary dir>/data)")
	size := flag.String("size", "", "force terminal size, e.g. 132x50 (default: probe)")
	logPath := flag.String("log", "", "debug log file (default: <binary dir>/simtower.log)")
	showVersion := flag.Bool("version", false, "print version and exit")
	demo := flag.Bool("demo", false, "run the UI on the scripted demo tower (no sim)")
	trace := flag.Bool("trace", os.Getenv("SIMTOWER_TRACE") != "", "log every input event (also SIMTOWER_TRACE=1, or a file named trace in the data dir)")
	flag.Parse()
	if *showVersion {
		fmt.Println("simtower", version)
		return
	}

	exeDir := "."
	if exe, err := os.Executable(); err == nil {
		exeDir = filepath.Dir(exe)
	}
	if *dataDir == "" {
		*dataDir = filepath.Join(exeDir, "data")
	}
	if *logPath == "" {
		*logPath = filepath.Join(exeDir, "simtower.log")
	}
	var logger *log.Logger
	if lf, err := os.OpenFile(*logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		logger = log.New(lf, "", log.LstdFlags)
		defer lf.Close()
	} else {
		logger = log.New(os.Stderr, "", log.LstdFlags)
	}

	drop, haveDrop := term.ReadDoor32(*dropfile)
	if *dropfile != "" && !haveDrop {
		logger.Printf("dropfile %q unreadable; using stdio", *dropfile)
	}
	name := *user
	if name == "" && haveDrop {
		name = drop.Alias
	}
	if name == "" {
		name = os.Getenv("USER")
	}
	if name == "" {
		name = "player"
	}

	t, restore := term.Open(drop, haveDrop, logger)
	defer restore()
	defer t.Close()

	cols, rows := 80, 24
	if *size != "" {
		fmt.Sscanf(*size, "%dx%d", &cols, &rows)
	} else {
		if c, r, ok := term.ProbeSize(t, 1500*time.Millisecond); ok {
			cols, rows = c, r
		} else if c, r, ok := term.LocalSize(); ok && !haveDrop {
			cols, rows = c, r
		}
	}
	if cols < 80 || rows < 24 {
		t.Write([]byte(fmt.Sprintf("\r\nSimTower needs a terminal of at least 80x24 (yours reports %dx%d).\r\n", cols, rows)))
		return
	}
	logger.Printf("start user=%q size=%dx%d door=%v version=%s", name, cols, rows, haveDrop, version)

	events := make(chan term.Event, 256)
	go term.ReadLoop(t, events)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		events <- term.Event{Disconnect: true}
	}()

	if !haveDrop {
		watchResize(events) // local tty: follow window resizes (SIGWINCH on unix)
	}
	probe := 3 * time.Second
	if *size != "" || !haveDrop {
		probe = 0 // forced size, or a local tty (watchResize covers it)
	}
	savePath := filepath.Join(*dataDir, saveName(name)+".json")
	var model ui.Model
	var launcher *ui.Launcher
	if *demo {
		model = ui.NewDemo()
	} else {
		// The start menu offers Continue (when a save exists), New tower,
		// Demo and Quit; the demo tower is only its backdrop.
		model = ui.NewDemo()
		launcher = &ui.Launcher{
			SaveInfo: ui.SaveSummary(savePath),
			Continue: func() (ui.Model, error) {
				m, err := ui.LoadSimModel(savePath)
				if err != nil {
					return nil, err
				}
				logger.Printf("loaded %s", savePath)
				return m, nil
			},
			NewGame: func() (ui.Model, error) {
				logger.Printf("new tower for %q", name)
				m := ui.NewSimModel(saveName(name), name+"'s tower", savePath)
				return m, m.Save()
			},
			Demo: func() ui.Model { return ui.NewDemo() },
		}
		if _, err := os.Stat(savePath); err == nil && launcher.SaveInfo == "" {
			logger.Printf("save %s exists but is unreadable", savePath)
			launcher.SaveInfo = "Saved tower (unreadable, see log)"
		}
	}

	t.Write([]byte(term.MouseOn + "\x1b[?25l"))
	app := ui.New(ui.Config{
		Model:      model,
		ProbeEvery: probe,
		Term:       t,
		Events:     events,
		Cols:       cols,
		Rows:       rows,
		Player:     name,
		SavePath:   savePath,
		Launcher:   launcher,
		Logger:     logger,
		Trace:      *trace || fileExists(filepath.Join(*dataDir, "trace")),
		Version:    version,
	})
	app.Run()
	t.Write([]byte("\x1b[?1003l" + term.MouseOff + "\x1b[0m\x1b[?25h\x1b[2J\x1b[H"))
	logger.Printf("exit user=%q", name)
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)

func saveName(alias string) string {
	s := unsafeName.ReplaceAllString(strings.TrimSpace(alias), "_")
	if s == "" || s == "." || s == ".." {
		s = "player"
	}
	return strings.ToLower(s)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

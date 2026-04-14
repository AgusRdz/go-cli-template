package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/agusrdz/mytool/config"
	"github.com/agusrdz/mytool/hooks"
	"github.com/agusrdz/mytool/updater"
	"github.com/mattn/go-isatty"
)

var version = "dev"

func main() {
	// Apply any pending auto-update from a previous run
	updater.ApplyPendingUpdate(version)
	// Show hint if a newer version is available and auto-update is off
	updater.NotifyIfUpdateAvailable(version)

	if len(os.Args) < 2 {
		// No args — hook mode if stdin is a pipe, otherwise help.
		if isatty.IsTerminal(os.Stdin.Fd()) && !isatty.IsCygwinTerminal(os.Stdin.Fd()) {
			printHelp()
			return
		}
		runHook()
		return
	}

	switch os.Args[1] {
	case "--help", "help", "-h":
		printHelp()

	case "--version", "version":
		fmt.Printf("mytool %s\n", version)

	case "--post-update-check":
		checkInstallDir()
		return

	case "--_bg-update":
		if len(os.Args) >= 3 {
			updater.RunBackgroundUpdate(os.Args[2])
		}
		return

	case "update":
		updater.Run(version)
		return

	case "auto-update":
		runAutoUpdate(os.Args[2:])
		return

	case "init", "setup":
		runInit()

	case "uninstall":
		runUninstall()

	case "enable":
		if hooks.IsDisabledGlobally() {
			if err := hooks.Enable(); err != nil {
				fmt.Fprintf(os.Stderr, "mytool: failed to enable: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("mytool enabled — checks will run on every Write")
		} else {
			fmt.Println("mytool is already enabled")
		}

	case "disable":
		if hooks.IsDisabledGlobally() {
			fmt.Println("mytool is already disabled")
		} else {
			if err := hooks.Disable(); err != nil {
				fmt.Fprintf(os.Stderr, "mytool: failed to disable: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("mytool disabled — hook will pass through all Writes")
			fmt.Println("run 'mytool enable' to resume")
		}

	case "doctor":
		runDoctor()

	case "run":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: mytool run <path>")
			os.Exit(1)
		}
		// TODO: implement manual run logic.
		fmt.Printf("mytool run %s — not yet implemented\n", os.Args[2])

	case "config":
		if len(os.Args) < 3 || os.Args[2] == "show" {
			cwd, _ := os.Getwd()
			config.Show(cwd)
		} else {
			fmt.Fprintf(os.Stderr, "unknown config subcommand %q\nusage: mytool config show\n", os.Args[2])
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\nrun 'mytool help' for usage\n", os.Args[1])
		os.Exit(1)
	}
}

// runHook is called when the tool is invoked by the Claude Code PostToolUse hook (stdin is a pipe).
// Replace this with your hook logic. The input JSON schema depends on the hook event.
func runHook() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		respond(fmt.Sprintf("mytool: failed to read stdin: %v", err))
		return
	}

	// TODO: parse data and implement hook logic.
	_ = data

	if hooks.IsDisabledGlobally() {
		respond("")
		return
	}

	respond("TODO: hook output")
}

// respond writes the PostToolUse JSON response to stdout.
func respond(output string) {
	resp := map[string]string{
		"action": "continue",
		"output": output,
	}
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}

func runInit() {
	if len(os.Args) < 3 {
		hooks.Install(version)
		return
	}
	switch os.Args[2] {
	case "--status":
		installed, path := hooks.IsInstalled()
		if installed {
			fmt.Printf("mytool hook is installed (%s)\n", path)
		} else {
			fmt.Println("mytool hook is NOT installed")
			fmt.Println("run 'mytool init' to install")
		}
	case "--uninstall":
		hooks.Uninstall()
	default:
		fmt.Fprintf(os.Stderr, "unknown flag %q\nusage: mytool init [--status|--uninstall]\n", os.Args[2])
		os.Exit(1)
	}
}

func runUninstall() {
	hooks.Uninstall()
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".config", "mytool")
	cacheDir := filepath.Join(home, ".cache", "mytool")
	os.RemoveAll(configDir)
	os.RemoveAll(cacheDir)
	fmt.Println("mytool uninstalled")
	fmt.Printf("  hook removed from ~/.claude/settings.json\n")
	fmt.Printf("  config removed:  %s\n", configDir)
	fmt.Printf("  cache removed:   %s\n", cacheDir)
	fmt.Println("\nbinary not removed — delete manually or via your package manager")
}

func runDoctor() {
	issues := 0

	// 1. Hook installed?
	installed, _ := hooks.IsInstalled()
	if !installed {
		fmt.Println("[!] hook is not installed")
		fmt.Println("    fix: mytool init")
		issues++
	} else {
		hookCmd := hooks.GetHookCommand()
		exe, err := os.Executable()
		if err == nil {
			exe, _ = filepath.EvalSymlinks(exe)
		}
		if err == nil && hookCmd != exe {
			fmt.Println("[!] hook points to wrong binary")
			fmt.Printf("    current:  %s\n", hookCmd)
			fmt.Printf("    expected: %s\n", exe)
			fmt.Println("    fix: mytool init")
			issues++
		} else {
			fmt.Println("[ok] hook is installed and path is correct")
		}
	}

	// 2. Disabled?
	if hooks.IsDisabledGlobally() {
		fmt.Println("[!] mytool is disabled — hook passes through all Writes")
		fmt.Println("    fix: mytool enable")
		issues++
	}

	// 3. Config valid?
	cfgPath := config.Path()
	if _, err := os.Stat(cfgPath); err == nil {
		if _, err := config.Load(""); err != nil {
			fmt.Printf("[!] config file has errors: %s\n", cfgPath)
			fmt.Printf("    %v\n", err)
			issues++
		} else {
			fmt.Printf("[ok] config is valid (%s)\n", cfgPath)
		}
	} else {
		fmt.Println("[ok] no global config (using defaults)")
	}

	// 4. Legacy binary location warning (Windows)
	if runtime.GOOS == "windows" {
		if exe, err := os.Executable(); err == nil {
			if exe, err = filepath.EvalSymlinks(exe); err == nil {
				home, _ := os.UserHomeDir()
				if strings.HasPrefix(exe, filepath.Join(home, "bin")) {
					fmt.Println("[!] binary is in ~/bin — consider moving to %LOCALAPPDATA%/Programs/mytool")
					issues++
				}
			}
		}
	}

	if issues == 0 {
		fmt.Println("\nall good!")
	} else {
		fmt.Printf("\n%d issue(s) found\n", issues)
	}
}

func runAutoUpdate(args []string) {
	if len(args) == 0 {
		if updater.IsAutoUpdateEnabled() {
			fmt.Println("auto-update: on")
		} else {
			fmt.Println("auto-update: off")
			fmt.Println("mytool will notify you when updates are available")
			fmt.Println("run 'mytool auto-update on' to enable automatic updates")
		}
		return
	}

	switch args[0] {
	case "on":
		if updater.IsAutoUpdateEnabled() {
			fmt.Println("auto-update is already on")
			return
		}
		if err := updater.SetAutoUpdate(true); err != nil {
			fmt.Fprintf(os.Stderr, "mytool: failed to enable auto-update: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("auto-update enabled — mytool will update itself in the background")
	case "off":
		if !updater.IsAutoUpdateEnabled() {
			fmt.Println("auto-update is already off")
			return
		}
		if err := updater.SetAutoUpdate(false); err != nil {
			fmt.Fprintf(os.Stderr, "mytool: failed to disable auto-update: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("auto-update disabled")
	default:
		fmt.Fprintf(os.Stderr, "usage: mytool auto-update [on|off]\n")
		os.Exit(1)
	}
}

func checkInstallDir() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	oldDir := filepath.Join(home, "bin")
	if !strings.HasPrefix(exe, oldDir+string(filepath.Separator)) {
		return
	}

	fmt.Println("")
	fmt.Println("note: mytool is installed in ~/bin, which is no longer the recommended location.")
	if runtime.GOOS == "windows" {
		fmt.Println("move it to %LOCALAPPDATA%\\Programs\\mytool and update your PATH.")
	} else {
		fmt.Println("move it to ~/.local/bin and update your PATH.")
	}
}

func printHelp() {
	const colW = 38
	section := func(name string) string { return bold(cyan(name)) + "\n" }
	row := func(cmd, desc string) string {
		return fmt.Sprintf("  %-*s%s\n", colW, cmd, dim(desc))
	}
	flag := func(f string) string { return yellow(f) }

	var b strings.Builder

	b.WriteString(fmt.Sprintf("%s %s — TODO: describe your tool here\n\n", bold("mytool"), version))

	b.WriteString(bold("Usage") + "\n")
	b.WriteString(row("mytool", "TODO: describe default invocation"))
	b.WriteString(row("mytool <subcommand>", "Run a management subcommand"))
	b.WriteString("\n")

	b.WriteString(section("Setup"))
	b.WriteString(row("init", "Install Claude Code hook (~/.claude/settings.json)"))
	b.WriteString(row("init "+flag("--status"), "Check hook installation status"))
	b.WriteString(row("init "+flag("--uninstall"), "Remove the hook from settings.json"))
	b.WriteString(row("uninstall", "Remove hook, config, and cache"))
	b.WriteString("\n")

	b.WriteString(section("Maintenance"))
	b.WriteString(row("doctor", "Check hook, config, and binary health"))
	b.WriteString(row("enable / disable", "Resume or bypass mytool globally"))
	b.WriteString(row("config show", "Show resolved config for current directory"))
	b.WriteString("\n")

	b.WriteString(section("Debug"))
	b.WriteString(row("run <path>", "Run checks on a file manually (bypass hook)"))
	b.WriteString("\n")

	b.WriteString(section("Updates"))
	b.WriteString(row("update", "Check and apply latest version"))
	b.WriteString(row("auto-update", "Show auto-update status"))
	b.WriteString(row("auto-update on / off", "Enable or disable background updates"))
	b.WriteString("\n")

	b.WriteString(section("Other"))
	b.WriteString(row("version", "Show version"))
	b.WriteString(row("help", "Show this help"))
	b.WriteString("\n")

	b.WriteString(bold("Config") + "\n")
	b.WriteString(dim(fmt.Sprintf("  global:  %s\n", config.Path())))
	b.WriteString(dim("  project: .mytool.yml (walk-up from written file)\n"))
	b.WriteString(dim("  Run 'mytool config show' to see the resolved config.\n"))
	b.WriteString("\n")

	b.WriteString(bold("Examples") + "\n")
	b.WriteString(row("mytool init", "Install hook and get started"))
	b.WriteString(row("mytool doctor", "Diagnose any setup issues"))
	b.WriteString(row("mytool disable", "Temporarily bypass mytool"))

	fmt.Print(b.String())
}
